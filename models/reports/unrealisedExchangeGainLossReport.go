package reports

import (
	"context"
	"errors"

	"bitbucket.org/mmdatafocus/books_backend/config"
	"bitbucket.org/mmdatafocus/books_backend/models"
	"bitbucket.org/mmdatafocus/books_backend/utils"
	"github.com/shopspring/decimal"
)

type UserDefinedExchangeRate struct {
	CurrencyId   int             `json:"currency_id"`
	ExchangeRate decimal.Decimal `json:"exchange_rate"`
}

type UnrealisedExchangeGainLossResponse struct {
	AccountId             int             `json:"account_id"`
	AccountName           string          `json:"account_name"`
	CurrencyId            int             `json:"currency_id"`
	CurrencyName          string          `json:"currency_name"`
	ForeignClosingBalance decimal.Decimal `json:"foreign_closing_balance"`
	BaseClosingBalance    decimal.Decimal `json:"base_closing_balance"`
	ExchangeRate          decimal.Decimal `json:"exchange_rate"`
	RevaluedBalance       decimal.Decimal `json:"revalued_balance"`
	GainLossAmount        decimal.Decimal `json:"gain_loss_amount"`
}

func GetUnrealisedExchangeGainLossReport(ctx context.Context, branchId *int, toDate models.MyDateString, rates []*UserDefinedExchangeRate) ([]*UnrealisedExchangeGainLossResponse, error) {
	sql := `
		WITH LatestTransactions AS (
			SELECT *,
				ROW_NUMBER() OVER (PARTITION BY account_id ORDER BY transaction_date DESC) AS rn
			FROM account_currency_daily_balances
			WHERE 
			business_id = @businessId
			AND branch_id = @branchId
			AND transaction_date <= @toDate
			AND currency_id != @baseCurrencyId
		)
		SELECT
			a.id AS account_id, a.name AS account_name, 
			c.id AS currency_id, c.name AS currency_name, 
			lt.running_balance AS foreign_closing_balance, 
			lt.running_base_balance AS base_closing_balance
		FROM LatestTransactions lt
		INNER JOIN accounts a ON lt.account_id = a.id
		INNER JOIN currencies c ON lt.currency_id = c.id
		WHERE lt.rn = 1
		AND a.detail_type IN ('AccountsReceivable', 'AccountsPayable', 'Bank')
	`

	businessId, ok := utils.GetBusinessIdFromContext(ctx)
	if !ok || businessId == "" {
		return nil, errors.New("business id is required")
	}
	business, err := models.GetBusiness(ctx)
	if err != nil {
		return nil, errors.New("business id is required")
	}
	if err := toDate.EndOfDayUTCTime(business.Timezone); err != nil {
		return nil, err
	}

	if branchId != nil && *branchId != 0 {
		if err := utils.ValidateResourceId[models.Branch](ctx, businessId, branchId); err != nil {
			return nil, errors.New("branch not found")
		}
	}

	// generating sql from template
	// sql, err := utils.ExecTemplate(sqlTemplate, map[string]interface{}{
	// 	"branchId": utils.DereferencePtr(branchId),
	// })
	// if err != nil {
	// 	return nil, err
	// }

	branch := 0
	if branchId != nil && *branchId != 0 {
		branch = *branchId
	}
	var records []*UnrealisedExchangeGainLossResponse
	db := config.GetDB()
	if err := db.WithContext(ctx).Raw(sql, map[string]interface{}{
		"businessId":     businessId,
		"toDate":         toDate,
		"baseCurrencyId": business.BaseCurrencyId,
		"branchId":       branch,
	}).Scan(&records).Error; err != nil {
		return nil, err
	}

	// Calculate revalued balance and gain/loss amount
	for _, record := range records {
		// Find the user-defined exchange rate for the current currency
		var exchangeRate decimal.Decimal
		for _, rate := range rates {
			if rate.CurrencyId == record.CurrencyId {
				exchangeRate = rate.ExchangeRate
				break
			}
		}

		if exchangeRate.IsZero() {
			// If no user-defined rate is found, use 1 as the exchange rate
			exchangeRate = decimal.NewFromInt(1)
		}
		record.ExchangeRate = exchangeRate

		// Calculate the revalued balance based on the user-defined exchange rate
		record.RevaluedBalance = record.ForeignClosingBalance.Mul(exchangeRate)

		// Calculate the gain/loss amount as the difference between the revalued balance and the base closing balance
		record.GainLossAmount = record.RevaluedBalance.Sub(record.BaseClosingBalance)
	}

	return records, nil
}
