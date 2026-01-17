package models_test

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/mmdatafocus/books_backend/config"
	"github.com/mmdatafocus/books_backend/models"
	"github.com/mmdatafocus/books_backend/models/reports"
	"github.com/mmdatafocus/books_backend/utils"
	"github.com/mmdatafocus/books_backend/workflow"
	"github.com/shopspring/decimal"
	"github.com/sirupsen/logrus"
)

// Full-stack regression: all reports/screens must agree on on-hand quantity and valuation.
func TestInventoryTruthConsistency_AllScreensAgree(t *testing.T) {
	if strings.TrimSpace(os.Getenv("INTEGRATION_TESTS")) == "" {
		t.Skip("set INTEGRATION_TESTS=1 to run integration tests (requires docker)")
	}

	ctx := context.Background()

	redisName, redisPort := startRedisContainer(t)
	t.Cleanup(func() { _ = dockerRmForce(redisName) })

	mysqlName, mysqlPort := startMySQLContainer(t)
	t.Cleanup(func() { _ = dockerRmForce(mysqlName) })

	t.Setenv("REDIS_ADDRESS", fmt.Sprintf("127.0.0.1:%s", redisPort))
	t.Setenv("DB_USER", "root")
	t.Setenv("DB_PASSWORD", "testpw")
	t.Setenv("DB_HOST", "127.0.0.1")
	t.Setenv("DB_PORT", mysqlPort)
	t.Setenv("DB_NAME_2", "pitibooks_test")
	t.Setenv("STOCK_COMMANDS_DOCS", "")

	config.ConnectDatabaseWithRetry()
	config.ConnectRedisWithRetry()
	models.MigrateTable()

	ctx = utils.SetUserIdInContext(ctx, 1)
	ctx = utils.SetUserNameInContext(ctx, "Test")
	ctx = utils.SetUsernameInContext(ctx, "test@local")

	biz, err := models.CreateBusiness(ctx, &models.NewBusiness{
		Name:  "Truth Co",
		Email: "owner@truth.test",
	})
	if err != nil {
		t.Fatalf("CreateBusiness: %v", err)
	}
	businessID := biz.ID.String()
	ctx = utils.SetBusinessIdInContext(ctx, businessID)

	db := config.GetDB()
	var primary models.Warehouse
	if err := db.WithContext(ctx).Where("business_id = ? AND name = ?", businessID, "Primary Warehouse").First(&primary).Error; err != nil {
		t.Fatalf("fetch primary warehouse: %v", err)
	}
	secondary, err := models.CreateWarehouse(ctx, &models.NewWarehouse{
		BranchId: biz.PrimaryBranchId,
		Name:     "Secondary",
	})
	if err != nil {
		t.Fatalf("CreateWarehouse secondary: %v", err)
	}

	unit, err := models.CreateProductUnit(ctx, &models.NewProductUnit{Name: "Pcs", Abbreviation: "pc", Precision: models.PrecisionZero})
	if err != nil {
		t.Fatalf("CreateProductUnit: %v", err)
	}
	sysAccounts, err := models.GetSystemAccounts(businessID)
	if err != nil {
		t.Fatalf("GetSystemAccounts: %v", err)
	}

	phone, err := models.CreateProduct(ctx, &models.NewProduct{
		Name:               "Phone",
		Sku:                "PHONE-1",
		Barcode:            "PHONE-1",
		UnitId:             unit.ID,
		SalesAccountId:     sysAccounts[models.AccountCodeSales],
		PurchaseAccountId:  sysAccounts[models.AccountCodeCostOfGoodsSold],
		InventoryAccountId: sysAccounts[models.AccountCodeInventoryAsset],
		IsBatchTracking:    utils.NewFalse(),
		OpeningStocks: []models.NewOpeningStock{
			{WarehouseId: primary.ID, Qty: decimal.NewFromInt(100), UnitValue: decimal.NewFromInt(50)},
			{WarehouseId: secondary.ID, Qty: decimal.NewFromInt(10), UnitValue: decimal.NewFromInt(50)},
		},
	})
	if err != nil {
		t.Fatalf("CreateProduct: %v", err)
	}

	// Process opening stock via outbox to populate stock_histories.
	var posOutbox models.PubSubMessageRecord
	if err := db.WithContext(ctx).
		Where("business_id = ? AND reference_type = ? AND reference_id = ? AND action = ?",
			businessID, models.AccountReferenceTypeProductOpeningStock, phone.ID, models.PubSubMessageActionCreate).
		Order("id DESC").
		First(&posOutbox).Error; err != nil {
		t.Fatalf("expected outbox record for product opening stock: %v", err)
	}
	logger := logrus.New()
	txPOS := db.Begin()
	if err := workflow.ProcessProductOpeningStockWorkflow(txPOS, logger, models.ConvertToPubSubMessage(posOutbox)); err != nil {
		t.Fatalf("ProcessProductOpeningStockWorkflow: %v", err)
	}
	if err := txPOS.Commit().Error; err != nil {
		t.Fatalf("opening stock workflow commit: %v", err)
	}

	addLedger := func(wh int, qty decimal.Decimal, stockDate time.Time, refType models.StockReferenceType, isOutgoing bool, isTransferIn bool, unitCost decimal.Decimal) {
		sh := models.StockHistory{
			BusinessId:    businessID,
			WarehouseId:   wh,
			ProductId:     phone.ID,
			ProductType:   models.ProductTypeSingle,
			StockDate:     stockDate,
			Qty:           qty,
			BaseUnitValue: unitCost,
			ReferenceType: refType,
			ReferenceID:   999,
			IsOutgoing: func() *bool {
				if isOutgoing {
					return utils.NewTrue()
				} else {
					return utils.NewFalse()
				}
			}(),
			IsTransferIn: func() *bool {
				if isTransferIn {
					return utils.NewTrue()
				} else {
					return utils.NewFalse()
				}
			}(),
			Description: "test-ledger",
		}
		if err := db.WithContext(ctx).Create(&sh).Error; err != nil {
			t.Fatalf("insert stock_history %s: %v", refType, err)
		}
	}

	// Dates
	poDate := time.Date(2024, 1, 11, 0, 0, 0, 0, time.UTC)
	invoiceDate := time.Date(2024, 1, 12, 0, 0, 0, 0, time.UTC)
	transferDate := time.Date(2024, 1, 13, 0, 0, 0, 0, time.UTC)
	adjustDate := time.Date(2024, 1, 9, 0, 0, 0, 0, time.UTC)

	// Backdated PO (+5) before invoice.
	addLedger(primary.ID, decimal.NewFromInt(5), poDate, models.StockReferenceTypeBill, false, false, decimal.NewFromInt(50))
	// Invoice (-8) after PO.
	addLedger(primary.ID, decimal.NewFromInt(-8), invoiceDate, models.StockReferenceTypeInvoice, true, false, decimal.NewFromInt(50))
	// Transfer 10 from secondary -> primary.
	addLedger(secondary.ID, decimal.NewFromInt(-10), transferDate, models.StockReferenceTypeTransferOrder, true, false, decimal.NewFromInt(50))
	addLedger(primary.ID, decimal.NewFromInt(10), transferDate, models.StockReferenceTypeTransferOrder, false, true, decimal.NewFromInt(50))
	// Inventory adjustment qty +2 on earlier date.
	addLedger(primary.ID, decimal.NewFromInt(2), adjustDate, models.StockReferenceTypeInventoryAdjustmentQuantity, false, false, decimal.NewFromInt(50))

	asOf := models.MyDateString(time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC))
	fromDate := models.MyDateString(time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC))

	// Canonical expectations
	expectedPrimary := decimal.NewFromInt(100).
		Add(decimal.NewFromInt(5)).  // PO
		Add(decimal.NewFromInt(-8)). // invoice
		Add(decimal.NewFromInt(10)). // transfer in
		Add(decimal.NewFromInt(2))   // adjustment
	expectedSecondary := decimal.NewFromInt(10).Add(decimal.NewFromInt(-10))
	expectedTotal := expectedPrimary.Add(expectedSecondary)

	stockSummary, err := reports.GetStockSummaryReport(ctx, fromDate, asOf, nil)
	if err != nil {
		t.Fatalf("GetStockSummaryReport: %v", err)
	}
	if len(stockSummary) != 1 {
		t.Fatalf("expected 1 product in stock summary, got %d", len(stockSummary))
	}
	if !stockSummary[0].ClosingStock.Equal(expectedTotal) {
		t.Fatalf("stock summary closing expected %s got %s", expectedTotal, stockSummary[0].ClosingStock)
	}

	invSummary, err := reports.GetInventorySummaryReport(ctx, asOf, nil)
	if err != nil {
		t.Fatalf("GetInventorySummaryReport: %v", err)
	}
	if len(invSummary) != 1 {
		t.Fatalf("expected 1 product in inventory summary, got %d", len(invSummary))
	}
	if !invSummary[0].CurrentQty.Equal(expectedTotal) {
		t.Fatalf("inventory summary current qty expected %s got %s", expectedTotal, invSummary[0].CurrentQty)
	}

	warehouseReport, err := reports.GetWarehouseInventoryReport(ctx, asOf)
	if err != nil {
		t.Fatalf("GetWarehouseInventoryReport: %v", err)
	}
	var primaryRow, secondaryRow *reports.WarehouseInventoryResponse
	for _, r := range warehouseReport {
		if r.WarehouseId == primary.ID {
			primaryRow = r
		}
		if r.WarehouseId == secondary.ID {
			secondaryRow = r
		}
	}
	if primaryRow == nil || secondaryRow == nil {
		t.Fatalf("warehouse rows missing")
	}
	if !primaryRow.CurrentQty.Equal(expectedPrimary) {
		t.Fatalf("primary current qty expected %s got %s", expectedPrimary, primaryRow.CurrentQty)
	}
	if !secondaryRow.CurrentQty.Equal(expectedSecondary) {
		t.Fatalf("secondary current qty expected %s got %s", expectedSecondary, secondaryRow.CurrentQty)
	}
	sumWarehouses := primaryRow.CurrentQty.Add(secondaryRow.CurrentQty)
	if !sumWarehouses.Equal(expectedTotal) {
		t.Fatalf("sum warehouses expected %s got %s", expectedTotal, sumWarehouses)
	}

	valSummary, err := reports.GetInventoryValuationSummaryReport(ctx, asOf, 0)
	if err != nil {
		t.Fatalf("GetInventoryValuationSummaryReport: %v", err)
	}
	if len(valSummary) != 1 {
		t.Fatalf("expected 1 product in valuation summary, got %d", len(valSummary))
	}
	if !valSummary[0].StockOnHand.Equal(expectedTotal) {
		t.Fatalf("valuation summary stockOnHand expected %s got %s", expectedTotal, valSummary[0].StockOnHand)
	}

	stockInHand, err := models.GetStockInHand(ctx, phone.ID, string(models.ProductTypeSingle))
	if err != nil {
		t.Fatalf("GetStockInHand: %v", err)
	}
	if !stockInHand.Equal(expectedTotal) {
		t.Fatalf("product StockInHand expected %s got %s", expectedTotal, stockInHand)
	}
}
