package middlewares

import (
	"context"

	"bitbucket.org/mmdatafocus/books_backend/models"
	"github.com/graph-gophers/dataloader/v7"
	"gorm.io/gorm"
)

type customerPaidInvoiceReader struct {
	db *gorm.DB
}

func (r *customerPaidInvoiceReader) GetCustomerPaidInvoices(ctx context.Context, Ids []int) []*dataloader.Result[[]*models.PaidInvoice] {
	var results []models.PaidInvoice
	err := r.db.WithContext(ctx).Where("customer_payment_id IN ?", Ids).Find(&results).Error
	if err != nil {
		return handleError[[]*models.PaidInvoice](len(Ids), err)
	}

	return generateLoaderArrayResults(results, Ids)
	// key => customer id (int)
	// value => array of billing address pointer []*PaidInvoicee
	// resultMap := make(map[int][]*models.PaidInvoice)
	// for _, result := range results {
	// 	resultMap[result.CustomerPaymentId] = append(resultMap[result.CustomerPaymentId], result)
	// }
	// var loaderResults []*dataloader.Result[[]*models.PaidInvoice]
	// for _, id := range Ids {
	// 	paidBills := resultMap[id]
	// 	loaderResults = append(loaderResults, &dataloader.Result[[]*models.PaidInvoice]{Data: paidBills})
	// }
	// return loaderResults
}

func GetCustomerPaidInvoices(ctx context.Context, orderId int) ([]*models.PaidInvoice, error) {
	loaders := For(ctx)
	return loaders.customerPaidInvoiceLoader.Load(ctx, orderId)()
}
