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

type SalesInvoiceDetailResponse struct {
	InvoiceID           int             `json:"invoiceId"`
	InvoiceNumber       string          `json:"invoiceNumber"`
	OrderNumber         string          `json:"orderNumber"`
	InvoiceStatus       string          `json:"invoiceStatus"`
	InvoiceDate         time.Time       `json:"invoiceDate"`
	InvoiceDueDate      time.Time       `json:"invoiceDueDate"`
	TotalAmount         decimal.Decimal `json:"totalAmount"`
	TotalAmountFcy      decimal.Decimal `json:"totalAmountFcy"`
	RemainingBalance    decimal.Decimal `json:"remainingBalance"`
	RemainingBalanceFcy decimal.Decimal `json:"remainingBalanceFcy"`
	// CurrencyID          int             `json:"currencyId"`
	CurrencySymbol string `json:"currencySymbol"`
	DecimalPlaces  models.DecimalPlaces
	CustomerID     int    `json:"customerId"`
	CustomerName   string `json:"customerName"`
}

func GetSalesInvoiceDetailReport(ctx context.Context, fromDate models.MyDateString, toDate models.MyDateString, branchID *int, warehouseID *int) ([]*SalesInvoiceDetailResponse, error) {

	sqlTemplate := `
SELECT
    invoice.id as invoice_id,
    invoice.invoice_number,
    invoice.order_number AS order_number,
    invoice.invoice_date,
    invoice.invoice_due_date,
    (
        CASE
            WHEN invoice.current_status IN ('Draft', 'Void') THEN invoice.current_status
            WHEN invoice.remaining_balance > 0
            AND DATEDIFF(UTC_TIMESTAMP(), invoice.invoice_due_date) > 0 THEN 'Overdue'
            ELSE invoice.current_status
        END
    ) invoice_status,
    (
		CASE
			WHEN invoice.currency_id <> @baseCurrencyId THEN invoice.invoice_total_amount
			ELSE 0
		END
	) total_amount_fcy,
    (
        CASE
            WHEN invoice.currency_id <> @baseCurrencyId THEN invoice.invoice_total_amount * invoice.exchange_rate
            ELSE invoice.invoice_total_amount
        END
    ) total_amount,
    (
		CASE
			WHEN invoice.currency_id <> @baseCurrencyId THEN invoice.remaining_balance
			ELSE 0
		END
	) remaining_balance_fcy,
    (
        CASE
            WHEN invoice.currency_id <> @baseCurrencyId THEN invoice.remaining_balance * invoice.exchange_rate
            ELSE invoice.remaining_balance
        END
    ) remaining_balance,
    invoice.currency_id,
    currencies.symbol AS currency_symbol,
	currencies.decimal_places,
    invoice.customer_id,
    customers.name AS customer_name
FROM
    sales_invoices invoice
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
    ) invoice_outbox_latest ON invoice_outbox_latest.reference_id = invoice.id
    LEFT JOIN pub_sub_message_records invoice_outbox ON invoice_outbox.id = invoice_outbox_latest.max_id
    LEFT JOIN currencies ON currencies.id = invoice.currency_id
    LEFT JOIN customers ON customers.id = invoice.customer_id
WHERE
    invoice.business_id = @businessId
    AND invoice.invoice_date BETWEEN @fromDate AND @toDate
    AND (invoice_outbox.processing_status IS NULL OR invoice_outbox.processing_status <> 'DEAD')
	{{- if .warehouseId }} AND invoice.warehouse_id = @warehouseId {{- end }}
	{{- if .branchId }} AND invoice.branch_id = @branchId {{- end }}
`
	var results []*SalesInvoiceDetailResponse

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

	sql, err := utils.ExecTemplate(sqlTemplate, map[string]interface{}{
		"branchId":    utils.DereferencePtr(branchID, 0),
		"warehouseId": utils.DereferencePtr(warehouseID, 0),
	})
	if err != nil {
		return nil, err
	}
	db := config.GetDB()
	if err := db.WithContext(ctx).Raw(sql, map[string]interface{}{
		"businessId":     businessId,
		"baseCurrencyId": business.BaseCurrencyId,
		"branchId":       branchID,
		"warehouseId":    warehouseID,
		"fromDate":       fromDate,
		"toDate":         toDate,
	}).Scan(&results).Error; err != nil {
		return nil, err
	}
	return results, nil
}
