package middlewares

import (
	"context"

	"github.com/graph-gophers/dataloader/v7"
	"github.com/mmdatafocus/books_backend/models"
	"gorm.io/gorm"
)

type supplierCreditDetailReader struct {
	db *gorm.DB
}

func (r *supplierCreditDetailReader) GetSupplierCreditDetails(ctx context.Context, Ids []int) []*dataloader.Result[[]*models.SupplierCreditDetail] {
	var results []models.SupplierCreditDetail
	err := r.db.WithContext(ctx).Where("supplier_credit_id IN ?", Ids).Find(&results).Error
	if err != nil {
		return handleError[[]*models.SupplierCreditDetail](len(Ids), err)
	}

	return generateLoaderArrayResults(results, Ids)
	// // key => customer id (int)
	// resultMap := make(map[int][]*models.SupplierCreditDetail)
	// for _, result := range results {
	// 	resultMap[result.SupplierCreditId] = append(resultMap[result.SupplierCreditId], result)
	// }
	// var loaderResults []*dataloader.Result[[]*models.SupplierCreditDetail]
	// for _, id := range Ids {
	// 	details := resultMap[id]
	// 	loaderResults = append(loaderResults, &dataloader.Result[[]*models.SupplierCreditDetail]{Data: details})
	// }
	// return loaderResults
}

func GetSupplierCreditDetails(ctx context.Context, billId int) ([]*models.SupplierCreditDetail, error) {
	loaders := For(ctx)
	return loaders.supplierCreditDetailLoader.Load(ctx, billId)()
}
