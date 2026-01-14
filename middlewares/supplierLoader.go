package middlewares

import (
	"context"

	"github.com/graph-gophers/dataloader/v7"
	"github.com/mmdatafocus/books_backend/models"
	"gorm.io/gorm"
)

type supplierReader struct {
	db *gorm.DB
}

func (r *supplierReader) getSuppliers(ctx context.Context, ids []int) []*dataloader.Result[*models.Supplier] {
	var results []models.Supplier
	err := r.db.WithContext(ctx).Where("id IN ?", ids).Find(&results).Error
	if err != nil {
		return handleError[*models.Supplier](len(ids), err)
	}

	return generateLoaderResults(results, ids)
	// resultMap := make(map[int]*models.Supplier)
	// resultMap[0] = &models.Supplier{}
	// for _, result := range results {
	// 	resultMap[result.ID] = result
	// }

	// loaderResults := make([]*dataloader.Result[*models.Supplier], 0, len(ids))
	// for _, id := range ids {
	// 	result := resultMap[id]
	// 	loaderResults = append(loaderResults, &dataloader.Result[*models.Supplier]{Data: result})
	// }
	// return loaderResults
}

func GetSupplier(ctx context.Context, id int) (*models.Supplier, error) {
	loaders := For(ctx)
	return loaders.supplierLoader.Load(ctx, id)()
}

func GetSuppliers(ctx context.Context, ids []int) ([]*models.Supplier, []error) {
	loaders := For(ctx)
	return loaders.supplierLoader.LoadMany(ctx, ids)()
}
