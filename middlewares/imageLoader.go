package middlewares

import (
	"context"

	"github.com/graph-gophers/dataloader/v7"
	"github.com/mmdatafocus/books_backend/models"
	"gorm.io/gorm"
)

type imageReader struct {
	db            *gorm.DB
	referenceType string
}

func (r *imageReader) GetImages(ctx context.Context, referenceIds []int) []*dataloader.Result[[]*models.Image] {
	var results []models.Image
	if err := r.db.WithContext(ctx).Where("reference_type = ? AND reference_id IN ?", r.referenceType, referenceIds).Find(&results).Error; err != nil {
		return handleError[[]*models.Image](len(referenceIds), err)
	}

	return generateLoaderArrayResults(results, referenceIds)
}

func GetImages(ctx context.Context, referenceType string, referenceId int) ([]*models.Image, error) {
	loaders := For(ctx)
	var imageLoader *dataloader.Loader[int, []*models.Image]

	switch referenceType {
	case "products":
		imageLoader = loaders.productImageLoader
	case "product_groups":
		imageLoader = loaders.productGroupImageLoader
	}
	return imageLoader.Load(ctx, referenceId)()
}
