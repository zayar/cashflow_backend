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

func TestSalesInvoiceEditUsesAsOfStockAndOldQty(t *testing.T) {
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

	beer, err := models.CreateProduct(ctx, &models.NewProduct{
		Name:               "A Beer",
		Sku:                "BEER-001",
		Barcode:            "BEER-001",
		UnitId:             unit.ID,
		SalesAccountId:     salesAcc,
		PurchaseAccountId:  purchaseAcc,
		InventoryAccountId: invAcc,
		IsBatchTracking:    utils.NewFalse(),
	})
	if err != nil {
		t.Fatalf("CreateProduct: %v", err)
	}
	snack, err := models.CreateProduct(ctx, &models.NewProduct{
		Name:               "B Snack",
		Sku:                "SNACK-001",
		Barcode:            "SNACK-001",
		UnitId:             unit.ID,
		SalesAccountId:     salesAcc,
		PurchaseAccountId:  purchaseAcc,
		InventoryAccountId: invAcc,
		IsBatchTracking:    utils.NewFalse(),
	})
	if err != nil {
		t.Fatalf("CreateProduct(snack): %v", err)
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

	// Seed opening stock: 10 units @ 50,000 on 01 Jan 2026.
	business, err := models.GetBusiness(ctx)
	if err != nil {
		t.Fatalf("GetBusiness: %v", err)
	}
	openingDate := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	stockDate, err := utils.ConvertToDate(openingDate, business.Timezone)
	if err != nil {
		t.Fatalf("ConvertToDate: %v", err)
	}
	logger := logrus.New()
	tx := db.Begin()
	openingBeer := &models.StockHistory{
		BusinessId:         businessID,
		WarehouseId:        primary.ID,
		ProductId:          beer.ID,
		ProductType:        models.ProductTypeSingle,
		BatchNumber:        "",
		StockDate:          stockDate,
		Qty:                decimal.NewFromInt(10),
		BaseUnitValue:      decimal.NewFromInt(50000),
		Description:        "Opening Stock",
		ReferenceType:      models.StockReferenceTypeProductOpeningStock,
		ReferenceID:        beer.ID,
		IsOutgoing:         utils.NewFalse(),
		IsTransferIn:       utils.NewFalse(),
		CumulativeSequence: 0,
	}
	openingSnack := &models.StockHistory{
		BusinessId:         businessID,
		WarehouseId:        primary.ID,
		ProductId:          snack.ID,
		ProductType:        models.ProductTypeSingle,
		BatchNumber:        "",
		StockDate:          stockDate,
		Qty:                decimal.NewFromInt(2),
		BaseUnitValue:      decimal.NewFromInt(10000),
		Description:        "Opening Stock",
		ReferenceType:      models.StockReferenceTypeProductOpeningStock,
		ReferenceID:        snack.ID,
		IsOutgoing:         utils.NewFalse(),
		IsTransferIn:       utils.NewFalse(),
		CumulativeSequence: 0,
	}
	if err := tx.WithContext(ctx).Create(openingBeer).Error; err != nil {
		tx.Rollback()
		t.Fatalf("create opening: %v", err)
	}
	if err := tx.WithContext(ctx).Create(openingSnack).Error; err != nil {
		tx.Rollback()
		t.Fatalf("create opening snack: %v", err)
	}
	if err := models.UpdateStockSummaryOpeningQty(tx.WithContext(ctx), businessID, primary.ID, beer.ID, string(models.ProductTypeSingle), "", decimal.NewFromInt(10), stockDate); err != nil {
		tx.Rollback()
		t.Fatalf("UpdateStockSummaryOpeningQty: %v", err)
	}
	if err := models.UpdateStockSummaryOpeningQty(tx.WithContext(ctx), businessID, primary.ID, snack.ID, string(models.ProductTypeSingle), "", decimal.NewFromInt(2), stockDate); err != nil {
		tx.Rollback()
		t.Fatalf("UpdateStockSummaryOpeningQty(snack): %v", err)
	}
	if _, err := workflow.ProcessIncomingStocks(tx.WithContext(ctx), logger, []*models.StockHistory{openingBeer, openingSnack}); err != nil {
		tx.Rollback()
		t.Fatalf("ProcessIncomingStocks(opening): %v", err)
	}
	if err := tx.Commit().Error; err != nil {
		t.Fatalf("commit opening: %v", err)
	}

	isTaxInclusive := false
	invDate1 := time.Date(2026, 1, 2, 12, 0, 0, 0, time.UTC)
	inv1, err := models.CreateSalesInvoice(ctx, &models.NewSalesInvoice{
		CustomerId:          customer.ID,
		BranchId:            biz.PrimaryBranchId,
		InvoiceDate:         invDate1,
		InvoicePaymentTerms: models.PaymentTermsDueOnReceipt,
		CurrencyId:          biz.BaseCurrencyId,
		ExchangeRate:        decimal.NewFromInt(1),
		WarehouseId:         primary.ID,
		IsTaxInclusive:      &isTaxInclusive,
		CurrentStatus:       models.SalesInvoiceStatusConfirmed,
		Details: []models.NewSalesInvoiceDetail{{
			ProductId:      beer.ID,
			ProductType:    models.ProductTypeSingle,
			BatchNumber:    "",
			Name:           "A Beer",
			Description:    "",
			DetailQty:      decimal.NewFromInt(1),
			DetailUnitRate: decimal.NewFromInt(50000),
		}},
	})
	if err != nil {
		t.Fatalf("CreateSalesInvoice(inv1): %v", err)
	}
	var inv1Outbox models.PubSubMessageRecord
	if err := db.WithContext(ctx).
		Where("business_id = ? AND reference_type = ? AND reference_id = ? AND action = ?",
			businessID, models.AccountReferenceTypeInvoice, inv1.ID, models.PubSubMessageActionCreate).
		Order("id DESC").
		First(&inv1Outbox).Error; err != nil {
		t.Fatalf("expected outbox record for inv1: %v", err)
	}
	wtx1 := db.Begin()
	if err := workflow.ProcessInvoiceWorkflow(wtx1, logger, models.ConvertToPubSubMessage(inv1Outbox)); err != nil {
		t.Fatalf("ProcessInvoiceWorkflow(inv1): %v", err)
	}
	if err := wtx1.Commit().Error; err != nil {
		t.Fatalf("inv1 workflow commit: %v", err)
	}

	// Second invoice later in time to reduce current on-hand.
	invDate2 := time.Date(2026, 1, 5, 12, 0, 0, 0, time.UTC)
	inv2, err := models.CreateSalesInvoice(ctx, &models.NewSalesInvoice{
		CustomerId:          customer.ID,
		BranchId:            biz.PrimaryBranchId,
		InvoiceDate:         invDate2,
		InvoicePaymentTerms: models.PaymentTermsDueOnReceipt,
		CurrencyId:          biz.BaseCurrencyId,
		ExchangeRate:        decimal.NewFromInt(1),
		WarehouseId:         primary.ID,
		IsTaxInclusive:      &isTaxInclusive,
		CurrentStatus:       models.SalesInvoiceStatusConfirmed,
		Details: []models.NewSalesInvoiceDetail{{
			ProductId:      beer.ID,
			ProductType:    models.ProductTypeSingle,
			BatchNumber:    "",
			Name:           "A Beer",
			Description:    "",
			DetailQty:      decimal.NewFromInt(8),
			DetailUnitRate: decimal.NewFromInt(50000),
		}},
	})
	if err != nil {
		t.Fatalf("CreateSalesInvoice(inv2): %v", err)
	}
	var inv2Outbox models.PubSubMessageRecord
	if err := db.WithContext(ctx).
		Where("business_id = ? AND reference_type = ? AND reference_id = ? AND action = ?",
			businessID, models.AccountReferenceTypeInvoice, inv2.ID, models.PubSubMessageActionCreate).
		Order("id DESC").
		First(&inv2Outbox).Error; err != nil {
		t.Fatalf("expected outbox record for inv2: %v", err)
	}
	wtx2 := db.Begin()
	if err := workflow.ProcessInvoiceWorkflow(wtx2, logger, models.ConvertToPubSubMessage(inv2Outbox)); err != nil {
		t.Fatalf("ProcessInvoiceWorkflow(inv2): %v", err)
	}
	if err := wtx2.Commit().Error; err != nil {
		t.Fatalf("inv2 workflow commit: %v", err)
	}

	// Reload invoice 1 with details to get detail id.
	var inv1Reload models.SalesInvoice
	if err := db.WithContext(ctx).Preload("Details").
		Where("business_id = ? AND id = ?", businessID, inv1.ID).
		First(&inv1Reload).Error; err != nil {
		t.Fatalf("reload inv1: %v", err)
	}
	if len(inv1Reload.Details) != 1 {
		t.Fatalf("expected 1 invoice detail for inv1")
	}
	detailID := inv1Reload.Details[0].ID

	// Edit inv1: increase qty to 5 (should PASS using as-of date + old qty).
	_, err = models.UpdateSalesInvoice(ctx, inv1.ID, &models.NewSalesInvoice{
		CustomerId:          customer.ID,
		BranchId:            biz.PrimaryBranchId,
		InvoiceDate:         invDate1,
		InvoicePaymentTerms: models.PaymentTermsDueOnReceipt,
		CurrencyId:          biz.BaseCurrencyId,
		ExchangeRate:        decimal.NewFromInt(1),
		WarehouseId:         primary.ID,
		IsTaxInclusive:      &isTaxInclusive,
		CurrentStatus:       models.SalesInvoiceStatusConfirmed,
		Details: []models.NewSalesInvoiceDetail{{
			DetailId:       detailID,
			ProductId:      beer.ID,
			ProductType:    models.ProductTypeSingle,
			BatchNumber:    "",
			Name:           "A Beer",
			Description:    "",
			DetailQty:      decimal.NewFromInt(5),
			DetailUnitRate: decimal.NewFromInt(50000),
		}},
	})
	if err != nil {
		t.Fatalf("UpdateSalesInvoice(inv1->5) unexpected error: %v", err)
	}
	var inv1UpdateOutbox models.PubSubMessageRecord
	if err := db.WithContext(ctx).
		Where("business_id = ? AND reference_type = ? AND reference_id = ? AND action = ?",
			businessID, models.AccountReferenceTypeInvoice, inv1.ID, models.PubSubMessageActionUpdate).
		Order("id DESC").
		First(&inv1UpdateOutbox).Error; err != nil {
		t.Fatalf("expected outbox record for inv1 update: %v", err)
	}
	wtxU := db.Begin()
	if err := workflow.ProcessInvoiceWorkflow(wtxU, logger, models.ConvertToPubSubMessage(inv1UpdateOutbox)); err != nil {
		t.Fatalf("ProcessInvoiceWorkflow(inv1 update): %v", err)
	}
	if err := wtxU.Commit().Error; err != nil {
		t.Fatalf("inv1 update workflow commit: %v", err)
	}

	// Edit inv1: add a new line item (should PASS using as-of date stock).
	_, err = models.UpdateSalesInvoice(ctx, inv1.ID, &models.NewSalesInvoice{
		CustomerId:          customer.ID,
		BranchId:            biz.PrimaryBranchId,
		InvoiceDate:         invDate1,
		InvoicePaymentTerms: models.PaymentTermsDueOnReceipt,
		CurrencyId:          biz.BaseCurrencyId,
		ExchangeRate:        decimal.NewFromInt(1),
		WarehouseId:         primary.ID,
		IsTaxInclusive:      &isTaxInclusive,
		CurrentStatus:       models.SalesInvoiceStatusConfirmed,
		Details: []models.NewSalesInvoiceDetail{
			{
				DetailId:       detailID,
				ProductId:      beer.ID,
				ProductType:    models.ProductTypeSingle,
				BatchNumber:    "",
				Name:           "A Beer",
				Description:    "",
				DetailQty:      decimal.NewFromInt(5),
				DetailUnitRate: decimal.NewFromInt(50000),
			},
			{
				ProductId:      snack.ID,
				ProductType:    models.ProductTypeSingle,
				BatchNumber:    "",
				Name:           "B Snack",
				Description:    "",
				DetailQty:      decimal.NewFromInt(1),
				DetailUnitRate: decimal.NewFromInt(10000),
			},
		},
	})
	if err != nil {
		t.Fatalf("UpdateSalesInvoice(inv1 add snack) unexpected error: %v", err)
	}
	var inv1AddOutbox models.PubSubMessageRecord
	if err := db.WithContext(ctx).
		Where("business_id = ? AND reference_type = ? AND reference_id = ? AND action = ?",
			businessID, models.AccountReferenceTypeInvoice, inv1.ID, models.PubSubMessageActionUpdate).
		Order("id DESC").
		First(&inv1AddOutbox).Error; err != nil {
		t.Fatalf("expected outbox record for inv1 add snack: %v", err)
	}
	wtxAdd := db.Begin()
	if err := workflow.ProcessInvoiceWorkflow(wtxAdd, logger, models.ConvertToPubSubMessage(inv1AddOutbox)); err != nil {
		t.Fatalf("ProcessInvoiceWorkflow(inv1 add snack): %v", err)
	}
	if err := wtxAdd.Commit().Error; err != nil {
		t.Fatalf("inv1 add snack workflow commit: %v", err)
	}

	// Reload invoice 1 to get detail ids for both lines.
	var inv1Reload2 models.SalesInvoice
	if err := db.WithContext(ctx).Preload("Details").
		Where("business_id = ? AND id = ?", businessID, inv1.ID).
		First(&inv1Reload2).Error; err != nil {
		t.Fatalf("reload inv1 after snack: %v", err)
	}
	var beerDetailID int
	var snackDetailID int
	for _, d := range inv1Reload2.Details {
		switch d.ProductId {
		case beer.ID:
			beerDetailID = d.ID
		case snack.ID:
			snackDetailID = d.ID
		}
	}
	if beerDetailID == 0 || snackDetailID == 0 {
		t.Fatalf("expected detail ids for beer and snack after update")
	}

	// Edit inv1: increase qty beyond available-for-edit (should FAIL).
	_, err = models.UpdateSalesInvoice(ctx, inv1.ID, &models.NewSalesInvoice{
		CustomerId:          customer.ID,
		BranchId:            biz.PrimaryBranchId,
		InvoiceDate:         invDate1,
		InvoicePaymentTerms: models.PaymentTermsDueOnReceipt,
		CurrencyId:          biz.BaseCurrencyId,
		ExchangeRate:        decimal.NewFromInt(1),
		WarehouseId:         primary.ID,
		IsTaxInclusive:      &isTaxInclusive,
		CurrentStatus:       models.SalesInvoiceStatusConfirmed,
		Details: []models.NewSalesInvoiceDetail{
			{
				DetailId:       beerDetailID,
				ProductId:      beer.ID,
				ProductType:    models.ProductTypeSingle,
				BatchNumber:    "",
				Name:           "A Beer",
				Description:    "",
				DetailQty:      decimal.NewFromInt(11),
				DetailUnitRate: decimal.NewFromInt(50000),
			},
			{
				DetailId:       snackDetailID,
				ProductId:      snack.ID,
				ProductType:    models.ProductTypeSingle,
				BatchNumber:    "",
				Name:           "B Snack",
				Description:    "",
				DetailQty:      decimal.NewFromInt(1),
				DetailUnitRate: decimal.NewFromInt(10000),
			},
		},
	})
	if err == nil {
		t.Fatalf("expected insufficient stock error for inv1->11, got nil")
	}
}
