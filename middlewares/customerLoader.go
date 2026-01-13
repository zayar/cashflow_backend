package middlewares

import (
	"context"

	"bitbucket.org/mmdatafocus/books_backend/models"
	"github.com/graph-gophers/dataloader/v7"
	"gorm.io/gorm"
)

type customerReader struct {
	db *gorm.DB
}

func (r *customerReader) getCustomers(ctx context.Context, ids []int) []*dataloader.Result[*models.Customer] {
	var results []models.Customer
	err := r.db.WithContext(ctx).Where("id IN ?", ids).Find(&results).Error
	if err != nil {
		return handleError[*models.Customer](len(ids), err)
	}

	return generateLoaderResults(results, ids)
	// resultMap := make(map[int]*models.Customer)
	// resultMap[0] = &models.Customer{}
	// for _, result := range results {
	// 	resultMap[result.ID] = result
	// }

	// loaderResults := make([]*dataloader.Result[*models.Customer], 0, len(ids))
	// for _, id := range ids {
	// 	result := resultMap[id]
	// 	loaderResults = append(loaderResults, &dataloader.Result[*models.Customer]{Data: result})
	// }
	// return loaderResults
}

func GetCustomer(ctx context.Context, id int) (*models.Customer, error) {
	loaders := For(ctx)
	return loaders.customerLoader.Load(ctx, id)()
}

func GetCustomers(ctx context.Context, ids []int) ([]*models.Customer, []error) {
	loaders := For(ctx)
	return loaders.customerLoader.LoadMany(ctx, ids)()
}
