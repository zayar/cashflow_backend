package models

import (
	"context"
	"errors"
	"fmt"
	"time"

	"bitbucket.org/mmdatafocus/books_backend/config"
	"bitbucket.org/mmdatafocus/books_backend/utils"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

type CustomerPayment struct {
	ID               int             `gorm:"primary_key" json:"id"`
	BusinessId       string          `gorm:"index;not null" json:"business_id" binding:"required"`
	CustomerId       int             `gorm:"index;not null" json:"customer_id" binding:"required"`
	BranchId         int             `gorm:"index;not null" json:"branch_id"`
	CurrencyId       int             `gorm:"index;not null" json:"currency_id" binding:"required"`
	ExchangeRate     decimal.Decimal `gorm:"type:decimal(20,4);default:0" json:"exchange_rate"`
	Amount           decimal.Decimal `gorm:"type:decimal(20,4);default:0" json:"amount" binding:"required"`
	BankCharges      decimal.Decimal `gorm:"type:decimal(20,4);default:0" json:"bank_charges"`
	PaymentDate      time.Time       `gorm:"not null" json:"payment_date" binding:"required"`
	PaymentNumber    string          `gorm:"size:255;not null" json:"payment_number" binding:"required"`
	SequenceNo       decimal.Decimal `gorm:"type:decimal(15);not null" json:"sequence_no"`
	PaymentModeId    int             `gorm:"default:null" json:"payment_mode"`
	DepositAccountId int             `gorm:"default:null" json:"deposit_account_id"`
	ReferenceNumber  string          `gorm:"size:255;default:null" json:"reference_number"`
	Notes            string          `gorm:"type:text;default:null" json:"notes"`
	Documents        []*Document     `gorm:"polymorphic:Reference" json:"documents"`
	PaidInvoices     []PaidInvoice   `json:"paid_invoices" validate:"required,dive,required"`
	CreatedAt        time.Time       `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt        time.Time       `gorm:"autoUpdateTime" json:"updated_at"`
}

type NewCustomerPayment struct {
	BusinessId       string           `json:"business_id" binding:"required"`
	CustomerId       int              `json:"customer_id" binding:"required"`
	BranchId         int              `json:"branch_id"`
	CurrencyId       int              `json:"currency_id" binding:"required"`
	Amount           decimal.Decimal  `json:"amount" binding:"required"`
	ExchangeRate     decimal.Decimal  `json:"exchange_rate"`
	BankCharges      decimal.Decimal  `json:"bank_charges"`
	PaymentDate      time.Time        `json:"payment_date" binding:"required"`
	PaymentModeId    int              `json:"payment_mode"`
	DepositAccountId int              `json:"deposit_account_id"`
	ReferenceNumber  string           `json:"reference_number"`
	Notes            string           `json:"notes"`
	Documents        []*NewDocument   `json:"documents"`
	PaidInvoices     []NewPaidInvoice `json:"paid_invoices"`
}

type CustomerPaymentsEdge Edge[CustomerPayment]
type CustomerPaymentsConnection struct {
	Edges    []*CustomerPaymentsEdge `json:"edges"`
	PageInfo *PageInfo               `json:"pageInfo"`
}

// implements methods for pagination

// node
// returns decoded curosr string
func (cp CustomerPayment) GetCursor() string {
	return cp.CreatedAt.String()
}

func (cp CustomerPayment) GetId() int {
	return cp.ID
}

type PaidInvoice struct {
	ID                int             `gorm:"primary_key" json:"id"`
	CustomerPaymentId int             `json:"customer_payment_id" binding:"required"`
	InvoiceId         int             `json:"invoice_id" binding:"required"`
	PaidAmount        decimal.Decimal `gorm:"type:decimal(20,4);default:0" json:"paid_amount" binding:"required"`
	CreatedAt         time.Time       `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt         time.Time       `gorm:"autoUpdateTime" json:"updated_at"`
}

type NewPaidInvoice struct {
	PaidInvoiceId int             `json:"paid_invoice_id"`
	InvoiceId     int             `json:"invoice_id" binding:"required"`
	PaidAmount    decimal.Decimal `json:"paid_amount" binding:"required"`
}

// GetID method for PaidInvoice reference Data
func (pi *PaidInvoice) GetID() int {
	return pi.ID
}

func (cp CustomerPayment) CheckTransactionLock(ctx context.Context) error {
	return validateTransactionLock(ctx, cp.PaymentDate, cp.BusinessId, SalesTransactionLock)
}

func (input NewCustomerPayment) validate(ctx context.Context, businessId string) error {

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
	// validate creditNoteDate
	if err := validateTransactionLock(ctx, input.PaymentDate, businessId, SalesTransactionLock); err != nil {
		return err
	}

	return nil
}

func CreateCustomerPayment(ctx context.Context, input *NewCustomerPayment) (*CustomerPayment, error) {

	businessId, ok := utils.GetBusinessIdFromContext(ctx)
	if !ok || businessId == "" {
		return nil, errors.New("business id is required")
	}

	if err := input.validate(ctx, businessId); err != nil {
		return nil, err
	}
	business, err := GetBusinessById(ctx, businessId)
	if err != nil {
		return nil, err
	}

	db := config.GetDB()
	tx := db.Begin()
	// construct paidInvoices
	paidInvoices, err := mapPaidInvoicesInput(tx, ctx, businessId, input)
	if err != nil {
		tx.Rollback()
		return nil, err
	}
	// validate input currency_id
	if input.CurrencyId != business.BaseCurrencyId {
		account, err := GetAccount(ctx, input.DepositAccountId)
		if err != nil {
			tx.Rollback()
			return nil, err
		}
		if account.CurrencyId != input.CurrencyId && account.CurrencyId != business.BaseCurrencyId {
			tx.Rollback()
			return nil, errors.New("multiple foreign currencies not allowed")
		}
	}
	// construct documents
	documents, err := mapNewDocuments(input.Documents, "documents", 0)
	if err != nil {
		tx.Rollback()
		return nil, err
	}

	customerPayment := CustomerPayment{
		BusinessId:       businessId,
		CustomerId:       input.CustomerId,
		BranchId:         input.BranchId,
		CurrencyId:       input.CurrencyId,
		ExchangeRate:     input.ExchangeRate,
		Amount:           input.Amount,
		BankCharges:      input.BankCharges,
		PaymentDate:      input.PaymentDate,
		PaymentModeId:    input.PaymentModeId,
		DepositAccountId: input.DepositAccountId,
		ReferenceNumber:  input.ReferenceNumber,
		Notes:            input.Notes,
		Documents:        documents,
		PaidInvoices:     paidInvoices,
	}

	seqNo, err := utils.GetSequence[CustomerPayment](ctx, businessId)
	if err != nil {
		tx.Rollback()
		return nil, err
	}
	prefix, err := getTransactionPrefix(ctx, input.BranchId, "Customer Payment")
	if err != nil {
		tx.Rollback()
		return nil, err
	}
	customerPayment.SequenceNo = decimal.NewFromInt(seqNo)
	customerPayment.PaymentNumber = prefix + fmt.Sprint(seqNo)

	err = tx.WithContext(ctx).Create(&customerPayment).Error
	if err != nil {
		tx.Rollback()
		return nil, err
	}

	err = PublishToAccounting(ctx, tx, businessId, customerPayment.PaymentDate, customerPayment.ID, AccountReferenceTypeCustomerPayment, customerPayment, nil, PubSubMessageActionCreate)
	if err != nil {
		tx.Rollback()
		return nil, err
	}

	if err = tx.Commit().Error; err != nil {
		return nil, err
	}

	return &customerPayment, nil
}

func UpdateCustomerPayment(ctx context.Context, paymentID int, updatedCustomerPayment *NewCustomerPayment) (*CustomerPayment, error) {

	businessId, ok := utils.GetBusinessIdFromContext(ctx)
	if !ok || businessId == "" {
		return nil, errors.New("business id is required")
	}

	// validate input
	if err := updatedCustomerPayment.validate(ctx, businessId); err != nil {
		return nil, err
	}
	business, err := GetBusinessById(ctx, businessId)
	if err != nil {
		return nil, err
	}

	oldCustomerPayment, err := utils.FetchModelForChange[CustomerPayment](ctx, businessId, paymentID)
	if err != nil {
		return nil, err
	}
	var existingCustomerPayment CustomerPayment = *oldCustomerPayment

	db := config.GetDB()
	tx := db.Begin()
	// construct paidBills
	paidInvoices, err := mapPaidInvoicesInput(tx, ctx, businessId, updatedCustomerPayment)
	if err != nil {
		tx.Rollback()
		return nil, err
	}
	// validate updatedCustomerPayment currency_id
	if updatedCustomerPayment.CurrencyId != business.BaseCurrencyId {
		account, err := GetAccount(ctx, updatedCustomerPayment.DepositAccountId)
		if err != nil {
			tx.Rollback()
			return nil, err
		}
		if account.CurrencyId != updatedCustomerPayment.CurrencyId && account.CurrencyId != business.BaseCurrencyId {
			tx.Rollback()
			return nil, errors.New("multiple foreign currencies not allowed")
		}
	}

	// Update the fields of the existing purchase order with the provided updated details
	existingCustomerPayment.CustomerId = updatedCustomerPayment.CustomerId
	existingCustomerPayment.BranchId = updatedCustomerPayment.BranchId
	existingCustomerPayment.CurrencyId = updatedCustomerPayment.CurrencyId
	existingCustomerPayment.ExchangeRate = updatedCustomerPayment.ExchangeRate
	existingCustomerPayment.Amount = updatedCustomerPayment.Amount
	existingCustomerPayment.BankCharges = updatedCustomerPayment.BankCharges
	existingCustomerPayment.PaymentDate = updatedCustomerPayment.PaymentDate
	existingCustomerPayment.PaymentModeId = updatedCustomerPayment.PaymentModeId
	existingCustomerPayment.DepositAccountId = updatedCustomerPayment.DepositAccountId
	existingCustomerPayment.ReferenceNumber = updatedCustomerPayment.ReferenceNumber
	existingCustomerPayment.Notes = updatedCustomerPayment.Notes
	existingCustomerPayment.PaidInvoices = paidInvoices

	// Save the updated purchase order to the database
	if err := tx.WithContext(ctx).Save(&existingCustomerPayment).Error; err != nil {
		tx.Rollback()
		return nil, err
	}

	err = PublishToAccounting(ctx, tx, businessId, existingCustomerPayment.PaymentDate, existingCustomerPayment.ID, AccountReferenceTypeCustomerPayment, existingCustomerPayment, oldCustomerPayment, PubSubMessageActionUpdate)
	if err != nil {
		tx.Rollback()
		return nil, err
	}

	documents, err := upsertDocuments(ctx, tx, updatedCustomerPayment.Documents, "customer_payments", paymentID)
	if err != nil {
		tx.Rollback()
		return nil, err
	}
	existingCustomerPayment.Documents = documents

	if err = tx.Commit().Error; err != nil {
		return nil, err
	}

	return &existingCustomerPayment, nil
}

func DeleteCustomerPayment(ctx context.Context, id int) (*CustomerPayment, error) {
	businessId, ok := utils.GetBusinessIdFromContext(ctx)
	if !ok || businessId == "" {
		return nil, errors.New("business id is required")
	}

	db := config.GetDB()
	result, err := utils.FetchModelForChange[CustomerPayment](ctx, businessId, id, "PaidInvoices", "Documents")
	if err != nil {
		return nil, err
	}

	// Adjust InvoiceTotalPaidAmount for each associated PaidInvoice
	tx := db.Begin()
	for _, paidInvoice := range result.PaidInvoices {

		inv, err := utils.FetchModelForChange[SalesInvoice](ctx, businessId, paidInvoice.InvoiceId)
		if err != nil {
			tx.Rollback()
			return nil, err
		}

		// remainingPaidAmount := inv.InvoiceTotalPaidAmount.Sub(paidInvoice.PaidAmount)
		invPaymentAmount := inv.InvoiceTotalPaidAmount
		invAdvanceAmount := inv.InvoiceTotalAdvanceUsedAmount
		invCreditAmount := inv.InvoiceTotalCreditUsedAmount
		remainingPaidAmount := invPaymentAmount.Add(invAdvanceAmount).Add(invCreditAmount).Sub(paidInvoice.PaidAmount)
		if remainingPaidAmount.IsNegative() {
			tx.Rollback()
			return nil, errors.New("resulting Invoice total paid amount cannot be negative")
		}
		if remainingPaidAmount.GreaterThan(decimal.Zero) {
			inv.CurrentStatus = SalesInvoiceStatusPartialPaid
		} else {
			inv.CurrentStatus = SalesInvoiceStatusConfirmed
		}
		inv.InvoiceTotalPaidAmount = invPaymentAmount.Sub(paidInvoice.PaidAmount)
		inv.RemainingBalance = inv.RemainingBalance.Add(paidInvoice.PaidAmount)

		// Update the inv
		if err := tx.WithContext(ctx).Save(&inv).Error; err != nil {
			tx.Rollback()
			return nil, err
		}
	}

	err = db.WithContext(ctx).Model(&result).Association("PaidInvoices").Unscoped().Clear()
	if err != nil {
		tx.Rollback()
		return nil, err
	}

	err = tx.WithContext(ctx).Delete(&result).Error
	if err != nil {
		tx.Rollback()
		return nil, err
	}

	if err := deleteDocuments(ctx, tx, result.Documents); err != nil {
		tx.Rollback()
		return nil, err
	}

	err = PublishToAccounting(ctx, tx, businessId, result.PaymentDate, result.ID, AccountReferenceTypeCustomerPayment, nil, result, PubSubMessageActionDelete)
	if err != nil {
		tx.Rollback()
		return nil, err
	}

	if err = tx.Commit().Error; err != nil {
		return nil, err
	}

	return result, nil
}

func GetCustomerPayment(ctx context.Context, id int) (*CustomerPayment, error) {
	db := config.GetDB()

	var result CustomerPayment
	err := db.WithContext(ctx).First(&result, id).Error
	if err != nil {
		return nil, err
	}

	return &result, nil
}

func GetPaidInvoice(ctx context.Context, id int) (*PaidInvoice, error) {
	db := config.GetDB()

	var result PaidInvoice
	err := db.WithContext(ctx).First(&result, id).Error
	if err != nil {
		return nil, err
	}

	return &result, nil
}

func GetCustomerPayments(ctx context.Context, customerId *int, paymentNumber *string) ([]*CustomerPayment, error) {
	db := config.GetDB()
	var results []*CustomerPayment

	businessId, ok := utils.GetBusinessIdFromContext(ctx)
	if !ok || businessId == "" {
		return nil, errors.New("business id is required")
	}

	dbCtx := db.WithContext(ctx).Where("business_id = ?", businessId)
	if *customerId > 0 {
		dbCtx = dbCtx.Where("customer_id  = ?", *customerId)
	}
	if paymentNumber != nil && len(*paymentNumber) > 0 {
		dbCtx = dbCtx.Where("payment_number LIKE ?", "%"+*paymentNumber+"%")
	}
	err := dbCtx.
		Find(&results).Error
	if err != nil {
		return nil, err
	}
	return results, nil
}

func mapPaidInvoicesInput(tx *gorm.DB, ctx context.Context, businessId string, input *NewCustomerPayment) ([]PaidInvoice, error) {

	var paidInvoices []PaidInvoice
	var totalInvoicePaidAmount decimal.Decimal

	for _, paidInvoiceInput := range input.PaidInvoices {

		saleInvoice, err := utils.FetchModelForChange[SalesInvoice](ctx, businessId, paidInvoiceInput.InvoiceId)
		if err != nil {
			tx.Rollback()
			return nil, err
		}

		// new paid invoice
		if paidInvoiceInput.PaidInvoiceId == 0 {

			// paidAmount is greater than billRemainingAmount
			if paidInvoiceInput.PaidAmount.Cmp(saleInvoice.RemainingBalance) == 1 {
				tx.Rollback()
				return nil, errors.New("the amount entered is more than the balance for the selected invoice number - " + saleInvoice.InvoiceNumber)
			} else if paidInvoiceInput.PaidAmount.Cmp(saleInvoice.RemainingBalance) == -1 {
				// paidAmount is less than invoiceRemainingAmount
				saleInvoice.CurrentStatus = SalesInvoiceStatusPartialPaid
			} else {
				// paidAmount is equal to invoiceRemainingAmount
				saleInvoice.CurrentStatus = SalesInvoiceStatusPaid
			}
			// invoiceRemainingAmount = saleInvoice.InvoiceTotalAmount.Sub(saleInvoice.InvoiceTotalPaidAmount)

			saleInvoice.InvoiceTotalPaidAmount = saleInvoice.InvoiceTotalPaidAmount.Add(paidInvoiceInput.PaidAmount)
			saleInvoice.RemainingBalance = saleInvoice.RemainingBalance.Sub(paidInvoiceInput.PaidAmount)

			// construct new paidInvoice
			paidInvoice := PaidInvoice{
				InvoiceId:  paidInvoiceInput.InvoiceId,
				PaidAmount: paidInvoiceInput.PaidAmount,
			}

			paidInvoices = append(paidInvoices, paidInvoice)

		} else if paidInvoiceInput.PaidInvoiceId > 0 {
			// existing paid invoice
			var existingPaidInvoice PaidInvoice
			if err := tx.WithContext(ctx).First(&existingPaidInvoice, paidInvoiceInput.PaidInvoiceId).Error; err != nil {
				tx.Rollback()
				return nil, err
			}
			// unpay previous paidAmount, and pay new paidAmount
			saleInvoice.InvoiceTotalPaidAmount = saleInvoice.InvoiceTotalPaidAmount.Sub(existingPaidInvoice.PaidAmount).Add(paidInvoiceInput.PaidAmount)
			saleInvoice.RemainingBalance = saleInvoice.RemainingBalance.Add(existingPaidInvoice.PaidAmount).Sub(paidInvoiceInput.PaidAmount)
			existingPaidInvoice.PaidAmount = paidInvoiceInput.PaidAmount

			if saleInvoice.RemainingBalance.IsNegative() {
				tx.Rollback()
				return nil, errors.New("the amount entered is more than the balance for the selected invoice number - " + saleInvoice.InvoiceNumber)
			} else if saleInvoice.RemainingBalance.IsPositive() { // paidAmount is less than invoiceRemainingAmount
				saleInvoice.CurrentStatus = SalesInvoiceStatusPartialPaid
			} else {
				// paidAmount is equal to invoiceRemainingAmount
				saleInvoice.CurrentStatus = SalesInvoiceStatusPaid
			}

			if err := tx.WithContext(ctx).Save(&existingPaidInvoice).Error; err != nil {
				tx.Rollback()
				return nil, err
			}
			paidInvoices = append(paidInvoices, existingPaidInvoice)
		}

		if err := tx.WithContext(ctx).Save(&saleInvoice).Error; err != nil {
			tx.Rollback()
			return nil, err
		}

		totalInvoicePaidAmount = totalInvoicePaidAmount.Add(paidInvoiceInput.PaidAmount)
	}
	// check equal Customer's paid amount and paid amount of each bills
	// if input.Amount != totalInvoicePaidAmount {
	if input.Amount.Cmp(totalInvoicePaidAmount) != 0 {
		tx.Rollback()
		return nil, errors.New("the amount entered is not equal for each selected invoice")
	}
	return paidInvoices, nil
}

func PaginateCustomerPayment(ctx context.Context, limit *int, after *string,
	paymentNumber *string,
	referenceNumber *string,
	branchID *int,
	customerID *int,
	depositAccountID *int,
	startPaymentDate *MyDateString,
	endPaymentDate *MyDateString,
) (*CustomerPaymentsConnection, error) {

	businessId, ok := utils.GetBusinessIdFromContext(ctx)
	if !ok || businessId == "" {
		return nil, errors.New("business id is required")
	}

	business, err := GetBusiness(ctx)
	if err != nil {
		return nil, errors.New("business id is required")
	}
	if err := startPaymentDate.StartOfDayUTCTime(business.Timezone); err != nil {
		return nil, err
	}
	if err := endPaymentDate.EndOfDayUTCTime(business.Timezone); err != nil {
		return nil, err
	}

	db := config.GetDB()
	dbCtx := db.WithContext(ctx).Where("business_id = ?", businessId)
	if paymentNumber != nil && *paymentNumber != "" {
		dbCtx.Where("payment_number LIKE ?", "%"+*paymentNumber+"%")
	}
	if referenceNumber != nil && *referenceNumber != "" {
		dbCtx.Where("reference_number LIKE ?", "%"+*referenceNumber+"%")
	}
	if branchID != nil && *branchID > 0 {
		dbCtx.Where("branch_id = ?", *branchID)
	}
	if customerID != nil && *customerID > 0 {
		dbCtx.Where("customer_id = ?", *customerID)
	}
	if depositAccountID != nil && *depositAccountID > 0 {
		dbCtx.Where("deposit_account_id = ?", *depositAccountID)
	}
	if startPaymentDate != nil && endPaymentDate != nil {
		dbCtx.Where("payment_date BETWEEN ? AND ?", startPaymentDate, endPaymentDate)
	}

	edges, pageInfo, err := FetchPageCompositeCursor[CustomerPayment](dbCtx, *limit, after, "created_at", "<")
	if err != nil {
		return nil, err
	}
	var customerPaymentsConnection CustomerPaymentsConnection
	customerPaymentsConnection.PageInfo = pageInfo
	for _, edge := range edges {
		customerPaymentEdge := CustomerPaymentsEdge(edge)
		customerPaymentsConnection.Edges = append(customerPaymentsConnection.Edges, &customerPaymentEdge)
	}

	return &customerPaymentsConnection, err
}
