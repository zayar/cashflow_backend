package reports

import (
	"context"
	"time"

	"github.com/mmdatafocus/books_backend/config"
	"github.com/mmdatafocus/books_backend/models"
	"github.com/mmdatafocus/books_backend/utils"
	"github.com/shopspring/decimal"
)

type ReceivableDetailResponse struct {
	ReceivableDate    time.Time       `json:"receivableDate"`
	ReceivableStatus  string          `json:"receivableStatus"`
	TransactionNumber string          `json:"transactionNumber"`
	TransactionType   string          `json:"transactionType"`
	CurrencyID        int             `json:"currencyId"`
	ItemName          string          `json:"itemName"`
	ItemQty           decimal.Decimal `json:"itemQty"`
	CustomerID        *int            `json:"customerId,omitempty"`
	CustomerName      *string         `json:"customerName,omitempty"`
	ItemPrice         decimal.Decimal `json:"itemPrice"`
	ItemAmount        decimal.Decimal `json:"itemAmount"`
	ItemPriceFcy      decimal.Decimal `json:"itemPriceFcy"`
	ItemAmountFcy     decimal.Decimal `json:"itemAmountFcy"`
}

func GetReceivableDetailReport(ctx context.Context, startDate models.MyDateString, endDate models.MyDateString, customerID *int, branchID *int, warehouseID *int) ([]*ReceivableDetailResponse, error) {
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
InvoiceDetail AS (
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
        iv.currency_id,
        iv.exchange_rate,
        ivd.name as item_name,
        ivd.detail_qty as item_qty,
        ivd.detail_total_amount as item_amount,
        ivd.detail_unit_rate as item_price
    FROM
        sales_invoices iv
        join sales_invoice_details ivd on iv.id = ivd.sales_invoice_id
        LEFT JOIN LatestInvoiceOutbox lio ON lio.reference_id = iv.id
        LEFT JOIN pub_sub_message_records outbox ON outbox.id = lio.max_id
    WHERE
        iv.business_id = @businessId
        AND iv.invoice_date BETWEEN @fromDate
        AND @toDate
        AND NOT iv.current_status IN ('Draft', 'Void')
        AND (outbox.processing_status IS NULL OR outbox.processing_status <> 'DEAD')
		{{- if .BranchId }} AND iv.branch_id = @branchId {{- end }}
		{{- if .WarehouseId }} AND iv.warehouse_id = @warehouseId {{- end }}
		{{- if .customerId }} AND iv.customer_id = @customerId {{- end }}
),
CreditNoteDetail AS (
    SELECT
        cn.credit_note_date as receivable_date,
        cn.current_status receivable_status,
        cn.credit_note_number as transaction_number,
        "customer Credit" as transaction_type,
        cn.customer_id,
        cn.currency_id,
        cn.exchange_rate,
        cnd.name as item_name,
        cnd.detail_qty * -1 as item_qty,
        cnd.detail_total_amount * -1 as item_amount,
        cnd.detail_unit_rate as item_price
    FROM
        credit_notes cn
        join credit_note_details cnd on cn.id = cnd.credit_note_id
        LEFT JOIN LatestCreditNoteOutbox lcn ON lcn.reference_id = cn.id
        LEFT JOIN pub_sub_message_records cn_outbox ON cn_outbox.id = lcn.max_id
    WHERE
        cn.business_id = @businessId
        AND cn.credit_note_date BETWEEN @fromDate
        AND @toDate
        AND NOT cn.current_status IN ('Draft', 'Void')
        AND (cn_outbox.processing_status IS NULL OR cn_outbox.processing_status <> 'DEAD')
		{{- if .BranchId }} AND cn.branch_id = @branchId {{- end }}
		{{- if .WarehouseId }} AND cn.warehouse_id = @warehouseId {{- end }}
		{{- if .customerId }} AND cn.customer_id = @customerId {{- end }}
),
RUnion AS (
    select
        *
    from
        InvoiceDetail
    union
    select
        *
    from
        CreditNoteDetail
)
select
    ru.receivable_date,
    ru.receivable_status,
    ru.transaction_number,
    ru.transaction_type,
    ru.customer_id,
    ru.item_name,
    ru.item_qty,
    ru.currency_id,
    (
        CASE
            WHEN ru.currency_id <> @baseCurrencyId THEN ru.item_amount
            ELSE 0
        END
    ) item_amount_fcy,
    (
        CASE
            WHEN ru.currency_id <> @baseCurrencyId THEN ru.item_price
            ELSE 0
        END
    ) item_price_fcy,
    (
        CASE
            WHEN ru.currency_id <> @baseCurrencyId THEN ru.item_amount * ru.exchange_rate
            ELSE ru.item_amount
        END
    ) AS item_amount,
    (
        CASE
            WHEN ru.currency_id <> @baseCurrencyId THEN ru.item_price * ru.exchange_rate
            ELSE ru.item_price
        END
    ) AS item_price,
    customers.name AS customer_name
from
    RUnion ru
    LEFT JOIN customers on ru.customer_id = customers.id
ORDER BY
    ru.receivable_date;
	`

	sql, err := utils.ExecTemplate(sqlT, map[string]interface{}{
		"BranchId":    utils.DereferencePtr(branchID, 0) > 0,
		"WarehouseId": utils.DereferencePtr(warehouseID, 0) > 0,
		"CustomerId":  utils.DereferencePtr(customerID, 0) > 0,
	})
	if err != nil {
		return nil, err
	}
	business, err := models.GetBusiness(ctx)
	if err != nil {
		return nil, err
	}

	db := config.GetDB()
	var results []*ReceivableDetailResponse
	if err := db.WithContext(ctx).Raw(sql, map[string]interface{}{
		"businessId":     business.ID,
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
