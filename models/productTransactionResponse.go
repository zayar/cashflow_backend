package models

import (
	"context"
	"errors"
	"time"

	"bitbucket.org/mmdatafocus/books_backend/config"
	"bitbucket.org/mmdatafocus/books_backend/utils"
	"github.com/shopspring/decimal"
)

type ProductTransactionResponse struct {
	TransactionDate   time.Time       `json:"transactionDate"`
	TransactionID     int             `json:"transactionId"`
	TransactionType   string          `json:"transactionType"`
	TransactionNumber string          `json:"transactionNumber"`
	Status            string          `json:"status"`
	Price             decimal.Decimal `json:"price"`
	Qty               decimal.Decimal `json:"qty"`
	Total             decimal.Decimal `json:"total"`
	CurrencySymbol    string
	DecimalPlaces     DecimalPlaces
	CustomerName      string
	SupplierName      string
}

// func GetProductTransactions(ctx context.Context, productID int, productType ProductType, transactionType *string) ([]*ProductTransactionResponse, error) {

func GetProductTransactions(ctx context.Context, productID int, productType ProductType, transactionType *string) ([]*ProductTransactionResponse, error) {
	var results []*ProductTransactionResponse

	businessId, ok := utils.GetBusinessIdFromContext(ctx)
	if !ok || businessId == "" {
		return nil, errors.New("business id is required")
	}

	if err := ValidateProductId(ctx, businessId, productID, productType); err != nil {
		return nil, err
	}

	sqlT := `

WITH Transactions AS (

	{{- if .SI }}
	SELECT 
		inv.invoice_number transaction_number,
		inv.invoice_date transaction_date,
		inv.id transaction_id,
		'sales_invoices' transaction_type,
		CASE
			WHEN
				inv.current_status IN ('Confirmed' , 'Partial Paid')
					AND UTC_TIMESTAMP() >= inv.invoice_due_date
			THEN
				'Overdue'
			ELSE inv.current_status
		END status,
		inv.customer_id,
		0 supplier_id,
		inv.currency_id,
		inv_dt.detail_qty qty,
		inv_dt.detail_unit_rate price,
		inv_dt.detail_qty * inv_dt.detail_unit_rate AS total
	FROM
		sales_invoice_details inv_dt
			LEFT JOIN
		sales_invoices inv ON inv.id = inv_dt.sales_invoice_id
	WHERE
		inv_dt.product_id = @productId
			AND inv_dt.product_type = @productType 
	{{- end }}
	{{- if .MultipleTransactionType }} UNION {{- end }}

	{{- if .SO }}
	SELECT 
		so.order_number transaction_number,
		so.order_date transaction_date,
		so.id transaction_id,
		'sales_order' transaction_type,
		CASE
			WHEN
				so.current_status IN ('Confirmed' , 'Partial Paid')
					AND UTC_TIMESTAMP() >= so.expected_shipment_date
			THEN
				'Overdue'
			ELSE so.current_status
		END status,
		so.customer_id,
		0 supplier_id,
		so.currency_id,
		dt.detail_qty qty,
		dt.detail_unit_rate price,
		dt.detail_qty * dt.detail_unit_rate AS total
	FROM
		sales_order_details dt
			LEFT JOIN
		sales_orders so ON so.id = dt.sales_order_id
	WHERE
		dt.product_id = @productId
			AND dt.product_type = @productType 
	{{- end }}
	{{- if .MultipleTransactionType }} UNION {{- end }}

	{{- if .PO }}
	SELECT 
		po.order_number transaction_number,
		po.order_date transaction_date,
		po.id transaction_id,
		'purchase_order' transaction_type,
		CASE
			WHEN
				po.current_status IN ('Confirmed' , 'Partial Paid')
					AND UTC_TIMESTAMP() >= po.expected_delivery_date
			THEN
				'Overdue'
			ELSE po.current_status
		END status,
		0 customer_id,
		po.supplier_id,
		po.currency_id,
		dt.detail_qty qty,
		dt.detail_unit_rate price,
		dt.detail_qty * dt.detail_unit_rate AS total
	FROM
		purchase_order_details dt
			LEFT JOIN
		purchase_orders po ON po.id = dt.purchase_order_id
	WHERE
		dt.product_id = @productId
			AND dt.product_type = @productType 
	{{- end }}
	{{- if .MultipleTransactionType }} UNION {{- end }}

	{{- if .BL }}
	SELECT 
		bill.bill_number transaction_number,
		bill.bill_date transaction_date,
		bill.id transaction_id,
		'bills' transaction_type,
		CASE
			WHEN
				bill.current_status IN ('Confirmed' , 'Partial Paid')
					AND UTC_TIMESTAMP() >= bill.bill_due_date
			THEN
				'Overdue'
			ELSE bill.current_status
		END status,
		0 customer_id,
		bill.supplier_id,
		bill.currency_id,
		dt.detail_qty qty,
		dt.detail_unit_rate price,
		dt.detail_qty * dt.detail_unit_rate AS total
	FROM
		bill_details dt
			LEFT JOIN
		bills bill ON bill.id = dt.bill_id
	WHERE
		dt.product_id = @productId
			AND dt.product_type = @productType 
	{{- end }}
	{{- if .MultipleTransactionType }} UNION {{- end }}

	{{- if .CN }}
	SELECT 
		cn.credit_note_number transaction_number,
		cn.credit_note_date transaction_date,
		cn.id transaction_id,
		'credit_notes' transaction_type,
		cn.current_status status,
		cn.customer_id,
		0 supplier_id,
		cn.currency_id,
		dt.detail_qty qty,
		dt.detail_unit_rate price,
		dt.detail_qty * dt.detail_unit_rate AS total
	FROM
		credit_note_details dt
			LEFT JOIN
		credit_notes cn ON cn.id = dt.credit_note_id
	WHERE
		dt.product_id = @productId
			AND dt.product_type = @productType 
	{{- end }}
	{{- if .MultipleTransactionType }} UNION {{- end }}

	{{- if .SC }}
	SELECT 
		sc.supplier_credit_number transaction_number,
		sc.supplier_credit_date transaction_date,
		sc.id transaction_id,
		'supplier_credits' transaction_type,
		sc.current_status status,
		0 customer_id,
		sc.supplier_id,
		sc.currency_id,
		dt.detail_qty qty,
		dt.detail_unit_rate price,
		dt.detail_qty * dt.detail_unit_rate AS total
	FROM
		supplier_credit_details dt
			LEFT JOIN
		supplier_credits sc ON sc.id = dt.supplier_credit_id
	WHERE
		dt.product_id = @productId
			AND dt.product_type = @productType 
	{{- end }}
)
SELECT
{{- if or .SO .SI .CN }}
	customers.name AS customer_name,
{{- end }}
{{- if or .PO .BL .SC }}
	suppliers.name AS supplier_name,
{{- end }}
	Transactions.*,
	currencies.symbol currency_symbol,
	currencies.decimal_places
FROM Transactions
{{- if or .SO .SI .CN }}
	LEFT JOIN customers ON Transactions.customer_id = customers.id
{{- end }}
{{- if or .PO .BL .SC }}
	LEFT JOIN suppliers ON Transactions.supplier_id = suppliers.id
{{- end }}
	LEFT JOIN currencies ON Transactions.currency_id = currencies.id
ORDER BY Transactions.transaction_date
	`

	sql, err := utils.ExecTemplate(sqlT, map[string]interface{}{
		"SI":                      transactionType == nil || *transactionType == "SI",
		"SO":                      transactionType == nil || *transactionType == "SO",
		"PO":                      transactionType == nil || *transactionType == "PO",
		"BL":                      transactionType == nil || *transactionType == "BL",
		"CN":                      transactionType == nil || *transactionType == "CN",
		"SC":                      transactionType == nil || *transactionType == "SC",
		"MultipleTransactionType": transactionType == nil,
	})
	if err != nil {
		return nil, err
	}
	db := config.GetDB()
	if err := db.WithContext(ctx).Raw(sql, map[string]interface{}{
		"productId":   productID,
		"productType": productType,
	}).Scan(&results).Error; err != nil {
		return nil, err
	}
	return results, nil
}
