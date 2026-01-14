package middlewares

import (
	"context"

	"github.com/graph-gophers/dataloader/v7"
	"github.com/mmdatafocus/books_backend/models"
	"gorm.io/gorm"
)

type supplierShippingAddressReader struct {
	db *gorm.DB
}

func (r *supplierShippingAddressReader) GetSupplierShippingAddresses(ctx context.Context, supplierIds []int) []*dataloader.Result[*models.ShippingAddress] {
	var results []models.ShippingAddress
	err := r.db.WithContext(ctx).Where("reference_type = 'suppliers' AND reference_id IN ?", supplierIds).Find(&results).Error
	if err != nil {
		return handleError[*models.ShippingAddress](len(supplierIds), err)
	}

	return generateLoaderResults(results, supplierIds)
}

func GetSupplierShippingAddress(ctx context.Context, supplierId int) (*models.ShippingAddress, error) {
	loaders := For(ctx)
	return loaders.supplierShippingAddressLoader.Load(ctx, supplierId)()
}
