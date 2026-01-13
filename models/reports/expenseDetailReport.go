package reports

import (
	"context"
	"errors"
	"time"

	"bitbucket.org/mmdatafocus/books_backend/config"
	"bitbucket.org/mmdatafocus/books_backend/models"
	"bitbucket.org/mmdatafocus/books_backend/utils"
	"github.com/shopspring/decimal"
)

type ExpenseDetail struct {
	ID               int             `json:"id"`
	ExpenseAccountId int             `json:"expense_account_id"`
	AssetAccountId   int             `json:"asset_account_id"`
	BranchId         int             `json:"branch_id"`
	ExpenseDate      time.Time       `json:"expense_date"`
	CurrencyId       int             `json:"currency_id"`
	ExchangeRate     decimal.Decimal `json:"exchange_rate"`
	SupplierId       int             `json:"supplier_id"`
	CustomerId       int             `json:"customer_id"`
	ExpenseNumber    string          `json:"expense_number"`
	SequenceNo       decimal.Decimal `json:"sequence_no"`
	ReferenceNumber  string          `json:"reference_number"`
	Notes            string          `json:"notes"`
	FcyAmount        decimal.Decimal `json:"fcy_amount"`
	Amount           decimal.Decimal `json:"amount"`
	AmountWithTax    decimal.Decimal `json:"amount_with_tax"`
}

func GetExpenseDetail(ctx context.Context, fromDate models.MyDateString, toDate models.MyDateString) ([]*ExpenseDetail, error) {
	businessId, ok := utils.GetBusinessIdFromContext(ctx)
	if !ok || businessId == "" {
		return nil, errors.New("business id is required")
	}
	business, err := models.GetBusiness(ctx)
	if err != nil {
		return nil, errors.New("business id is required")
	}
	if err := fromDate.StartOfDayUTCTime(business.Timezone); err != nil {
		return nil, err
	}
	if err := toDate.EndOfDayUTCTime(business.Timezone); err != nil {
		return nil, err
	}

	db := config.GetDB()
	dbCtx := db.WithContext(ctx).Where("business_id = ?", businessId).
		Where("expense_date BETWEEN ? AND ?", fromDate, toDate)

	var results []*models.Expense
	err = dbCtx.Order("expense_date").Find(&results).Error
	if err != nil {
		return nil, err
	}

	var expenseDetails []*ExpenseDetail
	for _, expense := range results {

		var fcyAmount decimal.Decimal

		if !*expense.IsTaxInclusive {
			expense.Amount = expense.Amount.Sub(expense.TaxAmount)
		}
		if business.BaseCurrencyId != expense.CurrencyId {
			fcyAmount = expense.Amount
			expense.Amount = expense.Amount.Mul(expense.ExchangeRate)
			expense.TotalAmount = expense.TotalAmount.Mul(expense.ExchangeRate)
		}
		detail := &ExpenseDetail{
			ID:               expense.ID,
			ExpenseAccountId: expense.ExpenseAccountId,
			AssetAccountId:   expense.AssetAccountId,
			BranchId:         expense.BranchId,
			ExpenseDate:      expense.ExpenseDate,
			CurrencyId:       expense.CurrencyId,
			ExchangeRate:     expense.ExchangeRate,
			SupplierId:       expense.SupplierId,
			CustomerId:       expense.CustomerId,
			ExpenseNumber:    expense.ExpenseNumber,
			SequenceNo:       expense.SequenceNo,
			ReferenceNumber:  expense.ReferenceNumber,
			Notes:            expense.Notes,
			FcyAmount:        fcyAmount,
			Amount:           expense.Amount,
			AmountWithTax:    expense.TotalAmount,
		}
		expenseDetails = append(expenseDetails, detail)
	}

	return expenseDetails, nil

}
