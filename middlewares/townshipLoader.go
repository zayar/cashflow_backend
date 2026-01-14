package middlewares

import (
	"context"

	"github.com/graph-gophers/dataloader/v7"
	"github.com/mmdatafocus/books_backend/models"
	"gorm.io/gorm"
)

type townshipReader struct {
	db *gorm.DB
}

func (r *townshipReader) getTownships(ctx context.Context, ids []int) []*dataloader.Result[*models.Township] {
	var results []models.Township
	err := r.db.WithContext(ctx).
		Where("id IN ?", ids).Find(&results).Error
	if err != nil {
		return handleError[*models.Township](len(ids), err)
	}

	return generateLoaderResults(results, ids)
	// resultMap := make(map[int]*models.Township)
	// resultMap[0] = &models.Township{}
	// for _, result := range results {
	// 	resultMap[result.ID] = result
	// }

	// loaderResults := make([]*dataloader.Result[*models.Township], 0, len(ids))
	// for _, id := range ids {
	// 	result := resultMap[id]
	// 	loaderResults = append(loaderResults, &dataloader.Result[*models.Township]{Data: result})
	// }

	// return loaderResults
}

func GetTownship(ctx context.Context, id int) (*models.Township, error) {
	loaders := For(ctx)
	return loaders.TownshipLoader.Load(ctx, id)()
}

func GetTownships(ctx context.Context, ids []int) ([]*models.Township, []error) {
	loaders := For(ctx)
	return loaders.TownshipLoader.LoadMany(ctx, ids)()
}
