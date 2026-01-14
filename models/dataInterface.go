package models

import (
	"time"

	"github.com/mmdatafocus/books_backend/utils"
)

type Identifier interface {
	GetId() int
}

// interface for dataloader result
type Data interface {
	Identifier
	GetDefault(int) Data
}

// key
func (a Account) GetId() int {
	return a.ID
}

func (a Account) GetDefault(id int) Data {
	return Account{
		ID:              id,
		DetailType:      AccountDetailTypeExpense,
		MainType:        AccountMainTypeExpense,
		IsActive:        utils.NewFalse(),
		IsSystemDefault: utils.NewFalse(),
		CreatedAt:       time.Now(),
		UpdatedAt:       time.Now(),
	}
}

func (a AccountJournal) GetId() int {
	return a.ID
}

func (a AccountJournal) GetDefault(id int) Data {
	return AccountJournal{
		ID:                  id,
		TransactionDateTime: time.Now(),
	}
}

func (b Bill) GetId() int {
	return b.ID
}

func (b Bill) GetDefault(id int) Data {
	return Bill{
		ID:               id,
		BillDate:         time.Now(),
		BillPaymentTerms: PaymentTermsCustom,
		CurrentStatus:    BillStatusDraft,
		IsTaxInclusive:   utils.NewFalse(),
		CreatedAt:        time.Now(),
		UpdatedAt:        time.Now(),
	}
}

func (b BillDetail) GetId() int {
	return b.ID
}

func (b BillDetail) GetDefault(id int) Data {
	return BillDetail{
		ID:          id,
		ProductType: ProductTypeSingle,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
}

// id basically means the key for loader
// want to store by referenceId, will not overlap since each reference type has its separate loader
func (b BillingAddress) GetId() int {
	return b.ReferenceID
}

func (b BillingAddress) GetDefault(id int) Data {
	return BillingAddress{
		ID:        id,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
}

func (b Branch) GetId() int {
	return b.ID
}

func (b Branch) GetDefault(id int) Data {
	return Branch{
		ID:        id,
		IsActive:  utils.NewFalse(),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
}

func (c ContactPerson) GetId() int {
	return c.ID
}

func (c ContactPerson) GetDefault(id int) Data {
	return ContactPerson{
		ID:        id,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
}

func (c Currency) GetId() int {
	return c.ID
}

func (c Currency) GetDefault(id int) Data {
	return Currency{
		ID:            id,
		DecimalPlaces: DecimalPlacesZero,
		IsActive:      utils.NewFalse(),
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}
}

func (c Customer) GetId() int {
	return c.ID
}

func (c Customer) GetDefault(id int) Data {
	return Customer{
		ID: id,
		// CustomerTaxType:      TaxTypeIndividual,
		CustomerPaymentTerms: PaymentTermsDueOnReceipt,
		IsActive:             utils.NewFalse(),
		CreatedAt:            time.Now(),
		UpdatedAt:            time.Now(),
	}
}

func (d DeliveryMethod) GetId() int {
	return d.ID
}

func (d DeliveryMethod) GetDefault(id int) Data {
	return DeliveryMethod{
		ID:        id,
		IsActive:  utils.NewFalse(),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
}

func (d Document) GetId() int {
	return d.ID
}

func (d Document) GetDefault(id int) Data {
	return Document{
		ID: id,
	}
}

func (m Module) GetId() int {
	return m.ID
}

func (m Module) GetDefault(id int) Data {
	return Module{
		ID:        id,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
}

func (p PaidInvoice) GetId() int {
	return p.ID
}

func (p PaidInvoice) GetDefault(id int) Data {
	return PaidInvoice{
		ID:        id,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
}

func (p PaymentMode) GetId() int {
	return p.ID
}

func (p PaymentMode) GetDefault(id int) Data {
	return PaymentMode{
		ID:        id,
		IsActive:  utils.NewFalse(),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
}

func (p Product) GetId() int {
	return p.ID
}

func (p Product) GetDefault(id int) Data {
	return Product{
		ID:                  id,
		IsSalesTaxInclusive: utils.NewFalse(),
		IsActive:            utils.NewFalse(),
		CreatedAt:           time.Now(),
		UpdatedAt:           time.Now(),
	}
}

func (p ProductVariant) GetId() int {
	return p.ID
}

func (p ProductVariant) GetDefault(id int) Data {
	return ProductVariant{
		ID:                  id,
		IsSalesTaxInclusive: utils.NewFalse(),
		IsActive:            utils.NewFalse(),
		CreatedAt:           time.Now(),
		UpdatedAt:           time.Now(),
	}
}

func (p ProductCategory) GetId() int {
	return p.ID
}

func (p ProductCategory) GetDefault(id int) Data {
	return ProductCategory{
		ID:        id,
		CreatedAt: time.Now(),
		IsActive:  utils.NewFalse(),
		UpdatedAt: time.Now(),
	}
}

func (p ProductModifier) GetId() int {
	return p.ID
}

func (p ProductModifier) GetDefault(id int) Data {
	return ProductModifier{
		ID:        id,
		CreatedAt: time.Now(),
		IsActive:  utils.NewFalse(),
		UpdatedAt: time.Now(),
	}
}

func (p ProductUnit) GetId() int {
	return p.ID
}

func (p ProductUnit) GetDefault(id int) Data {
	return ProductUnit{
		ID:        id,
		IsActive:  utils.NewFalse(),
		Precision: PrecisionOne,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
}

func (p ProductGroup) GetId() int {
	return p.ID
}

func (p ProductGroup) GetDefault(id int) Data {
	return ProductGroup{
		ID:        id,
		IsActive:  utils.NewFalse(),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
}

func (po PurchaseOrder) GetId() int {
	return po.ID
}

func (po PurchaseOrder) GetDefault(id int) Data {
	return PurchaseOrder{
		ID:             id,
		OrderDate:      time.Now(),
		IsTaxInclusive: utils.NewFalse(),
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}
}

func (r Role) GetId() int {
	return r.ID
}

func (r Role) GetDefault(id int) Data {
	return Role{
		ID:        id,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
}

func (s SalesPerson) GetId() int {
	return s.ID
}

func (s SalesPerson) GetDefault(id int) Data {
	return SalesPerson{
		ID:        id,
		CreatedAt: time.Now(),
		IsActive:  utils.NewFalse(),
		UpdatedAt: time.Now(),
	}
}

func (i SalesInvoice) GetId() int {
	return i.ID
}

func (i SalesInvoice) GetDefault(id int) Data {
	return SalesInvoice{
		ID:                  id,
		InvoiceDate:         time.Now(),
		InvoicePaymentTerms: PaymentTermsCustom,
		CurrentStatus:       SalesInvoiceStatusDraft,
		IsTaxInclusive:      utils.NewFalse(),
		CreatedAt:           time.Now(),
		UpdatedAt:           time.Now(),
	}
}

func (po SalesOrder) GetDefault(id int) Data {
	return SalesOrder{
		ID:             id,
		OrderDate:      time.Now(),
		IsTaxInclusive: utils.NewFalse(),
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}
}

func (s ShippingAddress) GetId() int {
	return s.ReferenceID
}

func (s ShippingAddress) GetDefault(id int) Data {
	return ShippingAddress{
		ID:        id,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
}

func (s State) GetId() int {
	return s.ID
}

func (s State) GetDefault(id int) Data {
	return State{
		ID:       id,
		IsActive: utils.NewFalse(),
	}
}

func (s Supplier) GetId() int {
	return s.ID
}

func (s Supplier) GetDefault(id int) Data {
	return Supplier{
		ID:                   id,
		SupplierPaymentTerms: PaymentTermsDueOnReceipt,
		IsActive:             utils.NewFalse(),
		CreatedAt:            time.Now(),
		UpdatedAt:            time.Now(),
	}
}

func (s SupplierCreditDetail) GetId() int {
	return s.ID
}

func (s SupplierCreditDetail) GetDefault(id int) Data {
	return SupplierCreditDetail{
		ID: id,
	}
}

func (s SupplierPaidBill) GetId() int {
	return s.ID
}

func (s SupplierPaidBill) GetDefault(id int) Data {
	return SupplierPaidBill{
		ID:        id,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
}

func (t Tax) GetId() int {
	return t.ID
}

func (t Tax) GetDefault(id int) Data {
	return Tax{
		ID:            id,
		IsCompoundTax: utils.NewFalse(),
		IsActive:      utils.NewFalse(),
	}
}

func (t TaxGroup) GetId() int {
	return t.ID
}

func (t TaxGroup) GetDefault(id int) Data {
	return TaxGroup{
		ID:       id,
		IsActive: utils.NewFalse(),
	}
}

func (t Township) GetId() int {
	return t.ID
}

func (t Township) GetDefault(id int) Data {
	return Township{
		ID:       id,
		IsActive: utils.NewFalse(),
	}
}

func (w Warehouse) GetId() int {
	return w.ID
}

func (w Warehouse) GetDefault(id int) Data {
	return Warehouse{
		ID:        id,
		CreatedAt: time.Now(),
		IsActive:  utils.NewFalse(),
		UpdatedAt: time.Now(),
	}
}

// loader loading more than one model by one id
type RelatedData interface {
	GetReferenceId() int
}

func (p ProductVariant) GetReferenceId() int {
	return p.ProductGroupId
}

func (s ShippingAddress) GetReferenceId() int {
	return s.ReferenceID
}

func (b BillingAddress) GetReferenceId() int {
	return b.ReferenceID
}

func (c ContactPerson) GetReferenceId() int {
	return c.ReferenceID
}

func (d Document) GetReferenceId() int {
	return d.ReferenceID
}

func (b BillDetail) GetReferenceId() int {
	return b.BillId
}

func (c CreditNoteDetail) GetReferenceId() int {
	return c.CreditNoteId
}

func (i Image) GetReferenceId() int {
	return i.ReferenceID
}

func (i PaidInvoice) GetReferenceId() int {
	return i.CustomerPaymentId
}

func (spb SupplierPaidBill) GetReferenceId() int {
	return spb.SupplierPaymentId
}

func (d PurchaseOrderDetail) GetReferenceId() int {
	return d.PurchaseOrderId
}

func (d RecurringBillDetail) GetReferenceId() int {
	return d.RecurringBillId
}

func (d SalesInvoiceDetail) GetReferenceId() int {
	return d.SalesInvoiceId
}

func (d SalesOrderDetail) GetReferenceId() int {
	return d.SalesOrderId
}

func (d SupplierCreditDetail) GetReferenceId() int {
	return d.SupplierCreditId
}

func (b TransferOrderDetail) GetReferenceId() int {
	return b.TransferOrderId
}

func (b InventoryAdjustmentDetail) GetReferenceId() int {
	return b.InventoryAdjustmentId
}

func (b BankingTransactionDetail) GetReferenceId() int {
	return b.BankingTransactionId
}

func (b BankingTransaction) GetReferenceId() int {
	return b.ID
}

func (b SupplierCreditBill) GetReferenceId() int {
	return b.ReferenceId
}

func (b CustomerCreditInvoice) GetReferenceId() int {
	return b.ReferenceId
}
