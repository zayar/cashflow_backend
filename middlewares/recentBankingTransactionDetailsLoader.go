package middlewares

import (
	"context"

	"github.com/graph-gophers/dataloader/v7"
	"github.com/mmdatafocus/books_backend/models"
	"gorm.io/gorm"
)

type recentBankingTransactionReader struct {
	db *gorm.DB
}

func (r *recentBankingTransactionReader) GetRecentBankingTransactions(ctx context.Context, Ids []int) []*dataloader.Result[[]*models.BankingTransaction] {
	var results []models.BankingTransaction

	// Construct the SQL query
	err := r.db.WithContext(ctx).Raw(`
		WITH RecentTransactions AS (
			SELECT *
			FROM banking_transactions
			WHERE from_account_id IN (?)
			OR to_account_id IN (?)
		),
		RankedTransactions AS (
			SELECT *,
				ROW_NUMBER() OVER (
					PARTITION BY 
						CASE 
							WHEN from_account_id IN (?) THEN from_account_id
							WHEN to_account_id IN (?) THEN to_account_id
						END
					ORDER BY transaction_date DESC, id DESC
				) AS rank_per_account
			FROM RecentTransactions
		)
		SELECT *
		FROM RankedTransactions
		WHERE rank_per_account <= 5
		ORDER BY transaction_date DESC, id DESC;

	`, Ids, Ids, Ids, Ids).Scan(&results).Error

	if err != nil {
		return handleError[[]*models.BankingTransaction](len(Ids), err)
	}

	resultMap := make(map[int]map[int]*models.BankingTransaction)

	// Populate resultMap ensuring no duplicates
	for _, result := range results {
		// Make a copy of the current result
		currentResult := result

		if resultMap[currentResult.FromAccountId] == nil {
			resultMap[currentResult.FromAccountId] = make(map[int]*models.BankingTransaction)
		}
		if resultMap[currentResult.ToAccountId] == nil {
			resultMap[currentResult.ToAccountId] = make(map[int]*models.BankingTransaction)
		}

		resultMap[currentResult.FromAccountId][currentResult.ID] = &currentResult
		resultMap[currentResult.ToAccountId][currentResult.ID] = &currentResult
	}

	var loaderResults []*dataloader.Result[[]*models.BankingTransaction]
	for _, id := range Ids {
		var transactions []*models.BankingTransaction
		if txMap, exists := resultMap[id]; exists {
			for _, tx := range txMap {
				transactions = append(transactions, tx)
			}
		} else {
			transactions = []*models.BankingTransaction{}
		}
		loaderResults = append(loaderResults, &dataloader.Result[[]*models.BankingTransaction]{Data: transactions})
	}

	return loaderResults
}

func GetRecentBankingTransactions(ctx context.Context, orderId int) ([]*models.BankingTransaction, error) {
	loaders := For(ctx)
	return loaders.recentBankingTransactionLoader.Load(ctx, orderId)()
}
