package models

import (
	"context"
	"errors"
	"fmt"
	"time"

	"bitbucket.org/mmdatafocus/books_backend/config"
	"bitbucket.org/mmdatafocus/books_backend/utils"
	"github.com/shopspring/decimal"
)

type Expense struct {
	ID                       int             `gorm:"primary_key" json:"id"`
	BusinessId               string          `gorm:"index;not null" json:"business_id" binding:"required"`
	ExpenseAccountId         int             `gorm:"index;not null" json:"expense_account_id" binding:"required"`
	AssetAccountId           int             `gorm:"index;not null" json:"asset_account_id" binding:"required"`
	BranchId                 int             `gorm:"index" json:"branch_id"`
	ExpenseDate              time.Time       `gorm:"not null" json:"expense_date" binding:"required"`
	CurrencyId               int             `gorm:"not null" json:"currency_id" binding:"required"`
	ExchangeRate             decimal.Decimal `gorm:"type:decimal(20,4);default:0" json:"exchange_rate"`
	SupplierId               int             `json:"supplier_id"`
	CustomerId               int             `json:"customer_id"`
	ExpenseNumber            string          `gorm:"size:255;not null" json:"expense_number" binding:"required"`
	SequenceNo               decimal.Decimal `gorm:"type:decimal(15);not null" json:"sequence_no"`
	ReferenceNumber          string          `gorm:"size:255" json:"reference_number"`
	Notes                    string          `gorm:"type:text" json:"notes"`
	ExpenseTaxId             int             `json:"expense_tax_id"`
	ExpenseTaxType           *TaxType        `gorm:"type:enum('I', 'G'); default:null" json:"expense_tax_type"`
	IsTaxInclusive           *bool           `gorm:"not null;default:false" json:"is_tax_inclusive"`
	Amount                   decimal.Decimal `gorm:"type:decimal(20,4);default:0" json:"amount"`
	BankCharges              decimal.Decimal `gorm:"type:decimal(20,4);default:0" json:"bank_charges"`
	TaxAmount                decimal.Decimal `gorm:"type:decimal(20,4);default:0" json:"tax_amount"`
	TotalAmount              decimal.Decimal `gorm:"type:decimal(20,4);default:0" json:"total_amount"`
	Documents                []*Document     `gorm:"polymorphic:Reference" json:"documents"`
	ExpenseTotalRefundAmount decimal.Decimal `gorm:"type:decimal(20,4);default:0" json:"expense_total_refund_amount"`
	RemainingBalance         decimal.Decimal `gorm:"type:decimal(20,4);default:0" json:"remaining_balance"`
	CreatedAt                time.Time       `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt                time.Time       `gorm:"autoUpdateTime" json:"updated_at"`
}

type NewExpense struct {
	BusinessId       string          `json:"business_id" binding:"required"`
	ExpenseAccountId int             `json:"expense_account_id" binding:"required"`
	AssetAccountId   int             `json:"asset_account_id" binding:"required"`
	BranchId         int             `json:"branch_id"`
	ExpenseDate      time.Time       `json:"expense_date" binding:"required"`
	CurrencyId       int             `json:"currency_id" binding:"required"`
	ExchangeRate     decimal.Decimal `json:"exchange_rate"`
	Amount           decimal.Decimal `json:"amount"`
	BankCharges      decimal.Decimal `json:"bank_charges"`
	SupplierId       int             `json:"supplier_id"`
	CustomerId       int             `json:"customer_id"`
	ReferenceNumber  string          `json:"reference_number"`
	Notes            string          `json:"notes"`
	ExpenseTaxId     int             `json:"expense_tax_id"`
	ExpenseTaxType   *TaxType        `json:"expense_tax_type"`
	IsTaxInclusive   *bool           `json:"is_tax_inclusive" binding:"required"`
	Documents        []*NewDocument  `json:"documents"`
}

type ExpensesEdge Edge[Expense]

func (obj Expense) GetId() int {
	return obj.ID
}

type ExpensesConnection struct {
	PageInfo *PageInfo       `json:"pageInfo"`
	Edges    []*ExpensesEdge `json:"edges"`
}

// implements Node
func (p Expense) GetCursor() string {
	return p.ExpenseDate.String()
}

func (e *Expense) GetRemainingBalance() decimal.Decimal {
	return e.RemainingBalance
}

func (e *Expense) AddRefundAmount(amount decimal.Decimal) error {
	if amount.GreaterThan(e.RemainingBalance) {
		return errors.New("amount must be less than or equal to remaining balance of expense")
	}
	e.ExpenseTotalRefundAmount = e.ExpenseTotalRefundAmount.Add(amount)
	e.RemainingBalance = e.RemainingBalance.Sub(amount)
	return nil
}

func (e *Expense) UpdateStatus() error {
	// if s.RemainingBalance.IsZero() {
	// 	s.CurrentStatus = ExpenseStatusClosed
	// } else {
	// 	s.CurrentStatus = ExpenseStatusConfirmed
	// }
	return nil
}

func (e *Expense) GetDueDate() time.Time {
	return e.ExpenseDate
}

func (e Expense) CheckTransactionLock(ctx context.Context) error {
	return validateTransactionLock(ctx, e.ExpenseDate, e.BusinessId, PurchaseTransactionLock)
}

// validate input for both create & update. (id = 0 for create)
func (input *NewExpense) validate(ctx context.Context, businessId string, _ int) error {
	// exists expense account
	// Expense Accounts - Cost of Goods Sold, Expense, Other Current Liability, Fixed Asset, Other Current Asset
	count, err := utils.ResourceCountWhere[Account](ctx, businessId, "id = ? AND detail_type IN ?", input.ExpenseAccountId,
		[]AccountDetailType{AccountDetailTypeCostOfGoodsSold, AccountDetailTypeExpense, AccountDetailTypeOtherCurrentLiability,
			AccountDetailTypeFixedAsset, AccountDetailTypeOtherCurrentAsset})
	if err != nil {
		return err
	}
	if count == 0 {
		return errors.New("expense account not found")
	}

	// exists asset account
	// Asset Accounts - Other Current Asset, Cash, Bank, Fixed Asset, Other Current Liability, Equity
	count, err = utils.ResourceCountWhere[Account](ctx, businessId, "id = ? AND detail_type IN ?", input.AssetAccountId,
		[]AccountDetailType{AccountDetailTypeOtherCurrentAsset, AccountDetailTypeCash, AccountDetailTypeBank,
			AccountDetailTypeFixedAsset, AccountDetailTypeOtherCurrentLiability, AccountDetailTypeEquity})
	if err != nil {
		return err
	}
	if count == 0 {
		return errors.New("asset account not found")
	}

	// exists branch
	if err := utils.ValidateResourceId[Branch](ctx, businessId, input.BranchId); err != nil {
		return errors.New("branch not found")
	}

	// exists currency
	if err := utils.ValidateResourceId[Currency](ctx, businessId, input.CurrencyId); err != nil {
		return errors.New("currency not found")
	}

	// exists supplier
	if input.SupplierId > 0 {
		if err := utils.ValidateResourceId[Supplier](ctx, businessId, input.SupplierId); err != nil {
			return errors.New("supplier not found")
		}
	}

	// exists customer
	if input.CustomerId > 0 {
		if err := utils.ValidateResourceId[Customer](ctx, businessId, input.CustomerId); err != nil {
			return errors.New("customer not found")
		}
	}

	// validate expense tax
	if input.ExpenseTaxType != nil {
		if err := validateTaxExists(ctx, businessId, input.ExpenseTaxId, *input.ExpenseTaxType); err != nil {
			return errors.New("expense tax not found")
		}
	}

	// validate ExpenseDate for transactionLock
	if err := validateTransactionLock(ctx, input.ExpenseDate, businessId, PurchaseTransactionLock); err != nil {
		return err
	}

	return nil
}

func CreateExpense(ctx context.Context, input *NewExpense) (*Expense, error) {

	businessId, ok := utils.GetBusinessIdFromContext(ctx)
	if !ok || businessId == "" {
		return nil, errors.New("business id is required")
	}
	// validate expense
	if err := input.validate(ctx, businessId, 0); err != nil {
		return nil, err
	}

	// construct Documents
	documents, err := mapNewDocuments(input.Documents, "expenses", 0)
	if err != nil {
		return nil, err
	}

	db := config.GetDB()
	// calculate tax
	isTaxInclusive := false
	if input.IsTaxInclusive != nil && *input.IsTaxInclusive {
		isTaxInclusive = true
	}
	taxAmount := decimal.NewFromInt(0)
	totalAmount := input.Amount.Add(input.BankCharges)
	if input.ExpenseTaxId > 0 {
		if *input.ExpenseTaxType == TaxTypeGroup {
			taxAmount = utils.CalculateTaxAmount(ctx, db, input.ExpenseTaxId, true, input.Amount, isTaxInclusive)
		} else {
			taxAmount = utils.CalculateTaxAmount(ctx, db, input.ExpenseTaxId, false, input.Amount, isTaxInclusive)
		}
		if !isTaxInclusive {
			totalAmount = totalAmount.Add(taxAmount)
		}
	}

	// store expense
	expense := Expense{
		BusinessId:       businessId,
		ExpenseAccountId: input.ExpenseAccountId,
		AssetAccountId:   input.AssetAccountId,
		BranchId:         input.BranchId,
		ExpenseDate:      input.ExpenseDate,
		CurrencyId:       input.CurrencyId,
		ExchangeRate:     input.ExchangeRate,
		Amount:           input.Amount,
		TaxAmount:        taxAmount,
		BankCharges:      input.BankCharges,
		TotalAmount:      totalAmount,
		RemainingBalance: totalAmount,
		SupplierId:       input.SupplierId,
		CustomerId:       input.CustomerId,
		ReferenceNumber:  input.ReferenceNumber,
		Notes:            input.Notes,
		ExpenseTaxId:     input.ExpenseTaxId,
		ExpenseTaxType:   input.ExpenseTaxType,
		IsTaxInclusive:   &isTaxInclusive,
		Documents:        documents,
	}

	tx := db.Begin()

	seqNo, err := utils.GetSequence[Expense](ctx, businessId)
	if err != nil {
		tx.Rollback()
		return nil, err
	}
	prefix, err := getTransactionPrefix(ctx, input.BranchId, "Expense")
	if err != nil {
		tx.Rollback()
		return nil, err
	}
	expense.SequenceNo = decimal.NewFromInt(seqNo)
	expense.ExpenseNumber = prefix + fmt.Sprint(seqNo)

	err = tx.WithContext(ctx).Create(&expense).Error
	if err != nil {
		tx.Rollback()
		return nil, err
	}

	expense.Documents = nil

	err = PublishToAccounting(ctx, tx, businessId, expense.ExpenseDate, expense.ID, AccountReferenceTypeExpense, expense, nil, PubSubMessageActionCreate)
	if err != nil {
		tx.Rollback()
		return nil, err
	}
	if err := tx.Commit().Error; err != nil {
		return nil, err
	}

	return &expense, nil
}

func UpdateExpense(ctx context.Context, id int, input *NewExpense) (*Expense, error) {

	businessId, ok := utils.GetBusinessIdFromContext(ctx)
	if !ok || businessId == "" {
		return nil, errors.New("business id is required")
	}

	// id exists
	beforeUpdate, err := utils.FetchModelForChange[Expense](ctx, businessId, id)
	if err != nil {
		return nil, err
	}
	// validate expense
	if err := input.validate(ctx, businessId, id); err != nil {
		return nil, err
	}

	db := config.GetDB()
	// calculate tax
	isTaxInclusive := false
	if input.IsTaxInclusive != nil && *input.IsTaxInclusive {
		isTaxInclusive = true
	}
	taxAmount := decimal.NewFromInt(0)
	totalAmount := input.Amount.Add(input.BankCharges)
	if input.ExpenseTaxId > 0 {
		if *input.ExpenseTaxType == TaxTypeGroup {
			taxAmount = utils.CalculateTaxAmount(ctx, db, input.ExpenseTaxId, true, input.Amount, isTaxInclusive)
		} else {
			taxAmount = utils.CalculateTaxAmount(ctx, db, input.ExpenseTaxId, false, input.Amount, isTaxInclusive)
		}
		if !isTaxInclusive {
			totalAmount = taxAmount.Add(taxAmount)
		}
	}

	update := Expense{
		ID:               id,
		BusinessId:       businessId,
		ExpenseAccountId: input.ExpenseAccountId,
		AssetAccountId:   input.AssetAccountId,
		BranchId:         input.BranchId,
		ExpenseDate:      input.ExpenseDate,
		CurrencyId:       input.CurrencyId,
		ExchangeRate:     input.ExchangeRate,
		Amount:           input.Amount,
		TaxAmount:        taxAmount,
		BankCharges:      input.BankCharges,
		TotalAmount:      totalAmount,
		RemainingBalance: totalAmount,
		SupplierId:       input.SupplierId,
		CustomerId:       input.CustomerId,
		ReferenceNumber:  input.ReferenceNumber,
		Notes:            input.Notes,
		ExpenseTaxId:     input.ExpenseTaxId,
		ExpenseTaxType:   input.ExpenseTaxType,
		IsTaxInclusive:   &isTaxInclusive,
	}

	tx := db.Begin()

	err = tx.WithContext(ctx).Model(&update).Updates(map[string]interface{}{
		"ExpenseAccountId": update.ExpenseAccountId,
		"AssetAccountId":   update.AssetAccountId,
		"BranchId":         update.BranchId,
		"ExpenseDate":      update.ExpenseDate,
		"CurrencyId":       update.CurrencyId,
		"ExchangeRate":     update.ExchangeRate,
		"Amount":           update.Amount,
		"BankCharges":      update.BankCharges,
		"TaxAmount":        update.TaxAmount,
		"TotalAmount":      update.TotalAmount,
		"RemainingBalance": update.RemainingBalance,
		"SupplierId":       update.SupplierId,
		"CustomerId":       update.CustomerId,
		"ReferenceNumber":  update.ReferenceNumber,
		"Notes":            update.Notes,
		"ExpenseTaxId":     update.ExpenseTaxId,
		"ExpenseTaxType":   update.ExpenseTaxType,
		"IsTaxInclusive":   update.IsTaxInclusive,
	}).Error
	if err != nil {
		tx.Rollback()
		return nil, err
	}

	_, err = upsertDocuments(ctx, tx, input.Documents, "expenses", id)
	if err != nil {
		tx.Rollback()
		return nil, err
	}
	update.Documents = nil
	beforeUpdate.Documents = nil

	err = PublishToAccounting(ctx, tx, businessId, update.ExpenseDate, update.ID, AccountReferenceTypeExpense, update, beforeUpdate, PubSubMessageActionUpdate)
	if err != nil {
		tx.Rollback()
		return nil, err
	}
	if err := tx.Commit().Error; err != nil {
		return nil, err
	}

	return &update, nil
}

func DeleteExpense(ctx context.Context, id int) (*Expense, error) {
	businessId, ok := utils.GetBusinessIdFromContext(ctx)
	if !ok || businessId == "" {
		return nil, errors.New("business id is required")
	}

	result, err := utils.FetchModelForChange[Expense](ctx, businessId, id, "Documents")
	if err != nil {
		return nil, err
	}

	// db action
	db := config.GetDB()
	tx := db.Begin()

	if err := tx.WithContext(ctx).Delete(&result).Error; err != nil {
		tx.Rollback()
		return nil, err
	}
	if err := deleteDocuments(ctx, tx, result.Documents); err != nil {
		tx.Rollback()
		return nil, err
	}
	result.Documents = nil
	err = PublishToAccounting(ctx, tx, businessId, result.ExpenseDate, result.ID, AccountReferenceTypeExpense, nil, result, PubSubMessageActionDelete)
	if err != nil {
		tx.Rollback()
		return nil, err
	}

	if err := tx.Commit().Error; err != nil {
		return nil, err
	}

	return result, nil
}

func GetExpense(ctx context.Context, id int) (*Expense, error) {

	return GetResource[Expense](ctx, id)
}

func PaginateExpense(ctx context.Context, limit *int, after *string, expenseAccountId *int, assetAccountId *int, branchId *int, fromDate *MyDateString, toDate *MyDateString, referenceNumber *string) (*ExpensesConnection, error) {
	businessId, ok := utils.GetBusinessIdFromContext(ctx)
	if !ok || businessId == "" {
		return nil, errors.New("business id is required")
	}
	db := config.GetDB()
	dbCtx := db.WithContext(ctx).Where("business_id = ?", businessId)

	business, err := GetBusiness(ctx)
	if err != nil {
		return nil, errors.New("business id is required")
	}
	if err := fromDate.StartOfDayUTCTime(business.Timezone); err != nil {
		return nil, err
	}
	if err := toDate.EndOfDayUTCTime(business.Timezone); err != nil {
		return nil, err
	}

	if expenseAccountId != nil && *expenseAccountId > 0 {
		dbCtx.Where("expense_account_id = ?", *expenseAccountId)
	}
	if expenseAccountId != nil && *expenseAccountId > 0 {
		dbCtx.Where("asset_account_id = ?", *assetAccountId)
	}
	if branchId != nil && *branchId > 0 {
		dbCtx.Where("branch_id = ?", *branchId)
	}
	if fromDate != nil && toDate != nil {
		dbCtx = dbCtx.Where("expense_date BETWEEN ? AND ?", fromDate, toDate)
	}
	if referenceNumber != nil && *referenceNumber != "" {
		dbCtx.Where("reference_number LIKE ?", "%"+*referenceNumber+"%")
	}

	// go forwards, ascending
	// err := PaginateModel[Expense](dbCtx, *limit, after, "created_at", "<", &connection)

	edges, pageInfo, err := FetchPageCompositeCursor[Expense](dbCtx, *limit, after, "expense_date", ">")
	if err != nil {
		return nil, err
	}
	var expensesConnection ExpensesConnection
	expensesConnection.PageInfo = pageInfo
	for _, edge := range edges {
		expensesEdge := ExpensesEdge(edge)
		expensesConnection.Edges = append(expensesConnection.Edges, &expensesEdge)
	}

	return &expensesConnection, err
}
