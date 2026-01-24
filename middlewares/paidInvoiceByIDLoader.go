package middlewares

import (
	"context"

	"github.com/graph-gophers/dataloader/v7"
	"github.com/mmdatafocus/books_backend/models"
	"gorm.io/gorm"
)

type paidInvoiceByIDReader struct {
	db *gorm.DB
}

func (r *paidInvoiceByIDReader) getPaidInvoicesByID(ctx context.Context, ids []int) []*dataloader.Result[*models.PaidInvoice] {
	var results []models.PaidInvoice
	err := r.db.WithContext(ctx).Where("id IN ?", ids).Find(&results).Error
	if err != nil {
		return handleError[*models.PaidInvoice](len(ids), err)
	}

	resultMap := make(map[int]*models.PaidInvoice, len(results))
	for i := range results {
		resultMap[results[i].ID] = &results[i]
	}

	loaderResults := make([]*dataloader.Result[*models.PaidInvoice], 0, len(ids))
	for _, id := range ids {
		if v, ok := resultMap[id]; ok {
			loaderResults = append(loaderResults, &dataloader.Result[*models.PaidInvoice]{Data: v})
		} else {
			loaderResults = append(loaderResults, &dataloader.Result[*models.PaidInvoice]{Error: gorm.ErrRecordNotFound})
		}
	}
	return loaderResults
}

func GetPaidInvoice(ctx context.Context, id int) (*models.PaidInvoice, error) {
	loaders := For(ctx)
	return loaders.paidInvoiceByIDLoader.Load(ctx, id)()
}

func GetPaidInvoicesByID(ctx context.Context, ids []int) ([]*models.PaidInvoice, []error) {
	loaders := For(ctx)
	return loaders.paidInvoiceByIDLoader.LoadMany(ctx, ids)()
}

