package middlewares

import (
	"context"

	"bitbucket.org/mmdatafocus/books_backend/models"
	"github.com/graph-gophers/dataloader/v7"
	"gorm.io/gorm"
)

type productVariantByGroupReader struct {
	db *gorm.DB
}

// batch function for productVariantLoader
func (r *productVariantByGroupReader) getProductVariantsByGroupId(ctx context.Context, ids []int) []*dataloader.Result[[]*models.ProductVariant] {
	var results []models.ProductVariant
	err := r.db.WithContext(ctx).Where("product_group_id IN ?", ids).Find(&results).Error
	if err != nil {
		return handleError[[]*models.ProductVariant](len(ids), err)
	}

	return generateLoaderArrayResults(results, ids)
	// key = productGroupId, value = its variants
	// pgIdToVariants := make(map[string][]*models.ProductVariant)
	// for _, result := range results {
	// 	pgIdToVariants[result.ProductGroupId] = append(pgIdToVariants[result.ProductGroupId], result)
	// }

	// var loaderResults []*dataloader.Result[[]*models.ProductVariant]
	// for _, id := range ids {
	// 	variants := pgIdToVariants[fmt.Sprint(id)]
	// 	loaderResults = append(loaderResults, &dataloader.Result[[]*models.ProductVariant]{Data: variants})
	// }
	// return loaderResults
}

// returns slice of product variants associated with product group id
func GetProductVariantsByGroupId(ctx context.Context, id int) ([]*models.ProductVariant, error) {
	loaders := For(ctx)
	return loaders.productVariantByGroupLoader.Load(ctx, id)()
}
