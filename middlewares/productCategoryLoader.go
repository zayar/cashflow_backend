package middlewares

import (
	"context"

	"bitbucket.org/mmdatafocus/books_backend/models"
	"github.com/graph-gophers/dataloader/v7"
	"gorm.io/gorm"
)

type productCategoryReader struct {
	db *gorm.DB
}

func (r *productCategoryReader) getProductCategories(ctx context.Context, ids []int) []*dataloader.Result[*models.ProductCategory] {
	var results []models.ProductCategory
	err := r.db.WithContext(ctx).Where("id IN ?", ids).Find(&results).Error
	if err != nil {
		return handleError[*models.ProductCategory](len(ids), err)
	}

	return generateLoaderResults(results, ids)
	// resultMap := make(map[int]*models.ProductCategory)
	// resultMap[0] = &models.ProductCategory{
	// 	IsActive: utils.NewFalse(),
	// }
	// for _, result := range results {
	// 	resultMap[result.ID] = result
	// }

	// loaderResults := make([]*dataloader.Result[*models.ProductCategory], 0, len(ids))
	// for _, id := range ids {
	// 	result := resultMap[id]
	// 	if result == nil {
	// 		// copying the value instead of passing the pointer
	// 		defaultResult := *resultMap[0]
	// 		defaultResult.ID = id
	// 		result = &defaultResult
	// 	}
	// 	loaderResults = append(loaderResults, &dataloader.Result[*models.ProductCategory]{Data: result})
	// }
	// return loaderResults
}

func GetProductCategory(ctx context.Context, id int) (*models.ProductCategory, error) {
	loaders := For(ctx)
	return loaders.productCategoryLoader.Load(ctx, id)()
}

func GetProductCategories(ctx context.Context, ids []int) ([]*models.ProductCategory, []error) {
	loaders := For(ctx)
	return loaders.productCategoryLoader.LoadMany(ctx, ids)()
}
