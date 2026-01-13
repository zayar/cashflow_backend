package middlewares

import (
	"context"

	"bitbucket.org/mmdatafocus/books_backend/models"
	"github.com/graph-gophers/dataloader/v7"
	"gorm.io/gorm"
)

type refundReader struct {
	db            *gorm.DB
	referenceType string
}

func (r *refundReader) GetRefunds(ctx context.Context, referenceIds []int) []*dataloader.Result[[]*models.Refund] {
	var results []*models.Refund
	err := r.db.WithContext(ctx).Where("reference_type = ? AND reference_id IN ?", r.referenceType, referenceIds).Find(&results).Error
	if err != nil {
		return handleError[[]*models.Refund](len(referenceIds), err)
	}

	// key => customer id (int)
	// value => array of billing address pointer []*Refund
	resultMap := make(map[int][]*models.Refund)
	for _, result := range results {
		resultMap[result.ReferenceId] = append(resultMap[result.ReferenceId], result)
	}
	var loaderResults []*dataloader.Result[[]*models.Refund]
	for _, id := range referenceIds {
		refunds := resultMap[id]
		loaderResults = append(loaderResults, &dataloader.Result[[]*models.Refund]{Data: refunds})
	}
	return loaderResults
}

func GetCreditNoteRefunds(ctx context.Context, creditNoteId int) ([]*models.Refund, error) {
	loaders := For(ctx)
	return loaders.creditNoteRefundLoader.Load(ctx, creditNoteId)()
}

func GetSupplierCreditRefunds(ctx context.Context, supplierCreditId int) ([]*models.Refund, error) {
	loaders := For(ctx)
	return loaders.supplierCreditRefundLoader.Load(ctx, supplierCreditId)()
}

func GetSupplierAdvanceRefunds(ctx context.Context, supplierAdvanceId int) ([]*models.Refund, error) {
	loaders := For(ctx)
	return loaders.supplierAdvanceRefundLoader.Load(ctx, supplierAdvanceId)()
}

func GetCustomerAdvanceRefunds(ctx context.Context, customerAdvanceId int) ([]*models.Refund, error) {
	loaders := For(ctx)
	return loaders.customerAdvanceRefundLoader.Load(ctx, customerAdvanceId)()
}