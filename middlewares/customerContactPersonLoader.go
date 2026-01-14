package middlewares

import (
	"context"

	"github.com/graph-gophers/dataloader/v7"
	"github.com/mmdatafocus/books_backend/models"
	"gorm.io/gorm"
)

type customerContactPersonReader struct {
	db *gorm.DB
}

func (r *customerContactPersonReader) GetCustomerContactPersons(ctx context.Context, customerIds []int) []*dataloader.Result[[]*models.ContactPerson] {
	var results []models.ContactPerson
	err := r.db.WithContext(ctx).Where("reference_type = 'customers' AND reference_id IN ?", customerIds).Find(&results).Error
	if err != nil {
		return handleError[[]*models.ContactPerson](len(customerIds), err)
	}

	return generateLoaderArrayResults(results, customerIds)
	// key => customer id (int)
	// value => array of billing contactPerson pointer []*contactPersons
	// resultMap := make(map[int][]*models.ContactPerson)
	// for _, result := range results {
	// 	resultMap[result.ReferenceID] = append(resultMap[result.ReferenceID], result)
	// }
	// var loaderResults []*dataloader.Result[[]*models.ContactPerson]
	// for _, id := range customerIds {
	// 	contactPersons := resultMap[id]
	// 	loaderResults = append(loaderResults, &dataloader.Result[[]*models.ContactPerson]{Data: contactPersons})
	// }
	// return loaderResults
}

func GetCustomerContactPersons(ctx context.Context, customerId int) ([]*models.ContactPerson, error) {
	loaders := For(ctx)
	return loaders.customerContactPersonLoader.Load(ctx, customerId)()
}
