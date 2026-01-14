package middlewares

import (
	"context"

	"github.com/graph-gophers/dataloader/v7"
	"github.com/mmdatafocus/books_backend/models"
	"gorm.io/gorm"
)

type billDetailReader struct {
	db *gorm.DB
}

func (r *billDetailReader) GetBillDetails(ctx context.Context, Ids []int) []*dataloader.Result[[]*models.BillDetail] {
	var results []models.BillDetail
	err := r.db.WithContext(ctx).Where("bill_id IN ?", Ids).Find(&results).Error
	if err != nil {
		return handleError[[]*models.BillDetail](len(Ids), err)
	}

	return generateLoaderArrayResults(results, Ids)
	// key => customer id (int)
	// value => array of billing address pointer []*BillDetaile
	// resultMap := make(map[int][]*models.BillDetail)
	// for _, result := range results {
	// 	resultMap[result.BillId] = append(resultMap[result.BillId], result)
	// }
	// var loaderResults []*dataloader.Result[[]*models.BillDetail]
	// for _, id := range Ids {
	// 	billDetails := resultMap[id]
	// 	loaderResults = append(loaderResults, &dataloader.Result[[]*models.BillDetail]{Data: billDetails})
	// }
	// return loaderResults
}

func GetBillDetails(ctx context.Context, billId int) ([]*models.BillDetail, error) {
	loaders := For(ctx)
	return loaders.billDetailLoader.Load(ctx, billId)()
}
