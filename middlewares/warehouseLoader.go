package middlewares

import (
	"context"

	"github.com/graph-gophers/dataloader/v7"
	"github.com/mmdatafocus/books_backend/models"
	"gorm.io/gorm"
)

type warehouseReader struct {
	db *gorm.DB
}

func (r *warehouseReader) getWarehouses(ctx context.Context, ids []int) []*dataloader.Result[*models.Warehouse] {
	var results []models.Warehouse
	err := r.db.WithContext(ctx).
		Where("id IN ?", ids).Find(&results).Error
	if err != nil {
		return handleError[*models.Warehouse](len(ids), err)
	}

	return generateLoaderResults(results, ids)
	// resultMap := make(map[int]*models.Warehouse)
	// resultMap[0] = &models.Warehouse{}
	// for _, result := range results {
	// 	resultMap[result.ID] = result
	// }

	// loaderResults := make([]*dataloader.Result[*models.Warehouse], 0, len(ids))
	// for _, id := range ids {
	// 	result := resultMap[id]
	// 	loaderResults = append(loaderResults, &dataloader.Result[*models.Warehouse]{Data: result})
	// }

	// return loaderResults
}

func GetWarehouse(ctx context.Context, id int) (*models.Warehouse, error) {
	loaders := For(ctx)
	return loaders.warehouseLoader.Load(ctx, id)()
}

func GetWarehouses(ctx context.Context, ids []int) ([]*models.Warehouse, []error) {
	loaders := For(ctx)
	return loaders.warehouseLoader.LoadMany(ctx, ids)()
}
