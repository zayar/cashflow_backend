package middlewares

import (
	"context"

	"github.com/graph-gophers/dataloader/v7"
	"github.com/mmdatafocus/books_backend/models"
	"gorm.io/gorm"
)

type appliedSupplierCreditReader struct {
	db *gorm.DB
}

func (r *appliedSupplierCreditReader) GetAppliedSupplierCredits(ctx context.Context, Ids []int) []*dataloader.Result[[]*models.SupplierCreditBill] {
	var results []*models.SupplierCreditBill
	err := r.db.WithContext(ctx).Where("bill_id IN ?", Ids).Find(&results).Error
	if err != nil {
		return handleError[[]*models.SupplierCreditBill](len(Ids), err)
	}

	resultMap := make(map[int][]*models.SupplierCreditBill)
	for _, result := range results {
		resultMap[result.BillId] = append(resultMap[result.BillId], result)
	}
	var loaderResults []*dataloader.Result[[]*models.SupplierCreditBill]
	for _, id := range Ids {
		documents := resultMap[id]
		loaderResults = append(loaderResults, &dataloader.Result[[]*models.SupplierCreditBill]{Data: documents})
	}
	return loaderResults
}

func GetAppliedSupplierCredits(ctx context.Context, billId int) ([]*models.SupplierCreditBill, error) {
	loaders := For(ctx)
	return loaders.appliedSupplierCreditLoader.Load(ctx, billId)()
}
