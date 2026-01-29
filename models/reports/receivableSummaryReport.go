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

type ReceivableSummaryResponse struct {
	ReceivableDate       time.Time       `json:"receivableDate"`
	ReceivableStatus     string          `json:"receivableStatus"`
	TransactionNumber    string          `json:"transactionNumber"`
	TransactionType      string          `json:"transactionType"`
	CustomerID           *int            `json:"customerId,omitempty"`
	CustomerName         *string         `json:"customerName,omitempty"`
	CurrencyId           int             `json:"currencyId"`
	ReceivableAmount     decimal.Decimal `json:"receivableAmount"`
	ReceivableBalance    decimal.Decimal `json:"receivableBalance"`
	ReceivableAmountFcy  decimal.Decimal `json:"receivableAmountFcy"`
	ReceivableBalanceFcy decimal.Decimal `json:"receivableBalanceFcy"`
}

func GetReceivableSummaryReport(ctx context.Context, startDate models.MyDateString, endDate models.MyDateString, customerID *int, branchID *int, warehouseID *int) ([]*ReceivableSummaryResponse, error) {
	sqlT := `
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
LatestCreditNoteOutbox AS (
    SELECT
        reference_id,
        MAX(id) AS max_id
    FROM
        pub_sub_message_records
    WHERE
        business_id = @businessId
        AND reference_type = 'CN'
    GROUP BY
        reference_id
),
InvoiceSummary as (
    SELECT
        iv.invoice_date as receivable_date,
        (CASE
            WHEN NOT iv.current_status IN ('Draft', 'Void') AND iv.remaining_balance > 0
            AND DATEDIFF(UTC_TIMESTAMP(), iv.invoice_due_date) > 0 THEN "Overdue"
            ELSE iv.current_status
        END) AS receivable_status,
        iv.invoice_number as transaction_number,
        "Invoice" as transaction_type,
        iv.customer_id,
        iv.invoice_total_amount receivable_amount,
        iv.remaining_balance receivable_balance,
        iv.currency_id,
        iv.exchange_rate
    FROM
        sales_invoices iv
        LEFT JOIN LatestInvoiceOutbox lio ON lio.reference_id = iv.id
        LEFT JOIN pub_sub_message_records outbox ON outbox.id = lio.max_id
    WHERE
        iv.business_id = @businessId
        AND iv.invoice_date BETWEEN @fromDate AND @toDate
        AND NOT iv.current_status IN ('Draft', 'Void')
        AND (outbox.processing_status IS NULL OR outbox.processing_status <> 'DEAD')
		{{- if .BranchId }} AND iv.branch_id = @branchId {{- end }}
		{{- if .WarehouseId }} AND iv.warehouse_id = @warehouseId {{- end }}
		{{- if .customerId }} AND iv.customer_id = @customerId {{- end }}
),

CreditNoteSummary as (
    SELECT
        cn.credit_note_date as receivable_date,
        cn.current_status AS receivable_status,
        cn.credit_note_number as transaction_number,
        "Credit Note" as transaction_type,
        cn.customer_id,
        (-1 * cn.credit_note_total_amount) AS receivable_amount, 
        (-1 * cn.remaining_balance) AS receivable_balance, 
        cn.currency_id,
        cn.exchange_rate
    FROM
        credit_notes cn
        LEFT JOIN LatestCreditNoteOutbox lcn ON lcn.reference_id = cn.id
        LEFT JOIN pub_sub_message_records cn_outbox ON cn_outbox.id = lcn.max_id
    WHERE
        cn.business_id = @businessId
        AND cn.credit_note_date BETWEEN @fromDate AND @toDate
		AND NOT cn.current_status IN ('Draft', 'Void')
        AND (cn_outbox.processing_status IS NULL OR cn_outbox.processing_status <> 'DEAD')
		{{- if .BranchId }} AND cn.branch_id = @branchId {{- end }}
		{{- if .WarehouseId }} AND cn.warehouse_id = @warehouseId {{- end }}
		{{- if .customerId }} AND cn.customer_id = @customerId {{- end }}
),

RUnion AS (
        SELECT
            *
        from
            InvoiceSummary
        UNION
        SELECT
            *
        from
            CreditNoteSummary
)

select
ru.receivable_date,
ru.receivable_status,
ru.transaction_number,
ru.transaction_type,
ru.customer_id,
customers.name customer_name,
	(
		CASE
			WHEN ru.currency_id <> @baseCurrencyId THEN ru.receivable_amount
			ELSE 0
		END
	) receivable_amount_fcy,
	(
		CASE
			WHEN ru.currency_id <> @baseCurrencyId THEN ru.receivable_balance
			ELSE 0
		END
	) receivable_balance_fcy,
    (
        CASE
            WHEN ru.currency_id <> @baseCurrencyId THEN ru.receivable_amount * ru.exchange_rate
            ELSE ru.receivable_amount
        END
    ) AS receivable_amount,
    (
        CASE
            WHEN ru.currency_id <> @baseCurrencyId THEN ru.receivable_balance * ru.exchange_rate
            ELSE ru.receivable_balance
        END
    ) AS receivable_balance,
ru.currency_id
from RUnion ru
left join customers on customers.id = ru.customer_id
order by ru.receivable_date
    `
	businessId, ok := utils.GetBusinessIdFromContext(ctx)
	if !ok || businessId == "" {
		return nil, errors.New("business id is required")
	}
	business, err := models.GetBusiness(ctx)
	if err != nil {
		return nil, errors.New("business id is required")
	}
	if err := startDate.StartOfDayUTCTime(business.Timezone); err != nil {
		return nil, err
	}
	if err := endDate.EndOfDayUTCTime(business.Timezone); err != nil {
		return nil, err
	}

	sql, err := utils.ExecTemplate(sqlT, map[string]interface{}{
		"BranchId":    utils.DereferencePtr(branchID, 0) > 0,
		"WarehouseId": utils.DereferencePtr(warehouseID, 0) > 0,
		"CustomerId":  utils.DereferencePtr(customerID, 0) > 0,
	})
	if err != nil {
		return nil, err
	}
	db := config.GetDB()
	var results []*ReceivableSummaryResponse
	if err := db.WithContext(ctx).Raw(sql, map[string]interface{}{
		"businessId":     businessId,
		"baseCurrencyId": business.BaseCurrencyId,
		"branchId":       branchID,
		"warehouseId":    warehouseID,
		"customerId":     customerID,
		"fromDate":       startDate,
		"toDate":         endDate,
	}).Scan(&results).Error; err != nil {
		return nil, err
	}

	return results, nil
}
