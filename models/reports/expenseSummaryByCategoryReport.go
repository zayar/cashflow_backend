package reports

import (
	"context"
	"errors"

	"bitbucket.org/mmdatafocus/books_backend/config"
	"bitbucket.org/mmdatafocus/books_backend/models"
	"bitbucket.org/mmdatafocus/books_backend/utils"
	"github.com/shopspring/decimal"
)

type ExpenseSummaryByCategory struct {
	TotalAmount                    decimal.Decimal                  `json:"total_amount"`
	TotalAmountWithTax             decimal.Decimal                  `json:"total_amount_with_tax"`
	ExpenseSummaryByCategoryDetail []ExpenseSummaryByCategoryDetail `json:"expense_summary_by_category_detail"`
}

type ExpenseSummaryByCategoryDetail struct {
	AccountId     int             `json:"account_id"`
	AccountName   string          `json:"account_name"`
	Amount        decimal.Decimal `json:"amount"`
	AmountWithTax decimal.Decimal `json:"amount_with_tax"`
}

func GetExpenseSummaryByCategory(ctx context.Context, fromDate models.MyDateString, toDate models.MyDateString) (*ExpenseSummaryByCategory, error) {
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

	accountReferenceTypeExpense := models.AccountReferenceTypeExpense
	accountReferenceTypeJournal := models.AccountReferenceTypeJournal
	accountReferenceTypeInvoiceWriteOff := models.AccountReferenceTypeInvoiceWriteOff
	accountDetailTypeExpense := models.AccountDetailTypeExpense
	query := `
        SELECT 
			ac.id as account_id,
			ac.name as account_name,
			SUM(acc_tran.base_debit - acc_tran.base_credit) as amount,
			SUM(acc_tran.base_debit - acc_tran.base_credit) + 
			SUM(
				CASE
					WHEN aj.reference_type = ? THEN 
						CASE 
							WHEN acc_tran.base_currency_id != exp.currency_id THEN COALESCE(exp.tax_amount * exp.exchange_rate, 0)
							ELSE COALESCE(exp.tax_amount, 0)
						END
					ELSE 0
				END
			) as amount_with_tax
		FROM account_transactions AS acc_tran
		JOIN accounts AS ac ON acc_tran.account_id = ac.id
		JOIN account_journals AS aj ON acc_tran.journal_id = aj.id
		LEFT JOIN expenses AS exp ON aj.reference_id = exp.id AND aj.reference_type = ?
		WHERE 
			acc_tran.transaction_date_time >= ?
			AND acc_tran.transaction_date_time <= ?
			AND acc_tran.business_id = ?
			AND aj.reference_type IN (?, ?, ?)
			AND acc_tran.account_id IN (
				SELECT id FROM accounts WHERE main_type = ?
			)
		GROUP BY
			ac.id,
			ac.name
		ORDER BY
			account_name;
	`

	rows, err := db.Raw(query,
		accountReferenceTypeExpense, accountReferenceTypeExpense,
		fromDate, toDate, businessId,
		accountReferenceTypeJournal, accountReferenceTypeExpense, accountReferenceTypeInvoiceWriteOff,
		accountDetailTypeExpense,
	).Rows()

	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var totalAmount decimal.Decimal
	var totalAmountWithTax decimal.Decimal
	var details []ExpenseSummaryByCategoryDetail

	// Iterate over query results
	for rows.Next() {
		var accountId int
		var accountName string
		var amount, amountWithTax decimal.Decimal

		if err := rows.Scan(&accountId, &accountName, &amount, &amountWithTax); err != nil {
			return nil, err
		}

		details = append(details, ExpenseSummaryByCategoryDetail{
			AccountId:     accountId,
			AccountName:   accountName,
			Amount:        amount,
			AmountWithTax: amountWithTax,
		})
		totalAmount = totalAmount.Add(amount)
		totalAmountWithTax = totalAmountWithTax.Add(amountWithTax)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	expenseSummary := &ExpenseSummaryByCategory{
		TotalAmount:                    totalAmount,
		TotalAmountWithTax:             totalAmountWithTax,
		ExpenseSummaryByCategoryDetail: details,
	}

	return expenseSummary, nil
}
