package middlewares

import (
	"context"

	"bitbucket.org/mmdatafocus/books_backend/models"
	"github.com/graph-gophers/dataloader/v7"
	"gorm.io/gorm"
)

type allTownshipReader struct {
	db *gorm.DB
}

func (r *allTownshipReader) getAllTownships(ctx context.Context, ids []int) []*dataloader.Result[*models.AllTownship] {
	resultMap, err := models.MapAllTownship(ctx)
	if err != nil {
		return handleError[*models.AllTownship](len(ids), err)
	}
	var loaderResults = make([]*dataloader.Result[*models.AllTownship], 0, len(ids))
	for _, id := range ids {
		result, ok := resultMap[id]
		if !ok {
			var v models.AllTownship
			v.ID = id
			result = &v
		}
		loaderResults = append(loaderResults, &dataloader.Result[*models.AllTownship]{Data: result})
	}
	return loaderResults
}

func GetAllTownship(ctx context.Context, id int) (*models.AllTownship, error) {
	loaders := For(ctx)
	return loaders.allTownshipLoader.Load(ctx, id)()
}

func GetAllTownships(ctx context.Context, ids []int) ([]*models.AllTownship, []error) {
	loaders := For(ctx)
	return loaders.allTownshipLoader.LoadMany(ctx, ids)()
}
