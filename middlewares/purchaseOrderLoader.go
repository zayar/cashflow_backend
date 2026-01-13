package middlewares

import (
	"context"

	"bitbucket.org/mmdatafocus/books_backend/models"
	"github.com/graph-gophers/dataloader/v7"
	"gorm.io/gorm"
)

type purchaseOrderReader struct {
	db *gorm.DB
}

func (r *purchaseOrderReader) getPurchaseOrders(ctx context.Context, ids []int) []*dataloader.Result[*models.PurchaseOrder] {
	var results []models.PurchaseOrder
	err := r.db.WithContext(ctx).Where("id IN ?", ids).Find(&results).Error
	if err != nil {
		return handleError[*models.PurchaseOrder](len(ids), err)
	}

	return generateLoaderResults(results, ids)
	// resultMap := make(map[int]*models.PurchaseOrder)
	// resultMap[0] = &models.PurchaseOrder{}
	// for _, result := range results {
	// 	resultMap[result.ID] = result
	// }

	// loaderResults := make([]*dataloader.Result[*models.PurchaseOrder], 0, len(ids))
	// for _, id := range ids {
	// 	result := resultMap[id]
	// 	loaderResults = append(loaderResults, &dataloader.Result[*models.PurchaseOrder]{Data: result})
	// }
	// return loaderResults
}

func GetPurchaseOrder(ctx context.Context, id int) (*models.PurchaseOrder, error) {
	loaders := For(ctx)
	return loaders.purchaseOrderLoader.Load(ctx, id)()
}

func GetPurchaseOrders(ctx context.Context, ids []int) ([]*models.PurchaseOrder, []error) {
	loaders := For(ctx)
	return loaders.purchaseOrderLoader.LoadMany(ctx, ids)()
}
