package reports

import (
	"context"
	"errors"

	"github.com/mmdatafocus/books_backend/config"
	"github.com/mmdatafocus/books_backend/models"
	"github.com/mmdatafocus/books_backend/utils"
	"github.com/shopspring/decimal"
)

type ARAgingSummaryResponse struct {
	CustomerID     int    `json:"customerId,omitempty"`
	CustomerName   string `json:"customerName,omitempty"`
	CurrencySymbol string
	DecimalPlaces  models.DecimalPlaces
	Total          decimal.Decimal `json:"total"`
	TotalFcy       decimal.Decimal `json:"totalFcy"`
	Current        decimal.Decimal `json:"currentDate"`
	Int1to15       decimal.Decimal `json:"int1to15"`
	Int16to30      decimal.Decimal `json:"int16to30"`
	Int31to45      decimal.Decimal `json:"int31to45"`
	Int46plus      decimal.Decimal `json:"int46plus"`
	InvoiceCount   int             `json:"invoiceCount"`
}

func GetARAgingSummaryReport(ctx context.Context, currentDate models.MyDateString, branchId *int, warehouseId *int) ([]*ARAgingSummaryResponse, error) {

	var results []*ARAgingSummaryResponse
	sqlTemplate := `
    WITH LatestInvoiceOutbox AS (
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
    ),
    InvoiceAging AS (
 SELECT
        si.customer_id,
        si.currency_id,
        -- si.invoice_number,
        -- si.invoice_date,
        (
			CASE
				WHEN si.currency_id <> @baseCurrencyId THEN si.remaining_balance
                ELSE 0
			END
		) AS remaining_balance,
        -- si.exchange_rate,
        (CASE
            WHEN si.currency_id = @baseCurrencyId THEN si.remaining_balance
            ELSE si.remaining_balance * si.exchange_rate
        END) AS adjusted_remaining_balance,
        CASE
            WHEN si.remaining_balance > 0 THEN DATEDIFF(@currentDate, si.invoice_due_date)
            ELSE 0
        END AS days_overdue
    FROM
        sales_invoices si
        LEFT JOIN LatestInvoiceOutbox lio ON lio.reference_id = si.id
        LEFT JOIN pub_sub_message_records iv_outbox ON iv_outbox.id = lio.max_id
    WHERE
        si.business_id = @businessId
        AND invoice_date < @currentDate
        AND si.current_status IN ('Confirmed', 'Partial Paid')
        AND (iv_outbox.processing_status IS NULL OR iv_outbox.processing_status <> 'DEAD')
        {{- if .branchId }} AND branch_id = @branchId {{- end}}
        {{- if .warehouseId }} AND warehouse_id = @warehouseId {{- end}}
)
SELECT
    customer_id,
    customers.name as customer_name,
    InvoiceAging.currency_id,
    currencies.symbol as currency_symbol,
	currencies.decimal_places,
    COUNT(*) as invoice_count,
    SUM(remaining_balance) as total_fcy,
    SUM(adjusted_remaining_balance) as total,
    SUM(
        CASE
            WHEN days_overdue <= 0 THEN adjusted_remaining_balance
            ELSE 0
        END
    ) AS current,
    SUM(
        CASE
            WHEN days_overdue BETWEEN 1
            AND 15 THEN adjusted_remaining_balance
            ELSE 0
        END
    ) AS int1to15,
    SUM(
        CASE
            WHEN days_overdue BETWEEN 16
            AND 30 THEN adjusted_remaining_balance
            ELSE 0
        END
    ) AS int16to30,
    SUM(
        CASE
            WHEN days_overdue BETWEEN 31
            AND 45 THEN adjusted_remaining_balance
            ELSE 0
        END
    ) AS int31to45,
    SUM(
        CASE
            WHEN days_overdue > 46 THEN adjusted_remaining_balance
            ELSE 0
        END
    ) AS int46plus
FROM
    InvoiceAging
    LEFT JOIN customers ON customers.id = InvoiceAging.customer_id
    LEFT JOIN currencies ON currencies.id = InvoiceAging.currency_id
GROUP BY
    InvoiceAging.customer_id,
    InvoiceAging.currency_id
ORDER BY
    InvoiceAging.customer_id,
    InvoiceAging.currency_id;
`

	businessId, ok := utils.GetBusinessIdFromContext(ctx)
	if !ok || businessId == "" {
		return nil, errors.New("business id is required")
	}
	business, err := models.GetBusiness(ctx)
	if err != nil {
		return nil, errors.New("business id is required")
	}
	if err := currentDate.EndOfDayUTCTime(business.Timezone); err != nil {
		return nil, err
	}

	db := config.GetDB()
	sql, err := utils.ExecTemplate(sqlTemplate, map[string]interface{}{
		"branchId":    utils.DereferencePtr(branchId, 0),
		"warehouseId": utils.DereferencePtr(warehouseId, 0),
	})
	if err != nil {
		return nil, err
	}

	if err := db.WithContext(ctx).Raw(sql, map[string]interface{}{
		"currentDate":    currentDate,
		"businessId":     businessId,
		"baseCurrencyId": business.BaseCurrencyId,
		"branchId":       branchId,
		"warehouseId":    warehouseId,
	}).Scan(&results).Error; err != nil {
		return nil, err
	}
	return results, nil

}
