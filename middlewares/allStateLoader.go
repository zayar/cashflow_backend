package middlewares

import (
	"context"

	"bitbucket.org/mmdatafocus/books_backend/models"
	"github.com/graph-gophers/dataloader/v7"
	"gorm.io/gorm"
)

type allStateReader struct {
	db *gorm.DB
}

func (r *allStateReader) getAllStates(ctx context.Context, ids []int) []*dataloader.Result[*models.AllState] {
	resultMap, err := models.MapAllState(ctx)
	if err != nil {
		return handleError[*models.AllState](len(ids), err)
	}
	var loaderResults = make([]*dataloader.Result[*models.AllState], 0, len(ids))
	for _, id := range ids {
		result, ok := resultMap[id]
		if !ok {
			var v models.AllState
			v.ID = id
			result = &v
		}
		loaderResults = append(loaderResults, &dataloader.Result[*models.AllState]{Data: result})
	}
	return loaderResults
}

func GetAllState(ctx context.Context, id int) (*models.AllState, error) {
	loaders := For(ctx)
	return loaders.allStateLoader.Load(ctx, id)()
}

func GetAllStates(ctx context.Context, ids []int) ([]*models.AllState, []error) {
	loaders := For(ctx)
	return loaders.allStateLoader.LoadMany(ctx, ids)()
}
