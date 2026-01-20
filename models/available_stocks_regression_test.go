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

// Regression: invoice/transfer selectors rely on getAvailableStocks(warehouseId, asOf).
// This must return correct on-hand per warehouse for a product that has stock.
func TestGetAvailableStocks_ReturnsPerWarehouseOnHand(t *testing.T) {
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
		Name:  "Avail Stocks Co",
		Email: "owner@avail.test",
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
	yangon, err := models.CreateWarehouse(ctx, &models.NewWarehouse{
		BranchId: biz.PrimaryBranchId,
		Name:     "Yangon Warehouse",
	})
	if err != nil {
		t.Fatalf("CreateWarehouse: %v", err)
	}

	unit, err := models.CreateProductUnit(ctx, &models.NewProductUnit{Name: "Pcs", Abbreviation: "pc", Precision: models.PrecisionZero})
	if err != nil {
		t.Fatalf("CreateProductUnit: %v", err)
	}
	sysAccounts, err := models.GetSystemAccounts(businessID)
	if err != nil {
		t.Fatalf("GetSystemAccounts: %v", err)
	}

	red, err := models.CreateProduct(ctx, &models.NewProduct{
		Name:               "Red",
		Sku:                "RED-001",
		Barcode:            "RED-001",
		UnitId:             unit.ID,
		SalesAccountId:     sysAccounts[models.AccountCodeSales],
		PurchaseAccountId:  sysAccounts[models.AccountCodeCostOfGoodsSold],
		InventoryAccountId: sysAccounts[models.AccountCodeInventoryAsset],
		IsBatchTracking:    utils.NewFalse(),
		OpeningStocks: []models.NewOpeningStock{
			{WarehouseId: primary.ID, Qty: decimal.NewFromInt(60), UnitValue: decimal.NewFromInt(3000)},
			{WarehouseId: yangon.ID, Qty: decimal.NewFromInt(2), UnitValue: decimal.NewFromInt(3000)},
		},
	})
	if err != nil {
		t.Fatalf("CreateProduct: %v", err)
	}

	// Process opening stock outbox so stock_histories exists (ledger-of-record).
	var posOutbox models.PubSubMessageRecord
	if err := db.WithContext(ctx).
		Where("business_id = ? AND reference_type = ? AND reference_id = ? AND action = ?",
			businessID, models.AccountReferenceTypeProductOpeningStock, red.ID, models.PubSubMessageActionCreate).
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

	asOf := models.MyDateString(time.Date(2026, 1, 31, 0, 0, 0, 0, time.UTC))

	primaryStocks, err := models.GetAvailableStocks(ctx, primary.ID, &asOf)
	if err != nil {
		t.Fatalf("GetAvailableStocks(primary): %v", err)
	}
	yangonStocks, err := models.GetAvailableStocks(ctx, yangon.ID, &asOf)
	if err != nil {
		t.Fatalf("GetAvailableStocks(yangon): %v", err)
	}

	findQty := func(rows []*models.StockSummary, pid int, pt models.ProductType) decimal.Decimal {
		for _, r := range rows {
			if r.ProductId == pid && r.ProductType == pt {
				return r.CurrentQty
			}
		}
		return decimal.Zero
	}

	if got := findQty(primaryStocks, red.ID, models.ProductTypeSingle); !got.Equal(decimal.NewFromInt(60)) {
		t.Fatalf("primary: expected Red on_hand=60, got %s", got)
	}
	if got := findQty(yangonStocks, red.ID, models.ProductTypeSingle); !got.Equal(decimal.NewFromInt(2)) {
		t.Fatalf("yangon: expected Red on_hand=2, got %s", got)
	}
}

