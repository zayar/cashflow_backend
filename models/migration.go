package models

import (
	"log"

	"bitbucket.org/mmdatafocus/books_backend/config"
)

func MigrateTable() {
	db := config.GetDB()

	err := db.AutoMigrate(
		&Account{}, &AccountCurrencyDailyBalance{}, &DailySummary{}, &AccountJournal{}, &AccountTransaction{},
		&BankingTransaction{}, &BankingTransactionDetail{}, &Bill{}, &BillDetail{}, &Branch{}, &Business{}, &TransactionLockingRecord{},
		&CreditNote{}, &CreditNoteDetail{},
		&Comment{}, &Currency{}, &CurrencyExchange{},
		&Customer{}, &CustomerPayment{}, &CustomerCreditInvoice{}, &CustomerCreditAdvance{}, &BillingAddress{}, &ShippingAddress{}, &ContactPerson{}, &Document{},
		&Expense{},
		&DeliveryMethod{}, &PaymentMode{}, &SalesPerson{}, &ShipmentPreference{}, &Reason{},
		&History{},
		&Image{}, &InventoryAdjustment{}, &InventoryAdjustmentDetail{},
		&Journal{}, &JournalTransaction{},
		&Module{}, &MoneyAccount{},
		&Product{}, &ProductGroup{}, &ProductOption{}, &ProductCategory{}, &ProductModifier{}, &ProductModifierUnit{},
		&ProductVariant{}, &ProductUnit{}, &PurchaseOrder{}, &PurchaseOrderDetail{},
		&Refund{}, &RecurringBill{}, &RecurringBillDetail{}, &Role{}, &RoleModule{},
		&SalesOrder{}, &SalesOrderDetail{}, &SalesInvoice{}, &SalesInvoiceDetail{}, &Supplier{}, &SupplierPayment{},
		&SupplierPaidBill{}, &SupplierCredit{}, &SupplierCreditDetail{}, &SupplierCreditBill{}, &SupplierCreditAdvance{}, &State{},
		&StockHistory{}, &StockSummary{}, &StockSummaryDailyBalance{},
		&PaidInvoice{}, &PubSubMessageRecord{}, &PosCheckoutInvoicePayment{},
		&Tax{}, &TaxGroup{}, &Township{}, &TransactionNumberSeries{}, &TransactionNumberSeriesModule{}, &TransferOrder{}, &TransferOrderDetail{},
		&User{},
		&Warehouse{},
		&OpeningBalance{}, &OpeningBalanceDetail{}, &OpeningStock{},
		&IdempotencyKey{},
		&InventoryMovement{}, &CogsAllocation{},
		&ReconciliationReport{},
	)
	if err != nil {
		log.Fatal(err)
	}
}
