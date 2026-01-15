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
	"github.com/mmdatafocus/books_backend/utils"
	"github.com/mmdatafocus/books_backend/workflow"
	"github.com/shopspring/decimal"
	"github.com/sirupsen/logrus"
)

// Regression: inventory valuation should not double-count invoice qty when the invoice workflow is retried.
//
// Bug symptom (UI): invoice row shows qty -2 while invoice line qty is 1.
// Root cause: duplicate active stock_histories rows created by at-least-once invoice workflow processing.
func TestInventoryValuation_DoesNotDoubleCountInvoiceOnWorkflowRetry(t *testing.T) {
	if strings.TrimSpace(os.Getenv("INTEGRATION_TESTS")) == "" {
		t.Skip("set INTEGRATION_TESTS=1 to run integration tests (requires docker)")
	}

	ctx := context.Background()

	redisName, redisPort := startRedisContainer(t)
	t.Cleanup(func() { _ = dockerRmForce(redisName) })

	mysqlName, mysqlPort := startMySQLContainer(t)
	t.Cleanup(func() { _ = dockerRmForce(mysqlName) })

	// Wire env for config.Connect* helpers.
	t.Setenv("REDIS_ADDRESS", fmt.Sprintf("127.0.0.1:%s", redisPort))
	t.Setenv("DB_USER", "root")
	t.Setenv("DB_PASSWORD", "testpw")
	t.Setenv("DB_HOST", "127.0.0.1")
	t.Setenv("DB_PORT", mysqlPort)
	t.Setenv("DB_NAME_2", "pitibooks_test")
	// Use legacy mode to match many production deployments; invoice stock_summaries updates happen synchronously anyway.
	t.Setenv("STOCK_COMMANDS_DOCS", "")

	config.ConnectDatabaseWithRetry()
	config.ConnectRedisWithRetry()
	models.MigrateTable()

	// Context required by model hooks/history and business scoping.
	ctx = utils.SetUserIdInContext(ctx, 1)
	ctx = utils.SetUserNameInContext(ctx, "Test")
	ctx = utils.SetUsernameInContext(ctx, "test@local")

	// 1) Create business.
	biz, err := models.CreateBusiness(ctx, &models.NewBusiness{
		Name:  "Test Biz",
		Email: "owner@test.local",
	})
	if err != nil {
		t.Fatalf("CreateBusiness: %v", err)
	}
	businessID := biz.ID.String()
	ctx = utils.SetBusinessIdInContext(ctx, businessID)

	db := config.GetDB()
	// Tests use historical dates; relax transaction lock dates created at business creation.
	relaxDate := time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)
	if err := db.WithContext(ctx).Model(&models.Business{}).Where("id = ?", biz.ID).Updates(map[string]interface{}{
		"MigrationDate":                 relaxDate,
		"SalesTransactionLockDate":      relaxDate,
		"PurchaseTransactionLockDate":   relaxDate,
		"BankingTransactionLockDate":    relaxDate,
		"AccountantTransactionLockDate": relaxDate,
	}).Error; err != nil {
		t.Fatalf("relax business lock dates: %v", err)
	}
	var primary models.Warehouse
	if err := db.WithContext(ctx).Where("business_id = ? AND name = ?", businessID, "Primary Warehouse").First(&primary).Error; err != nil {
		t.Fatalf("fetch primary warehouse: %v", err)
	}

	// Reason for adjustment.
	reason, err := models.CreateReason(ctx, &models.NewReason{Name: "Damage"})
	if err != nil {
		t.Fatalf("CreateReason: %v", err)
	}

	// Product unit.
	unit, err := models.CreateProductUnit(ctx, &models.NewProductUnit{Name: "Pcs", Abbreviation: "pc", Precision: models.PrecisionZero})
	if err != nil {
		t.Fatalf("CreateProductUnit: %v", err)
	}

	// System accounts.
	sysAccounts, err := models.GetSystemAccounts(businessID)
	if err != nil {
		t.Fatalf("GetSystemAccounts: %v", err)
	}
	invAcc := sysAccounts[models.AccountCodeInventoryAsset]
	salesAcc := sysAccounts[models.AccountCodeSales]
	cogsAcc := sysAccounts[models.AccountCodeCostOfGoodsSold]
	if invAcc == 0 || salesAcc == 0 || cogsAcc == 0 {
		t.Fatalf("missing required system accounts (inv=%d sales=%d cogs=%d)", invAcc, salesAcc, cogsAcc)
	}

	// 2) Create product Shoes with opening stock 20 at Primary Warehouse.
	shoes, err := models.CreateProduct(ctx, &models.NewProduct{
		Name:               "Shoes",
		Sku:                "SHOES-001",
		Barcode:            "SHOES-001",
		UnitId:             unit.ID,
		SalesAccountId:     salesAcc,
		PurchaseAccountId:  cogsAcc,
		InventoryAccountId: invAcc,
		IsBatchTracking:    utils.NewFalse(),
		OpeningStocks: []models.NewOpeningStock{
			{WarehouseId: primary.ID, Qty: decimal.NewFromInt(20), UnitValue: decimal.NewFromInt(500)},
		},
	})
	if err != nil {
		t.Fatalf("CreateProduct: %v", err)
	}

	// 3) Create customer.
	customer, err := models.CreateCustomer(ctx, &models.NewCustomer{
		Name:                 "customer3",
		Email:                "customer3@test.local",
		CurrencyId:           biz.BaseCurrencyId,
		ExchangeRate:         decimal.NewFromInt(1),
		CustomerPaymentTerms: models.PaymentTermsDueOnReceipt,
	})
	if err != nil {
		t.Fatalf("CreateCustomer: %v", err)
	}

	// 4) Match UI scenario date.
	invDate := time.Date(2026, 1, 15, 12, 0, 0, 0, time.UTC)
	isTaxInclusive := false
	saleInvoice, err := models.CreateSalesInvoice(ctx, &models.NewSalesInvoice{
		CustomerId:          customer.ID,
		BranchId:            biz.PrimaryBranchId,
		InvoiceDate:         invDate,
		InvoicePaymentTerms: models.PaymentTermsDueOnReceipt,
		CurrencyId:          biz.BaseCurrencyId,
		ExchangeRate:        decimal.NewFromInt(1),
		WarehouseId:         primary.ID,
		IsTaxInclusive:      &isTaxInclusive,
		CurrentStatus:       models.SalesInvoiceStatusConfirmed,
		Details: []models.NewSalesInvoiceDetail{
			{
				ProductId:      shoes.ID,
				ProductType:    models.ProductTypeSingle,
				BatchNumber:    "",
				Name:           "Shoes",
				Description:    "",
				DetailQty:      decimal.NewFromInt(1),
				DetailUnitRate: decimal.NewFromInt(1000),
			},
		},
	})
	if err != nil {
		t.Fatalf("CreateSalesInvoice: %v", err)
	}

	// 5) Process invoice outbox record via workflow (simulate accounting worker).
	var invOutbox models.PubSubMessageRecord
	if err := db.WithContext(ctx).
		Where("business_id = ? AND reference_type = ? AND reference_id = ? AND action = ?",
			businessID, models.AccountReferenceTypeInvoice, saleInvoice.ID, models.PubSubMessageActionCreate).
		Order("id DESC").
		First(&invOutbox).Error; err != nil {
		t.Fatalf("expected outbox record for invoice: %v", err)
	}
	invMsg := models.ConvertToPubSubMessage(invOutbox)
	logger := logrus.New()

	// First processing run.
	wtx := db.Begin()
	if err := workflow.ProcessInvoiceWorkflow(wtx, logger, invMsg); err != nil {
		t.Fatalf("ProcessInvoiceWorkflow(1): %v", err)
	}
	if err := wtx.Commit().Error; err != nil {
		t.Fatalf("invoice workflow commit(1): %v", err)
	}

	// Second processing run (simulate at-least-once retry/double delivery).
	wtx2 := db.Begin()
	if err := workflow.ProcessInvoiceWorkflow(wtx2, logger, invMsg); err != nil {
		t.Fatalf("ProcessInvoiceWorkflow(2): %v", err)
	}
	if err := wtx2.Commit().Error; err != nil {
		t.Fatalf("invoice workflow commit(2): %v", err)
	}

	// 6) Create inventory adjustment qty -1 at Primary Warehouse on 15 Jan 2026, status Adjusted.
	ia, err := models.CreateInventoryAdjustment(ctx, &models.NewInventoryAdjustment{
		ReferenceNumber: "IA-72",
		AdjustmentType:  models.InventoryAdjustmentTypeQuantity,
		AdjustmentDate:  invDate,
		AccountId:       cogsAcc,
		BranchId:        biz.PrimaryBranchId,
		WarehouseId:     primary.ID,
		CurrentStatus:   models.InventoryAdjustmentStatusAdjusted,
		ReasonId:        reason.ID,
		Description:     "Damage",
		Details: []models.NewInventoryAdjustmentDetail{
			{
				ProductId:     shoes.ID,
				ProductType:   models.ProductTypeSingle,
				BatchNumber:   "",
				Name:          "Shoes",
				AdjustedValue: decimal.NewFromInt(-1),
				CostPrice:     decimal.NewFromInt(500),
			},
		},
	})
	if err != nil {
		t.Fatalf("CreateInventoryAdjustment: %v", err)
	}

	// Process adjustment outbox record via workflow.
	var iaOutbox models.PubSubMessageRecord
	if err := db.WithContext(ctx).
		Where("business_id = ? AND reference_type = ? AND reference_id = ? AND action = ?",
			businessID, models.AccountReferenceTypeInventoryAdjustmentQuantity, ia.ID, models.PubSubMessageActionCreate).
		Order("id DESC").
		First(&iaOutbox).Error; err != nil {
		t.Fatalf("expected outbox record for inventory adjustment: %v", err)
	}
	iaMsg := models.ConvertToPubSubMessage(iaOutbox)
	wtx3 := db.Begin()
	if err := workflow.ProcessInventoryAdjustmentQuantityWorkflow(wtx3, logger, iaMsg); err != nil {
		t.Fatalf("ProcessInventoryAdjustmentQuantityWorkflow: %v", err)
	}
	if err := wtx3.Commit().Error; err != nil {
		t.Fatalf("adjustment workflow commit: %v", err)
	}

	// 7) Assert Inventory Valuation: opening 20, invoice -1, adjustment -1, closing 18 (not 17).
	from := models.MyDateString(time.Date(2025, 12, 31, 0, 0, 0, 0, time.UTC))
	to := models.MyDateString(time.Date(2026, 1, 31, 0, 0, 0, 0, time.UTC))
	val, err := models.GetInventoryValuation(ctx, from, to, shoes.ID, models.ProductTypeSingle, primary.ID)
	if err != nil {
		t.Fatalf("GetInventoryValuation: %v", err)
	}
	if val.OpeningStockOnHand.Cmp(decimal.NewFromInt(20)) != 0 {
		t.Fatalf("expected opening stock=20; got %s", val.OpeningStockOnHand.String())
	}
	if val.ClosingStockOnHand.Cmp(decimal.NewFromInt(18)) != 0 {
		t.Fatalf("expected closing stock=18; got %s", val.ClosingStockOnHand.String())
	}

	// Invoice row qty should be exactly -1 (not -2).
	var invoiceQty decimal.Decimal
	invoiceDesc := "Invoice #" + saleInvoice.InvoiceNumber
	for _, d := range val.Details {
		if d == nil {
			continue
		}
		if d.TransactionDescription == invoiceDesc {
			invoiceQty = invoiceQty.Add(d.Qty)
		}
	}
	if invoiceQty.Cmp(decimal.NewFromInt(-1)) != 0 {
		t.Fatalf("expected invoice qty=-1 in valuation details; got %s (desc=%q)", invoiceQty.String(), invoiceDesc)
	}

	// Sanity: there should be only one active stock_history row for this invoice detail.
	var invDetailID int
	if len(saleInvoice.Details) > 0 {
		invDetailID = saleInvoice.Details[0].ID
	}
	if invDetailID == 0 {
		t.Fatalf("expected invoice detail id to be populated")
	}
	var activeCount int64
	if err := db.WithContext(ctx).Model(&models.StockHistory{}).
		Where("business_id = ? AND reference_type = ? AND reference_id = ? AND reference_detail_id = ? AND is_reversal = 0 AND reversed_by_stock_history_id IS NULL",
			businessID, models.StockReferenceTypeInvoice, saleInvoice.ID, invDetailID).
		Count(&activeCount).Error; err != nil {
		t.Fatalf("count active stock_histories for invoice: %v", err)
	}
	if activeCount != 1 {
		t.Fatalf("expected exactly 1 active stock_history for invoice detail; got %d", activeCount)
	}
}

func TestInventoryValuation_Order_AdjustmentThenInvoice_RemainsConsistent(t *testing.T) {
	if strings.TrimSpace(os.Getenv("INTEGRATION_TESTS")) == "" {
		t.Skip("set INTEGRATION_TESTS=1 to run integration tests (requires docker)")
	}

	ctx := context.Background()

	redisName, redisPort := startRedisContainer(t)
	t.Cleanup(func() { _ = dockerRmForce(redisName) })

	mysqlName, mysqlPort := startMySQLContainer(t)
	t.Cleanup(func() { _ = dockerRmForce(mysqlName) })

	// Wire env for config.Connect* helpers.
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
		Name:  "Test Biz",
		Email: "owner@test.local",
	})
	if err != nil {
		t.Fatalf("CreateBusiness: %v", err)
	}
	businessID := biz.ID.String()
	ctx = utils.SetBusinessIdInContext(ctx, businessID)

	db := config.GetDB()
	// Relax transaction lock dates for historical postings.
	relaxDate := time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)
	if err := db.WithContext(ctx).Model(&models.Business{}).Where("id = ?", biz.ID).Updates(map[string]interface{}{
		"MigrationDate":                 relaxDate,
		"SalesTransactionLockDate":      relaxDate,
		"PurchaseTransactionLockDate":   relaxDate,
		"BankingTransactionLockDate":    relaxDate,
		"AccountantTransactionLockDate": relaxDate,
	}).Error; err != nil {
		t.Fatalf("relax business lock dates: %v", err)
	}
	var primary models.Warehouse
	if err := db.WithContext(ctx).Where("business_id = ? AND name = ?", businessID, "Primary Warehouse").First(&primary).Error; err != nil {
		t.Fatalf("fetch primary warehouse: %v", err)
	}

	reason, err := models.CreateReason(ctx, &models.NewReason{Name: "Damage"})
	if err != nil {
		t.Fatalf("CreateReason: %v", err)
	}

	unit, err := models.CreateProductUnit(ctx, &models.NewProductUnit{Name: "Pcs", Abbreviation: "pc", Precision: models.PrecisionZero})
	if err != nil {
		t.Fatalf("CreateProductUnit: %v", err)
	}

	sysAccounts, err := models.GetSystemAccounts(businessID)
	if err != nil {
		t.Fatalf("GetSystemAccounts: %v", err)
	}
	invAcc := sysAccounts[models.AccountCodeInventoryAsset]
	salesAcc := sysAccounts[models.AccountCodeSales]
	cogsAcc := sysAccounts[models.AccountCodeCostOfGoodsSold]
	if invAcc == 0 || salesAcc == 0 || cogsAcc == 0 {
		t.Fatalf("missing required system accounts (inv=%d sales=%d cogs=%d)", invAcc, salesAcc, cogsAcc)
	}

	// Product "MAC": opening 20 @ unit value 100 to match screenshots.
	mac, err := models.CreateProduct(ctx, &models.NewProduct{
		Name:               "MAC",
		Sku:                "MAC-001",
		Barcode:            "MAC-001",
		UnitId:             unit.ID,
		SalesAccountId:     salesAcc,
		PurchaseAccountId:  cogsAcc,
		InventoryAccountId: invAcc,
		IsBatchTracking:    utils.NewFalse(),
		OpeningStocks: []models.NewOpeningStock{
			{WarehouseId: primary.ID, Qty: decimal.NewFromInt(20), UnitValue: decimal.NewFromInt(100)},
		},
	})
	if err != nil {
		t.Fatalf("CreateProduct(MAC): %v", err)
	}

	customer, err := models.CreateCustomer(ctx, &models.NewCustomer{
		Name:                 "customer3",
		Email:                "customer3@test.local",
		CurrencyId:           biz.BaseCurrencyId,
		ExchangeRate:         decimal.NewFromInt(1),
		CustomerPaymentTerms: models.PaymentTermsDueOnReceipt,
	})
	if err != nil {
		t.Fatalf("CreateCustomer: %v", err)
	}

	logger := logrus.New()
	// Match UI scenario date.
	invDate := time.Date(2026, 1, 15, 12, 0, 0, 0, time.UTC)

	// Adjustment first (-1 @ 100).
	ia, err := models.CreateInventoryAdjustment(ctx, &models.NewInventoryAdjustment{
		ReferenceNumber: "IA-76",
		AdjustmentType:  models.InventoryAdjustmentTypeQuantity,
		AdjustmentDate:  invDate,
		AccountId:       cogsAcc,
		BranchId:        biz.PrimaryBranchId,
		WarehouseId:     primary.ID,
		CurrentStatus:   models.InventoryAdjustmentStatusAdjusted,
		ReasonId:        reason.ID,
		Description:     "Damage",
		Details: []models.NewInventoryAdjustmentDetail{
			{
				ProductId:     mac.ID,
				ProductType:   models.ProductTypeSingle,
				BatchNumber:   "",
				Name:          "MAC",
				AdjustedValue: decimal.NewFromInt(-1),
				CostPrice:     decimal.NewFromInt(100),
			},
		},
	})
	if err != nil {
		t.Fatalf("CreateInventoryAdjustment(MAC): %v", err)
	}

	var iaOutbox models.PubSubMessageRecord
	if err := db.WithContext(ctx).
		Where("business_id = ? AND reference_type = ? AND reference_id = ? AND action = ?",
			businessID, models.AccountReferenceTypeInventoryAdjustmentQuantity, ia.ID, models.PubSubMessageActionCreate).
		Order("id DESC").
		First(&iaOutbox).Error; err != nil {
		t.Fatalf("expected outbox record for inventory adjustment: %v", err)
	}
	iaMsg := models.ConvertToPubSubMessage(iaOutbox)
	wtxIA := db.Begin()
	if err := workflow.ProcessInventoryAdjustmentQuantityWorkflow(wtxIA, logger, iaMsg); err != nil {
		t.Fatalf("ProcessInventoryAdjustmentQuantityWorkflow: %v", err)
	}
	if err := wtxIA.Commit().Error; err != nil {
		t.Fatalf("adjustment workflow commit: %v", err)
	}

	// Second processing run (simulate at-least-once retry/double delivery for adjustment).
	wtxIA2 := db.Begin()
	if err := workflow.ProcessInventoryAdjustmentQuantityWorkflow(wtxIA2, logger, iaMsg); err != nil {
		t.Fatalf("ProcessInventoryAdjustmentQuantityWorkflow(2): %v", err)
	}
	if err := wtxIA2.Commit().Error; err != nil {
		t.Fatalf("adjustment workflow commit(2): %v", err)
	}

	// Then invoice qty 1.
	isTaxInclusive := false
	saleInvoice, err := models.CreateSalesInvoice(ctx, &models.NewSalesInvoice{
		CustomerId:          customer.ID,
		BranchId:            biz.PrimaryBranchId,
		InvoiceDate:         invDate,
		InvoicePaymentTerms: models.PaymentTermsDueOnReceipt,
		CurrencyId:          biz.BaseCurrencyId,
		ExchangeRate:        decimal.NewFromInt(1),
		WarehouseId:         primary.ID,
		IsTaxInclusive:      &isTaxInclusive,
		CurrentStatus:       models.SalesInvoiceStatusConfirmed,
		Details: []models.NewSalesInvoiceDetail{
			{
				ProductId:      mac.ID,
				ProductType:    models.ProductTypeSingle,
				BatchNumber:    "",
				Name:           "MAC",
				Description:    "",
				DetailQty:      decimal.NewFromInt(1),
				DetailUnitRate: decimal.NewFromInt(100),
			},
		},
	})
	if err != nil {
		t.Fatalf("CreateSalesInvoice(MAC): %v", err)
	}

	var invOutbox models.PubSubMessageRecord
	if err := db.WithContext(ctx).
		Where("business_id = ? AND reference_type = ? AND reference_id = ? AND action = ?",
			businessID, models.AccountReferenceTypeInvoice, saleInvoice.ID, models.PubSubMessageActionCreate).
		Order("id DESC").
		First(&invOutbox).Error; err != nil {
		t.Fatalf("expected outbox record for invoice: %v", err)
	}
	invMsg := models.ConvertToPubSubMessage(invOutbox)
	wtxInv := db.Begin()
	if err := workflow.ProcessInvoiceWorkflow(wtxInv, logger, invMsg); err != nil {
		t.Fatalf("ProcessInvoiceWorkflow: %v", err)
	}
	if err := wtxInv.Commit().Error; err != nil {
		t.Fatalf("invoice workflow commit: %v", err)
	}

	// Assert valuation invoice qty is -1.
	from := models.MyDateString(time.Date(2025, 12, 31, 0, 0, 0, 0, time.UTC))
	to := models.MyDateString(time.Date(2026, 1, 31, 0, 0, 0, 0, time.UTC))
	val, err := models.GetInventoryValuation(ctx, from, to, mac.ID, models.ProductTypeSingle, primary.ID)
	if err != nil {
		t.Fatalf("GetInventoryValuation: %v", err)
	}
	invoiceDesc := "Invoice #" + saleInvoice.InvoiceNumber
	var invoiceQty decimal.Decimal
	for _, d := range val.Details {
		if d != nil && d.TransactionDescription == invoiceDesc {
			invoiceQty = invoiceQty.Add(d.Qty)
		}
	}
	if invoiceQty.Cmp(decimal.NewFromInt(-1)) != 0 {
		t.Fatalf("expected invoice qty=-1 in valuation details; got %s (desc=%q)", invoiceQty.String(), invoiceDesc)
	}
}

func TestInventoryValuation_Order_InvoiceThenAdjustment_DoesNotBecomeMinusTwo(t *testing.T) {
	if strings.TrimSpace(os.Getenv("INTEGRATION_TESTS")) == "" {
		t.Skip("set INTEGRATION_TESTS=1 to run integration tests (requires docker)")
	}

	ctx := context.Background()

	redisName, redisPort := startRedisContainer(t)
	t.Cleanup(func() { _ = dockerRmForce(redisName) })

	mysqlName, mysqlPort := startMySQLContainer(t)
	t.Cleanup(func() { _ = dockerRmForce(mysqlName) })

	// Wire env for config.Connect* helpers.
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
		Name:  "Test Biz",
		Email: "owner@test.local",
	})
	if err != nil {
		t.Fatalf("CreateBusiness: %v", err)
	}
	businessID := biz.ID.String()
	ctx = utils.SetBusinessIdInContext(ctx, businessID)

	db := config.GetDB()
	// Relax transaction lock dates for historical postings.
	relaxDate := time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)
	if err := db.WithContext(ctx).Model(&models.Business{}).Where("id = ?", biz.ID).Updates(map[string]interface{}{
		"MigrationDate":                 relaxDate,
		"SalesTransactionLockDate":      relaxDate,
		"PurchaseTransactionLockDate":   relaxDate,
		"BankingTransactionLockDate":    relaxDate,
		"AccountantTransactionLockDate": relaxDate,
	}).Error; err != nil {
		t.Fatalf("relax business lock dates: %v", err)
	}
	var primary models.Warehouse
	if err := db.WithContext(ctx).Where("business_id = ? AND name = ?", businessID, "Primary Warehouse").First(&primary).Error; err != nil {
		t.Fatalf("fetch primary warehouse: %v", err)
	}

	reason, err := models.CreateReason(ctx, &models.NewReason{Name: "Damage"})
	if err != nil {
		t.Fatalf("CreateReason: %v", err)
	}

	unit, err := models.CreateProductUnit(ctx, &models.NewProductUnit{Name: "Pcs", Abbreviation: "pc", Precision: models.PrecisionZero})
	if err != nil {
		t.Fatalf("CreateProductUnit: %v", err)
	}

	sysAccounts, err := models.GetSystemAccounts(businessID)
	if err != nil {
		t.Fatalf("GetSystemAccounts: %v", err)
	}
	invAcc := sysAccounts[models.AccountCodeInventoryAsset]
	salesAcc := sysAccounts[models.AccountCodeSales]
	cogsAcc := sysAccounts[models.AccountCodeCostOfGoodsSold]
	if invAcc == 0 || salesAcc == 0 || cogsAcc == 0 {
		t.Fatalf("missing required system accounts (inv=%d sales=%d cogs=%d)", invAcc, salesAcc, cogsAcc)
	}

	// Product "abc": opening 10 @ 100 to match screenshots.
	abc, err := models.CreateProduct(ctx, &models.NewProduct{
		Name:               "abc",
		Sku:                "ABC-001",
		Barcode:            "ABC-001",
		UnitId:             unit.ID,
		SalesAccountId:     salesAcc,
		PurchaseAccountId:  cogsAcc,
		InventoryAccountId: invAcc,
		IsBatchTracking:    utils.NewFalse(),
		OpeningStocks: []models.NewOpeningStock{
			{WarehouseId: primary.ID, Qty: decimal.NewFromInt(10), UnitValue: decimal.NewFromInt(100)},
		},
	})
	if err != nil {
		t.Fatalf("CreateProduct(abc): %v", err)
	}

	customer, err := models.CreateCustomer(ctx, &models.NewCustomer{
		Name:                 "customer3",
		Email:                "customer3@test.local",
		CurrencyId:           biz.BaseCurrencyId,
		ExchangeRate:         decimal.NewFromInt(1),
		CustomerPaymentTerms: models.PaymentTermsDueOnReceipt,
	})
	if err != nil {
		t.Fatalf("CreateCustomer: %v", err)
	}

	logger := logrus.New()
	// Match UI scenario date.
	invDate := time.Date(2026, 1, 15, 12, 0, 0, 0, time.UTC)

	// Invoice first qty 1.
	isTaxInclusive := false
	saleInvoice, err := models.CreateSalesInvoice(ctx, &models.NewSalesInvoice{
		CustomerId:          customer.ID,
		BranchId:            biz.PrimaryBranchId,
		InvoiceDate:         invDate,
		InvoicePaymentTerms: models.PaymentTermsDueOnReceipt,
		CurrencyId:          biz.BaseCurrencyId,
		ExchangeRate:        decimal.NewFromInt(1),
		WarehouseId:         primary.ID,
		IsTaxInclusive:      &isTaxInclusive,
		CurrentStatus:       models.SalesInvoiceStatusConfirmed,
		Details: []models.NewSalesInvoiceDetail{
			{
				ProductId:      abc.ID,
				ProductType:    models.ProductTypeSingle,
				BatchNumber:    "",
				Name:           "abc",
				Description:    "",
				DetailQty:      decimal.NewFromInt(1),
				DetailUnitRate: decimal.NewFromInt(100),
			},
		},
	})
	if err != nil {
		t.Fatalf("CreateSalesInvoice(abc): %v", err)
	}

	var invOutbox models.PubSubMessageRecord
	if err := db.WithContext(ctx).
		Where("business_id = ? AND reference_type = ? AND reference_id = ? AND action = ?",
			businessID, models.AccountReferenceTypeInvoice, saleInvoice.ID, models.PubSubMessageActionCreate).
		Order("id DESC").
		First(&invOutbox).Error; err != nil {
		t.Fatalf("expected outbox record for invoice: %v", err)
	}
	invMsg := models.ConvertToPubSubMessage(invOutbox)
	wtxInv := db.Begin()
	if err := workflow.ProcessInvoiceWorkflow(wtxInv, logger, invMsg); err != nil {
		t.Fatalf("ProcessInvoiceWorkflow: %v", err)
	}
	if err := wtxInv.Commit().Error; err != nil {
		t.Fatalf("invoice workflow commit: %v", err)
	}

	// Active invoice movements should be 1 at this point.
	invDetailID := 0
	if len(saleInvoice.Details) > 0 {
		invDetailID = saleInvoice.Details[0].ID
	}
	if invDetailID == 0 {
		t.Fatalf("expected invoice detail id to be populated")
	}
	var beforeCount int64
	if err := db.WithContext(ctx).Model(&models.StockHistory{}).
		Where("business_id = ? AND reference_type = ? AND reference_id = ? AND reference_detail_id = ? AND is_reversal = 0 AND reversed_by_stock_history_id IS NULL",
			businessID, models.StockReferenceTypeInvoice, saleInvoice.ID, invDetailID).
		Count(&beforeCount).Error; err != nil {
		t.Fatalf("count active invoice stock_histories(before): %v", err)
	}
	if beforeCount != 1 {
		t.Fatalf("expected 1 active invoice stock_history before adjustment; got %d", beforeCount)
	}

	// Then adjustment (-1 @ 100).
	ia, err := models.CreateInventoryAdjustment(ctx, &models.NewInventoryAdjustment{
		ReferenceNumber: "IA-77",
		AdjustmentType:  models.InventoryAdjustmentTypeQuantity,
		AdjustmentDate:  invDate,
		AccountId:       cogsAcc,
		BranchId:        biz.PrimaryBranchId,
		WarehouseId:     primary.ID,
		CurrentStatus:   models.InventoryAdjustmentStatusAdjusted,
		ReasonId:        reason.ID,
		Description:     "Damage",
		Details: []models.NewInventoryAdjustmentDetail{
			{
				ProductId:     abc.ID,
				ProductType:   models.ProductTypeSingle,
				BatchNumber:   "",
				Name:          "abc",
				AdjustedValue: decimal.NewFromInt(-1),
				CostPrice:     decimal.NewFromInt(100),
			},
		},
	})
	if err != nil {
		t.Fatalf("CreateInventoryAdjustment(abc): %v", err)
	}

	var iaOutbox models.PubSubMessageRecord
	if err := db.WithContext(ctx).
		Where("business_id = ? AND reference_type = ? AND reference_id = ? AND action = ?",
			businessID, models.AccountReferenceTypeInventoryAdjustmentQuantity, ia.ID, models.PubSubMessageActionCreate).
		Order("id DESC").
		First(&iaOutbox).Error; err != nil {
		t.Fatalf("expected outbox record for inventory adjustment: %v", err)
	}
	iaMsg := models.ConvertToPubSubMessage(iaOutbox)
	wtxIA := db.Begin()
	if err := workflow.ProcessInventoryAdjustmentQuantityWorkflow(wtxIA, logger, iaMsg); err != nil {
		t.Fatalf("ProcessInventoryAdjustmentQuantityWorkflow: %v", err)
	}
	if err := wtxIA.Commit().Error; err != nil {
		t.Fatalf("adjustment workflow commit: %v", err)
	}

	// This is the bug: invoice becomes -2 after adjustment. Assert we never have >1 active movement.
	var afterCount int64
	if err := db.WithContext(ctx).Model(&models.StockHistory{}).
		Where("business_id = ? AND reference_type = ? AND reference_id = ? AND reference_detail_id = ? AND is_reversal = 0 AND reversed_by_stock_history_id IS NULL",
			businessID, models.StockReferenceTypeInvoice, saleInvoice.ID, invDetailID).
		Count(&afterCount).Error; err != nil {
		t.Fatalf("count active invoice stock_histories(after): %v", err)
	}
	if afterCount != 1 {
		t.Fatalf("expected 1 active invoice stock_history after adjustment; got %d", afterCount)
	}

	from := models.MyDateString(time.Date(2025, 12, 31, 0, 0, 0, 0, time.UTC))
	to := models.MyDateString(time.Date(2026, 1, 31, 0, 0, 0, 0, time.UTC))
	val, err := models.GetInventoryValuation(ctx, from, to, abc.ID, models.ProductTypeSingle, primary.ID)
	if err != nil {
		t.Fatalf("GetInventoryValuation: %v", err)
	}

	invoiceDesc := "Invoice #" + saleInvoice.InvoiceNumber
	var invoiceQty decimal.Decimal
	for _, d := range val.Details {
		if d != nil && d.TransactionDescription == invoiceDesc {
			invoiceQty = invoiceQty.Add(d.Qty)
		}
	}
	if invoiceQty.Cmp(decimal.NewFromInt(-1)) != 0 {
		t.Fatalf("expected invoice qty=-1 in valuation details; got %s (desc=%q)", invoiceQty.String(), invoiceDesc)
	}
}

