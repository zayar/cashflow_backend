package middlewares

import (
	"context"

	"github.com/graph-gophers/dataloader/v7"
	"github.com/mmdatafocus/books_backend/models"
	"gorm.io/gorm"
)

type salesInvoiceReader struct {
	db *gorm.DB
}

func (r *salesInvoiceReader) getSalesInvoices(ctx context.Context, ids []int) []*dataloader.Result[*models.SalesInvoice] {
	var results []models.SalesInvoice
	err := r.db.WithContext(ctx).Where("id IN ?", ids).Find(&results).Error
	if err != nil {
		return handleError[*models.SalesInvoice](len(ids), err)
	}

	return generateLoaderResults(results, ids)
	// resultMap := make(map[int]*models.SalesInvoice)
	// resultMap[0] = &models.SalesInvoice{}
	// for _, result := range results {
	// 	resultMap[result.ID] = result
	// }

	// loaderResults := make([]*dataloader.Result[*models.SalesInvoice], 0, len(ids))
	// for _, id := range ids {
	// 	result := resultMap[id]
	// 	loaderResults = append(loaderResults, &dataloader.Result[*models.SalesInvoice]{Data: result})
	// }
	// return loaderResults
}

func GetSalesInvoice(ctx context.Context, id int) (*models.SalesInvoice, error) {
	loaders := For(ctx)
	return loaders.salesInvoiceLoader.Load(ctx, id)()
}

func GetSalesInvoices(ctx context.Context, ids []int) ([]*models.SalesInvoice, []error) {
	loaders := For(ctx)
	return loaders.salesInvoiceLoader.LoadMany(ctx, ids)()
}
