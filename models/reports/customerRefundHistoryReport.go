package reports

import (
	"context"
	"time"

	"github.com/mmdatafocus/books_backend/config"
	"github.com/mmdatafocus/books_backend/models"
	"github.com/shopspring/decimal"
)

type CustomerRefundHistory struct {
	RefundDate        time.Time       `json:"refundDate"`
	CustomerID        int             `json:"customerId"`
	CustomerName      string          `json:"customerName"`
	TransactionId     int             `json:"transactionId"`
	TransactionType   string          `json:"transactionType"`
	TransactionNumber string          `json:"transactionNumber"`
	ReferenceNumber   string          `json:"referenceNumber"`
	PaymentMode       string          `json:"paymentMode"`
	Notes             string          `json:"notes"`
	Amount            decimal.Decimal `json:"amount"`
	AmountFcy         decimal.Decimal `json:"amountFcy"`
	CurrencyId        int             `json:"currencyId"`
}

func GetCustomerRefundHistoryReport(ctx context.Context, fromDate models.MyDateString, toDate models.MyDateString) ([]*CustomerRefundHistory, error) {

	sql := `
	
WITH TransactionDetails AS (
    SELECT
        id transaction_id,
        'CN' transaction_type,
        credit_note_number transaction_number
    from
        credit_notes
    UNION
    SELECT
        id transaction_id,
        'CA' transaction_type,
        CONCAT('CA-', id) transaction_number
    from
        customer_credit_advances
)
SELECT
	rf.customer_id,
	customers.name customer_name,
	td.transaction_number,
    rf.reference_type transaction_type,
    rf.reference_id transaction_id,
    rf.reference_number,
    rf.refund_date,
    payment_modes.name payment_mode,
    (
		CASE
			WHEN rf.currency_id <> @baseCurrencyId THEN rf.amount
			ELSE 0
		END
	) AS amount_fcy,
    (
        CASE
            WHEN rf.currency_id <> @baseCurrencyId THEN rf.amount * rf.exchange_rate
            ELSE rf.amount
        END
    ) amount,
    rf.description notes,
    rf.currency_id
	-- rf.account_id,
    -- rf.reference_id
from
    refunds rf
    LEFT JOIN TransactionDetails td ON td.transaction_type = rf.reference_type
    AND td.transaction_id = rf.reference_id
    LEFT JOIN customers ON rf.customer_id = customers.id
    LEFT JOIN payment_modes ON rf.payment_mode_id = payment_modes.id
WHERE
	rf.business_id = @businessId
    AND rf.reference_type IN ('CN', 'CA')
	AND rf.refund_date BETWEEN @fromDate AND @toDate;
	`
	// WITH TransactionDetails AS (
	//     SELECT
	//         id transaction_id,
	//         'CN' transaction_type,
	//         credit_note_number transaction_number
	//     from
	//         credit_notes
	//     UNION
	//     SELECT
	//         id transaction_id,
	//         'CA' transaction_type,
	//         CONCAT('CA-', id)
	//     from
	//         customer_credit_advances
	// )
	// SELECT
	//     rf.reference_number,
	//     rf.customer_id,
	//     rf.refund_date,
	//     rf.amount amount_fcy,
	//     (
	//         CASE
	//             WHEN rf.currency_id <> 1 THEN rf.amount * rf.exchange_rate
	//             ELSE rf.amount
	//         END
	//     ) amount,
	//     rf.description notes,
	//     rf.payment_mode_id,
	//     -- rf.account_id,
	//     rf.currency_id
	// from
	//     refunds rf
	// WHERE
	//     rf.reference_type IN ('CN', 'CA');

	business, err := models.GetBusiness(ctx)
	if err != nil {
		return nil, err
	}

	if err := fromDate.StartOfDayUTCTime(business.Timezone); err != nil {
		return nil, err
	}
	if err := toDate.EndOfDayUTCTime(business.Timezone); err != nil {
		return nil, err
	}

	db := config.GetDB()
	var results []*CustomerRefundHistory
	if err := db.WithContext(ctx).Raw(sql, map[string]interface{}{
		"businessId":     business.ID,
		"baseCurrencyId": business.BaseCurrencyId,
		"fromDate":       fromDate,
		"toDate":         toDate,
	}).Scan(&results).Error; err != nil {
		return nil, err
	}
	return results, nil
}
