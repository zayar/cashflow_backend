package middlewares

import (
	"context"

	"github.com/graph-gophers/dataloader/v7"
	"github.com/mmdatafocus/books_backend/models"
	"gorm.io/gorm"
)

type salesOrderDetailReader struct {
	db *gorm.DB
}

func (r *salesOrderDetailReader) GetSalesOrderDetails(ctx context.Context, Ids []int) []*dataloader.Result[[]*models.SalesOrderDetail] {
	var results []models.SalesOrderDetail
	err := r.db.WithContext(ctx).Where("sales_order_id IN ?", Ids).Find(&results).Error
	if err != nil {
		return handleError[[]*models.SalesOrderDetail](len(Ids), err)
	}

	return generateLoaderArrayResults(results, Ids)
	// // key => customer id (int)
	// // value => array of billing address pointer []*SalesOrderDetaile
	// resultMap := make(map[int][]*models.SalesOrderDetail)
	// for _, result := range results {
	// 	resultMap[result.SalesOrderId] = append(resultMap[result.SalesOrderId], result)
	// }
	// var loaderResults []*dataloader.Result[[]*models.SalesOrderDetail]
	// for _, id := range Ids {
	// 	salesOrderDetails := resultMap[id]
	// 	loaderResults = append(loaderResults, &dataloader.Result[[]*models.SalesOrderDetail]{Data: salesOrderDetails})
	// }
	// return loaderResults
}

func GetSalesOrderDetails(ctx context.Context, orderId int) ([]*models.SalesOrderDetail, error) {
	loaders := For(ctx)
	return loaders.salesOrderDetailLoader.Load(ctx, orderId)()
}
