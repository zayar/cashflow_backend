package middlewares

import (
	"context"

	"github.com/graph-gophers/dataloader/v7"
	"github.com/mmdatafocus/books_backend/models"
	"gorm.io/gorm"
)

type productModifierReader struct {
	db *gorm.DB
}

func (r *productModifierReader) getProductModifiers(ctx context.Context, ids []int) []*dataloader.Result[*models.ProductModifier] {
	var results []models.ProductModifier
	err := r.db.WithContext(ctx).Where("id IN ?", ids).Preload("ModifierUnits").Find(&results).Error
	if err != nil {
		return handleError[*models.ProductModifier](len(ids), err)
	}

	return generateLoaderResults(results, ids)
	// resultMap := make(map[int]*models.ProductModifier)
	// resultMap[0] = &models.ProductModifier{}
	// for _, result := range results {
	// 	resultMap[result.ID] = result
	// }

	// loaderResults := make([]*dataloader.Result[*models.ProductModifier], 0, len(ids))
	// for _, id := range ids {
	// 	result := resultMap[id]
	// 	loaderResults = append(loaderResults, &dataloader.Result[*models.ProductModifier]{Data: result})
	// }
	// return loaderResults
}

func GetProductModifier(ctx context.Context, id int) (*models.ProductModifier, error) {
	loaders := For(ctx)
	return loaders.productModifierLoader.Load(ctx, id)()
}

func GetProductModifiers(ctx context.Context, ids []int) ([]*models.ProductModifier, []error) {
	loaders := For(ctx)
	return loaders.productModifierLoader.LoadMany(ctx, ids)()
}
