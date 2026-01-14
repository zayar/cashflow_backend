package middlewares

import (
	"context"

	"github.com/graph-gophers/dataloader/v7"
	"github.com/mmdatafocus/books_backend/models"
	"gorm.io/gorm"
)

type allReasonReader struct {
	db *gorm.DB
}

func (r *allReasonReader) getAllReasons(ctx context.Context, ids []int) []*dataloader.Result[*models.AllReason] {
	resultMap, err := models.MapAllReason(ctx)
	if err != nil {
		return handleError[*models.AllReason](len(ids), err)
	}
	var loaderResults = make([]*dataloader.Result[*models.AllReason], 0, len(ids))
	for _, id := range ids {
		result, ok := resultMap[id]
		if !ok {
			var v models.AllReason
			v.ID = id
			result = &v
		}
		loaderResults = append(loaderResults, &dataloader.Result[*models.AllReason]{Data: result})
	}
	return loaderResults
}

func GetAllReason(ctx context.Context, id int) (*models.AllReason, error) {
	loaders := For(ctx)
	return loaders.allReasonLoader.Load(ctx, id)()
}

func GetAllReasons(ctx context.Context, ids []int) ([]*models.AllReason, []error) {
	loaders := For(ctx)
	return loaders.allReasonLoader.LoadMany(ctx, ids)()
}
