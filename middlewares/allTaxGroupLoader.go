package middlewares

import (
	"context"

	"github.com/graph-gophers/dataloader/v7"
	"github.com/mmdatafocus/books_backend/models"
	"gorm.io/gorm"
)

type allTaxGroupReader struct {
	db *gorm.DB
}

func (r *allTaxGroupReader) getAllTaxGroups(ctx context.Context, ids []int) []*dataloader.Result[*models.AllTaxGroup] {
	resultMap, err := models.MapAllTaxGroup(ctx)
	if err != nil {
		return handleError[*models.AllTaxGroup](len(ids), err)
	}
	var loaderResults = make([]*dataloader.Result[*models.AllTaxGroup], 0, len(ids))
	for _, id := range ids {
		result, ok := resultMap[id]
		if !ok {
			var v models.AllTaxGroup
			v.ID = id
			result = &v
		}
		loaderResults = append(loaderResults, &dataloader.Result[*models.AllTaxGroup]{Data: result})
	}
	return loaderResults
}

func GetAllTaxGroup(ctx context.Context, id int) (*models.AllTaxGroup, error) {
	loaders := For(ctx)
	return loaders.allTaxGroupLoader.Load(ctx, id)()
}

func GetAllTaxGroups(ctx context.Context, ids []int) ([]*models.AllTaxGroup, []error) {
	loaders := For(ctx)
	return loaders.allTaxGroupLoader.LoadMany(ctx, ids)()
}
