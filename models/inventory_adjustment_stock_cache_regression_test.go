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

func TestInventoryAdjustmentQuantity_UpdatesStockSummariesAndReports(t *testing.T) {
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
	// Ensure the legacy mode that produced the bug.
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
	var primary models.Warehouse
	if err := db.WithContext(ctx).Where("business_id = ? AND name = ?", businessID, "Primary Warehouse").First(&primary).Error; err != nil {
		t.Fatalf("fetch primary warehouse: %v", err)
	}

	// Create a second warehouse "YGN".
	ygn, err := models.CreateWarehouse(ctx, &models.NewWarehouse{
		BranchId: biz.PrimaryBranchId,
		Name:     "YGN",
	})
	if err != nil {
		t.Fatalf("CreateWarehouse(YGN): %v", err)
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

	// 2) Create product Toyota with opening stocks Primary=100, YGN=10.
	toyota, err := models.CreateProduct(ctx, &models.NewProduct{
		Name:               "Toyota",
		Sku:                "TOYOTA-001",
		Barcode:            "TOYOTA-001",
		UnitId:             unit.ID,
		SalesAccountId:     salesAcc,
		PurchaseAccountId:  cogsAcc,
		InventoryAccountId: invAcc,
		IsBatchTracking:    utils.NewFalse(),
		OpeningStocks: []models.NewOpeningStock{
			{WarehouseId: primary.ID, Qty: decimal.NewFromInt(100), UnitValue: decimal.NewFromInt(500)},
			{WarehouseId: ygn.ID, Qty: decimal.NewFromInt(10), UnitValue: decimal.NewFromInt(500)},
		},
	})
	if err != nil {
		t.Fatalf("CreateProduct: %v", err)
	}

	// 3) Create inventory adjustment (Quantity) at YGN: +10 on 14 Jan 2026, status Adjusted.
	adjDate := time.Date(2026, 1, 14, 12, 0, 0, 0, time.UTC)
	ia, err := models.CreateInventoryAdjustment(ctx, &models.NewInventoryAdjustment{
		ReferenceNumber: "IA-0001",
		AdjustmentType:  models.InventoryAdjustmentTypeQuantity,
		AdjustmentDate:  adjDate,
		AccountId:       cogsAcc,
		BranchId:        biz.PrimaryBranchId,
		WarehouseId:     ygn.ID,
		CurrentStatus:   models.InventoryAdjustmentStatusAdjusted,
		ReasonId:        reason.ID,
		Description:     "Increase stock",
		Details: []models.NewInventoryAdjustmentDetail{
			{
				ProductId:     toyota.ID,
				ProductType:   models.ProductTypeSingle,
				BatchNumber:   "",
				Name:          "Toyota",
				AdjustedValue: decimal.NewFromInt(10),
				CostPrice:     decimal.NewFromInt(500),
			},
		},
	})
	if err != nil {
		t.Fatalf("CreateInventoryAdjustment: %v", err)
	}
	if ia.CurrentStatus != models.InventoryAdjustmentStatusAdjusted {
		t.Fatalf("expected adjustment status Adjusted; got %s", ia.CurrentStatus)
	}

	// 4) Process the outbox record via workflow to simulate accounting worker.
	var outbox models.PubSubMessageRecord
	if err := db.WithContext(ctx).
		Where("business_id = ? AND reference_type = ? AND reference_id = ? AND action = ?", businessID, models.AccountReferenceTypeInventoryAdjustmentQuantity, ia.ID, models.PubSubMessageActionCreate).
		Order("id DESC").
		First(&outbox).Error; err != nil {
		t.Fatalf("expected outbox record for inventory adjustment: %v", err)
	}
	msg := models.ConvertToPubSubMessage(outbox)
	logger := logrus.New()
	wtx := db.Begin()
	if err := workflow.ProcessInventoryAdjustmentQuantityWorkflow(wtx, logger, msg); err != nil {
		t.Fatalf("ProcessInventoryAdjustmentQuantityWorkflow: %v", err)
	}
	if err := wtx.Commit().Error; err != nil {
		t.Fatalf("workflow commit: %v", err)
	}

	// 5) Assert operational stock cache (stock_summaries) updated per-warehouse.
	var ssPrimary models.StockSummary
	if err := db.WithContext(ctx).
		Where("business_id = ? AND warehouse_id = ? AND product_id = ? AND product_type = ?", businessID, primary.ID, toyota.ID, models.ProductTypeSingle).
		First(&ssPrimary).Error; err != nil {
		t.Fatalf("fetch stock summary (primary): %v", err)
	}
	var ssYGN models.StockSummary
	if err := db.WithContext(ctx).
		Where("business_id = ? AND warehouse_id = ? AND product_id = ? AND product_type = ?", businessID, ygn.ID, toyota.ID, models.ProductTypeSingle).
		First(&ssYGN).Error; err != nil {
		t.Fatalf("fetch stock summary (ygn): %v", err)
	}
	if ssPrimary.CurrentQty.Cmp(decimal.NewFromInt(100)) != 0 {
		t.Fatalf("expected primary current_qty=100; got %s", ssPrimary.CurrentQty.String())
	}
	if ssYGN.CurrentQty.Cmp(decimal.NewFromInt(20)) != 0 {
		t.Fatalf("expected ygn current_qty=20; got %s", ssYGN.CurrentQty.String())
	}

	// 6) Assert Warehouse Inventory Report (stock_summary_daily_balances) updated.
	asOf := models.MyDateString(time.Date(2026, 1, 31, 0, 0, 0, 0, time.UTC))
	rows, err := reports.GetWarehouseInventoryReport(ctx, asOf)
	if err != nil {
		t.Fatalf("GetWarehouseInventoryReport: %v", err)
	}
	rPrimary := findWarehouseProductRow(rows, primary.ID, toyota.ID)
	rYGN := findWarehouseProductRow(rows, ygn.ID, toyota.ID)
	if rPrimary == nil || rPrimary.CurrentQty.Cmp(decimal.NewFromInt(100)) != 0 {
		t.Fatalf("warehouse report primary mismatch: %+v", rPrimary)
	}
	if rYGN == nil || rYGN.CurrentQty.Cmp(decimal.NewFromInt(20)) != 0 || rYGN.AdjustedQtyIn.Cmp(decimal.NewFromInt(10)) != 0 {
		t.Fatalf("warehouse report ygn mismatch: %+v", rYGN)
	}

	// 7) Assert availability query path (inventory summary available_stock) updated.
	invRows, err := reports.GetInventorySummaryReport(ctx, asOf, &ygn.ID)
	if err != nil {
		t.Fatalf("GetInventorySummaryReport(ygn): %v", err)
	}
	var toyotaInvRow *reports.InventorySummaryResponse
	for _, r := range invRows {
		if r != nil && r.ProductId == toyota.ID {
			toyotaInvRow = r
			break
		}
	}
	if toyotaInvRow == nil {
		t.Fatalf("expected toyota row in inventory summary report")
	}
	if toyotaInvRow.AvailableStock.Cmp(decimal.NewFromInt(20)) != 0 {
		t.Fatalf("expected available_stock=20; got %s", toyotaInvRow.AvailableStock.String())
	}

	// 8) Assert valuation report closing matches 120 total across warehouses.
	from := models.MyDateString(time.Date(2025, 12, 31, 0, 0, 0, 0, time.UTC))
	toDate := models.MyDateString(time.Date(2026, 1, 31, 0, 0, 0, 0, time.UTC))
	val, err := models.GetInventoryValuation(ctx, from, toDate, toyota.ID, models.ProductTypeSingle, 0)
	if err != nil {
		t.Fatalf("GetInventoryValuation: %v", err)
	}
	if val.ClosingStockOnHand.Cmp(decimal.NewFromInt(120)) != 0 {
		t.Fatalf("expected valuation closing stock=120; got %s", val.ClosingStockOnHand.String())
	}
}

func TestInventoryAdjustmentQuantity_DraftDoesNotChangeStock(t *testing.T) {
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
	ygn, err := models.CreateWarehouse(ctx, &models.NewWarehouse{
		BranchId: biz.PrimaryBranchId,
		Name:     "YGN",
	})
	if err != nil {
		t.Fatalf("CreateWarehouse(YGN): %v", err)
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

	toyota, err := models.CreateProduct(ctx, &models.NewProduct{
		Name:               "Toyota",
		Sku:                "TOYOTA-001",
		Barcode:            "TOYOTA-001",
		UnitId:             unit.ID,
		SalesAccountId:     salesAcc,
		PurchaseAccountId:  cogsAcc,
		InventoryAccountId: invAcc,
		IsBatchTracking:    utils.NewFalse(),
		OpeningStocks: []models.NewOpeningStock{
			{WarehouseId: primary.ID, Qty: decimal.NewFromInt(100), UnitValue: decimal.NewFromInt(500)},
			{WarehouseId: ygn.ID, Qty: decimal.NewFromInt(10), UnitValue: decimal.NewFromInt(500)},
		},
	})
	if err != nil {
		t.Fatalf("CreateProduct: %v", err)
	}

	// Draft adjustment should not publish, hence should not affect ledger or cache.
	adjDate := time.Date(2026, 1, 14, 12, 0, 0, 0, time.UTC)
	ia, err := models.CreateInventoryAdjustment(ctx, &models.NewInventoryAdjustment{
		ReferenceNumber: "IA-DRAFT-0001",
		AdjustmentType:  models.InventoryAdjustmentTypeQuantity,
		AdjustmentDate:  adjDate,
		AccountId:       cogsAcc,
		BranchId:        biz.PrimaryBranchId,
		WarehouseId:     ygn.ID,
		CurrentStatus:   models.InventoryAdjustmentStatusDraft,
		ReasonId:        reason.ID,
		Description:     "Draft",
		Details: []models.NewInventoryAdjustmentDetail{
			{
				ProductId:     toyota.ID,
				ProductType:   models.ProductTypeSingle,
				BatchNumber:   "",
				Name:          "Toyota",
				AdjustedValue: decimal.NewFromInt(10),
				CostPrice:     decimal.NewFromInt(500),
			},
		},
	})
	if err != nil {
		t.Fatalf("CreateInventoryAdjustment(draft): %v", err)
	}
	if ia.CurrentStatus != models.InventoryAdjustmentStatusDraft {
		t.Fatalf("expected draft status; got %s", ia.CurrentStatus)
	}

	var outboxCount int64
	if err := db.WithContext(ctx).Model(&models.PubSubMessageRecord{}).
		Where("business_id = ? AND reference_type = ? AND reference_id = ?", businessID, models.AccountReferenceTypeInventoryAdjustmentQuantity, ia.ID).
		Count(&outboxCount).Error; err != nil {
		t.Fatalf("count outbox: %v", err)
	}
	if outboxCount != 0 {
		t.Fatalf("expected no outbox record for draft adjustment; got %d", outboxCount)
	}

	var ssYGN models.StockSummary
	if err := db.WithContext(ctx).
		Where("business_id = ? AND warehouse_id = ? AND product_id = ? AND product_type = ?", businessID, ygn.ID, toyota.ID, models.ProductTypeSingle).
		First(&ssYGN).Error; err != nil {
		t.Fatalf("fetch stock summary (ygn): %v", err)
	}
	if ssYGN.CurrentQty.Cmp(decimal.NewFromInt(10)) != 0 {
		t.Fatalf("expected ygn current_qty still 10; got %s", ssYGN.CurrentQty.String())
	}
}

func TestInventoryAdjustmentQuantity_DeleteRevertsStockSummaries(t *testing.T) {
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
	ygn, err := models.CreateWarehouse(ctx, &models.NewWarehouse{
		BranchId: biz.PrimaryBranchId,
		Name:     "YGN",
	})
	if err != nil {
		t.Fatalf("CreateWarehouse(YGN): %v", err)
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

	toyota, err := models.CreateProduct(ctx, &models.NewProduct{
		Name:               "Toyota",
		Sku:                "TOYOTA-001",
		Barcode:            "TOYOTA-001",
		UnitId:             unit.ID,
		SalesAccountId:     salesAcc,
		PurchaseAccountId:  cogsAcc,
		InventoryAccountId: invAcc,
		IsBatchTracking:    utils.NewFalse(),
		OpeningStocks: []models.NewOpeningStock{
			{WarehouseId: primary.ID, Qty: decimal.NewFromInt(100), UnitValue: decimal.NewFromInt(500)},
			{WarehouseId: ygn.ID, Qty: decimal.NewFromInt(10), UnitValue: decimal.NewFromInt(500)},
		},
	})
	if err != nil {
		t.Fatalf("CreateProduct: %v", err)
	}

	adjDate := time.Date(2026, 1, 14, 12, 0, 0, 0, time.UTC)
	ia, err := models.CreateInventoryAdjustment(ctx, &models.NewInventoryAdjustment{
		ReferenceNumber: "IA-DEL-0001",
		AdjustmentType:  models.InventoryAdjustmentTypeQuantity,
		AdjustmentDate:  adjDate,
		AccountId:       cogsAcc,
		BranchId:        biz.PrimaryBranchId,
		WarehouseId:     ygn.ID,
		CurrentStatus:   models.InventoryAdjustmentStatusAdjusted,
		ReasonId:        reason.ID,
		Description:     "Increase stock",
		Details: []models.NewInventoryAdjustmentDetail{
			{
				ProductId:     toyota.ID,
				ProductType:   models.ProductTypeSingle,
				BatchNumber:   "",
				Name:          "Toyota",
				AdjustedValue: decimal.NewFromInt(10),
				CostPrice:     decimal.NewFromInt(500),
			},
		},
	})
	if err != nil {
		t.Fatalf("CreateInventoryAdjustment: %v", err)
	}

	// Process create outbox.
	var outboxCreate models.PubSubMessageRecord
	if err := db.WithContext(ctx).
		Where("business_id = ? AND reference_type = ? AND reference_id = ? AND action = ?", businessID, models.AccountReferenceTypeInventoryAdjustmentQuantity, ia.ID, models.PubSubMessageActionCreate).
		Order("id DESC").
		First(&outboxCreate).Error; err != nil {
		t.Fatalf("expected create outbox record: %v", err)
	}
	logger := logrus.New()
	wtx := db.Begin()
	if err := workflow.ProcessInventoryAdjustmentQuantityWorkflow(wtx, logger, models.ConvertToPubSubMessage(outboxCreate)); err != nil {
		t.Fatalf("ProcessInventoryAdjustmentQuantityWorkflow(create): %v", err)
	}
	if err := wtx.Commit().Error; err != nil {
		t.Fatalf("create workflow commit: %v", err)
	}

	// Now delete adjustment (publishes delete outbox).
	if _, err := models.DeleteInventoryAdjustment(ctx, ia.ID); err != nil {
		t.Fatalf("DeleteInventoryAdjustment: %v", err)
	}
	var outboxDelete models.PubSubMessageRecord
	if err := db.WithContext(ctx).
		Where("business_id = ? AND reference_type = ? AND reference_id = ? AND action = ?", businessID, models.AccountReferenceTypeInventoryAdjustmentQuantity, ia.ID, models.PubSubMessageActionDelete).
		Order("id DESC").
		First(&outboxDelete).Error; err != nil {
		t.Fatalf("expected delete outbox record: %v", err)
	}
	wtx2 := db.Begin()
	if err := workflow.ProcessInventoryAdjustmentQuantityWorkflow(wtx2, logger, models.ConvertToPubSubMessage(outboxDelete)); err != nil {
		t.Fatalf("ProcessInventoryAdjustmentQuantityWorkflow(delete): %v", err)
	}
	if err := wtx2.Commit().Error; err != nil {
		t.Fatalf("delete workflow commit: %v", err)
	}

	// Stock should revert: YGN back to 10.
	var ssYGN models.StockSummary
	if err := db.WithContext(ctx).
		Where("business_id = ? AND warehouse_id = ? AND product_id = ? AND product_type = ?", businessID, ygn.ID, toyota.ID, models.ProductTypeSingle).
		First(&ssYGN).Error; err != nil {
		t.Fatalf("fetch stock summary (ygn): %v", err)
	}
	if ssYGN.CurrentQty.Cmp(decimal.NewFromInt(10)) != 0 {
		t.Fatalf("expected ygn current_qty=10 after delete; got %s", ssYGN.CurrentQty.String())
	}
}

