package middlewares

import (
	"context"

	"bitbucket.org/mmdatafocus/books_backend/models"
	"github.com/graph-gophers/dataloader/v7"
	"gorm.io/gorm"
)

type allWarehouseReader struct {
	db *gorm.DB
}

func (r *allWarehouseReader) getAllWarehouses(ctx context.Context, ids []int) []*dataloader.Result[*models.AllWarehouse] {
	resultMap, err := models.MapAllWarehouse(ctx)
	if err != nil {
		return handleError[*models.AllWarehouse](len(ids), err)
	}
	var loaderResults = make([]*dataloader.Result[*models.AllWarehouse], 0, len(ids))
	for _, id := range ids {
		result, ok := resultMap[id]
		if !ok {
			var v models.AllWarehouse
			v.ID = id
			result = &v
		}
		loaderResults = append(loaderResults, &dataloader.Result[*models.AllWarehouse]{Data: result})
	}
	return loaderResults
}

func GetAllWarehouse(ctx context.Context, id int) (*models.AllWarehouse, error) {
	loaders := For(ctx)
	return loaders.allWarehouseLoader.Load(ctx, id)()
}

func GetAllWarehouses(ctx context.Context, ids []int) ([]*models.AllWarehouse, []error) {
	loaders := For(ctx)
	return loaders.allWarehouseLoader.LoadMany(ctx, ids)()
}
