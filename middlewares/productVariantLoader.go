package middlewares

import (
	"context"

	"bitbucket.org/mmdatafocus/books_backend/models"
	"github.com/graph-gophers/dataloader/v7"
	"gorm.io/gorm"
)

type productVariantReader struct {
	db *gorm.DB
}

func (r *productVariantReader) getProductVariants(ctx context.Context, ids []int) []*dataloader.Result[*models.ProductVariant] {
	var results []models.ProductVariant
	err := r.db.WithContext(ctx).Where("id IN ?", ids).Find(&results).Error
	if err != nil {
		return handleError[*models.ProductVariant](len(ids), err)
	}

	// resultMap := make(map[int]*models.ProductVariant)
	// resultMap[0] = &models.ProductVariant{}
	// for _, result := range results {
	// 	resultMap[result.ID] = result
	// }

	// loaderResults := make([]*dataloader.Result[*models.ProductVariant], 0, len(ids))
	// for _, id := range ids {
	// 	result := resultMap[id]
	// 	loaderResults = append(loaderResults, &dataloader.Result[*models.ProductVariant]{Data: result})
	// }
	return generateLoaderResults[models.ProductVariant](results, ids)
}

func GetProductVariant(ctx context.Context, id int) (*models.ProductVariant, error) {
	loaders := For(ctx)
	return loaders.productVariantLoader.Load(ctx, id)()
}

func GetProductVariants(ctx context.Context, ids []int) ([]*models.ProductVariant, []error) {
	loaders := For(ctx)
	return loaders.productVariantLoader.LoadMany(ctx, ids)()
}
