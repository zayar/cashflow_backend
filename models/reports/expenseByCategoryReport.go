package reports

import (
	"context"
	"errors"
	"time"

	"github.com/mmdatafocus/books_backend/config"
	"github.com/mmdatafocus/books_backend/models"
	"github.com/mmdatafocus/books_backend/utils"
	"github.com/shopspring/decimal"
)

type ExpenseByCategory struct {
	TransactionDate time.Time       `json:"transaction_date"`
	TransactionType string          `json:"transaction_type"`
	CustomerName    string          `json:"customer_name"`
	CustomerId      int             `json:"customer_id"`
	SupplierName    string          `json:"supplier_name"`
	SupplierId      int             `json:"supplier_id"`
	Amount          decimal.Decimal `json:"amount"`
	AmountWithTax   decimal.Decimal `json:"amount_with_tax"`
}

func GetExpenseByCategory(ctx context.Context, accountId int, fromDate models.MyDateString, toDate models.MyDateString) ([]*ExpenseByCategory, error) {
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

	var results []*ExpenseByCategory

	query := db.Raw(`
            SELECT 
				aj.reference_id,
				aj.reference_type,
				aj.customer_id,
				aj.supplier_id,
				cus.name as customer_name,
				sup.name as supplier_name,
				SUM(at.base_debit - at.base_credit) as amount,
				SUM(at.base_debit - at.base_credit) + 
				SUM(
					CASE
						WHEN aj.reference_type = ? THEN 
							CASE 
								WHEN at.base_currency_id != exp.currency_id THEN COALESCE(exp.tax_amount * exp.exchange_rate, 0)
								ELSE COALESCE(exp.tax_amount, 0)
							END
						ELSE 0
					END
				) as amount_with_tax,
				at.transaction_date_time as transaction_date
			FROM account_journals AS aj
			JOIN account_transactions AS at ON at.journal_id = aj.id
			LEFT JOIN expenses AS exp ON aj.reference_id = exp.id AND aj.reference_type = ?
			LEFT JOIN customers AS cus ON aj.customer_id = cus.id
			LEFT JOIN suppliers AS sup ON aj.supplier_id = sup.id
			WHERE 
				aj.transaction_date_time >= ?
				AND aj.transaction_date_time <= ?
				AND aj.business_id = ?
				AND at.business_id = ?
				AND aj.is_reversal = 0
				AND aj.reversed_by_journal_id IS NULL
				AND at.account_id = ?
				AND aj.reference_type IN (?, ?, ?)
			GROUP BY
				aj.customer_id,
				aj.supplier_id,
				aj.reference_id,
				aj.reference_type,
				at.transaction_date_time
			ORDER BY
				transaction_date;
        `,
		accountReferenceTypeExpense, accountReferenceTypeExpense,
		fromDate, toDate, businessId, businessId, accountId,
		accountReferenceTypeJournal, accountReferenceTypeExpense, accountReferenceTypeInvoiceWriteOff,
	)

	err = query.Scan(&results).Error

	if err != nil {
		return nil, err
	}

	return results, nil
}
