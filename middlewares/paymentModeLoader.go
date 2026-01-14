package middlewares

import (
	"context"

	"github.com/graph-gophers/dataloader/v7"
	"github.com/mmdatafocus/books_backend/models"
	"gorm.io/gorm"
)

type paymentModeReader struct {
	db *gorm.DB
}

func (r *paymentModeReader) getPaymentModes(ctx context.Context, ids []int) []*dataloader.Result[*models.PaymentMode] {
	var results []models.PaymentMode
	err := r.db.WithContext(ctx).
		Where("id IN ?", ids).Find(&results).Error
	if err != nil {
		return handleError[*models.PaymentMode](len(ids), err)
	}

	return generateLoaderResults(results, ids)
	// resultMap := make(map[int]*models.PaymentMode)
	// resultMap[0] = &models.PaymentMode{}
	// for _, result := range results {
	// 	resultMap[result.ID] = result
	// }

	// loaderResults := make([]*dataloader.Result[*models.PaymentMode], 0, len(ids))
	// for _, id := range ids {
	// 	result := resultMap[id]
	// 	loaderResults = append(loaderResults, &dataloader.Result[*models.PaymentMode]{Data: result})
	// }

	// return loaderResults
}

func GetPaymentMode(ctx context.Context, id int) (*models.PaymentMode, error) {
	loaders := For(ctx)
	return loaders.paymentModeLoader.Load(ctx, id)()
}

func GetPaymentModes(ctx context.Context, ids []int) ([]*models.PaymentMode, []error) {
	loaders := For(ctx)
	return loaders.paymentModeLoader.LoadMany(ctx, ids)()
}
