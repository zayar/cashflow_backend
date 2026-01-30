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

func TestCustomerBalanceSummary_CreditNotes_IssuedAppliedRemaining_AsOf(t *testing.T) {
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
		Name:  "AR Credit Notes Co",
		Email: "owner@arcredit.test",
	})
	if err != nil {
		t.Fatalf("CreateBusiness: %v", err)
	}
	businessID := biz.ID.String()
	ctx = utils.SetBusinessIdInContext(ctx, businessID)

	cust, err := models.CreateCustomer(ctx, &models.NewCustomer{Name: "Smile"})
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

	// Base-currency scenario:
	// Invoice 77,000; Credit Note 20,000; apply 20,000.
	invoiceDate := time.Date(2026, 1, 1, 10, 0, 0, 0, time.UTC)
	creditNoteDate := time.Date(2026, 1, 5, 10, 0, 0, 0, time.UTC)
	applyTime := time.Date(2026, 1, 15, 10, 0, 0, 0, time.UTC) // application happens later

	// Invoice journal (AR debit)
	ivJ := models.AccountJournal{
		BusinessId:          businessID,
		BranchId:            biz.PrimaryBranchId,
		CustomerId:          cust.ID,
		TransactionDateTime: invoiceDate,
		TransactionNumber:   "IV-1",
		ReferenceId:         1,
		ReferenceType:       models.AccountReferenceTypeInvoice,
		AccountTransactions: []models.AccountTransaction{
			{
				BusinessId:          businessID,
				AccountId:           ar,
				BranchId:            biz.PrimaryBranchId,
				TransactionDateTime: invoiceDate,
				BaseCurrencyId:      biz.BaseCurrencyId,
				ForeignCurrencyId:   biz.BaseCurrencyId,
				BaseDebit:           decimal.NewFromInt(77000),
				BaseCredit:          decimal.Zero,
				ExchangeRate:        decimal.NewFromInt(1),
				IsInventoryValuation: utils.NewFalse(),
				IsTransferIn:         utils.NewFalse(),
			},
			{
				BusinessId:          businessID,
				AccountId:           sales,
				BranchId:            biz.PrimaryBranchId,
				TransactionDateTime: invoiceDate,
				BaseCurrencyId:      biz.BaseCurrencyId,
				ForeignCurrencyId:   biz.BaseCurrencyId,
				BaseDebit:           decimal.Zero,
				BaseCredit:          decimal.NewFromInt(77000),
				ExchangeRate:        decimal.NewFromInt(1),
				IsInventoryValuation: utils.NewFalse(),
				IsTransferIn:         utils.NewFalse(),
			},
		},
	}
	if err := db.WithContext(ctx).Create(&ivJ).Error; err != nil {
		t.Fatalf("create invoice journal: %v", err)
	}

	cn := models.CreditNote{
		BusinessId:            businessID,
		CustomerId:            cust.ID,
		BranchId:              biz.PrimaryBranchId,
		CreditNoteNumber:      "CN-1",
		SequenceNo:            decimal.NewFromInt(1),
		CreditNoteDate:        creditNoteDate,
		WarehouseId:           1,
		CurrencyId:            biz.BaseCurrencyId,
		ExchangeRate:          decimal.NewFromInt(1),
		CurrentStatus:         models.CreditNoteStatusConfirmed,
		IsTaxInclusive:        utils.NewFalse(),
		CreditNoteTotalAmount: decimal.NewFromInt(20000),
		RemainingBalance:      decimal.NewFromInt(20000),
	}
	if err := db.WithContext(ctx).Create(&cn).Error; err != nil {
		t.Fatalf("create credit note: %v", err)
	}

	apply := models.CustomerCreditInvoice{
		BusinessId:           businessID,
		ReferenceId:          cn.ID,
		ReferenceType:        models.CustomerCreditApplyTypeCredit,
		BranchId:             biz.PrimaryBranchId,
		CustomerId:           cust.ID,
		InvoiceId:            1,
		CreditDate:           creditNoteDate,
		Amount:               decimal.NewFromInt(20000),
		CustomerCreditNumber: cn.CreditNoteNumber,
		InvoiceNumber:        "IV-1",
		CurrencyId:           biz.BaseCurrencyId,
		ExchangeRate:         decimal.NewFromInt(1),
		InvoiceCurrencyId:    biz.BaseCurrencyId,
		InvoiceExchangeRate:  decimal.NewFromInt(1),
		CreatedAt:            applyTime,
		UpdatedAt:            applyTime,
	}
	if err := db.WithContext(ctx).Create(&apply).Error; err != nil {
		t.Fatalf("create credit application: %v", err)
	}

	// As-of BEFORE apply: should show issued=20k, applied=0, remaining=20k.
	asOfBefore := models.MyDateString(time.Date(2026, 1, 10, 0, 0, 0, 0, time.UTC))
	rows, err := reports.GetCustomerBalanceSummaryReport(ctx, &asOfBefore, nil)
	if err != nil {
		t.Fatalf("GetCustomerBalanceSummaryReport(before): %v", err)
	}
	assertCustomerBalanceRow(t, rows, "Smile", func(r *reports.CustomerBalance) {
		if !r.InvoiceBalance.Equal(decimal.NewFromInt(77000)) {
			t.Fatalf("before: invoice_balance expected 77000, got %s", r.InvoiceBalance)
		}
		if !r.CreditNoteIssued.Equal(decimal.NewFromInt(20000)) {
			t.Fatalf("before: credit_note_issued expected 20000, got %s", r.CreditNoteIssued)
		}
		if !r.CreditApplied.Equal(decimal.Zero) {
			t.Fatalf("before: credit_applied expected 0, got %s", r.CreditApplied)
		}
		if !r.RemainingCredit.Equal(decimal.NewFromInt(20000)) {
			t.Fatalf("before: remaining_credit expected 20000, got %s", r.RemainingCredit)
		}
		if !r.ClosingBalance.Equal(decimal.NewFromInt(57000)) {
			t.Fatalf("before: closing_balance expected 57000, got %s", r.ClosingBalance)
		}
	})

	// As-of AFTER apply: should show issued=20k, applied=20k, remaining=0; closing still 57k.
	asOfAfter := models.MyDateString(time.Date(2026, 1, 31, 0, 0, 0, 0, time.UTC))
	rows, err = reports.GetCustomerBalanceSummaryReport(ctx, &asOfAfter, nil)
	if err != nil {
		t.Fatalf("GetCustomerBalanceSummaryReport(after): %v", err)
	}
	assertCustomerBalanceRow(t, rows, "Smile", func(r *reports.CustomerBalance) {
		if !r.InvoiceBalance.Equal(decimal.NewFromInt(77000)) {
			t.Fatalf("after: invoice_balance expected 77000, got %s", r.InvoiceBalance)
		}
		if !r.CreditNoteIssued.Equal(decimal.NewFromInt(20000)) {
			t.Fatalf("after: credit_note_issued expected 20000, got %s", r.CreditNoteIssued)
		}
		if !r.CreditApplied.Equal(decimal.NewFromInt(20000)) {
			t.Fatalf("after: credit_applied expected 20000, got %s", r.CreditApplied)
		}
		if !r.RemainingCredit.Equal(decimal.Zero) {
			t.Fatalf("after: remaining_credit expected 0, got %s", r.RemainingCredit)
		}
		if !r.ClosingBalance.Equal(decimal.NewFromInt(57000)) {
			t.Fatalf("after: closing_balance expected 57000, got %s", r.ClosingBalance)
		}
	})
}

func assertCustomerBalanceRow(t *testing.T, rows []*reports.CustomerBalance, customerName string, assert func(r *reports.CustomerBalance)) {
	t.Helper()
	for _, r := range rows {
		if r.CustomerName == customerName {
			assert(r)
			return
		}
	}
	t.Fatalf("expected customer row for %s", customerName)
}

