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

func TestFifoCogsUsesInboundCostAndBlocksNegative(t *testing.T) {
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
		Name:  "FIFO Co",
		Email: "owner@fifo.test",
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

	unit, err := models.CreateProductUnit(ctx, &models.NewProductUnit{Name: "Pcs", Abbreviation: "pc", Precision: models.PrecisionZero})
	if err != nil {
		t.Fatalf("CreateProductUnit: %v", err)
	}
	sysAccounts, err := models.GetSystemAccounts(businessID)
	if err != nil {
		t.Fatalf("GetSystemAccounts: %v", err)
	}

	customer, err := models.CreateCustomer(ctx, &models.NewCustomer{
		Name:                 "Test Customer",
		CurrencyId:           biz.BaseCurrencyId,
		CustomerPaymentTerms: models.PaymentTermsDueOnReceipt,
	})
	if err != nil {
		t.Fatalf("CreateCustomer: %v", err)
	}

	product, err := models.CreateProduct(ctx, &models.NewProduct{
		Name:               "Mouse",
		Sku:                "MOUSE-1",
		Barcode:            "MOUSE-1",
		UnitId:             unit.ID,
		SalesPrice:         decimal.NewFromInt(35000),
		PurchasePrice:      decimal.NewFromInt(10000), // fallback should NOT be used
		SalesAccountId:     sysAccounts[models.AccountCodeSales],
		PurchaseAccountId:  sysAccounts[models.AccountCodeCostOfGoodsSold],
		InventoryAccountId: sysAccounts[models.AccountCodeInventoryAsset],
		IsBatchTracking:    utils.NewFalse(),
		OpeningStocks: []models.NewOpeningStock{
			{WarehouseId: primary.ID, Qty: decimal.NewFromInt(11), UnitValue: decimal.NewFromInt(15000)},
		},
	})
	if err != nil {
		t.Fatalf("CreateProduct: %v", err)
	}

	// Process opening stock via outbox to populate stock_histories.
	var posOutbox models.PubSubMessageRecord
	if err := db.WithContext(ctx).
		Where("business_id = ? AND reference_type = ? AND reference_id = ? AND action = ?",
			businessID, models.AccountReferenceTypeProductOpeningStock, product.ID, models.PubSubMessageActionCreate).
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

	invoiceDate := time.Date(2025, 12, 24, 0, 0, 0, 0, time.UTC)
	dueDate := invoiceDate
	invoice := models.SalesInvoice{
		BusinessId:          businessID,
		CustomerId:          customer.ID,
		BranchId:            biz.PrimaryBranchId,
		SequenceNo:          decimal.NewFromInt(1),
		InvoiceNumber:       "IV-TEST-1",
		InvoiceDate:         invoiceDate,
		InvoicePaymentTerms: models.PaymentTermsDueOnReceipt,
		InvoiceDueDate:      &dueDate,
		CurrencyId:          biz.BaseCurrencyId,
		WarehouseId:         primary.ID,
		IsTaxInclusive:      utils.NewFalse(),
		CurrentStatus:       models.SalesInvoiceStatusConfirmed,
	}
	if err := db.WithContext(ctx).Create(&invoice).Error; err != nil {
		t.Fatalf("create invoice: %v", err)
	}

	detail := models.SalesInvoiceDetail{
		SalesInvoiceId:   invoice.ID,
		ProductId:        product.ID,
		ProductType:      models.ProductTypeSingle,
		Name:             "Mouse",
		DetailQty:        decimal.NewFromInt(1),
		DetailUnitRate:   decimal.NewFromInt(35000),
		DetailAccountId:  sysAccounts[models.AccountCodeSales],
		DetailTotalAmount: decimal.NewFromInt(35000),
	}
	if err := db.WithContext(ctx).Create(&detail).Error; err != nil {
		t.Fatalf("create invoice detail: %v", err)
	}

	outgoing := models.StockHistory{
		BusinessId:        businessID,
		WarehouseId:       primary.ID,
		ProductId:         product.ID,
		ProductType:       models.ProductTypeSingle,
		BatchNumber:       "",
		StockDate:         invoiceDate,
		Qty:               decimal.NewFromInt(-1),
		Description:       "Invoice IV-TEST-1",
		ReferenceType:     models.StockReferenceTypeInvoice,
		ReferenceID:       invoice.ID,
		ReferenceDetailID: detail.ID,
		IsOutgoing:        utils.NewTrue(),
	}
	if err := db.WithContext(ctx).Create(&outgoing).Error; err != nil {
		t.Fatalf("create outgoing stock history: %v", err)
	}

	tx := db.Begin()
	if _, err := workflow.ProcessOutgoingStocks(tx, logger, []*models.StockHistory{&outgoing}); err != nil {
		_ = tx.Rollback().Error
		t.Fatalf("ProcessOutgoingStocks: %v", err)
	}
	if err := tx.Commit().Error; err != nil {
		t.Fatalf("commit outgoing: %v", err)
	}

	var refreshed models.SalesInvoiceDetail
	if err := db.WithContext(ctx).First(&refreshed, detail.ID).Error; err != nil {
		t.Fatalf("fetch invoice detail: %v", err)
	}
	expectedCogs := decimal.NewFromInt(15000)
	if !refreshed.Cogs.Equal(expectedCogs) {
		t.Fatalf("expected COGS %s got %s", expectedCogs, refreshed.Cogs)
	}

	// Attempt to over-sell: should error (negative stock not allowed)
	detail2 := models.SalesInvoiceDetail{
		SalesInvoiceId:   invoice.ID,
		ProductId:        product.ID,
		ProductType:      models.ProductTypeSingle,
		Name:             "Mouse",
		DetailQty:        decimal.NewFromInt(20),
		DetailUnitRate:   decimal.NewFromInt(35000),
		DetailAccountId:  sysAccounts[models.AccountCodeSales],
		DetailTotalAmount: decimal.NewFromInt(700000),
	}
	if err := db.WithContext(ctx).Create(&detail2).Error; err != nil {
		t.Fatalf("create invoice detail 2: %v", err)
	}

	outgoing2 := models.StockHistory{
		BusinessId:        businessID,
		WarehouseId:       primary.ID,
		ProductId:         product.ID,
		ProductType:       models.ProductTypeSingle,
		BatchNumber:       "",
		StockDate:         invoiceDate.AddDate(0, 0, 1),
		Qty:               decimal.NewFromInt(-20),
		Description:       "Invoice IV-TEST-1 (oversell)",
		ReferenceType:     models.StockReferenceTypeInvoice,
		ReferenceID:       invoice.ID,
		ReferenceDetailID: detail2.ID,
		IsOutgoing:        utils.NewTrue(),
	}
	if err := db.WithContext(ctx).Create(&outgoing2).Error; err != nil {
		t.Fatalf("create outgoing stock history 2: %v", err)
	}

	tx2 := db.Begin()
	if _, err := workflow.ProcessOutgoingStocks(tx2, logger, []*models.StockHistory{&outgoing2}); err == nil {
		_ = tx2.Rollback().Error
		t.Fatalf("expected negative stock error, got nil")
	} else {
		_ = tx2.Rollback().Error
	}
}
