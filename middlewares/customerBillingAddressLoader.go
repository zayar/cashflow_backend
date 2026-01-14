package middlewares

import (
	"context"

	"github.com/graph-gophers/dataloader/v7"
	"github.com/mmdatafocus/books_backend/models"
	"gorm.io/gorm"
)

type customerBillingAddressReader struct {
	db *gorm.DB
}

func (r *customerBillingAddressReader) GetCustomerBillingAddresses(ctx context.Context, customerIds []int) []*dataloader.Result[*models.BillingAddress] {
	var results []models.BillingAddress
	err := r.db.WithContext(ctx).Where("reference_type = 'customers' AND reference_id IN ?", customerIds).Find(&results).Error
	if err != nil {
		return handleError[*models.BillingAddress](len(customerIds), err)
	}

	return generateLoaderResults(results, customerIds)
}

func GetCustomerBillingAddress(ctx context.Context, customerId int) (*models.BillingAddress, error) {
	loaders := For(ctx)
	return loaders.customerBillingAddressLoader.Load(ctx, customerId)()
}

// var loaderResults []*dataloader.Result[[]*models.BillingAddress]
// for _, id := range customerIds {
// 	billingAddresses := make([]*models.BillingAddress, 0)
// 	for _, result := range results {
// 		if result.ReferenceID == id {
// 			billingAddresses = append(billingAddresses, result)
// 		}
// 	}
// 	loaderResults = append(loaderResults, &dataloader.Result[[]*models.BillingAddress]{Data: billingAddresses})
// }
