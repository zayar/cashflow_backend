package middlewares

import (
	"context"

	"github.com/graph-gophers/dataloader/v7"
	"github.com/mmdatafocus/books_backend/models"
	"gorm.io/gorm"
)

type productGroupReader struct {
	db *gorm.DB
}

func (r *productGroupReader) getProductGroups(ctx context.Context, ids []int) []*dataloader.Result[*models.ProductGroup] {
	var results []models.ProductGroup
	err := r.db.WithContext(ctx).Where("id IN ?", ids).Find(&results).Error
	if err != nil {
		return handleError[*models.ProductGroup](len(ids), err)
	}

	return generateLoaderResults(results, ids)
}

func GetProductGroup(ctx context.Context, id int) (*models.ProductGroup, error) {
	loaders := For(ctx)
	return loaders.productGroupLoader.Load(ctx, id)()
}

func GetProductGroups(ctx context.Context, ids []int) ([]*models.ProductGroup, []error) {
	loaders := For(ctx)
	return loaders.productGroupLoader.LoadMany(ctx, ids)()
}
