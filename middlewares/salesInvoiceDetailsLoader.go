package middlewares

import (
	"context"

	"github.com/graph-gophers/dataloader/v7"
	"github.com/mmdatafocus/books_backend/models"
	"gorm.io/gorm"
)

type salesInvoiceDetailReader struct {
	db *gorm.DB
}

func (r *salesInvoiceDetailReader) GetSalesInvoiceDetails(ctx context.Context, Ids []int) []*dataloader.Result[[]*models.SalesInvoiceDetail] {
	var results []models.SalesInvoiceDetail
	err := r.db.WithContext(ctx).Where("sales_invoice_id IN ?", Ids).Find(&results).Error
	if err != nil {
		return handleError[[]*models.SalesInvoiceDetail](len(Ids), err)
	}

	return generateLoaderArrayResults(results, Ids)
	// key => customer id (int)
	// // value => array of billing address pointer []*SalesInvoiceDetaile
	// resultMap := make(map[int][]*models.SalesInvoiceDetail)
	// for _, result := range results {
	// 	resultMap[result.SalesInvoiceId] = append(resultMap[result.SalesInvoiceId], result)
	// }
	// var loaderResults []*dataloader.Result[[]*models.SalesInvoiceDetail]
	// for _, id := range Ids {
	// 	salesInvoiceDetails := resultMap[id]
	// 	loaderResults = append(loaderResults, &dataloader.Result[[]*models.SalesInvoiceDetail]{Data: salesInvoiceDetails})
	// }
	// return loaderResults
}

func GetSalesInvoiceDetails(ctx context.Context, invoiceId int) ([]*models.SalesInvoiceDetail, error) {
	loaders := For(ctx)
	return loaders.salesInvoiceDetailLoader.Load(ctx, invoiceId)()
}
