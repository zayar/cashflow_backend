package workflow

import (
	"context"

	"github.com/mmdatafocus/books_backend/config"
	"github.com/mmdatafocus/books_backend/models"
)

// LockTypeForReferenceType maps a posting reference type to a module lock type.
// If a reference type is not mapped, it means no lock gate is enforced here.
func LockTypeForReferenceType(referenceType string) (models.TransactionLockType, bool) {
	switch referenceType {
	// Sales
	case string(models.AccountReferenceTypeInvoice),
		string(models.AccountReferenceTypeCreditNote),
		string(models.AccountReferenceTypeCustomerPayment),
		string(models.AccountReferenceTypeInvoiceWriteOff),
		string(models.AccountReferenceTypeCustomerOpeningBalance),
		string(models.AccountReferenceTypeAdvanceCustomerPayment):
		return models.SalesTransactionLock, true

	// Purchases
	case string(models.AccountReferenceTypeBill),
		string(models.AccountReferenceTypeSupplierPayment),
		string(models.AccountReferenceTypeSupplierCredit),
		string(models.AccountReferenceTypeSupplierOpeningBalance),
		string(models.AccountReferenceTypeAdvanceSupplierPayment):
		return models.PurchaseTransactionLock, true

	// Banking
	case string(models.AccountReferenceTypeAccountTransfer),
		string(models.AccountReferenceTypeAccountDeposit),
		string(models.AccountReferenceTypeOwnerContribution),
		string(models.AccountReferenceTypeOwnerDrawing),
		string(models.AccountReferenceTypeOtherIncome),
		string(models.AccountReferenceTypeSupplierCreditRefund),
		string(models.AccountReferenceTypeCreditNoteRefund),
		string(models.AccountReferenceTypeSupplierAdvanceRefund),
		string(models.AccountReferenceTypeCustomerAdvanceRefund):
		return models.BankingTransactionLock, true

	// Accountant / Inventory / opening balances
	case string(models.AccountReferenceTypeJournal),
		string(models.AccountReferenceTypeOpeningBalance),
		string(models.AccountReferenceTypeInventoryAdjustmentQuantity),
		string(models.AccountReferenceTypeInventoryAdjustmentValue),
		string(models.AccountReferenceTypeTransferOrder),
		string(models.AccountReferenceTypeProductOpeningStock),
		string(models.AccountReferenceTypeProductGroupOpeningStock):
		return models.AccountantTransactionLock, true
	}
	return "", false
}

// EnforcePostingGate validates period locks for the message (worker-side).
func EnforcePostingGate(ctx context.Context, msg config.PubSubMessage) error {
	lockType, ok := LockTypeForReferenceType(msg.ReferenceType)
	if !ok {
		return nil
	}
	return models.ValidateTransactionLock(ctx, msg.TransactionDateTime, msg.BusinessId, lockType)
}
