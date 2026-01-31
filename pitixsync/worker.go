package pitixsync

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/mmdatafocus/books_backend/config"
	"github.com/mmdatafocus/books_backend/models"
	"github.com/mmdatafocus/books_backend/utils"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

type pitixCustomer struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Email     string `json:"email"`
	Phone     string `json:"phone"`
	Mobile    string `json:"mobile"`
	UpdatedAt string `json:"updated_at"`
}

type pitixItem struct {
	ID             string          `json:"id"`
	Name           string          `json:"name"`
	Sku            string          `json:"sku"`
	Barcode        string          `json:"barcode"`
	Description    string          `json:"description"`
	Active         bool            `json:"active"`
	TrackInventory bool            `json:"track_inventory"`
	SellingPrice   json.Number     `json:"selling_price"`
	CostPrice      json.Number     `json:"cost_price"`
	UpdatedAt      string          `json:"updated_at"`
	Stocks         []pitixItemStock `json:"stocks"`
	StockItems     []pitixItemStock `json:"stock_items"`
	Taxes          []pitixTax       `json:"taxes"`
}

type pitixItemStock struct {
	SellingPrice json.Number `json:"selling_price"`
	CostPrice    json.Number `json:"cost_price"`
	Sku          string      `json:"sku"`
	Barcode      string      `json:"barcode"`
}

type pitixSale struct {
	ID           string        `json:"id"`
	SaleNumber   string        `json:"sale_number"`
	SaleDate     string        `json:"sale_date"`
	SaleStatus   string        `json:"sale_status"`
	PaymentStatus string       `json:"payment_status"`
	CustomerId   string        `json:"customer_id"`
	NetAmount    json.Number   `json:"net_amount"`
	TaxAmount    json.Number   `json:"tax_amount"`
	DiscountAmount json.Number `json:"discount_amount"`
	ShippingAmount json.Number `json:"shipping_amount"`
	Items        []pitixSaleItem `json:"items"`
	UpdatedAt    string        `json:"updated_at"`
	PaymentMethod   string     `json:"payment_method"`
	PaymentMethodId string     `json:"payment_method_id"`
}

type pitixSaleItem struct {
	ID         string      `json:"id"`
	Name       string      `json:"name"`
	StockId    string      `json:"stock_id"`
	ProductId  string      `json:"product_id"`
	ItemId     string      `json:"item_id"`
	Quantity   json.Number `json:"quantity"`
	UnitPrice  json.Number `json:"unit_price"`
	NetAmount  json.Number `json:"net_amount"`
	DiscountAmount json.Number `json:"discount_amount"`
	Taxes      []pitixTax  `json:"taxes"`
}

type pitixTax struct {
	ID   string      `json:"id"`
	Name string      `json:"name"`
	Rate json.Number `json:"rate"`
}

func processSyncRun(ctx context.Context, payload SyncPubSubPayload) error {
	if payload.RunId == 0 || payload.BusinessId == "" {
		return errors.New("invalid payload")
	}

	ctx = utils.SetBusinessIdInContext(ctx, payload.BusinessId)
	db := config.GetDB().WithContext(ctx)

	var run models.IntegrationSyncRun
	if err := db.Where("id = ? AND business_id = ?", payload.RunId, payload.BusinessId).Take(&run).Error; err != nil {
		return err
	}

	if run.Status == models.SyncRunStatusSuccess || run.Status == models.SyncRunStatusFailed || run.Status == models.SyncRunStatusPartial {
		return nil
	}

	var conn models.IntegrationConnection
	if err := db.Where("id = ? AND business_id = ?", run.ConnectionId, payload.BusinessId).Take(&conn).Error; err != nil {
		return err
	}
	if conn.Status != models.IntegrationStatusConnected {
		return errors.New("pitix not connected")
	}

	modules := DecodeModules(run.ModulesJSON)
	cursorState := DecodeCursorState(conn.CursorStateJSON)

	now := time.Now()
	startedAt := run.StartedAt
	if startedAt == nil {
		startedAt = &now
	}

	if err := db.Model(&run).Updates(map[string]interface{}{
		"status":     models.SyncRunStatusRunning,
		"started_at": startedAt,
	}).Error; err != nil {
		return err
	}

	client, err := newPitixClient(conn.AuthSecretRef)
	if err != nil {
		return err
	}

	stats := map[string]int{
		"customers": 0,
		"items":     0,
		"invoices":  0,
	}
	errorCount := 0

	if modules.Customers {
		count, newCursor, newUpdatedSince, err := syncCustomers(ctx, db, run.ID, payload.BusinessId, conn, client, cursorState.Customers)
		if err != nil {
			errorCount++
			_ = createSyncError(ctx, db, run.ID, payload.BusinessId, "customers", "", "sync_failed", err.Error(), nil, true)
		} else {
			stats["customers"] = count
			cursorState.Customers = CursorEntry{UpdatedSince: newUpdatedSince, Cursor: newCursor}
		}
	}

	if modules.Items {
		count, newCursor, newUpdatedSince, err := syncItems(ctx, db, run.ID, payload.BusinessId, conn, client, cursorState.Items)
		if err != nil {
			errorCount++
			_ = createSyncError(ctx, db, run.ID, payload.BusinessId, "items", "", "sync_failed", err.Error(), nil, true)
		} else {
			stats["items"] = count
			cursorState.Items = CursorEntry{UpdatedSince: newUpdatedSince, Cursor: newCursor}
		}
	}

	if modules.Invoices {
		count, newCursor, newUpdatedSince, err := syncInvoices(ctx, db, run.ID, payload.BusinessId, conn, client, cursorState.Invoices)
		if err != nil {
			errorCount++
			_ = createSyncError(ctx, db, run.ID, payload.BusinessId, "invoices", "", "sync_failed", err.Error(), nil, true)
		} else {
			stats["invoices"] = count
			cursorState.Invoices = CursorEntry{UpdatedSince: newUpdatedSince, Cursor: newCursor}
		}
	}

	finishedAt := time.Now()
	durationMs := finishedAt.Sub(*startedAt).Milliseconds()
	status := models.SyncRunStatusSuccess
	totalSynced := stats["customers"] + stats["items"] + stats["invoices"]
	if errorCount > 0 && totalSynced == 0 {
		status = models.SyncRunStatusFailed
	} else if errorCount > 0 {
		status = models.SyncRunStatusPartial
	}

	statsJSON, _ := json.Marshal(stats)
	cursorJSON := EncodeCursorState(cursorState)
	if err := db.Model(&run).Updates(map[string]interface{}{
		"status":        status,
		"finished_at":   finishedAt,
		"duration_ms":   durationMs,
		"records_synced": totalSynced,
		"error_count":   errorCount,
		"stats_json":    statsJSON,
		"cursor_state_json": cursorJSON,
	}).Error; err != nil {
		return err
	}

	connUpdates := map[string]interface{}{
		"last_sync_at":      finishedAt,
		"cursor_state_json": cursorJSON,
	}
	if status == models.SyncRunStatusSuccess {
		connUpdates["last_success_sync_at"] = finishedAt
	}
	if err := db.Model(&models.IntegrationConnection{}).
		Where("id = ? AND business_id = ?", conn.ID, payload.BusinessId).
		Updates(connUpdates).Error; err != nil {
		return err
	}

	return nil
}

func syncCustomers(ctx context.Context, db *gorm.DB, runID uint, businessId string, conn models.IntegrationConnection, client *pitixClient, cursor CursorEntry) (int, string, string, error) {
	business, err := models.GetBusinessById(ctx, businessId)
	if err != nil {
		return 0, cursor.Cursor, cursor.UpdatedSince, err
	}

	updatedSince := strings.TrimSpace(cursor.UpdatedSince)
	if updatedSince == "" && conn.LastSuccessSyncAt != nil {
		updatedSince = conn.LastSuccessSyncAt.UTC().Format(time.RFC3339)
	}
	if updatedSince == "" {
		updatedSince = time.Now().Add(-30 * 24 * time.Hour).UTC().Format(time.RFC3339)
	}

	nextCursor := strings.TrimSpace(cursor.Cursor)
	total := 0

	for {
		params := url.Values{}
		params.Set("updated_since", updatedSince)
		if nextCursor != "" {
			params.Set("cursor", nextCursor)
		}
		params.Set("limit", "200")

		resp, err := client.getList(ctx, "/v1/customers", params)
		if err != nil {
			return total, nextCursor, updatedSince, err
		}

		items := resp.Data
		if len(items) == 0 {
			items = resp.Items
		}

		for _, raw := range items {
			var cust pitixCustomer
			if err := json.Unmarshal(raw, &cust); err != nil {
				_ = createSyncError(ctx, db, runID, businessId, "customer", "", "invalid_payload", err.Error(), raw, true)
				continue
			}
			extID := strings.TrimSpace(cust.ID)
			if extID == "" {
				_ = createSyncError(ctx, db, runID, businessId, "customer", "", "missing_id", "customer id missing", raw, false)
				continue
			}

			name := strings.TrimSpace(cust.Name)
			if name == "" {
				name = fmt.Sprintf("PitiX Customer %s", extID)
			}

			input := &models.NewCustomer{
				Name:                 name,
				Email:                strings.TrimSpace(cust.Email),
				Phone:                strings.TrimSpace(cust.Phone),
				Mobile:               strings.TrimSpace(cust.Mobile),
				CurrencyId:           business.BaseCurrencyId,
				CustomerPaymentTerms: models.PaymentTermsDueOnReceipt,
			}

			internalID, err := upsertCustomer(ctx, db, businessId, conn.ID, extID, input)
			if err != nil {
				_ = createSyncError(ctx, db, runID, businessId, "customer", extID, "sync_failed", err.Error(), raw, true)
				continue
			}
			total++
			_ = touchMapping(ctx, db, businessId, conn.ID, models.IntegrationProviderPitiX, "customer", extID, internalID, cust.UpdatedAt)
		}

		if resp.NextCursor == "" || (resp.HasMore != nil && !*resp.HasMore) {
			return total, resp.NextCursor, updatedSince, nil
		}
		nextCursor = resp.NextCursor
	}
}

func syncItems(ctx context.Context, db *gorm.DB, runID uint, businessId string, conn models.IntegrationConnection, client *pitixClient, cursor CursorEntry) (int, string, string, error) {
	updatedSince := strings.TrimSpace(cursor.UpdatedSince)
	if updatedSince == "" && conn.LastSuccessSyncAt != nil {
		updatedSince = conn.LastSuccessSyncAt.UTC().Format(time.RFC3339)
	}
	if updatedSince == "" {
		updatedSince = time.Now().Add(-30 * 24 * time.Hour).UTC().Format(time.RFC3339)
	}

	nextCursor := strings.TrimSpace(cursor.Cursor)
	total := 0

	itemsPath := strings.TrimSpace(os.Getenv("PITIX_ITEMS_PATH"))
	if itemsPath == "" {
		itemsPath = "/v1/items"
	}

	for {
		params := url.Values{}
		params.Set("updated_since", updatedSince)
		if nextCursor != "" {
			params.Set("cursor", nextCursor)
		}
		params.Set("limit", "200")

		resp, err := client.getList(ctx, itemsPath, params)
		if err != nil {
			return total, nextCursor, updatedSince, err
		}

		items := resp.Data
		if len(items) == 0 {
			items = resp.Items
		}

		for _, raw := range items {
			var item pitixItem
			if err := json.Unmarshal(raw, &item); err != nil {
				_ = createSyncError(ctx, db, runID, businessId, "item", "", "invalid_payload", err.Error(), raw, true)
				continue
			}
			extID := strings.TrimSpace(item.ID)
			if extID == "" {
				_ = createSyncError(ctx, db, runID, businessId, "item", "", "missing_id", "item id missing", raw, false)
				continue
			}

			sku := strings.TrimSpace(item.Sku)
			if sku == "" {
				sku = "PITIX-" + extID
			}
			barcode := strings.TrimSpace(item.Barcode)
			if barcode == "" {
				barcode = "PITIX-" + extID
			}
			if sku == "" || barcode == "" {
				_ = createSyncError(ctx, db, runID, businessId, "item", extID, "missing_sku_barcode", "sku/barcode required", raw, false)
				continue
			}

			salesPrice := decimalFromNumber(item.SellingPrice)
			purchasePrice := decimalFromNumber(item.CostPrice)
			if salesPrice.IsZero() || purchasePrice.IsZero() {
				stock := pickFirstStock(item)
				if salesPrice.IsZero() {
					salesPrice = decimalFromNumber(stock.SellingPrice)
				}
				if purchasePrice.IsZero() {
					purchasePrice = decimalFromNumber(stock.CostPrice)
				}
			}

			salesAccountId, inventoryAccountId, purchaseAccountId, accountErr := getDefaultAccounts(ctx, businessId)
			if accountErr != nil {
				_ = createSyncError(ctx, db, runID, businessId, "item", extID, "default_accounts_missing", accountErr.Error(), raw, false)
			}

			taxID, taxType, taxErr := resolveTaxMapping(ctx, db, businessId, conn.ID, item.Taxes)
			if taxErr != nil {
				_ = createSyncError(ctx, db, runID, businessId, "item", extID, "tax_mapping_failed", taxErr.Error(), raw, true)
			}

			input := &models.NewProduct{
				Name:                strings.TrimSpace(item.Name),
				Description:         strings.TrimSpace(item.Description),
				Sku:                 sku,
				Barcode:             barcode,
				SalesPrice:          salesPrice,
				PurchasePrice:       purchasePrice,
				SalesAccountId:      salesAccountId,
				PurchaseAccountId:   purchaseAccountId,
				InventoryAccountId:  inventoryAccountId,
				SalesTaxId:          taxID,
				SalesTaxType:        taxType,
				IsSalesTaxInclusive: utils.NewFalse(),
				IsBatchTracking:     utils.NewFalse(),
			}
			if input.Name == "" {
				input.Name = "PitiX Item " + extID
			}

			internalID, err := upsertProduct(ctx, db, businessId, conn.ID, extID, input)
			if err != nil {
				_ = createSyncError(ctx, db, runID, businessId, "item", extID, "sync_failed", err.Error(), raw, true)
				continue
			}
			total++
			_ = touchMapping(ctx, db, businessId, conn.ID, models.IntegrationProviderPitiX, "item", extID, internalID, item.UpdatedAt)
		}

		if resp.NextCursor == "" || (resp.HasMore != nil && !*resp.HasMore) {
			return total, resp.NextCursor, updatedSince, nil
		}
		nextCursor = resp.NextCursor
	}
}

func syncInvoices(ctx context.Context, db *gorm.DB, runID uint, businessId string, conn models.IntegrationConnection, client *pitixClient, cursor CursorEntry) (int, string, string, error) {
	updatedSince := strings.TrimSpace(cursor.UpdatedSince)
	if updatedSince == "" && conn.LastSuccessSyncAt != nil {
		updatedSince = conn.LastSuccessSyncAt.UTC().Format(time.RFC3339)
	}
	if updatedSince == "" {
		updatedSince = time.Now().Add(-30 * 24 * time.Hour).UTC().Format(time.RFC3339)
	}

	nextCursor := strings.TrimSpace(cursor.Cursor)
	total := 0

	salesPath := strings.TrimSpace(os.Getenv("PITIX_SALES_PATH"))
	if salesPath == "" {
		salesPath = "/v1/sales"
	}

	for {
		params := url.Values{}
		params.Set("updated_since", updatedSince)
		if nextCursor != "" {
			params.Set("cursor", nextCursor)
		}
		params.Set("limit", "200")

		resp, err := client.getList(ctx, salesPath, params)
		if err != nil {
			return total, nextCursor, updatedSince, err
		}

		items := resp.Data
		if len(items) == 0 {
			items = resp.Items
		}

		for _, raw := range items {
			var sale pitixSale
			if err := json.Unmarshal(raw, &sale); err != nil {
				_ = createSyncError(ctx, db, runID, businessId, "invoice", "", "invalid_payload", err.Error(), raw, true)
				continue
			}
			extID := strings.TrimSpace(sale.ID)
			if extID == "" {
				_ = createSyncError(ctx, db, runID, businessId, "invoice", "", "missing_id", "sale id missing", raw, false)
				continue
			}

			existing, err := findMapping(ctx, db, businessId, conn.ID, "invoice", extID)
			if err != nil {
				_ = createSyncError(ctx, db, runID, businessId, "invoice", extID, "mapping_error", err.Error(), raw, true)
				continue
			}
			if existing != nil {
				total++
				continue
			}

			customerID, err := resolveCustomerForSale(ctx, db, businessId, conn.ID, sale.CustomerId)
			if err != nil {
				_ = createSyncError(ctx, db, runID, businessId, "invoice", extID, "customer_missing", err.Error(), raw, true)
				continue
			}

			branchID, warehouseID, err := resolveBranchWarehouse(ctx, businessId)
			if err != nil {
				_ = createSyncError(ctx, db, runID, businessId, "invoice", extID, "warehouse_missing", err.Error(), raw, true)
				continue
			}

			if sale.PaymentMethodId != "" || sale.PaymentMethod != "" {
				if _, err := ensurePaymentModeMapping(ctx, db, businessId, conn.ID, sale.PaymentMethodId, sale.PaymentMethod); err != nil {
					_ = createSyncError(ctx, db, runID, businessId, "payment_mode", sale.PaymentMethodId, "payment_mode_mapping_failed", err.Error(), raw, true)
				}
			}

			invoiceDate := parseTimeOrNow(sale.SaleDate)
			invoiceNumber := strings.TrimSpace(sale.SaleNumber)
			if invoiceNumber == "" {
				invoiceNumber = "PITIX-" + extID
			}

			var details []models.NewSalesInvoiceDetail
			for _, item := range sale.Items {
				extItemId := strings.TrimSpace(item.ProductId)
				if extItemId == "" {
					extItemId = strings.TrimSpace(item.StockId)
				}
				if extItemId == "" {
					extItemId = strings.TrimSpace(item.ItemId)
				}

				productID := 0
				if extItemId != "" {
					if mapping, err := findMapping(ctx, db, businessId, conn.ID, "item", extItemId); err == nil && mapping != nil {
						if pid, err := strconv.Atoi(mapping.InternalId); err == nil {
							productID = pid
						}
					}
				}

				qty := decimalFromNumber(item.Quantity)
				if qty.LessThanOrEqual(decimal.Zero) {
					qty = decimal.NewFromInt(1)
				}
				unitPrice := decimalFromNumber(item.UnitPrice)
				if unitPrice.IsZero() {
					unitPrice = decimalFromNumber(item.NetAmount).Div(qty)
				}

				taxID, taxType, taxErr := resolveTaxMapping(ctx, db, businessId, conn.ID, item.Taxes)
				if taxErr != nil {
					_ = createSyncError(ctx, db, runID, businessId, "invoice", extID, "tax_mapping_failed", taxErr.Error(), raw, true)
				}

				detail := models.NewSalesInvoiceDetail{
					ProductId:      productID,
					ProductType:    models.ProductTypeSingle,
					Name:           strings.TrimSpace(item.Name),
					DetailQty:      qty,
					DetailUnitRate: unitPrice,
					DetailDiscount: decimalFromNumber(item.DiscountAmount),
					DetailTaxId:    taxID,
					DetailTaxType:  taxType,
				}
				if detail.Name == "" {
					detail.Name = "PitiX Item"
				}
				details = append(details, detail)
			}

			if len(details) == 0 {
				_ = createSyncError(ctx, db, runID, businessId, "invoice", extID, "empty_items", "no sale items", raw, false)
				continue
			}

			currencyId := getBusinessCurrency(ctx, businessId)
			if currencyId == 0 {
				_ = createSyncError(ctx, db, runID, businessId, "invoice", extID, "currency_missing", "business base currency not set", raw, false)
				continue
			}

			input := &models.NewSalesInvoice{
				CustomerId:          customerID,
				BranchId:            branchID,
				InvoiceDate:         invoiceDate,
				InvoicePaymentTerms: models.PaymentTermsDueOnReceipt,
				CurrencyId:          currencyId,
				ExchangeRate:        decimal.NewFromInt(1),
				WarehouseId:         warehouseID,
				IsTaxInclusive:      utils.NewFalse(),
				CurrentStatus:       mapSaleStatus(sale.SaleStatus),
				Details:             details,
				Notes:               "Imported from PitiX",
				ReferenceNumber:     invoiceNumber,
			}

			invoice, err := models.CreateSalesInvoice(ctx, input)
			if err != nil {
				_ = createSyncError(ctx, db, runID, businessId, "invoice", extID, "create_failed", err.Error(), raw, true)
				continue
			}
			internalID := strconv.Itoa(invoice.ID)
			if err := createMapping(ctx, db, businessId, conn.ID, "invoice", extID, internalID); err != nil {
				_ = createSyncError(ctx, db, runID, businessId, "invoice", extID, "mapping_failed", err.Error(), raw, true)
				continue
			}
			total++
			_ = touchMapping(ctx, db, businessId, conn.ID, models.IntegrationProviderPitiX, "invoice", extID, internalID, sale.UpdatedAt)
		}

		if resp.NextCursor == "" || (resp.HasMore != nil && !*resp.HasMore) {
			return total, resp.NextCursor, updatedSince, nil
		}
		nextCursor = resp.NextCursor
	}
}

func upsertCustomer(ctx context.Context, db *gorm.DB, businessId string, connectionId uint, externalId string, input *models.NewCustomer) (string, error) {
	mapping, err := findMapping(ctx, db, businessId, connectionId, "customer", externalId)
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return "", err
	}

	if mapping != nil {
		internalID, err := strconv.Atoi(mapping.InternalId)
		if err != nil {
			return "", err
		}
		if _, err := models.UpdateCustomer(ctx, internalID, input); err != nil {
			return "", err
		}
		return mapping.InternalId, nil
	}

	customer, err := models.CreateCustomer(ctx, input)
	if err != nil {
		return "", err
	}

	internalID := strconv.Itoa(customer.ID)
	if err := createMapping(ctx, db, businessId, connectionId, "customer", externalId, internalID); err != nil {
		return "", err
	}
	return internalID, nil
}

func upsertProduct(ctx context.Context, db *gorm.DB, businessId string, connectionId uint, externalId string, input *models.NewProduct) (string, error) {
	mapping, err := findMapping(ctx, db, businessId, connectionId, "item", externalId)
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return "", err
	}

	if mapping != nil {
		internalID, err := strconv.Atoi(mapping.InternalId)
		if err != nil {
			return "", err
		}
		if _, err := models.UpdateProduct(ctx, internalID, input); err != nil {
			return "", err
		}
		return mapping.InternalId, nil
	}

	product, err := models.CreateProduct(ctx, input)
	if err != nil {
		return "", err
	}

	internalID := strconv.Itoa(product.ID)
	if err := createMapping(ctx, db, businessId, connectionId, "item", externalId, internalID); err != nil {
		return "", err
	}
	return internalID, nil
}

func resolveCustomerForSale(ctx context.Context, db *gorm.DB, businessId string, connectionId uint, externalCustomerId string) (int, error) {
	externalCustomerId = strings.TrimSpace(externalCustomerId)
	if externalCustomerId != "" {
		if mapping, err := findMapping(ctx, db, businessId, connectionId, "customer", externalCustomerId); err == nil && mapping != nil {
			if id, err := strconv.Atoi(mapping.InternalId); err == nil {
				return id, nil
			}
		}
	}
	return getOrCreateWalkInCustomer(ctx, db, businessId)
}

func getDefaultAccounts(ctx context.Context, businessId string) (int, int, int, error) {
	sysAccounts, err := models.GetSystemAccounts(businessId)
	if err != nil {
		return 0, 0, 0, err
	}
	salesAcc := sysAccounts[models.AccountCodeSales]
	invAcc := sysAccounts[models.AccountCodeInventoryAsset]
	purchaseAcc := sysAccounts[models.AccountCodeCostOfGoodsSold]
	if salesAcc == 0 || invAcc == 0 || purchaseAcc == 0 {
		return salesAcc, invAcc, purchaseAcc, errors.New("system default accounts missing")
	}
	return salesAcc, invAcc, purchaseAcc, nil
}

func resolveTaxMapping(ctx context.Context, db *gorm.DB, businessId string, connectionId uint, taxes []pitixTax) (int, *models.TaxType, error) {
	if len(taxes) == 0 {
		return 0, nil, nil
	}
	// Use first tax entry for now.
	tax := taxes[0]
	name := strings.TrimSpace(tax.Name)
	if name == "" {
		name = "PitiX Tax"
	}
	rate := decimalFromNumber(tax.Rate)
	if rate.IsZero() {
		return 0, nil, errors.New("tax rate missing")
	}

	if mapping, err := findMapping(ctx, db, businessId, connectionId, "tax", tax.ID); err == nil && mapping != nil {
		if id, err := strconv.Atoi(mapping.InternalId); err == nil {
			tt := models.TaxTypeIndividual
			return id, &tt, nil
		}
	}

	var existing models.Tax
	if err := db.WithContext(ctx).
		Where("business_id = ? AND name = ? AND rate = ?", businessId, name, rate).
		Take(&existing).Error; err == nil {
		tt := models.TaxTypeIndividual
		_ = createMapping(ctx, db, businessId, connectionId, "tax", tax.ID, strconv.Itoa(existing.ID))
		return existing.ID, &tt, nil
	}

	if strings.EqualFold(strings.TrimSpace(os.Getenv("PITIX_SYNC_CREATE_TAXES")), "true") {
		newTax := models.NewTax{
			Name:          name,
			Rate:          rate,
			IsCompoundTax: utils.NewFalse(),
		}
		created, err := models.CreateTax(ctx, &newTax)
		if err != nil {
			return 0, nil, err
		}
		_ = createMapping(ctx, db, businessId, connectionId, "tax", tax.ID, strconv.Itoa(created.ID))
		tt := models.TaxTypeIndividual
		return created.ID, &tt, nil
	}

	return 0, nil, errors.New("tax not found")
}

func ensurePaymentModeMapping(ctx context.Context, db *gorm.DB, businessId string, connectionId uint, extID string, name string) (int, error) {
	extID = strings.TrimSpace(extID)
	name = strings.TrimSpace(name)
	if name == "" && extID != "" {
		name = "PitiX " + extID
	}
	if name == "" {
		return 0, errors.New("payment mode name missing")
	}

	if mapping, err := findMapping(ctx, db, businessId, connectionId, "payment_mode", extID); err == nil && mapping != nil {
		if id, err := strconv.Atoi(mapping.InternalId); err == nil {
			return id, nil
		}
	}

	var existing models.PaymentMode
	if err := db.WithContext(ctx).
		Where("business_id = ? AND name = ?", businessId, name).
		Take(&existing).Error; err == nil {
		if extID != "" {
			_ = createMapping(ctx, db, businessId, connectionId, "payment_mode", extID, strconv.Itoa(existing.ID))
		}
		_, _ = ensureMoneyAccountMapping(ctx, db, businessId, connectionId, extID, name)
		return existing.ID, nil
	}

	if strings.EqualFold(strings.TrimSpace(os.Getenv("PITIX_SYNC_CREATE_PAYMENT_MODES")), "true") {
		created, err := models.CreatePaymentMode(ctx, &models.NewPaymentMode{Name: name})
		if err != nil {
			return 0, err
		}
		if extID != "" {
			_ = createMapping(ctx, db, businessId, connectionId, "payment_mode", extID, strconv.Itoa(created.ID))
		}
		_, _ = ensureMoneyAccountMapping(ctx, db, businessId, connectionId, extID, name)
		return created.ID, nil
	}

	return 0, errors.New("payment mode not found")
}

func ensureMoneyAccountMapping(ctx context.Context, db *gorm.DB, businessId string, connectionId uint, extID string, name string) (int, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return 0, errors.New("money account name missing")
	}

	var existing models.MoneyAccount
	if err := db.WithContext(ctx).
		Where("business_id = ? AND account_name = ?", businessId, name).
		Take(&existing).Error; err == nil {
		if extID != "" {
			_ = createMapping(ctx, db, businessId, connectionId, "money_account", extID, strconv.Itoa(existing.ID))
		}
		return existing.ID, nil
	}

	if strings.EqualFold(strings.TrimSpace(os.Getenv("PITIX_SYNC_CREATE_MONEY_ACCOUNTS")), "true") {
		currencyId := getBusinessCurrency(ctx, businessId)
		if currencyId == 0 {
			return 0, errors.New("business currency missing")
		}
		created, err := models.CreateMoneyAccount(ctx, &models.NewMoneyAccount{
			AccountType:       models.MoneyAccountTypeCash,
			AccountName:       name,
			AccountCurrencyId: currencyId,
			Description:       "Imported from PitiX",
		})
		if err != nil {
			return 0, err
		}
		if extID != "" {
			_ = createMapping(ctx, db, businessId, connectionId, "money_account", extID, strconv.Itoa(created.ID))
		}
		return created.ID, nil
	}

	return 0, errors.New("money account not found")
}

func getOrCreateWalkInCustomer(ctx context.Context, db *gorm.DB, businessId string) (int, error) {
	var existing models.Customer
	if err := db.WithContext(ctx).
		Where("business_id = ? AND name = ?", businessId, "PitiX Walk-in").
		Take(&existing).Error; err == nil {
		return existing.ID, nil
	}

	input := &models.NewCustomer{
		Name:                 "PitiX Walk-in",
		CurrencyId:           getBusinessCurrency(ctx, businessId),
		CustomerPaymentTerms: models.PaymentTermsDueOnReceipt,
	}
	customer, err := models.CreateCustomer(ctx, input)
	if err != nil {
		return 0, err
	}
	return customer.ID, nil
}

func resolveBranchWarehouse(ctx context.Context, businessId string) (int, int, error) {
	business, err := models.GetBusinessById(ctx, businessId)
	if err != nil {
		return 0, 0, err
	}
	if business.PrimaryBranchId == 0 {
		return 0, 0, errors.New("primary branch not set")
	}

	var warehouseId int
	if err := config.GetDB().
		WithContext(ctx).
		Table("warehouses").
		Select("id").
		Where("business_id = ?", businessId).
		Order("id").
		Limit(1).
		Scan(&warehouseId).Error; err != nil {
		return 0, 0, err
	}
	if warehouseId == 0 {
		return 0, 0, errors.New("warehouse not found")
	}
	return business.PrimaryBranchId, warehouseId, nil
}

func getBusinessCurrency(ctx context.Context, businessId string) int {
	business, err := models.GetBusinessById(ctx, businessId)
	if err != nil {
		return 0
	}
	return business.BaseCurrencyId
}

func pickFirstStock(item pitixItem) pitixItemStock {
	if len(item.Stocks) > 0 {
		return item.Stocks[0]
	}
	if len(item.StockItems) > 0 {
		return item.StockItems[0]
	}
	return pitixItemStock{}
}

func decimalFromNumber(num json.Number) decimal.Decimal {
	if num.String() == "" {
		return decimal.Zero
	}
	if d, err := decimal.NewFromString(num.String()); err == nil {
		return d
	}
	return decimal.Zero
}

func parseTimeOrNow(value string) time.Time {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Now()
	}
	if t, err := time.Parse(time.RFC3339, value); err == nil {
		return t
	}
	return time.Now()
}

func mapSaleStatus(status string) models.SalesInvoiceStatus {
	switch strings.ToUpper(strings.TrimSpace(status)) {
	case "COMPLETED":
		return models.SalesInvoiceStatusConfirmed
	case "CANCELED", "CANCELLED":
		return models.SalesInvoiceStatusVoid
	default:
		return models.SalesInvoiceStatusDraft
	}
}

func findMapping(ctx context.Context, db *gorm.DB, businessId string, connectionId uint, entityType string, externalId string) (*models.IntegrationEntityMapping, error) {
	var mapping models.IntegrationEntityMapping
	err := db.WithContext(ctx).
		Where("business_id = ? AND connection_id = ? AND provider = ? AND entity_type = ? AND external_id = ?",
			businessId, connectionId, models.IntegrationProviderPitiX, entityType, externalId).
		Take(&mapping).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &mapping, nil
}

func createMapping(ctx context.Context, db *gorm.DB, businessId string, connectionId uint, entityType string, externalId string, internalId string) error {
	mapping := models.IntegrationEntityMapping{
		BusinessId:   businessId,
		ConnectionId: connectionId,
		Provider:     models.IntegrationProviderPitiX,
		EntityType:   entityType,
		ExternalId:   externalId,
		InternalId:   internalId,
	}
	return db.WithContext(ctx).Create(&mapping).Error
}

func touchMapping(ctx context.Context, db *gorm.DB, businessId string, connectionId uint, provider string, entityType string, externalId string, internalId string, updatedAt string) error {
	var metadata map[string]string
	if strings.TrimSpace(updatedAt) != "" {
		metadata = map[string]string{"updated_at": updatedAt}
	}
	metadataJSON, _ := json.Marshal(metadata)
	return db.WithContext(ctx).
		Model(&models.IntegrationEntityMapping{}).
		Where("business_id = ? AND connection_id = ? AND provider = ? AND entity_type = ? AND external_id = ?",
			businessId, connectionId, provider, entityType, externalId).
		Updates(map[string]interface{}{
			"internal_id":  internalId,
			"last_seen_at": time.Now(),
			"metadata_json": metadataJSON,
		}).Error
}

func createSyncError(ctx context.Context, db *gorm.DB, runId uint, businessId string, entityType string, externalId string, code string, message string, payload []byte, retryable bool) error {
	errRec := models.IntegrationSyncError{
		SyncRunId:  runId,
		BusinessId: businessId,
		EntityType: entityType,
		ExternalId: externalId,
		ErrorCode:  code,
		Message:    message,
		PayloadJSON: payload,
		Retryable:  retryable,
	}
	return db.WithContext(ctx).Create(&errRec).Error
}
