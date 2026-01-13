package middlewares

import (
	"context"

	"bitbucket.org/mmdatafocus/books_backend/models"
	"github.com/graph-gophers/dataloader/v7"
	"gorm.io/gorm"
)

type stateReader struct {
	db *gorm.DB
}

func (r *stateReader) getStates(ctx context.Context, ids []int) []*dataloader.Result[*models.State] {
	var results []models.State
	err := r.db.WithContext(ctx).
		Where("id IN ?", ids).
		Find(&results).Error
	if err != nil {
		return handleError[*models.State](len(ids), err)
	}

	return generateLoaderResults(results, ids)
	// // map id to model
	// resultMap := make(map[int]*models.State)
	// resultMap[0] = &models.State{}
	// for _, result := range results {
	// 	resultMap[result.ID] = result
	// }

	// loaderResults := make([]*dataloader.Result[*models.State], 0, len(ids))
	// for _, id := range ids {
	// 	result := resultMap[id]
	// 	loaderResults = append(loaderResults, &dataloader.Result[*models.State]{Data: result})
	// }
	// return loaderResults
}

func GetState(ctx context.Context, id int) (*models.State, error) {
	loaders := For(ctx)
	return loaders.StateLoader.Load(ctx, id)()
}

func GetStates(ctx context.Context, ids []int) ([]*models.State, []error) {
	loaders := For(ctx)
	return loaders.StateLoader.LoadMany(ctx, ids)()
}
