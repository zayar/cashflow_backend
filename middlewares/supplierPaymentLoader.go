package middlewares

import (
	"context"

	"github.com/graph-gophers/dataloader/v7"
	"github.com/mmdatafocus/books_backend/models"
	"gorm.io/gorm"
)

type supplierPaymentByIDReader struct {
	db *gorm.DB
}

func (r *supplierPaymentByIDReader) getSupplierPaymentsByID(ctx context.Context, ids []int) []*dataloader.Result[*models.SupplierPayment] {
	var results []models.SupplierPayment
	err := r.db.WithContext(ctx).Where("id IN ?", ids).Find(&results).Error
	if err != nil {
		return handleError[*models.SupplierPayment](len(ids), err)
	}

	resultMap := make(map[int]*models.SupplierPayment, len(results))
	for i := range results {
		resultMap[results[i].ID] = &results[i]
	}

	loaderResults := make([]*dataloader.Result[*models.SupplierPayment], 0, len(ids))
	for _, id := range ids {
		if v, ok := resultMap[id]; ok {
			loaderResults = append(loaderResults, &dataloader.Result[*models.SupplierPayment]{Data: v})
		} else {
			loaderResults = append(loaderResults, &dataloader.Result[*models.SupplierPayment]{Error: gorm.ErrRecordNotFound})
		}
	}
	return loaderResults
}

func GetSupplierPayment(ctx context.Context, id int) (*models.SupplierPayment, error) {
	loaders := For(ctx)
	return loaders.supplierPaymentByIDLoader.Load(ctx, id)()
}

func GetSupplierPaymentsByID(ctx context.Context, ids []int) ([]*models.SupplierPayment, []error) {
	loaders := For(ctx)
	return loaders.supplierPaymentByIDLoader.LoadMany(ctx, ids)()
}

