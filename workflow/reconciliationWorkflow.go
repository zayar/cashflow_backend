package workflow

import (
	"strconv"

	"github.com/mmdatafocus/books_backend/config"
	"github.com/mmdatafocus/books_backend/models"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

func ProcessReconciliationWorkflow(tx *gorm.DB, logger *logrus.Logger, pubSubMsg config.PubSubMessage) error {
	businessId := pubSubMsg.BusinessId
	var records []models.PubSubMessageRecord
	var err error
	err = tx.Where("business_id = ? AND is_processed = 0", businessId).Find(&records).Error
	if err != nil {
		config.LogError(logger, "ReconciliationWorkflow.go", "ProcessReconciliationWorkflow", "Querying PubSubMessageRecords", pubSubMsg, err)
		return err
	}

	for _, record := range records {
		// Durable idempotency per outbox record (reconcile can be retried safely).
		handlerName := string(record.ReferenceType)
		messageId := strconv.Itoa(record.ID)
		skip, err := BeginIdempotency(tx, businessId, handlerName, messageId)
		if err != nil {
			return err
		}
		if skip {
			continue
		}

		msg := config.PubSubMessage{
			ID:                  record.ID,
			BusinessId:          record.BusinessId,
			TransactionDateTime: record.TransactionDateTime,
			ReferenceId:         record.ReferenceId,
			ReferenceType:       string(record.ReferenceType),
			Action:              string(record.Action),
			OldObj:              record.OldObj,
			NewObj:              record.NewObj,
			CorrelationId:       record.CorrelationId,
		}
		switch record.ReferenceType {
		case models.AccountReferenceTypeOpeningBalance:
			err = ProcessOpeningBalanceWorkflow(tx, logger, msg)
		case models.AccountReferenceTypeProductOpeningStock:
			err = ProcessProductOpeningStockWorkflow(tx, logger, msg)
		case models.AccountReferenceTypeProductGroupOpeningStock:
			err = ProcessProductGroupOpeningStockWorkflow(tx, logger, msg)
		case models.AccountReferenceTypeInventoryAdjustmentQuantity:
			err = ProcessInventoryAdjustmentQuantityWorkflow(tx, logger, msg)
		case models.AccountReferenceTypeInventoryAdjustmentValue:
			err = ProcessInventoryAdjustmentValueWorkflow(tx, logger, msg)
		case models.AccountReferenceTypeTransferOrder:
			err = ProcessTransferOrderWorkflow(tx, logger, msg)
		case models.AccountReferenceTypeAccountTransfer,
			models.AccountReferenceTypeAccountDeposit,
			models.AccountReferenceTypeOwnerContribution,
			models.AccountReferenceTypeOwnerDrawing,
			models.AccountReferenceTypeOtherIncome,
			models.AccountReferenceTypeAdvanceCustomerPayment,
			models.AccountReferenceTypeAdvanceSupplierPayment,
			models.AccountReferenceTypeSupplierCreditRefund,
			models.AccountReferenceTypeCreditNoteRefund,
			models.AccountReferenceTypeSupplierAdvanceRefund,
			models.AccountReferenceTypeCustomerAdvanceRefund:

			err = ProcessBankingTransactionWorkflow(tx, logger, msg)
		case models.AccountReferenceTypeBill:
			err = ProcessBillWorkflow(tx, logger, msg)
		case models.AccountReferenceTypeInvoice:
			err = ProcessInvoiceWorkflow(tx, logger, msg)
		case models.AccountReferenceTypeInvoiceWriteOff:
			err = ProcessInvoiceWriteOffWorkflow(tx, logger, msg)
		case models.AccountReferenceTypeCustomerOpeningBalance:
			err = ProcessCustomerOpeningBalanceWorkflow(tx, logger, msg)
		case models.AccountReferenceTypeCreditNote:
			err = ProcessCreditNoteWorkflow(tx, logger, msg)
		case models.AccountReferenceTypeExpense:
			err = ProcessExpenseWorkflow(tx, logger, msg)
		case models.AccountReferenceTypeJournal:
			err = ProcessManualJournalWorkflow(tx, logger, msg)
		case models.AccountReferenceTypeSupplierOpeningBalance:
			err = ProcessSupplierOpeningBalanceWorkflow(tx, logger, msg)
		case models.AccountReferenceTypeSupplierCredit:
			err = ProcessSupplierCreditWorkflow(tx, logger, msg)
		case models.AccountReferenceTypeSupplierPayment:
			err = ProcessSupplierPaymentWorkflow(tx, logger, msg)
		case models.AccountReferenceTypeCustomerPayment:
			err = ProcessCustomerPaymentWorkflow(tx, logger, msg)
		case models.AccountReferenceTypeSupplierAdvanceApplied:
			err = ProcessSupplierAdvanceAppliedWorkflow(tx, logger, msg)
		case models.AccountReferenceTypeCustomerAdvanceApplied:
			err = ProcessCustomerAdvanceAppliedWorkflow(tx, logger, msg)
		case models.AccountReferenceTypePosInvoicePayment:
			err = ProcessPosInvoicePaymentWorkflow(tx, logger, msg)
		}
		if err != nil {
			_ = MarkIdempotencyFailed(tx, businessId, handlerName, messageId, err)
			return err
		}
		if err := MarkIdempotencySucceeded(tx, businessId, handlerName, messageId); err != nil {
			return err
		}
	}
	return nil
}
