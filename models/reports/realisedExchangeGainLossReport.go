package reports

import (
	"context"
	"errors"
	"time"

	"github.com/mmdatafocus/books_backend/config"
	"github.com/mmdatafocus/books_backend/models"
	"github.com/mmdatafocus/books_backend/utils"
	"github.com/shopspring/decimal"
)

type RealisedExchangeGainLossResponse struct {
	Date            time.Time       `json:"date"`
	TransactionType string          `json:"transaction_type"`
	CurrencyId      int             `json:"currency_id"`
	CurrencyName    string          `json:"currency_name"`
	ExchangeRate    decimal.Decimal `json:"exchange_rate"`
	RealisedAmount  decimal.Decimal `json:"realised_balance"`
	GainLossAmount  decimal.Decimal `json:"gain_loss_amount"`
}

func GetRealisedExchangeGainLossReport(ctx context.Context, branchId *int, fromDate models.MyDateString, toDate models.MyDateString) ([]*RealisedExchangeGainLossResponse, error) {
	sqlTemplate := `
		SELECT at.transaction_date_time AS date, aj.reference_type AS transaction_type, 
		c.id AS currency_id, c.name AS currency_name, at.exchange_rate AS exchange_rate, 
		at.realised_amount AS realised_amount, 
		CASE WHEN at.base_debit > 0 THEN at.base_debit ELSE -at.base_credit END AS gain_loss_amount
		FROM account_transactions at 
		INNER JOIN account_journals aj ON at.journal_id = aj.id
		INNER JOIN currencies c ON at.foreign_currency_id = c.id
		WHERE 
		at.business_id = @businessId
		{{- if .branchId }} AND at.branch_id = @branchId {{- end }}
        AND at.transaction_date_time BETWEEN @fromDate AND @toDate
		AND aj.is_reversal = 0
		AND aj.reversed_by_journal_id IS NULL
		AND at.account_id = @accountId;
	`

	businessId, ok := utils.GetBusinessIdFromContext(ctx)
	if !ok || businessId == "" {
		return nil, errors.New("business id is required")
	}
	business, err := models.GetBusiness(ctx)
	if err != nil {
		return nil, errors.New("business id is required")
	}
	if err := fromDate.StartOfDayUTCTime(business.Timezone); err != nil {
		return nil, err
	}
	if err := toDate.EndOfDayUTCTime(business.Timezone); err != nil {
		return nil, err
	}
	systemAccounts, err := models.GetSystemAccounts(businessId)
	if err != nil {
		return nil, err
	}
	if branchId != nil && *branchId != 0 {
		if err := utils.ValidateResourceId[models.Branch](ctx, businessId, branchId); err != nil {
			return nil, errors.New("branch not found")
		}
	}

	// generating sql from template
	sql, err := utils.ExecTemplate(sqlTemplate, map[string]interface{}{
		"branchId": utils.DereferencePtr(branchId),
	})
	if err != nil {
		return nil, err
	}

	var records []*RealisedExchangeGainLossResponse
	db := config.GetDB()
	if err := db.WithContext(ctx).Raw(sql, map[string]interface{}{
		"businessId": businessId,
		"fromDate":   fromDate,
		"toDate":     toDate,
		"branchId":   branchId,
		"accountId":  systemAccounts[models.AccountCodeExchangeGainOrLoss],
	}).Scan(&records).Error; err != nil {
		return nil, err
	}

	return records, nil
}
