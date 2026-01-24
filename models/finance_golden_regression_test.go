package models_test

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
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

// Finance golden regression harness.
//
// Non-negotiable safety: this test is intended to catch changes that would alter:
// - inventory valuation outputs
// - journal entries (debits/credits) produced by workflows
//
// Usage:
// - Run (requires Docker): INTEGRATION_TESTS=1 go test ./models -run FinanceGolden -v
// - Update golden snapshot: INTEGRATION_TESTS=1 UPDATE_GOLDEN=1 go test ./models -run FinanceGolden -v
//
// Golden files live under models/testdata/golden/.

type goldenJournalTxn struct {
	AccountId      int             `json:"account_id"`
	BaseDebit      decimal.Decimal `json:"base_debit"`
	BaseCredit     decimal.Decimal `json:"base_credit"`
	ForeignDebit   decimal.Decimal `json:"foreign_debit"`
	ForeignCredit  decimal.Decimal `json:"foreign_credit"`
	Description    string          `json:"description"`
	CurrencyId     int             `json:"currency_id"`
	ExchangeRate   decimal.Decimal `json:"exchange_rate"`
}

type goldenJournal struct {
	TransactionDateTime string           `json:"transaction_date_time"`
	ReferenceType       string           `json:"reference_type"`
	ReferenceId         int              `json:"reference_id"`
	TransactionNumber   string           `json:"transaction_number"`
	ReferenceNumber     string           `json:"reference_number"`
	Transactions        []goldenJournalTxn `json:"transactions"`
}

type financeSnapshot struct {
	AsOfDate string `json:"as_of_date"`

	InventoryValuationAllWarehouses []*models.InventoryValuationSummaryResponse `json:"inventory_valuation_all_warehouses"`
	WarehouseInventoryReport        []*models.WarehouseInventoryResponse        `json:"warehouse_inventory_report"`
	JournalReport                   []goldenJournal                            `json:"journal_report"`
}

func snapshotPath(name string) string {
	return filepath.Join("models", "testdata", "golden", name+".json")
}

func loadOrUpdateGolden(t *testing.T, path string, actual any) {
	t.Helper()

	update := strings.TrimSpace(os.Getenv("UPDATE_GOLDEN")) != ""
	b, err := os.ReadFile(path)
	if err != nil {
		if update {
			if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
				t.Fatalf("mkdir golden dir: %v", err)
			}
			out, merr := json.MarshalIndent(actual, "", "  ")
			if merr != nil {
				t.Fatalf("marshal golden: %v", merr)
			}
			if werr := os.WriteFile(path, out, 0o644); werr != nil {
				t.Fatalf("write golden: %v", werr)
			}
			t.Logf("wrote golden snapshot: %s", path)
			return
		}
		t.Skipf("golden snapshot missing (%s). Re-run with UPDATE_GOLDEN=1 to generate.", path)
	}

	var expected any
	if err := json.Unmarshal(b, &expected); err != nil {
		t.Fatalf("unmarshal golden (%s): %v", path, err)
	}
	ab, _ := json.Marshal(actual)
	var got any
	_ = json.Unmarshal(ab, &got)

	// Simple structural compare via JSON; stable because we normalize IDs.
	if string(ab) != string(mustMarshalJSON(t, expected)) {
		// Provide readable diff payloads.
		prettyExpected, _ := json.MarshalIndent(expected, "", "  ")
		prettyActual, _ := json.MarshalIndent(got, "", "  ")
		t.Fatalf("finance regression mismatch\n\nEXPECTED (%s):\n%s\n\nACTUAL:\n%s\n", path, string(prettyExpected), string(prettyActual))
	}
}

func mustMarshalJSON(t *testing.T, v any) []byte {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal json: %v", err)
	}
	return b
}

func normalizeJournal(j []*models.AccountJournal) []goldenJournal {
	out := make([]goldenJournal, 0, len(j))
	for _, row := range j {
		if row == nil {
			continue
		}
		txs := make([]goldenJournalTxn, 0, len(row.AccountTransactions))
		for _, t := range row.AccountTransactions {
			txs = append(txs, goldenJournalTxn{
				AccountId:     t.AccountId,
				BaseDebit:     t.BaseDebit,
				BaseCredit:    t.BaseCredit,
				ForeignDebit:  t.ForeignDebit,
				ForeignCredit: t.ForeignCredit,
				Description:   t.Description,
				CurrencyId:    t.BaseCurrencyId,
				ExchangeRate:  t.ExchangeRate,
			})
		}
		out = append(out, goldenJournal{
			TransactionDateTime: row.TransactionDateTime.UTC().Format(time.RFC3339Nano),
			ReferenceType:       string(row.ReferenceType),
			ReferenceId:         row.ReferenceId,
			TransactionNumber:   row.TransactionNumber,
			ReferenceNumber:     row.ReferenceNumber,
			Transactions:        txs,
		})
	}
	return out
}

func TestFinanceGolden_TransferOrderScenario(t *testing.T) {
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
		Name:  "Golden Biz",
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

	green, err := models.CreateWarehouse(ctx, &models.NewWarehouse{
		BranchId: biz.PrimaryBranchId,
		Name:     "Green Warehouse",
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
			{WarehouseId: primary.ID, Qty: decimal.NewFromInt(25), UnitValue: decimal.NewFromInt(5000)},
		},
	})
	if err != nil {
		t.Fatalf("CreateProduct: %v", err)
	}

	transferDate := time.Date(2026, 1, 3, 12, 0, 0, 0, time.UTC)
	to, err := models.CreateTransferOrder(ctx, &models.NewTransferOrder{
		OrderNumber:            "TO-0001",
		TransferDate:           transferDate,
		ReasonId:               reason.ID,
		SourceWarehouseId:      primary.ID,
		DestinationWarehouseId: green.ID,
		CurrentStatus:          models.TransferOrderStatusConfirmed,
		Details: []models.NewTransferOrderDetail{
			{ProductId: stapler.ID, ProductType: models.ProductTypeSingle, Name: "Stapler", TransferQty: decimal.NewFromInt(10)},
		},
	})
	if err != nil {
		t.Fatalf("CreateTransferOrder: %v", err)
	}

	// Process outbox to produce stock_histories + journals.
	var outbox models.PubSubMessageRecord
	if err := db.WithContext(ctx).
		Where("business_id = ? AND reference_type = ? AND reference_id = ?", businessID, models.AccountReferenceTypeTransferOrder, to.ID).
		Order("id DESC").
		First(&outbox).Error; err != nil {
		t.Fatalf("expected outbox record for transfer order: %v", err)
	}
	logger := logrus.New()
	wtx := db.Begin()
	if err := workflow.ProcessTransferOrderWorkflow(wtx, logger, models.ConvertToPubSubMessage(outbox)); err != nil {
		t.Fatalf("ProcessTransferOrderWorkflow: %v", err)
	}
	if err := wtx.Commit().Error; err != nil {
		t.Fatalf("ProcessTransferOrderWorkflow commit: %v", err)
	}

	asOf := models.MyDateString(time.Date(2026, 1, 31, 0, 0, 0, 0, time.UTC))
	val, err := models.GetInventoryValuationSummaryLedger(ctx, asOf, 0)
	if err != nil {
		t.Fatalf("GetInventoryValuationSummaryLedger: %v", err)
	}
	warehouseRows, err := reports.GetWarehouseInventoryReport(ctx, asOf)
	if err != nil {
		t.Fatalf("GetWarehouseInventoryReport: %v", err)
	}
	jFrom := models.MyDateString(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))
	jTo := models.MyDateString(time.Date(2026, 1, 31, 0, 0, 0, 0, time.UTC))
	journals, err := reports.GetAllJournalReport(ctx, jFrom, jTo, "", nil)
	if err != nil {
		t.Fatalf("GetAllJournalReport: %v", err)
	}

	snap := financeSnapshot{
		AsOfDate:                       time.Time(asOf).UTC().Format(time.RFC3339Nano),
		InventoryValuationAllWarehouses: val,
		WarehouseInventoryReport:        warehouseRows,
		JournalReport:                   normalizeJournal(journals),
	}

	loadOrUpdateGolden(t, snapshotPath("finance_transfer_order"), snap)
}

