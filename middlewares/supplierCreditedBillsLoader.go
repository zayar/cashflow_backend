package middlewares

import (
	"context"

	"bitbucket.org/mmdatafocus/books_backend/models"
	"github.com/graph-gophers/dataloader/v7"
	"gorm.io/gorm"
)

type supplierCreditBillReader struct {
	db *gorm.DB
}

func (r *supplierCreditBillReader) GetSupplierCreditedBills(ctx context.Context, Ids []int) []*dataloader.Result[[]*models.SupplierCreditBill] {
	var results []models.SupplierCreditBill
	err := r.db.WithContext(ctx).Where("reference_type = ? AND reference_id IN ?", models.SupplierCreditApplyTypeCredit, Ids).Find(&results).Error
	if err != nil {
		return handleError[[]*models.SupplierCreditBill](len(Ids), err)
	}

	return generateLoaderArrayResults(results, Ids)
}

func GetSupplierCreditedBills(ctx context.Context, id int) ([]*models.SupplierCreditBill, error) {
	loaders := For(ctx)
	return loaders.supplierCreditBillLoader.Load(ctx, id)()
}
