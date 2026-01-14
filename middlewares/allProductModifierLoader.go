package middlewares

import (
	"context"

	"github.com/graph-gophers/dataloader/v7"
	"github.com/mmdatafocus/books_backend/models"
	"gorm.io/gorm"
)

type allProductModifierReader struct {
	db *gorm.DB
}

func (r *allProductModifierReader) getAllProductModifiers(ctx context.Context, ids []int) []*dataloader.Result[*models.AllProductModifier] {
	resultMap, err := models.MapAllProductModifier(ctx)
	if err != nil {
		return handleError[*models.AllProductModifier](len(ids), err)
	}
	var loaderResults = make([]*dataloader.Result[*models.AllProductModifier], 0, len(ids))
	for _, id := range ids {
		result, ok := resultMap[id]
		if !ok {
			var v models.AllProductModifier
			v.ID = id
			result = &v
		}
		loaderResults = append(loaderResults, &dataloader.Result[*models.AllProductModifier]{Data: result})
	}
	return loaderResults
}

func GetAllProductModifier(ctx context.Context, id int) (*models.AllProductModifier, error) {
	loaders := For(ctx)
	return loaders.allProductModifierLoader.Load(ctx, id)()
}

func GetAllProductModifiers(ctx context.Context, ids []int) ([]*models.AllProductModifier, []error) {
	loaders := For(ctx)
	return loaders.allProductModifierLoader.LoadMany(ctx, ids)()
}
