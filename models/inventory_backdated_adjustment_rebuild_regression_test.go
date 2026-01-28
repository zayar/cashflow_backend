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

// Regression: backdated inventory adjustment (outgoing) must rebuild FIFO so later invoices reprice COGS.
func TestInventoryValuation_BackdatedAdjustment_RepricesInvoice(t *testing.T) {
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
		Name:  "Test Biz",
		Email: "owner@test.local",
	})
	if err != nil {
		t.Fatalf("CreateBusiness: %v", err)
	}
	businessID := biz.ID.String()
	ctx = utils.SetBusinessIdInContext(ctx, businessID)

	db := config.GetDB()
	// Seed opening stock on 31 Dec 2025 to match scenario.
	migrationDate := time.Date(2025, 12, 31, 0, 0, 0, 0, time.UTC)
	relaxDate := time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)
	if err := db.WithContext(ctx).Model(&models.Business{}).Where("id = ?", biz.ID).Updates(map[string]interface{}{
		"MigrationDate":                 migrationDate,
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

	mango, err := models.CreateProduct(ctx, &models.NewProduct{
		Name:               "Mango",
		Sku:                "MANGO-001",
		Barcode:            "MANGO-001",
		UnitId:             unit.ID,
		SalesAccountId:     salesAcc,
		PurchaseAccountId:  cogsAcc,
		InventoryAccountId: invAcc,
		PurchasePrice:      decimal.NewFromInt(1700),
		IsBatchTracking:    utils.NewFalse(),
		OpeningStocks: []models.NewOpeningStock{
			{WarehouseId: primary.ID, Qty: decimal.NewFromInt(1), UnitValue: decimal.NewFromInt(1700)},
		},
	})
	if err != nil {
		t.Fatalf("CreateProduct: %v", err)
	}

	// Process opening stock outbox so ledger exists.
	var posOutbox models.PubSubMessageRecord
	if err := db.WithContext(ctx).
		Where("business_id = ? AND reference_type = ? AND reference_id = ? AND action = ?",
			businessID, models.AccountReferenceTypeProductOpeningStock, mango.ID, models.PubSubMessageActionCreate).
		Order("id DESC").
		First(&posOutbox).Error; err != nil {
		t.Fatalf("expected outbox record for product opening stock: %v", err)
	}
	logger := logrus.New()
	wtxPOS := db.Begin()
	if err := workflow.ProcessProductOpeningStockWorkflow(wtxPOS, logger, models.ConvertToPubSubMessage(posOutbox)); err != nil {
		t.Fatalf("ProcessProductOpeningStockWorkflow: %v", err)
	}
	if err := wtxPOS.Commit().Error; err != nil {
		t.Fatalf("opening stock workflow commit: %v", err)
	}

	supplier, err := models.CreateSupplier(ctx, &models.NewSupplier{
		Name:                 "Supplier A",
		Email:                "supplier@test.local",
		CurrencyId:           biz.BaseCurrencyId,
		ExchangeRate:         decimal.NewFromInt(1),
		SupplierPaymentTerms: models.PaymentTermsDueOnReceipt,
	})
	if err != nil {
		t.Fatalf("CreateSupplier: %v", err)
	}

	customer, err := models.CreateCustomer(ctx, &models.NewCustomer{
		Name:                 "Customer A",
		Email:                "customer@test.local",
		CurrencyId:           biz.BaseCurrencyId,
		ExchangeRate:         decimal.NewFromInt(1),
		CustomerPaymentTerms: models.PaymentTermsDueOnReceipt,
	})
	if err != nil {
		t.Fatalf("CreateCustomer: %v", err)
	}

	isTaxInclusive := false

	// Bill on 03 Jan 2026: +2 @ 1,800
	billDate := time.Date(2026, 1, 3, 12, 0, 0, 0, time.UTC)
	bill, err := models.CreateBill(ctx, &models.NewBill{
		SupplierId:       supplier.ID,
		BranchId:         biz.PrimaryBranchId,
		BillDate:         billDate,
		BillPaymentTerms: models.PaymentTermsDueOnReceipt,
		CurrencyId:       biz.BaseCurrencyId,
		ExchangeRate:     decimal.NewFromInt(1),
		WarehouseId:      primary.ID,
		IsTaxInclusive:   &isTaxInclusive,
		CurrentStatus:    models.BillStatusConfirmed,
		ReferenceNumber:  "BL-3",
		Details: []models.NewBillDetail{{
			ProductId:      mango.ID,
			ProductType:    models.ProductTypeSingle,
			BatchNumber:    "",
			Name:           "Mango",
			Description:    "",
			DetailAccountId: cogsAcc,
			DetailQty:      decimal.NewFromInt(2),
			DetailUnitRate: decimal.NewFromInt(1800),
		}},
	})
	if err != nil {
		t.Fatalf("CreateBill: %v", err)
	}
	var billOutbox models.PubSubMessageRecord
	if err := db.WithContext(ctx).
		Where("business_id = ? AND reference_type = ? AND reference_id = ? AND action = ?",
			businessID, models.AccountReferenceTypeBill, bill.ID, models.PubSubMessageActionCreate).
		Order("id DESC").
		First(&billOutbox).Error; err != nil {
		t.Fatalf("expected outbox record for bill: %v", err)
	}
	wtxBL := db.Begin()
	if err := workflow.ProcessBillWorkflow(wtxBL, logger, models.ConvertToPubSubMessage(billOutbox)); err != nil {
		t.Fatalf("ProcessBillWorkflow: %v", err)
	}
	if err := wtxBL.Commit().Error; err != nil {
		t.Fatalf("bill workflow commit: %v", err)
	}

	// Invoice on 08 Jan 2026: -2
	invDate := time.Date(2026, 1, 8, 12, 0, 0, 0, time.UTC)
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
		Details: []models.NewSalesInvoiceDetail{{
			ProductId:      mango.ID,
			ProductType:    models.ProductTypeSingle,
			BatchNumber:    "",
			Name:           "Mango",
			Description:    "",
			DetailQty:      decimal.NewFromInt(2),
			DetailUnitRate: decimal.NewFromInt(2500),
		}},
	})
	if err != nil {
		t.Fatalf("CreateSalesInvoice: %v", err)
	}
	var invOutbox models.PubSubMessageRecord
	if err := db.WithContext(ctx).
		Where("business_id = ? AND reference_type = ? AND reference_id = ? AND action = ?",
			businessID, models.AccountReferenceTypeInvoice, saleInvoice.ID, models.PubSubMessageActionCreate).
		Order("id DESC").
		First(&invOutbox).Error; err != nil {
		t.Fatalf("expected outbox record for invoice: %v", err)
	}
	wtxINV := db.Begin()
	if err := workflow.ProcessInvoiceWorkflow(wtxINV, logger, models.ConvertToPubSubMessage(invOutbox)); err != nil {
		t.Fatalf("ProcessInvoiceWorkflow: %v", err)
	}
	if err := wtxINV.Commit().Error; err != nil {
		t.Fatalf("invoice workflow commit: %v", err)
	}

	// Backdated inventory adjustment on 05 Jan 2026: -1 @ 1,700
	adjDate := time.Date(2026, 1, 5, 12, 0, 0, 0, time.UTC)
	adj, err := models.CreateInventoryAdjustment(ctx, &models.NewInventoryAdjustment{
		ReferenceNumber: "IA-1",
		AdjustmentType:  models.InventoryAdjustmentTypeQuantity,
		AdjustmentDate:  adjDate,
		AccountId:       cogsAcc,
		BranchId:        biz.PrimaryBranchId,
		WarehouseId:     primary.ID,
		CurrentStatus:   models.InventoryAdjustmentStatusAdjusted,
		ReasonId:        reason.ID,
		Description:     "Backdated adjustment",
		Details: []models.NewInventoryAdjustmentDetail{{
			ProductId:     mango.ID,
			ProductType:   models.ProductTypeSingle,
			BatchNumber:   "",
			Name:          "Mango",
			AdjustedValue: decimal.NewFromInt(-1),
			CostPrice:     decimal.NewFromInt(1700),
		}},
	})
	if err != nil {
		t.Fatalf("CreateInventoryAdjustment: %v", err)
	}
	var adjOutbox models.PubSubMessageRecord
	if err := db.WithContext(ctx).
		Where("business_id = ? AND reference_type = ? AND reference_id = ? AND action = ?",
			businessID, models.AccountReferenceTypeInventoryAdjustmentQuantity, adj.ID, models.PubSubMessageActionCreate).
		Order("id DESC").
		First(&adjOutbox).Error; err != nil {
		t.Fatalf("expected outbox record for inventory adjustment: %v", err)
	}
	wtxAdj := db.Begin()
	if err := workflow.ProcessInventoryAdjustmentQuantityWorkflow(wtxAdj, logger, models.ConvertToPubSubMessage(adjOutbox)); err != nil {
		t.Fatalf("ProcessInventoryAdjustmentQuantityWorkflow: %v", err)
	}
	if err := wtxAdj.Commit().Error; err != nil {
		t.Fatalf("adjustment workflow commit: %v", err)
	}

	// Inventory valuation should end at zero qty and zero value.
	from := models.MyDateString(time.Date(2025, 12, 31, 0, 0, 0, 0, time.UTC))
	to := models.MyDateString(time.Date(2026, 1, 31, 0, 0, 0, 0, time.UTC))
	val, err := models.GetInventoryValuation(ctx, from, to, mango.ID, models.ProductTypeSingle, primary.ID)
	if err != nil {
		t.Fatalf("GetInventoryValuation: %v", err)
	}
	if !val.ClosingStockOnHand.IsZero() {
		t.Fatalf("expected closing stock=0; got %s", val.ClosingStockOnHand.String())
	}
	if !val.ClosingAssetValue.IsZero() {
		t.Fatalf("expected closing asset value=0; got %s", val.ClosingAssetValue.String())
	}

	// Invoice allocations must be re-priced to 1,800 (total COGS 3,600).
	var invoiceStocks []models.StockHistory
	if err := db.WithContext(ctx).
		Where("business_id = ? AND reference_type = ? AND reference_id = ? AND is_outgoing = 1 AND is_reversal = 0 AND reversed_by_stock_history_id IS NULL",
			businessID, models.StockReferenceTypeInvoice, saleInvoice.ID).
		Order("id ASC").
		Find(&invoiceStocks).Error; err != nil {
		t.Fatalf("load invoice stock_histories: %v", err)
	}
	if len(invoiceStocks) == 0 {
		t.Fatalf("expected invoice stock_histories to exist")
	}
	sumQty := decimal.NewFromInt(0)
	sumValue := decimal.NewFromInt(0)
	distinctUnitCosts := make(map[string]struct{})
	for _, sh := range invoiceStocks {
		sumQty = sumQty.Add(sh.Qty)
		sumValue = sumValue.Add(sh.Qty.Mul(sh.BaseUnitValue))
		distinctUnitCosts[sh.BaseUnitValue.String()] = struct{}{}
	}
	if sumQty.Cmp(decimal.NewFromInt(-2)) != 0 {
		t.Fatalf("expected invoice qty=-2; got %s", sumQty.String())
	}
	if sumValue.Cmp(decimal.NewFromInt(-3600)) != 0 {
		t.Fatalf("expected invoice total COGS=-3600; got %s", sumValue.String())
	}
	if len(distinctUnitCosts) != 1 || invoiceStocks[0].BaseUnitValue.Cmp(decimal.NewFromInt(1800)) != 0 {
		t.Fatalf("expected invoice unit cost=1800; got %v", distinctUnitCosts)
	}

	// Invoice detail COGS should be updated.
	var invDetails []models.SalesInvoiceDetail
	if err := db.WithContext(ctx).Where("sales_invoice_id = ?", saleInvoice.ID).Find(&invDetails).Error; err != nil {
		t.Fatalf("load invoice details: %v", err)
	}
	if len(invDetails) != 1 {
		t.Fatalf("expected 1 invoice detail; got %d", len(invDetails))
	}
	if invDetails[0].Cogs.Cmp(decimal.NewFromInt(3600)) != 0 {
		t.Fatalf("expected invoice detail cogs=3600; got %s", invDetails[0].Cogs.String())
	}

	// Invoice journal should reflect re-priced valuation lines.
	var journal models.AccountJournal
	if err := db.WithContext(ctx).
		Preload("AccountTransactions").
		Where("business_id = ? AND reference_type = ? AND reference_id = ? AND is_reversal = 0 AND (reversed_by_journal_id IS NULL OR reversed_by_journal_id = 0)",
			businessID, models.AccountReferenceTypeInvoice, saleInvoice.ID).
		First(&journal).Error; err != nil {
		t.Fatalf("load invoice journal: %v", err)
	}
	cogsDebit := decimal.NewFromInt(0)
	invCredit := decimal.NewFromInt(0)
	for _, tx := range journal.AccountTransactions {
		if tx.IsInventoryValuation == nil || !*tx.IsInventoryValuation {
			continue
		}
		if tx.AccountId == cogsAcc {
			cogsDebit = cogsDebit.Add(tx.BaseDebit)
		}
		if tx.AccountId == invAcc {
			invCredit = invCredit.Add(tx.BaseCredit)
		}
	}
	if cogsDebit.Cmp(decimal.NewFromInt(3600)) != 0 {
		t.Fatalf("expected COGS debit=3600; got %s", cogsDebit.String())
	}
	if invCredit.Cmp(decimal.NewFromInt(3600)) != 0 {
		t.Fatalf("expected Inventory credit=3600; got %s", invCredit.String())
	}
}
