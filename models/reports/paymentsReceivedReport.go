package reports

import (
	"context"
	"time"

	"github.com/mmdatafocus/books_backend/config"
	"github.com/mmdatafocus/books_backend/models"
	"github.com/shopspring/decimal"
)

type PaymentReceived struct {
	PaymentNumber    string          `json:"paymentNumber"`
	PaymentDate      time.Time       `json:"paymentDate"`
	ReferenceNumber  *string         `json:"referenceNumber,omitempty"`
	CustomerID       int             `json:"customerId"`
	CustomerName     string          `json:"customerName"`
	PaymentMode      string          `json:"paymentMode"`
	Notes            *string         `json:"notes,omitempty"`
	InvoiceNumbers   *string         `json:"invoiceNumbers"`
	DepositAccountId int             `json:"depositAccountId"`
	PaymentAmount    decimal.Decimal `json:"paymentAmount"`
	PaymentAmountFcy decimal.Decimal `json:"paymentAmountFcy"`
	UnusedAmount     decimal.Decimal `json:"unusedAmount"`
	UnusedAmountFcy  decimal.Decimal `json:"unusedAmountFcy"`
	CurrencyId       int             `json:"currencyId"`
}

func GetPaymentsReceivedReport(ctx context.Context, fromDate models.MyDateString, toDate models.MyDateString) ([]*PaymentReceived, error) {

	sql := `
WITH CPM AS(
    (SELECT
        cca.id payment_number,
        cca.date payment_date,
        btx.payment_mode_id,
        cca.currency_id,
        (
			CASE
				WHEN cca.currency_id <> @baseCurrencyId THEN cca.amount
                ELSE 0
			END
		) payment_amount_fcy,
        (
            CASE
                WHEN cca.currency_id <> @baseCurrencyId THEN cca.amount * cca.exchange_rate
                ELSE cca.amount
            END
        ) payment_amount,
        (
			CASE
				WHEN cca.currency_id <> @baseCurrencyId THEN cca.remaining_balance
                ELSE 0
			END
		) unused_amount_fcy,
        (
            CASE
                WHEN cca.currency_id <> @baseCurrencyId THEN cca.remaining_balance * cca.exchange_rate
                ELSE cca.remaining_balance
            END
        ) unused_amount,
        cca.customer_id,
        btx.to_account_id deposit_account_id,
        btx.reference_number,
        btx.description notes,
        "" invoice_numbers
    FROM
        customer_credit_advances cca
        LEFT JOIN banking_transactions btx ON btx.credit_advance_id = cca.id
        AND btx.transaction_type = 'CustomerAdvance'
    WHERE
		cca.business_id = @businessId
        AND NOT cca.current_status IN ('Draft', 'Void')
        AND cca.date BETWEEN @fromDate
        AND @toDate)
    UNION
    (SELECT
        cp.payment_number,
        cp.payment_date,
        cp.payment_mode_id,
        cp.currency_id,
        (
			CASE
				WHEN cp.currency_id <> @baseCurrencyId THEN cp.amount
                ELSE 0
			END
		) payment_amount_fcy,
        (
            CASE
                WHEN cp.currency_id <> @baseCurrencyId THEN cp.exchange_rate * cp.amount
                ELSE cp.amount
            END
        ) payment_amount,
        0 AS unused_amount_fcy,
        0 AS unused_amount,
        cp.customer_id,
        cp.deposit_account_id,
        cp.reference_number,
        cp.notes,
        GROUP_CONCAT(sales_invoices.invoice_number) invoice_numbers
    FROM
        customer_payments cp
        INNER JOIN paid_invoices ON cp.id = paid_invoices.customer_payment_id
        INNER JOIN sales_invoices ON sales_invoices.id = paid_invoices.invoice_id
        LEFT JOIN (
            SELECT
                reference_id,
                MAX(id) AS max_id
            FROM
                pub_sub_message_records
            WHERE
                business_id = @businessId
                AND reference_type = 'IV'
            GROUP BY
                reference_id
        ) iv_outbox_latest ON iv_outbox_latest.reference_id = sales_invoices.id
        LEFT JOIN pub_sub_message_records iv_outbox ON iv_outbox.id = iv_outbox_latest.max_id
    WHERE
        cp.business_id = @businessId
        AND cp.payment_date BETWEEN @fromDate
        AND @toDate
        AND sales_invoices.current_status NOT IN ('Draft', 'Void')
        AND (iv_outbox.processing_status IS NULL OR iv_outbox.processing_status <> 'DEAD')
    GROUP BY
        cp.id)
)
SELECT
    CPM.*,
    customers.name customer_name,
    payment_modes.name payment_mode
FROM
    CPM
    LEFT JOIN customers ON customers.id = CPM.customer_id
    LEFT JOIN payment_modes ON payment_modes.id = CPM.payment_mode_id
ORDER BY
    CPM.payment_date;
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
	var results []*PaymentReceived
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
