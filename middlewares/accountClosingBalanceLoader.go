package middlewares

import (
	"context"

	"bitbucket.org/mmdatafocus/books_backend/models"
	"github.com/graph-gophers/dataloader/v7"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

type accountClosingBalanceReader struct {
	db *gorm.DB
}

func (r *accountClosingBalanceReader) GetAccountClosingBalances(ctx context.Context, ids []int) []*dataloader.Result[*models.AccountCurrencyDailyBalance] {
	business, err := models.GetBusiness(ctx)
	if err != nil {
		return handleError[*models.AccountCurrencyDailyBalance](len(ids), err)
	}

	var results []*models.AccountCurrencyDailyBalance
	query := `WITH lastBalances AS (
				SELECT 
					acdb.account_id, 
					acdb.running_balance,
					a.main_type,
					ROW_NUMBER() OVER(PARTITION BY acdb.account_id ORDER BY acdb.transaction_date DESC) AS row_num
				FROM 
					account_currency_daily_balances acdb
				JOIN 
					accounts a ON acdb.account_id = a.id
				WHERE 
					acdb.account_id IN (?) 
					AND acdb.branch_id = 0 
					AND acdb.currency_id = ?
			)
			SELECT 
				account_id,
				CASE 
					WHEN main_type = 'Liability' THEN -running_balance
					WHEN main_type = 'Equity' THEN -running_balance
					WHEN main_type = 'Income' THEN -running_balance
					ELSE running_balance
				END AS running_balance
			FROM 
				lastBalances 
			WHERE 
				row_num = 1;
`

	// Execute the raw SQL query
	err = r.db.WithContext(ctx).Raw(query, ids, business.BaseCurrencyId).Scan(&results).Error
	if err != nil {
		return handleError[*models.AccountCurrencyDailyBalance](len(ids), err)
	}

	resultMap := make(map[int]*models.AccountCurrencyDailyBalance)
	resultMap[0] = &models.AccountCurrencyDailyBalance{AccountId: 0, RunningBalance: decimal.NewFromInt(0)}
	for _, result := range results {
		resultMap[result.AccountId] = result
	}
	loaderResults := make([]*dataloader.Result[*models.AccountCurrencyDailyBalance], 0, len(ids))
	for _, id := range ids {
		loaderResults = append(loaderResults, &dataloader.Result[*models.AccountCurrencyDailyBalance]{Data: resultMap[id]})
	}

	return loaderResults
}

func GetAccountClosingBalance(ctx context.Context, id int) (*models.AccountCurrencyDailyBalance, error) {
	loaders := For(ctx)
	return loaders.accountClosingBalanceLoader.Load(ctx, id)()
}

func GetAccountClosingBalances(ctx context.Context, ids []int) ([]*models.AccountCurrencyDailyBalance, []error) {
	loaders := For(ctx)
	return loaders.accountClosingBalanceLoader.LoadMany(ctx, ids)()
}
