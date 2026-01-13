package models

import (
	"context"
	"errors"
	"time"

	"bitbucket.org/mmdatafocus/books_backend/config"
	"bitbucket.org/mmdatafocus/books_backend/utils"
	"github.com/shopspring/decimal"
)

type CustomerCreditInvoice struct {
	ID                   int                     `gorm:"primary_key" json:"id"`
	BusinessId           string                  `gorm:"autoIncrement:false;not null" json:"business_id" binding:"required"`
	ReferenceId          int                     `gorm:"autoIncrement:false;not null" json:"reference_id" binding:"required"`
	ReferenceType        CustomerCreditApplyType `gorm:"type:enum('Credit','Advance');not null" json:"reference_type"`
	BranchId             int                     `gorm:"not null" json:"branch_id"`
	CustomerId           int                     `gorm:"not null" json:"customer_id"`
	InvoiceId            int                     `gorm:"autoIncrement:false;not null" json:"invoice_id" binding:"required"`
	CreditDate           time.Time               `gorm:"not null" json:"credit_date"`
	Amount               decimal.Decimal         `gorm:"type:decimal(20,4);default:0" json:"amount"`
	CustomerCreditNumber string                  `gorm:"size:255;default:null" json:"customer_credit_number"`
	InvoiceNumber        string                  `gorm:"size:255;default:null" json:"invoice_number"`
	CurrencyId           int                     `gorm:"not null" json:"currency_id" binding:"required"`
	ExchangeRate         decimal.Decimal         `gorm:"type:decimal(20,4);default:0" json:"exchange_rate"`
	InvoiceCurrencyId    int                     `gorm:"default:0" json:"invoice_currency_id"`
	InvoiceExchangeRate  decimal.Decimal         `gorm:"type:decimal(20,4);default:0" json:"invoice_exchange_rate"`
	CreatedAt            time.Time               `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt            time.Time               `gorm:"autoUpdateTime" json:"updated_at"`
}

// amount is credit's amount, currencyId is credit's currency, based on that, the amount applied to invoice may or may not be the same amount

func (cci CustomerCreditInvoice) CheckTransactionLock(ctx context.Context) error {
	return validateTransactionLock(ctx, cci.CreditDate, cci.BusinessId, SalesTransactionLock)
}

// type NewCustomerCreditApplyToInvoice struct {
// 	CustomerCreditId int                     `json:"customer_credit_id" binding:"required"`
// 	ApplyInvoices    []NewCreditApplyInvoice `json:"invoices"`
// }
// type NewCreditApplyInvoice struct {
// 	InvoiceId     int             `json:"invoice_id" binding:"required"`
// 	InvoiceNumber string          `json:"invoice_number"`
// 	Amount        decimal.Decimal `json:"amount"`
// 	CurrencyId    int             `json:"currency_id"`
// 	ExchangeRate  decimal.Decimal `json:"exchange_rate"`
// }

type NewInvoiceApplyToCustomerCredit struct {
	InvoiceId    int                     `json:"invoice_id" binding:"required"`
	ApplyCredits []NewInvoiceApplyCredit `json:"credits"`
}

type NewInvoiceApplyCredit struct {
	ReferenceId   int                     `json:"reference_id"`
	ReferenceType CustomerCreditApplyType `json:"reference_type"`
	Amount        decimal.Decimal         `json:"amount"`
	CurrencyId    int                     `json:"currency_id"`
	ExchangeRate  decimal.Decimal         `json:"exchange_rate"`
}

// commenting since not used, has multi-currency issues
// // use CreditNote to pay Invoices
// // 2d fix just like the other function
// func CreateCustomerApplyToInvoice(ctx context.Context, input *NewCustomerCreditApplyToInvoice) ([]*CustomerCreditInvoice, error) {
// 	db := config.GetDB()

// 	businessId, ok := utils.GetBusinessIdFromContext(ctx)
// 	if !ok || businessId == "" {
// 		return nil, errors.New("business id is required")
// 	}

// 	existingCreditNote, err := utils.FetchModelForChange[CreditNote](ctx, businessId, input.CustomerCreditId)
// 	if err != nil {
// 		return nil, err
// 	}

// 	if existingCreditNote.CurrentStatus != CreditNoteStatusConfirmed {
// 		return nil, errors.New("customer credit status must be confirm")
// 	}

// 	var customerCreditTotalUsedAmount decimal.Decimal
// 	var createdCustomerCreditInvoices []*CustomerCreditInvoice

// 	tx := db.Begin()

// 	for _, item := range input.ApplyInvoices {
// 		var baseCurrencyAmount decimal.Decimal
// 		var invoiceCreditUseAmount, remainingBalance decimal.Decimal
// 		invoice, err := utils.FetchModelForChange[SalesInvoice](ctx, businessId, item.InvoiceId)
// 		if err != nil {
// 			tx.Rollback()
// 			return nil, err
// 		}

// 		if invoice.CurrentStatus != SalesInvoiceStatusConfirmed && invoice.CurrentStatus != SalesInvoiceStatusPartialPaid {
// 			tx.Rollback()
// 			return nil, errors.New("invoice status must be confirm or partial paid")
// 		}

// 		if item.CurrencyId != existingCreditNote.CurrencyId {
// 			baseCurrencyAmount = item.Amount.Mul(item.ExchangeRate)
// 			if baseCurrencyAmount.GreaterThan(invoice.RemainingBalance) {
// 				tx.Rollback()
// 				return nil, errors.New("amount less than or equal invoice remaining amount")
// 			}

// 			invoiceCreditUseAmount = invoice.InvoiceTotalCreditUsedAmount.Add(baseCurrencyAmount)
// 			remainingBalance = invoice.RemainingBalance.Sub(baseCurrencyAmount)
// 		} else {
// 			if item.Amount.GreaterThan(invoice.RemainingBalance) {
// 				tx.Rollback()
// 				return nil, errors.New("amount less than or equal invoice remaining amount")
// 			}

// 			invoiceCreditUseAmount = invoice.InvoiceTotalCreditUsedAmount.Add(item.Amount)
// 			remainingBalance = invoice.RemainingBalance.Sub(item.Amount)
// 		}

// 		customerCreditInvoice := CustomerCreditInvoice{
// 			BusinessId:           businessId,
// 			ReferenceId:          existingCreditNote.ID,
// 			ReferenceType:        CustomerCreditApplyTypeCredit,
// 			BranchId:             existingCreditNote.BranchId,
// 			CustomerId:           existingCreditNote.CustomerId,
// 			InvoiceId:            item.InvoiceId,
// 			InvoiceNumber:        item.InvoiceNumber,
// 			CustomerCreditNumber: existingCreditNote.CreditNoteNumber,
// 			CreditDate:           existingCreditNote.CreditNoteDate,
// 			Amount:               item.Amount,
// 			CurrencyId:           item.CurrencyId,
// 			ExchangeRate:         item.ExchangeRate,
// 		}

// 		err = tx.WithContext(ctx).Create(&customerCreditInvoice).Error
// 		if err != nil {
// 			tx.Rollback()
// 			return nil, err
// 		}

// 		if remainingBalance.IsZero() {
// 			invoice.CurrentStatus = SalesInvoiceStatusPaid
// 		} else {
// 			invoice.CurrentStatus = SalesInvoiceStatusPartialPaid
// 		}
// 		invoice.RemainingBalance = remainingBalance
// 		invoice.InvoiceTotalCreditUsedAmount = invoiceCreditUseAmount
// 		// Save the updated invoice's credit used amount
// 		if err := tx.WithContext(ctx).Save(&invoice).Error; err != nil {
// 			tx.Rollback()
// 			return nil, err
// 		}
// 		customerCreditTotalUsedAmount = customerCreditTotalUsedAmount.Add(item.Amount)
// 		// Append the created CustomerCreditBill to the slice
// 		createdCustomerCreditInvoices = append(createdCustomerCreditInvoices, &customerCreditInvoice)
// 	}

// 	totalUseAmount := existingCreditNote.CreditNoteTotalUsedAmount.Add(customerCreditTotalUsedAmount)

// 	if customerCreditTotalUsedAmount.GreaterThan(existingCreditNote.RemainingBalance) {
// 		tx.Rollback()
// 		return nil, errors.New("total amount less than or equal remaining balance of credit note")
// 	}

// 	remainingBalance := existingCreditNote.RemainingBalance.Sub(customerCreditTotalUsedAmount)

// 	if remainingBalance.IsZero() {
// 		existingCreditNote.CurrentStatus = CreditNoteStatusClosed
// 	}
// 	existingCreditNote.RemainingBalance = remainingBalance
// 	existingCreditNote.CreditNoteTotalUsedAmount = totalUseAmount
// 	// Save the updated bill's credit used amount
// 	if err := tx.WithContext(ctx).Save(&existingCreditNote).Error; err != nil {
// 		tx.Rollback()
// 		return nil, err
// 	}

// 	if err := tx.Commit().Error; err != nil {
// 		return nil, err
// 	}

// 	return createdCustomerCreditInvoices, nil
// }

// apply multiple credits to one invoice
func CreateCustomerApplyCredit(ctx context.Context, input *NewInvoiceApplyToCustomerCredit) ([]*CustomerCreditInvoice, error) {
	db := config.GetDB()

	business, err := GetBusiness(ctx)
	if err != nil {
		return nil, err
	}
	businessId := business.ID.String()

	invoice, err := utils.FetchModelForChange[SalesInvoice](ctx, businessId, input.InvoiceId)
	if err != nil {
		return nil, err
	}
	if invoice.CurrentStatus != SalesInvoiceStatusConfirmed && invoice.CurrentStatus != SalesInvoiceStatusPartialPaid {
		return nil, errors.New("invoice status must be confirm or partial paid")
	}

	var invoiceTotalCredit decimal.Decimal
	var invoiceTotalAdvance decimal.Decimal
	var customerCreditInvoices []*CustomerCreditInvoice

	tx := db.Begin()

	for _, item := range input.ApplyCredits {
		if item.Amount.IsZero() {
			continue
		}
		var referenceId int
		var creditNumber string
		var date time.Time
		var currencyId int
		exchangeRate := item.ExchangeRate
		invoiceCurrencyId := invoice.CurrencyId
		invoiceExchangeRate := invoice.ExchangeRate

		var adjustedAmount decimal.Decimal
		// var remainingBalance, advanceTotalUsedAmount decimal.Decimal

		if item.ReferenceType == CustomerCreditApplyTypeCredit {
			customerCredit, err := utils.FetchModelForChange[CreditNote](ctx, businessId, item.ReferenceId)
			if err != nil {
				tx.Rollback()
				return nil, err
			}

			if customerCredit.BranchId != invoice.BranchId {
				tx.Rollback()
				return nil, errors.New("cannot apply credit belonging to other branch")
			}
			// use creditNote amount
			if err := customerCredit.useAmount(item.Amount); err != nil {
				tx.Rollback()
				return nil, err
			}
			// if customerCredit.CurrentStatus != CreditNoteStatusConfirmed {
			// 	tx.Rollback()
			// 	return nil, errors.New("customerCredit status must be confirm")
			// }

			// if customerCredit.RemainingBalance.LessThan(item.Amount) {
			// 	tx.Rollback()
			// 	return nil, errors.New("credit remaining balance less than applied amount")
			// }
			// creditTotalUsedAmount = customerCredit.CreditNoteTotalUsedAmount.Add(item.Amount)
			// remainingBalance = customerCredit.RemainingBalance.Sub(item.Amount)
			// if remainingBalance.IsZero() {
			// 	customerCredit.CurrentStatus = CreditNoteStatusClosed
			// }

			// customerCredit.CreditNoteTotalUsedAmount = creditTotalUsedAmount
			// customerCredit.RemainingBalance = remainingBalance
			// Save the updated Customer credit's used amount
			if err := tx.WithContext(ctx).Save(&customerCredit).Error; err != nil {
				tx.Rollback()
				return nil, err
			}

			referenceId = customerCredit.ID
			creditNumber = customerCredit.CreditNoteNumber
			date = customerCredit.CreditNoteDate
			currencyId = customerCredit.CurrencyId

			adjustedAmount = item.Amount
			if customerCredit.CurrencyId != invoice.CurrencyId {
				// fromCurrency = baseCurrency
				adjustedAmount = business.AdjustCurrency(customerCredit.CurrencyId, item.Amount, item.ExchangeRate)
				if invoice.CurrencyId == business.BaseCurrencyId {
					exchangeRate = customerCredit.ExchangeRate
					invoiceExchangeRate = item.ExchangeRate
				}
			} else {
				exchangeRate = customerCredit.ExchangeRate
			}

			invoiceTotalCredit = invoiceTotalCredit.Add(adjustedAmount)
		} else {

			customerAdvance, err := utils.FetchModelForChange[CustomerCreditAdvance](ctx, businessId, item.ReferenceId)
			if err != nil {
				tx.Rollback()
				return nil, err
			}

			// use customerAdvance
			if err := customerAdvance.useAmount(item.Amount); err != nil {
				tx.Rollback()
				return nil, err
			}
			// if customerAdvance.CurrentStatus != CustomerAdvanceStatusConfirmed {
			// 	tx.Rollback()
			// 	return nil, errors.New("customer advance status must be confirm")
			// }

			// if customerAdvance.RemainingBalance.LessThan(item.Amount) {
			// 	tx.Rollback()
			// 	return nil, errors.New("advacne remaining balance less than applied amount")
			// }
			// advanceTotalUsedAmount = customerAdvance.UsedAmount.Add(item.Amount)
			// remainingBalance = customerAdvance.RemainingBalance.Sub(item.Amount)
			// if remainingBalance.IsZero() {
			// 	customerAdvance.CurrentStatus = CustomerAdvanceStatusClosed
			// }
			// customerAdvance.UsedAmount = advanceTotalUsedAmount
			// customerAdvance.RemainingBalance = remainingBalance
			// Save the updated customer advance's used amount
			if err := tx.WithContext(ctx).Save(&customerAdvance).Error; err != nil {
				tx.Rollback()
				return nil, err
			}

			referenceId = customerAdvance.ID
			creditNumber = ""
			date = customerAdvance.CreatedAt
			// validate CreatedAt which is used as CreditDate for newly created CustomerCreditInvoice
			if err := validateTransactionLock(ctx, customerAdvance.CreatedAt, businessId, SalesTransactionLock); err != nil {
				tx.Rollback()
				return nil, err
			}
			currencyId = customerAdvance.CurrencyId

			adjustedAmount = item.Amount
			if customerAdvance.CurrencyId != invoice.CurrencyId {
				adjustedAmount = business.AdjustCurrency(item.CurrencyId, item.Amount, item.ExchangeRate)
				// adjustedAmount = item.Amount.Mul(item.ExchangeRate)
				if invoice.CurrencyId == business.BaseCurrencyId {
					invoiceExchangeRate = item.ExchangeRate
					exchangeRate = customerAdvance.ExchangeRate
				}
			} else {
				exchangeRate = customerAdvance.ExchangeRate
			}

			invoiceTotalAdvance = invoiceTotalAdvance.Add(adjustedAmount)
		}

		customerCreditInvoice := CustomerCreditInvoice{
			BusinessId:           businessId,
			ReferenceId:          referenceId,
			ReferenceType:        item.ReferenceType,
			BranchId:             invoice.BranchId,
			CustomerId:           invoice.CustomerId,
			InvoiceId:            invoice.ID,
			InvoiceNumber:        invoice.InvoiceNumber,
			CustomerCreditNumber: creditNumber,
			CreditDate:           date,
			Amount:               item.Amount,
			CurrencyId:           currencyId,
			ExchangeRate:         exchangeRate,
			InvoiceCurrencyId:    invoiceCurrencyId,
			InvoiceExchangeRate:  invoiceExchangeRate,
		}

		err := tx.WithContext(ctx).Create(&customerCreditInvoice).Error
		if err != nil {
			tx.Rollback()
			return nil, err
		}

		// Append the created CustomerCreditAdvance to the slice
		customerCreditInvoices = append(customerCreditInvoices, &customerCreditInvoice)
	}

	totalCreditAmount := invoice.InvoiceTotalCreditUsedAmount.Add(invoiceTotalCredit)
	totalAdvanceAmount := invoice.InvoiceTotalAdvanceUsedAmount.Add(invoiceTotalAdvance)
	totalUseAmount := invoiceTotalCredit.Add(invoiceTotalAdvance)
	totalRemainingAmount := invoice.RemainingBalance.Sub(totalUseAmount)
	if totalUseAmount.GreaterThan(invoice.RemainingBalance) {
		tx.Rollback()
		return nil, errors.New("total applied amount greater than invoice total amount")
	}
	if totalRemainingAmount.IsZero() {
		invoice.CurrentStatus = SalesInvoiceStatusPaid
	} else {
		invoice.CurrentStatus = SalesInvoiceStatusPartialPaid
	}

	invoice.InvoiceTotalCreditUsedAmount = totalCreditAmount
	invoice.InvoiceTotalAdvanceUsedAmount = totalAdvanceAmount
	invoice.RemainingBalance = totalRemainingAmount
	// Save the updated bill's credit used amount
	if err := tx.WithContext(ctx).Save(&invoice).Error; err != nil {
		tx.Rollback()
		return nil, err
	}

	if len(customerCreditInvoices) > 0 {
		for _, customerCreditInvoice := range customerCreditInvoices {
			if customerCreditInvoice.ReferenceType == CustomerCreditApplyTypeAdvance {
				err := PublishToAccounting(ctx, tx, businessId, customerCreditInvoice.CreditDate, customerCreditInvoice.ID, AccountReferenceTypeCustomerAdvanceApplied, customerCreditInvoice, nil, PubSubMessageActionCreate)
				if err != nil {
					tx.Rollback()
					return nil, err
				}
			} else if customerCreditInvoice.ReferenceType == CustomerCreditApplyTypeCredit &&
				!customerCreditInvoice.ExchangeRate.Equals(customerCreditInvoice.InvoiceExchangeRate) {
				err := PublishToAccounting(ctx, tx, businessId, customerCreditInvoice.CreditDate, customerCreditInvoice.ID, AccountReferenceTypeCreditNoteApplied, customerCreditInvoice, nil, PubSubMessageActionCreate)
				if err != nil {
					tx.Rollback()
					return nil, err
				}
			}
		}
	}

	if err := tx.Commit().Error; err != nil {
		return nil, err
	}

	return customerCreditInvoices, nil
}

func DeleteCustomerCreditInvoice(ctx context.Context, id int) (*CustomerCreditInvoice, error) {

	business, err := GetBusiness(ctx)
	if err != nil {
		return nil, err
	}
	businessId := business.ID.String()

	db := config.GetDB()

	result, err := utils.FetchModelForChange[CustomerCreditInvoice](ctx, businessId, id)
	if err != nil {
		return nil, err
	}

	invoice, err := utils.FetchModelForChange[SalesInvoice](ctx, businessId, result.InvoiceId)
	if err != nil {
		return nil, err
	}

	tx := db.Begin()

	invoiceAppliedAmount := result.Amount
	// if result.CurrencyId != invoice.CurrencyId {
	// 	// invoiceAppliedAmount = result.Amount.Mul(result.ExchangeRate)
	// 	invoiceAppliedAmount = business.AdjustCurrency(result.CurrencyId, result.Amount, result.ExchangeRate)
	// }
	if result.CurrencyId == business.BaseCurrencyId && result.InvoiceCurrencyId != business.BaseCurrencyId {
		invoiceAppliedAmount = result.Amount.DivRound(result.ExchangeRate, 4)
	} else if result.CurrencyId != business.BaseCurrencyId && result.InvoiceCurrencyId == business.BaseCurrencyId {
		invoiceAppliedAmount = result.Amount.Mul(result.InvoiceExchangeRate)
	}

	if result.ReferenceType == CustomerCreditApplyTypeCredit {
		customerCredit, err := utils.FetchModelForChange[CreditNote](ctx, businessId, result.ReferenceId)
		if err != nil {
			tx.Rollback()
			return nil, err
		}

		if err := customerCredit.unUseAmount(result.Amount); err != nil {
			tx.Rollback()
			return nil, err
		}
		// if customerCredit.CurrentStatus == CreditNoteStatusClosed {
		// 	customerCredit.CurrentStatus = CreditNoteStatusConfirmed
		// }

		// customerCredit.CreditNoteTotalUsedAmount = customerCredit.CreditNoteTotalUsedAmount.Sub(result.Amount)
		// customerCredit.RemainingBalance = customerCredit.RemainingBalance.Add(result.Amount)
		// Save the updated customerCredit's credit used amount
		if err := tx.WithContext(ctx).Save(&customerCredit).Error; err != nil {
			tx.Rollback()
			return nil, err
		}

		invoice.InvoiceTotalCreditUsedAmount = invoice.InvoiceTotalCreditUsedAmount.Sub(invoiceAppliedAmount)

	} else {
		advance, err := utils.FetchModel[CustomerCreditAdvance](ctx, businessId, result.ReferenceId)
		if err != nil {
			tx.Rollback()
			return nil, err
		}

		if err := advance.unUseAmount(result.Amount); err != nil {
			tx.Rollback()
			return nil, err
		}
		// if advance.CurrentStatus == CustomerAdvanceStatusClosed {
		// 	advance.CurrentStatus = CustomerAdvanceStatusConfirmed
		// }

		// advance.UsedAmount = advance.UsedAmount.Sub(result.Amount)
		// advance.RemainingBalance = advance.RemainingBalance.Add(result.Amount)
		// Save the updated advance used amount
		if err := tx.WithContext(ctx).Save(&advance).Error; err != nil {
			tx.Rollback()
			return nil, err
		}

		invoice.InvoiceTotalAdvanceUsedAmount = invoice.InvoiceTotalAdvanceUsedAmount.Sub(invoiceAppliedAmount)
	}

	// update amount and status of bill
	invoice.RemainingBalance = invoice.RemainingBalance.Add(invoiceAppliedAmount)

	if invoice.InvoiceTotalAmount.GreaterThan(invoice.RemainingBalance) {
		invoice.CurrentStatus = SalesInvoiceStatusPartialPaid
	} else {
		invoice.CurrentStatus = SalesInvoiceStatusConfirmed
	}
	// Save the updated bill's credit used amount
	if err := tx.WithContext(ctx).Save(&invoice).Error; err != nil {
		tx.Rollback()
		return nil, err
	}

	err = tx.WithContext(ctx).Delete(&result).Error
	if err != nil {
		tx.Rollback()
		return nil, err
	}

	if result.ReferenceType == CustomerCreditApplyTypeAdvance {
		err = PublishToAccounting(ctx, tx, businessId, result.CreditDate, result.ID, AccountReferenceTypeCustomerAdvanceApplied, nil, result, PubSubMessageActionDelete)
		if err != nil {
			tx.Rollback()
			return nil, err
		}
	} else if result.ReferenceType == CustomerCreditApplyTypeCredit &&
		!result.ExchangeRate.Equals(result.InvoiceExchangeRate) {
		err = PublishToAccounting(ctx, tx, businessId, result.CreditDate, result.ID, AccountReferenceTypeCreditNoteApplied, nil, result, PubSubMessageActionDelete)
		if err != nil {
			tx.Rollback()
			return nil, err
		}
	}

	if err := tx.Commit().Error; err != nil {
		return nil, err
	}

	return result, nil
}
