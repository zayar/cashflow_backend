package reports

import (
	"context"
	"time"

	"github.com/mmdatafocus/books_backend/config"
	"github.com/mmdatafocus/books_backend/models"
	"github.com/shopspring/decimal"
)

type PaymentMade struct {
	PaymentNumber     string          `json:"paymentNumber"`
	PaymentDate       time.Time       `json:"paymentDate"`
	ReferenceNumber   *string         `json:"referenceNumber,omitempty"`
	SupplierID        int             `json:"supplierId"`
	SupplierName      string          `json:"supplierName"`
	PaymentMode       string          `json:"paymentMode"`
	Notes             *string         `json:"notes,omitempty"`
	BillNumbers       *string         `json:"billNumbers,omitempty"`
	WithdrawAccountId int             `json:"withdrawAccountId"`
	PaymentAmount     decimal.Decimal `json:"paymentAmount"`
	PaymentAmountFcy  decimal.Decimal `json:"paymentAmountFcy"`
	UnusedAmount      decimal.Decimal `json:"unusedAmount"`
	UnusedAmountFcy   decimal.Decimal `json:"unusedAmountFcy"`
	CurrencyId        int             `json:"currencyId"`
}

func GetPaymentsMadeReport(ctx context.Context, fromDate models.MyDateString, toDate models.MyDateString) ([]*PaymentMade, error) {

	sql := `
WITH SPM AS(
    (SELECT
        sca.id payment_number,
        sca.date payment_date,
        btx.payment_mode_id,
        sca.currency_id,
        (
			CASE
				WHEN sca.currency_id <> @baseCurrencyId THEN sca.amount
                ELSE 0
			END
		) payment_amount_fcy,
        (
            CASE
                WHEN sca.currency_id <> @baseCurrencyId THEN sca.amount * sca.exchange_rate
                ELSE sca.amount
            END
        ) payment_amount,
        (
			CASE
				WHEN sca.currency_id <> @baseCurrencyId THEN sca.remaining_balance
                ELSE 0
			END
		) unused_amount_fcy,
        (
            CASE
                WHEN sca.currency_id <> @baseCurrencyId THEN sca.remaining_balance * sca.exchange_rate
                ELSE sca.remaining_balance
            END
        ) unused_amount,
        sca.supplier_id,
        btx.from_account_id withdraw_account_id,
        btx.reference_number,
        btx.description notes,
        "" bill_numbers
    FROM
        supplier_credit_advances sca
        LEFT JOIN banking_transactions btx ON btx.credit_advance_id = sca.id
        AND btx.transaction_type = 'SupplierAdvance'
    WHERE
		sca.business_id = @businessId
        AND NOT sca.current_status = 'Draft'
        AND sca.date BETWEEN @fromDate
        AND @toDate)
    UNION
    (SELECT
        sp.payment_number,
        sp.payment_date,
        sp.payment_mode_id,
        sp.currency_id,
        (
			CASE
				WHEN sp.currency_id <> @baseCurrencyId THEN sp.amount
                ELSE 0
			END
		) payment_amount_fcy,
        (
            CASE
                WHEN sp.currency_id <> @baseCurrencyId THEN sp.exchange_rate * sp.amount
                ELSE sp.amount
            END
        ) payment_amount,
        0 AS unused_amount_fcy,
        0 AS unused_amount,
        sp.supplier_id,
        sp.withdraw_account_id,
        sp.reference_number,
        sp.notes,
        GROUP_CONCAT(bills.bill_number) bill_numbers
    FROM
        supplier_payments sp
        INNER JOIN supplier_paid_bills ON sp.id = supplier_paid_bills.supplier_payment_id
        INNER JOIN bills ON bills.id = supplier_paid_bills.bill_id
    WHERE
        sp.business_id = @businessId
        AND sp.payment_date BETWEEN @fromDate
        AND @toDate
    GROUP BY
        sp.id)
)
SELECT
    SPM.*,
    suppliers.name supplier_name,
    payment_modes.name payment_mode
FROM
    SPM
    LEFT JOIN suppliers ON suppliers.id = SPM.supplier_id
    LEFT JOIN payment_modes ON payment_modes.id = SPM.payment_mode_id
ORDER BY
    SPM.payment_date;
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
	var results []*PaymentMade
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
