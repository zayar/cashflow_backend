package models

import (
	"context"
	"errors"

	"github.com/mmdatafocus/books_backend/config"
	"github.com/mmdatafocus/books_backend/utils"
	"github.com/shopspring/decimal"
)

type BankingAccount struct {
	ID                int               `json:"id"`
	Name              string            `json:"name"`
	Branches          string            `json:"branches"`
	Code              *string           `json:"code,omitempty"`
	CurrencyId        int               `json:"currencyId"`
	DetailType        AccountDetailType `json:"detailType"`
	MainType          AccountMainType   `json:"mainType"`
	IsActive          bool              `json:"isActive"`
	SystemDefaultCode *string           `json:"systemDefaultCode,omitempty"`
	Balance           *decimal.Decimal  `json:"balance,omitempty"`
	AccountNumber     string            `json:"account_number"`
	Description       string            `json:"description"`
	ParentAccountId   int               `json:"parentAccountId"`
}

type TotalSummary struct {
	DetailType     string          `json:"detail_type"`
	CurrencySymbol string          `json:"currency_symbol"`
	TotalBalance   decimal.Decimal `json:"total_balance"`
}

type ListBankingAccountResponse struct {
	ListBankingAccount []*BankingAccount `json:"listBankingAccount"`
	TotalSummary       []TotalSummary    `json:"totalSummary"`
}

// func ListBankingAccount(ctx context.Context) ([]*BankingAccount, error) {
func ListBankingAccount(ctx context.Context) (*ListBankingAccountResponse, error) {
	businessId, ok := utils.GetBusinessIdFromContext(ctx)
	if !ok || businessId == "" {
		return nil, errors.New("business id is required")
	}
	bankingTypes := []string{
		"Bank",
		"Cash",
		"CreditCard",
	}
	bankingAccounts := make([]*BankingAccount, 0)
	db := config.GetDB()
	dbCtx := db.WithContext(ctx).Model(&Account{}).Where("business_id = ?", businessId)
	dbCtx.Where("detail_type IN ?", bankingTypes)
	if err := dbCtx.Select("id",
		"name",
		"code",
		"detail_type",
		"main_type",
		"is_active",
		"system_default_code",
		"currency_id",
		"branches",
		"parent_account_id",
		"account_number",
		"description",
	).Scan(&bankingAccounts).Error; err != nil {
		return nil, err
	}

	for _, ba := range bankingAccounts {
		amount, err := ba.GetBankingAccountBalance(ctx)
		if err != nil {
			return nil, err
		}
		ba.Balance = &amount
	}

	totalSummary, err := GetTotalSummary(ctx, businessId)
	if err != nil {
		return nil, err
	}

	response := &ListBankingAccountResponse{
		ListBankingAccount: bankingAccounts,
		TotalSummary:       totalSummary,
	}

	return response, nil

	// return bankingAccounts, nil
}

func (ba BankingAccount) GetBankingAccountBalance(ctx context.Context) (decimal.Decimal, error) {
	var amount decimal.Decimal
	businessId, ok := utils.GetBusinessIdFromContext(ctx)
	if !ok || businessId == "" {
		return amount, errors.New("business id is required")
	}
	db := config.GetDB()
	// err := db.WithContext(ctx).Table("account_currency_daily_balances").
	// 	// Where("account_id = ? AND currency_id = ?", ba.ID, ba.CurrencyId).
	// 	Where("account_id = ? AND branch_id = ?", ba.ID, 0).
	// 	Order("transaction_date DESC").Limit(1).Select("running_balance").Scan(&amount).Error
	// IMPORTANT: Always filter by business_id and LIMIT 1 to avoid cross-tenant scans
	// and expensive sorts that can make the Banking page "spin forever".
	err := db.WithContext(ctx).Raw(`
		SELECT acdb.running_balance
		FROM account_currency_daily_balances acdb
		WHERE acdb.business_id = ?
			AND acdb.branch_id = ?
			AND acdb.account_id = ?
		ORDER BY acdb.transaction_date DESC
		LIMIT 1;
	`, businessId, 0, ba.ID).Scan(&amount).Error
	if err != nil {
		return amount, err
	}

	return amount, nil
}

func GetTotalSummary(ctx context.Context, businessId string) ([]TotalSummary, error) {
	db := config.GetDB()
	totalSummary := make([]TotalSummary, 0)
	accountTypes := []string{
		string(AccountDetailTypeCash),
		string(AccountDetailTypeBank),
	}
	err := db.WithContext(ctx).Raw(`
		WITH LastRows AS (
			SELECT 
				acb.account_id AS account_id,
				acb.currency_id AS currency_id,
				c.name AS currency_name,
				c.symbol AS currency_symbol,
				ac.detail_type AS detail_type,
				ac.name AS account_name,
				acb.running_balance,
				ROW_NUMBER() OVER (PARTITION BY acb.account_id ORDER BY acb.transaction_date DESC) AS row_num
			FROM 
				account_currency_daily_balances AS acb
			JOIN
				accounts AS ac ON acb.account_id = ac.id
			JOIN
				currencies AS c ON acb.currency_id = c.id
			WHERE 
				acb.business_id = ?
				AND acb.branch_id = ?
				AND acb.currency_id = ac.currency_id
				AND acb.account_id IN (
					SELECT id FROM accounts WHERE detail_type IN ? 
				)
		)
		SELECT 
			detail_type,
			currency_symbol,
			SUM(running_balance) AS total_balance
		FROM 
			LastRows
		WHERE 
			row_num = 1
		GROUP BY 
			detail_type,
			currency_symbol
		ORDER BY 
			detail_type desc;
	`, businessId, 0, accountTypes).Scan(&totalSummary).Error

	if err != nil {
		return nil, err
	}

	return totalSummary, nil
}
