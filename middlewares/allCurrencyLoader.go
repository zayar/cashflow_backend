package middlewares

import (
	"context"

	"bitbucket.org/mmdatafocus/books_backend/models"
	"github.com/graph-gophers/dataloader/v7"
	"gorm.io/gorm"
)

type allCurrencyReader struct {
	db *gorm.DB
}

func (r *allCurrencyReader) getAllCurrencys(ctx context.Context, ids []int) []*dataloader.Result[*models.AllCurrency] {
	resultMap, err := models.MapAllCurrency(ctx)
	if err != nil {
		return handleError[*models.AllCurrency](len(ids), err)
	}
	var loaderResults = make([]*dataloader.Result[*models.AllCurrency], 0, len(ids))
	for _, id := range ids {
		result, ok := resultMap[id]
		if !ok {
			var v models.AllCurrency
			v.ID = id
			result = &v
		}
		loaderResults = append(loaderResults, &dataloader.Result[*models.AllCurrency]{Data: result})
	}
	return loaderResults
}

func GetAllCurrency(ctx context.Context, id int) (*models.AllCurrency, error) {
	loaders := For(ctx)
	return loaders.allCurrencyLoader.Load(ctx, id)()
}

func GetAllCurrencys(ctx context.Context, ids []int) ([]*models.AllCurrency, []error) {
	loaders := For(ctx)
	return loaders.allCurrencyLoader.LoadMany(ctx, ids)()
}
