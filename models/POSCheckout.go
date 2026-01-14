package models

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/mmdatafocus/books_backend/config"
	"github.com/mmdatafocus/books_backend/utils"
	"github.com/shopspring/decimal"
)

type PosCheckoutInvoicePayment struct {
	ID                         int             `gorm:"primary_key" json:"id"`
	BusinessId                 string          `gorm:"index;not null" json:"business_id" binding:"required"`
	SalesInvoiceId             int             `gorm:"index;not null" json:"sales_invoice_id" binding:"required"`
	CustomerPaymentId          int             `gorm:"index;not null" json:"customer_payment_id" binding:"required"`
	CustomerId                 int             `gorm:"index;not null" json:"customer_id" binding:"required"`
	BranchId                   int             `gorm:"index;not null" json:"branch_id" binding:"required"`
	SequenceNo                 decimal.Decimal `gorm:"type:decimal(15);not null" json:"sequence_no"`
	InvoiceNumber              string          `gorm:"size:255;not null" json:"invoice_number" binding:"required"`
	ReferenceNumber            string          `gorm:"size:255;default:null" json:"reference_number"`
	InvoiceDate                time.Time       `gorm:"not null" json:"invoice_date" binding:"required"`
	SalesPersonId              int             `gorm:"default:null" json:"sales_person_id"`
	Notes                      string          `gorm:"type:text;default:null" json:"notes"`
	CurrencyId                 int             `gorm:"not null" json:"currency_id" binding:"required"`
	ExchangeRate               decimal.Decimal `gorm:"type:decimal(20,4);default:0" json:"exchange_rate"`
	WarehouseId                int             `gorm:"not null" json:"warehouse_id" binding:"required"`
	ShippingCharges            decimal.Decimal `gorm:"type:decimal(20,4);default:0" json:"shipping_charges"`
	AdjustmentAmount           decimal.Decimal `gorm:"type:decimal(20,4);default:0" json:"adjustment_amount"`
	IsTaxInclusive             *bool           `gorm:"not null;default:false" json:"is_tax_inclusive"`
	InvoiceTaxId               int             `gorm:"default:null" json:"invoice_tax_id"`
	InvoiceTaxType             *TaxType        `gorm:"type:enum('I', 'G');default:null" json:"invoice_tax_type"`
	InvoiceTaxAmount           decimal.Decimal `gorm:"type:decimal(20,4);default:0" json:"invoice_tax_amount"`
	InvoiceTotalDiscountAmount decimal.Decimal `gorm:"type:decimal(20,4);default:0" json:"invoice_total_discount_amount"`
	InvoiceTotalTaxAmount      decimal.Decimal `gorm:"type:decimal(20,4);default:0" json:"invoice_total_tax_amount"`
	InvoiceTotalAmount         decimal.Decimal `gorm:"type:decimal(20,4);default:0" json:"invoice_total_amount"`
	InvoiceTotalPaidAmount     decimal.Decimal `gorm:"type:decimal(20,4);default:0" json:"invoice_total_paid_amount"`
	// customer payment
	BankCharges      decimal.Decimal `gorm:"type:decimal(20,4);default:0" json:"bank_charges"`
	PaymentNumber    string          `gorm:"size:255;default:null" json:"payment_number"`
	PaymentModeId    int             `gorm:"default:null" json:"payment_mode"`
	DepositAccountId int             `gorm:"default:null" json:"deposit_account_id"`
	CreatedAt        time.Time       `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt        time.Time       `gorm:"autoUpdateTime" json:"updated_at"`
}

type NewPosCheckout struct {
	CustomerId                    int                     `json:"customer_id" binding:"required"`
	BranchId                      int                     `json:"branch_id" binding:"required"`
	ReferenceNumber               string                  `json:"reference_number"`
	InvoiceDate                   time.Time               `json:"invoice_date" binding:"required"`
	InvoicePaymentTerms           PaymentTerms            `json:"invoice_payment_terms" binding:"required"`
	InvoicePaymentTermsCustomDays int                     `json:"invoice_payment_terms_custom_days"`
	SalesPersonId                 int                     `json:"sales_person_id"`
	InvoiceSubject                string                  `json:"invoice_subject"`
	Notes                         string                  `json:"notes"`
	TermsAndConditions            string                  `json:"terms_and_conditions"`
	CurrencyId                    int                     `json:"currency_id" binding:"required"`
	ExchangeRate                  decimal.Decimal         `json:"exchange_rate"`
	WarehouseId                   int                     `json:"warehouse_id" binding:"required"`
	InvoiceDiscount               decimal.Decimal         `json:"invoice_discount"`
	InvoiceDiscountType           *DiscountType           `json:"invoice_discount_type"`
	ShippingCharges               decimal.Decimal         `json:"shipping_charges"`
	AdjustmentAmount              decimal.Decimal         `json:"adjustment_amount"`
	IsTaxInclusive                *bool                   `json:"is_tax_inclusive" binding:"required"`
	InvoiceTaxId                  int                     `json:"invoice_tax_id"`
	InvoiceTaxType                *TaxType                `json:"invoice_tax_type"`
	CurrentStatus                 SalesInvoiceStatus      `json:"current_status" binding:"required"`
	Details                       []NewSalesInvoiceDetail `json:"details"`
	// customer payment
	BankCharges      decimal.Decimal `json:"bank_charges"`
	PaymentNumber    string          `json:"payment_number" binding:"required"`
	PaymentModeId    int             `json:"payment_mode"`
	DepositAccountId int             `json:"deposit_account_id"`
}

func (input NewPosCheckout) validate(ctx context.Context, businessId string, _ int) error {
	// exists customer
	if err := utils.ValidateResourceId[Customer](ctx, businessId, input.CustomerId); err != nil {
		return errors.New("customer not found")
	}
	// exists branch
	if err := utils.ValidateResourceId[Branch](ctx, businessId, input.BranchId); err != nil {
		return errors.New("branch not found")
	}
	// exists Currency
	if err := utils.ValidateResourceId[Currency](ctx, businessId, input.CurrencyId); err != nil {
		return errors.New("currency not found")
	}
	// exists wareshouse
	if input.WarehouseId > 0 {
		// exists warehouse
		if err := utils.ValidateResourceId[Warehouse](ctx, businessId, input.WarehouseId); err != nil {
			return errors.New("warehouse not found")
		}
	}
	// exists SalePerson
	if input.SalesPersonId > 0 {
		// exists SalesPerson
		if err := utils.ValidateResourceId[SalesPerson](ctx, businessId, input.SalesPersonId); err != nil {
			return errors.New("salesPerson not found")
		}
	}
	// validate invoiceDate
	if err := validateTransactionLock(ctx, input.InvoiceDate, businessId, SalesTransactionLock); err != nil {
		return err
	}
	// check for inventory value adjustment
	for _, detail := range input.Details {
		if err := ValidateValueAdjustment(ctx, businessId, input.InvoiceDate, detail.ProductType, detail.ProductId, &detail.BatchNumber); err != nil {
			return fmt.Errorf(err.Error(), detail.Name)
		}
	}

	return nil
}

func CreatePosInvoicePayment(ctx context.Context, input *NewPosCheckout) (string, error) {
	db := config.GetDB()

	businessId, ok := utils.GetBusinessIdFromContext(ctx)
	if !ok || businessId == "" {
		return "", errors.New("business id is required")
	}

	business, err := GetBusinessById(ctx, businessId)
	if err != nil {
		return "", err
	}

	// validate SalesInvoice
	if err := input.validate(ctx, businessId, 0); err != nil {
		return "", err
	}

	tx := db.Begin()

	var invoiceItems []SalesInvoiceDetail
	var invoiceSubtotal,
		invoiceTotalAmount,
		totalExclusiveTaxAmount,
		totalDetailDiscountAmount,
		totalDetailTaxAmount decimal.Decimal

	for _, item := range input.Details {
		invoiceItem := SalesInvoiceDetail{
			ProductId:          item.ProductId,
			ProductType:        item.ProductType,
			BatchNumber:        item.BatchNumber,
			Name:               item.Name,
			Description:        item.Description,
			DetailQty:          item.DetailQty,
			DetailUnitRate:     item.DetailUnitRate,
			DetailTaxId:        item.DetailTaxId,
			DetailTaxType:      item.DetailTaxType,
			DetailDiscount:     item.DetailDiscount,
			DetailDiscountType: item.DetailDiscountType,
			DetailAccountId:    item.DetailAccountId,
			SalesOrderItemId:   item.SalesOrderItemId,
		}

		if err := ValidateProductStock(tx, ctx, businessId, input.WarehouseId, item.BatchNumber, item.ProductType, item.ProductId, item.DetailQty); err != nil {
			return "", err
		}
		// Calculate tax and total amounts for the item
		invoiceItem.CalculateSaleItemDiscountAndTax(ctx, *input.IsTaxInclusive)

		invoiceSubtotal = invoiceSubtotal.Add(invoiceItem.DetailTotalAmount)
		totalDetailDiscountAmount = totalDetailDiscountAmount.Add(invoiceItem.DetailDiscountAmount)
		totalDetailTaxAmount = totalDetailTaxAmount.Add(invoiceItem.DetailTaxAmount)

		if input.IsTaxInclusive != nil && *input.IsTaxInclusive {
			totalExclusiveTaxAmount = totalExclusiveTaxAmount.Add(decimal.NewFromFloat(0.0))
		} else {
			totalExclusiveTaxAmount = totalExclusiveTaxAmount.Add(invoiceItem.DetailTaxAmount)
		}
		invoiceItem.Cogs = decimal.NewFromInt(0)

		// Add the item to the PurchaseOrder
		invoiceItems = append(invoiceItems, invoiceItem)
	}

	// calculate order discount
	var invoiceDiscountAmount decimal.Decimal

	if input.InvoiceDiscountType != nil {
		invoiceDiscountAmount = utils.CalculateDiscountAmount(invoiceSubtotal, input.InvoiceDiscount, string(*input.InvoiceDiscountType))
	}

	// invoiceSubtotal = invoiceSubtotal.Sub(invoiceDiscountAmount)

	// calculate order tax amount (always exclusive)
	var invoiceTaxAmount decimal.Decimal
	if input.InvoiceTaxId > 0 {
		if *input.InvoiceTaxType == TaxTypeGroup {
			invoiceTaxAmount = utils.CalculateTaxAmount(ctx, db, input.InvoiceTaxId, true, invoiceSubtotal, false)
		} else {
			invoiceTaxAmount = utils.CalculateTaxAmount(ctx, db, input.InvoiceTaxId, false, invoiceSubtotal, false)
		}
	} else {
		invoiceTaxAmount = decimal.NewFromFloat(0)
	}

	// Sum (order discount + total detail discount)
	totalInvoiceDiscountAmount := invoiceDiscountAmount.Add(totalDetailDiscountAmount)
	// Sum (Invoice tax amount + total detail tax amount)
	totalInvoiceTaxAmount := invoiceTaxAmount.Add(totalDetailTaxAmount)

	invoiceTotalAmount = invoiceSubtotal.Add(invoiceTaxAmount).Add(totalExclusiveTaxAmount).Add(input.AdjustmentAmount).Add(input.ShippingCharges).Sub(invoiceDiscountAmount)

	// store saleInvoice
	saleInvoice := SalesInvoice{
		BusinessId:                    businessId,
		CustomerId:                    input.CustomerId,
		BranchId:                      input.BranchId,
		ReferenceNumber:               input.ReferenceNumber,
		InvoiceDate:                   input.InvoiceDate,
		InvoiceDueDate:                calculateDueDate(input.InvoiceDate, input.InvoicePaymentTerms, input.InvoicePaymentTermsCustomDays),
		InvoicePaymentTerms:           input.InvoicePaymentTerms,
		InvoicePaymentTermsCustomDays: input.InvoicePaymentTermsCustomDays,
		SalesPersonId:                 input.SalesPersonId,
		Notes:                         input.Notes,
		TermsAndConditions:            input.TermsAndConditions,
		CurrencyId:                    input.CurrencyId,
		ExchangeRate:                  input.ExchangeRate,
		WarehouseId:                   input.WarehouseId,
		InvoiceDiscount:               input.InvoiceDiscount,
		InvoiceDiscountType:           input.InvoiceDiscountType,
		InvoiceDiscountAmount:         invoiceDiscountAmount,
		ShippingCharges:               input.ShippingCharges,
		AdjustmentAmount:              input.AdjustmentAmount,
		IsTaxInclusive:                input.IsTaxInclusive,
		InvoiceTaxId:                  input.InvoiceTaxId,
		InvoiceTaxType:                input.InvoiceTaxType,
		InvoiceTaxAmount:              invoiceTaxAmount,
		CurrentStatus:                 input.CurrentStatus,
		Details:                       invoiceItems,
		InvoiceTotalDiscountAmount:    totalInvoiceDiscountAmount,
		InvoiceTotalTaxAmount:         totalInvoiceTaxAmount,
		InvoiceSubtotal:               invoiceSubtotal,
		InvoiceTotalAmount:            invoiceTotalAmount,
		InvoiceTotalPaidAmount:        invoiceTotalAmount,
	}

	seqNo, err := utils.GetSequence[SalesInvoice](ctx, businessId)
	if err != nil {
		tx.Rollback()
		return "", err
	}
	prefix, err := getTransactionPrefix(ctx, input.BranchId, "Invoice")
	if err != nil {
		tx.Rollback()
		return "", err
	}
	saleInvoice.SequenceNo = decimal.NewFromInt(seqNo)
	saleInvoice.InvoiceNumber = prefix + fmt.Sprint(seqNo)

	err = tx.WithContext(ctx).Create(&saleInvoice).Error
	if err != nil {
		tx.Rollback()
		return "", err
	}

	// construct paidInvoices
	var paidInvoices []PaidInvoice

	// construct new paidInvoice
	paidInvoice := PaidInvoice{
		InvoiceId:  saleInvoice.ID,
		PaidAmount: saleInvoice.InvoiceTotalAmount,
	}

	paidInvoices = append(paidInvoices, paidInvoice)
	// validate input currency_id
	if input.CurrencyId != business.BaseCurrencyId {
		account, err := GetAccount(ctx, input.DepositAccountId)
		if err != nil {
			tx.Rollback()
			return "", err
		}
		if account.CurrencyId != input.CurrencyId && account.CurrencyId != business.BaseCurrencyId {
			tx.Rollback()
			return "", errors.New("multiple foreign currencies not allowed")
		}
	}

	customerPayment := CustomerPayment{
		BusinessId:       businessId,
		CustomerId:       input.CustomerId,
		BranchId:         input.BranchId,
		CurrencyId:       input.CurrencyId,
		ExchangeRate:     input.ExchangeRate,
		Amount:           saleInvoice.InvoiceTotalAmount,
		BankCharges:      input.BankCharges,
		PaymentDate:      input.InvoiceDate,
		PaymentModeId:    input.PaymentModeId,
		DepositAccountId: input.DepositAccountId,
		ReferenceNumber:  input.ReferenceNumber,
		Notes:            input.Notes,
		PaidInvoices:     paidInvoices,
	}

	paymentSeqNo, err := utils.GetSequence[CustomerPayment](ctx, businessId)
	if err != nil {
		tx.Rollback()
		return "", err
	}
	paymentPrefix, err := getTransactionPrefix(ctx, input.BranchId, "Customer Payment")
	if err != nil {
		tx.Rollback()
		return "", err
	}
	customerPayment.SequenceNo = decimal.NewFromInt(paymentSeqNo)
	customerPayment.PaymentNumber = paymentPrefix + fmt.Sprint(paymentSeqNo)

	err = tx.WithContext(ctx).Create(&customerPayment).Error
	if err != nil {
		tx.Rollback()
		return "", err
	}

	posCheckout := PosCheckoutInvoicePayment{
		BusinessId:                 saleInvoice.BusinessId,
		SalesInvoiceId:             saleInvoice.ID,
		CustomerId:                 saleInvoice.CustomerId,
		BranchId:                   saleInvoice.BranchId,
		InvoiceDate:                saleInvoice.InvoiceDate,
		ReferenceNumber:            saleInvoice.ReferenceNumber,
		SalesPersonId:              saleInvoice.SalesPersonId,
		CurrencyId:                 saleInvoice.CurrencyId,
		Notes:                      saleInvoice.Notes,
		ExchangeRate:               saleInvoice.ExchangeRate,
		WarehouseId:                saleInvoice.WarehouseId,
		ShippingCharges:            saleInvoice.ShippingCharges,
		AdjustmentAmount:           saleInvoice.AdjustmentAmount,
		IsTaxInclusive:             saleInvoice.IsTaxInclusive,
		InvoiceTotalAmount:         saleInvoice.InvoiceTotalAmount,
		InvoiceTotalDiscountAmount: saleInvoice.InvoiceTotalDiscountAmount,
		InvoiceTotalTaxAmount:      saleInvoice.InvoiceTotalTaxAmount,
		InvoiceTotalPaidAmount:     saleInvoice.InvoiceTotalPaidAmount,
		CustomerPaymentId:          customerPayment.ID,
		BankCharges:                customerPayment.BankCharges,
		PaymentNumber:              customerPayment.PaymentNumber,
		PaymentModeId:              customerPayment.PaymentModeId,
		DepositAccountId:           customerPayment.DepositAccountId,
	}

	posSeqNo, err := utils.GetSequence[PosCheckoutInvoicePayment](ctx, businessId)
	if err != nil {
		tx.Rollback()
		return "", err
	}
	posPrefix, err := getTransactionPrefix(ctx, input.BranchId, "POS Invoice Payment")
	if err != nil {
		tx.Rollback()
		return "", err
	}
	posCheckout.SequenceNo = decimal.NewFromInt(posSeqNo)
	posCheckout.InvoiceNumber = posPrefix + fmt.Sprint(posSeqNo)

	err = tx.WithContext(ctx).Create(&posCheckout).Error
	if err != nil {
		tx.Rollback()
		return "", err
	}

	err = PublishToAccounting(ctx, tx, businessId, posCheckout.InvoiceDate, posCheckout.ID, AccountReferenceTypePosInvoicePayment, posCheckout, nil, PubSubMessageActionCreate)
	if err != nil {
		tx.Rollback()
		return "", err
	}

	// Commit the transaction
	if err := tx.Commit().Error; err != nil {
		return "", err
	}

	return "Order placed successfully.", nil
}
