package models

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/mmdatafocus/books_backend/config"
	"github.com/mmdatafocus/books_backend/utils"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

type SalesInvoice struct {
	ID                            int                  `gorm:"primary_key" json:"id"`
	BusinessId                    string               `gorm:"index;not null" json:"business_id" binding:"required"`
	CustomerId                    int                  `gorm:"index;not null" json:"customer_id" binding:"required"`
	BranchId                      int                  `gorm:"index;not null" json:"branch_id" binding:"required"`
	SalesOrderId                  int                  `gorm:"index;default:null" json:"sales_order_id"`
	OrderNumber                   string               `gorm:"size:255;" json:"order_number"`
	SequenceNo                    decimal.Decimal      `gorm:"type:decimal(15);not null" json:"sequence_no"`
	InvoiceNumber                 string               `gorm:"size:255;not null" json:"invoice_number" binding:"required"`
	ReferenceNumber               string               `gorm:"size:255;default:null" json:"reference_number"`
	InvoiceDate                   time.Time            `gorm:"not null" json:"invoice_date" binding:"required"`
	InvoicePaymentTerms           PaymentTerms         `gorm:"type:enum('Net15', 'Net30', 'Net45', 'Net60', 'DueMonthEnd', 'DueNextMonthEnd', 'DueOnReceipt', 'Custom');not null" json:"invoice_payment_terms" binding:"required"`
	InvoicePaymentTermsCustomDays int                  `gorm:"default:0" json:"invoice_payment_terms_custom_days"`
	InvoiceDueDate                *time.Time           `gorm:"not null" json:"invoice_due_date" binding:"required"`
	SalesPersonId                 int                  `gorm:"default:null" json:"sales_person_id"`
	InvoiceSubject                string               `gorm:"size:255;default:null" json:"invoice_subject"`
	Notes                         string               `gorm:"type:text;default:null" json:"notes"`
	TermsAndConditions            string               `gorm:"type:text;default:null" json:"terms_and_conditions"`
	CurrencyId                    int                  `gorm:"not null" json:"currency_id" binding:"required"`
	ExchangeRate                  decimal.Decimal      `gorm:"type:decimal(20,4);default:0" json:"exchange_rate"`
	WarehouseId                   int                  `gorm:"not null" json:"warehouse_id" binding:"required"`
	InvoiceDiscount               decimal.Decimal      `gorm:"type:decimal(20,4);default:0" json:"invoice_discount"`
	InvoiceDiscountType           *DiscountType        `gorm:"type:enum('P', 'A');default:null" json:"invoice_discount_type"`
	InvoiceDiscountAmount         decimal.Decimal      `gorm:"type:decimal(20,4);default:0" json:"invoice_discount_amount"`
	ShippingCharges               decimal.Decimal      `gorm:"type:decimal(20,4);default:0" json:"shipping_charges"`
	AdjustmentAmount              decimal.Decimal      `gorm:"type:decimal(20,4);default:0" json:"adjustment_amount"`
	IsTaxInclusive                *bool                `gorm:"not null;default:false" json:"is_tax_inclusive"`
	InvoiceTaxId                  int                  `gorm:"default:null" json:"invoice_tax_id"`
	InvoiceTaxType                *TaxType             `gorm:"type:enum('I', 'G');default:null" json:"invoice_tax_type"`
	InvoiceTaxAmount              decimal.Decimal      `gorm:"type:decimal(20,4);default:0" json:"invoice_tax_amount"`
	CurrentStatus                 SalesInvoiceStatus   `gorm:"type:enum('Draft', 'Confirmed', 'Void', 'Partial Paid', 'Paid', 'Write Off');not null" json:"current_status" binding:"required"`
	Documents                     []*Document          `gorm:"polymorphic:Reference" json:"documents"`
	Details                       []SalesInvoiceDetail `gorm:"foreignKey:SalesInvoiceId" json:"details"`
	InvoiceSubtotal               decimal.Decimal      `gorm:"type:decimal(20,4);default:0" json:"invoice_subtotal"`
	InvoiceTotalDiscountAmount    decimal.Decimal      `gorm:"type:decimal(20,4);default:0" json:"invoice_total_discount_amount"`
	InvoiceTotalTaxAmount         decimal.Decimal      `gorm:"type:decimal(20,4);default:0" json:"invoice_total_tax_amount"`
	InvoiceTotalAmount            decimal.Decimal      `gorm:"type:decimal(20,4);default:0" json:"invoice_total_amount"`
	InvoiceTotalPaidAmount        decimal.Decimal      `gorm:"type:decimal(20,4);default:0" json:"invoice_total_paid_amount"`
	InvoiceTotalCreditUsedAmount  decimal.Decimal      `gorm:"type:decimal(20,4);default:0" json:"invoice_total_credit_used_amount"`
	InvoiceTotalAdvanceUsedAmount decimal.Decimal      `gorm:"type:decimal(20,4);default:0" json:"invoice_total_advance_used_amount"`
	InvoiceTotalWriteOffAmount    decimal.Decimal      `gorm:"type:decimal(20,4);default:0" json:"invoice_total_write_off_amount"`
	RemainingBalance              decimal.Decimal      `gorm:"type:decimal(20,4);default:0" json:"remaining_balance"`
	WriteOffDate                  *time.Time           `json:"write_off_date"`
	WriteOffReason                string               `gorm:"type:text;default:null" json:"write_off_reason"`
	CreatedAt                     time.Time            `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt                     time.Time            `gorm:"autoUpdateTime" json:"updated_at"`
}

type NewSalesInvoice struct {
	CustomerId                    int                     `json:"customer_id" binding:"required"`
	BranchId                      int                     `json:"branch_id" binding:"required"`
	SalesOrderId                  int                     `json:"sales_order_id"`
	OrderNumber                   string                  `json:"sales_order_number"`
	ReferenceNumber               string                  `json:"reference_number"`
	InvoiceDate                   time.Time               `json:"invoice_date" binding:"required"`
	InvoicePaymentTerms           PaymentTerms            `json:"invoice_payment_terms" binding:"required"`
	InvoicePaymentTermsCustomDays int                     `json:"invoice_payment_terms_custom_days"`
	SalesPersonId                 int                     `json:"sales_person_id"`
	InvoiceSubject                string                  `json:"invoice_subject"`
	Notes                         string                  `json:"notes"`
	TermsAndConditions            string                  `json:"terms_and_conditions"`
	CurrencyId                    int                     `json:"currency_id" binding:"required"`
	ExchangeRate                  decimal.Decimal         `json:"exchange_rate"`
	WarehouseId                   int                     `json:"warehouse_id" binding:"required"`
	InvoiceDiscount               decimal.Decimal         `json:"invoice_discount"`
	InvoiceDiscountType           *DiscountType           `json:"invoice_discount_type"`
	ShippingCharges               decimal.Decimal         `json:"shipping_charges"`
	AdjustmentAmount              decimal.Decimal         `json:"adjustment_amount"`
	IsTaxInclusive                *bool                   `json:"is_tax_inclusive" binding:"required"`
	InvoiceTaxId                  int                     `json:"invoice_tax_id"`
	InvoiceTaxType                *TaxType                `json:"invoice_tax_type"`
	CurrentStatus                 SalesInvoiceStatus      `json:"current_status" binding:"required"`
	Documents                     []*NewDocument          `json:"documents"`
	Details                       []NewSalesInvoiceDetail `json:"details"`
}

type SalesInvoiceDetail struct {
	ID                   int             `gorm:"primary_key" json:"id"`
	SalesInvoiceId       int             `gorm:"index;not null" json:"sales_invoice_id" binding:"required"`
	ProductId            int             `gorm:"index" json:"product_id"`
	ProductType          ProductType     `gorm:"type:enum('S','G','C','V','I');default:S" json:"product_type"`
	BatchNumber          string          `gorm:"size:100" json:"batch_number"`
	Name                 string          `gorm:"size:100" json:"name" binding:"required"`
	Description          string          `gorm:"size:255;default:null" json:"description"`
	DetailQty            decimal.Decimal `gorm:"type:decimal(20,4);default:0" json:"detail_qty" binding:"required"`
	DetailUnitRate       decimal.Decimal `gorm:"type:decimal(20,4);default:0" json:"detail_unit_rate" binding:"required"`
	DetailTaxId          int             `gorm:"default:null" json:"detail_tax_id"`
	DetailTaxType        *TaxType        `gorm:"type:enum('I', 'G');default:null" json:"detail_tax_type"`
	DetailAccountId      int             `gorm:"default:null" json:"detail_account_id"`
	DetailDiscount       decimal.Decimal `json:"detail_discount"`
	DetailDiscountType   *DiscountType   `gorm:"type:enum('P', 'A');default:null" json:"detail_discount_type"`
	DetailDiscountAmount decimal.Decimal `gorm:"type:decimal(20,4);default:0" json:"detail_discount_amount"`
	DetailTaxAmount      decimal.Decimal `gorm:"type:decimal(20,4);default:0" json:"detail_tax_amount"`
	DetailTotalAmount    decimal.Decimal `gorm:"type:decimal(20,4);default:0" json:"detail_total_amount"`
	Cogs                 decimal.Decimal `gorm:"type:decimal(20,4);default:0" json:"cogs"`
	StockId              int             `gorm:"default:0" json:"stock_id"`
	SalesOrderItemId     int             `gorm:"index" json:"sales_order_item_id"`
	CreatedAt            time.Time       `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt            time.Time       `gorm:"autoUpdateTime" json:"updated_at"`
}

type NewSalesInvoiceDetail struct {
	DetailId           int             `json:"detail_id"`
	ProductId          int             `json:"product_id"`
	ProductType        ProductType     `json:"product_type"`
	BatchNumber        string          `json:"batch_number"`
	Name               string          `json:"name" binding:"required"`
	Description        string          `json:"description"`
	DetailQty          decimal.Decimal `json:"detail_qty" binding:"required"`
	DetailUnitRate     decimal.Decimal `json:"detail_unit_rate" binding:"required"`
	DetailTaxId        int             `json:"detail_tax_id"`
	DetailTaxType      *TaxType        `json:"detail_tax_type"`
	DetailDiscount     decimal.Decimal `json:"detail_discount"`
	DetailDiscountType *DiscountType   `json:"detail_discount_type"`
	DetailAccountId    int             `json:"detail_account_id"`
	IsDeletedItem      *bool           `json:"is_deleted_item"`
	SalesOrderItemId   int             `json:"sales_order_item_id"`
}

type SalesInvoicesConnection struct {
	Edges               []*SalesInvoicesEdge `json:"edges"`
	PageInfo            *PageInfo            `json:"pageInfo"`
	InvoiceTotalSummary InvoiceTotalSummary  `json:"invoiceTotalSummary"`
}

type InvoiceTotalSummary struct {
	TotalOutstandingReceivable decimal.Decimal `json:"total_outstanding_receivable"`
	DueToday                   decimal.Decimal `json:"due_today"`
	DueWithin30Days            decimal.Decimal `json:"due_within_30_days"`

	TotalOverdue decimal.Decimal `json:"total_overdue"`
}

type SalesInvoicesEdge Edge[SalesInvoice]

// implements methods for pagination

// node
// returns decoded curosr string
func (si SalesInvoice) GetCursor() string {
	return si.CreatedAt.String()
}

// GetID method for SaleInvoice reference data
func (s *SalesInvoice) GetID() int {
	return s.ID
}

func (s *SalesInvoice) GetFieldValues(tx *gorm.DB) (*utils.DetailFieldValues, error) {
	return utils.FetchDetailFieldValues(tx, &SalesInvoiceDetail{}, "sales_invoice_id", s.ID)
}

func (input NewSalesInvoice) validate(ctx context.Context, businessId string, _ int) error {
	// exists customer
	if err := utils.ValidateResourceId[Customer](ctx, businessId, input.CustomerId); err != nil {
		return errors.New("customer not found")
	}
	// exists branch
	if err := utils.ValidateResourceId[Branch](ctx, businessId, input.BranchId); err != nil {
		return errors.New("branch not found")
	}
	// exists Currency
	if err := utils.ValidateResourceId[Currency](ctx, businessId, input.CurrencyId); err != nil {
		return errors.New("currency not found")
	}
	// exists wareshouse
	if input.WarehouseId > 0 {
		// exists warehouse
		if err := utils.ValidateResourceId[Warehouse](ctx, businessId, input.WarehouseId); err != nil {
			return errors.New("warehouse not found")
		}
	}
	// exists SalePerson
	if input.SalesPersonId > 0 {
		// exists SalesPerson
		if err := utils.ValidateResourceId[SalesPerson](ctx, businessId, input.SalesPersonId); err != nil {
			return errors.New("salesPerson not found")
		}
	}
	// exists SaleOrder
	if input.SalesOrderId > 0 {
		// exists SalesPerson
		if err := utils.ValidateResourceId[SalesOrder](ctx, businessId, input.SalesOrderId); err != nil {
			return errors.New("salesOrder not found")
		}
	}
	// validate invoiceDate
	if err := validateTransactionLock(ctx, input.InvoiceDate, businessId, SalesTransactionLock); err != nil {
		return err
	}
	// check for inventory value adjustment
	for _, detail := range input.Details {
		if err := ValidateValueAdjustment(ctx, businessId, input.InvoiceDate, detail.ProductType, detail.ProductId, &detail.BatchNumber); err != nil {
			return fmt.Errorf(err.Error(), detail.Name)
		}
	}

	return nil
}

// validate transaction lock
func (si SalesInvoice) CheckTransactionLock(ctx context.Context) error {

	if si.InvoiceNumber == "Customer Opening Balance" {
		return nil
	}
	if err := validateTransactionLock(ctx, si.InvoiceDate, si.BusinessId, SalesTransactionLock); err != nil {
		return err
	}
	// check for inventory value adjustment
	for _, detail := range si.Details {
		if err := ValidateValueAdjustment(ctx, si.BusinessId, si.InvoiceDate, detail.ProductType, detail.ProductId, &detail.BatchNumber); err != nil {
			return fmt.Errorf(err.Error(), detail.Name)
		}
	}
	return nil
}

func (item *SalesInvoiceDetail) CalculateSaleItemDiscountAndTax(ctx context.Context, isTaxInclusive bool) {

	db := config.GetDB()

	// calculate detail subtotal
	detailAmount := item.DetailQty.Mul(item.DetailUnitRate)
	// calculate discount amount
	var discountAmount decimal.Decimal

	if item.DetailDiscountType != nil {
		discountAmount = utils.CalculateDiscountAmount(detailAmount, item.DetailDiscount, string(*item.DetailDiscountType))
	}
	item.DetailDiscountAmount = discountAmount

	// Calculate subtotal amount
	item.DetailTotalAmount = item.DetailQty.Mul(item.DetailUnitRate).Sub(item.DetailDiscountAmount)

	var taxAmount decimal.Decimal
	if item.DetailTaxId > 0 {
		// Defensive: DetailTaxType can be nil (bad/missing client input). Default to Individual.
		taxType := TaxTypeIndividual
		if item.DetailTaxType != nil {
			taxType = *item.DetailTaxType
		}
		if taxType == TaxTypeGroup {
			taxAmount = utils.CalculateTaxAmount(ctx, db, item.DetailTaxId, true, item.DetailTotalAmount, isTaxInclusive)
		} else {
			taxAmount = utils.CalculateTaxAmount(ctx, db, item.DetailTaxId, false, item.DetailTotalAmount, isTaxInclusive)
		}
	} else {
		taxAmount = decimal.NewFromFloat(0)
	}

	item.DetailTaxAmount = taxAmount
}

func updateInvoiceItemDetailTotal(item *SalesInvoiceDetail, isTaxInclusive bool, orderSubtotal decimal.Decimal, totalExclusiveTaxAmount decimal.Decimal, totalDetailDiscountAmount decimal.Decimal, totalDetailTaxAmount decimal.Decimal) (decimal.Decimal, decimal.Decimal, decimal.Decimal, decimal.Decimal) {

	// var orderSubtotal, totalExclusiveTaxAmount, totalDetailDiscountAmount, totalDetailTaxAmount decimal.Decimal

	orderSubtotal = orderSubtotal.Add(item.DetailTotalAmount)
	totalDetailDiscountAmount = totalDetailDiscountAmount.Add(item.DetailDiscountAmount)
	totalDetailTaxAmount = totalDetailTaxAmount.Add(item.DetailTaxAmount)
	if isTaxInclusive {
		totalExclusiveTaxAmount = totalExclusiveTaxAmount.Add(decimal.NewFromFloat(0.0))
	} else {
		totalExclusiveTaxAmount = totalExclusiveTaxAmount.Add(item.DetailTaxAmount)
	}

	return orderSubtotal, totalDetailDiscountAmount, totalDetailTaxAmount, totalExclusiveTaxAmount
}

func CreateSalesInvoice(ctx context.Context, input *NewSalesInvoice) (*SalesInvoice, error) {
	db := config.GetDB()

	businessId, ok := utils.GetBusinessIdFromContext(ctx)
	if !ok || businessId == "" {
		return nil, errors.New("business id is required")
	}

	// Defensive: GraphQL input may omit optional booleans. Avoid nil deref panics.
	if input.IsTaxInclusive == nil {
		return nil, errors.New("is_tax_inclusive is required")
	}

	// IMPORTANT (correctness): if callers request "Confirmed" on create, we still create as Draft
	// and then transition Draft -> Confirmed inside the same DB transaction.
	// This ensures stock movements happen through the same status-transition path everywhere.
	requestedStatus := input.CurrentStatus

	// validate SalesInvoice
	if err := input.validate(ctx, businessId, 0); err != nil {
		return nil, err
	}

	var saleOrder SalesOrder
	saleOrderId := 0
	if len(input.OrderNumber) > 0 {
		// exists sale order
		err := db.Where("business_id = ? AND order_number = ?", businessId, input.OrderNumber).First(&saleOrder).Error
		if err != nil {
			return nil, errors.New("sale order not found")
		}
		saleOrderId = saleOrder.ID
	}

	// construct Images
	documents, err := mapNewDocuments(input.Documents, "sales_invoices", 0)
	if err != nil {
		return nil, err
	}

	tx := db.Begin()
	// IMPORTANT: always rollback on early-return or panic to avoid leaking DB locks
	// (leaked transactions are a common cause of MySQL 1205 lock wait timeouts).
	defer func() {
		if r := recover(); r != nil {
			_ = tx.Rollback().Error
			panic(r)
		}
	}()
	defer func() { _ = tx.Rollback().Error }()

	var invoiceItems []SalesInvoiceDetail
	var invoiceSubtotal,
		invoiceTotalAmount,
		totalExclusiveTaxAmount,
		totalDetailDiscountAmount,
		totalDetailTaxAmount decimal.Decimal

	var productIds, variantIds, taxIds, taxGroupIds, accountIds []int
	// Guardrail: validate stock against ledger-of-record (stock_histories) so we don't allow creating
	// an invoice from "summary stock" that isn't actually present in FIFO layers.
	//
	// This prevents: invoice created successfully but posting fails later (no invoice journal),
	// typically when transfer orders updated stock_summaries but their accounting workflow failed.
	// Reservation tracking within this invoice request so multiple lines can't over-consume the same stock.
	//
	// - For batch-specific lines, reserve per-batch (strict).
	// - For empty-batch lines, treat batches as fungible and reserve globally across all batches.
	reservedGlobal := make(map[string]decimal.Decimal)  // key: product_id-product_type
	reservedByBatch := make(map[string]decimal.Decimal) // key: product_id-product_type-batch
	for _, item := range input.Details {
		invoiceItem := SalesInvoiceDetail{
			ProductId:          item.ProductId,
			ProductType:        item.ProductType,
			BatchNumber:        item.BatchNumber,
			Name:               item.Name,
			Description:        item.Description,
			DetailQty:          item.DetailQty,
			DetailUnitRate:     item.DetailUnitRate,
			DetailTaxId:        item.DetailTaxId,
			DetailTaxType:      item.DetailTaxType,
			DetailDiscount:     item.DetailDiscount,
			DetailDiscountType: item.DetailDiscountType,
			DetailAccountId:    item.DetailAccountId,
			SalesOrderItemId:   item.SalesOrderItemId,
		}

		// Keep legacy behavior: if this line is for a non-inventory item, skip stock checks.
		// For inventory-tracked items, validate using ledger snapshots as-of invoice date.
		if item.ProductId > 0 && (item.ProductType == ProductTypeSingle || item.ProductType == ProductTypeVariant) {
			product, err := GetProductOrVariant(ctx, string(item.ProductType), item.ProductId)
			if err != nil {
				tx.Rollback()
				return nil, err
			}
			if product.GetInventoryAccountID() > 0 {
				globalKey := fmt.Sprintf("%d-%s", item.ProductId, string(item.ProductType))
				batchTrim := strings.TrimSpace(item.BatchNumber)
				batchKey := fmt.Sprintf("%d-%s-%s", item.ProductId, string(item.ProductType), batchTrim)
				asOf := MyDateString(input.InvoiceDate)
				pid := item.ProductId
				pt := item.ProductType
				var batchPtr *string
				if batchTrim != "" {
					// Strict batch: only count this batch.
					batchPtr = &batchTrim
				} else {
					// Empty batch: fungible across batches => snapshot must NOT filter by batch.
					batchPtr = nil
				}
				rows, err := InventorySnapshotByProductWarehouse(ctx, asOf, &input.WarehouseId, &pid, &pt, batchPtr)
				if err != nil {
					tx.Rollback()
					return nil, err
				}
				onHand := decimal.Zero
				for _, r := range rows {
					onHand = onHand.Add(r.StockOnHand)
				}
				var available decimal.Decimal
				if batchTrim == "" {
					already := reservedGlobal[globalKey]
					available = onHand.Sub(already)
					if available.LessThan(item.DetailQty) {
						tx.Rollback()
						return nil, fmt.Errorf("insufficient stock on hand for %s (available=%s, invoice_qty=%s)", strings.TrimSpace(item.Name), available.String(), item.DetailQty.String())
					}
					reservedGlobal[globalKey] = already.Add(item.DetailQty)
				} else {
					alreadyBatch := reservedByBatch[batchKey]
					available = onHand.Sub(alreadyBatch)
					if available.LessThan(item.DetailQty) {
						tx.Rollback()
						return nil, fmt.Errorf("insufficient stock on hand for %s (available=%s, invoice_qty=%s)", strings.TrimSpace(item.Name), available.String(), item.DetailQty.String())
					}
					reservedByBatch[batchKey] = alreadyBatch.Add(item.DetailQty)
					// Also reserve globally so empty-batch lines can't over-consume after batch-specific lines.
					reservedGlobal[globalKey] = reservedGlobal[globalKey].Add(item.DetailQty)
				}
			}
		} else {
			// Fallback for other product types / legacy behavior.
			if err := ValidateProductStock(tx, ctx, businessId, input.WarehouseId, item.BatchNumber, item.ProductType, item.ProductId, item.DetailQty); err != nil {
				tx.Rollback()
				return nil, err
			}
		}
		// Calculate tax and total amounts for the item
		invoiceItem.CalculateSaleItemDiscountAndTax(ctx, *input.IsTaxInclusive)

		invoiceSubtotal = invoiceSubtotal.Add(invoiceItem.DetailTotalAmount)
		totalDetailDiscountAmount = totalDetailDiscountAmount.Add(invoiceItem.DetailDiscountAmount)
		totalDetailTaxAmount = totalDetailTaxAmount.Add(invoiceItem.DetailTaxAmount)

		if input.IsTaxInclusive != nil && *input.IsTaxInclusive {
			totalExclusiveTaxAmount = totalExclusiveTaxAmount.Add(decimal.NewFromFloat(0.0))
		} else {
			totalExclusiveTaxAmount = totalExclusiveTaxAmount.Add(invoiceItem.DetailTaxAmount)
		}
		invoiceItem.Cogs = decimal.NewFromInt(0)

		// Add the item to the PurchaseOrder
		invoiceItems = append(invoiceItems, invoiceItem)

		// update invoiced qty in sale order detail (when invoice will be confirmed)
		if saleOrderId > 0 && requestedStatus == SalesInvoiceStatusConfirmed {
			if err := UpdateSaleOrderDetailInvoicedQty(tx, ctx, saleOrderId, invoiceItem, "create", decimal.NewFromFloat(0), 0); err != nil {
				tx.Rollback()
				return nil, err
			}
		}
		if item.ProductId > 0 {
			if item.ProductType == ProductTypeSingle {
				productIds = append(productIds, item.ProductId)
			} else if item.ProductType == ProductTypeVariant {
				variantIds = append(variantIds, item.ProductId)
			}
		}
		if item.DetailTaxId > 0 {
			taxType := TaxTypeIndividual
			if item.DetailTaxType != nil {
				taxType = *item.DetailTaxType
			}
			if taxType == TaxTypeIndividual {
				taxIds = append(taxIds, item.DetailTaxId)
			} else {
				taxGroupIds = append(taxGroupIds, item.DetailTaxId)
			}
		}
		if item.DetailAccountId > 0 {
			accountIds = append(accountIds, item.DetailAccountId)
		}
	}

	// validate products belong to the same business
	businessFilter := utils.Filter{Cond: "business_id = ?", Values: []interface{}{businessId}}
	if err := utils.MassValidateResourceIds(ctx, []utils.ValidationRule[int]{
		{Model: Product{}, Ids: productIds, Message: "products Not found", Filter: businessFilter},
		{Model: ProductVariant{}, Ids: variantIds, Message: "variants Not found", Filter: businessFilter},
		{Model: Tax{}, Ids: taxIds, Message: "tax not found", Filter: businessFilter},
		{Model: TaxGroup{}, Ids: taxGroupIds, Message: "tax not found", Filter: businessFilter},
		{Model: Account{}, Ids: accountIds, Message: "item account not found", Filter: businessFilter},
	}); err != nil {
		tx.Rollback()
		return nil, err
	}

	// calculate order discount
	var invoiceDiscountAmount decimal.Decimal

	if input.InvoiceDiscountType != nil {
		invoiceDiscountAmount = utils.CalculateDiscountAmount(invoiceSubtotal, input.InvoiceDiscount, string(*input.InvoiceDiscountType))
	}

	// invoiceSubtotal = invoiceSubtotal.Sub(invoiceDiscountAmount)

	// calculate order tax amount (always exclusive)
	var invoiceTaxAmount decimal.Decimal
	if input.InvoiceTaxId > 0 {
		if input.InvoiceTaxType == nil {
			tx.Rollback()
			return nil, errors.New("invoice_tax_type is required when invoice_tax_id > 0")
		}
		if *input.InvoiceTaxType == TaxTypeGroup {
			invoiceTaxAmount = utils.CalculateTaxAmount(ctx, db, input.InvoiceTaxId, true, invoiceSubtotal, false)
		} else {
			invoiceTaxAmount = utils.CalculateTaxAmount(ctx, db, input.InvoiceTaxId, false, invoiceSubtotal, false)
		}
	} else {
		invoiceTaxAmount = decimal.NewFromFloat(0)
	}

	// Sum (order discount + total detail discount)
	totalInvoiceDiscountAmount := invoiceDiscountAmount.Add(totalDetailDiscountAmount)
	// Sum (Invoice tax amount + total detail tax amount)
	totalInvoiceTaxAmount := invoiceTaxAmount.Add(totalDetailTaxAmount)

	invoiceTotalAmount = invoiceSubtotal.Add(invoiceTaxAmount).Add(totalExclusiveTaxAmount).Add(input.AdjustmentAmount).Add(input.ShippingCharges).Sub(invoiceDiscountAmount)

	// store saleInvoice
	saleInvoice := SalesInvoice{
		BusinessId:                    businessId,
		CustomerId:                    input.CustomerId,
		BranchId:                      input.BranchId,
		SalesOrderId:                  saleOrderId,
		OrderNumber:                   input.OrderNumber,
		ReferenceNumber:               input.ReferenceNumber,
		InvoiceDate:                   input.InvoiceDate,
		InvoiceDueDate:                calculateDueDate(input.InvoiceDate, input.InvoicePaymentTerms, input.InvoicePaymentTermsCustomDays),
		InvoicePaymentTerms:           input.InvoicePaymentTerms,
		InvoicePaymentTermsCustomDays: input.InvoicePaymentTermsCustomDays,
		SalesPersonId:                 input.SalesPersonId,
		Notes:                         input.Notes,
		TermsAndConditions:            input.TermsAndConditions,
		CurrencyId:                    input.CurrencyId,
		ExchangeRate:                  input.ExchangeRate,
		WarehouseId:                   input.WarehouseId,
		InvoiceDiscount:               input.InvoiceDiscount,
		InvoiceDiscountType:           input.InvoiceDiscountType,
		InvoiceDiscountAmount:         invoiceDiscountAmount,
		ShippingCharges:               input.ShippingCharges,
		AdjustmentAmount:              input.AdjustmentAmount,
		IsTaxInclusive:                input.IsTaxInclusive,
		InvoiceTaxId:                  input.InvoiceTaxId,
		InvoiceTaxType:                input.InvoiceTaxType,
		InvoiceTaxAmount:              invoiceTaxAmount,
		CurrentStatus:                 SalesInvoiceStatusDraft,
		Documents:                     documents,
		Details:                       invoiceItems,
		InvoiceTotalDiscountAmount:    totalInvoiceDiscountAmount,
		InvoiceTotalTaxAmount:         totalInvoiceTaxAmount,
		InvoiceSubtotal:               invoiceSubtotal,
		InvoiceTotalAmount:            invoiceTotalAmount,
		RemainingBalance:              invoiceTotalAmount,
	}

	seqNo, err := utils.GetSequence[SalesInvoice](ctx, businessId)
	if err != nil {
		tx.Rollback()
		return nil, err
	}
	prefix, err := getTransactionPrefix(ctx, input.BranchId, "Invoice")
	if err != nil {
		tx.Rollback()
		return nil, err
	}
	saleInvoice.SequenceNo = decimal.NewFromInt(seqNo)
	saleInvoice.InvoiceNumber = prefix + fmt.Sprint(seqNo)

	err = tx.WithContext(ctx).Create(&saleInvoice).Error
	if err != nil {
		tx.Rollback()
		return nil, err
	}

	// Reload with Details so stock side-effects can access them.
	if err := tx.WithContext(ctx).Preload("Details").First(&saleInvoice, saleInvoice.ID).Error; err != nil {
		tx.Rollback()
		return nil, err
	}

	// If requested "Confirmed", apply the status transition deterministically (Draft -> Confirmed).
	if requestedStatus == SalesInvoiceStatusConfirmed {
		// Persist new status first.
		if err := tx.WithContext(ctx).Model(&saleInvoice).Update("CurrentStatus", SalesInvoiceStatusConfirmed).Error; err != nil {
			tx.Rollback()
			return nil, err
		}
		// Update in-memory model so downstream uses the correct status.
		saleInvoice.CurrentStatus = SalesInvoiceStatusConfirmed

		// Apply stock side-effects deterministically.
		if config.UseStockCommandsFor("SALES_INVOICE") {
			if err := ApplySalesInvoiceStockForStatusTransition(tx.WithContext(ctx), &saleInvoice, SalesInvoiceStatusDraft); err != nil {
				tx.Rollback()
				return nil, err
			}
		} else {
			// Legacy behavior (will be removed): use model-hook-style helper.
			if err := saleInvoice.AfterUpdateCurrentStatus(tx.WithContext(ctx), string(SalesInvoiceStatusDraft)); err != nil {
				tx.Rollback()
				return nil, err
			}
		}

		// Write outbox record (publishing happens after commit via dispatcher).
		if err := PublishToAccounting(ctx, tx, businessId, saleInvoice.InvoiceDate, saleInvoice.ID, AccountReferenceTypeInvoice, saleInvoice, nil, PubSubMessageActionCreate); err != nil {
			tx.Rollback()
			return nil, err
		}
	}

	// Commit the transaction
	if err := tx.Commit().Error; err != nil {
		return nil, err
	}

	return &saleInvoice, nil
}

func UpdateSalesInvoice(ctx context.Context, invoiceId int, updatedInvoice *NewSalesInvoice) (*SalesInvoice, error) {

	businessId, ok := utils.GetBusinessIdFromContext(ctx)
	if !ok || businessId == "" {
		return nil, errors.New("business id is required")
	}

	// validate SalesInvoice
	if err := updatedInvoice.validate(ctx, businessId, invoiceId); err != nil {
		return nil, err
	}
	// Fetch the existing
	oldInvoice, err := utils.FetchModelForChange[SalesInvoice](ctx, businessId, invoiceId, "Details")
	if err != nil {
		return nil, err
	}
	// don't allow updating (partial) paid invoice
	if oldInvoice.CurrentStatus == SalesInvoiceStatusPartialPaid || oldInvoice.CurrentStatus == SalesInvoiceStatusPaid {
		return nil, errors.New("cannot update paid/partial paid invoice")
	}
	// Fintech integrity guardrail (behind flag): inventory-affecting docs are immutable after confirm.
	if config.StrictInventoryDocImmutability() && oldInvoice.CurrentStatus == SalesInvoiceStatusConfirmed {
		return nil, errors.New("cannot edit a confirmed invoice; void and recreate to preserve inventory/COGS integrity")
	}
	// deep copy of oldInvoice.Details
	existingInvoiceDetails := append([]SalesInvoiceDetail(nil), oldInvoice.Details...)
	var existingInvoice SalesInvoice = *oldInvoice

	oldStatus := existingInvoice.CurrentStatus
	// validate current status
	switch updatedInvoice.CurrentStatus {
	case SalesInvoiceStatusDraft:
		if oldStatus != SalesInvoiceStatusDraft {
			return nil, errors.New("invalid status")
		}
	case SalesInvoiceStatusConfirmed:
		break
	default:
		return nil, errors.New("invalid status")
	}

	var oldInvoiceDate *time.Time
	// nil if invoice dates are the same, old invoice date if not
	if !existingInvoice.InvoiceDate.Equal(updatedInvoice.InvoiceDate) {
		v := existingInvoice.InvoiceDate
		oldInvoiceDate = &v
	}

	var oldWarehouseId *int
	// nil if warehouse ids are the same, old warehouse id if not
	if existingInvoice.WarehouseId != updatedInvoice.WarehouseId {
		v := existingInvoice.WarehouseId
		oldWarehouseId = &v
	}

	db := config.GetDB()

	// Update the fields of the existing purchase order with the provided updated details
	existingInvoice.CustomerId = updatedInvoice.CustomerId
	existingInvoice.BranchId = updatedInvoice.BranchId
	existingInvoice.OrderNumber = updatedInvoice.OrderNumber
	existingInvoice.ReferenceNumber = updatedInvoice.ReferenceNumber
	existingInvoice.InvoiceDate = updatedInvoice.InvoiceDate
	existingInvoice.InvoiceDueDate = calculateDueDate(updatedInvoice.InvoiceDate, updatedInvoice.InvoicePaymentTerms, updatedInvoice.InvoicePaymentTermsCustomDays)
	existingInvoice.InvoicePaymentTerms = updatedInvoice.InvoicePaymentTerms
	existingInvoice.SalesPersonId = updatedInvoice.SalesPersonId
	existingInvoice.Notes = updatedInvoice.Notes
	existingInvoice.TermsAndConditions = updatedInvoice.TermsAndConditions
	existingInvoice.CurrencyId = updatedInvoice.CurrencyId
	existingInvoice.ExchangeRate = updatedInvoice.ExchangeRate
	existingInvoice.WarehouseId = updatedInvoice.WarehouseId
	existingInvoice.InvoiceDiscount = updatedInvoice.InvoiceDiscount
	existingInvoice.InvoiceDiscountType = updatedInvoice.InvoiceDiscountType
	existingInvoice.ShippingCharges = updatedInvoice.ShippingCharges
	existingInvoice.AdjustmentAmount = updatedInvoice.AdjustmentAmount
	existingInvoice.IsTaxInclusive = updatedInvoice.IsTaxInclusive
	existingInvoice.InvoiceTaxId = updatedInvoice.InvoiceTaxId
	existingInvoice.InvoiceTaxType = updatedInvoice.InvoiceTaxType
	existingInvoice.CurrentStatus = updatedInvoice.CurrentStatus

	var invoiceSubtotal,
		invoiceTotalAmount,
		totalExclusiveTaxAmount,
		totalDetailDiscountAmount,
		totalDetailTaxAmount decimal.Decimal

	// Iterate through the updated items

	tx := db.Begin()
	for _, updatedItem := range updatedInvoice.Details {
		var existingItem *SalesInvoiceDetail

		// Check if the item already exists in the purchase order
		for _, item := range existingInvoiceDetails {
			if item.ID == updatedItem.DetailId {
				existingItem = &item
				break
			}
		}

		// If the item doesn't exist, add it to the purchase order
		if existingItem == nil {
			// CREATE new item
			fmt.Println("is not existing- ")
			newItem := SalesInvoiceDetail{
				ProductId:          updatedItem.ProductId,
				ProductType:        updatedItem.ProductType,
				BatchNumber:        updatedItem.BatchNumber,
				Name:               updatedItem.Name,
				Description:        updatedItem.Description,
				DetailQty:          updatedItem.DetailQty,
				DetailUnitRate:     updatedItem.DetailUnitRate,
				DetailTaxId:        updatedItem.DetailTaxId,
				DetailTaxType:      updatedItem.DetailTaxType,
				DetailDiscount:     updatedItem.DetailDiscount,
				DetailDiscountType: updatedItem.DetailDiscountType,
				DetailAccountId:    updatedItem.DetailAccountId,
				SalesOrderItemId:   updatedItem.SalesOrderItemId,
			}

			// check if to-be-added product has enough stock
			if err := ValidateProductStock(tx, ctx, businessId, updatedInvoice.WarehouseId, updatedItem.BatchNumber, updatedItem.ProductType, updatedItem.ProductId, updatedItem.DetailQty); err != nil {
				tx.Rollback()
				return nil, err
			}
			// if updatedItem.ProductType != ProductTypeInput {
			// 	currentStock, err := GetProductStock(ctx, businessId, updatedItem.ProductType, updatedItem.ProductId)
			// 	if err != nil {
			// 		tx.Rollback()
			// 		return nil, err
			// 	}
			// 	if currentStock.IsPositive() && currentStock.LessThan(updatedItem.DetailQty) {
			// 		tx.Rollback()
			// 		return nil, errors.New("stock on hand is less than input qty")
			// 	}
			// }

			// Calculate tax and total amounts for the item
			newItem.CalculateSaleItemDiscountAndTax(ctx, *updatedInvoice.IsTaxInclusive)
			invoiceSubtotal, totalDetailDiscountAmount, totalDetailTaxAmount, totalExclusiveTaxAmount = updateInvoiceItemDetailTotal(&newItem, *updatedInvoice.IsTaxInclusive, invoiceSubtotal, totalExclusiveTaxAmount, totalDetailDiscountAmount, totalDetailTaxAmount)
			newItem.Cogs = decimal.NewFromInt(0)
			existingInvoice.Details = append(existingInvoice.Details, newItem)
			if existingInvoice.CurrentStatus == SalesInvoiceStatusConfirmed {
				inventoryAccId := 0
				if newItem.ProductId > 0 {
					product, err := GetProductOrVariant(ctx, string(newItem.ProductType), newItem.ProductId)
					if err != nil {
						tx.Rollback()
						return nil, err
					}

					// updating stock summary
					if product.GetInventoryAccountID() > 0 {
						// add to stockSummary
						if err := UpdateStockSummarySaleQty(tx, businessId, existingInvoice.WarehouseId, newItem.ProductId, string(newItem.ProductType), newItem.BatchNumber, newItem.DetailQty, existingInvoice.InvoiceDate); err != nil {
							tx.Rollback()
							return nil, err
						}
						inventoryAccId = product.GetInventoryAccountID()
					}

				}

				if existingInvoice.SalesOrderId > 0 {

					if err := UpdateSaleOrderDetailInvoicedQty(tx, ctx, existingInvoice.SalesOrderId, newItem, "create", decimal.NewFromFloat(0), inventoryAccId); err != nil {
						tx.Rollback()
						return nil, err
					}

					_, err = ChangeSaleOrderCurrentStatus(tx.WithContext(ctx), ctx, businessId, existingInvoice.SalesOrderId)
					if err != nil {
						tx.Rollback()
						return nil, err
					}
				}
			}

		} else { // the item exists already

			var oldDetailQty *decimal.Decimal
			// nil if detail qty does not change
			if !existingItem.DetailQty.Equal(updatedItem.DetailQty) {
				v := existingItem.DetailQty
				oldDetailQty = &v
			}

			var oldBatchNumber *string
			// nil if batch number does not change
			if existingItem.BatchNumber != updatedItem.BatchNumber {
				v := existingItem.BatchNumber
				oldBatchNumber = &v
			}
			//2d what if product type changed?

			if updatedItem.IsDeletedItem != nil && *updatedItem.IsDeletedItem {
				// DELETE existing item
				// Find the index of the item to delete
				for i, item := range existingInvoice.Details {
					if item.ID == updatedItem.DetailId {
						if oldStatus == SalesInvoiceStatusConfirmed {
							inventoryAccId := 0
							if item.ProductId > 0 {
								product, err := GetProductOrVariant(ctx, string(item.ProductType), item.ProductId)
								if err != nil {
									tx.Rollback()
									return nil, err
								}
								if product.GetInventoryAccountID() > 0 {
									// update stock summary

									if err := UpdateStockSummarySaleQty(tx,
										existingInvoice.BusinessId,
										utils.DereferencePtr(oldWarehouseId, existingInvoice.WarehouseId),
										item.ProductId,
										string(item.ProductType),
										utils.DereferencePtr(oldBatchNumber, existingItem.BatchNumber),
										utils.DereferencePtr(oldDetailQty, existingItem.DetailQty).Neg(),
										utils.DereferencePtr(oldInvoiceDate, existingInvoice.InvoiceDate),
									); err != nil {
										tx.Rollback()
										return nil, err
									}
									inventoryAccId = product.GetInventoryAccountID()
								}
							}

							if existingInvoice.SalesOrderId > 0 {
								if err := UpdateSaleOrderDetailInvoicedQty(tx, ctx, existingInvoice.SalesOrderId, item, "delete", decimal.NewFromFloat(0), inventoryAccId); err != nil {
									tx.Rollback()
									return nil, err
								}

								_, err = ChangeSaleOrderCurrentStatus(tx.WithContext(ctx), ctx, businessId, existingInvoice.SalesOrderId)
								if err != nil {
									tx.Rollback()
									return nil, err
								}
							}
						}
						// Delete the item from the database
						if err := tx.WithContext(ctx).Delete(&existingInvoice.Details[i]).Error; err != nil {
							tx.Rollback()
							return nil, err
						}
						// Remove the item from the slice
						existingInvoice.Details = append(existingInvoice.Details[:i], existingInvoice.Details[i+1:]...)
						break // Exit the loop after deleting the item
					}
				}
			} else {
				// UPDATE existing item
				var oldQty decimal.Decimal = existingItem.DetailQty

				existingItem.BatchNumber = updatedItem.BatchNumber
				existingItem.Name = updatedItem.Name
				existingItem.Description = updatedItem.Description
				existingItem.DetailQty = updatedItem.DetailQty
				existingItem.DetailUnitRate = updatedItem.DetailUnitRate
				existingItem.DetailTaxId = updatedItem.DetailTaxId
				existingItem.DetailTaxType = updatedItem.DetailTaxType
				existingItem.DetailDiscount = updatedItem.DetailDiscount
				existingItem.DetailDiscountType = updatedItem.DetailDiscountType
				existingItem.DetailAccountId = updatedItem.DetailAccountId
				existingItem.SalesOrderItemId = updatedItem.SalesOrderItemId

				// Calculate tax and total amounts for the item
				existingItem.CalculateSaleItemDiscountAndTax(ctx, *updatedInvoice.IsTaxInclusive)
				invoiceSubtotal, totalDetailDiscountAmount, totalDetailTaxAmount, totalExclusiveTaxAmount = updateInvoiceItemDetailTotal(existingItem, *updatedInvoice.IsTaxInclusive, invoiceSubtotal, totalExclusiveTaxAmount, totalDetailDiscountAmount, totalDetailTaxAmount)
				// existingInvoice.Details = append(existingInvoice.Details, *existingItem)
				inventoryAccId := 0
				if existingItem.ProductId > 0 {
					product, err := GetProductOrVariant(ctx, string(existingItem.ProductType), existingItem.ProductId)
					if err != nil {
						tx.Rollback()
						return nil, err
					}
					if product.GetInventoryAccountID() > 0 {
						inventoryAccId = product.GetInventoryAccountID()
						// newly confirm
						if existingInvoice.CurrentStatus == SalesInvoiceStatusConfirmed && oldStatus == SalesInvoiceStatusDraft {
							// check for stock availablity
							if err := ValidateProductStock(tx, ctx,
								businessId,
								existingInvoice.WarehouseId,
								existingItem.BatchNumber,
								existingItem.ProductType,
								existingItem.ProductId,
								existingItem.DetailQty); err != nil {
								tx.Rollback()
								return nil, err
							}
							if err := UpdateStockSummarySaleQty(tx,
								businessId,
								existingInvoice.WarehouseId,
								existingItem.ProductId,
								string(existingItem.ProductType),
								existingItem.BatchNumber,
								existingItem.DetailQty,
								existingInvoice.InvoiceDate); err != nil {
								tx.Rollback()
								return nil, err
							}
							// 	// newly draft
							// } else if existingInvoice.CurrentStatus == SalesInvoiceStatusDraft && oldStatus == SalesInvoiceStatusConfirmed {
							// 	if err := UpdateStockSummarySaleQty(tx,
							// 		businessId,
							// 		utils.DereferencePtr(oldWarehouseId, existingInvoice.WarehouseId),
							// 		existingItem.ProductId,
							// 		string(existingItem.ProductType),
							// 		utils.DereferencePtr(oldBatchNumber, existingItem.BatchNumber),
							// 		utils.DereferencePtr(oldDetailQty, existingItem.DetailQty).Neg(),
							// 		utils.DereferencePtr(oldInvoiceDate, existingInvoice.InvoiceDate)); err != nil {
							// 		tx.Rollback()
							// 		return nil, err
							// 	}
							// update confirmed
						} else if existingInvoice.CurrentStatus == SalesInvoiceStatusConfirmed && oldStatus == SalesInvoiceStatusConfirmed {

							// updateStockSummary
							// if invoice dates differ
							if oldInvoiceDate != nil || oldWarehouseId != nil || oldBatchNumber != nil {

								// get actual added qty for validation only
								addedQty := existingItem.DetailQty
								if oldWarehouseId == nil && oldBatchNumber == nil {
									// if belongs to the same stockSummary
									addedQty = addedQty.Sub(utils.DereferencePtr(oldDetailQty, existingItem.DetailQty))
								}
								// // check for stock availablity
								// if err := ValidateProductStock(tx, ctx, businessId, existingInvoice.WarehouseId, existingItem.BatchNumber, existingItem.ProductType, existingItem.ProductId, addedQty); err != nil {
								// 	tx.Rollback()
								// 	return nil, err
								// }

								// lock business
								err := utils.BusinessLock(ctx, businessId, "stockLock", "modelHoodsStockSummary.go", "SaleInvoiceDetailBeforeUpdate")
								if err != nil {
									tx.Rollback()
									return nil, err
								}
								// remove old stock old date old warehouse
								if err := UpdateStockSummarySaleQty(tx, businessId,
									utils.DereferencePtr(oldWarehouseId, existingInvoice.WarehouseId),
									existingItem.ProductId,
									string(existingItem.ProductType),
									utils.DereferencePtr(oldBatchNumber, existingItem.BatchNumber),
									utils.DereferencePtr(oldDetailQty, existingItem.DetailQty).Neg(),
									utils.DereferencePtr(oldInvoiceDate, existingInvoice.InvoiceDate),
								); err != nil {
									tx.Rollback()
									return nil, err
								}
								// check for stock availablity
								if err := ValidateProductStock(tx, ctx, businessId, existingInvoice.WarehouseId, existingItem.BatchNumber, existingItem.ProductType, existingItem.ProductId, addedQty); err != nil {
									tx.Rollback()
									return nil, err
								}
								// add new stock
								if err := UpdateStockSummarySaleQty(tx, businessId,
									existingInvoice.WarehouseId,
									existingItem.ProductId,
									string(existingItem.ProductType),
									existingItem.BatchNumber,
									existingItem.DetailQty,
									existingInvoice.InvoiceDate); err != nil {
									tx.Rollback()
									return nil, err
								}

							} else if oldDetailQty != nil {
								// same warehouse, same date, same batch number but qty differs

								addedQty := existingItem.DetailQty.Sub(*oldDetailQty)
								if addedQty.IsPositive() {
									if err := ValidateProductStock(tx, ctx, businessId, existingInvoice.WarehouseId, existingItem.BatchNumber, existingItem.ProductType, existingItem.ProductId, addedQty); err != nil {
										tx.Rollback()
										return nil, err
									}
								}
								// lock business
								err := utils.BusinessLock(ctx, businessId, "stockLock", "modelHoodsStockSummary.go", "SaleInvoiceDetailBeforeUpdate")
								if err != nil {
									tx.Rollback()
									return nil, err
								}
								if err := UpdateStockSummarySaleQty(tx, businessId,
									existingInvoice.WarehouseId,
									existingItem.ProductId,
									string(existingItem.ProductType),
									existingItem.BatchNumber,
									addedQty,
									existingInvoice.InvoiceDate); err != nil {
									tx.Rollback()
									return nil, err
								}
							}
						}
					}
				}

				if err := tx.WithContext(ctx).Save(&existingItem).Error; err != nil {
					tx.Rollback()
					return nil, err
				}
				// process salesOrder
				if existingInvoice.SalesOrderId > 0 && existingInvoice.CurrentStatus == SalesInvoiceStatusConfirmed {

					if !existingItem.DetailQty.Equal(oldQty) || (oldStatus == SalesInvoiceStatusDraft && existingInvoice.CurrentStatus == SalesInvoiceStatusConfirmed) {

						if !existingItem.DetailQty.Equal(oldQty) && oldStatus == SalesInvoiceStatusConfirmed {
							if err := UpdateSaleOrderDetailInvoicedQty(tx, ctx, existingInvoice.SalesOrderId, *existingItem, "update", oldQty, inventoryAccId); err != nil {
								tx.Rollback()
								return nil, err
							}
						}

						if oldStatus == SalesInvoiceStatusDraft && existingInvoice.CurrentStatus == SalesInvoiceStatusConfirmed {
							if err := UpdateSaleOrderDetailInvoicedQty(tx, ctx, existingInvoice.SalesOrderId, *existingItem, "create", decimal.NewFromFloat(0), inventoryAccId); err != nil {
								tx.Rollback()
								return nil, err
							}
						}

						_, err = ChangeSaleOrderCurrentStatus(tx.WithContext(ctx), ctx, businessId, existingInvoice.SalesOrderId)
						if err != nil {
							tx.Rollback()
							return nil, err
						}
					}
				}
			}

		}
	}
	// calculate order discount
	var invoiceDiscountAmount decimal.Decimal

	if updatedInvoice.InvoiceDiscountType != nil {
		invoiceDiscountAmount = utils.CalculateDiscountAmount(invoiceSubtotal, updatedInvoice.InvoiceDiscount, string(*updatedInvoice.InvoiceDiscountType))
	}

	existingInvoice.InvoiceDiscountAmount = invoiceDiscountAmount

	// invoiceSubtotal = invoiceSubtotal.Sub(invoiceDiscountAmount)
	existingInvoice.InvoiceSubtotal = invoiceSubtotal

	// calculate order tax amount (always exclusive)
	var orderTaxAmount decimal.Decimal
	if updatedInvoice.InvoiceTaxId > 0 {
		if *updatedInvoice.InvoiceTaxType == TaxTypeGroup {
			orderTaxAmount = utils.CalculateTaxAmount(ctx, db, updatedInvoice.InvoiceTaxId, true, invoiceSubtotal, false)
		} else {
			orderTaxAmount = utils.CalculateTaxAmount(ctx, db, updatedInvoice.InvoiceTaxId, false, invoiceSubtotal, false)
		}
	} else {
		orderTaxAmount = decimal.NewFromFloat(0)
	}

	existingInvoice.InvoiceTaxAmount = orderTaxAmount

	// Sum (order discount + total detail discount)
	totalInvoiceDiscountAmount := invoiceDiscountAmount.Add(totalDetailDiscountAmount)
	existingInvoice.InvoiceTotalDiscountAmount = totalInvoiceDiscountAmount

	// Sum (Invoice tax amount + total detail tax amount)
	totalInvoiceTaxAmount := orderTaxAmount.Add(totalDetailTaxAmount)
	existingInvoice.InvoiceTotalTaxAmount = totalInvoiceTaxAmount
	// Sum Grand total amount (subtotal+ exclusive tax + adj amount)
	invoiceTotalAmount = invoiceSubtotal.Add(orderTaxAmount).Add(totalExclusiveTaxAmount).Add(updatedInvoice.AdjustmentAmount).Add(updatedInvoice.ShippingCharges).Sub(invoiceDiscountAmount)

	existingInvoice.InvoiceTotalAmount = invoiceTotalAmount
	existingInvoice.RemainingBalance = invoiceTotalAmount

	// Save the updated purchase order to the database
	if err := tx.WithContext(ctx).Save(&existingInvoice).Error; err != nil {
		tx.Rollback()
		return nil, err
	}

	// Refresh the existingInvoice to get the latest details
	if err := tx.WithContext(ctx).Preload("Details").First(&existingInvoice, invoiceId).Error; err != nil {
		tx.Rollback()
		return nil, err
	}

	// if err := existingInvoice.AfterUpdateCurrentStatus(tx.WithContext(ctx), string(oldStatus)); err != nil {
	// 	tx.Rollback()
	// 	return nil, err
	// }

	if oldStatus == SalesInvoiceStatusDraft && existingInvoice.CurrentStatus == SalesInvoiceStatusConfirmed {
		err := PublishToAccounting(ctx, tx, businessId, existingInvoice.InvoiceDate, existingInvoice.ID, AccountReferenceTypeInvoice, existingInvoice, nil, PubSubMessageActionCreate)
		if err != nil {
			tx.Rollback()
			return nil, err
		}
	} else if oldStatus == SalesInvoiceStatusConfirmed && existingInvoice.CurrentStatus == SalesInvoiceStatusConfirmed {
		err := PublishToAccounting(ctx, tx, businessId, existingInvoice.InvoiceDate, existingInvoice.ID, AccountReferenceTypeInvoice, existingInvoice, oldInvoice, PubSubMessageActionUpdate)
		if err != nil {
			tx.Rollback()
			return nil, err
		}
	}

	documents, err := upsertDocuments(ctx, tx, updatedInvoice.Documents, "sales_invoices", invoiceId)
	if err != nil {
		tx.Rollback()
		return nil, err
	}
	existingInvoice.Documents = documents

	if err := tx.Commit().Error; err != nil {
		return nil, err
	}

	return &existingInvoice, nil
}

func DeleteSalesInvoice(ctx context.Context, id int) (*SalesInvoice, error) {
	businessId, ok := utils.GetBusinessIdFromContext(ctx)
	if !ok || businessId == "" {
		return nil, errors.New("business id is required")
	}

	result, err := utils.FetchModelForChange[SalesInvoice](ctx, businessId, id, "Details", "Documents")
	if err != nil {
		return nil, err
	}

	db := config.GetDB()
	tx := db.Begin()
	// reduced received qty from stock summary if sale order is confirmed

	for _, item := range result.Details {
		if result.CurrentStatus == SalesInvoiceStatusConfirmed {
			inventoryAccId := 0
			if item.ProductId > 0 {
				product, err := GetProductOrVariant(ctx, string(item.ProductType), item.ProductId)
				if err != nil {
					tx.Rollback()
					return nil, err
				}
				if product.GetInventoryAccountID() > 0 {

					if err := UpdateStockSummarySaleQty(tx, result.BusinessId, result.WarehouseId, item.ProductId, string(item.ProductType), item.BatchNumber, item.DetailQty.Neg(), result.InvoiceDate); err != nil {
						tx.Rollback()
						return nil, err
					}

					inventoryAccId = product.GetInventoryAccountID()
				}
			}
			if result.SalesOrderId > 0 {
				if err := UpdateSaleOrderDetailInvoicedQty(tx, ctx, result.SalesOrderId, item, "delete", decimal.NewFromFloat(0), inventoryAccId); err != nil {
					tx.Rollback()
					return nil, err
				}

				_, err = ChangeSaleOrderCurrentStatus(tx.WithContext(ctx), ctx, businessId, result.SalesOrderId)
				if err != nil {
					tx.Rollback()
					return nil, err
				}
			}
		}
	}

	err = tx.WithContext(ctx).Model(&result).Association("Details").Unscoped().Clear()
	if err != nil {
		tx.Rollback()
		return nil, err
	}

	err = tx.WithContext(ctx).Delete(&result).Error
	if err != nil {
		tx.Rollback()
		return nil, err
	}

	if result.CurrentStatus == SalesInvoiceStatusConfirmed {
		err = PublishToAccounting(ctx, tx, businessId, result.InvoiceDate, result.ID, AccountReferenceTypeInvoice, nil, result, PubSubMessageActionDelete)
		if err != nil {
			tx.Rollback()
			return nil, err
		}
	}

	if err := deleteDocuments(ctx, tx, result.Documents); err != nil {
		tx.Rollback()
		return nil, err
	}

	// Commit the transaction
	if err := tx.Commit().Error; err != nil {
		return nil, err
	}

	return result, nil
}

func UpdateStatusSalesInvoice(ctx context.Context, id int, status string) (*SalesInvoice, error) {
	businessId, ok := utils.GetBusinessIdFromContext(ctx)
	if !ok || businessId == "" {
		return nil, errors.New("business id is required")
	}

	saleInvoice, err := utils.FetchModelForChange[SalesInvoice](ctx, businessId, id, "Details")
	if err != nil {
		return nil, err
	}

	oldStatus := saleInvoice.CurrentStatus

	// db action
	db := config.GetDB()
	tx := db.Begin()

	err = tx.WithContext(ctx).Model(&saleInvoice).Updates(map[string]interface{}{
		"CurrentStatus": status,
	}).Error

	if err != nil {
		tx.Rollback()
		return nil, err
	}

	// Apply inventory side-effects deterministically (prefer explicit command handler).
	if config.UseStockCommandsFor("SALES_INVOICE") {
		saleInvoice.CurrentStatus = SalesInvoiceStatus(status)
		if err := ApplySalesInvoiceStockForStatusTransition(tx.WithContext(ctx), saleInvoice, oldStatus); err != nil {
			tx.Rollback()
			return nil, err
		}
	} else {
		if err := saleInvoice.AfterUpdateCurrentStatus(tx.WithContext(ctx), string(oldStatus)); err != nil {
			tx.Rollback()
			return nil, err
		}
	}

	if saleInvoice.SalesOrderId > 0 {

		for _, item := range saleInvoice.Details {
			inventoryAccId := 0
			if item.ProductId > 0 {
				product, err := GetProductOrVariant(ctx, string(item.ProductType), item.ProductId)
				if err != nil {
					tx.Rollback()
					return nil, err
				}
				inventoryAccId = product.GetInventoryAccountID()
			}

			if status == string(SalesInvoiceStatusVoid) || status == string(SalesInvoiceStatusConfirmed) {
				if status == string(SalesInvoiceStatusVoid) {
					if err := UpdateSaleOrderDetailInvoicedQty(tx, ctx, saleInvoice.SalesOrderId, item, "delete", decimal.NewFromFloat(0), inventoryAccId); err != nil {
						tx.Rollback()
						return nil, err
					}
				}
				if status == string(SalesInvoiceStatusConfirmed) {
					if err := UpdateSaleOrderDetailInvoicedQty(tx, ctx, saleInvoice.SalesOrderId, item, "create", decimal.NewFromFloat(0), inventoryAccId); err != nil {
						tx.Rollback()
						return nil, err
					}
				}

				_, err = ChangeSaleOrderCurrentStatus(tx.WithContext(ctx), ctx, businessId, saleInvoice.SalesOrderId)
				if err != nil {
					tx.Rollback()
					return nil, err
				}
			}

		}
	}

	if oldStatus == SalesInvoiceStatusDraft && status == string(SalesInvoiceStatusConfirmed) {
		err := PublishToAccounting(ctx, tx, businessId, saleInvoice.InvoiceDate, saleInvoice.ID, AccountReferenceTypeInvoice, saleInvoice, nil, PubSubMessageActionCreate)
		if err != nil {
			tx.Rollback()
			return nil, err
		}
	} else if oldStatus == SalesInvoiceStatusConfirmed && status == string(SalesInvoiceStatusVoid) {
		err = PublishToAccounting(ctx, tx, businessId, saleInvoice.InvoiceDate, saleInvoice.ID, AccountReferenceTypeInvoice, nil, saleInvoice, PubSubMessageActionDelete)
		if err != nil {
			tx.Rollback()
			return nil, err
		}
	}

	// Commit the transaction
	if err := tx.Commit().Error; err != nil {
		return nil, err
	}

	return saleInvoice, nil
}

func WriteOffSalesInvoice(ctx context.Context, id int, date time.Time, reason string) (*SalesInvoice, error) {
	businessId, ok := utils.GetBusinessIdFromContext(ctx)
	if !ok || businessId == "" {
		return nil, errors.New("business id is required")
	}

	saleInvoice, err := utils.FetchModelForChange[SalesInvoice](ctx, businessId, id)
	if err != nil {
		return nil, err
	}

	if saleInvoice.CurrentStatus == SalesInvoiceStatusPaid {
		return nil, errors.New("invoice is fully paid")
	} else if saleInvoice.CurrentStatus == SalesInvoiceStatusWriteOff {
		return nil, errors.New("invoice is already written off")
	}

	// db action
	db := config.GetDB()
	tx := db.Begin()

	err = tx.WithContext(ctx).Model(&saleInvoice).Updates(map[string]interface{}{
		"CurrentStatus":              SalesInvoiceStatusWriteOff,
		"WriteOffDate":               date,
		"WriteOffReason":             reason,
		"InvoiceTotalWriteOffAmount": saleInvoice.RemainingBalance,
		"RemainingBalance":           decimal.NewFromInt(0),
	}).Error

	if err != nil {
		tx.Rollback()
		return nil, err
	}

	err = PublishToAccounting(ctx, tx, businessId, date, saleInvoice.ID, AccountReferenceTypeInvoiceWriteOff, saleInvoice, nil, PubSubMessageActionCreate)
	if err != nil {
		tx.Rollback()
		return nil, err
	}

	// Commit the transaction
	if err := tx.Commit().Error; err != nil {
		return nil, err
	}

	return saleInvoice, nil
}

func CancelWriteOffSalesInvoice(ctx context.Context, id int) (*SalesInvoice, error) {
	businessId, ok := utils.GetBusinessIdFromContext(ctx)
	if !ok || businessId == "" {
		return nil, errors.New("business id is required")
	}

	saleInvoice, err := utils.FetchModelForChange[SalesInvoice](ctx, businessId, id)
	if err != nil {
		return nil, err
	}

	if saleInvoice.CurrentStatus != SalesInvoiceStatusWriteOff {
		return nil, errors.New("invoice is not written off yet")
	}

	// db action
	db := config.GetDB()
	tx := db.Begin()

	writeOffDate := saleInvoice.WriteOffDate
	newStatus := SalesInvoiceStatusConfirmed
	if saleInvoice.InvoiceTotalAmount.GreaterThan(saleInvoice.InvoiceTotalWriteOffAmount) {
		newStatus = SalesInvoiceStatusPartialPaid
	}

	err = tx.WithContext(ctx).Model(&saleInvoice).Updates(map[string]interface{}{
		"CurrentStatus":              newStatus,
		"WriteOffDate":               nil,
		"WriteOffReason":             nil,
		"InvoiceTotalWriteOffAmount": decimal.NewFromInt(0),
		"RemainingBalance":           saleInvoice.InvoiceTotalWriteOffAmount,
	}).Error

	if err != nil {
		tx.Rollback()
		return nil, err
	}

	err = PublishToAccounting(ctx, tx, businessId, *writeOffDate, saleInvoice.ID, AccountReferenceTypeInvoiceWriteOff, nil, saleInvoice, PubSubMessageActionDelete)
	if err != nil {
		tx.Rollback()
		return nil, err
	}

	// Commit the transaction
	if err := tx.Commit().Error; err != nil {
		return nil, err
	}

	return saleInvoice, nil
}

func GetSalesInvoice(ctx context.Context, id int) (*SalesInvoice, error) {
	db := config.GetDB()

	var result SalesInvoice
	err := db.WithContext(ctx).First(&result, id).Error
	if err != nil {
		return nil, err
	}

	return &result, nil
}

func GetSalesInvoices(ctx context.Context, customerId *int, notes *string) ([]*SalesInvoice, error) {
	db := config.GetDB()
	var results []*SalesInvoice

	businessId, ok := utils.GetBusinessIdFromContext(ctx)
	if !ok || businessId == "" {
		return nil, errors.New("business id is required")
	}

	dbCtx := db.WithContext(ctx).Where("business_id = ?", businessId)

	if notes != nil && len(*notes) > 0 {
		dbCtx = dbCtx.Where("notes LIKE ?", "%"+*notes+"%")
	}

	if customerId != nil && *customerId != 0 && *customerId > 0 {
		dbCtx = dbCtx.Where("customer_id =?", customerId)
	}

	err := dbCtx.
		Find(&results).Error
	if err != nil {
		return nil, err
	}
	return results, nil
}

func PaginateSalesInvoice(ctx context.Context, limit *int, after *string,
	invoiceNumber *string,
	referenceNumber *string,
	branchID *int,
	warehouseID *int,
	customerID *int,
	status *SalesInvoiceStatus,
	startInvoiceDate *MyDateString,
	endInvoiceDate *MyDateString,
	startInvoiceDueDate *MyDateString,
	endInvoiceDueDate *MyDateString) (*SalesInvoicesConnection,
	error) {

	businessId, ok := utils.GetBusinessIdFromContext(ctx)
	if !ok || businessId == "" {
		return nil, errors.New("business id is required")
	}

	business, err := GetBusiness(ctx)
	if err != nil {
		return nil, errors.New("business id is required")
	}
	if err := startInvoiceDate.StartOfDayUTCTime(business.Timezone); err != nil {
		return nil, err
	}
	if err := endInvoiceDate.EndOfDayUTCTime(business.Timezone); err != nil {
		return nil, err
	}
	if err := startInvoiceDueDate.StartOfDayUTCTime(business.Timezone); err != nil {
		return nil, err
	}
	if err := endInvoiceDueDate.EndOfDayUTCTime(business.Timezone); err != nil {
		return nil, err
	}

	db := config.GetDB()
	dbCtx := db.WithContext(ctx).Where("business_id = ?", businessId)

	if invoiceNumber != nil && *invoiceNumber != "" {
		dbCtx.Where("invoice_number LIKE ?", "%"+*invoiceNumber+"%")
	}
	if referenceNumber != nil && *referenceNumber != "" {
		dbCtx.Where("reference_number LIKE ?", "%"+*referenceNumber+"%")
	}
	if branchID != nil && *branchID > 0 {
		dbCtx.Where("branch_id = ?", *branchID)
	}
	if customerID != nil && *customerID > 0 {
		dbCtx.Where("customer_id = ?", *customerID)
	}
	if warehouseID != nil && *warehouseID > 0 {
		dbCtx.Where("warehouse_id = ?", *warehouseID)
	}
	if status != nil {
		dbCtx.Where("current_status = ?", *status)
	}
	if startInvoiceDate != nil && endInvoiceDate != nil {
		dbCtx.Where("invoice_date BETWEEN ? AND ?", startInvoiceDate, endInvoiceDate)
	}
	if startInvoiceDueDate != nil && endInvoiceDueDate != nil {
		dbCtx.Where("invoice_due_date BETWEEN ? AND ?", startInvoiceDueDate, endInvoiceDueDate)
	}

	edges, pageInfo, err := FetchPageCompositeCursor[SalesInvoice](dbCtx, *limit, after, "created_at", "<")
	if err != nil {
		return nil, err
	}
	var salesInvoicesConnection SalesInvoicesConnection
	salesInvoicesConnection.PageInfo = pageInfo
	for _, edge := range edges {
		salesInvoicesEdge := SalesInvoicesEdge(edge)
		salesInvoicesConnection.Edges = append(salesInvoicesConnection.Edges, &salesInvoicesEdge)
	}

	return &salesInvoicesConnection, err
}

func GetInvoiceTotalSummary(ctx context.Context) (*InvoiceTotalSummary, error) {
	var totalSummary InvoiceTotalSummary
	businessId, ok := utils.GetBusinessIdFromContext(ctx)
	if !ok || businessId == "" {
		return nil, errors.New("business id is required")
	}
	business, err := GetBusiness(ctx)
	if err != nil {
		return nil, errors.New("business id is required")
	}
	timezone := "Asia/Yangon"
	if business.Timezone != "" {
		timezone = business.Timezone
	}

	db := config.GetDB()
	dbCtx := db.WithContext(ctx).Where("business_id = ?", businessId)

	err = dbCtx.Table("sales_invoices").
		Select("SUM((invoice_total_amount - invoice_total_paid_amount) * CASE WHEN exchange_rate = 0 THEN 1 ELSE exchange_rate END) AS TotalOutstandingReceivable, "+
			"SUM(CASE WHEN DATE(CONVERT_TZ(invoice_due_date, 'UTC', ?)) = DATE(CONVERT_TZ(UTC_TIMESTAMP(), 'UTC', ?)) THEN (invoice_total_amount - invoice_total_paid_amount) * CASE WHEN exchange_rate = 0 THEN 1 ELSE exchange_rate END ELSE 0 END) AS DueToday, "+
			"SUM(CASE WHEN DATE(CONVERT_TZ(invoice_due_date, 'UTC', ?)) BETWEEN DATE(CONVERT_TZ(UTC_TIMESTAMP(), 'UTC', ?)) AND DATE_ADD(DATE(CONVERT_TZ(UTC_TIMESTAMP(), 'UTC', ?)), INTERVAL 30 DAY) THEN (invoice_total_amount - invoice_total_paid_amount) * CASE WHEN exchange_rate = 0 THEN 1 ELSE exchange_rate END ELSE 0 END) AS DueWithin30Days, "+
			"SUM(CASE WHEN DATE(CONVERT_TZ(invoice_due_date, 'UTC', ?)) < DATE(CONVERT_TZ(UTC_TIMESTAMP(), 'UTC', ?)) THEN (invoice_total_amount - invoice_total_paid_amount) * CASE WHEN exchange_rate = 0 THEN 1 ELSE exchange_rate END ELSE 0 END) AS TotalOverdue",
			timezone, timezone, timezone, timezone, timezone, timezone, timezone).
		Where("current_status IN ('Confirmed', 'Partial Paid')").
		Scan(&totalSummary).Error

	if err != nil {
		return nil, err
	}
	return &totalSummary, nil
}

func GetSalesInvoiceIdsByCustomerID(ctx context.Context, customerId int, invoiceStatus string) ([]int, error) {

	db := config.GetDB()
	var invoiceIds []int

	businessId, ok := utils.GetBusinessIdFromContext(ctx)
	if !ok || businessId == "" {
		return nil, errors.New("business id is required")
	}

	dbCtx := db.WithContext(ctx).Model(&SalesInvoice{}).Where("business_id = ?", businessId).Where("customer_id = ?", customerId)

	var result *gorm.DB // Declare result variable

	if invoiceStatus == "Paid" {
		result = dbCtx.Where("current_status = ?", SalesInvoiceStatusPaid).Pluck("id", &invoiceIds)
	} else if invoiceStatus == "UnPaid" {
		result = dbCtx.Where("current_status = ? OR current_status = ?", SalesInvoiceStatusPartialPaid, SalesInvoiceStatusConfirmed).Pluck("id", &invoiceIds)
	} else {
		result = dbCtx.Pluck("id", &invoiceIds)
	}

	if result.Error != nil {
		return nil, result.Error
	}

	return invoiceIds, nil
}
