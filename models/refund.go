package models

import (
	"context"
	"errors"
	"strconv"
	"time"

	"github.com/mmdatafocus/books_backend/config"
	"github.com/mmdatafocus/books_backend/utils"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

type Refund struct {
	ID                    int                 `gorm:"primary_key" json:"id"`
	BusinessId            string              `gorm:"index;not null" json:"business_id"`
	BranchId              int                 `gorm:"not null" json:"branch_id"`
	ReferenceType         RefundReferenceType `gorm:"type:enum('CN','SC','CA','SA','E')" json:"reference_type"`
	ReferenceId           int                 `gorm:"index;not null" json:"reference_id"`
	RefundDate            time.Time           `gorm:"not null" json:"refund_date"`
	Amount                decimal.Decimal     `gorm:"type:decimal(20,4);default:0" json:"amount"`
	ReferenceNumber       string              `gorm:"size:255;default:null" json:"reference_number"`
	Description           string              `gorm:"index;size:100;not null" json:"description"`
	PaymentModeId         int                 `json:"payment_mode_id"`
	AccountId             int                 `gorm:"index;not null" json:"account_id"`
	SupplierId            int                 `json:"supplier_id"`
	CustomerId            int                 `json:"customer_id"`
	CurrencyId            int                 `gorm:"not null" json:"currency_id" binding:"required"`
	ExchangeRate          decimal.Decimal     `gorm:"type:decimal(20,4);default:0" json:"exchange_rate"`
	ReferenceExchangeRate decimal.Decimal     `gorm:"type:decimal(20,4);default:0" json:"reference_exchange_rate"`
	CreatedAt             time.Time           `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt             time.Time           `gorm:"autoUpdateTime" json:"updated_at"`
}

type NewRefund struct {
	BranchId        int                 `json:"branch_id"`
	ReferenceType   RefundReferenceType `json:"reference_type"`
	ReferenceId     int                 `json:"reference_id"`
	RefundDate      time.Time           `json:"refund_date"`
	Amount          decimal.Decimal     `json:"amount"`
	ReferenceNumber string              `json:"reference_number"`
	Description     string              `json:"description"`
	PaymentModeId   int                 `json:"payment_mode_id"`
	AccountId       int                 `json:"account_id"`
	CurrencyId      int                 `json:"currency_id"`
	ExchangeRate    decimal.Decimal     `json:"exchange_rate"`
	SupplierId      int                 `json:"supplier_id"`
	CustomerId      int                 `json:"customer_id"`
}

type RefundReferenceInterface interface {
	GetId() int
	GetRemainingBalance() decimal.Decimal
	AddRefundAmount(amount decimal.Decimal) error
	UpdateStatus() error
	CheckTransactionLock(context.Context) error
	GetDueDate() time.Time
}

func (input NewRefund) validate(ctx context.Context, businessId string, _ int) error {
	// exists supplier
	if err := utils.ValidateResourceId[Account](ctx, businessId, input.AccountId); err != nil {
		return errors.New("account id not found")
	}
	// exists payment mode
	if input.PaymentModeId > 0 {
		if err := utils.ValidateResourceId[PaymentMode](ctx, businessId, input.PaymentModeId); err != nil {
			return errors.New("payment mode id not found")
		}
	}
	// validate RefundDate
	var err error
	switch input.ReferenceType {
	case RefundReferenceTypeCreditNote, RefundReferenceTypeCustomerAdvance:
		if input.CustomerId > 0 {
			if err := utils.ValidateResourceId[Customer](ctx, businessId, input.CustomerId); err != nil {
				return errors.New("customer id not found")
			}
		}
		err = validateTransactionLock(ctx, input.RefundDate, businessId, SalesTransactionLock)
	case RefundReferenceTypeSupplierCredit, RefundReferenceTypeSupplierAdvance:
		if input.SupplierId > 0 {
			if err := utils.ValidateResourceId[Supplier](ctx, businessId, input.SupplierId); err != nil {
				return errors.New("supplier id not found")
			}
		}
		err = validateTransactionLock(ctx, input.RefundDate, businessId, PurchaseTransactionLock)
	}
	if err != nil {
		return err
	}

	return nil
}

func (r Refund) CheckTransactionLock(ctx context.Context) error {
	var err error
	switch r.ReferenceType {
	case RefundReferenceTypeCreditNote, RefundReferenceTypeCustomerAdvance:
		err = validateTransactionLock(ctx, r.RefundDate, r.BusinessId, SalesTransactionLock)
	case RefundReferenceTypeSupplierCredit, RefundReferenceTypeSupplierAdvance:
		err = validateTransactionLock(ctx, r.RefundDate, r.BusinessId, PurchaseTransactionLock)
	}

	return err
}

func processRefund(tx *gorm.DB, ctx context.Context, reference RefundReferenceInterface, amount decimal.Decimal) error {

	if amount.GreaterThan(reference.GetRemainingBalance()) {
		return errors.New("amount must be less than or equal to remaining balance")
	}

	if err := reference.AddRefundAmount(amount); err != nil {
		return err
	}

	if err := reference.UpdateStatus(); err != nil {
		return err
	}

	return tx.WithContext(ctx).Save(reference).Error
}

func getRefundReferenceInterface(tx *gorm.DB, ctx context.Context, referenceId int, referenceType string, businessId string) (RefundReferenceInterface, error) {
	var reference RefundReferenceInterface

	switch referenceType {
	case string(RefundReferenceTypeCreditNote):

		creditNote, err := GetConfirmedCreditNote(tx, ctx, referenceId, businessId)
		if err != nil {
			tx.Rollback()
			return nil, err
		}
		reference = creditNote

	case string(RefundReferenceTypeSupplierCredit):

		supplierCredit, err := GetConfirmedSupplierCredit(tx, ctx, referenceId, businessId)
		if err != nil {
			tx.Rollback()
			return nil, err
		}
		reference = supplierCredit

	case string(RefundReferenceTypeCustomerAdvance):

		customerCreditAdvance, err := GetConfirmedCustomerAdvance(tx, ctx, referenceId, businessId)
		if err != nil {
			tx.Rollback()
			return nil, err
		}
		reference = customerCreditAdvance

	case string(RefundReferenceTypeSupplierAdvance):

		supplierCreditAdvance, err := GetConfirmedSupplierAdvance(tx, ctx, referenceId, businessId)
		if err != nil {
			tx.Rollback()
			return nil, err
		}
		reference = supplierCreditAdvance

	case string(RefundReferenceTypeExpense):

		expense, err := GetExpense(ctx, referenceId)
		if err != nil {
			tx.Rollback()
			return nil, err
		}
		reference = expense

	default:
		tx.Rollback()
		return nil, errors.New("invalid reference type")
	}
	return reference, nil
}

func CreateRefund(ctx context.Context, input *NewRefund) (*Refund, error) {
	businessId, ok := utils.GetBusinessIdFromContext(ctx)
	if !ok || businessId == "" {
		return nil, errors.New("business id is required")
	}

	if err := input.validate(ctx, businessId, 0); err != nil {
		return nil, err
	}

	systemAccounts, err := GetSystemAccounts(businessId)
	if err != nil {
		return nil, err
	}

	db := config.GetDB()
	tx := db.Begin()

	reference, err := getRefundReferenceInterface(tx, ctx, input.ReferenceId, string(input.ReferenceType), businessId)
	if err != nil {
		tx.Rollback()
		return nil, err
	}

	if err := reference.CheckTransactionLock(ctx); err != nil {
		tx.Rollback()
		return nil, err
	}

	if err := processRefund(tx, ctx, reference, input.Amount); err != nil {
		tx.Rollback()
		return nil, err
	}

	exchangeRate := input.ExchangeRate
	referenceExchangeRate := decimal.NewFromInt(0)
	if input.ReferenceType == RefundReferenceTypeCreditNote {
		credit, err := GetResource[CreditNote](ctx, input.ReferenceId)
		if err != nil {
			tx.Rollback()
			return nil, err
		}
		referenceExchangeRate = credit.ExchangeRate
	} else if input.ReferenceType == RefundReferenceTypeCustomerAdvance {
		advance, err := GetResource[CustomerCreditAdvance](ctx, input.ReferenceId)
		if err != nil {
			tx.Rollback()
			return nil, err
		}
		referenceExchangeRate = advance.ExchangeRate
	} else if input.ReferenceType == RefundReferenceTypeSupplierCredit {
		credit, err := GetResource[SupplierCredit](ctx, input.ReferenceId)
		if err != nil {
			tx.Rollback()
			return nil, err
		}
		referenceExchangeRate = credit.ExchangeRate
	} else if input.ReferenceType == RefundReferenceTypeSupplierAdvance {
		advance, err := GetResource[SupplierCreditAdvance](ctx, input.ReferenceId)
		if err != nil {
			tx.Rollback()
			return nil, err
		}
		referenceExchangeRate = advance.ExchangeRate
	}
	if exchangeRate.IsZero() {
		exchangeRate = referenceExchangeRate
	}

	refund := Refund{
		BusinessId:            businessId,
		ReferenceType:         input.ReferenceType,
		ReferenceId:           input.ReferenceId,
		RefundDate:            input.RefundDate,
		Amount:                input.Amount,
		ReferenceNumber:       input.ReferenceNumber,
		Description:           input.Description,
		PaymentModeId:         input.PaymentModeId,
		AccountId:             input.AccountId,
		CustomerId:            input.CustomerId,
		SupplierId:            input.SupplierId,
		CurrencyId:            input.CurrencyId,
		ExchangeRate:          exchangeRate,
		ReferenceExchangeRate: referenceExchangeRate,
	}

	if err := tx.WithContext(ctx).Create(&refund).Error; err != nil {
		tx.Rollback()
		return nil, err
	}

	var detailItems []BankingTransactionDetail
	detailItem := BankingTransactionDetail{
		InvoiceReferenceId: reference.GetId(),
		InvoiceNo:          strconv.Itoa(reference.GetId()),
		DueAmount:          reference.GetRemainingBalance(),
		DueDate:            reference.GetDueDate(),
		PaymentAmount:      input.Amount,
	}
	detailItems = append(detailItems, detailItem)

	bankingTransaction := BankingTransaction{
		BusinessId:        businessId,
		BranchId:          input.BranchId,
		PaymentModeId:     input.PaymentModeId,
		TransactionDate:   input.RefundDate,
		TransactionId:     refund.ID,
		TransactionNumber: strconv.Itoa(refund.ID),
		ExchangeRate:      exchangeRate,
		CurrencyId:        input.CurrencyId,
		Amount:            input.Amount,
		ReferenceNumber:   input.ReferenceNumber,
		Description:       input.Description,
		CustomerId:        input.CustomerId,
		SupplierId:        input.SupplierId,
		Details:           detailItems,
	}

	bankingTransaction.FromAccountAmount = input.Amount
	bankingTransaction.ToAccountAmount = input.Amount

	if input.ReferenceType == RefundReferenceTypeCreditNote {
		bankingTransaction.FromAccountId = input.AccountId
		bankingTransaction.ToAccountId = systemAccounts[AccountCodeAccountsReceivable]
		bankingTransaction.TransactionType = BankingTransactionTypeCreditNoteRefund
		if err := tx.WithContext(ctx).Create(&bankingTransaction).Error; err != nil {
			tx.Rollback()
			return nil, err
		}
		err = PublishToAccounting(ctx, tx, businessId, bankingTransaction.TransactionDate, bankingTransaction.ID, AccountReferenceTypeCreditNoteRefund, bankingTransaction, nil, PubSubMessageActionCreate)
		if err != nil {
			tx.Rollback()
			return nil, err
		}
	} else if input.ReferenceType == RefundReferenceTypeCustomerAdvance {
		bankingTransaction.FromAccountId = input.AccountId
		bankingTransaction.ToAccountId = systemAccounts[AccountCodeUnearnedRevenue]
		bankingTransaction.TransactionType = BankingTransactionTypeCustomerAdvanceRefund
		if err := tx.WithContext(ctx).Create(&bankingTransaction).Error; err != nil {
			tx.Rollback()
			return nil, err
		}
		err = PublishToAccounting(ctx, tx, businessId, bankingTransaction.TransactionDate, bankingTransaction.ID, AccountReferenceTypeCustomerAdvanceRefund, bankingTransaction, nil, PubSubMessageActionCreate)
		if err != nil {
			tx.Rollback()
			return nil, err
		}
	} else if input.ReferenceType == RefundReferenceTypeSupplierAdvance {
		bankingTransaction.FromAccountId = systemAccounts[AccountCodeAdvancePayment]
		bankingTransaction.ToAccountId = input.AccountId
		bankingTransaction.TransactionType = BankingTransactionTypeSupplierAdvanceRefund
		if err := tx.WithContext(ctx).Create(&bankingTransaction).Error; err != nil {
			tx.Rollback()
			return nil, err
		}
		err = PublishToAccounting(ctx, tx, businessId, bankingTransaction.TransactionDate, bankingTransaction.ID, AccountReferenceTypeSupplierAdvanceRefund, bankingTransaction, nil, PubSubMessageActionCreate)
		if err != nil {
			tx.Rollback()
			return nil, err
		}
	} else if input.ReferenceType == RefundReferenceTypeSupplierCredit {
		bankingTransaction.FromAccountId = systemAccounts[AccountCodeAccountsPayable]
		bankingTransaction.ToAccountId = input.AccountId
		bankingTransaction.TransactionType = BankingTransactionTypeSupplierCreditRefund
		if err := tx.WithContext(ctx).Create(&bankingTransaction).Error; err != nil {
			tx.Rollback()
			return nil, err
		}
		err = PublishToAccounting(ctx, tx, businessId, bankingTransaction.TransactionDate, bankingTransaction.ID, AccountReferenceTypeSupplierCreditRefund, bankingTransaction, nil, PubSubMessageActionCreate)
		if err != nil {
			tx.Rollback()
			return nil, err
		}
	}

	if err := tx.Commit().Error; err != nil {
		return nil, err
	}

	return &refund, nil
}

func UpdateRefund(ctx context.Context, refundId int, input *NewRefund) (*Refund, error) {
	businessId, ok := utils.GetBusinessIdFromContext(ctx)
	if !ok || businessId == "" {
		return nil, errors.New("business id is required")
	}

	existingRefund, err := utils.FetchModelForChange[Refund](ctx, businessId, refundId)
	if err != nil {
		return nil, err
	}

	db := config.GetDB()
	tx := db.Begin()

	// Calculate the difference in the refund amount
	amountDifference := input.Amount.Sub(existingRefund.Amount)

	reference, err := getRefundReferenceInterface(tx, ctx, input.ReferenceId, string(input.ReferenceType), businessId)
	if err != nil {
		tx.Rollback()
		return nil, err
	}
	if err := reference.CheckTransactionLock(ctx); err != nil {
		tx.Rollback()
		return nil, err
	}

	if err := processRefund(tx, ctx, reference, amountDifference); err != nil {
		tx.Rollback()
		return nil, err
	}

	// Update the refund record
	existingRefund.Amount = input.Amount
	existingRefund.RefundDate = input.RefundDate
	existingRefund.ReferenceNumber = input.ReferenceNumber
	existingRefund.Description = input.Description
	existingRefund.PaymentModeId = input.PaymentModeId
	existingRefund.AccountId = input.AccountId
	existingRefund.SupplierId = input.SupplierId
	existingRefund.CustomerId = input.CustomerId
	existingRefund.CurrencyId = input.CurrencyId

	exchangeRate := input.ExchangeRate
	referenceExchangeRate := decimal.NewFromInt(0)
	if input.ReferenceType == RefundReferenceTypeCreditNote {
		credit, err := GetResource[CreditNote](ctx, input.ReferenceId)
		if err != nil {
			tx.Rollback()
			return nil, err
		}
		referenceExchangeRate = credit.ExchangeRate
	} else if input.ReferenceType == RefundReferenceTypeCustomerAdvance {
		advance, err := GetResource[CustomerCreditAdvance](ctx, input.ReferenceId)
		if err != nil {
			tx.Rollback()
			return nil, err
		}
		referenceExchangeRate = advance.ExchangeRate
	} else if input.ReferenceType == RefundReferenceTypeSupplierCredit {
		credit, err := GetResource[SupplierCredit](ctx, input.ReferenceId)
		if err != nil {
			tx.Rollback()
			return nil, err
		}
		referenceExchangeRate = credit.ExchangeRate
	} else if input.ReferenceType == RefundReferenceTypeSupplierAdvance {
		advance, err := GetResource[SupplierCreditAdvance](ctx, input.ReferenceId)
		if err != nil {
			tx.Rollback()
			return nil, err
		}
		referenceExchangeRate = advance.ExchangeRate
	}
	if exchangeRate.IsZero() {
		exchangeRate = referenceExchangeRate
	}
	existingRefund.ExchangeRate = exchangeRate
	existingRefund.ReferenceExchangeRate = referenceExchangeRate

	if err := tx.WithContext(ctx).Save(&existingRefund).Error; err != nil {
		tx.Rollback()
		return nil, err
	}

	if existingRefund.ReferenceType == RefundReferenceTypeCreditNote {
		var oldBankingTransaction BankingTransaction
		var bankingTransaction BankingTransaction
		err := tx.WithContext(ctx).Where("transaction_id = ? AND transaction_type = ?", existingRefund.ID, BankingTransactionTypeCreditNoteRefund).First(&oldBankingTransaction).Error
		if err != nil {
			tx.Rollback()
			return nil, err
		}
		err = tx.WithContext(ctx).Where("transaction_id = ? AND transaction_type = ?", existingRefund.ID, BankingTransactionTypeCreditNoteRefund).First(&bankingTransaction).Error
		if err != nil {
			tx.Rollback()
			return nil, err
		}
		bankingTransaction.Amount = input.Amount
		bankingTransaction.TransactionDate = input.RefundDate
		bankingTransaction.ReferenceNumber = input.ReferenceNumber
		bankingTransaction.Description = input.Description
		bankingTransaction.PaymentModeId = input.PaymentModeId
		bankingTransaction.FromAccountId = input.AccountId
		bankingTransaction.SupplierId = input.SupplierId
		bankingTransaction.CustomerId = input.CustomerId
		bankingTransaction.CurrencyId = input.CurrencyId
		bankingTransaction.ExchangeRate = input.ExchangeRate

		err = tx.WithContext(ctx).Save(&bankingTransaction).Error
		if err != nil {
			tx.Rollback()
			return nil, err
		}

		err = PublishToAccounting(ctx, tx, businessId, bankingTransaction.TransactionDate, bankingTransaction.ID, AccountReferenceTypeCreditNoteRefund, bankingTransaction, oldBankingTransaction, PubSubMessageActionUpdate)
		if err != nil {
			tx.Rollback()
			return nil, err
		}
	} else if existingRefund.ReferenceType == RefundReferenceTypeCustomerAdvance {
		var oldBankingTransaction BankingTransaction
		var bankingTransaction BankingTransaction
		err := tx.WithContext(ctx).Where("transaction_id = ? AND transaction_type = ?", existingRefund.ID, BankingTransactionTypeCustomerAdvanceRefund).First(&oldBankingTransaction).Error
		if err != nil {
			tx.Rollback()
			return nil, err
		}
		err = tx.WithContext(ctx).Where("transaction_id = ? AND transaction_type = ?", existingRefund.ID, BankingTransactionTypeCustomerAdvanceRefund).First(&bankingTransaction).Error
		if err != nil {
			tx.Rollback()
			return nil, err
		}
		bankingTransaction.Amount = input.Amount
		bankingTransaction.TransactionDate = input.RefundDate
		bankingTransaction.ReferenceNumber = input.ReferenceNumber
		bankingTransaction.Description = input.Description
		bankingTransaction.PaymentModeId = input.PaymentModeId
		bankingTransaction.FromAccountId = input.AccountId
		bankingTransaction.SupplierId = input.SupplierId
		bankingTransaction.CustomerId = input.CustomerId
		bankingTransaction.CurrencyId = input.CurrencyId
		bankingTransaction.ExchangeRate = input.ExchangeRate

		err = tx.WithContext(ctx).Save(&bankingTransaction).Error
		if err != nil {
			tx.Rollback()
			return nil, err
		}

		err = PublishToAccounting(ctx, tx, businessId, bankingTransaction.TransactionDate, bankingTransaction.ID, AccountReferenceTypeCustomerAdvanceRefund, bankingTransaction, oldBankingTransaction, PubSubMessageActionUpdate)
		if err != nil {
			tx.Rollback()
			return nil, err
		}
	} else if existingRefund.ReferenceType == RefundReferenceTypeSupplierAdvance {
		var oldBankingTransaction BankingTransaction
		var bankingTransaction BankingTransaction
		err := tx.WithContext(ctx).Where("transaction_id = ? AND transaction_type = ?", existingRefund.ID, BankingTransactionTypeSupplierAdvanceRefund).First(&oldBankingTransaction).Error
		if err != nil {
			tx.Rollback()
			return nil, err
		}
		err = tx.WithContext(ctx).Where("transaction_id = ? AND transaction_type = ?", existingRefund.ID, BankingTransactionTypeSupplierAdvanceRefund).First(&bankingTransaction).Error
		if err != nil {
			tx.Rollback()
			return nil, err
		}
		bankingTransaction.Amount = input.Amount
		bankingTransaction.TransactionDate = input.RefundDate
		bankingTransaction.ReferenceNumber = input.ReferenceNumber
		bankingTransaction.Description = input.Description
		bankingTransaction.PaymentModeId = input.PaymentModeId
		bankingTransaction.ToAccountId = input.AccountId
		bankingTransaction.SupplierId = input.SupplierId
		bankingTransaction.CustomerId = input.CustomerId
		bankingTransaction.CurrencyId = input.CurrencyId
		bankingTransaction.ExchangeRate = input.ExchangeRate

		err = tx.WithContext(ctx).Save(&bankingTransaction).Error
		if err != nil {
			tx.Rollback()
			return nil, err
		}

		err = PublishToAccounting(ctx, tx, businessId, bankingTransaction.TransactionDate, bankingTransaction.ID, AccountReferenceTypeSupplierAdvanceRefund, bankingTransaction, oldBankingTransaction, PubSubMessageActionUpdate)
		if err != nil {
			tx.Rollback()
			return nil, err
		}
	} else if existingRefund.ReferenceType == RefundReferenceTypeSupplierCredit {
		var oldBankingTransaction BankingTransaction
		var bankingTransaction BankingTransaction
		err := tx.WithContext(ctx).Where("transaction_id = ? AND transaction_type = ?", existingRefund.ID, BankingTransactionTypeSupplierCreditRefund).First(&oldBankingTransaction).Error
		if err != nil {
			tx.Rollback()
			return nil, err
		}
		err = tx.WithContext(ctx).Where("transaction_id = ? AND transaction_type = ?", existingRefund.ID, BankingTransactionTypeSupplierCreditRefund).First(&bankingTransaction).Error
		if err != nil {
			tx.Rollback()
			return nil, err
		}

		bankingTransaction.Amount = input.Amount
		bankingTransaction.TransactionDate = input.RefundDate
		bankingTransaction.ReferenceNumber = input.ReferenceNumber
		bankingTransaction.Description = input.Description
		bankingTransaction.PaymentModeId = input.PaymentModeId
		bankingTransaction.ToAccountId = input.AccountId
		bankingTransaction.SupplierId = input.SupplierId
		bankingTransaction.CustomerId = input.CustomerId
		bankingTransaction.CurrencyId = input.CurrencyId
		bankingTransaction.ExchangeRate = input.ExchangeRate

		err = tx.WithContext(ctx).Save(&bankingTransaction).Error
		if err != nil {
			tx.Rollback()
			return nil, err
		}

		err = PublishToAccounting(ctx, tx, businessId, bankingTransaction.TransactionDate, bankingTransaction.ID, AccountReferenceTypeSupplierCreditRefund, bankingTransaction, oldBankingTransaction, PubSubMessageActionUpdate)
		if err != nil {
			tx.Rollback()
			return nil, err
		}
	}

	if err := tx.Commit().Error; err != nil {
		return nil, err
	}

	return existingRefund, nil
}

func DeleteRefund(ctx context.Context, refundId int) (*Refund, error) {
	businessId, ok := utils.GetBusinessIdFromContext(ctx)
	if !ok || businessId == "" {
		return nil, errors.New("business id is required")
	}

	// Retrieve the existing refund
	existingRefund, err := utils.FetchModelForChange[Refund](ctx, businessId, refundId)
	if err != nil {
		return nil, err
	}

	db := config.GetDB()
	tx := db.Begin()

	reference, err := getRefundReferenceInterface(tx, ctx, existingRefund.ReferenceId, string(existingRefund.ReferenceType), businessId)
	if err != nil {
		tx.Rollback()
		return nil, err
	}

	if err := reference.CheckTransactionLock(ctx); err != nil {
		tx.Rollback()
		return nil, err
	}

	// Subtract the refund amount from the referenced entity
	if err := reference.AddRefundAmount(existingRefund.Amount.Neg()); err != nil {
		tx.Rollback()
		return nil, err
	}

	if err := reference.UpdateStatus(); err != nil {
		tx.Rollback()
		return nil, err
	}

	if err := tx.WithContext(ctx).Save(reference).Error; err != nil {
		tx.Rollback()
		return nil, err
	}

	// Delete the refund record
	if err := tx.WithContext(ctx).Delete(&existingRefund).Error; err != nil {
		tx.Rollback()
		return nil, err
	}

	if existingRefund.ReferenceType == RefundReferenceTypeCreditNote {
		var oldBankingTransaction BankingTransaction
		err := tx.WithContext(ctx).Where("transaction_id = ? AND transaction_type = ?", existingRefund.ID, BankingTransactionTypeCreditNoteRefund).First(&oldBankingTransaction).Error
		if err != nil {
			tx.Rollback()
			return nil, err
		}
		// err = tx.WithContext(ctx).Delete(&oldBankingTransaction).Error
		// if err != nil {
		// 	tx.Rollback()
		// 	return nil, err
		// }
		if err := oldBankingTransaction.Delete(tx, ctx); err != nil {
			tx.Rollback()
			return nil, err
		}
		err = PublishToAccounting(ctx, tx, businessId, oldBankingTransaction.TransactionDate, oldBankingTransaction.ID, AccountReferenceTypeCreditNoteRefund, nil, oldBankingTransaction, PubSubMessageActionDelete)
		if err != nil {
			tx.Rollback()
			return nil, err
		}
	} else if existingRefund.ReferenceType == RefundReferenceTypeCustomerAdvance {
		var oldBankingTransaction BankingTransaction
		err := tx.WithContext(ctx).Where("transaction_id = ? AND transaction_type = ?", existingRefund.ID, BankingTransactionTypeCustomerAdvanceRefund).First(&oldBankingTransaction).Error
		if err != nil {
			tx.Rollback()
			return nil, err
		}
		// err = tx.WithContext(ctx).Delete(&oldBankingTransaction).Error
		// if err != nil {
		// 	tx.Rollback()
		// 	return nil, err
		// }
		if err := oldBankingTransaction.Delete(tx, ctx); err != nil {
			tx.Rollback()
			return nil, err
		}
		err = PublishToAccounting(ctx, tx, businessId, oldBankingTransaction.TransactionDate, oldBankingTransaction.ID, AccountReferenceTypeCustomerAdvanceRefund, nil, oldBankingTransaction, PubSubMessageActionDelete)
		if err != nil {
			tx.Rollback()
			return nil, err
		}
	} else if existingRefund.ReferenceType == RefundReferenceTypeSupplierAdvance {
		var oldBankingTransaction BankingTransaction
		err := tx.WithContext(ctx).Where("transaction_id = ? AND transaction_type = ?", existingRefund.ID, BankingTransactionTypeSupplierAdvanceRefund).First(&oldBankingTransaction).Error
		if err != nil {
			tx.Rollback()
			return nil, err
		}
		if err := oldBankingTransaction.Delete(tx, ctx); err != nil {
			tx.Rollback()
			return nil, err
		}
		// err = tx.WithContext(ctx).Select("Details.*").Delete(&oldBankingTransaction).Error
		// if err != nil {
		// 	tx.Rollback()
		// 	return nil, err
		// }
		err = PublishToAccounting(ctx, tx, businessId, oldBankingTransaction.TransactionDate, oldBankingTransaction.ID, AccountReferenceTypeSupplierAdvanceRefund, nil, oldBankingTransaction, PubSubMessageActionDelete)
		if err != nil {
			tx.Rollback()
			return nil, err
		}
	} else if existingRefund.ReferenceType == RefundReferenceTypeSupplierCredit {
		var oldBankingTransaction BankingTransaction
		err := tx.WithContext(ctx).Where("transaction_id = ? AND transaction_type = ?", existingRefund.ID, BankingTransactionTypeSupplierCreditRefund).First(&oldBankingTransaction).Error
		if err != nil {
			tx.Rollback()
			return nil, err
		}
		if err := oldBankingTransaction.Delete(tx, ctx); err != nil {
			tx.Rollback()
			return nil, err
		}
		// err = tx.WithContext(ctx).Delete(&oldBankingTransaction).Error
		// if err != nil {
		// 	tx.Rollback()
		// 	return nil, err
		// }
		err = PublishToAccounting(ctx, tx, businessId, oldBankingTransaction.TransactionDate, oldBankingTransaction.ID, AccountReferenceTypeSupplierCreditRefund, nil, oldBankingTransaction, PubSubMessageActionDelete)
		if err != nil {
			tx.Rollback()
			return nil, err
		}
	}

	if err := tx.Commit().Error; err != nil {
		return nil, err
	}

	return existingRefund, nil
}
