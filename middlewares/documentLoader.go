package middlewares

import (
	"context"

	"bitbucket.org/mmdatafocus/books_backend/models"
	"github.com/graph-gophers/dataloader/v7"
	"gorm.io/gorm"
)

type documentReader struct {
	db            *gorm.DB
	referenceType string
}

func (r *documentReader) GetDocuments(ctx context.Context, referenceIds []int) []*dataloader.Result[[]*models.Document] {
	var results []*models.Document
	err := r.db.WithContext(ctx).Where("reference_type = ? AND reference_id IN ?", r.referenceType, referenceIds).Find(&results).Error
	if err != nil {
		return handleError[[]*models.Document](len(referenceIds), err)
	}

	// key => customer id (int)
	// value => array of billing address pointer []*Document
	resultMap := make(map[int][]*models.Document)
	for _, result := range results {
		resultMap[result.ReferenceID] = append(resultMap[result.ReferenceID], result)
	}
	var loaderResults []*dataloader.Result[[]*models.Document]
	for _, id := range referenceIds {
		documents := resultMap[id]
		loaderResults = append(loaderResults, &dataloader.Result[[]*models.Document]{Data: documents})
	}
	return loaderResults
}

func GetSupplierDocuments(ctx context.Context, supplierId int) ([]*models.Document, error) {
	loaders := For(ctx)
	return loaders.supplierDocumentLoader.Load(ctx, supplierId)()
}

func GetCustomerDocuments(ctx context.Context, customerId int) ([]*models.Document, error) {
	loaders := For(ctx)
	return loaders.customerDocumentLoader.Load(ctx, customerId)()
}

func GetPurchaseOrderDocuments(ctx context.Context, purchaseOrderId int) ([]*models.Document, error) {
	loaders := For(ctx)
	return loaders.purchaseOrderDocumentLoader.Load(ctx, purchaseOrderId)()
}

func GetBillDocuments(ctx context.Context, billId int) ([]*models.Document, error) {
	loaders := For(ctx)
	return loaders.billDocumentLoader.Load(ctx, billId)()
}

func GetCustomerPaymentDocuments(ctx context.Context, customerPaymentId int) ([]*models.Document, error) {
	loaders := For(ctx)
	return loaders.customerPaymentDocumentLoader.Load(ctx, customerPaymentId)()
}

func GetSalesOrderDocuments(ctx context.Context, salesOrderId int) ([]*models.Document, error) {
	loaders := For(ctx)
	return loaders.salesOrderDocumentLoader.Load(ctx, salesOrderId)()
}

func GetSalesInvoiceDocuments(ctx context.Context, salesInvoiceId int) ([]*models.Document, error) {
	loaders := For(ctx)
	return loaders.salesInvoiceDocumentLoader.Load(ctx, salesInvoiceId)()
}

func GetSupplierPaymentDocuments(ctx context.Context, supplierPaymentId int) ([]*models.Document, error) {
	loaders := For(ctx)
	return loaders.supplierPaymentDocumentLoader.Load(ctx, supplierPaymentId)()
}

func GetSupplierCreditDocuments(ctx context.Context, supplierCreditId int) ([]*models.Document, error) {
	loaders := For(ctx)
	return loaders.supplierCreditDocumentLoader.Load(ctx, supplierCreditId)()
}

func GetInventoryAdjustmentDocuments(ctx context.Context, inventoryAdjustmentId int) ([]*models.Document, error) {
	loaders := For(ctx)
	return loaders.inventoryAdjustmentDocumentLoader.Load(ctx, inventoryAdjustmentId)()
}

func GetTransferOrderDocuments(ctx context.Context, transferOrderId int) ([]*models.Document, error) {
	loaders := For(ctx)
	return loaders.transferOrderDocumentLoader.Load(ctx, transferOrderId)()
}

func GetBankingTransactionDocuments(ctx context.Context, bankingTransactionId int) ([]*models.Document, error) {
	loaders := For(ctx)
	return loaders.bankingTransactionDocumentLoader.Load(ctx, bankingTransactionId)()
}

func GetAccountTransferTransactionDocuments(ctx context.Context, transactionId int) ([]*models.Document, error) {
	loaders := For(ctx)
	return loaders.accountTransferTransactionDocumentLoader.Load(ctx, transactionId)()
}

func GetExpenseDocuments(ctx context.Context, expenseId int) ([]*models.Document, error) {
	loaders := For(ctx)
	return loaders.expenseDocumentLoader.Load(ctx, expenseId)()
}

func GetJournalDocuments(ctx context.Context, journalId int) ([]*models.Document, error) {
	loaders := For(ctx)
	return loaders.journalDocumentLoader.Load(ctx, journalId)()
}

func GetCreditNoteDocuments(ctx context.Context, creditNoteId int) ([]*models.Document, error) {
	loaders := For(ctx)
	return loaders.creditNoteDocumentLoader.Load(ctx, creditNoteId)()
}
