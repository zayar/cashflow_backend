package middlewares

import (
	"context"

	"github.com/graph-gophers/dataloader/v7"
	"github.com/mmdatafocus/books_backend/models"
	"gorm.io/gorm"
)

type salesPersonReader struct {
	db *gorm.DB
}

func (r *salesPersonReader) getSalesPersons(ctx context.Context, ids []int) []*dataloader.Result[*models.SalesPerson] {
	var results []models.SalesPerson
	err := r.db.WithContext(ctx).Where("id IN ?", ids).Find(&results).Error
	if err != nil {
		return handleError[*models.SalesPerson](len(ids), err)
	}

	return generateLoaderResults(results, ids)
	// resultMap := make(map[int]*models.SalesPerson)
	// resultMap[0] = &models.SalesPerson{}
	// for _, result := range results {
	// 	resultMap[result.ID] = result
	// }

	// loaderResults := make([]*dataloader.Result[*models.SalesPerson], 0, len(ids))
	// for _, id := range ids {
	// 	result := resultMap[id]
	// 	loaderResults = append(loaderResults, &dataloader.Result[*models.SalesPerson]{Data: result})
	// }
	// return loaderResults
}

func GetSalesPerson(ctx context.Context, id int) (*models.SalesPerson, error) {
	loaders := For(ctx)
	return loaders.salesPersonLoader.Load(ctx, id)()
}

func GetSalesPersons(ctx context.Context, ids []int) ([]*models.SalesPerson, []error) {
	loaders := For(ctx)
	return loaders.salesPersonLoader.LoadMany(ctx, ids)()
}
