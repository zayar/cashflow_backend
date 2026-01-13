package reports

import (
	"context"
	"time"

	"bitbucket.org/mmdatafocus/books_backend/config"
	"bitbucket.org/mmdatafocus/books_backend/models"
	"github.com/shopspring/decimal"
)

type SupplierRefundHistory struct {
	RefundDate        time.Time       `json:"refundDate"`
	SupplierID        int             `json:"supplierId"`
	SupplierName      string          `json:"supplierName"`
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

func GetSupplierRefundHistoryReport(ctx context.Context, fromDate models.MyDateString, toDate models.MyDateString) ([]*SupplierRefundHistory, error) {

	sql := `
	
WITH TransactionDetails AS (
    SELECT
        id transaction_id,
        'SC' transaction_type,
        supplier_credit_number transaction_number
    from
		supplier_credits
    UNION
    SELECT
        id transaction_id,
        'SA' transaction_type,
        CONCAT('SA-', id) transaction_number
    from
        supplier_credit_advances
)
SELECT
	rf.supplier_id,
	suppliers.name supplier_name,
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
	) amount_fcy,
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
    LEFT JOIN suppliers ON rf.supplier_id = suppliers.id
    LEFT JOIN payment_modes ON rf.payment_mode_id = payment_modes.id
WHERE
	rf.business_id = @businessId
    AND rf.reference_type IN ('SC', 'SA')
	AND rf.refund_date BETWEEN @fromDate AND @toDate;
	`

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
	var results []*SupplierRefundHistory
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
