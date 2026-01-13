package middlewares

import (
	"context"

	"bitbucket.org/mmdatafocus/books_backend/models"
	"github.com/graph-gophers/dataloader/v7"
	"gorm.io/gorm"
)

type allTaxReader struct {
	db *gorm.DB
}

func (r *allTaxReader) getAllTaxs(ctx context.Context, ids []int) []*dataloader.Result[*models.AllTax] {
	resultMap, err := models.MapAllTax(ctx)
	if err != nil {
		return handleError[*models.AllTax](len(ids), err)
	}
	var loaderResults = make([]*dataloader.Result[*models.AllTax], 0, len(ids))
	for _, id := range ids {
		result, ok := resultMap[id]
		if !ok {
			var v models.AllTax
			v.ID = id
			result = &v
		}
		loaderResults = append(loaderResults, &dataloader.Result[*models.AllTax]{Data: result})
	}
	return loaderResults
}

func GetAllTax(ctx context.Context, id int) (*models.AllTax, error) {
	loaders := For(ctx)
	return loaders.allTaxLoader.Load(ctx, id)()
}

func GetAllTaxs(ctx context.Context, ids []int) ([]*models.AllTax, []error) {
	loaders := For(ctx)
	return loaders.allTaxLoader.LoadMany(ctx, ids)()
}
