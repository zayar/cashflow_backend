package middlewares

import (
	"context"

	"github.com/graph-gophers/dataloader/v7"
	"github.com/mmdatafocus/books_backend/models"
	"gorm.io/gorm"
)

type allPaymentModeReader struct {
	db *gorm.DB
}

func (r *allPaymentModeReader) getAllPaymentModes(ctx context.Context, ids []int) []*dataloader.Result[*models.AllPaymentMode] {
	resultMap, err := models.MapAllPaymentMode(ctx)
	if err != nil {
		return handleError[*models.AllPaymentMode](len(ids), err)
	}
	var loaderResults = make([]*dataloader.Result[*models.AllPaymentMode], 0, len(ids))
	for _, id := range ids {
		result, ok := resultMap[id]
		if !ok {
			var v models.AllPaymentMode
			v.ID = id
			result = &v
		}
		loaderResults = append(loaderResults, &dataloader.Result[*models.AllPaymentMode]{Data: result})
	}
	return loaderResults
}

func GetAllPaymentMode(ctx context.Context, id int) (*models.AllPaymentMode, error) {
	loaders := For(ctx)
	return loaders.allPaymentModeLoader.Load(ctx, id)()
}

func GetAllPaymentModes(ctx context.Context, ids []int) ([]*models.AllPaymentMode, []error) {
	loaders := For(ctx)
	return loaders.allPaymentModeLoader.LoadMany(ctx, ids)()
}
