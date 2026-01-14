package middlewares

import (
	"context"
	"reflect"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/graph-gophers/dataloader/v7"
	"github.com/mmdatafocus/books_backend/config"
	"github.com/mmdatafocus/books_backend/models"
	"gorm.io/gorm"
)

type ctxKey string

const (
	loadersKey = ctxKey("dataloaders")
)

// Loaders wrap your data loaders to inject via middleware
type Loaders struct {
	AccountLoader         *dataloader.Loader[int, *models.Account]
	StateLoader           *dataloader.Loader[int, *models.State]
	TownshipLoader        *dataloader.Loader[int, *models.Township]
	CurrencyLoader        *dataloader.Loader[int, *models.Currency]
	BranchLoader          *dataloader.Loader[int, *models.Branch]
	RoleLoader            *dataloader.Loader[int, *models.Role]
	ModuleLoader          *dataloader.Loader[int, *models.Module]
	RoleModuleLoader      *dataloader.Loader[int, []*models.RoleModule]
	productCategoryLoader *dataloader.Loader[int, *models.ProductCategory]
	productUnitLoader     *dataloader.Loader[int, *models.ProductUnit]
	productVariantLoader  *dataloader.Loader[int, *models.ProductVariant]
	productGroupLoader    *dataloader.Loader[int, *models.ProductGroup]
	productModifierLoader *dataloader.Loader[int, *models.ProductModifier]
	productLoader         *dataloader.Loader[int, *models.Product]
	salesPersonLoader     *dataloader.Loader[int, *models.SalesPerson]
	deliveryMethodLoader  *dataloader.Loader[int, *models.DeliveryMethod]
	warehouseLoader       *dataloader.Loader[int, *models.Warehouse]
	paymentModeLoader     *dataloader.Loader[int, *models.PaymentMode]
	allPaymentModeLoader  *dataloader.Loader[int, *models.AllPaymentMode]
	allProductUnitLoader  *dataloader.Loader[int, *models.AllProductUnit]

	customerLoader                *dataloader.Loader[int, *models.Customer]
	customerBillingAddressLoader  *dataloader.Loader[int, *models.BillingAddress]
	customerShippingAddressLoader *dataloader.Loader[int, *models.ShippingAddress]
	customerContactPersonLoader   *dataloader.Loader[int, []*models.ContactPerson]
	customerDocumentLoader        *dataloader.Loader[int, []*models.Document]

	supplierLoader *dataloader.Loader[int, *models.Supplier]

	supplierBillingAddressLoader  *dataloader.Loader[int, *models.BillingAddress]
	supplierShippingAddressLoader *dataloader.Loader[int, *models.ShippingAddress]
	supplierContactPersonLoader   *dataloader.Loader[int, []*models.ContactPerson]
	supplierDocumentLoader        *dataloader.Loader[int, []*models.Document]

	purchaseOrderLoader         *dataloader.Loader[int, *models.PurchaseOrder]
	purchaseOrderDetailLoader   *dataloader.Loader[int, []*models.PurchaseOrderDetail]
	purchaseOrderDocumentLoader *dataloader.Loader[int, []*models.Document]

	billLoader         *dataloader.Loader[int, *models.Bill]
	billDetailLoader   *dataloader.Loader[int, []*models.BillDetail]
	billDocumentLoader *dataloader.Loader[int, []*models.Document]

	salesOrderLoader           *dataloader.Loader[int, *models.SalesOrder]
	salesOrderDetailLoader     *dataloader.Loader[int, []*models.SalesOrderDetail]
	salesOrderDocumentLoader   *dataloader.Loader[int, []*models.Document]
	salesInvoiceLoader         *dataloader.Loader[int, *models.SalesInvoice]
	salesInvoiceDetailLoader   *dataloader.Loader[int, []*models.SalesInvoiceDetail]
	salesInvoiceDocumentLoader *dataloader.Loader[int, []*models.Document]

	customerPaidInvoiceLoader     *dataloader.Loader[int, []*models.PaidInvoice]
	customerPaymentDocumentLoader *dataloader.Loader[int, []*models.Document]

	supplierPaidBillLoader        *dataloader.Loader[int, []*models.SupplierPaidBill]
	supplierPaymentDocumentLoader *dataloader.Loader[int, []*models.Document]

	recurringBillDetailLoader    *dataloader.Loader[int, []*models.RecurringBillDetail]
	supplierCreditDetailLoader   *dataloader.Loader[int, []*models.SupplierCreditDetail]
	supplierCreditDocumentLoader *dataloader.Loader[int, []*models.Document]

	creditNoteDetailsLoader *dataloader.Loader[int, []*models.CreditNoteDetail]

	accountJournalLoader *dataloader.Loader[int, *models.AccountJournal]

	taxLoader      *dataloader.Loader[int, *models.Tax]
	taxGroupLoader *dataloader.Loader[int, *models.TaxGroup]

	productVariantByGroupLoader *dataloader.Loader[int, []*models.ProductVariant]

	// productStockLoader *dataloader.Loader[int, []*models.Stock]

	recentAccountTransactionLoader *dataloader.Loader[int, []*models.AccountTransaction]
	accountClosingBalanceLoader    *dataloader.Loader[int, *models.AccountCurrencyDailyBalance]

	allDeliveryMethodLoader  *dataloader.Loader[int, *models.AllDeliveryMethod]
	allTownshipLoader        *dataloader.Loader[int, *models.AllTownship]
	allBranchLoader          *dataloader.Loader[int, *models.AllBranch]
	allCurrencyLoader        *dataloader.Loader[int, *models.AllCurrency]
	allProductModifierLoader *dataloader.Loader[int, *models.AllProductModifier]
	// allProductUnitLoader        *dataloader.Loader[int, *models.AllProductUnit]
	allRoleLoader            *dataloader.Loader[int, *models.AllRole]
	allSalesPersonLoader     *dataloader.Loader[int, *models.AllSalesPerson]
	allStateLoader           *dataloader.Loader[int, *models.AllState]
	allAccountLoader         *dataloader.Loader[int, *models.AllAccount]
	allMoneyAccountLoader    *dataloader.Loader[int, *models.AllMoneyAccount]
	allReasonLoader          *dataloader.Loader[int, *models.AllReason]
	allTaxGroupLoader        *dataloader.Loader[int, *models.AllTaxGroup]
	allUserLoader            *dataloader.Loader[int, *models.AllUser]
	allProductCategoryLoader *dataloader.Loader[int, *models.AllProductCategory]
	// allPaymentModeLoader        *dataloader.Loader[int, *models.AllPaymentMode]
	allWarehouseLoader          *dataloader.Loader[int, *models.AllWarehouse]
	allShipmentPreferenceLoader *dataloader.Loader[int, *models.AllShipmentPreference]
	allTaxLoader                *dataloader.Loader[int, *models.AllTax]

	transferOrderDetailLoader   *dataloader.Loader[int, []*models.TransferOrderDetail]
	transferOrderDocumentLoader *dataloader.Loader[int, []*models.Document]

	inventoryAdjustmentDetailLoader   *dataloader.Loader[int, []*models.InventoryAdjustmentDetail]
	inventoryAdjustmentDocumentLoader *dataloader.Loader[int, []*models.Document]

	productImageLoader      *dataloader.Loader[int, []*models.Image]
	productGroupImageLoader *dataloader.Loader[int, []*models.Image]

	bankingTransactionDetailLoader   *dataloader.Loader[int, []*models.BankingTransactionDetail]
	recentBankingTransactionLoader   *dataloader.Loader[int, []*models.BankingTransaction]
	bankingTransactionDocumentLoader *dataloader.Loader[int, []*models.Document]

	supplierCreditBillLoader    *dataloader.Loader[int, []*models.SupplierCreditBill]
	appliedSupplierCreditLoader *dataloader.Loader[int, []*models.SupplierCreditBill]
	customerCreditInvoiceLoader *dataloader.Loader[int, []*models.CustomerCreditInvoice]
	appliedCustomerCreditLoader *dataloader.Loader[int, []*models.CustomerCreditInvoice]

	creditNoteRefundLoader      *dataloader.Loader[int, []*models.Refund]
	supplierCreditRefundLoader  *dataloader.Loader[int, []*models.Refund]
	customerAdvanceRefundLoader *dataloader.Loader[int, []*models.Refund]
	supplierAdvanceRefundLoader *dataloader.Loader[int, []*models.Refund]

	accountTransferTransactionDocumentLoader     *dataloader.Loader[int, []*models.Document]
	expenseDocumentLoader, journalDocumentLoader *dataloader.Loader[int, []*models.Document]
	creditNoteDocumentLoader                     *dataloader.Loader[int, []*models.Document]
}

// NewLoaders instantiates data loaders for the middleware
func NewLoaders(conn *gorm.DB) *Loaders {
	// define the data loader
	accountReader := &accountReader{db: conn}
	stateReader := &stateReader{db: conn}
	townshipReader := &townshipReader{db: conn}
	currencyReader := &currencyReader{db: conn}
	branchReader := &branchReader{db: conn}
	roleReader := &roleReader{db: conn}
	moduleReader := &moduleReader{db: conn}
	roleModuleReader := &RoleModuleReader{db: conn}
	productReader := &productReader{db: conn}
	productCategoryReader := &productCategoryReader{db: conn}
	productUnitReader := &productUnitReader{db: conn}
	productVariantReader := &productVariantReader{db: conn}
	productGroupReader := &productGroupReader{db: conn}
	productModifierReader := &productModifierReader{db: conn}
	salesPersonReader := &salesPersonReader{db: conn}
	deliveryMethodReader := &deliveryMethodReader{db: conn}
	warehouseReader := &warehouseReader{db: conn}
	paymentModeReader := &paymentModeReader{db: conn}
	allPaymentModeReader := &allPaymentModeReader{db: conn}
	allProductUnitReader := &allProductUnitReader{db: conn}

	customerReader := &customerReader{db: conn}
	customerBillingAddressReader := &customerBillingAddressReader{db: conn}
	customerShippingAddressReader := &customerShippingAddressReader{db: conn}
	customerContactPersonReader := &customerContactPersonReader{db: conn}

	customerDocumentReader := &documentReader{db: conn, referenceType: "customers"}
	supplierDocumentReader := &documentReader{db: conn, referenceType: "suppliers"}
	purchaseOrderDocumentReader := &documentReader{db: conn, referenceType: "purchase_orders"}
	billDocumentReader := &documentReader{db: conn, referenceType: "bills"}
	supplierPaymentDocumentReader := &documentReader{db: conn, referenceType: "supplier_payments"}
	supplierCreditDocumentReader := &documentReader{db: conn, referenceType: "supplier_credits"}
	salesOrderDocumentReader := &documentReader{db: conn, referenceType: "sales_orders"}
	salesInvoiceDocumentReader := &documentReader{db: conn, referenceType: "sales_invoices"}
	customerPaymentDocumentReader := &documentReader{db: conn, referenceType: "customer_payments"}
	transferOrderDocumentReader := &documentReader{db: conn, referenceType: "transfer_orders"}
	inventoryAdjustmentDocumentReader := &documentReader{db: conn, referenceType: "inventory_adjustments"}
	bankingTransactionDocumentReader := &documentReader{db: conn, referenceType: "banking_transactions"}

	supplierReader := &supplierReader{db: conn}
	supplierBillingAddressReader := &supplierBillingAddressReader{db: conn}
	supplierShippingAddressReader := &supplierShippingAddressReader{db: conn}
	supplierContactPersonReader := &supplierContactPersonReader{db: conn}

	purchaseOrderReader := &purchaseOrderReader{db: conn}
	purchaseOrderDetailReader := &purchaseOrderDetailReader{db: conn}

	billReader := &billReader{db: conn}
	billDetailReader := &billDetailReader{db: conn}

	salesOrderReader := &salesOrderReader{db: conn}
	salesOrderDetailReader := &salesOrderDetailReader{db: conn}
	salesInvoiceReader := &salesInvoiceReader{db: conn}
	salesInvoiceDetailReader := &salesInvoiceDetailReader{db: conn}

	customerPaidInvoiceReader := &customerPaidInvoiceReader{db: conn}

	supplierPaidBillReader := &supplierPaidBillReader{db: conn}

	recurringBillDetailReader := &recurringBillDetailReader{db: conn}
	supplierCreditDetailReader := &supplierCreditDetailReader{db: conn}

	creditNoteDetailsReader := &creditNoteDetailsReader{db: conn}

	accountJournalReader := &accountJournalReader{db: conn}

	taxReader := &taxReader{db: conn}
	taxGroupReader := &taxGroupReader{db: conn}

	productVariantByGroupReader := &productVariantByGroupReader{db: conn}

	// productStockReader := &productStockReader{db: conn}

	recentAccountTransactionReader := &recentAccountTransactionReader{db: conn}

	accountClosingBalanceReader := &accountClosingBalanceReader{db: conn}

	allDeliveryMethodReader := &allDeliveryMethodReader{db: conn}
	allTownshipReader := &allTownshipReader{db: conn}
	allBranchReader := &allBranchReader{db: conn}
	allCurrencyReader := &allCurrencyReader{db: conn}
	allProductModifierReader := &allProductModifierReader{db: conn}
	// allProductUnitReader := &allProductUnitReader{db: conn}
	allRoleReader := &allRoleReader{db: conn}
	allSalesPersonReader := &allSalesPersonReader{db: conn}
	allStateReader := &allStateReader{db: conn}
	allAccountReader := &allAccountReader{db: conn}
	allMoneyAccountReader := &allMoneyAccountReader{db: conn}
	allReasonReader := &allReasonReader{db: conn}
	allTaxGroupReader := &allTaxGroupReader{db: conn}
	allUserReader := &allUserReader{db: conn}
	allProductCategoryReader := &allProductCategoryReader{db: conn}
	// allPaymentModeReader := &allPaymentModeReader{db: conn}
	allWarehouseReader := &allWarehouseReader{db: conn}
	allShipmentPreferenceReader := &allShipmentPreferenceReader{db: conn}
	allTaxReader := &allTaxReader{db: conn}

	transferOrderDetailReader := &transferOrderDetailReader{db: conn}

	inventoryAdjustmentDetailReader := &inventoryAdjustmentDetailReader{db: conn}

	productImageReader := &imageReader{db: conn, referenceType: "products"}
	productGroupImageReader := &imageReader{db: conn, referenceType: "product_groups"}

	bankingTransactionDetailReader := &bankingTransactionDetailReader{db: conn}
	recentBankingTransactionReader := &recentBankingTransactionReader{db: conn}

	supplierCreditBillReader := &supplierCreditBillReader{db: conn}
	appliedSupplierCreditReader := &appliedSupplierCreditReader{db: conn}
	customerCreditInvoiceReader := &customerCreditInvoiceReader{db: conn}
	appliedCustomerCreditReader := &appliedCustomerCreditReader{db: conn}

	creditNoteRefundReader := &refundReader{db: conn, referenceType: string(models.RefundReferenceTypeCreditNote)}
	supplierCreditRefundReader := &refundReader{db: conn, referenceType: string(models.RefundReferenceTypeSupplierCredit)}
	customerAdvanceRefundReader := &refundReader{db: conn, referenceType: string(models.RefundReferenceTypeCustomerAdvance)}
	supplierAdvanceRefundReader := &refundReader{db: conn, referenceType: string(models.RefundReferenceTypeSupplierAdvance)}

	accountTransferTransactionDocumentReader := &documentReader{db: conn, referenceType: "account_transfer_transactions"}
	expenseDocumentReader := &documentReader{db: conn, referenceType: "expenses"}
	journalDocumentReader := &documentReader{db: conn, referenceType: "journals"}
	creditNoteDocumentReader := &documentReader{db: conn, referenceType: "credit_notes"}

	return &Loaders{
		AccountLoader:                 dataloader.NewBatchedLoader(accountReader.getAccounts, dataloader.WithWait[int, *models.Account](time.Millisecond)),
		StateLoader:                   dataloader.NewBatchedLoader(stateReader.getStates, dataloader.WithWait[int, *models.State](time.Millisecond)),
		TownshipLoader:                dataloader.NewBatchedLoader(townshipReader.getTownships, dataloader.WithWait[int, *models.Township](time.Millisecond)),
		CurrencyLoader:                dataloader.NewBatchedLoader(currencyReader.getCurrencies, dataloader.WithWait[int, *models.Currency](time.Millisecond)),
		BranchLoader:                  dataloader.NewBatchedLoader(branchReader.getBranches, dataloader.WithWait[int, *models.Branch](time.Millisecond)),
		RoleLoader:                    dataloader.NewBatchedLoader(roleReader.getRoles, dataloader.WithWait[int, *models.Role](time.Millisecond)),
		ModuleLoader:                  dataloader.NewBatchedLoader(moduleReader.getModules, dataloader.WithWait[int, *models.Module](time.Millisecond)),
		RoleModuleLoader:              dataloader.NewBatchedLoader(roleModuleReader.getRoleModules, dataloader.WithWait[int, []*models.RoleModule](time.Millisecond)),
		productLoader:                 dataloader.NewBatchedLoader(productReader.getProducts, dataloader.WithWait[int, *models.Product](time.Millisecond)),
		productCategoryLoader:         dataloader.NewBatchedLoader(productCategoryReader.getProductCategories, dataloader.WithWait[int, *models.ProductCategory](time.Millisecond)),
		productUnitLoader:             dataloader.NewBatchedLoader(productUnitReader.getProductUnits, dataloader.WithWait[int, *models.ProductUnit](time.Millisecond)),
		productVariantLoader:          dataloader.NewBatchedLoader(productVariantReader.getProductVariants, dataloader.WithWait[int, *models.ProductVariant](time.Millisecond)),
		productGroupLoader:            dataloader.NewBatchedLoader(productGroupReader.getProductGroups, dataloader.WithWait[int, *models.ProductGroup](time.Millisecond)),
		productModifierLoader:         dataloader.NewBatchedLoader(productModifierReader.getProductModifiers, dataloader.WithWait[int, *models.ProductModifier](time.Millisecond)),
		salesPersonLoader:             dataloader.NewBatchedLoader(salesPersonReader.getSalesPersons, dataloader.WithWait[int, *models.SalesPerson](time.Millisecond)),
		deliveryMethodLoader:          dataloader.NewBatchedLoader(deliveryMethodReader.getDeliveryMethods, dataloader.WithWait[int, *models.DeliveryMethod](time.Millisecond)),
		warehouseLoader:               dataloader.NewBatchedLoader(warehouseReader.getWarehouses, dataloader.WithWait[int, *models.Warehouse](time.Millisecond)),
		paymentModeLoader:             dataloader.NewBatchedLoader(paymentModeReader.getPaymentModes, dataloader.WithWait[int, *models.PaymentMode](time.Millisecond)),
		customerLoader:                dataloader.NewBatchedLoader(customerReader.getCustomers, dataloader.WithWait[int, *models.Customer](time.Millisecond)),
		customerBillingAddressLoader:  dataloader.NewBatchedLoader(customerBillingAddressReader.GetCustomerBillingAddresses, dataloader.WithWait[int, *models.BillingAddress](time.Millisecond)),
		customerShippingAddressLoader: dataloader.NewBatchedLoader(customerShippingAddressReader.GetCustomerShippingAddresses, dataloader.WithWait[int, *models.ShippingAddress](time.Millisecond)),
		customerContactPersonLoader:   dataloader.NewBatchedLoader(customerContactPersonReader.GetCustomerContactPersons, dataloader.WithWait[int, []*models.ContactPerson](time.Millisecond)),
		customerDocumentLoader:        dataloader.NewBatchedLoader(customerDocumentReader.GetDocuments, dataloader.WithWait[int, []*models.Document](time.Millisecond)),

		supplierLoader: dataloader.NewBatchedLoader(supplierReader.getSuppliers, dataloader.WithWait[int, *models.Supplier](time.Millisecond)),

		supplierBillingAddressLoader:  dataloader.NewBatchedLoader(supplierBillingAddressReader.GetSupplierBillingAddresses, dataloader.WithWait[int, *models.BillingAddress](time.Millisecond)),
		supplierShippingAddressLoader: dataloader.NewBatchedLoader(supplierShippingAddressReader.GetSupplierShippingAddresses, dataloader.WithWait[int, *models.ShippingAddress](time.Millisecond)),
		supplierContactPersonLoader:   dataloader.NewBatchedLoader(supplierContactPersonReader.GetSupplierContactPersons, dataloader.WithWait[int, []*models.ContactPerson](time.Millisecond)),
		supplierDocumentLoader:        dataloader.NewBatchedLoader(supplierDocumentReader.GetDocuments, dataloader.WithWait[int, []*models.Document](time.Millisecond)),

		purchaseOrderLoader:         dataloader.NewBatchedLoader(purchaseOrderReader.getPurchaseOrders, dataloader.WithWait[int, *models.PurchaseOrder](time.Millisecond)),
		purchaseOrderDetailLoader:   dataloader.NewBatchedLoader(purchaseOrderDetailReader.GetPurchaseOrderDetails, dataloader.WithWait[int, []*models.PurchaseOrderDetail](time.Millisecond)),
		purchaseOrderDocumentLoader: dataloader.NewBatchedLoader(purchaseOrderDocumentReader.GetDocuments, dataloader.WithWait[int, []*models.Document](time.Millisecond)),

		billLoader:         dataloader.NewBatchedLoader(billReader.getBills, dataloader.WithWait[int, *models.Bill](time.Millisecond)),
		billDetailLoader:   dataloader.NewBatchedLoader(billDetailReader.GetBillDetails, dataloader.WithWait[int, []*models.BillDetail](time.Millisecond)),
		billDocumentLoader: dataloader.NewBatchedLoader(billDocumentReader.GetDocuments, dataloader.WithWait[int, []*models.Document](time.Millisecond)),

		salesOrderLoader:           dataloader.NewBatchedLoader(salesOrderReader.getSalesOrders, dataloader.WithWait[int, *models.SalesOrder](time.Millisecond)),
		salesOrderDetailLoader:     dataloader.NewBatchedLoader(salesOrderDetailReader.GetSalesOrderDetails, dataloader.WithWait[int, []*models.SalesOrderDetail](time.Millisecond)),
		salesOrderDocumentLoader:   dataloader.NewBatchedLoader(salesOrderDocumentReader.GetDocuments, dataloader.WithWait[int, []*models.Document](time.Millisecond)),
		salesInvoiceLoader:         dataloader.NewBatchedLoader(salesInvoiceReader.getSalesInvoices, dataloader.WithWait[int, *models.SalesInvoice](time.Millisecond)),
		salesInvoiceDetailLoader:   dataloader.NewBatchedLoader(salesInvoiceDetailReader.GetSalesInvoiceDetails, dataloader.WithWait[int, []*models.SalesInvoiceDetail](time.Millisecond)),
		salesInvoiceDocumentLoader: dataloader.NewBatchedLoader(salesInvoiceDocumentReader.GetDocuments, dataloader.WithWait[int, []*models.Document](time.Millisecond)),

		customerPaidInvoiceLoader:     dataloader.NewBatchedLoader(customerPaidInvoiceReader.GetCustomerPaidInvoices, dataloader.WithWait[int, []*models.PaidInvoice](time.Millisecond)),
		customerPaymentDocumentLoader: dataloader.NewBatchedLoader(customerPaymentDocumentReader.GetDocuments, dataloader.WithWait[int, []*models.Document](time.Millisecond)),

		supplierPaidBillLoader:        dataloader.NewBatchedLoader(supplierPaidBillReader.GetSupplierPaidBills, dataloader.WithWait[int, []*models.SupplierPaidBill](time.Millisecond)),
		supplierPaymentDocumentLoader: dataloader.NewBatchedLoader(supplierPaymentDocumentReader.GetDocuments, dataloader.WithWait[int, []*models.Document](time.Millisecond)),

		recurringBillDetailLoader:    dataloader.NewBatchedLoader(recurringBillDetailReader.GetRecurringBillDetails, dataloader.WithWait[int, []*models.RecurringBillDetail](time.Millisecond)),
		supplierCreditDetailLoader:   dataloader.NewBatchedLoader(supplierCreditDetailReader.GetSupplierCreditDetails, dataloader.WithWait[int, []*models.SupplierCreditDetail](time.Millisecond)),
		supplierCreditDocumentLoader: dataloader.NewBatchedLoader(supplierCreditDocumentReader.GetDocuments, dataloader.WithWait[int, []*models.Document](time.Millisecond)),

		creditNoteDetailsLoader: dataloader.NewBatchedLoader(creditNoteDetailsReader.GetDetails, dataloader.WithWait[int, []*models.CreditNoteDetail](time.Millisecond)),

		accountJournalLoader: dataloader.NewBatchedLoader(accountJournalReader.GetAccountJournals, dataloader.WithWait[int, *models.AccountJournal](time.Millisecond)),

		taxLoader:      dataloader.NewBatchedLoader(taxReader.getTaxes, dataloader.WithWait[int, *models.Tax](time.Millisecond)),
		taxGroupLoader: dataloader.NewBatchedLoader(taxGroupReader.getTaxGroups, dataloader.WithWait[int, *models.TaxGroup](time.Millisecond)),

		productVariantByGroupLoader: dataloader.NewBatchedLoader(productVariantByGroupReader.getProductVariantsByGroupId, dataloader.WithWait[int, []*models.ProductVariant](time.Millisecond)),

		// productStockLoader: dataloader.NewBatchedLoader(productStockReader.GetProductStocks, dataloader.WithWait[int, []*models.Stock](time.Millisecond)),

		recentAccountTransactionLoader: dataloader.NewBatchedLoader(recentAccountTransactionReader.GetRecentAccountTransactions, dataloader.WithWait[int, []*models.AccountTransaction](time.Millisecond)),
		accountClosingBalanceLoader:    dataloader.NewBatchedLoader(accountClosingBalanceReader.GetAccountClosingBalances, dataloader.WithWait[int, *models.AccountCurrencyDailyBalance](time.Millisecond)),

		allPaymentModeLoader: dataloader.NewBatchedLoader(allPaymentModeReader.getAllPaymentModes, dataloader.WithWait[int, *models.AllPaymentMode](time.Millisecond)),
		allProductUnitLoader: dataloader.NewBatchedLoader(allProductUnitReader.getAllProductUnits, dataloader.WithWait[int, *models.AllProductUnit](time.Millisecond)),

		allDeliveryMethodLoader:  dataloader.NewBatchedLoader(allDeliveryMethodReader.getAllDeliveryMethods, dataloader.WithWait[int, *models.AllDeliveryMethod](time.Millisecond)),
		allTownshipLoader:        dataloader.NewBatchedLoader(allTownshipReader.getAllTownships, dataloader.WithWait[int, *models.AllTownship](time.Millisecond)),
		allBranchLoader:          dataloader.NewBatchedLoader(allBranchReader.getAllBranchs, dataloader.WithWait[int, *models.AllBranch](time.Millisecond)),
		allCurrencyLoader:        dataloader.NewBatchedLoader(allCurrencyReader.getAllCurrencys, dataloader.WithWait[int, *models.AllCurrency](time.Millisecond)),
		allProductModifierLoader: dataloader.NewBatchedLoader(allProductModifierReader.getAllProductModifiers, dataloader.WithWait[int, *models.AllProductModifier](time.Millisecond)),
		// allProductUnitLoader: dataloader.NewBatchedLoader(allProductUnitReader.getAllProductUnits, dataloader.WithWait[int, *models.AllProductUnit](time.Millisecond)),
		allRoleLoader:            dataloader.NewBatchedLoader(allRoleReader.getAllRoles, dataloader.WithWait[int, *models.AllRole](time.Millisecond)),
		allSalesPersonLoader:     dataloader.NewBatchedLoader(allSalesPersonReader.getAllSalesPersons, dataloader.WithWait[int, *models.AllSalesPerson](time.Millisecond)),
		allStateLoader:           dataloader.NewBatchedLoader(allStateReader.getAllStates, dataloader.WithWait[int, *models.AllState](time.Millisecond)),
		allAccountLoader:         dataloader.NewBatchedLoader(allAccountReader.getAllAccounts, dataloader.WithWait[int, *models.AllAccount](time.Millisecond)),
		allMoneyAccountLoader:    dataloader.NewBatchedLoader(allMoneyAccountReader.getAllMoneyAccounts, dataloader.WithWait[int, *models.AllMoneyAccount](time.Millisecond)),
		allReasonLoader:          dataloader.NewBatchedLoader(allReasonReader.getAllReasons, dataloader.WithWait[int, *models.AllReason](time.Millisecond)),
		allTaxGroupLoader:        dataloader.NewBatchedLoader(allTaxGroupReader.getAllTaxGroups, dataloader.WithWait[int, *models.AllTaxGroup](time.Millisecond)),
		allUserLoader:            dataloader.NewBatchedLoader(allUserReader.getAllUsers, dataloader.WithWait[int, *models.AllUser](time.Millisecond)),
		allProductCategoryLoader: dataloader.NewBatchedLoader(allProductCategoryReader.getAllProductCategorys, dataloader.WithWait[int, *models.AllProductCategory](time.Millisecond)),
		// allPaymentModeLoader: dataloader.NewBatchedLoader(allPaymentModeReader.getAllPaymentModes, dataloader.WithWait[int, *models.AllPaymentMode](time.Millisecond)),
		allWarehouseLoader:          dataloader.NewBatchedLoader(allWarehouseReader.getAllWarehouses, dataloader.WithWait[int, *models.AllWarehouse](time.Millisecond)),
		allShipmentPreferenceLoader: dataloader.NewBatchedLoader(allShipmentPreferenceReader.getAllShipmentPreferences, dataloader.WithWait[int, *models.AllShipmentPreference](time.Millisecond)),
		allTaxLoader:                dataloader.NewBatchedLoader(allTaxReader.getAllTaxs, dataloader.WithWait[int, *models.AllTax](time.Millisecond)),

		transferOrderDetailLoader:   dataloader.NewBatchedLoader(transferOrderDetailReader.GetTransferOrderDetails, dataloader.WithWait[int, []*models.TransferOrderDetail](time.Millisecond)),
		transferOrderDocumentLoader: dataloader.NewBatchedLoader(transferOrderDocumentReader.GetDocuments, dataloader.WithWait[int, []*models.Document](time.Millisecond)),

		inventoryAdjustmentDetailLoader:   dataloader.NewBatchedLoader(inventoryAdjustmentDetailReader.GetInventoryAdjustmentDetails, dataloader.WithWait[int, []*models.InventoryAdjustmentDetail](time.Millisecond)),
		inventoryAdjustmentDocumentLoader: dataloader.NewBatchedLoader(inventoryAdjustmentDocumentReader.GetDocuments, dataloader.WithWait[int, []*models.Document](time.Millisecond)),

		// image loaders
		productImageLoader:      dataloader.NewBatchedLoader(productImageReader.GetImages, dataloader.WithWait[int, []*models.Image](time.Millisecond)),
		productGroupImageLoader: dataloader.NewBatchedLoader(productGroupImageReader.GetImages, dataloader.WithWait[int, []*models.Image](time.Millisecond)),

		bankingTransactionDetailLoader:   dataloader.NewBatchedLoader(bankingTransactionDetailReader.GetBankingTransactionDetails, dataloader.WithWait[int, []*models.BankingTransactionDetail](time.Millisecond)),
		recentBankingTransactionLoader:   dataloader.NewBatchedLoader(recentBankingTransactionReader.GetRecentBankingTransactions, dataloader.WithWait[int, []*models.BankingTransaction](time.Millisecond)),
		bankingTransactionDocumentLoader: dataloader.NewBatchedLoader(bankingTransactionDocumentReader.GetDocuments, dataloader.WithWait[int, []*models.Document](time.Millisecond)),

		supplierCreditBillLoader:    dataloader.NewBatchedLoader(supplierCreditBillReader.GetSupplierCreditedBills, dataloader.WithWait[int, []*models.SupplierCreditBill](time.Millisecond)),
		appliedSupplierCreditLoader: dataloader.NewBatchedLoader(appliedSupplierCreditReader.GetAppliedSupplierCredits, dataloader.WithWait[int, []*models.SupplierCreditBill](time.Millisecond)),
		customerCreditInvoiceLoader: dataloader.NewBatchedLoader(customerCreditInvoiceReader.GetCustomerCreditedInvoices, dataloader.WithWait[int, []*models.CustomerCreditInvoice](time.Millisecond)),
		appliedCustomerCreditLoader: dataloader.NewBatchedLoader(appliedCustomerCreditReader.GetAppliedCustomerCredits, dataloader.WithWait[int, []*models.CustomerCreditInvoice](time.Millisecond)),

		creditNoteRefundLoader:      dataloader.NewBatchedLoader(creditNoteRefundReader.GetRefunds, dataloader.WithWait[int, []*models.Refund](time.Millisecond)),
		supplierCreditRefundLoader:  dataloader.NewBatchedLoader(supplierCreditRefundReader.GetRefunds, dataloader.WithWait[int, []*models.Refund](time.Millisecond)),
		customerAdvanceRefundLoader: dataloader.NewBatchedLoader(customerAdvanceRefundReader.GetRefunds, dataloader.WithWait[int, []*models.Refund](time.Millisecond)),
		supplierAdvanceRefundLoader: dataloader.NewBatchedLoader(supplierAdvanceRefundReader.GetRefunds, dataloader.WithWait[int, []*models.Refund](time.Millisecond)),

		accountTransferTransactionDocumentLoader: dataloader.NewBatchedLoader(accountTransferTransactionDocumentReader.GetDocuments, dataloader.WithWait[int, []*models.Document](time.Millisecond)),
		expenseDocumentLoader:                    dataloader.NewBatchedLoader(expenseDocumentReader.GetDocuments, dataloader.WithWait[int, []*models.Document](time.Millisecond)),
		journalDocumentLoader:                    dataloader.NewBatchedLoader(journalDocumentReader.GetDocuments, dataloader.WithWait[int, []*models.Document](time.Millisecond)),
		creditNoteDocumentLoader:                 dataloader.NewBatchedLoader(creditNoteDocumentReader.GetDocuments, dataloader.WithWait[int, []*models.Document](time.Millisecond)),
	}
}

func LoaderMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		loader := NewLoaders(config.GetDB())
		ctx := context.WithValue(c.Request.Context(), loadersKey, loader)
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	}
}

func For(ctx context.Context) *Loaders {
	return ctx.Value(loadersKey).(*Loaders)
}

// handleError creates array of result with the same error repeated for as many items requested
func handleError[T any](itemsLength int, err error) []*dataloader.Result[T] {
	result := make([]*dataloader.Result[T], itemsLength)
	for i := 0; i < itemsLength; i++ {
		result[i] = &dataloader.Result[T]{Error: err}
	}
	return result
}

// turns results from db into dataloader results
// (T must be a struct)
func generateLoaderResults[T models.Data](results []T, ids []int) []*dataloader.Result[*T] {
	// generate resultMap from results
	resultMap := make(map[int]T)
	var resultZero T
	resultMap[0] = resultZero.GetDefault(0).(T)
	for _, result := range results {
		resultMap[result.GetId()] = result
	}

	loaderResults := make([]*dataloader.Result[*T], 0, len(ids))
	for _, id := range ids {
		data := resultMap[id]
		if reflect.ValueOf(data).IsZero() {
			data = data.GetDefault(id).(T)
		}
		loaderResults = append(loaderResults, &dataloader.Result[*T]{Data: &data})
	}
	return loaderResults
}

// T must be struct
// each id has many related results
func generateLoaderArrayResults[T models.RelatedData](results []T, referenceIds []int) (loaderResults []*dataloader.Result[[]*T]) {
	resultMap := make(map[int][]*T)
	for _, result := range results {
		// creating a new variable every turn, to avoid pointing to the adddress of result
		copy := result
		resultMap[result.GetReferenceId()] = append(resultMap[result.GetReferenceId()], &copy)
	}
	for _, id := range referenceIds {
		resultArray := resultMap[id]
		loaderResults = append(loaderResults, &dataloader.Result[[]*T]{Data: resultArray})
	}
	return loaderResults
}
