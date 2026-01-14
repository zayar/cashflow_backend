package middlewares

import (
	"context"

	"github.com/graph-gophers/dataloader/v7"
	"github.com/mmdatafocus/books_backend/models"
	"gorm.io/gorm"
)

type customerShippingAddressReader struct {
	db *gorm.DB
}

func (r *customerShippingAddressReader) GetCustomerShippingAddresses(ctx context.Context, customerIds []int) []*dataloader.Result[*models.ShippingAddress] {
	var results []models.ShippingAddress
	err := r.db.WithContext(ctx).Where("reference_type = 'customers' AND reference_id IN ?", customerIds).Find(&results).Error
	if err != nil {
		return handleError[*models.ShippingAddress](len(customerIds), err)
	}

	return generateLoaderResults(results, customerIds)
}

func GetCustomerShippingAddress(ctx context.Context, customerId int) (*models.ShippingAddress, error) {
	loaders := For(ctx)
	return loaders.customerShippingAddressLoader.Load(ctx, customerId)()
}
