package middlewares

import (
	"context"

	"github.com/graph-gophers/dataloader/v7"
	"github.com/mmdatafocus/books_backend/models"
	"gorm.io/gorm"
)

type recurringBillDetailReader struct {
	db *gorm.DB
}

func (r *recurringBillDetailReader) GetRecurringBillDetails(ctx context.Context, Ids []int) []*dataloader.Result[[]*models.RecurringBillDetail] {
	var results []models.RecurringBillDetail
	err := r.db.WithContext(ctx).Where("recurring_bill_id IN ?", Ids).Find(&results).Error
	if err != nil {
		return handleError[[]*models.RecurringBillDetail](len(Ids), err)
	}

	return generateLoaderArrayResults(results, Ids)
	// key => customer id (int)
	// value => array of billing address pointer []*RecurringBillDetaile
	// resultMap := make(map[int][]*models.RecurringBillDetail)
	// for _, result := range results {
	// 	resultMap[result.RecurringBillId] = append(resultMap[result.RecurringBillId], result)
	// }
	// var loaderResults []*dataloader.Result[[]*models.RecurringBillDetail]
	// for _, id := range Ids {
	// 	details := resultMap[id]
	// 	loaderResults = append(loaderResults, &dataloader.Result[[]*models.RecurringBillDetail]{Data: details})
	// }
	// return loaderResults
}

func GetRecurringBillDetails(ctx context.Context, orderId int) ([]*models.RecurringBillDetail, error) {
	loaders := For(ctx)
	return loaders.recurringBillDetailLoader.Load(ctx, orderId)()
}
