package middlewares

import (
	"context"

	"github.com/graph-gophers/dataloader/v7"
	"github.com/mmdatafocus/books_backend/models"
	"gorm.io/gorm"
)

type allSalesPersonReader struct {
	db *gorm.DB
}

func (r *allSalesPersonReader) getAllSalesPersons(ctx context.Context, ids []int) []*dataloader.Result[*models.AllSalesPerson] {
	resultMap, err := models.MapAllSalesPerson(ctx)
	if err != nil {
		return handleError[*models.AllSalesPerson](len(ids), err)
	}
	var loaderResults = make([]*dataloader.Result[*models.AllSalesPerson], 0, len(ids))
	for _, id := range ids {
		result, ok := resultMap[id]
		if !ok {
			var v models.AllSalesPerson
			v.ID = id
			result = &v
		}
		loaderResults = append(loaderResults, &dataloader.Result[*models.AllSalesPerson]{Data: result})
	}
	return loaderResults
}

func GetAllSalesPerson(ctx context.Context, id int) (*models.AllSalesPerson, error) {
	loaders := For(ctx)
	return loaders.allSalesPersonLoader.Load(ctx, id)()
}

func GetAllSalesPersons(ctx context.Context, ids []int) ([]*models.AllSalesPerson, []error) {
	loaders := For(ctx)
	return loaders.allSalesPersonLoader.LoadMany(ctx, ids)()
}
