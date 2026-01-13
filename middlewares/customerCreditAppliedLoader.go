package middlewares

import (
	"context"

	"bitbucket.org/mmdatafocus/books_backend/models"
	"github.com/graph-gophers/dataloader/v7"
	"gorm.io/gorm"
)

type appliedCustomerCreditReader struct {
	db *gorm.DB
}

func (r *appliedCustomerCreditReader) GetAppliedCustomerCredits(ctx context.Context, Ids []int) []*dataloader.Result[[]*models.CustomerCreditInvoice] {
	var results []*models.CustomerCreditInvoice
	err := r.db.WithContext(ctx).Where("invoice_id IN ?", Ids).Find(&results).Error
	if err != nil {
		return handleError[[]*models.CustomerCreditInvoice](len(Ids), err)
	}

	resultMap := make(map[int][]*models.CustomerCreditInvoice)
	for _, result := range results {
		resultMap[result.InvoiceId] = append(resultMap[result.InvoiceId], result)
	}
	var loaderResults []*dataloader.Result[[]*models.CustomerCreditInvoice]
	for _, id := range Ids {
		documents := resultMap[id]
		loaderResults = append(loaderResults, &dataloader.Result[[]*models.CustomerCreditInvoice]{Data: documents})
	}
	return loaderResults
}

func GetAppliedCustomerCredits(ctx context.Context, invoiceId int) ([]*models.CustomerCreditInvoice, error) {
	loaders := For(ctx)
	return loaders.appliedCustomerCreditLoader.Load(ctx, invoiceId)()
}
