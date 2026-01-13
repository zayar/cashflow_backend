package models

func (a Account) GetBusinessId() string {
	return a.BusinessId
}

func (a AccountCurrencyDailyBalance) GetBusinessId() string {
	return a.BusinessId
}

func (a AccountJournal) GetBusinessId() string {
	return a.BusinessId
}

func (a AccountTransaction) GetBusinessId() string {
	return a.BusinessId
}

func (b Bill) GetBusinessId() string {
	return b.BusinessId
}

func (b Branch) GetBusinessId() string {
	return b.BusinessId
}

func (c Comment) GetBusinessId() string {
	return c.BusinessId
}

func (c CreditNote) GetBusinessId() string {
	return c.BusinessId
}

func (c CustomerCreditAdvance) GetBusinessId() string {
	return c.BusinessId
}

func (c Currency) GetBusinessId() string {
	return c.BusinessId
}

func (c CurrencyExchange) GetBusinessId() string {
	return c.BusinessId
}

func (c Customer) GetBusinessId() string {
	return c.BusinessId
}

func (c CustomerPayment) GetBusinessId() string {
	return c.BusinessId
}

func (d DeliveryMethod) GetBusinessId() string {
	return d.BusinessId
}

func (d Expense) GetBusinessId() string {
	return d.BusinessId
}

func (h History) GetBusinessId() string {
	return h.BusinessId
}

func (j Journal) GetBusinessId() string {
	return j.BusinessId
}

func (m Module) GetBusinessId() string {
	return m.BusinessId
}

func (m MoneyAccount) GetBusinessId() string {
	return m.BusinessId
}

func (p PaymentMode) GetBusinessId() string {
	return p.BusinessId
}

func (p Product) GetBusinessId() string {
	return p.BusinessId
}

func (p ProductCategory) GetBusinessId() string {
	return p.BusinessId
}

func (p ProductGroup) GetBusinessId() string {
	return p.BusinessId
}

func (p ProductModifier) GetBusinessId() string {
	return p.BusinessId
}

func (p ProductUnit) GetBusinessId() string {
	return p.BusinessId
}

func (p ProductVariant) GetBusinessId() string {
	return p.BusinessId
}

func (a PubSubMessageRecord) GetBusinessId() string {
	return a.BusinessId
}

func (p PurchaseOrder) GetBusinessId() string {
	return p.BusinessId
}

func (r RecurringBill) GetBusinessId() string {
	return r.BusinessId
}

func (r Role) GetBusinessId() string {
	return r.BusinessId
}

func (r RoleModule) GetBusinessId() string {
	return r.BusinessId
}

func (s SalesInvoice) GetBusinessId() string {
	return s.BusinessId
}

func (s SalesOrder) GetBusinessId() string {
	return s.BusinessId
}

func (s SalesPerson) GetBusinessId() string {
	return s.BusinessId
}

// func (s Stock) GetBusinessId() string {
// 	return s.BusinessId
// }

func (s Supplier) GetBusinessId() string {
	return s.BusinessId
}

func (s SupplierCredit) GetBusinessId() string {
	return s.BusinessId
}

func (s SupplierCreditAdvance) GetBusinessId() string {
	return s.BusinessId
}

func (s SupplierPayment) GetBusinessId() string {
	return s.BusinessId
}

func (t Tax) GetBusinessId() string {
	return t.BusinessId
}

func (t TaxGroup) GetBusinessId() string {
	return t.BusinessId
}

func (t TransactionNumberSeries) GetBusinessId() string {
	return t.BusinessId
}

func (u User) GetBusinessId() string {
	return u.BusinessId
}

func (w Warehouse) GetBusinessId() string {
	return w.BusinessId
}
