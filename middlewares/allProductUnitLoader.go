package middlewares

import (
	"context"

	"bitbucket.org/mmdatafocus/books_backend/models"
	"github.com/graph-gophers/dataloader/v7"
	"gorm.io/gorm"
)

type allProductUnitReader struct {
	db *gorm.DB
}

func (r *allProductUnitReader) getAllProductUnits(ctx context.Context, ids []int) []*dataloader.Result[*models.AllProductUnit] {
	resultMap, err := models.MapAllProductUnit(ctx)
	if err != nil {
		return handleError[*models.AllProductUnit](len(ids), err)
	}
	var loaderResults = make([]*dataloader.Result[*models.AllProductUnit], 0, len(ids))
	for _, id := range ids {
		result, ok := resultMap[id]
		if !ok {
			var v models.AllProductUnit
			v.ID = id
			result = &v
		}
		loaderResults = append(loaderResults, &dataloader.Result[*models.AllProductUnit]{Data: result})
	}
	return loaderResults
}

func GetAllProductUnit(ctx context.Context, id int) (*models.AllProductUnit, error) {
	loaders := For(ctx)
	return loaders.allProductUnitLoader.Load(ctx, id)()
}

func GetAllProductUnits(ctx context.Context, ids []int) ([]*models.AllProductUnit, []error) {
	loaders := For(ctx)
	return loaders.allProductUnitLoader.LoadMany(ctx, ids)()
}
