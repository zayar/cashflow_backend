package middlewares

import (
	"context"

	"github.com/graph-gophers/dataloader/v7"
	"github.com/mmdatafocus/books_backend/models"
	"gorm.io/gorm"
)

type customerCreditInvoiceReader struct {
	db *gorm.DB
}

func (r *customerCreditInvoiceReader) GetCustomerCreditedInvoices(ctx context.Context, Ids []int) []*dataloader.Result[[]*models.CustomerCreditInvoice] {
	var results []models.CustomerCreditInvoice
	err := r.db.WithContext(ctx).Where("reference_type = ? AND reference_id IN ?", models.CustomerCreditApplyTypeCredit, Ids).Find(&results).Error
	if err != nil {
		return handleError[[]*models.CustomerCreditInvoice](len(Ids), err)
	}

	return generateLoaderArrayResults(results, Ids)
}

func GetCustomerCreditedInvoices(ctx context.Context, id int) ([]*models.CustomerCreditInvoice, error) {
	loaders := For(ctx)
	return loaders.customerCreditInvoiceLoader.Load(ctx, id)()
}
