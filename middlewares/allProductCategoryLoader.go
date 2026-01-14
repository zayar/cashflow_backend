package middlewares

import (
	"context"

	"github.com/graph-gophers/dataloader/v7"
	"github.com/mmdatafocus/books_backend/models"
	"gorm.io/gorm"
)

type allProductCategoryReader struct {
	db *gorm.DB
}

func (r *allProductCategoryReader) getAllProductCategorys(ctx context.Context, ids []int) []*dataloader.Result[*models.AllProductCategory] {
	resultMap, err := models.MapAllProductCategory(ctx)
	if err != nil {
		return handleError[*models.AllProductCategory](len(ids), err)
	}
	var loaderResults = make([]*dataloader.Result[*models.AllProductCategory], 0, len(ids))
	for _, id := range ids {
		result, ok := resultMap[id]
		if !ok {
			var v models.AllProductCategory
			v.ID = id
			result = &v
		}
		loaderResults = append(loaderResults, &dataloader.Result[*models.AllProductCategory]{Data: result})
	}
	return loaderResults
}

func GetAllProductCategory(ctx context.Context, id int) (*models.AllProductCategory, error) {
	loaders := For(ctx)
	return loaders.allProductCategoryLoader.Load(ctx, id)()
}

func GetAllProductCategorys(ctx context.Context, ids []int) ([]*models.AllProductCategory, []error) {
	loaders := For(ctx)
	return loaders.allProductCategoryLoader.LoadMany(ctx, ids)()
}
