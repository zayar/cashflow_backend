package middlewares

import (
	"context"

	"github.com/graph-gophers/dataloader/v7"
	"gorm.io/gorm"

	"github.com/mmdatafocus/books_backend/models"
)

type accountReader struct {
	db *gorm.DB
}

func (r *accountReader) getAccounts(ctx context.Context, ids []int) []*dataloader.Result[*models.Account] {
	var results []models.Account

	err := r.db.WithContext(ctx).Where("id IN ?", ids).Find(&results).Error
	if err != nil {
		return handleError[*models.Account](len(ids), err)
	}
	return generateLoaderResults(results, ids)

	// resultMap := make(map[int]*models.Account)
	// resultMap[0] = &models.Account{IsActive: utils.NewFalse(), CreatedAt: time.Now(), UpdatedAt: time.Now()}
	// for _, result := range results {
	// 	resultMap[result.ID] = result
	// }

	// loaderResults := make([]*dataloader.Result[*models.Account], 0, len(ids))
	// for _, id := range ids {
	// 	result := resultMap[id]
	// 	loaderResults = append(loaderResults, &dataloader.Result[*models.Account]{Data: result})
	// }
	// return loaderResults
}

// GetUser returns single user by id efficiently
func GetAccount(ctx context.Context, id int) (*models.Account, error) {
	loaders := For(ctx)
	return loaders.AccountLoader.Load(ctx, id)()
}

// GetUsers returns many users by ids efficiently
func GetAccounts(ctx context.Context, ids []int) ([]*models.Account, []error) {
	loaders := For(ctx)
	return loaders.AccountLoader.LoadMany(ctx, ids)()
}
