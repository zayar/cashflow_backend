package models

import (
	"database/sql/driver"
	"errors"
	"fmt"
	"io"
	"strconv"
	"time"
)

type AccountMainType string

const (
	AccountMainTypeAsset     AccountMainType = "Asset"
	AccountMainTypeLiability AccountMainType = "Liability"
	AccountMainTypeEquity    AccountMainType = "Equity"
	AccountMainTypeIncome    AccountMainType = "Income"
	AccountMainTypeExpense   AccountMainType = "Expense"
)

// convert enum to send response
func (t AccountMainType) MarshalGQL(w io.Writer) {
	w.Write([]byte(strconv.Quote(string(t))))
}

// convert input to enum type
func (t *AccountMainType) UnmarshalGQL(i interface{}) error {
	str, ok := i.(string)
	if !ok {
		return errors.New("account main type must be string")
	}
	switch str {
	case "Asset":
		*t = AccountMainTypeAsset
	case "Liability":
		*t = AccountMainTypeLiability
	case "Equity":
		*t = AccountMainTypeEquity
	case "Income":
		*t = AccountMainTypeIncome
	case "Expense":
		*t = AccountMainTypeExpense
	default:
		return errors.New("invalid account main type")
	}
	return nil
}

type AccountDetailType string

const (
	AccountDetailTypeOtherAsset            AccountDetailType = "OtherAsset"
	AccountDetailTypeOtherCurrentAsset     AccountDetailType = "OtherCurrentAsset"
	AccountDetailTypeCash                  AccountDetailType = "Cash"
	AccountDetailTypeBank                  AccountDetailType = "Bank"
	AccountDetailTypeFixedAsset            AccountDetailType = "FixedAsset"
	AccountDetailTypeStock                 AccountDetailType = "Stock"
	AccountDetailTypePaymentClearing       AccountDetailType = "PaymentClearing"
	AccountDetailTypeInputTax              AccountDetailType = "InputTax"
	AccountDetailTypeOtherCurrentLiability AccountDetailType = "OtherCurrentLiability"
	AccountDetailTypeCreditCard            AccountDetailType = "CreditCard"
	AccountDetailTypeLongTermLiability     AccountDetailType = "LongTermLiability"
	AccountDetailTypeOtherLiability        AccountDetailType = "OtherLiability"
	AccountDetailTypeOverseasTaxPayable    AccountDetailType = "OverseasTaxPayable"
	AccountDetailTypeOutputTax             AccountDetailType = "OutputTax"
	AccountDetailTypeEquity                AccountDetailType = "Equity"
	AccountDetailTypeIncome                AccountDetailType = "Income"
	AccountDetailTypeOtherIncome           AccountDetailType = "OtherIncome"
	AccountDetailTypeExpense               AccountDetailType = "Expense"
	AccountDetailTypeCostOfGoodsSold       AccountDetailType = "CostOfGoodsSold"
	AccountDetailTypeOtherExpense          AccountDetailType = "OtherExpense"
	AccountDetailTypeAccountsReceivable    AccountDetailType = "AccountsReceivable"
	AccountDetailTypeAccountsPayable       AccountDetailType = "AccountsPayable"
)

func (t AccountDetailType) MarshalGQL(w io.Writer) {
	w.Write([]byte(strconv.Quote(string(t))))
}

func (t *AccountDetailType) UnmarshalGQL(i interface{}) error {
	str, ok := i.(string)
	if !ok {
		return errors.New("accountDetailType must be string")
	}
	accountDetailTypes := map[string]AccountDetailType{
		"OtherAsset":            AccountDetailTypeOtherAsset,
		"OtherCurrentAsset":     AccountDetailTypeOtherCurrentAsset,
		"Cash":                  AccountDetailTypeCash,
		"Bank":                  AccountDetailTypeBank,
		"FixedAsset":            AccountDetailTypeFixedAsset,
		"Stock":                 AccountDetailTypeStock,
		"PaymentClearing":       AccountDetailTypePaymentClearing,
		"InputTax":              AccountDetailTypeInputTax,
		"OtherCurrentLiability": AccountDetailTypeOtherCurrentLiability,
		"CreditCard":            AccountDetailTypeCreditCard,
		"LongTermLiability":     AccountDetailTypeLongTermLiability,
		"OtherLiability":        AccountDetailTypeOtherLiability,
		"OverseasTaxPayable":    AccountDetailTypeOverseasTaxPayable,
		"OutputTax":             AccountDetailTypeOutputTax,
		"Equity":                AccountDetailTypeEquity,
		"Income":                AccountDetailTypeIncome,
		"OtherIncome":           AccountDetailTypeOtherIncome,
		"Expense":               AccountDetailTypeExpense,
		"CostOfGoodsSold":       AccountDetailTypeCostOfGoodsSold,
		"OtherExpense":          AccountDetailTypeOtherExpense,
		"AccountsReceivable":    AccountDetailTypeAccountsReceivable,
		"AccountsPayable":       AccountDetailTypeAccountsPayable,
	}
	*t, ok = accountDetailTypes[str]
	if !ok {
		return errors.New("invalid accountDetailType")
	}
	return nil
}

// --- Accounting classification (reporting + cashflow) ---
// These are stored on Account to make reporting logic explicit and stable.

type NormalBalance string

const (
	NormalBalanceDebit  NormalBalance = "DEBIT"
	NormalBalanceCredit NormalBalance = "CREDIT"
)

func (t NormalBalance) MarshalGQL(w io.Writer) {
	w.Write([]byte(strconv.Quote(string(t))))
}

func (t *NormalBalance) UnmarshalGQL(i interface{}) error {
	str, ok := i.(string)
	if !ok {
		return errors.New("normalBalance must be string")
	}
	switch str {
	case "DEBIT":
		*t = NormalBalanceDebit
	case "CREDIT":
		*t = NormalBalanceCredit
	default:
		return errors.New("invalid normalBalance")
	}
	return nil
}

// AccountReportGroup is a stable grouping used by P&L/Balance Sheet reports.
// Keep values as strings for easier evolution and DB safety.
type AccountReportGroup string

const (
	AccountReportGroupCashAndCashEquivalents AccountReportGroup = "CASH_AND_CASH_EQUIVALENTS"
	AccountReportGroupAccountsReceivable     AccountReportGroup = "ACCOUNTS_RECEIVABLE"
	AccountReportGroupInventory              AccountReportGroup = "INVENTORY"
	AccountReportGroupOtherCurrentAsset      AccountReportGroup = "OTHER_CURRENT_ASSET"
	AccountReportGroupFixedAsset             AccountReportGroup = "FIXED_ASSET"
	AccountReportGroupAccountsPayable        AccountReportGroup = "ACCOUNTS_PAYABLE"
	AccountReportGroupOtherCurrentLiability  AccountReportGroup = "OTHER_CURRENT_LIABILITY"
	AccountReportGroupLongTermLiability      AccountReportGroup = "LONG_TERM_LIABILITY"
	AccountReportGroupEquity                 AccountReportGroup = "EQUITY"
	AccountReportGroupSalesRevenue           AccountReportGroup = "SALES_REVENUE"
	AccountReportGroupOtherIncome            AccountReportGroup = "OTHER_INCOME"
	AccountReportGroupCOGS                   AccountReportGroup = "COGS"
	AccountReportGroupOperatingExpense       AccountReportGroup = "OPERATING_EXPENSE"
	AccountReportGroupOtherExpense           AccountReportGroup = "OTHER_EXPENSE"
	AccountReportGroupTaxExpense             AccountReportGroup = "TAX_EXPENSE"
)

func (t AccountReportGroup) MarshalGQL(w io.Writer) {
	w.Write([]byte(strconv.Quote(string(t))))
}

func (t *AccountReportGroup) UnmarshalGQL(i interface{}) error {
	str, ok := i.(string)
	if !ok {
		return errors.New("accountReportGroup must be string")
	}
	// Allow blank (unset) for backwards compatibility.
	if str == "" {
		*t = ""
		return nil
	}
	switch AccountReportGroup(str) {
	case AccountReportGroupCashAndCashEquivalents,
		AccountReportGroupAccountsReceivable,
		AccountReportGroupInventory,
		AccountReportGroupOtherCurrentAsset,
		AccountReportGroupFixedAsset,
		AccountReportGroupAccountsPayable,
		AccountReportGroupOtherCurrentLiability,
		AccountReportGroupLongTermLiability,
		AccountReportGroupEquity,
		AccountReportGroupSalesRevenue,
		AccountReportGroupOtherIncome,
		AccountReportGroupCOGS,
		AccountReportGroupOperatingExpense,
		AccountReportGroupOtherExpense,
		AccountReportGroupTaxExpense:
		*t = AccountReportGroup(str)
		return nil
	default:
		return errors.New("invalid accountReportGroup")
	}
}

type CashflowActivity string

const (
	CashflowActivityOperating CashflowActivity = "OPERATING"
	CashflowActivityInvesting CashflowActivity = "INVESTING"
	CashflowActivityFinancing CashflowActivity = "FINANCING"
)

func (t CashflowActivity) MarshalGQL(w io.Writer) {
	w.Write([]byte(strconv.Quote(string(t))))
}

func (t *CashflowActivity) UnmarshalGQL(i interface{}) error {
	str, ok := i.(string)
	if !ok {
		return errors.New("cashflowActivity must be string")
	}
	// Allow blank (unset) for backwards compatibility.
	if str == "" {
		*t = ""
		return nil
	}
	switch str {
	case "OPERATING":
		*t = CashflowActivityOperating
	case "INVESTING":
		*t = CashflowActivityInvesting
	case "FINANCING":
		*t = CashflowActivityFinancing
	default:
		return errors.New("invalid cashflowActivity")
	}
	return nil
}

type AccountReferenceType string

const (
	AccountReferenceTypeJournal                      AccountReferenceType = "JN"
	AccountReferenceTypeInvoice                      AccountReferenceType = "IV"
	AccountReferenceTypeCustomerPayment              AccountReferenceType = "CP"
	AccountReferenceTypeCreditNote                   AccountReferenceType = "CN"
	AccountReferenceTypeCreditNoteApplied            AccountReferenceType = "CNA"
	AccountReferenceTypeCreditNoteRefund             AccountReferenceType = "CNR"
	AccountReferenceTypeCustomerAdvanceApplied       AccountReferenceType = "CAA"
	AccountReferenceTypeCustomerAdvanceRefund        AccountReferenceType = "CAR"
	AccountReferenceTypeExpense                      AccountReferenceType = "EP"
	AccountReferenceTypeExpenseRefund                AccountReferenceType = "ER"
	AccountReferenceTypeBill                         AccountReferenceType = "BL"
	AccountReferenceTypeSupplierPayment              AccountReferenceType = "SP"
	AccountReferenceTypeProductOpeningStock          AccountReferenceType = "POS"
	AccountReferenceTypeProductGroupOpeningStock     AccountReferenceType = "PGOS"
	AccountReferenceTypeProductCompositeOpeningStock AccountReferenceType = "PCOS"
	AccountReferenceTypeInventoryAdjustmentQuantity  AccountReferenceType = "IVAQ"
	AccountReferenceTypeInventoryAdjustmentValue     AccountReferenceType = "IVAV"
	AccountReferenceTypeInvoiceWriteOff              AccountReferenceType = "IWO"
	AccountReferenceTypeAdvanceCustomerPayment       AccountReferenceType = "ACP"
	AccountReferenceTypeAdvanceSupplierPayment       AccountReferenceType = "ASP"
	AccountReferenceTypeCustomerOpeningBalance       AccountReferenceType = "COB"
	AccountReferenceTypeSupplierOpeningBalance       AccountReferenceType = "SOB"
	AccountReferenceTypeOpeningBalance               AccountReferenceType = "OB"
	AccountReferenceTypeAccountTransfer              AccountReferenceType = "AC"
	AccountReferenceTypeAccountDeposit               AccountReferenceType = "AD"
	AccountReferenceTypeOwnerDrawing                 AccountReferenceType = "OD"
	AccountReferenceTypeOwnerContribution            AccountReferenceType = "OC"
	AccountReferenceTypeSupplierCredit               AccountReferenceType = "SC"
	AccountReferenceTypeSupplierCreditApplied        AccountReferenceType = "SCA"
	AccountReferenceTypeSupplierCreditRefund         AccountReferenceType = "SCR"
	AccountReferenceTypeSupplierAdvanceApplied       AccountReferenceType = "SAA"
	AccountReferenceTypeSupplierAdvanceRefund        AccountReferenceType = "SAR"
	AccountReferenceTypeOtherIncome                  AccountReferenceType = "OI"
	AccountReferenceTypeTransferOrder                AccountReferenceType = "TO"
	AccountReferenceTypePosInvoicePayment            AccountReferenceType = "POSIVP"
)

func (t AccountReferenceType) MarshalGQL(w io.Writer) {
	w.Write([]byte(strconv.Quote(string(t))))
}

func (t *AccountReferenceType) UnmarshalGQL(i interface{}) error {
	str, ok := i.(string)
	if !ok {
		return errors.New("account reference type must be string")
	}

	accountReferenceType := map[string]AccountReferenceType{
		"JN":     AccountReferenceTypeJournal,
		"IV":     AccountReferenceTypeInvoice,
		"CP":     AccountReferenceTypeCustomerPayment,
		"CN":     AccountReferenceTypeCreditNote,
		"CNA":    AccountReferenceTypeCreditNoteApplied,
		"CNR":    AccountReferenceTypeCreditNoteRefund,
		"CAA":    AccountReferenceTypeCustomerAdvanceApplied,
		"CAR":    AccountReferenceTypeCustomerAdvanceRefund,
		"EP":     AccountReferenceTypeExpense,
		"ER":     AccountReferenceTypeExpenseRefund,
		"BL":     AccountReferenceTypeBill,
		"SP":     AccountReferenceTypeSupplierPayment,
		"OD":     AccountReferenceTypeOwnerDrawing,
		"OC":     AccountReferenceTypeOwnerContribution,
		"POS":    AccountReferenceTypeProductOpeningStock,
		"PGOS":   AccountReferenceTypeProductGroupOpeningStock,
		"PCOS":   AccountReferenceTypeProductCompositeOpeningStock,
		"IVAQ":   AccountReferenceTypeInventoryAdjustmentQuantity,
		"IVAV":   AccountReferenceTypeInventoryAdjustmentValue,
		"IWO":    AccountReferenceTypeInvoiceWriteOff,
		"ACP":    AccountReferenceTypeAdvanceCustomerPayment,
		"ASP":    AccountReferenceTypeAdvanceSupplierPayment,
		"COB":    AccountReferenceTypeCustomerOpeningBalance,
		"SOB":    AccountReferenceTypeSupplierOpeningBalance,
		"OB":     AccountReferenceTypeOpeningBalance,
		"AC":     AccountReferenceTypeAccountTransfer,
		"AD":     AccountReferenceTypeAccountDeposit,
		"SC":     AccountReferenceTypeSupplierCredit,
		"SCA":    AccountReferenceTypeSupplierCreditApplied,
		"SCR":    AccountReferenceTypeSupplierCreditRefund,
		"SAA":    AccountReferenceTypeSupplierAdvanceApplied,
		"SAR":    AccountReferenceTypeSupplierAdvanceRefund,
		"OI":     AccountReferenceTypeOtherIncome,
		"TO":     AccountReferenceTypeTransferOrder,
		"POSIVP": AccountReferenceTypePosInvoicePayment,
	}

	*t, ok = accountReferenceType[str]
	if !ok {
		return errors.New("invalid account reference type")
	}

	return nil
}

type MoneyAccountType string

const (
	MoneyAccountTypeCash MoneyAccountType = "Cash"
	MoneyAccountTypeBank MoneyAccountType = "Bank"
	MoneyAccountTypeCard MoneyAccountType = "Card"
)

// convert enum to send response
func (t MoneyAccountType) MarshalGQL(w io.Writer) {
	w.Write([]byte(strconv.Quote(string(t))))
}

// convert input to enum type
func (t *MoneyAccountType) UnmarshalGQL(i interface{}) error {
	str, ok := i.(string)
	if !ok {
		return errors.New("money account type must be string")
	}
	switch str {
	case "Cash":
		*t = MoneyAccountTypeCash
	case "Bank":
		*t = MoneyAccountTypeBank
	case "Card":
		*t = MoneyAccountTypeCard
	default:
		return errors.New("invalid money account type")
	}
	return nil
}

type BillStatus string

const (
	BillStatusDraft       BillStatus = "Draft"
	BillStatusConfirmed   BillStatus = "Confirmed"
	BillStatusVoid        BillStatus = "Void"
	BillStatusPartialPaid BillStatus = "Partial Paid"
	BillStatusPaid        BillStatus = "Paid"
)

func (s BillStatus) MarshalGQL(w io.Writer) {
	w.Write([]byte(strconv.Quote(string(s))))
}

func (s *BillStatus) UnmarshalGQL(i interface{}) error {
	str, ok := i.(string)
	if !ok {
		return errors.New("bill status must be string")
	}

	billStatus := map[string]BillStatus{
		"Draft":        BillStatusDraft,
		"Confirmed":    BillStatusConfirmed,
		"Void":         BillStatusVoid,
		"Partial Paid": BillStatusPartialPaid,
		"Paid":         BillStatusPaid,
	}

	*s, ok = billStatus[str]
	if !ok {
		return errors.New("invalid bill status")
	}
	return nil
}

type CreditNoteStatus string

const (
	CreditNoteStatusDraft     CreditNoteStatus = "Draft"
	CreditNoteStatusConfirmed CreditNoteStatus = "Confirmed"
	CreditNoteStatusVoid      CreditNoteStatus = "Void"
	CreditNoteStatusClosed    CreditNoteStatus = "Closed"
)

// convert enum to send response
func (s CreditNoteStatus) MarshalGQL(w io.Writer) {
	w.Write([]byte(strconv.Quote(string(s))))
}

// convert input to enum type
func (s *CreditNoteStatus) UnmarshalGQL(i interface{}) error {
	str, ok := i.(string)
	if !ok {
		return errors.New("credit note status must be string")
	}
	switch str {
	case "Draft":
		*s = CreditNoteStatusDraft
	case "Confirmed":
		*s = CreditNoteStatusConfirmed
	case "Void":
		*s = CreditNoteStatusVoid
	case "Closed":
		*s = CreditNoteStatusClosed
	default:
		return errors.New("invalid credit note status")
	}
	return nil
}

type DecimalPlaces string

const (
	DecimalPlacesZero  DecimalPlaces = "0"
	DecimalPlacesTwo   DecimalPlaces = "2"
	DecimalPlacesThree DecimalPlaces = "3"
)

func (p DecimalPlaces) MarshalGQL(w io.Writer) {
	w.Write([]byte(strconv.Quote(string(p))))
}

func (p *DecimalPlaces) UnmarshalGQL(i interface{}) error {
	str, ok := i.(string)
	if !ok {
		return errors.New("decialPlaces must be string")
	}
	switch str {
	case "0":
		*p = DecimalPlacesZero
	case "2":
		*p = DecimalPlacesTwo
	case "3":
		*p = DecimalPlacesThree
	default:
		return errors.New("invalid decialPlaces")
	}
	return nil
}

type DiscountType string

const (
	DiscountTypePercent DiscountType = "P"
	DiscountTypeAmount  DiscountType = "A"
)

func (t DiscountType) MarshalGQL(w io.Writer) {
	w.Write([]byte(strconv.Quote(string(t))))
}

func (t *DiscountType) UnmarshalGQL(i interface{}) error {
	str, ok := i.(string)
	if !ok {
		return errors.New("discount type must be string")
	}
	switch str {
	case "P":
		*t = DiscountTypePercent
	case "A":
		*t = DiscountTypeAmount
	default:
		return errors.New("invalid discount type")
	}
	return nil
}

type FiscalYear string

const (
	FiscalYearJan FiscalYear = "Jan"
	FiscalYearFeb FiscalYear = "Feb"
	FiscalYearMar FiscalYear = "Mar"
	FiscalYearApr FiscalYear = "Apr"
	FiscalYearMay FiscalYear = "May"
	FiscalYearJun FiscalYear = "Jun"
	FiscalYearJul FiscalYear = "Jul"
	FiscalYearAug FiscalYear = "Aug"
	FiscalYearSep FiscalYear = "Sep"
	FiscalYearOct FiscalYear = "Oct"
	FiscalYearNov FiscalYear = "Nov"
	FiscalYearDec FiscalYear = "Dec"
)

func (y FiscalYear) MarshalGQL(w io.Writer) {
	w.Write([]byte(strconv.Quote(string(y))))
}

func (y *FiscalYear) UnmarshalGQL(i interface{}) error {
	str, ok := i.(string)
	if !ok {
		return errors.New("FiscalYear must be string")
	}

	fiscalYears := map[string]FiscalYear{
		"Jan": FiscalYearJan,
		"Feb": FiscalYearFeb,
		"Mar": FiscalYearMar,
		"Apr": FiscalYearApr,
		"May": FiscalYearMay,
		"Jun": FiscalYearJun,
		"Jul": FiscalYearJul,
		"Aug": FiscalYearAug,
		"Sep": FiscalYearSep,
		"Oct": FiscalYearOct,
		"Nov": FiscalYearNov,
		"Dec": FiscalYearDec,
	}

	*y, ok = fiscalYears[str]
	if !ok {
		return errors.New("invalid FiscalYear")
	}
	return nil
}

type JournalStatus string

const (
	JournalStatusDraft     JournalStatus = "Draft"
	JournalStatusPublished JournalStatus = "Published"
)

// convert enum to send response
func (s JournalStatus) MarshalGQL(w io.Writer) {
	w.Write([]byte(strconv.Quote(string(s))))
}

// convert input to enum type
func (s *JournalStatus) UnmarshalGQL(i interface{}) error {
	str, ok := i.(string)
	if !ok {
		return errors.New("journal status must be string")
	}
	switch str {
	case "Draft":
		*s = JournalStatusDraft
	case "Published":
		*s = JournalStatusPublished
	default:
		return errors.New("invalid journal status")
	}
	return nil
}

type JournalTransactionType string

const (
	JournalTransactionTypeNA        JournalTransactionType = "NA"
	JournalTransactionTypeSales     JournalTransactionType = "Sales"
	JournalTransactionTypePurchases JournalTransactionType = "Purchases"
)

// convert enum to send response
func (t JournalTransactionType) MarshalGQL(w io.Writer) {
	w.Write([]byte(strconv.Quote(string(t))))
}

// convert input to enum type
func (t *JournalTransactionType) UnmarshalGQL(i interface{}) error {
	str, ok := i.(string)
	if !ok {
		return errors.New("journal transaction type must be string")
	}
	switch str {
	case "NA":
		*t = JournalTransactionTypeNA
	case "Sales":
		*t = JournalTransactionTypeSales
	case "Purchases":
		*t = JournalTransactionTypePurchases
	default:
		return errors.New("invalid journal transaction type")
	}
	return nil
}

type PaymentTerms string

const (
	PaymentTermsNet15             PaymentTerms = "Net15"
	PaymentTermsNet30             PaymentTerms = "Net30"
	PaymentTermsNet45             PaymentTerms = "Net45"
	PaymentTermsNet60             PaymentTerms = "Net60"
	PaymentTermsDueEndOfMonth     PaymentTerms = "DueMonthEnd"
	PaymentTermsDueEndOfNextMonth PaymentTerms = "DueNextMonthEnd"
	PaymentTermsDueOnReceipt      PaymentTerms = "DueOnReceipt"
	PaymentTermsCustom            PaymentTerms = "Custom"
)

func (p PaymentTerms) MarshalGQL(w io.Writer) {
	w.Write([]byte(strconv.Quote(string(p))))
}

func (p *PaymentTerms) UnmarshalGQL(i interface{}) error {
	str, ok := i.(string)
	if !ok {
		return errors.New("paymentTerms must be string")
	}

	paymentTerms := map[string]PaymentTerms{
		"Net15":           PaymentTermsNet15,
		"Net30":           PaymentTermsNet30,
		"Net45":           PaymentTermsNet45,
		"Net60":           PaymentTermsNet60,
		"DueMonthEnd":     PaymentTermsDueEndOfMonth,
		"DueNextMonthEnd": PaymentTermsDueEndOfNextMonth,
		"DueOnReceipt":    PaymentTermsDueOnReceipt,
		"Custom":          PaymentTermsCustom,
	}

	*p, ok = paymentTerms[str]
	if !ok {
		return errors.New("invalid paymentTerms")
	}

	return nil
}

type Precision string

const (
	PrecisionZero  Precision = "0"
	PrecisionOne   Precision = "1"
	PrecisionTwo   Precision = "2"
	PrecisionThree Precision = "3"
	PrecisionFour  Precision = "4"
)

func (p Precision) MarshalGQL(w io.Writer) {
	w.Write([]byte(strconv.Quote(string(p))))
}

func (p *Precision) UnmarshalGQL(i interface{}) error {
	str, ok := i.(string)
	if !ok {
		return errors.New("precision must be string")
	}

	switch str {
	case "0":
		*p = PrecisionZero
	case "1":
		*p = PrecisionOne
	case "2":
		*p = PrecisionTwo
	case "3":
		*p = PrecisionThree
	case "4":
		*p = PrecisionFour
	default:
		return errors.New("invalid precision")
	}
	return nil
}

type ProductNature string

const (
	ProductNatureGoods   ProductNature = "G"
	ProductNatureService ProductNature = "S"
)

func (t ProductNature) MarshalGQL(w io.Writer) {
	w.Write([]byte(strconv.Quote(string(t))))
}

func (t *ProductNature) UnmarshalGQL(i interface{}) error {
	str, ok := i.(string)
	if !ok {
		return errors.New("product nature must be string")
	}
	switch str {
	case "G":
		*t = ProductNatureGoods
	case "S":
		*t = ProductNatureService
	default:
		return errors.New("invalid product nature")
	}
	return nil
}

type ProductType string

const (
	ProductTypeSingle    ProductType = "S"
	ProductTypeGroup     ProductType = "G"
	ProductTypeComposite ProductType = "C"
	ProductTypeVariant   ProductType = "V"
	ProductTypeInput     ProductType = "I"
)

func (t ProductType) MarshalGQL(w io.Writer) {
	w.Write([]byte(strconv.Quote(string(t))))
}

func (t *ProductType) UnmarshalGQL(i interface{}) error {
	str, ok := i.(string)
	if !ok {
		return errors.New("product type must be string")
	}
	switch str {
	case "S":
		*t = ProductTypeSingle
	case "G":
		*t = ProductTypeGroup
	case "C":
		*t = ProductTypeComposite
	case "V":
		*t = ProductTypeVariant
	case "I":
		*t = ProductTypeInput
	default:
		return errors.New("invalid product type")
	}
	return nil
}

type PubSubMessageAction string

const (
	PubSubMessageActionCreate PubSubMessageAction = "C"
	PubSubMessageActionUpdate PubSubMessageAction = "U"
	PubSubMessageActionDelete PubSubMessageAction = "D"
)

// convert enum to send response
func (t PubSubMessageAction) MarshalGQL(w io.Writer) {
	w.Write([]byte(strconv.Quote(string(t))))
}

// convert input to enum type
func (t *PubSubMessageAction) UnmarshalGQL(i interface{}) error {
	str, ok := i.(string)
	if !ok {
		return errors.New("pub sub message action must be string")
	}
	switch str {
	case "C":
		*t = PubSubMessageActionCreate
	case "U":
		*t = PubSubMessageActionUpdate
	case "D":
		*t = PubSubMessageActionDelete
	default:
		return errors.New("invalid pub sub message action")
	}
	return nil
}

type PurchaseOrderStatus string

const (
	PurchaseOrderStatusDraft           PurchaseOrderStatus = "Draft"
	PurchaseOrderStatusConfirmed       PurchaseOrderStatus = "Confirmed"
	PurchaseOrderStatusPartiallyBilled PurchaseOrderStatus = "Partially Billed"
	PurchaseOrderStatusClosed          PurchaseOrderStatus = "Closed"
	PurchaseOrderStatusCancelled       PurchaseOrderStatus = "Cancelled"
)

func (s PurchaseOrderStatus) MarshalGQL(w io.Writer) {
	w.Write([]byte(strconv.Quote(string(s))))
}

func (s *PurchaseOrderStatus) UnmarshalGQL(i interface{}) error {
	str, ok := i.(string)
	if !ok {
		return errors.New("purchase order status must be string")
	}

	purchaseOrderStatus := map[string]PurchaseOrderStatus{
		"Draft":            PurchaseOrderStatusDraft,
		"Confirmed":        PurchaseOrderStatusConfirmed,
		"Partially Billed": PurchaseOrderStatusPartiallyBilled,
		"Closed":           PurchaseOrderStatusClosed,
		"Cancelled":        PurchaseOrderStatusCancelled,
	}

	*s, ok = purchaseOrderStatus[str]
	if !ok {
		return errors.New("invalid purchase order status")
	}
	return nil
}

type RecurringTerms string

const (
	RecurringTermsDay   RecurringTerms = "D"
	RecurringTermsWeek  RecurringTerms = "W"
	RecurringTermsMonth RecurringTerms = "M"
	RecurringTermsYear  RecurringTerms = "Y"
)

func (p RecurringTerms) MarshalGQL(w io.Writer) {
	w.Write([]byte(strconv.Quote(string(p))))
}

func (p *RecurringTerms) UnmarshalGQL(i interface{}) error {
	str, ok := i.(string)
	if !ok {
		return errors.New("recurringTerms must be string")
	}

	recurringTerms := map[string]RecurringTerms{
		"D": RecurringTermsDay,
		"W": RecurringTermsWeek,
		"M": RecurringTermsMonth,
		"Y": RecurringTermsYear,
	}

	*p, ok = recurringTerms[str]
	if !ok {
		return errors.New("invalid recurringTerms")
	}
	return nil
}

type ReportBasis string

const (
	ReportBasisAccrual ReportBasis = "Accrual"
	ReportBasisCash    ReportBasis = "Cash"
)

func (t ReportBasis) MarshalGQL(w io.Writer) {
	w.Write([]byte(strconv.Quote(string(t))))
}

func (t *ReportBasis) UnmarshalGQL(i interface{}) error {
	str, ok := i.(string)
	if !ok {
		return errors.New("report basis must be string")
	}
	switch str {
	case "Accrual":
		*t = ReportBasisAccrual
	case "Cash":
		*t = ReportBasisCash
	default:
		return errors.New("invalid report basis")
	}
	return nil
}

type SalesInvoiceStatus string

const (
	SalesInvoiceStatusDraft       SalesInvoiceStatus = "Draft"
	SalesInvoiceStatusConfirmed   SalesInvoiceStatus = "Confirmed"
	SalesInvoiceStatusVoid        SalesInvoiceStatus = "Void"
	SalesInvoiceStatusPartialPaid SalesInvoiceStatus = "Partial Paid"
	SalesInvoiceStatusPaid        SalesInvoiceStatus = "Paid"
	SalesInvoiceStatusWriteOff    SalesInvoiceStatus = "Write Off"
)

func (s SalesInvoiceStatus) MarshalGQL(w io.Writer) {
	w.Write([]byte(strconv.Quote(string(s))))
}

func (s *SalesInvoiceStatus) UnmarshalGQL(i interface{}) error {
	str, ok := i.(string)
	if !ok {
		return errors.New("sales invoice status must be string")
	}

	salesInvoiceStatus := map[string]SalesInvoiceStatus{
		"Draft":        SalesInvoiceStatusDraft,
		"Confirmed":    SalesInvoiceStatusConfirmed,
		"Void":         SalesInvoiceStatusVoid,
		"Partial Paid": SalesInvoiceStatusPartialPaid,
		"Paid":         SalesInvoiceStatusPaid,
		"Write Off":    SalesInvoiceStatusWriteOff,
	}

	*s, ok = salesInvoiceStatus[str]
	if !ok {
		return errors.New("invalid sales invoice status")
	}
	return nil
}

type SalesOrderStatus string

const (
	SalesOrderStatusDraft             SalesOrderStatus = "Draft"
	SalesOrderStatusConfirmed         SalesOrderStatus = "Confirmed"
	SalesOrderStatusPartiallyInvoiced SalesOrderStatus = "Partially Invoiced"
	SalesOrderStatusClosed            SalesOrderStatus = "Closed"
	SalesOrderStatusCancelled         SalesOrderStatus = "Cancelled"
)

func (s SalesOrderStatus) MarshalGQL(w io.Writer) {
	w.Write([]byte(strconv.Quote(string(s))))
}

func (s *SalesOrderStatus) UnmarshalGQL(i interface{}) error {
	str, ok := i.(string)
	if !ok {
		return errors.New("sales order status must be string")
	}

	salesOrderStatus := map[string]SalesOrderStatus{
		"Draft":              SalesOrderStatusDraft,
		"Confirmed":          SalesOrderStatusConfirmed,
		"Partially Invoiced": SalesOrderStatusPartiallyInvoiced,
		"Closed":             SalesOrderStatusClosed,
		"Cancelled":          SalesOrderStatusCancelled,
	}

	*s, ok = salesOrderStatus[str]
	if !ok {
		return errors.New("invalid sales order status")
	}
	return nil
}

type StockReferenceType string

const (
	StockReferenceTypeBill                         StockReferenceType = "BL"
	StockReferenceTypeInvoice                      StockReferenceType = "IV"
	StockReferenceTypeCreditNote                   StockReferenceType = "CN"
	StockReferenceTypeSupplierCredit               StockReferenceType = "SC"
	StockReferenceTypeProductOpeningStock          StockReferenceType = "POS"
	StockReferenceTypeProductGroupOpeningStock     StockReferenceType = "PGOS"
	StockReferenceTypeProductCompositeOpeningStock StockReferenceType = "PCOS"
	StockReferenceTypeInventoryAdjustmentQuantity  StockReferenceType = "IVAQ"
	StockReferenceTypeInventoryAdjustmentValue     StockReferenceType = "IVAV"
	StockReferenceTypeTransferOrder                StockReferenceType = "TO"
)

func (t StockReferenceType) MarshalGQL(w io.Writer) {
	w.Write([]byte(strconv.Quote(string(t))))
}

func (t *StockReferenceType) UnmarshalGQL(i interface{}) error {
	str, ok := i.(string)
	if !ok {
		return errors.New("stock reference type must be string")
	}

	stockReferenceType := map[string]StockReferenceType{
		"IV":   StockReferenceTypeInvoice,
		"IVAQ": StockReferenceTypeInventoryAdjustmentQuantity,
		"IVAV": StockReferenceTypeInventoryAdjustmentValue,
		"BL":   StockReferenceTypeBill,
		"CN":   StockReferenceTypeCreditNote,
		"SC":   StockReferenceTypeSupplierCredit,
		"POS":  StockReferenceTypeProductOpeningStock,
		"PGOS": StockReferenceTypeProductGroupOpeningStock,
		"PCOS": StockReferenceTypeProductCompositeOpeningStock,
		"TO":   StockReferenceTypeTransferOrder,
	}

	*t, ok = stockReferenceType[str]
	if !ok {
		return errors.New("invalid stock reference type")
	}

	return nil
}

type SupplierCreditStatus string

const (
	SupplierCreditStatusDraft     SupplierCreditStatus = "Draft"
	SupplierCreditStatusConfirmed SupplierCreditStatus = "Confirmed"
	SupplierCreditStatusVoid      SupplierCreditStatus = "Void"
	SupplierCreditStatusClosed    SupplierCreditStatus = "Closed"
)

// convert enum to send response
func (s SupplierCreditStatus) MarshalGQL(w io.Writer) {
	w.Write([]byte(strconv.Quote(string(s))))
}

// convert input to enum type
func (s *SupplierCreditStatus) UnmarshalGQL(i interface{}) error {
	str, ok := i.(string)
	if !ok {
		return errors.New("supplier credit status must be string")
	}
	switch str {
	case "Draft":
		*s = SupplierCreditStatusDraft
	case "Confirmed":
		*s = SupplierCreditStatusConfirmed
	case "Void":
		*s = SupplierCreditStatusVoid
	case "Closed":
		*s = SupplierCreditStatusClosed
	default:
		return errors.New("invalid supplier credit status")
	}
	return nil
}

type SupplierAdvanceStatus string

const (
	SupplierAdvanceStatusDraft     SupplierAdvanceStatus = "Draft"
	SupplierAdvanceStatusConfirmed SupplierAdvanceStatus = "Confirmed"
	SupplierAdvanceStatusClosed    SupplierAdvanceStatus = "Closed"
)

// convert enum to send response
func (s SupplierAdvanceStatus) MarshalGQL(w io.Writer) {
	w.Write([]byte(strconv.Quote(string(s))))
}

// convert input to enum type
func (s *SupplierAdvanceStatus) UnmarshalGQL(i interface{}) error {
	str, ok := i.(string)
	if !ok {
		return errors.New("supplier advance status must be string")
	}
	switch str {
	case "Draft":
		*s = SupplierAdvanceStatusDraft
	case "Confirmed":
		*s = SupplierAdvanceStatusConfirmed
	case "Closed":
		*s = SupplierAdvanceStatusClosed
	default:
		return errors.New("invalid supplier advance status")
	}
	return nil
}

type CustomerAdvanceStatus string

const (
	CustomerAdvanceStatusDraft     CustomerAdvanceStatus = "Draft"
	CustomerAdvanceStatusConfirmed CustomerAdvanceStatus = "Confirmed"
	CustomerAdvanceStatusClosed    CustomerAdvanceStatus = "Closed"
)

// convert enum to send response
func (s CustomerAdvanceStatus) MarshalGQL(w io.Writer) {
	w.Write([]byte(strconv.Quote(string(s))))
}

// convert input to enum type
func (s *CustomerAdvanceStatus) UnmarshalGQL(i interface{}) error {
	str, ok := i.(string)
	if !ok {
		return errors.New("customer advance status must be string")
	}
	switch str {
	case "Draft":
		*s = CustomerAdvanceStatusDraft
	case "Confirmed":
		*s = CustomerAdvanceStatusConfirmed
	case "Closed":
		*s = CustomerAdvanceStatusClosed
	default:
		return errors.New("invalid customer advance status")
	}
	return nil
}

type TaxType string

const (
	TaxTypeIndividual TaxType = "I"
	TaxTypeGroup      TaxType = "G"
)

func (t TaxType) MarshalGQL(w io.Writer) {
	w.Write([]byte(strconv.Quote(string(t))))
}

func (t *TaxType) UnmarshalGQL(i interface{}) error {
	str, ok := i.(string)
	if !ok {
		return errors.New("tax type must be string")
	}
	switch str {
	case "I":
		*t = TaxTypeIndividual
	case "G":
		*t = TaxTypeGroup
	default:
		return errors.New("invalid tax type")
	}
	return nil
}

type UserRole string

const (
	UserRoleAdmin  UserRole = "A"
	UserRoleOwner  UserRole = "O"
	UserRoleCustom UserRole = "C"
)

func (p UserRole) MarshalGQL(w io.Writer) {
	w.Write([]byte(strconv.Quote(string(p))))
}

func (p *UserRole) UnmarshalGQL(i interface{}) error {
	str, ok := i.(string)
	if !ok {
		return errors.New("user role must be string")
	}

	userRole := map[string]UserRole{
		"A": UserRoleAdmin,
		"O": UserRoleOwner,
		"C": UserRoleCustom,
	}

	*p, ok = userRole[str]
	if !ok {
		return errors.New("invalid user role")
	}
	return nil
}

type TransferOrderStatus string

const (
	TransferOrderStatusDraft     TransferOrderStatus = "Draft"
	TransferOrderStatusConfirmed TransferOrderStatus = "Confirmed"
	TransferOrderStatusClosed    TransferOrderStatus = "Closed"
)

func (s TransferOrderStatus) MarshalGQL(w io.Writer) {
	w.Write([]byte(strconv.Quote(string(s))))
}

func (s *TransferOrderStatus) UnmarshalGQL(i interface{}) error {
	str, ok := i.(string)
	if !ok {
		return errors.New("transfer order status must be string")
	}

	transferOrderStatus := map[string]TransferOrderStatus{
		"Draft":     TransferOrderStatusDraft,
		"Confirmed": TransferOrderStatusConfirmed,
		"Closed":    TransferOrderStatusClosed,
	}

	*s, ok = transferOrderStatus[str]
	if !ok {
		return errors.New("invalid transfer order status")
	}
	return nil
}

type InventoryAdjustmentStatus string

const (
	InventoryAdjustmentStatusDraft    InventoryAdjustmentStatus = "Draft"
	InventoryAdjustmentStatusAdjusted InventoryAdjustmentStatus = "Adjusted"
)

func (s InventoryAdjustmentStatus) MarshalGQL(w io.Writer) {
	w.Write([]byte(strconv.Quote(string(s))))
}

func (s *InventoryAdjustmentStatus) UnmarshalGQL(i interface{}) error {
	str, ok := i.(string)
	if !ok {
		return errors.New("inventory adjustment status must be string")
	}

	inventoryAdjustmentStatus := map[string]InventoryAdjustmentStatus{
		"Draft":    InventoryAdjustmentStatusDraft,
		"Adjusted": InventoryAdjustmentStatusAdjusted,
	}

	*s, ok = inventoryAdjustmentStatus[str]
	if !ok {
		return errors.New("invalid inventory adjustment status")
	}
	return nil
}

type InventoryAdjustmentType string

const (
	InventoryAdjustmentTypeQuantity InventoryAdjustmentType = "Quantity"
	InventoryAdjustmentTypeValue    InventoryAdjustmentType = "Value"
)

func (s InventoryAdjustmentType) MarshalGQL(w io.Writer) {
	w.Write([]byte(strconv.Quote(string(s))))
}

func (s *InventoryAdjustmentType) UnmarshalGQL(i interface{}) error {
	str, ok := i.(string)
	if !ok {
		return errors.New("inventory adjustment type must be string")
	}

	inventoryAdjustmentType := map[string]InventoryAdjustmentType{
		"Quantity": InventoryAdjustmentTypeQuantity,
		"Value":    InventoryAdjustmentTypeValue,
	}

	*s, ok = inventoryAdjustmentType[str]
	if !ok {
		return errors.New("invalid inventory adjustment type")
	}
	return nil
}

type BankingTransactionType string

const (
	BankingTransactionTypeExpense                     BankingTransactionType = "Expense"
	BankingTransactionTypeSupplierAdvance             BankingTransactionType = "SupplierAdvance"
	BankingTransactionTypeSupplierPayment             BankingTransactionType = "SupplierPayment"
	BankingTransactionTypeTransferToAnotherAccount    BankingTransactionType = "TransferToAnotherAccount"
	BankingTransactionTypeSalesReturn                 BankingTransactionType = "SalesReturn"
	BankingTransactionTypeCardPayment                 BankingTransactionType = "CardPayment"
	BankingTransactionTypeOwnerDrawings               BankingTransactionType = "OwnerDrawings"
	BankingTransactionTypeDepositToOtherAccounts      BankingTransactionType = "DepositToOtherAccounts"
	BankingTransactionTypeCreditNoteRefund            BankingTransactionType = "CreditNoteRefund"
	BankingTransactionTypeCustomerAdvanceRefund       BankingTransactionType = "CustomerAdvanceRefund"
	BankingTransactionTypeEmployeeReimbursement       BankingTransactionType = "EmployeeReimbursement"
	BankingTransactionTypeCustomerAdvance             BankingTransactionType = "CustomerAdvance"
	BankingTransactionTypeCustomerPayment             BankingTransactionType = "CustomerPayment"
	BankingTransactionTypeSalesWithoutInvoices        BankingTransactionType = "SalesWithoutInvoices"
	BankingTransactionTypeTransferFromAnotherAccounts BankingTransactionType = "TransferFromAnotherAccounts"
	BankingTransactionTypeInterestIncome              BankingTransactionType = "InterestIncome"
	BankingTransactionTypeOtherIncome                 BankingTransactionType = "OtherIncome"
	BankingTransactionTypeExpenseRefund               BankingTransactionType = "ExpenseRefund"
	BankingTransactionTypeDepositFromOtherAccounts    BankingTransactionType = "DepositFromOtherAccounts"
	BankingTransactionTypeOwnerContribution           BankingTransactionType = "OwnerContribution"
	BankingTransactionTypeSupplierCreditRefund        BankingTransactionType = "SupplierCreditRefund"
	BankingTransactionTypeSupplierAdvanceRefund       BankingTransactionType = "SupplierAdvanceRefund"
	BankingTransactionTypeManualJournal               BankingTransactionType = "ManualJournal"
	BankingTransactionTypeOpeningBalance              BankingTransactionType = "OpeningBalance"
)

func (t BankingTransactionType) MarshalGQL(w io.Writer) {
	w.Write([]byte(strconv.Quote(string(t))))
}

func (t *BankingTransactionType) UnmarshalGQL(i interface{}) error {
	str, ok := i.(string)
	if !ok {
		return errors.New("banking transaction type must be string")
	}
	bankingTransactionTypes := map[string]BankingTransactionType{
		"Expense":                     BankingTransactionTypeExpense,
		"SupplierAdvance":             BankingTransactionTypeSupplierAdvance,
		"SupplierPayment":             BankingTransactionTypeSupplierPayment,
		"TransferToAnotherAccount":    BankingTransactionTypeTransferToAnotherAccount,
		"SalesReturn":                 BankingTransactionTypeSalesReturn,
		"CardPayment":                 BankingTransactionTypeCardPayment,
		"OwnerDrawings":               BankingTransactionTypeOwnerDrawings,
		"DepositToOtherAccounts":      BankingTransactionTypeDepositToOtherAccounts,
		"CreditNoteRefund":            BankingTransactionTypeCreditNoteRefund,
		"CustomerAdvanceRefund":       BankingTransactionTypeCustomerAdvanceRefund,
		"EmployeeReimbursement":       BankingTransactionTypeEmployeeReimbursement,
		"CustomerAdvance":             BankingTransactionTypeCustomerAdvance,
		"CustomerPayment":             BankingTransactionTypeCustomerPayment,
		"SalesWithoutInvoices":        BankingTransactionTypeSalesWithoutInvoices,
		"TransferFromAnotherAccounts": BankingTransactionTypeTransferFromAnotherAccounts,
		"InterestIncome":              BankingTransactionTypeInterestIncome,
		"OtherIncome":                 BankingTransactionTypeOtherIncome,
		"ExpenseRefund":               BankingTransactionTypeExpenseRefund,
		"DepositFromOtherAccounts":    BankingTransactionTypeDepositFromOtherAccounts,
		"OwnerContribution":           BankingTransactionTypeOwnerContribution,
		"SupplierCreditRefund":        BankingTransactionTypeSupplierCreditRefund,
		"SupplierAdvanceRefund":       BankingTransactionTypeSupplierAdvanceRefund,
		"ManualJournal":               BankingTransactionTypeManualJournal,
		"OpeningBalance":              BankingTransactionTypeOpeningBalance,
	}
	*t, ok = bankingTransactionTypes[str]
	if !ok {
		return errors.New("invalid banking transaction")
	}
	return nil
}

type SupplierCreditApplyType string

const (
	SupplierCreditApplyTypeCredit  SupplierCreditApplyType = "Credit"
	SupplierCreditApplyTypeAdvance SupplierCreditApplyType = "Advance"
)

func (p SupplierCreditApplyType) MarshalGQL(w io.Writer) {
	w.Write([]byte(strconv.Quote(string(p))))
}

func (p *SupplierCreditApplyType) UnmarshalGQL(i interface{}) error {
	str, ok := i.(string)
	if !ok {
		return errors.New("supplier credit apply type must be string")
	}

	supplierCreditApplyType := map[string]SupplierCreditApplyType{
		"Credit":  SupplierCreditApplyTypeCredit,
		"Advance": SupplierCreditApplyTypeAdvance,
	}

	*p, ok = supplierCreditApplyType[str]
	if !ok {
		return errors.New("invalid supplier credit applyType")
	}

	return nil
}

type CustomerCreditApplyType string

const (
	CustomerCreditApplyTypeCredit  CustomerCreditApplyType = "Credit"
	CustomerCreditApplyTypeAdvance CustomerCreditApplyType = "Advance"
)

func (p CustomerCreditApplyType) MarshalGQL(w io.Writer) {
	w.Write([]byte(strconv.Quote(string(p))))
}

func (p *CustomerCreditApplyType) UnmarshalGQL(i interface{}) error {
	str, ok := i.(string)
	if !ok {
		return errors.New("customer credit apply type must be string")
	}

	customerCreditApplyType := map[string]CustomerCreditApplyType{
		"Credit":  CustomerCreditApplyTypeCredit,
		"Advance": CustomerCreditApplyTypeAdvance,
	}

	*p, ok = customerCreditApplyType[str]
	if !ok {
		return errors.New("invalid customer Credit applyType")
	}

	return nil
}

type RefundReferenceType string

const (
	RefundReferenceTypeCreditNote      RefundReferenceType = "CN"
	RefundReferenceTypeSupplierCredit  RefundReferenceType = "SC"
	RefundReferenceTypeCustomerAdvance RefundReferenceType = "CA"
	RefundReferenceTypeSupplierAdvance RefundReferenceType = "SA"
	RefundReferenceTypeExpense         RefundReferenceType = "E"
)

func (t RefundReferenceType) MarshalGQL(w io.Writer) {
	w.Write([]byte(strconv.Quote(string(t))))
}

func (t *RefundReferenceType) UnmarshalGQL(i interface{}) error {
	str, ok := i.(string)
	if !ok {
		return errors.New("refund reference type must be string")
	}

	refundReferenceType := map[string]RefundReferenceType{
		"CN": RefundReferenceTypeCreditNote,
		"SC": RefundReferenceTypeSupplierCredit,
		"CA": RefundReferenceTypeCustomerAdvance,
		"SA": RefundReferenceTypeSupplierAdvance,
		"E":  RefundReferenceTypeExpense,
	}

	*t, ok = refundReferenceType[str]
	if !ok {
		return errors.New("invalid refund reference type")
	}

	return nil
}

type MyDateString time.Time

func (t MyDateString) MarshalGQL(w io.Writer) {
	w.Write([]byte(strconv.Quote(time.Time(t).String())))
}

// Parse the string into time.Time object
func (t *MyDateString) UnmarshalGQL(i interface{}) error {
	str, ok := i.(string)
	if !ok {
		return errors.New("MyDateString must be string")
	}

	// Parse the date string into a time.Time object
	localTime, err := time.Parse("2006-01-02T15:04:05", str)
	if err != nil {
		return errors.New("error parsing datetime")
	}
	*t = MyDateString(localTime)

	return nil
}

func (t *MyDateString) StartOfDayUTCTime(timezone string) error {
	// do nothing if the pointer is nil
	if t == nil {
		return nil
	}

	localTime := time.Time(*t)

	if timezone == "" {
		timezone = "Asia/Yangon"
	}

	// Load the location for the given timezone
	location, err := time.LoadLocation(timezone)
	if err != nil {
		fmt.Println("Error loading location:", err)
		return err
	}

	// Convert the start of the day local time to the specified timezone
	localTimeInZone := time.Date(
		localTime.Year(), localTime.Month(), localTime.Day(),
		0, 0, 0, 0,
		location,
	)

	// Convert the time to UTC
	utcTime := localTimeInZone.In(time.UTC)
	*t = MyDateString(utcTime)

	return nil
}

func (t *MyDateString) EndOfDayUTCTime(timezone string) error {
	// do nothing if the pointer is nil
	if t == nil {
		return nil
	}

	localTime := time.Time(*t)

	if timezone == "" {
		timezone = "Asia/Yangon"
	}

	// Load the location for the given timezone
	location, err := time.LoadLocation(timezone)
	if err != nil {
		fmt.Println("Error loading location:", err)
		return err
	}

	// Convert the end of the day local time to the specified timezone
	localTimeInZone := time.Date(
		localTime.Year(), localTime.Month(), localTime.Day(),
		23, 59, 59, 999, // Max nanoseconds
		location,
	)

	// Convert the time to UTC
	utcTime := localTimeInZone.In(time.UTC)
	*t = MyDateString(utcTime)

	return nil
}

func (t *MyDateString) UTCTime(timezone string) error {
	// do nothing if the pointer is nil
	if t == nil {
		return nil
	}

	localTime := time.Time(*t)

	if timezone == "" {
		timezone = "Asia/Yangon"
	}

	// Load the location for the given timezone
	location, err := time.LoadLocation(timezone)
	if err != nil {
		fmt.Println("Error loading location:", err)
		return err
	}

	// Convert the local time to the specified timezone
	localTimeInZone := time.Date(
		localTime.Year(), localTime.Month(), localTime.Day(),
		localTime.Hour(), localTime.Minute(), localTime.Second(), localTime.Nanosecond(),
		location,
	)

	// Convert the time to UTC
	utcTime := localTimeInZone.In(time.UTC)
	*t = MyDateString(utcTime)

	return nil
}

// Value implements the driver.Valuer interface
func (t MyDateString) Value() (driver.Value, error) {
	return time.Time(t), nil
}

// Scan implements the sql.Scanner interface
func (t *MyDateString) Scan(value interface{}) error {
	if value == nil {
		*t = MyDateString(time.Time{})
		return nil
	}
	switch v := value.(type) {
	case time.Time:
		*t = MyDateString(v)
	default:
		return fmt.Errorf("cannot convert %T to MyDateString", value)
	}
	return nil
}

func (t *MyDateString) SetDefaultNowIfNil() *MyDateString {
	if t == nil {
		now := MyDateString(time.Now())
		return &now
	}
	return t
}
