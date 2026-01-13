package middlewares

import (
// "context"

// "bitbucket.org/mmdatafocus/books_backend/models"
// "github.com/graph-gophers/dataloader/v7"
// "gorm.io/gorm"
)

// type productStockReader struct {
// 	db *gorm.DB
// }

// func (r *productStockReader) GetProductStocks(ctx context.Context, productIds []int) []*dataloader.Result[[]*models.Stock] {
// 	var results []*models.Stock
// 	err := r.db.WithContext(ctx).Where("product_id IN ?", productIds).Find(&results).Error
// 	if err != nil {
// 		return handleError[[]*models.Stock](len(productIds), err)
// 	}

// 	// key => customer id (int)
// 	// value => array of billing address pointer []*Stock
// 	resultMap := make(map[int][]*models.Stock)
// 	for _, result := range results {
// 		resultMap[result.ProductId] = append(resultMap[result.ProductId], result)
// 	}
// 	var loaderResults []*dataloader.Result[[]*models.Stock]
// 	for _, id := range productIds {
// 		stocks := resultMap[id]
// 		loaderResults = append(loaderResults, &dataloader.Result[[]*models.Stock]{Data: stocks})
// 	}
// 	return loaderResults
// }

// func GetProductStocks(ctx context.Context, productId int) ([]*models.Stock, error) {
// 	loaders := For(ctx)
// 	return loaders.productStockLoader.Load(ctx, productId)()
// }
