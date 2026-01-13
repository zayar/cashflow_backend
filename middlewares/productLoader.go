package middlewares

import (
	"context"

	"bitbucket.org/mmdatafocus/books_backend/models"
	"github.com/graph-gophers/dataloader/v7"
	"gorm.io/gorm"
)

type productReader struct {
	db *gorm.DB
}

func (r *productReader) getProducts(ctx context.Context, ids []int) []*dataloader.Result[*models.Product] {
	var results []models.Product
	err := r.db.WithContext(ctx).Where("id IN ?", ids).Find(&results).Error
	if err != nil {
		return handleError[*models.Product](len(ids), err)
	}

	return generateLoaderResults(results, ids)
	// resultMap := make(map[int]*models.Product)
	// resultMap[0] = &models.Product{}
	// for _, result := range results {
	// 	resultMap[result.ID] = result
	// }

	// loaderResults := make([]*dataloader.Result[*models.Product], 0, len(ids))
	// for _, id := range ids {
	// 	result := resultMap[id]
	// 	loaderResults = append(loaderResults, &dataloader.Result[*models.Product]{Data: result})
	// }
	// return loaderResults
}

func GetProduct(ctx context.Context, id int) (*models.Product, error) {
	loaders := For(ctx)
	return loaders.productLoader.Load(ctx, id)()
}

func GetProducts(ctx context.Context, ids []int) ([]*models.Product, []error) {
	loaders := For(ctx)
	return loaders.productLoader.LoadMany(ctx, ids)()
}
