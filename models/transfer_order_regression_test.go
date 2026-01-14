package models_test

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"regexp"
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

func TestTransferOrderConfirmedUpdatesWarehouseInventoryReport(t *testing.T) {
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
	// Ensure stock commands are "disabled" to match the reported bug conditions.
	// The fix must still apply stock summary updates on confirm.
	t.Setenv("STOCK_COMMANDS_DOCS", "")
	// Helpful to see logs in CI when debugging failures.
	t.Setenv("DEBUG_TRANSFER_ORDER", "true")

	// Connect deps.
	config.ConnectDatabaseWithRetry()
	config.ConnectRedisWithRetry()

	// Migrate schema (in a fresh DB).
	models.MigrateTable()

	// Many model hooks write History records and require user context.
	ctx = utils.SetUserIdInContext(ctx, 1)
	ctx = utils.SetUserNameInContext(ctx, "Test")
	ctx = utils.SetUsernameInContext(ctx, "test@local")

	// 1) Create a new business (includes default branch + primary warehouse + system accounts).
	biz, err := models.CreateBusiness(ctx, &models.NewBusiness{
		Name:  "Test Biz",
		Email: "owner@test.local",
	})
	if err != nil {
		t.Fatalf("CreateBusiness: %v", err)
	}
	businessID := biz.ID.String()
	ctx = utils.SetBusinessIdInContext(ctx, businessID)
	// user context already set above

	// Fetch primary warehouse.
	db := config.GetDB()
	if db == nil {
		t.Fatalf("db is nil after ConnectDatabaseWithRetry")
	}
	var primary models.Warehouse
	if err := db.WithContext(ctx).Where("business_id = ? AND name = ?", businessID, "Primary Warehouse").First(&primary).Error; err != nil {
		t.Fatalf("fetch primary warehouse: %v", err)
	}

	// Create destination warehouse.
	green, err := models.CreateWarehouse(ctx, &models.NewWarehouse{
		BranchId: biz.PrimaryBranchId,
		Name:     "Green Warehouse",
	})
	if err != nil {
		t.Fatalf("CreateWarehouse: %v", err)
	}

	// Create reason.
	reason, err := models.CreateReason(ctx, &models.NewReason{Name: "Transfer"})
	if err != nil {
		t.Fatalf("CreateReason: %v", err)
	}

	// Create product unit.
	unit, err := models.CreateProductUnit(ctx, &models.NewProductUnit{Name: "Pcs", Abbreviation: "pc", Precision: models.PrecisionZero})
	if err != nil {
		t.Fatalf("CreateProductUnit: %v", err)
	}

	// Create a product "Stapler" with initial stock 25 at Primary Warehouse.
	sysAccounts, err := models.GetSystemAccounts(businessID)
	if err != nil {
		t.Fatalf("GetSystemAccounts: %v", err)
	}
	invAcc := sysAccounts[models.AccountCodeInventoryAsset]
	salesAcc := sysAccounts[models.AccountCodeSales]
	purchaseAcc := sysAccounts[models.AccountCodeCostOfGoodsSold]
	if invAcc == 0 || salesAcc == 0 || purchaseAcc == 0 {
		t.Fatalf("missing required system accounts (inv=%d sales=%d purchase=%d)", invAcc, salesAcc, purchaseAcc)
	}

	stapler, err := models.CreateProduct(ctx, &models.NewProduct{
		Name:               "Stapler",
		Sku:                "STAPLER-001",
		Barcode:            "STAPLER-001",
		UnitId:             unit.ID,
		SalesAccountId:     salesAcc,
		PurchaseAccountId:  purchaseAcc,
		InventoryAccountId: invAcc,
		IsBatchTracking:    utils.NewFalse(),
		OpeningStocks: []models.NewOpeningStock{
			{
				WarehouseId: primary.ID,
				Qty:         decimal.NewFromInt(25),
				UnitValue:   decimal.NewFromInt(5000),
			},
		},
	})
	if err != nil {
		t.Fatalf("CreateProduct: %v", err)
	}

	// Sanity check: warehouse report shows Primary=25, Green=0 before transfer.
	asOf := models.MyDateString(time.Date(2026, 1, 31, 0, 0, 0, 0, time.UTC))
	beforeRows, err := reports.GetWarehouseInventoryReport(ctx, asOf)
	if err != nil {
		t.Fatalf("GetWarehouseInventoryReport(before): %v", err)
	}
	beforePrimary := findWarehouseProductRow(beforeRows, primary.ID, stapler.ID)
	beforeGreen := findWarehouseProductRow(beforeRows, green.ID, stapler.ID)
	if beforePrimary == nil || beforePrimary.CurrentQty.Cmp(decimal.NewFromInt(25)) != 0 {
		t.Fatalf("before: expected primary current_qty=25; got %+v", beforePrimary)
	}
	// The report is based on stock_summary_daily_balances, so warehouses with no rows may be absent.
	if beforeGreen != nil && !beforeGreen.CurrentQty.IsZero() {
		t.Fatalf("before: expected green current_qty=0 (or missing row); got %+v", beforeGreen)
	}

	// 2) Create/Confirm transfer order: move 10 from Primary -> Green (Confirmed).
	transferDate := time.Date(2026, 1, 3, 12, 0, 0, 0, time.UTC)
	to, err := models.CreateTransferOrder(ctx, &models.NewTransferOrder{
		OrderNumber:            "TO-0001",
		TransferDate:           transferDate,
		ReasonId:               reason.ID,
		SourceWarehouseId:      primary.ID,
		DestinationWarehouseId: green.ID,
		CurrentStatus:          models.TransferOrderStatusConfirmed,
		Details: []models.NewTransferOrderDetail{
			{
				ProductId:   stapler.ID,
				ProductType: models.ProductTypeSingle,
				Name:        "Stapler",
				TransferQty: decimal.NewFromInt(10),
			},
		},
	})
	if err != nil {
		t.Fatalf("CreateTransferOrder: %v", err)
	}
	if to.CurrentStatus != models.TransferOrderStatusConfirmed {
		t.Fatalf("expected transfer order status Confirmed; got %s", to.CurrentStatus)
	}

	// 3) Verify outbox record exists for TO (journal posting is async).
	var outbox models.PubSubMessageRecord
	if err := db.WithContext(ctx).
		Where("business_id = ? AND reference_type = ? AND reference_id = ?", businessID, models.AccountReferenceTypeTransferOrder, to.ID).
		Order("id DESC").
		First(&outbox).Error; err != nil {
		t.Fatalf("expected outbox record for transfer order: %v", err)
	}

	// 4) Process the outbox record via workflow to simulate accounting worker.
	// This creates stock_histories and account journals/transactions.
	msg := models.ConvertToPubSubMessage(outbox)
	logger := logrus.New()
	wtx := db.Begin()
	if err := workflow.ProcessTransferOrderWorkflow(wtx, logger, msg); err != nil {
		t.Fatalf("ProcessTransferOrderWorkflow: %v", err)
	}
	if err := wtx.Commit().Error; err != nil {
		t.Fatalf("ProcessTransferOrderWorkflow commit: %v", err)
	}

	// 5) Verify warehouse report reflects the transfer (source decreased, destination increased).
	afterRows, err := reports.GetWarehouseInventoryReport(ctx, asOf)
	if err != nil {
		t.Fatalf("GetWarehouseInventoryReport(after): %v", err)
	}
	afterPrimary := findWarehouseProductRow(afterRows, primary.ID, stapler.ID)
	afterGreen := findWarehouseProductRow(afterRows, green.ID, stapler.ID)

	// Confirm semantics in this system: Confirm == immediate transfer (no separate receive step).
	// Source: 25 - 10 = 15; TransferQtyOut should be 10
	if afterPrimary == nil || afterPrimary.CurrentQty.Cmp(decimal.NewFromInt(15)) != 0 || afterPrimary.TransferQtyOut.Cmp(decimal.NewFromInt(10)) != 0 {
		t.Fatalf("after: expected primary current_qty=15 transfer_qty_out=10; got %+v", afterPrimary)
	}
	// Destination: 0 + 10 = 10; TransferQtyIn should be 10
	if afterGreen == nil || afterGreen.CurrentQty.Cmp(decimal.NewFromInt(10)) != 0 || afterGreen.TransferQtyIn.Cmp(decimal.NewFromInt(10)) != 0 {
		t.Fatalf("after: expected green current_qty=10 transfer_qty_in=10; got %+v", afterGreen)
	}

	// 6) Verify product-by-warehouse (stock_summaries) matches report truth source.
	var ssPrimary models.StockSummary
	if err := db.WithContext(ctx).
		Where("business_id = ? AND warehouse_id = ? AND product_id = ? AND product_type = ?", businessID, primary.ID, stapler.ID, models.ProductTypeSingle).
		First(&ssPrimary).Error; err != nil {
		t.Fatalf("fetch stock summary (primary): %v", err)
	}
	var ssGreen models.StockSummary
	if err := db.WithContext(ctx).
		Where("business_id = ? AND warehouse_id = ? AND product_id = ? AND product_type = ?", businessID, green.ID, stapler.ID, models.ProductTypeSingle).
		First(&ssGreen).Error; err != nil {
		t.Fatalf("fetch stock summary (green): %v", err)
	}
	if ssPrimary.CurrentQty.Cmp(decimal.NewFromInt(15)) != 0 || ssPrimary.TransferQtyOut.Cmp(decimal.NewFromInt(10)) != 0 {
		t.Fatalf("stock_summaries primary mismatch: %+v", ssPrimary)
	}
	if ssGreen.CurrentQty.Cmp(decimal.NewFromInt(10)) != 0 || ssGreen.TransferQtyIn.Cmp(decimal.NewFromInt(10)) != 0 {
		t.Fatalf("stock_summaries green mismatch: %+v", ssGreen)
	}
}

func TestTransferOrderInventoryValuationByItemIncludesAllFIFOLayersOnDestination(t *testing.T) {
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

	// Context required by model hooks.
	ctx = utils.SetUserIdInContext(ctx, 1)
	ctx = utils.SetUserNameInContext(ctx, "Test")
	ctx = utils.SetUsernameInContext(ctx, "test@local")

	// Business + warehouses.
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
	pink, err := models.CreateWarehouse(ctx, &models.NewWarehouse{
		BranchId: biz.PrimaryBranchId,
		Name:     "Pink Warehouse",
	})
	if err != nil {
		t.Fatalf("CreateWarehouse: %v", err)
	}
	reason, err := models.CreateReason(ctx, &models.NewReason{Name: "Transfer"})
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
	purchaseAcc := sysAccounts[models.AccountCodeCostOfGoodsSold]
	if invAcc == 0 || salesAcc == 0 || purchaseAcc == 0 {
		t.Fatalf("missing required system accounts (inv=%d sales=%d purchase=%d)", invAcc, salesAcc, purchaseAcc)
	}

	// Create product with inventory tracking enabled (no opening stocks; we seed stock_histories below).
	cardHolder, err := models.CreateProduct(ctx, &models.NewProduct{
		Name:               "Card Holder",
		Sku:                "CARD-001",
		Barcode:            "CARD-001",
		UnitId:             unit.ID,
		SalesAccountId:     salesAcc,
		PurchaseAccountId:  purchaseAcc,
		InventoryAccountId: invAcc,
		IsBatchTracking:    utils.NewFalse(),
	})
	if err != nil {
		t.Fatalf("CreateProduct: %v", err)
	}

	// Seed two FIFO layers at Primary (2 @ 6000, 1 @ 5000) dated 31 Dec 2025.
	business, err := models.GetBusiness(ctx)
	if err != nil {
		t.Fatalf("GetBusiness: %v", err)
	}
	openingDate := time.Date(2025, 12, 31, 12, 0, 0, 0, time.UTC)
	stockDate, err := utils.ConvertToDate(openingDate, business.Timezone)
	if err != nil {
		t.Fatalf("ConvertToDate: %v", err)
	}

	logger := logrus.New()
	tx := db.Begin()

	in1 := &models.StockHistory{
		BusinessId:         businessID,
		WarehouseId:        primary.ID,
		ProductId:          cardHolder.ID,
		ProductType:        models.ProductTypeSingle,
		BatchNumber:        "",
		StockDate:          stockDate,
		Qty:                decimal.NewFromInt(2),
		BaseUnitValue:      decimal.NewFromInt(6000),
		Description:        "Opening Stock",
		ReferenceType:      models.StockReferenceTypeProductOpeningStock,
		ReferenceID:        cardHolder.ID,
		IsOutgoing:         utils.NewFalse(),
		IsTransferIn:       utils.NewFalse(),
		CumulativeSequence: 0,
	}
	if err := tx.WithContext(ctx).Create(in1).Error; err != nil {
		tx.Rollback()
		t.Fatalf("create incoming stock history 1: %v", err)
	}

	in2 := &models.StockHistory{
		BusinessId:         businessID,
		WarehouseId:        primary.ID,
		ProductId:          cardHolder.ID,
		ProductType:        models.ProductTypeSingle,
		BatchNumber:        "",
		StockDate:          stockDate,
		Qty:                decimal.NewFromInt(1),
		BaseUnitValue:      decimal.NewFromInt(5000),
		Description:        "Opening Stock",
		ReferenceType:      models.StockReferenceTypeProductOpeningStock,
		ReferenceID:        cardHolder.ID,
		IsOutgoing:         utils.NewFalse(),
		IsTransferIn:       utils.NewFalse(),
		CumulativeSequence: 0,
	}
	if err := tx.WithContext(ctx).Create(in2).Error; err != nil {
		tx.Rollback()
		t.Fatalf("create incoming stock history 2: %v", err)
	}

	// Keep stock_summaries consistent enough for any stock-dependent UI/helpers.
	if err := models.UpdateStockSummaryOpeningQty(tx.WithContext(ctx), businessID, primary.ID, cardHolder.ID, string(models.ProductTypeSingle), "", decimal.NewFromInt(3), stockDate); err != nil {
		tx.Rollback()
		t.Fatalf("UpdateStockSummaryOpeningQty: %v", err)
	}

	// Run incoming processor to compute cumulative/closing balances.
	if _, err := workflow.ProcessIncomingStocks(tx.WithContext(ctx), logger, []*models.StockHistory{in1, in2}); err != nil {
		tx.Rollback()
		t.Fatalf("ProcessIncomingStocks(seed): %v", err)
	}
	if err := tx.Commit().Error; err != nil {
		t.Fatalf("commit seed: %v", err)
	}

	// Create/Confirm transfer order qty 3 from Primary -> Pink on 02 Jan 2026.
	transferDate := time.Date(2026, 1, 2, 12, 0, 0, 0, time.UTC)
	to, err := models.CreateTransferOrder(ctx, &models.NewTransferOrder{
		OrderNumber:            "TO-VAL-0001",
		TransferDate:           transferDate,
		ReasonId:               reason.ID,
		SourceWarehouseId:      primary.ID,
		DestinationWarehouseId: pink.ID,
		CurrentStatus:          models.TransferOrderStatusConfirmed,
		Details: []models.NewTransferOrderDetail{
			{
				ProductId:   cardHolder.ID,
				ProductType: models.ProductTypeSingle,
				Name:        "Card Holder",
				TransferQty: decimal.NewFromInt(3),
			},
		},
	})
	if err != nil {
		t.Fatalf("CreateTransferOrder: %v", err)
	}

	// Process outbox record via workflow (creates stock_histories + valuations).
	var outbox models.PubSubMessageRecord
	if err := db.WithContext(ctx).
		Where("business_id = ? AND reference_type = ? AND reference_id = ?", businessID, models.AccountReferenceTypeTransferOrder, to.ID).
		Order("id DESC").
		First(&outbox).Error; err != nil {
		t.Fatalf("expected outbox record for transfer order: %v", err)
	}
	msg := models.ConvertToPubSubMessage(outbox)
	wtx := db.Begin()
	if err := workflow.ProcessTransferOrderWorkflow(wtx, logger, msg); err != nil {
		t.Fatalf("ProcessTransferOrderWorkflow: %v", err)
	}
	if err := wtx.Commit().Error; err != nil {
		t.Fatalf("ProcessTransferOrderWorkflow commit: %v", err)
	}

	// Inventory valuation by item should show Transfer Out = -3 total and Transfer In = +3 total (may be split into layers).
	from := models.MyDateString(time.Date(2025, 12, 31, 0, 0, 0, 0, time.UTC))
	toDate := models.MyDateString(time.Date(2026, 1, 31, 0, 0, 0, 0, time.UTC))

	srcVal, err := models.GetInventoryValuation(ctx, from, toDate, cardHolder.ID, models.ProductTypeSingle, primary.ID)
	if err != nil {
		t.Fatalf("GetInventoryValuation(source): %v", err)
	}
	dstVal, err := models.GetInventoryValuation(ctx, from, toDate, cardHolder.ID, models.ProductTypeSingle, pink.ID)
	if err != nil {
		t.Fatalf("GetInventoryValuation(destination): %v", err)
	}

	sumByDesc := func(v *models.InventoryValuationResponse, desc string) decimal.Decimal {
		total := decimal.Zero
		if v == nil {
			return total
		}
		for _, d := range v.Details {
			if d == nil {
				continue
			}
			if d.TransactionDescription == desc {
				total = total.Add(d.Qty)
			}
		}
		return total
	}

	srcOut := sumByDesc(srcVal, "Transfer Out")
	dstIn := sumByDesc(dstVal, "Transfer In")
	if srcOut.Cmp(decimal.NewFromInt(-3)) != 0 {
		t.Fatalf("expected source Transfer Out sum = -3; got %s", srcOut.String())
	}
	if dstIn.Cmp(decimal.NewFromInt(3)) != 0 {
		t.Fatalf("expected destination Transfer In sum = 3; got %s", dstIn.String())
	}

	// Conservation: sum(qty) across warehouses for the same transfer reference should be 0.
	var sumQty decimal.Decimal
	if err := db.WithContext(ctx).Model(&models.StockHistory{}).
		Where("business_id = ? AND reference_type = ? AND reference_id = ? AND is_reversal = 0", businessID, models.StockReferenceTypeTransferOrder, to.ID).
		Select("COALESCE(SUM(qty), 0)").Scan(&sumQty).Error; err != nil {
		t.Fatalf("sum transfer stock_histories qty: %v", err)
	}
	if !sumQty.IsZero() {
		t.Fatalf("expected transfer qty conservation (sum=0); got %s", sumQty.String())
	}
}

func TestInventoryValuationByItem_DoesNotDoubleCountTransferReversals(t *testing.T) {
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
		t.Fatalf("CreateWarehouse: %v", err)
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
	purchaseAcc := sysAccounts[models.AccountCodeCostOfGoodsSold]

	amouage, err := models.CreateProduct(ctx, &models.NewProduct{
		Name:               "Amouage",
		Sku:                "AMO-001",
		Barcode:            "AMO-001",
		UnitId:             unit.ID,
		SalesAccountId:     salesAcc,
		PurchaseAccountId:  purchaseAcc,
		InventoryAccountId: invAcc,
		IsBatchTracking:    utils.NewFalse(),
	})
	if err != nil {
		t.Fatalf("CreateProduct: %v", err)
	}

	business, err := models.GetBusiness(ctx)
	if err != nil {
		t.Fatalf("GetBusiness: %v", err)
	}
	openingDate := time.Date(2025, 12, 31, 12, 0, 0, 0, time.UTC)
	stockDate, err := utils.ConvertToDate(openingDate, business.Timezone)
	if err != nil {
		t.Fatalf("ConvertToDate: %v", err)
	}

	// Seed opening stock of 100 @ 100,000 to make asset value visible.
	logger := logrus.New()
	tx := db.Begin()
	opening := &models.StockHistory{
		BusinessId:    businessID,
		WarehouseId:   primary.ID,
		ProductId:     amouage.ID,
		ProductType:   models.ProductTypeSingle,
		BatchNumber:   "",
		StockDate:     stockDate,
		Qty:           decimal.NewFromInt(100),
		BaseUnitValue: decimal.NewFromInt(100000),
		Description:   "Opening Stock",
		ReferenceType: models.StockReferenceTypeProductOpeningStock,
		ReferenceID:   amouage.ID,
		IsOutgoing:    utils.NewFalse(),
		IsTransferIn:  utils.NewFalse(),
	}
	if err := tx.WithContext(ctx).Create(opening).Error; err != nil {
		tx.Rollback()
		t.Fatalf("create opening stock history: %v", err)
	}
	if err := models.UpdateStockSummaryOpeningQty(tx.WithContext(ctx), businessID, primary.ID, amouage.ID, string(models.ProductTypeSingle), "", decimal.NewFromInt(100), stockDate); err != nil {
		tx.Rollback()
		t.Fatalf("UpdateStockSummaryOpeningQty: %v", err)
	}
	if _, err := workflow.ProcessIncomingStocks(tx.WithContext(ctx), logger, []*models.StockHistory{opening}); err != nil {
		tx.Rollback()
		t.Fatalf("ProcessIncomingStocks(seed): %v", err)
	}
	if err := tx.Commit().Error; err != nil {
		t.Fatalf("commit seed: %v", err)
	}

	// Transfer 10 out of Primary to YGN on 14 Jan 2026.
	transferDate := time.Date(2026, 1, 14, 12, 0, 0, 0, time.UTC)
	to, err := models.CreateTransferOrder(ctx, &models.NewTransferOrder{
		OrderNumber:            "TO-0002",
		TransferDate:           transferDate,
		ReasonId:               reason.ID,
		SourceWarehouseId:      primary.ID,
		DestinationWarehouseId: ygn.ID,
		CurrentStatus:          models.TransferOrderStatusConfirmed,
		Details: []models.NewTransferOrderDetail{
			{ProductId: amouage.ID, ProductType: models.ProductTypeSingle, Name: "Amouage", TransferQty: decimal.NewFromInt(10)},
		},
	})
	if err != nil {
		t.Fatalf("CreateTransferOrder: %v", err)
	}
	var outbox models.PubSubMessageRecord
	if err := db.WithContext(ctx).
		Where("business_id = ? AND reference_type = ? AND reference_id = ?", businessID, models.AccountReferenceTypeTransferOrder, to.ID).
		Order("id DESC").
		First(&outbox).Error; err != nil {
		t.Fatalf("expected outbox record for transfer order: %v", err)
	}
	msg := models.ConvertToPubSubMessage(outbox)
	wtx := db.Begin()
	if err := workflow.ProcessTransferOrderWorkflow(wtx, logger, msg); err != nil {
		t.Fatalf("ProcessTransferOrderWorkflow: %v", err)
	}
	if err := wtx.Commit().Error; err != nil {
		t.Fatalf("ProcessTransferOrderWorkflow commit: %v", err)
	}

	// Source of truth: stock summary should be 90/10.
	var ssPrimary models.StockSummary
	if err := db.WithContext(ctx).
		Where("business_id = ? AND warehouse_id = ? AND product_id = ? AND product_type = ?", businessID, primary.ID, amouage.ID, models.ProductTypeSingle).
		First(&ssPrimary).Error; err != nil {
		t.Fatalf("fetch stock summary (primary): %v", err)
	}
	var ssYGN models.StockSummary
	if err := db.WithContext(ctx).
		Where("business_id = ? AND warehouse_id = ? AND product_id = ? AND product_type = ?", businessID, ygn.ID, amouage.ID, models.ProductTypeSingle).
		First(&ssYGN).Error; err != nil {
		t.Fatalf("fetch stock summary (ygn): %v", err)
	}
	if ssPrimary.CurrentQty.Cmp(decimal.NewFromInt(90)) != 0 {
		t.Fatalf("expected primary current_qty=90; got %s", ssPrimary.CurrentQty.String())
	}
	if ssYGN.CurrentQty.Cmp(decimal.NewFromInt(10)) != 0 {
		t.Fatalf("expected ygn current_qty=10; got %s", ssYGN.CurrentQty.String())
	}

	from := models.MyDateString(time.Date(2025, 12, 31, 0, 0, 0, 0, time.UTC))
	toDate := models.MyDateString(time.Date(2026, 1, 31, 0, 0, 0, 0, time.UTC))

	primaryVal, err := models.GetInventoryValuation(ctx, from, toDate, amouage.ID, models.ProductTypeSingle, primary.ID)
	if err != nil {
		t.Fatalf("GetInventoryValuation(primary): %v", err)
	}
	if primaryVal.ClosingStockOnHand.Cmp(decimal.NewFromInt(90)) != 0 {
		t.Fatalf("expected report closing stock (primary)=90; got %s", primaryVal.ClosingStockOnHand.String())
	}
	// No unexpected REV lines in the normal report view.
	for _, d := range primaryVal.Details {
		if d == nil {
			continue
		}
		if strings.HasPrefix(d.TransactionDescription, "REV:") {
			t.Fatalf("unexpected REV line in report details: %q", d.TransactionDescription)
		}
		// Ensure we don't leak destination warehouse rows into source-warehouse report.
		if d.WarehouseName != nil && *d.WarehouseName != "Primary Warehouse" {
			t.Fatalf("unexpected warehouse row in primary report: %q", *d.WarehouseName)
		}
	}
}

func findWarehouseProductRow(rows []*reports.WarehouseInventoryResponse, warehouseID int, productID int) *reports.WarehouseInventoryResponse {
	for _, r := range rows {
		if r == nil {
			continue
		}
		if r.WarehouseId == warehouseID && r.ProductId == productID {
			return r
		}
	}
	return nil
}

func startRedisContainer(t *testing.T) (containerName, hostPort string) {
	t.Helper()
	name := fmt.Sprintf("books-test-redis-%d", time.Now().UnixNano())
	out, err := dockerRun(
		"run", "-d", "--name", name,
		"-p", "127.0.0.1:0:6379",
		"redis:7-alpine",
	)
	if err != nil {
		t.Fatalf("start redis container: %v\n%s", err, out)
	}
	port, err := dockerHostPort(name, "6379/tcp")
	if err != nil {
		t.Fatalf("redis docker port: %v", err)
	}
	// wait until ready
	deadline := time.Now().Add(60 * time.Second)
	for time.Now().Before(deadline) {
		_, err := dockerRun("exec", name, "redis-cli", "ping")
		if err == nil {
			return name, port
		}
		time.Sleep(250 * time.Millisecond)
	}
	t.Fatalf("redis did not become ready")
	return "", ""
}

func startMySQLContainer(t *testing.T) (containerName, hostPort string) {
	t.Helper()
	name := fmt.Sprintf("books-test-mysql-%d", time.Now().UnixNano())
	out, err := dockerRun(
		"run", "-d", "--name", name,
		"-e", "MYSQL_ROOT_PASSWORD=testpw",
		"-e", "MYSQL_DATABASE=pitibooks_test",
		"-p", "127.0.0.1:0:3306",
		"mysql:8.0",
		"--default-authentication-plugin=mysql_native_password",
	)
	if err != nil {
		t.Fatalf("start mysql container: %v\n%s", err, out)
	}
	port, err := dockerHostPort(name, "3306/tcp")
	if err != nil {
		t.Fatalf("mysql docker port: %v", err)
	}
	// wait until ready
	deadline := time.Now().Add(120 * time.Second)
	for time.Now().Before(deadline) {
		_, err := dockerRun("exec", name, "mysqladmin", "ping", "-h", "127.0.0.1", "-ptestpw", "--silent")
		if err == nil {
			return name, port
		}
		time.Sleep(500 * time.Millisecond)
	}
	t.Fatalf("mysql did not become ready")
	return "", ""
}

func dockerHostPort(container, portProto string) (string, error) {
	out, err := dockerRun("port", container, portProto)
	if err != nil {
		return "", fmt.Errorf("docker port: %w: %s", err, out)
	}
	// Example: "127.0.0.1:49154\n"
	re := regexp.MustCompile(`:(\d+)`)
	m := re.FindStringSubmatch(out)
	if len(m) != 2 {
		return "", fmt.Errorf("unexpected docker port output: %q", out)
	}
	return m[1], nil
}

func dockerRmForce(container string) error {
	if strings.TrimSpace(container) == "" {
		return nil
	}
	_, err := dockerRun("rm", "-f", container)
	return err
}

func dockerRun(args ...string) (string, error) {
	cmd := exec.Command("docker", args...)
	b, err := cmd.CombinedOutput()
	return string(b), err
}
