package models

import (
	"context"
	"errors"
	"time"

	"bitbucket.org/mmdatafocus/books_backend/config"
	"bitbucket.org/mmdatafocus/books_backend/utils"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

type BankingTransaction struct {
	ID                        int                        `gorm:"primary_key" json:"id"`
	BusinessId                string                     `gorm:"index" json:"business_id"  binding:"required"`
	BranchId                  int                        `gorm:"index" json:"branch_id"`
	TransactionDate           time.Time                  `gorm:"not null" json:"transaction_date" binding:"required"`
	Amount                    decimal.Decimal            `gorm:"type:decimal(20,4);default:0" json:"amount"`
	ReferenceNumber           string                     `gorm:"size:255;default:null" json:"reference_number"`
	Description               string                     `gorm:"type:text;default:null" json:"description"`
	TransactionNumber         string                     `gorm:"size:255;default:null" json:"transaction_number"`
	TransactionId             int                        `json:"transaction_id"`
	TransactionType           BankingTransactionType     `gorm:"not null;type:enum('Expense','SupplierAdvance','SupplierPayment','TransferToAnotherAccount','SalesReturn','CardPayment','OwnerDrawings','DepositToOtherAccounts','CreditNoteRefund','CustomerAdvanceRefund','EmployeeReimbursement','CustomerAdvance','CustomerPayment','SalesWithoutInvoices','TransferFromAnotherAccounts','InterestIncome','OtherIncome','ExpenseRefund','DepositFromOtherAccounts','OwnerContribution','SupplierCreditRefund','SupplierAdvanceRefund','ManualJournal','OpeningBalance');" json:"transaction_type"`
	FromAccountId             int                        `gorm:"index" json:"from_account_id"`
	FromAccountAmount         decimal.Decimal            `gorm:"type:decimal(20,4);default:0" json:"from_account_amount"`
	ToAccountId               int                        `gorm:"index" json:"to_account_id"`
	ToAccountAmount           decimal.Decimal            `gorm:"type:decimal(20,4);default:0" json:"to_account_amount"`
	ExchangeRate              decimal.Decimal            `gorm:"type:decimal(20,4);default:0" json:"exchange_rate"`
	CurrencyId                int                        `gorm:"index;not null" json:"currency_id"`
	PaymentModeId             int                        `gorm:"index" json:"payment_mode_id"`
	BankCharges               decimal.Decimal            `gorm:"type:decimal(20,4);default:0" json:"bank_charges"`
	TaxAmount                 decimal.Decimal            `gorm:"type:decimal(20,4);default:0" json:"tax_amount"`
	SupplierId                int                        `gorm:"index" json:"supplier_id"`
	CustomerId                int                        `gorm:"index" json:"customer_id"`
	Details                   []BankingTransactionDetail `gorm:"foreignKey:BankingTransactionId" json:"details"`
	CreditAdvanceId           int                        `gorm:"default:0" json:"credit_advance_id"`
	FromAccountClosingBalance decimal.Decimal            `gorm:"type:decimal(20,4);default:0" json:"from_account_closing_balance"`
	ToAccountClosingBalance   decimal.Decimal            `gorm:"type:decimal(20,4);default:0" json:"to_account_closing_balance"`
	// PaidAccountId     			int             `gorm:"index" json:"paid_account_id"`
	// ReceivedAccountId   			int             `gorm:"index" json:"received_account_id"`
	// ReceivedFromCustomerId   	int          	`gorm:"index" json:"received_from_customer_id"`
	// to account DepositToAccountId int          	`gorm:"index" json:"deposit_to_account_id"`
	// to account RevenueAccountId   int          	`gorm:"index" json:"revenue_to_account_id"`
	// EmployeeId  					int          	`gorm:"index" json:"employee_id"`
	Documents []*Document `gorm:"polymorphic:Reference" json:"documents"`
	CreatedAt time.Time   `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt time.Time   `gorm:"autoUpdateTime" json:"updated_at"`
}

type NewBankingTransaction struct {
	BranchId          int                           `json:"branchId"`
	TransactionDate   time.Time                     `json:"transaction_date"`
	Amount            decimal.Decimal               `json:"amount"`
	ReferenceNumber   string                        `json:"referenceNumber,omitempty"`
	Description       string                        `json:"description,omitempty"`
	IsMoneyIn         *bool                         `json:"is_money_in"`
	TransactionId     int                           `json:"transaction_id"`
	TransactionNumber string                        `json:"transaction_number"`
	TransactionType   BankingTransactionType        `json:"transaction_type"`
	FromAccountId     int                           `json:"from_account_id,omitempty"`
	ToAccountId       int                           `json:"to_account_id"`
	PaymentModeId     int                           `json:"payment_mode_id"`
	CurrencyId        int                           `json:"currency_id"`
	ExchangeRate      decimal.Decimal               `json:"exchangeRate,omitempty"`
	BankCharges       decimal.Decimal               `json:"bank_charges"`
	TaxAmount         decimal.Decimal               `json:"tax_amount"`
	CustomerId        int                           `json:"customer_id"`
	SupplierId        int                           `json:"supplier_id"`
	Documents         []*NewDocument                `json:"documents,omitempty"`
	Details           []NewBankingTransactionDetail `json:"details"`
}

type BankingTransactionDetail struct {
	ID                   int             `gorm:"primary_key" json:"id"`
	BankingTransactionId int             `gorm:"index" json:"banking_transaction_id"`
	InvoiceReferenceId   int             `json:"invoice_reference_id"`
	InvoiceNo            string          `json:"invoice_no"`
	DueAmount            decimal.Decimal `gorm:"type:decimal(20,4);default:0" json:"due_amount"`
	PaymentAmount        decimal.Decimal `gorm:"type:decimal(20,4);default:0" json:"payment_amount"`
	DueDate              time.Time       `json:"due_date" binding:"required"`
	CreatedAt            time.Time       `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt            time.Time       `gorm:"autoUpdateTime" json:"updated_at"`
}

func (d BankingTransactionDetail) GetId() int {
	return d.ID
}

func (d BankingTransactionDetail) fillable() map[string]interface{} {
	return map[string]interface{}{
		"InvoiceReferenceId": d.InvoiceReferenceId,
		"InvoiceNo":          d.InvoiceNo,
		"DueAmount":          d.DueAmount,
		"PaymentAmount":      d.PaymentAmount,
		"DueDate":            d.DueDate,
	}
}

type NewBankingTransactionDetail struct {
	HasId
	InvoiceReferenceId string          `json:"invoice_reference_id"`
	InvoiceNo          string          `json:"invoice_no"`
	DueAmount          decimal.Decimal `json:"due_amount"`
	PaymentAmount      decimal.Decimal `json:"payment_amount"`
	DueDate            time.Time       `json:"due_date"`
}

func (bt BankingTransaction) CheckTransactionLock(ctx context.Context) error {
	return validateTransactionLock(ctx, bt.TransactionDate, bt.BusinessId, BankingTransactionLock)
}

func (bt BankingTransaction) Delete(tx *gorm.DB, ctx context.Context) error {
	if err := tx.WithContext(ctx).Model(&bt).Association("Details").Unscoped().Clear(); err != nil {
		return err
	}
	if err := tx.WithContext(ctx).Delete(&bt).Error; err != nil {
		return err
	}
	return nil
}

type BankingTransactionsConnection struct {
	Edges    []*BankingTransactionsEdge `json:"edges"`
	PageInfo *PageInfo                  `json:"pageInfo"`
}

type BankingTransactionsEdge Edge[BankingTransaction]

// returns decoded curosr string
func (bt BankingTransaction) GetCursor() string {
	return bt.TransactionDate.String()
}

func (b BankingTransaction) GetId() int {
	return b.ID
}

func (input NewBankingTransaction) validate(ctx context.Context, businessId string) error {
	// exists
	if err := utils.ValidateResourceId[Branch](ctx, businessId, input.BranchId); err != nil {
		return errors.New("branch id not found")
	}
	if input.CurrencyId > 0 {
		if err := utils.ValidateResourceId[Currency](ctx, businessId, input.CurrencyId); err != nil {
			return errors.New("currency id not found")
		}
	}
	if input.FromAccountId > 0 {
		if err := utils.ValidateResourceId[Account](ctx, businessId, input.FromAccountId); err != nil {
			return errors.New("from account id not found")
		}
	}
	if input.ToAccountId > 0 {
		if err := utils.ValidateResourceId[Account](ctx, businessId, input.ToAccountId); err != nil {
			return errors.New("to account id not found")
		}
	}
	if input.CustomerId > 0 {
		if err := utils.ValidateResourceId[Customer](ctx, businessId, input.CustomerId); err != nil {
			return errors.New("customer id not found")
		}
	}
	if input.SupplierId > 0 {
		if err := utils.ValidateResourceId[Supplier](ctx, businessId, input.SupplierId); err != nil {
			return errors.New("supplier id not found")
		}
	}

	// validate TransactionDate
	if err := validateTransactionLock(ctx, input.TransactionDate, businessId, BankingTransactionLock); err != nil {
		return err
	}

	// validate if account-to-account transfer
	if input.TransactionType == BankingTransactionTypeTransferFromAnotherAccounts ||
		input.TransactionType == BankingTransactionTypeTransferToAnotherAccount ||
		input.TransactionType == BankingTransactionTypeDepositFromOtherAccounts ||
		input.TransactionType == BankingTransactionTypeDepositToOtherAccounts ||
		input.TransactionType == BankingTransactionTypeOwnerContribution ||
		input.TransactionType == BankingTransactionTypeOwnerDrawings {

		fromAccount, err := utils.FetchModel[Account](ctx, businessId, input.FromAccountId)
		if err != nil {
			return errors.New("from account id not found")
		}

		toAccount, err := utils.FetchModel[Account](ctx, businessId, input.ToAccountId)
		if err != nil {
			return errors.New("to account id not found")
		}

		business, err := GetBusiness(ctx)
		if err != nil {
			return errors.New("business not found")
		}

		if fromAccount.CurrencyId != 0 && fromAccount.CurrencyId != business.BaseCurrencyId {
			if toAccount.CurrencyId != 0 && toAccount.CurrencyId != business.BaseCurrencyId {
				if fromAccount.CurrencyId != toAccount.CurrencyId {
					return errors.New("cannot transfer to different currency except base currency")
				}
			}
		}
	}

	return nil
}

func CreateBankingTransaction(ctx context.Context, input *NewBankingTransaction) (*BankingTransaction, error) {
	businessId, ok := utils.GetBusinessIdFromContext(ctx)
	if !ok || businessId == "" {
		return nil, errors.New("business id is required")
	}

	if err := input.validate(ctx, businessId); err != nil {
		return nil, err
	}
	documents, err := mapNewDocuments(input.Documents, "banking_transactions", 0)
	if err != nil {
		return nil, err
	}

	var detailItems []BankingTransactionDetail
	for _, item := range input.Details {

		detailItem := BankingTransactionDetail{
			InvoiceNo:     item.InvoiceNo,
			DueAmount:     item.DueAmount,
			DueDate:       item.DueDate,
			PaymentAmount: item.PaymentAmount,
		}
		detailItems = append(detailItems, detailItem)
	}

	bankingTransaction := BankingTransaction{
		BusinessId:        businessId,
		BranchId:          input.BranchId,
		FromAccountId:     input.FromAccountId,
		ToAccountId:       input.ToAccountId,
		CustomerId:        input.CustomerId,
		SupplierId:        input.SupplierId,
		PaymentModeId:     input.PaymentModeId,
		TransactionDate:   input.TransactionDate,
		TransactionId:     input.TransactionId,
		TransactionNumber: input.TransactionNumber,
		TransactionType:   input.TransactionType,
		ExchangeRate:      input.ExchangeRate,
		CurrencyId:        input.CurrencyId,
		Amount:            input.Amount,
		TaxAmount:         input.TaxAmount,
		BankCharges:       input.BankCharges,
		ReferenceNumber:   input.ReferenceNumber,
		Description:       input.Description,
		Documents:         documents,
		Details:           detailItems,
	}

	if bankingTransaction.TransactionType == BankingTransactionTypeTransferFromAnotherAccounts ||
		//bankingTransaction.TransactionType == BankingTransactionTypeTransferToAnotherAccount ||
		bankingTransaction.TransactionType == BankingTransactionTypeDepositFromOtherAccounts ||
		bankingTransaction.TransactionType == BankingTransactionTypeTransferToAnotherAccount {

		bankingTransaction.FromAccountAmount = input.Amount.Add(input.BankCharges).Add(input.TaxAmount)
		bankingTransaction.ToAccountAmount = input.Amount
	} else {
		bankingTransaction.FromAccountAmount = input.Amount
		bankingTransaction.ToAccountAmount = input.Amount.Sub(input.BankCharges).Sub(input.TaxAmount)
	}

	if bankingTransaction.TransactionType == BankingTransactionTypeSupplierAdvance {
		systemAccounts, err := GetSystemAccounts(businessId)
		if err != nil {
			return nil, err
		}
		bankingTransaction.ToAccountId = systemAccounts[AccountCodeAdvancePayment] // advance payment
	} else if bankingTransaction.TransactionType == BankingTransactionTypeCustomerAdvance {
		systemAccounts, err := GetSystemAccounts(businessId)
		if err != nil {
			return nil, err
		}
		bankingTransaction.FromAccountId = systemAccounts[AccountCodeUnearnedRevenue] // unearned revenue
	}

	db := config.GetDB()
	// db action
	tx := db.Begin()
	err = tx.WithContext(ctx).Create(&bankingTransaction).Error
	if err != nil {
		tx.Rollback()
		return nil, err
	}

	if bankingTransaction.TransactionType == BankingTransactionTypeTransferFromAnotherAccounts ||
		bankingTransaction.TransactionType == BankingTransactionTypeTransferToAnotherAccount {
		err = PublishToAccounting(ctx, tx, businessId, bankingTransaction.TransactionDate, bankingTransaction.ID, AccountReferenceTypeAccountTransfer, bankingTransaction, nil, PubSubMessageActionCreate)
		if err != nil {
			tx.Rollback()
			return nil, err
		}
	} else if bankingTransaction.TransactionType == BankingTransactionTypeDepositFromOtherAccounts ||
		bankingTransaction.TransactionType == BankingTransactionTypeDepositToOtherAccounts {
		err = PublishToAccounting(ctx, tx, businessId, bankingTransaction.TransactionDate, bankingTransaction.ID, AccountReferenceTypeAccountDeposit, bankingTransaction, nil, PubSubMessageActionCreate)
		if err != nil {
			tx.Rollback()
			return nil, err
		}
	} else if bankingTransaction.TransactionType == BankingTransactionTypeOwnerContribution {
		err = PublishToAccounting(ctx, tx, businessId, bankingTransaction.TransactionDate, bankingTransaction.ID, AccountReferenceTypeOwnerContribution, bankingTransaction, nil, PubSubMessageActionCreate)
		if err != nil {
			tx.Rollback()
			return nil, err
		}
	} else if bankingTransaction.TransactionType == BankingTransactionTypeOwnerDrawings {
		err = PublishToAccounting(ctx, tx, businessId, bankingTransaction.TransactionDate, bankingTransaction.ID, AccountReferenceTypeOwnerDrawing, bankingTransaction, nil, PubSubMessageActionCreate)
		if err != nil {
			tx.Rollback()
			return nil, err
		}
	} else if bankingTransaction.TransactionType == BankingTransactionTypeOtherIncome ||
		bankingTransaction.TransactionType == BankingTransactionTypeInterestIncome {

		err = PublishToAccounting(ctx, tx, businessId, bankingTransaction.TransactionDate, bankingTransaction.ID, AccountReferenceTypeOtherIncome, bankingTransaction, nil, PubSubMessageActionCreate)
		if err != nil {
			tx.Rollback()
			return nil, err
		}
	} else if bankingTransaction.TransactionType == BankingTransactionTypeSupplierAdvance {
		err = PublishToAccounting(ctx, tx, businessId, bankingTransaction.TransactionDate, bankingTransaction.ID, AccountReferenceTypeAdvanceSupplierPayment, bankingTransaction, nil, PubSubMessageActionCreate)
		if err != nil {
			tx.Rollback()
			return nil, err
		}
	} else if bankingTransaction.TransactionType == BankingTransactionTypeCustomerAdvance {
		err = PublishToAccounting(ctx, tx, businessId, bankingTransaction.TransactionDate, bankingTransaction.ID, AccountReferenceTypeAdvanceCustomerPayment, bankingTransaction, nil, PubSubMessageActionCreate)
		if err != nil {
			tx.Rollback()
			return nil, err
		}
	}

	if err := tx.Commit().Error; err != nil {
		return nil, err
	}

	return &bankingTransaction, nil
}

func UpdateBankingTransaction(ctx context.Context, id int, input *NewBankingTransaction) (*BankingTransaction, error) {

	businessId, ok := utils.GetBusinessIdFromContext(ctx)
	if !ok || businessId == "" {
		return nil, errors.New("business id is required")
	}

	if err := input.validate(ctx, businessId); err != nil {
		return nil, err
	}

	oldBankingTransaction, err := utils.FetchModelForChange[BankingTransaction](ctx, businessId, id)
	if err != nil {
		return nil, err
	}

	bankingTransaction := BankingTransaction{
		ID:                id,
		BusinessId:        businessId,
		BranchId:          input.BranchId,
		FromAccountId:     input.FromAccountId,
		ToAccountId:       input.ToAccountId,
		CustomerId:        input.CustomerId,
		SupplierId:        input.SupplierId,
		PaymentModeId:     input.PaymentModeId,
		TransactionDate:   input.TransactionDate,
		TransactionId:     input.TransactionId,
		TransactionNumber: input.TransactionNumber,
		TransactionType:   input.TransactionType,
		ExchangeRate:      input.ExchangeRate,
		CurrencyId:        input.CurrencyId,
		Amount:            input.Amount,
		TaxAmount:         input.TaxAmount,
		BankCharges:       input.BankCharges,
		ReferenceNumber:   input.ReferenceNumber,
		Description:       input.Description,
	}

	if bankingTransaction.TransactionType == BankingTransactionTypeTransferFromAnotherAccounts ||
		//bankingTransaction.TransactionType == BankingTransactionTypeTransferToAnotherAccount ||
		bankingTransaction.TransactionType == BankingTransactionTypeDepositFromOtherAccounts ||
		bankingTransaction.TransactionType == BankingTransactionTypeTransferToAnotherAccount {

		bankingTransaction.FromAccountAmount = input.Amount.Add(input.BankCharges).Add(input.TaxAmount)
		bankingTransaction.ToAccountAmount = input.Amount
	} else {
		bankingTransaction.ToAccountAmount = input.Amount.Add(input.BankCharges).Add(input.TaxAmount)
		bankingTransaction.FromAccountAmount = input.Amount
	}

	// validate if advance supplier payment
	if input.TransactionType == BankingTransactionTypeSupplierAdvance && oldBankingTransaction.CreditAdvanceId > 0 {
		advance, err := utils.FetchModelForChange[SupplierCreditAdvance](ctx, businessId, oldBankingTransaction.CreditAdvanceId)
		if err != nil {
			return nil, err
		}
		if advance.BranchId != input.BranchId || advance.SupplierId != input.SupplierId || advance.CurrencyId != input.CurrencyId {
			return nil, errors.New("branch or supplier or currency update is not allowed")
		}
		if input.Amount.LessThan(advance.Amount) && advance.Amount.Sub(input.Amount).GreaterThan(advance.RemainingBalance) {
			return nil, errors.New("advance amount cannot be less than used/refund amount")
		}
		if !input.ExchangeRate.Equal(advance.ExchangeRate) && advance.isUsed() {
			return nil, errors.New("cannot update exchage rate if advance is used")
		}
		bankingTransaction.CreditAdvanceId = oldBankingTransaction.CreditAdvanceId
	}
	// validate if advance customer payment
	if input.TransactionType == BankingTransactionTypeCustomerAdvance && oldBankingTransaction.CreditAdvanceId > 0 {
		advance, err := utils.FetchModelForChange[CustomerCreditAdvance](ctx, businessId, oldBankingTransaction.CreditAdvanceId)
		if err != nil {
			return nil, err
		}
		if advance.BranchId != input.BranchId || advance.CustomerId != input.CustomerId || advance.CurrencyId != input.CurrencyId {
			return nil, errors.New("branch or customer or currency update is not allowed")
		}
		if input.Amount.LessThan(advance.Amount) && advance.Amount.Sub(input.Amount).GreaterThan(advance.RemainingBalance) {
			return nil, errors.New("advance amount cannot be less than used/refund amount")
		}
		if !input.ExchangeRate.Equal(advance.ExchangeRate) && advance.isUsed() {
			return nil, errors.New("cannot update exchage rate if advance is used")
		}
		bankingTransaction.CreditAdvanceId = oldBankingTransaction.CreditAdvanceId
	}

	if bankingTransaction.TransactionType == BankingTransactionTypeSupplierAdvance {
		systemAccounts, err := GetSystemAccounts(businessId)
		if err != nil {
			return nil, err
		}
		bankingTransaction.ToAccountId = systemAccounts[AccountCodeAdvancePayment] // advance payment
		input.ToAccountId = systemAccounts[AccountCodeAdvancePayment]
	} else if bankingTransaction.TransactionType == BankingTransactionTypeCustomerAdvance {
		systemAccounts, err := GetSystemAccounts(businessId)
		if err != nil {
			return nil, err
		}
		bankingTransaction.FromAccountId = systemAccounts[AccountCodeUnearnedRevenue] // unearned revenue
		input.FromAccountId = systemAccounts[AccountCodeUnearnedRevenue]
	}

	db := config.GetDB()
	tx := db.Begin()
	err = tx.WithContext(ctx).Model(&bankingTransaction).Updates(map[string]interface{}{
		"BranchId":          input.BranchId,
		"FromAccountId":     input.FromAccountId,
		"ToAccountId":       input.ToAccountId,
		"FromAccountAmount": bankingTransaction.FromAccountAmount,
		"ToAccountAmount":   bankingTransaction.ToAccountAmount,
		"CustomerId":        input.CustomerId,
		"SupplierId":        input.SupplierId,
		"TransactionDate":   input.TransactionDate,
		"TransactionId":     input.TransactionId,
		"TransactionNumber": input.TransactionNumber,
		"ExchangeRate":      input.ExchangeRate,
		"CurrencyId":        input.CurrencyId,
		"Amount":            input.Amount,
		"TaxAmount":         input.TaxAmount,
		"PaymentModeId":     input.PaymentModeId,
		"BankCharges":       input.BankCharges,
		"ReferenceNumber":   input.ReferenceNumber,
		"Description":       input.Description,
	}).Error
	if err != nil {
		tx.Rollback()
		return nil, err
	}

	var detailItems []BankingTransactionDetail
	for _, item := range input.Details {
		detailItem := BankingTransactionDetail{
			ID:                   item.ID,
			BankingTransactionId: id,
			InvoiceNo:            item.InvoiceNo,
			DueAmount:            item.DueAmount,
			DueDate:              item.DueDate,
			PaymentAmount:        item.PaymentAmount,
		}
		detailItems = append(detailItems, detailItem)
	}
	// create if detail id is zero or does not exist, update if it does, remove excluded ids
	if err := ReplaceAssociation(ctx, tx, detailItems, "banking_transaction_id = ?", id); err != nil {
		tx.Rollback()
		return nil, err
	}

	if _, err = upsertDocuments(ctx, tx, input.Documents, "banking_transactions", id); err != nil {
		tx.Rollback()
		return nil, err
	}

	if bankingTransaction.TransactionType == BankingTransactionTypeTransferFromAnotherAccounts ||
		bankingTransaction.TransactionType == BankingTransactionTypeTransferToAnotherAccount {
		err = PublishToAccounting(ctx, tx, businessId, bankingTransaction.TransactionDate, bankingTransaction.ID, AccountReferenceTypeAccountTransfer, bankingTransaction, oldBankingTransaction, PubSubMessageActionUpdate)
		if err != nil {
			tx.Rollback()
			return nil, err
		}
	} else if bankingTransaction.TransactionType == BankingTransactionTypeDepositFromOtherAccounts ||
		bankingTransaction.TransactionType == BankingTransactionTypeDepositToOtherAccounts {
		err = PublishToAccounting(ctx, tx, businessId, bankingTransaction.TransactionDate, bankingTransaction.ID, AccountReferenceTypeAccountDeposit, bankingTransaction, oldBankingTransaction, PubSubMessageActionUpdate)
		if err != nil {
			tx.Rollback()
			return nil, err
		}
	} else if bankingTransaction.TransactionType == BankingTransactionTypeOwnerContribution {
		err = PublishToAccounting(ctx, tx, businessId, bankingTransaction.TransactionDate, bankingTransaction.ID, AccountReferenceTypeOwnerContribution, bankingTransaction, oldBankingTransaction, PubSubMessageActionUpdate)
		if err != nil {
			tx.Rollback()
			return nil, err
		}
	} else if bankingTransaction.TransactionType == BankingTransactionTypeOwnerDrawings {
		err = PublishToAccounting(ctx, tx, businessId, bankingTransaction.TransactionDate, bankingTransaction.ID, AccountReferenceTypeOwnerDrawing, bankingTransaction, oldBankingTransaction, PubSubMessageActionUpdate)
		if err != nil {
			tx.Rollback()
			return nil, err
		}
	} else if bankingTransaction.TransactionType == BankingTransactionTypeOtherIncome ||
		bankingTransaction.TransactionType == BankingTransactionTypeInterestIncome {

		err = PublishToAccounting(ctx, tx, businessId, bankingTransaction.TransactionDate, bankingTransaction.ID, AccountReferenceTypeOtherIncome, bankingTransaction, oldBankingTransaction, PubSubMessageActionUpdate)
		if err != nil {
			tx.Rollback()
			return nil, err
		}
	} else if bankingTransaction.TransactionType == BankingTransactionTypeSupplierAdvance {
		err = PublishToAccounting(ctx, tx, businessId, bankingTransaction.TransactionDate, bankingTransaction.ID, AccountReferenceTypeAdvanceSupplierPayment, bankingTransaction, oldBankingTransaction, PubSubMessageActionUpdate)
		if err != nil {
			tx.Rollback()
			return nil, err
		}
	} else if bankingTransaction.TransactionType == BankingTransactionTypeCustomerAdvance {
		err = PublishToAccounting(ctx, tx, businessId, bankingTransaction.TransactionDate, bankingTransaction.ID, AccountReferenceTypeAdvanceCustomerPayment, bankingTransaction, oldBankingTransaction, PubSubMessageActionUpdate)
		if err != nil {
			tx.Rollback()
			return nil, err
		}
	}

	return &bankingTransaction, tx.Commit().Error
}

func DeleteBankingTransaction(ctx context.Context, id int) (*BankingTransaction, error) {

	businessId, ok := utils.GetBusinessIdFromContext(ctx)
	if !ok || businessId == "" {
		return nil, errors.New("business id is required")
	}

	result, err := utils.FetchModelForChange[BankingTransaction](ctx, businessId, id)
	if err != nil {
		return nil, err
	}

	db := config.GetDB()
	tx := db.Begin()

	// validate if advance supplier payment
	if result.TransactionType == BankingTransactionTypeSupplierAdvance && result.CreditAdvanceId > 0 {
		advance, err := utils.FetchModel[SupplierCreditAdvance](ctx, businessId, result.CreditAdvanceId)
		if err != nil {
			return nil, err
		}
		if err := advance.CheckTransactionLock(ctx); err != nil {
			return nil, err
		}
		if !advance.Amount.Equals(advance.RemainingBalance) {
			return nil, errors.New("advance amount is already used")
		}
		err = tx.WithContext(ctx).Delete(&advance).Error
		if err != nil {
			tx.Rollback()
			return nil, err
		}
	}
	// validate if advance customer payment
	if result.TransactionType == BankingTransactionTypeCustomerAdvance && result.CreditAdvanceId > 0 {
		advance, err := utils.FetchModel[CustomerCreditAdvance](ctx, businessId, result.CreditAdvanceId)
		if err != nil {
			return nil, err
		}
		if err := advance.CheckTransactionLock(ctx); err != nil {
			return nil, err
		}
		if !advance.Amount.Equals(advance.RemainingBalance) {
			return nil, errors.New("advance amount is already used")
		}
		err = tx.WithContext(ctx).Delete(&advance).Error
		if err != nil {
			tx.Rollback()
			return nil, err
		}
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

	if result.TransactionType == BankingTransactionTypeTransferFromAnotherAccounts ||
		result.TransactionType == BankingTransactionTypeTransferToAnotherAccount {
		err = PublishToAccounting(ctx, tx, businessId, result.TransactionDate, result.ID, AccountReferenceTypeAccountTransfer, nil, result, PubSubMessageActionDelete)
		if err != nil {
			tx.Rollback()
			return nil, err
		}
	} else if result.TransactionType == BankingTransactionTypeDepositFromOtherAccounts ||
		result.TransactionType == BankingTransactionTypeDepositToOtherAccounts {
		err = PublishToAccounting(ctx, tx, businessId, result.TransactionDate, result.ID, AccountReferenceTypeAccountDeposit, nil, result, PubSubMessageActionDelete)
		if err != nil {
			tx.Rollback()
			return nil, err
		}
	} else if result.TransactionType == BankingTransactionTypeOwnerContribution {
		err = PublishToAccounting(ctx, tx, businessId, result.TransactionDate, result.ID, AccountReferenceTypeOwnerContribution, nil, result, PubSubMessageActionDelete)
		if err != nil {
			tx.Rollback()
			return nil, err
		}
	} else if result.TransactionType == BankingTransactionTypeOwnerDrawings {
		err = PublishToAccounting(ctx, tx, businessId, result.TransactionDate, result.ID, AccountReferenceTypeOwnerDrawing, nil, result, PubSubMessageActionDelete)
		if err != nil {
			tx.Rollback()
			return nil, err
		}
	} else if result.TransactionType == BankingTransactionTypeOtherIncome ||
		result.TransactionType == BankingTransactionTypeInterestIncome {
		err = PublishToAccounting(ctx, tx, businessId, result.TransactionDate, result.ID, AccountReferenceTypeOtherIncome, nil, result, PubSubMessageActionDelete)
		if err != nil {
			tx.Rollback()
			return nil, err
		}
	} else if result.TransactionType == BankingTransactionTypeSupplierAdvance {
		err = PublishToAccounting(ctx, tx, businessId, result.TransactionDate, result.ID, AccountReferenceTypeAdvanceSupplierPayment, nil, result, PubSubMessageActionDelete)
		if err != nil {
			tx.Rollback()
			return nil, err
		}
	} else if result.TransactionType == BankingTransactionTypeCustomerAdvance {
		err = PublishToAccounting(ctx, tx, businessId, result.TransactionDate, result.ID, AccountReferenceTypeAdvanceCustomerPayment, nil, result, PubSubMessageActionDelete)
		if err != nil {
			tx.Rollback()
			return nil, err
		}
	}
	return result, tx.Commit().Error
}

func GetBankingTransaction(ctx context.Context, id int) (*BankingTransaction, error) {
	businessId, ok := utils.GetBusinessIdFromContext(ctx)
	if !ok || businessId == "" {
		return nil, errors.New("business id is required")
	}

	return utils.FetchModel[BankingTransaction](ctx, businessId, id)
}

func PaginateBankingTransaction(
	ctx context.Context, limit *int, after *string,
	referenceNumber *string,
	branchID *int,
	accountID *int,
	startDate *MyDateString,
	endDate *MyDateString,
) (*BankingTransactionsConnection, error) {

	businessId, ok := utils.GetBusinessIdFromContext(ctx)
	if !ok || businessId == "" {
		return nil, errors.New("business id is required")
	}
	business, err := GetBusiness(ctx)
	if err != nil {
		return nil, errors.New("business id is required")
	}
	if err := startDate.StartOfDayUTCTime(business.Timezone); err != nil {
		return nil, err
	}
	if err := endDate.EndOfDayUTCTime(business.Timezone); err != nil {
		return nil, err
	}

	db := config.GetDB()
	dbCtx := db.WithContext(ctx).Where("business_id = ?", businessId)

	if referenceNumber != nil && *referenceNumber != "" {
		dbCtx.Where("reference_number LIKE ?", "%"+*referenceNumber+"%")
	}
	if branchID != nil && *branchID > 0 {
		dbCtx.Where("branch_id = ?", *branchID)
	}
	if accountID != nil && *accountID > 0 {
		dbCtx.Where("from_account_id = ? OR to_account_id = ?", *accountID, *accountID)
	}

	if startDate != nil && endDate != nil {
		dbCtx.Where("transaction_date BETWEEN ? AND ?", startDate, endDate)
	}

	edges, pageInfo, err := FetchPageCompositeCursor[BankingTransaction](dbCtx, *limit, after, "transaction_date", "<")
	if err != nil {
		return nil, err
	}
	var bankingTransactionsConnection BankingTransactionsConnection
	bankingTransactionsConnection.PageInfo = pageInfo
	for _, edge := range edges {
		bankingTransactionEdge := BankingTransactionsEdge(edge)
		bankingTransactionsConnection.Edges = append(bankingTransactionsConnection.Edges, &bankingTransactionEdge)
	}

	return &bankingTransactionsConnection, err
}
