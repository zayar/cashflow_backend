package middlewares

import (
	"context"

	"github.com/graph-gophers/dataloader/v7"
	"github.com/mmdatafocus/books_backend/models"
	"gorm.io/gorm"
)

type productUnitReader struct {
	db *gorm.DB
}

func (r *productUnitReader) getProductUnits(ctx context.Context, ids []int) []*dataloader.Result[*models.ProductUnit] {
	var results []models.ProductUnit
	err := r.db.WithContext(ctx).Where("id IN ?", ids).Find(&results).Error
	if err != nil {
		return handleError[*models.ProductUnit](len(ids), err)
	}

	// resultMap := make(map[int]*models.ProductUnit)
	// resultMap[0] = &models.ProductUnit{}
	// for _, result := range results {
	// 	resultMap[result.ID] = result
	// }

	// loaderResults := make([]*dataloader.Result[*models.ProductUnit], 0, len(ids))
	// for _, id := range ids {
	// 	result := resultMap[id]
	// 	loaderResults = append(loaderResults, &dataloader.Result[*models.ProductUnit]{Data: result})
	// }
	return generateLoaderResults[models.ProductUnit](results, ids)
}

func GetProductUnit(ctx context.Context, id int) (*models.ProductUnit, error) {
	loaders := For(ctx)
	return loaders.productUnitLoader.Load(ctx, id)()
}

func GetProductUnits(ctx context.Context, ids []int) ([]*models.ProductUnit, []error) {
	loaders := For(ctx)
	return loaders.productUnitLoader.LoadMany(ctx, ids)()
}
