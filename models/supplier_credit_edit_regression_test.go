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

// Regression: editing/saving a Supplier Credit must not duplicate detail rows or inventory ledger rows.
//
// Bug symptom (UI): SupplierCredit #SC-1 shows multiple qty rows (e.g. -2 and -4) after edit-save,
// which implies supplier_credit_details were duplicated (old detail not updated) and valuation double-counts.
func TestSupplierCredit_EditDoesNotDuplicateDetailsOrInventoryValuation(t *testing.T) {
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
	// Match many production deployments: stock commands feature flag disabled.
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

	unit, err := models.CreateProductUnit(ctx, &models.NewProductUnit{Name: "Pcs", Abbreviation: "pc", Precision: models.PrecisionZero})
	if err != nil {
		t.Fatalf("CreateProductUnit: %v", err)
	}

	sysAccounts, err := models.GetSystemAccounts(businessID)
	if err != nil {
		t.Fatalf("GetSystemAccounts: %v", err)
	}
	invAcc := sysAccounts[models.AccountCodeInventoryAsset]
	purchaseAcc := sysAccounts[models.AccountCodeCostOfGoodsSold]
	if invAcc == 0 || purchaseAcc == 0 {
		t.Fatalf("missing required system accounts (inv=%d purchase=%d)", invAcc, purchaseAcc)
	}

	product, err := models.CreateProduct(ctx, &models.NewProduct{
		Name:               "Apple cover",
		Sku:                "APPLE-COVER-001",
		Barcode:            "APPLE-COVER-001",
		UnitId:             unit.ID,
		SalesAccountId:     sysAccounts[models.AccountCodeSales],
		PurchaseAccountId:  purchaseAcc,
		InventoryAccountId: invAcc,
		IsBatchTracking:    utils.NewFalse(),
		OpeningStocks:      nil, // stock comes from Bill
	})
	if err != nil {
		t.Fatalf("CreateProduct: %v", err)
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

	// 1) Create a Bill to bring stock in (qty +10).
	docDate := time.Date(2026, 1, 15, 12, 0, 0, 0, time.UTC)
	isTaxInclusive := false
	bill, err := models.CreateBill(ctx, &models.NewBill{
		SupplierId:        supplier.ID,
		BranchId:          biz.PrimaryBranchId,
		BillDate:          docDate,
		BillPaymentTerms:  models.PaymentTermsDueOnReceipt,
		CurrencyId:        biz.BaseCurrencyId,
		ExchangeRate:      decimal.NewFromInt(1),
		WarehouseId:       primary.ID,
		IsTaxInclusive:    &isTaxInclusive,
		CurrentStatus:     models.BillStatusConfirmed,
		ReferenceNumber:   "BL-1",
		Details: []models.NewBillDetail{{
			ProductId:      product.ID,
			ProductType:    models.ProductTypeSingle,
			BatchNumber:    "",
			Name:           "Apple cover",
			Description:    "",
			DetailAccountId: purchaseAcc,
			DetailQty:      decimal.NewFromInt(10),
			DetailUnitRate: decimal.NewFromInt(2000),
		}},
	})
	if err != nil {
		t.Fatalf("CreateBill: %v", err)
	}

	logger := logrus.New()

	var billOutbox models.PubSubMessageRecord
	if err := db.WithContext(ctx).
		Where("business_id = ? AND reference_type = ? AND reference_id = ? AND action = ?",
			businessID, models.AccountReferenceTypeBill, bill.ID, models.PubSubMessageActionCreate).
		Order("id DESC").
		First(&billOutbox).Error; err != nil {
		t.Fatalf("expected outbox record for bill: %v", err)
	}
	billMsg := models.ConvertToPubSubMessage(billOutbox)
	wtx := db.Begin()
	if err := workflow.ProcessBillWorkflow(wtx, logger, billMsg); err != nil {
		t.Fatalf("ProcessBillWorkflow: %v", err)
	}
	if err := wtx.Commit().Error; err != nil {
		t.Fatalf("bill workflow commit: %v", err)
	}

	// 2) Create Supplier Credit qty 2 (outgoing / return to supplier).
	sc, err := models.CreateSupplierCredit(ctx, &models.NewSupplierCredit{
		SupplierId:        supplier.ID,
		BranchId:          biz.PrimaryBranchId,
		WarehouseId:       primary.ID,
		ReferenceNumber:   "SC-1",
		SupplierCreditDate: docDate,
		CurrencyId:        biz.BaseCurrencyId,
		ExchangeRate:      decimal.NewFromInt(1),
		IsTaxInclusive:    &isTaxInclusive,
		CurrentStatus:     models.SupplierCreditStatusConfirmed,
		Details: []models.NewSupplierCreditDetail{{
			ProductId:       product.ID,
			ProductType:     models.ProductTypeSingle,
			BatchNumber:     "",
			Name:            "Apple cover",
			Description:     "",
			DetailAccountId: purchaseAcc,
			DetailQty:       decimal.NewFromInt(2),
			DetailUnitRate:  decimal.NewFromInt(2000),
		}},
	})
	if err != nil {
		t.Fatalf("CreateSupplierCredit: %v", err)
	}

	var createOutbox models.PubSubMessageRecord
	if err := db.WithContext(ctx).
		Where("business_id = ? AND reference_type = ? AND reference_id = ? AND action = ?",
			businessID, models.AccountReferenceTypeSupplierCredit, sc.ID, models.PubSubMessageActionCreate).
		Order("id DESC").
		First(&createOutbox).Error; err != nil {
		t.Fatalf("expected outbox record for supplier credit create: %v", err)
	}
	createMsg := models.ConvertToPubSubMessage(createOutbox)

	// Process create message twice (at-least-once delivery).
	for i := 0; i < 2; i++ {
		wtxc := db.Begin()
		if err := workflow.ProcessSupplierCreditWorkflow(wtxc, logger, createMsg); err != nil {
			t.Fatalf("ProcessSupplierCreditWorkflow(create #%d): %v", i+1, err)
		}
		if err := wtxc.Commit().Error; err != nil {
			t.Fatalf("supplier credit create workflow commit #%d: %v", i+1, err)
		}
	}

	// Reload Supplier Credit with details (need stable detail_id for edit).
	var scReload models.SupplierCredit
	if err := db.WithContext(ctx).Preload("Details").First(&scReload, sc.ID).Error; err != nil {
		t.Fatalf("reload supplier credit: %v", err)
	}
	if len(scReload.Details) != 1 {
		t.Fatalf("expected 1 supplier credit detail after create; got %d", len(scReload.Details))
	}
	detailID := scReload.Details[0].ID

	assertDetailCountAndStockSum := func(step string, wantDetailCount int64, wantSumQty decimal.Decimal) {
		var detailCountRow struct {
			Cnt int64 `gorm:"column:cnt"`
		}
		if err := db.WithContext(ctx).
			Raw("SELECT COUNT(*) AS cnt FROM supplier_credit_details WHERE supplier_credit_id = ?", sc.ID).
			Scan(&detailCountRow).Error; err != nil {
			t.Fatalf("%s: count supplier_credit_details: %v", step, err)
		}
		t.Logf("%s SQL: SELECT COUNT(*) FROM supplier_credit_details WHERE supplier_credit_id=%d => %d", step, sc.ID, detailCountRow.Cnt)
		if detailCountRow.Cnt != wantDetailCount {
			t.Fatalf("%s: expected detail_count=%d; got %d", step, wantDetailCount, detailCountRow.Cnt)
		}

		var sumRow struct {
			SumQty decimal.Decimal `gorm:"column:sum_qty"`
		}
		if err := db.WithContext(ctx).
			Raw(`SELECT COALESCE(SUM(qty), 0) AS sum_qty
			     FROM stock_histories
			     WHERE business_id = ?
			       AND reference_type = ?
			       AND reference_id = ?
			       AND is_reversal = 0
			       AND reversed_by_stock_history_id IS NULL`, businessID, models.StockReferenceTypeSupplierCredit, sc.ID).
			Scan(&sumRow).Error; err != nil {
			t.Fatalf("%s: sum stock_histories qty: %v", step, err)
		}
		t.Logf("%s SQL: SUM(stock_histories.qty) WHERE ref=SC#%d => %s", step, sc.ID, sumRow.SumQty.String())
		if sumRow.SumQty.Cmp(wantSumQty) != 0 {
			t.Fatalf("%s: expected sum_qty=%s; got %s", step, wantSumQty.String(), sumRow.SumQty.String())
		}
	}

	assertDetailCountAndStockSum("after create", 1, decimal.NewFromInt(-2))

	// 3) Edit Supplier Credit: change qty 2 -> 4, process update workflow.
	updated, err := models.UpdateSupplierCredit(ctx, sc.ID, &models.NewSupplierCredit{
		SupplierId:        supplier.ID,
		BranchId:          biz.PrimaryBranchId,
		WarehouseId:       primary.ID,
		ReferenceNumber:   "SC-1",
		SupplierCreditDate: docDate,
		CurrencyId:        biz.BaseCurrencyId,
		ExchangeRate:      decimal.NewFromInt(1),
		IsTaxInclusive:    &isTaxInclusive,
		CurrentStatus:     models.SupplierCreditStatusConfirmed,
		Details: []models.NewSupplierCreditDetail{{
			DetailId:        detailID,
			ProductId:       product.ID,
			ProductType:     models.ProductTypeSingle,
			BatchNumber:     "",
			Name:            "Apple cover",
			Description:     "",
			DetailAccountId: purchaseAcc,
			DetailQty:       decimal.NewFromInt(4),
			DetailUnitRate:  decimal.NewFromInt(2000),
		}},
	})
	if err != nil {
		t.Fatalf("UpdateSupplierCredit: %v", err)
	}
	if updated == nil {
		t.Fatalf("UpdateSupplierCredit returned nil")
	}

	var updateOutbox models.PubSubMessageRecord
	if err := db.WithContext(ctx).
		Where("business_id = ? AND reference_type = ? AND reference_id = ? AND action = ?",
			businessID, models.AccountReferenceTypeSupplierCredit, sc.ID, models.PubSubMessageActionUpdate).
		Order("id DESC").
		First(&updateOutbox).Error; err != nil {
		t.Fatalf("expected outbox record for supplier credit update: %v", err)
	}
	updateMsg := models.ConvertToPubSubMessage(updateOutbox)
	wtxu := db.Begin()
	if err := workflow.ProcessSupplierCreditWorkflow(wtxu, logger, updateMsg); err != nil {
		t.Fatalf("ProcessSupplierCreditWorkflow(update): %v", err)
	}
	if err := wtxu.Commit().Error; err != nil {
		t.Fatalf("supplier credit update workflow commit: %v", err)
	}

	assertDetailCountAndStockSum("after update qty 4", 1, decimal.NewFromInt(-4))

	// 4) Save again with the same values (idempotent edit); must not change totals or duplicate rows.
	updated2, err := models.UpdateSupplierCredit(ctx, sc.ID, &models.NewSupplierCredit{
		SupplierId:        supplier.ID,
		BranchId:          biz.PrimaryBranchId,
		WarehouseId:       primary.ID,
		ReferenceNumber:   "SC-1",
		SupplierCreditDate: docDate,
		CurrencyId:        biz.BaseCurrencyId,
		ExchangeRate:      decimal.NewFromInt(1),
		IsTaxInclusive:    &isTaxInclusive,
		CurrentStatus:     models.SupplierCreditStatusConfirmed,
		Details: []models.NewSupplierCreditDetail{{
			DetailId:        detailID,
			ProductId:       product.ID,
			ProductType:     models.ProductTypeSingle,
			BatchNumber:     "",
			Name:            "Apple cover",
			Description:     "",
			DetailAccountId: purchaseAcc,
			DetailQty:       decimal.NewFromInt(4),
			DetailUnitRate:  decimal.NewFromInt(2000),
		}},
	})
	if err != nil {
		t.Fatalf("UpdateSupplierCredit(2): %v", err)
	}
	if updated2 == nil {
		t.Fatalf("UpdateSupplierCredit(2) returned nil")
	}

	var updateOutbox2 models.PubSubMessageRecord
	if err := db.WithContext(ctx).
		Where("business_id = ? AND reference_type = ? AND reference_id = ? AND action = ?",
			businessID, models.AccountReferenceTypeSupplierCredit, sc.ID, models.PubSubMessageActionUpdate).
		Order("id DESC").
		First(&updateOutbox2).Error; err != nil {
		t.Fatalf("expected outbox record for supplier credit update (2): %v", err)
	}
	updateMsg2 := models.ConvertToPubSubMessage(updateOutbox2)
	wtxu2 := db.Begin()
	if err := workflow.ProcessSupplierCreditWorkflow(wtxu2, logger, updateMsg2); err != nil {
		t.Fatalf("ProcessSupplierCreditWorkflow(update 2): %v", err)
	}
	if err := wtxu2.Commit().Error; err != nil {
		t.Fatalf("supplier credit update workflow commit (2): %v", err)
	}

	assertDetailCountAndStockSum("after update save again", 1, decimal.NewFromInt(-4))
}

