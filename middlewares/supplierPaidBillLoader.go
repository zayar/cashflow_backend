package middlewares

import (
	"context"

	"bitbucket.org/mmdatafocus/books_backend/models"
	"github.com/graph-gophers/dataloader/v7"
	"gorm.io/gorm"
)

type supplierPaidBillReader struct {
	db *gorm.DB
}

func (r *supplierPaidBillReader) GetSupplierPaidBills(ctx context.Context, Ids []int) []*dataloader.Result[[]*models.SupplierPaidBill] {
	var results []models.SupplierPaidBill
	err := r.db.WithContext(ctx).Where("supplier_payment_id IN ?", Ids).Find(&results).Error
	if err != nil {
		return handleError[[]*models.SupplierPaidBill](len(Ids), err)
	}

	return generateLoaderArrayResults(results, Ids)
	// key => customer id (int)
	// value => array of billing address pointer []*SupplierPaidBille
	// resultMap := make(map[int][]*models.SupplierPaidBill)
	// for _, result := range results {
	// 	resultMap[result.SupplierPaymentId] = append(resultMap[result.SupplierPaymentId], result)
	// }
	// var loaderResults []*dataloader.Result[[]*models.SupplierPaidBill]
	// for _, id := range Ids {
	// 	paidBills := resultMap[id]
	// 	loaderResults = append(loaderResults, &dataloader.Result[[]*models.SupplierPaidBill]{Data: paidBills})
	// }
	// return loaderResults
}

func GetSupplierPaidBills(ctx context.Context, orderId int) ([]*models.SupplierPaidBill, error) {
	loaders := For(ctx)
	return loaders.supplierPaidBillLoader.Load(ctx, orderId)()
}
