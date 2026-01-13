package middlewares

import (
	"context"

	"bitbucket.org/mmdatafocus/books_backend/models"
	"github.com/graph-gophers/dataloader/v7"
	"gorm.io/gorm"
)

type purchaseOrderDetailReader struct {
	db *gorm.DB
}

func (r *purchaseOrderDetailReader) GetPurchaseOrderDetails(ctx context.Context, Ids []int) []*dataloader.Result[[]*models.PurchaseOrderDetail] {
	var results []models.PurchaseOrderDetail
	err := r.db.WithContext(ctx).Where("purchase_order_id IN ?", Ids).Find(&results).Error
	if err != nil {
		return handleError[[]*models.PurchaseOrderDetail](len(Ids), err)
	}

	return generateLoaderArrayResults(results, Ids)
	// // key => customer id (int)
	// // value => array of billing address pointer []*PurchaseOrderDetaile
	// resultMap := make(map[int][]*models.PurchaseOrderDetail)
	// for _, result := range results {
	// 	resultMap[result.PurchaseOrderId] = append(resultMap[result.PurchaseOrderId], result)
	// }
	// var loaderResults []*dataloader.Result[[]*models.PurchaseOrderDetail]
	// for _, id := range Ids {
	// 	purchaseOrderDetails := resultMap[id]
	// 	loaderResults = append(loaderResults, &dataloader.Result[[]*models.PurchaseOrderDetail]{Data: purchaseOrderDetails})
	// }
	// return loaderResults
}

func GetPurchaseOrderDetails(ctx context.Context, orderId int) ([]*models.PurchaseOrderDetail, error) {
	loaders := For(ctx)
	return loaders.purchaseOrderDetailLoader.Load(ctx, orderId)()
}
