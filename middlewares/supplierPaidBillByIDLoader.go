package middlewares

import (
	"context"

	"github.com/graph-gophers/dataloader/v7"
	"github.com/mmdatafocus/books_backend/models"
	"gorm.io/gorm"
)

type supplierPaidBillByIDReader struct {
	db *gorm.DB
}

func (r *supplierPaidBillByIDReader) getSupplierPaidBillsByID(ctx context.Context, ids []int) []*dataloader.Result[*models.SupplierPaidBill] {
	var results []models.SupplierPaidBill
	err := r.db.WithContext(ctx).Where("id IN ?", ids).Find(&results).Error
	if err != nil {
		return handleError[*models.SupplierPaidBill](len(ids), err)
	}

	resultMap := make(map[int]*models.SupplierPaidBill, len(results))
	for i := range results {
		resultMap[results[i].ID] = &results[i]
	}

	loaderResults := make([]*dataloader.Result[*models.SupplierPaidBill], 0, len(ids))
	for _, id := range ids {
		if v, ok := resultMap[id]; ok {
			loaderResults = append(loaderResults, &dataloader.Result[*models.SupplierPaidBill]{Data: v})
		} else {
			loaderResults = append(loaderResults, &dataloader.Result[*models.SupplierPaidBill]{Error: gorm.ErrRecordNotFound})
		}
	}
	return loaderResults
}

func GetSupplierPaidBill(ctx context.Context, id int) (*models.SupplierPaidBill, error) {
	loaders := For(ctx)
	return loaders.supplierPaidBillByIDLoader.Load(ctx, id)()
}

func GetSupplierPaidBillsByID(ctx context.Context, ids []int) ([]*models.SupplierPaidBill, []error) {
	loaders := For(ctx)
	return loaders.supplierPaidBillByIDLoader.LoadMany(ctx, ids)()
}

