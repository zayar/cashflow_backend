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
	"github.com/shopspring/decimal"
)

// Regression: Customer Balance Summary must NOT double-count invoices when journals are reposted
// via reversal + replacement (REV-* entries).
func TestCustomerBalanceSummary_IgnoresReversalJournals(t *testing.T) {
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
		Name:  "AR Report Co",
		Email: "owner@ar.test",
	})
	if err != nil {
		t.Fatalf("CreateBusiness: %v", err)
	}
	businessID := biz.ID.String()
	ctx = utils.SetBusinessIdInContext(ctx, businessID)

	// Customer
	cust, err := models.CreateCustomer(ctx, &models.NewCustomer{
		Name: "Christ",
	})
	if err != nil {
		t.Fatalf("CreateCustomer: %v", err)
	}

	accs, err := models.GetSystemAccounts(businessID)
	if err != nil {
		t.Fatalf("GetSystemAccounts: %v", err)
	}
	ar := accs[models.AccountCodeAccountsReceivable]
	sales := accs[models.AccountCodeSales]

	db := config.GetDB()
	txDate := time.Date(2025, 11, 6, 10, 0, 0, 0, time.UTC)

	// 1) Original invoice journal (active initially)
	orig := models.AccountJournal{
		BusinessId:          businessID,
		BranchId:            biz.PrimaryBranchId,
		CustomerId:          cust.ID,
		TransactionDateTime: txDate,
		TransactionNumber:   "IV-1",
		ReferenceId:         1,
		ReferenceType:       models.AccountReferenceTypeInvoice,
		AccountTransactions: []models.AccountTransaction{
			{
				BusinessId:         businessID,
				AccountId:          ar,
				BranchId:           biz.PrimaryBranchId,
				TransactionDateTime: txDate,
				BaseCurrencyId:     biz.BaseCurrencyId,
				ForeignCurrencyId:  biz.BaseCurrencyId,
				BaseDebit:          decimal.NewFromInt(240000),
				BaseCredit:         decimal.Zero,
				ForeignDebit:       decimal.Zero,
				ForeignCredit:      decimal.Zero,
				ExchangeRate:       decimal.NewFromInt(1),
				IsInventoryValuation: utils.NewFalse(),
				IsTransferIn:         utils.NewFalse(),
			},
			{
				BusinessId:         businessID,
				AccountId:          sales,
				BranchId:           biz.PrimaryBranchId,
				TransactionDateTime: txDate,
				BaseCurrencyId:     biz.BaseCurrencyId,
				ForeignCurrencyId:  biz.BaseCurrencyId,
				BaseDebit:          decimal.Zero,
				BaseCredit:         decimal.NewFromInt(240000),
				ForeignDebit:       decimal.Zero,
				ForeignCredit:      decimal.Zero,
				ExchangeRate:       decimal.NewFromInt(1),
				IsInventoryValuation: utils.NewFalse(),
				IsTransferIn:         utils.NewFalse(),
			},
		},
	}
	if err := db.WithContext(ctx).Create(&orig).Error; err != nil {
		t.Fatalf("create original journal: %v", err)
	}

	// 2) Reversal journal (REV-IV-1), marks original as reversed.
	rev := models.AccountJournal{
		BusinessId:          businessID,
		BranchId:            biz.PrimaryBranchId,
		CustomerId:          cust.ID,
		TransactionDateTime: txDate.Add(1 * time.Minute),
		TransactionNumber:   "REV-IV-1",
		ReferenceId:         1,
		ReferenceType:       models.AccountReferenceTypeInvoice,
		IsReversal:          true,
		ReversesJournalId:   &orig.ID,
		AccountTransactions: []models.AccountTransaction{
			{
				BusinessId:         businessID,
				AccountId:          ar,
				BranchId:           biz.PrimaryBranchId,
				TransactionDateTime: txDate.Add(1 * time.Minute),
				BaseCurrencyId:     biz.BaseCurrencyId,
				ForeignCurrencyId:  biz.BaseCurrencyId,
				BaseDebit:          decimal.Zero,
				BaseCredit:         decimal.NewFromInt(240000),
				ForeignDebit:       decimal.Zero,
				ForeignCredit:      decimal.Zero,
				ExchangeRate:       decimal.NewFromInt(1),
				IsInventoryValuation: utils.NewFalse(),
				IsTransferIn:         utils.NewFalse(),
			},
		},
	}
	if err := db.WithContext(ctx).Create(&rev).Error; err != nil {
		t.Fatalf("create reversal journal: %v", err)
	}
	// Link original -> reversed by rev.
	if err := db.WithContext(ctx).Model(&models.AccountJournal{}).
		Where("id = ?", orig.ID).
		Updates(map[string]interface{}{"reversed_by_journal_id": rev.ID}).Error; err != nil {
		t.Fatalf("mark original reversed: %v", err)
	}

	// 3) Replacement journal (active) with same invoice amount.
	repl := models.AccountJournal{
		BusinessId:          businessID,
		BranchId:            biz.PrimaryBranchId,
		CustomerId:          cust.ID,
		TransactionDateTime: txDate.Add(2 * time.Minute),
		TransactionNumber:   "IV-1",
		ReferenceId:         1,
		ReferenceType:       models.AccountReferenceTypeInvoice,
		AccountTransactions: []models.AccountTransaction{
			{
				BusinessId:         businessID,
				AccountId:          ar,
				BranchId:           biz.PrimaryBranchId,
				TransactionDateTime: txDate.Add(2 * time.Minute),
				BaseCurrencyId:     biz.BaseCurrencyId,
				ForeignCurrencyId:  biz.BaseCurrencyId,
				BaseDebit:          decimal.NewFromInt(240000),
				BaseCredit:         decimal.Zero,
				ForeignDebit:       decimal.Zero,
				ForeignCredit:      decimal.Zero,
				ExchangeRate:       decimal.NewFromInt(1),
				IsInventoryValuation: utils.NewFalse(),
				IsTransferIn:         utils.NewFalse(),
			},
		},
	}
	if err := db.WithContext(ctx).Create(&repl).Error; err != nil {
		t.Fatalf("create replacement journal: %v", err)
	}

	end := models.MyDateString(time.Date(2026, 1, 31, 0, 0, 0, 0, time.UTC))
	rows, err := reports.GetCustomerBalanceSummaryReport(ctx, &end, nil)
	if err != nil {
		t.Fatalf("GetCustomerBalanceSummaryReport: %v", err)
	}

	// Find the customer row (base currency) and assert invoice balance is 240,000 (not doubled).
	found := false
	for _, r := range rows {
		if r.CustomerName == "Christ" {
			found = true
			if !r.InvoiceBalance.Equal(decimal.NewFromInt(240000)) {
				t.Fatalf("expected invoice_balance=240000, got %s", r.InvoiceBalance)
			}
			break
		}
	}
	if !found {
		t.Fatalf("expected customer row for Christ")
	}
}

