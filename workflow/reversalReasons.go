package workflow

// Standardized reasons for ledger reversals.
// These are human-readable strings stored in AccountJournal.reversal_reason.
const (
	ReversalReasonSalesInvoiceVoidUpdate           = "SalesInvoice void/update"
	ReversalReasonBillVoidUpdate                   = "Bill void/update"
	ReversalReasonCreditNoteVoidUpdate             = "Credit note void/update"
	ReversalReasonSupplierCreditVoidUpdate         = "Supplier credit void/update"
	ReversalReasonManualJournalVoidUpdate          = "Manual journal void/update"
	ReversalReasonCustomerPaymentVoidUpdate        = "Customer payment void/update"
	ReversalReasonSupplierPaymentVoidUpdate        = "Supplier payment void/update"
	ReversalReasonExpenseVoidUpdate                = "Expense void/update"
	ReversalReasonInvoiceWriteOffVoidUpdate        = "Invoice write-off void/update"
	ReversalReasonBankingTransactionVoidUpdate     = "Banking transaction void/update"
	ReversalReasonOpeningBalanceResetVoid          = "Opening balance reset/void"
	ReversalReasonCustomerOpeningBalanceResetVoid  = "Customer opening balance reset/void"
	ReversalReasonSupplierOpeningBalanceResetVoid  = "Supplier opening balance reset/void"
	ReversalReasonCustomerAdvanceAppliedVoidUpdate = "Customer advance applied void/update"
	ReversalReasonSupplierAdvanceAppliedVoidUpdate = "Supplier advance applied void/update"
	ReversalReasonSupplierCreditAppliedVoidUpdate  = "Supplier credit applied void/update"
	ReversalReasonCreditNoteAppliedVoidUpdate      = "Credit note applied void/update"
	ReversalReasonInventoryAdjustQtyVoidUpdate     = "Inventory adjustment (qty) void/update"
	ReversalReasonInventoryAdjustValueVoidUpdate   = "Inventory adjustment (value) void/update"
	ReversalReasonInventoryValuationReprice        = "Inventory valuation repricing"
)
