package middlewares

import (
	"context"

	"github.com/graph-gophers/dataloader/v7"
	"github.com/mmdatafocus/books_backend/models"
	"gorm.io/gorm"
)

type recentAccountTransactionReader struct {
	db *gorm.DB
}

func (r *recentAccountTransactionReader) GetRecentAccountTransactions(ctx context.Context, ids []int) []*dataloader.Result[[]*models.AccountTransaction] {
	var results []*models.AccountTransaction
	// err := r.db.WithContext(ctx).Where("account_id IN ?", ids).Order("transaction_date_time DESC").Limit(5).Find(&results).Error
	query := `WITH recentTransactions AS (
                    SELECT *,
                           ROW_NUMBER() OVER(PARTITION BY account_id ORDER BY transaction_date_time DESC) AS row_num
                    FROM account_transactions
                    WHERE account_id IN (?)
             )
             SELECT * FROM recentTransactions WHERE row_num <= 5`

	// Execute the raw SQL query
	err := r.db.WithContext(ctx).Raw(query, ids).Scan(&results).Error
	if err != nil {
		return handleError[[]*models.AccountTransaction](len(ids), err)
	}

	// key =>  account id (int)
	// value => array of billing address pointer []*AccountTransaction
	resultMap := make(map[int][]*models.AccountTransaction)
	for _, result := range results {
		resultMap[result.AccountId] = append(resultMap[result.AccountId], result)
	}
	var loaderResults []*dataloader.Result[[]*models.AccountTransaction]
	for _, id := range ids {
		accTransactions := resultMap[id]
		loaderResults = append(loaderResults, &dataloader.Result[[]*models.AccountTransaction]{Data: accTransactions})
	}
	return loaderResults
}

func GetRecentAccountTransactions(ctx context.Context, id int) ([]*models.AccountTransaction, error) {
	loaders := For(ctx)
	return loaders.recentAccountTransactionLoader.Load(ctx, id)()
}
