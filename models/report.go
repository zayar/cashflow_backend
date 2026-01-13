package models

import (
	"time"

	"github.com/shopspring/decimal"
)

type DetailedGeneralLedger struct {
	AccountId          int
	AccountName        string
	CurrencyId         int
	CurrencyName       string
	CurrencySymbol     string
	OpeningBalance     decimal.Decimal
	OpeningBalanceDate time.Time
	ClosingBalance     decimal.Decimal
	ClosingBalanceDate time.Time
	Transactions       []*DetailLedgerTransaction
}

type DetailLedgerTransaction struct {
	AccountId           int
	AccountName         string
	BranchId            int
	BaseCurrencyId      int
	CurrencyName        string
	CurrencySymbol      string
	TransactionDateTime time.Time
	Description         string
	Debit               decimal.Decimal
	Credit              decimal.Decimal
	ForeignDebit        decimal.Decimal
	ForeignCredit       decimal.Decimal
	ExchangeRate        decimal.Decimal
	// BaseClosingBalance  decimal.Decimal
	TransactionType    string
	TransactionNumber  string
	TransactionDetails string
	ReferenceNumber    string
	CustomerName       string
	SupplierName       string
	ForeignCurrencyName        string
	ForeignCurrencySymbol      string
}

type AccountSummary struct {
	AccountId       int
	AccountName     string
	AccountCode     string
	AccountMainType string
	Code            string
	Debit           decimal.Decimal
	Credit          decimal.Decimal
	Balance         decimal.Decimal
	OpeningBalance  decimal.Decimal
	ClosingBalance  decimal.Decimal
}

type AccountSummaryGroup struct {
	AccountMainType  string
	AccountSummaries []AccountSummary
}

type TrialBalance struct {
	AccountId       int
	AccountMainType string
	AccountName     string
	AccountCode     string
	Credit          decimal.Decimal
	Debit           decimal.Decimal
}

type BalanceSheet struct {
	Id               int
	AccountMainType  string
	AccountGroupType string
	AccountSubType   string
	AccountName      string
	ParentAccountName string
	ParentAccountId int
	AccountId        int
	Amount           decimal.Decimal
}

type SubAccount struct {
    AccountName       string       `json:"account_name"`
    AccountId         int          `json:"account_id"`
    Amount            decimal.Decimal `json:"amount"`
    ParentAccountName string       `json:"parent_account_name"`
}

type SubType struct {
	AccountName 	  string
	AccountId   	  int
	ParentAccountName string
	Amount      	  decimal.Decimal
	Total             decimal.Decimal `json:"total,omitempty"` 
    SubAccounts       []SubAccount `json:"subAccounts,omitempty"`
}

type GroupType struct {
	SubType  string
	Total    decimal.Decimal
	Accounts []SubType
}

type MainType struct {
	GroupType string
	Total     decimal.Decimal
	Accounts  []GroupType
}

type BalanceSheetResponse struct {
	MainType string
	Total    decimal.Decimal
	Accounts []MainType
}

type ProfitAndLossResponse struct {
	GrossProfit     decimal.Decimal
	OperatingProfit decimal.Decimal
	NetProfit       decimal.Decimal
	PlAccountGroups []PlAccountGroup
}

type PlAccountGroup struct {
	GroupType string
	Total     decimal.Decimal
	Accounts  []AccountGroupItem
}

type AccountGroupItem struct {
	MainType    string
	DetailType  string
	AccountName string
	AccountID   int
	Amount      decimal.Decimal
}
