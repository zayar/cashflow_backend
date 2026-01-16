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

// Regression: Value Adjustment must be allowed when stock exists (even if last ledger row had stale closing_qty),
// and Quantity Adjustment must remain unaffected.
func TestInventoryAdjustmentValue_WithOnHand_AllowsSaveAndKeepsQty(t *testing.T) {
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
	// Match many production deployments: stock commands disabled.
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

	// Stock exists: 9 @ 100 in Primary.
	philips, err := models.CreateProduct(ctx, &models.NewProduct{
		Name:               "Philips",
		Sku:                "PHILIPS-001",
		Barcode:            "PHILIPS-001",
		UnitId:             unit.ID,
		SalesAccountId:     salesAcc,
		PurchaseAccountId:  cogsAcc,
		InventoryAccountId: invAcc,
		IsBatchTracking:    utils.NewFalse(),
		OpeningStocks: []models.NewOpeningStock{
			{WarehouseId: primary.ID, Qty: decimal.NewFromInt(9), UnitValue: decimal.NewFromInt(100)},
		},
	})
	if err != nil {
		t.Fatalf("CreateProduct: %v", err)
	}

	// Process opening stock outbox so stock_histories exists and is consistent.
	var posOutbox models.PubSubMessageRecord
	if err := db.WithContext(ctx).
		Where("business_id = ? AND reference_type = ? AND reference_id = ? AND action = ?",
			businessID, models.AccountReferenceTypeProductOpeningStock, philips.ID, models.PubSubMessageActionCreate).
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

	// Simulate real production behavior: stock_histories.stock_date can include a non-midnight timestamp.
	// Prior to the fix, Value Adjustment validation used stock_date <= start-of-day and incorrectly treated
	// same-day stock as "ledger missing". This update makes the regression deterministic.
	sameDayStockTs := time.Date(2026, 1, 15, 10, 0, 0, 0, time.UTC)
	res := db.WithContext(ctx).Exec(`
		UPDATE stock_histories
		SET stock_date = ?
		WHERE business_id = ?
		  AND warehouse_id = ?
		  AND product_id = ?
		  AND product_type = ?
	`, sameDayStockTs, businessID, primary.ID, philips.ID, models.ProductTypeSingle)
	if res.Error != nil {
		t.Fatalf("update stock_histories.stock_date: %v", res.Error)
	}
	if res.RowsAffected == 0 {
		t.Fatalf("expected to update at least 1 stock_history row for opening stock; got 0")
	}

	adjDate := time.Date(2026, 1, 15, 12, 0, 0, 0, time.UTC)

	// Value adjustment UI semantics: adjustedValue is NEW UNIT COST.
	// New unit cost = 101 (from opening 100, changed +1).
	ia, err := models.CreateInventoryAdjustment(ctx, &models.NewInventoryAdjustment{
		ReferenceNumber: "IVAV-0001",
		AdjustmentType:  models.InventoryAdjustmentTypeValue,
		AdjustmentDate:  adjDate,
		AccountId:       cogsAcc,
		BranchId:        biz.PrimaryBranchId,
		WarehouseId:     primary.ID,
		CurrentStatus:   models.InventoryAdjustmentStatusAdjusted,
		ReasonId:        reason.ID,
		Description:     "Revalue",
		Details: []models.NewInventoryAdjustmentDetail{
			{
				ProductId:     philips.ID,
				ProductType:   models.ProductTypeSingle,
				BatchNumber:   "",
				Name:          "Philips",
				AdjustedValue: decimal.NewFromInt(101), // new unit cost
				CostPrice:     decimal.NewFromInt(100), // required field; not used by IVAV engine
			},
		},
	})
	if err != nil {
		t.Fatalf("CreateInventoryAdjustment(value): %v", err)
	}

	// Process IVAV outbox via workflow.
	var ivavOutbox models.PubSubMessageRecord
	if err := db.WithContext(ctx).
		Where("business_id = ? AND reference_type = ? AND reference_id = ? AND action = ?",
			businessID, models.AccountReferenceTypeInventoryAdjustmentValue, ia.ID, models.PubSubMessageActionCreate).
		Order("id DESC").
		First(&ivavOutbox).Error; err != nil {
		t.Fatalf("expected outbox record for value adjustment: %v", err)
	}
	wtxIVAV := db.Begin()
	if err := workflow.ProcessInventoryAdjustmentValueWorkflow(wtxIVAV, logger, models.ConvertToPubSubMessage(ivavOutbox)); err != nil {
		t.Fatalf("ProcessInventoryAdjustmentValueWorkflow: %v", err)
	}
	if err := wtxIVAV.Commit().Error; err != nil {
		t.Fatalf("IVAV workflow commit: %v", err)
	}

	// Assert qty unchanged at 9 and asset value becomes 9*101=909 as-of date.
	type sums struct {
		Qty        decimal.Decimal `gorm:"column:qty"`
		AssetValue decimal.Decimal `gorm:"column:asset_value"`
	}
	var s sums
	if err := db.WithContext(ctx).Raw(`
		SELECT
		  COALESCE(SUM(qty), 0) AS qty,
		  COALESCE(SUM(qty * base_unit_value), 0) AS asset_value
		FROM stock_histories
		WHERE business_id = ?
		  AND warehouse_id = ?
		  AND product_id = ?
		  AND product_type = ?
		  AND COALESCE(batch_number,'') = ''
		  AND stock_date <= ?
		  AND is_reversal = 0
		  AND reversed_by_stock_history_id IS NULL
	`, businessID, primary.ID, philips.ID, models.ProductTypeSingle, adjDate).Scan(&s).Error; err != nil {
		t.Fatalf("sum stock_histories: %v", err)
	}
	if s.Qty.Cmp(decimal.NewFromInt(9)) != 0 {
		t.Fatalf("expected qty=9 after value adjustment; got %s", s.Qty.String())
	}
	if s.AssetValue.Round(0).Cmp(decimal.NewFromInt(909)) != 0 {
		t.Fatalf("expected asset_value=909 after value adjustment; got %s", s.AssetValue.String())
	}
}

func TestInventoryAdjustmentValue_ZeroOnHand_StillErrors(t *testing.T) {
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

	p, err := models.CreateProduct(ctx, &models.NewProduct{
		Name:               "NoStock",
		Sku:                "NOSTOCK-001",
		Barcode:            "NOSTOCK-001",
		UnitId:             unit.ID,
		SalesAccountId:     salesAcc,
		PurchaseAccountId:  cogsAcc,
		InventoryAccountId: invAcc,
		IsBatchTracking:    utils.NewFalse(),
		OpeningStocks:      nil,
	})
	if err != nil {
		t.Fatalf("CreateProduct: %v", err)
	}

	adjDate := time.Date(2026, 1, 15, 12, 0, 0, 0, time.UTC)
	_, err = models.CreateInventoryAdjustment(ctx, &models.NewInventoryAdjustment{
		ReferenceNumber: "IVAV-ZERO",
		AdjustmentType:  models.InventoryAdjustmentTypeValue,
		AdjustmentDate:  adjDate,
		AccountId:       cogsAcc,
		BranchId:        biz.PrimaryBranchId,
		WarehouseId:     primary.ID,
		CurrentStatus:   models.InventoryAdjustmentStatusAdjusted,
		ReasonId:        reason.ID,
		Description:     "Revalue",
		Details: []models.NewInventoryAdjustmentDetail{
			{
				ProductId:     p.ID,
				ProductType:   models.ProductTypeSingle,
				BatchNumber:   "",
				Name:          "NoStock",
				AdjustedValue: decimal.NewFromInt(1),
				CostPrice:     decimal.NewFromInt(100),
			},
		},
	})
	if err == nil {
		t.Fatalf("expected error for zero on-hand value adjustment; got nil")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "stock on hand is zero") {
		t.Fatalf("expected 'stock on hand is zero' error; got %v", err)
	}
}

// Regression: eliminate nondeterminism where stock_summaries shows on-hand but stock_histories isn't ready yet
// because Product Opening Stock outbox hasn't been processed.
func TestIVAVReadiness_OpeningStockOutboxRace_BecomesDeterministic(t *testing.T) {
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

	// Create product with opening stock. This updates stock_summaries immediately and publishes outbox,
	// but we intentionally do NOT process the opening stock workflow yet (simulate worker lag).
	p, err := models.CreateProduct(ctx, &models.NewProduct{
		Name:               "RaceItem",
		Sku:                "RACE-001",
		Barcode:            "RACE-001",
		UnitId:             unit.ID,
		SalesAccountId:     salesAcc,
		PurchaseAccountId:  cogsAcc,
		InventoryAccountId: invAcc,
		IsBatchTracking:    utils.NewFalse(),
		OpeningStocks: []models.NewOpeningStock{
			{WarehouseId: primary.ID, Qty: decimal.NewFromInt(9), UnitValue: decimal.NewFromInt(100)},
		},
	})
	if err != nil {
		t.Fatalf("CreateProduct: %v", err)
	}

	adjDate := time.Date(2026, 1, 15, 12, 0, 0, 0, time.UTC)
	input := &models.NewInventoryAdjustment{
		ReferenceNumber: "IVAV-RACE",
		AdjustmentType:  models.InventoryAdjustmentTypeValue,
		AdjustmentDate:  adjDate,
		AccountId:       cogsAcc,
		BranchId:        biz.PrimaryBranchId,
		WarehouseId:     primary.ID,
		CurrentStatus:   models.InventoryAdjustmentStatusDraft,
		ReasonId:        reason.ID,
		Description:     "Revalue",
		Details: []models.NewInventoryAdjustmentDetail{
			{
				ProductId:     p.ID,
				ProductType:   models.ProductTypeSingle,
				BatchNumber:   "",
				Name:          "RaceItem",
				AdjustedValue: decimal.NewFromInt(100), // new unit cost
				CostPrice:     decimal.NewFromInt(100),
			},
		},
	}

	// Without processing opening stock outbox, validation should deterministically fail with "ledger missing".
	if err := models.ValidateInventoryAdjustmentInput(ctx, input); err == nil {
		t.Fatalf("expected ledger-missing validation error before processing opening stock outbox; got nil")
	}

	// Now process outbox synchronously via readiness helper (the same thing GraphQL mutation does).
	logger := logrus.New()
	didWork, err := workflow.EnsureIVAVPrereqLedgerReady(ctx, logger, businessID, primary.ID, p.ID, models.ProductTypeSingle, "", adjDate)
	if err != nil {
		t.Fatalf("EnsureIVAVPrereqLedgerReady: %v", err)
	}
	if !didWork {
		t.Fatalf("expected readiness helper to process opening stock outbox (didWork=false)")
	}

	// After readiness, repeating validation should be stable (no nondeterminism).
	for i := 0; i < 10; i++ {
		if err := models.ValidateInventoryAdjustmentInput(ctx, input); err != nil {
			t.Fatalf("expected validation OK after readiness (iter=%d): %v", i, err)
		}
	}
}
