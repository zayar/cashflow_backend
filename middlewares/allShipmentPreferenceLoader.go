package middlewares

import (
	"context"

	"github.com/graph-gophers/dataloader/v7"
	"github.com/mmdatafocus/books_backend/models"
	"gorm.io/gorm"
)

type allShipmentPreferenceReader struct {
	db *gorm.DB
}

func (r *allShipmentPreferenceReader) getAllShipmentPreferences(ctx context.Context, ids []int) []*dataloader.Result[*models.AllShipmentPreference] {
	resultMap, err := models.MapAllShipmentPreference(ctx)
	if err != nil {
		return handleError[*models.AllShipmentPreference](len(ids), err)
	}
	var loaderResults = make([]*dataloader.Result[*models.AllShipmentPreference], 0, len(ids))
	for _, id := range ids {
		result, ok := resultMap[id]
		if !ok {
			var v models.AllShipmentPreference
			v.ID = id
			result = &v
		}
		loaderResults = append(loaderResults, &dataloader.Result[*models.AllShipmentPreference]{Data: result})
	}
	return loaderResults
}

func GetAllShipmentPreference(ctx context.Context, id int) (*models.AllShipmentPreference, error) {
	loaders := For(ctx)
	return loaders.allShipmentPreferenceLoader.Load(ctx, id)()
}

func GetAllShipmentPreferences(ctx context.Context, ids []int) ([]*models.AllShipmentPreference, []error) {
	loaders := For(ctx)
	return loaders.allShipmentPreferenceLoader.LoadMany(ctx, ids)()
}
