package middlewares

import (
	"context"

	"github.com/graph-gophers/dataloader/v7"
	"github.com/mmdatafocus/books_backend/models"
	"gorm.io/gorm"
)

type salesOrderReader struct {
	db *gorm.DB
}

func (r *salesOrderReader) getSalesOrders(ctx context.Context, ids []int) []*dataloader.Result[*models.SalesOrder] {
	var results []models.SalesOrder
	err := r.db.WithContext(ctx).Where("id IN ?", ids).Find(&results).Error
	if err != nil {
		return handleError[*models.SalesOrder](len(ids), err)
	}

	return generateLoaderResults(results, ids)
	// resultMap := make(map[int]*models.SalesOrder)
	// resultMap[0] = &models.SalesOrder{}
	// for _, result := range results {
	// 	resultMap[result.ID] = result
	// }

	// loaderResults := make([]*dataloader.Result[*models.SalesOrder], 0, len(ids))
	// for _, id := range ids {
	// 	result := resultMap[id]
	// 	loaderResults = append(loaderResults, &dataloader.Result[*models.SalesOrder]{Data: result})
	// }
	// return loaderResults
}

func GetSalesOrder(ctx context.Context, id int) (*models.SalesOrder, error) {
	loaders := For(ctx)
	return loaders.salesOrderLoader.Load(ctx, id)()
}

func GetSalesOrders(ctx context.Context, ids []int) ([]*models.SalesOrder, []error) {
	loaders := For(ctx)
	return loaders.salesOrderLoader.LoadMany(ctx, ids)()
}
