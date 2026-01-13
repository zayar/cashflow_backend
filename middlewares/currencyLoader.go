package middlewares

import (
	"context"

	"bitbucket.org/mmdatafocus/books_backend/models"
	"github.com/graph-gophers/dataloader/v7"
	"gorm.io/gorm"
)

type currencyReader struct {
	db *gorm.DB
}

func (r *currencyReader) getCurrencies(ctx context.Context, ids []int) []*dataloader.Result[*models.Currency] {
	var results []models.Currency
	err := r.db.WithContext(ctx).
		Where("id IN ?", ids).Find(&results).Error
	if err != nil {
		return handleError[*models.Currency](len(ids), err)
	}
	return generateLoaderResults(results, ids)

	// resultMap := make(map[int]*models.Currency)
	// resultMap[0] = &models.Currency{}
	// for _, result := range results {
	// 	resultMap[result.ID] = result
	// }

	// loaderResults := make([]*dataloader.Result[*models.Currency], 0, len(ids))
	// for _, id := range ids {
	// 	result := resultMap[id]
	// 	loaderResults = append(loaderResults, &dataloader.Result[*models.Currency]{Data: result})
	// }
	// return loaderResults
}

func GetCurrency(ctx context.Context, id int) (*models.Currency, error) {
	loaders := For(ctx)
	return loaders.CurrencyLoader.Load(ctx, id)()
}

func GetCurrencies(ctx context.Context, ids []int) ([]*models.Currency, []error) {
	loaders := For(ctx)
	return loaders.CurrencyLoader.LoadMany(ctx, ids)()
}
