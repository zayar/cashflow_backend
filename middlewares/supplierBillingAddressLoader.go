package middlewares

import (
	"context"

	"github.com/graph-gophers/dataloader/v7"
	"github.com/mmdatafocus/books_backend/models"
	"gorm.io/gorm"
)

type supplierBillingAddressReader struct {
	db *gorm.DB
}

func (r *supplierBillingAddressReader) GetSupplierBillingAddresses(ctx context.Context, supplierIds []int) []*dataloader.Result[*models.BillingAddress] {
	var results []models.BillingAddress
	err := r.db.WithContext(ctx).Where("reference_type = 'suppliers' AND reference_id IN ?", supplierIds).Find(&results).Error
	if err != nil {
		return handleError[*models.BillingAddress](len(supplierIds), err)
	}

	return generateLoaderResults(results, supplierIds)
}

func GetSupplierBillingAddress(ctx context.Context, supplierId int) (*models.BillingAddress, error) {
	loaders := For(ctx)
	return loaders.supplierBillingAddressLoader.Load(ctx, supplierId)()
}

// var loaderResults []*dataloader.Result[[]*models.BillingAddress]
// for _, id := range supplierIds {
// 	billingAddresses := make([]*models.BillingAddress, 0)
// 	for _, result := range results {
// 		if result.ReferenceID == id {
// 			billingAddresses = append(billingAddresses, result)
// 		}
// 	}
// 	loaderResults = append(loaderResults, &dataloader.Result[[]*models.BillingAddress]{Data: billingAddresses})
// }
