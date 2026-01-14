package middlewares

import (
	"context"

	"github.com/graph-gophers/dataloader/v7"
	"github.com/mmdatafocus/books_backend/models"
	"gorm.io/gorm"
)

type allDeliveryMethodReader struct {
	db *gorm.DB
}

func (r *allDeliveryMethodReader) getAllDeliveryMethods(ctx context.Context, ids []int) []*dataloader.Result[*models.AllDeliveryMethod] {
	resultMap, err := models.MapAllDeliveryMethod(ctx)
	if err != nil {
		return handleError[*models.AllDeliveryMethod](len(ids), err)
	}
	var loaderResults = make([]*dataloader.Result[*models.AllDeliveryMethod], 0, len(ids))
	for _, id := range ids {
		result, ok := resultMap[id]
		if !ok {
			var v models.AllDeliveryMethod
			v.ID = id
			result = &v
		}
		loaderResults = append(loaderResults, &dataloader.Result[*models.AllDeliveryMethod]{Data: result})
	}
	return loaderResults
}

func GetAllDeliveryMethod(ctx context.Context, id int) (*models.AllDeliveryMethod, error) {
	loaders := For(ctx)
	return loaders.allDeliveryMethodLoader.Load(ctx, id)()
}

func GetAllDeliveryMethods(ctx context.Context, ids []int) ([]*models.AllDeliveryMethod, []error) {
	loaders := For(ctx)
	return loaders.allDeliveryMethodLoader.LoadMany(ctx, ids)()
}
