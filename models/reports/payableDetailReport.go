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

type PayableDetailResponse struct {
	PayableDate       time.Time       `json:"payableDate"`
	PayableStatus     string          `json:"payableStatus"`
	TransactionNumber string          `json:"transactionNumber"`
	TransactionType   string          `json:"transactionType"`
	ItemName          string          `json:"itemName"`
	ItemQty           decimal.Decimal `json:"itemQty"`
	SupplierID        *int            `json:"supplierId,omitempty"`
	SupplierName      *string         `json:"supplierName,omitempty"`
	CurrencyId        int             `json:"currencyId"`
	ItemPrice         decimal.Decimal `json:"itemPrice"`
	ItemAmount        decimal.Decimal `json:"itemAmount"`
	ItemPriceFcy      decimal.Decimal `json:"itemPriceFcy"`
	ItemAmountFcy     decimal.Decimal `json:"itemAmountFcy"`
}

func GetPayableDetailReport(ctx context.Context, startDate models.MyDateString, endDate models.MyDateString, supplierID *int, branchID *int, warehouseID *int) ([]*PayableDetailResponse, error) {

	sqlT := `
WITH LatestBillOutbox AS (
    SELECT
        reference_id,
        MAX(id) AS max_id
    FROM
        pub_sub_message_records
    WHERE
        business_id = @businessId
        AND reference_type = 'BL'
    GROUP BY
        reference_id
),
LatestSupplierCreditOutbox AS (
    SELECT
        reference_id,
        MAX(id) AS max_id
    FROM
        pub_sub_message_records
    WHERE
        business_id = @businessId
        AND reference_type = 'SC'
    GROUP BY
        reference_id
),
BillDetail AS (
    SELECT
        b.bill_date as payable_date,
        -- b.current_status as payable_status,
        (CASE
            WHEN NOT b.current_status IN ('Draft', 'Void') AND b.remaining_balance > 0
            AND DATEDIFF(UTC_TIMESTAMP(), b.bill_due_date) > 0 THEN "Overdue"
            ELSE b.current_status
        END) AS payable_status,
        b.bill_number as transaction_number,
        "Bill" as transaction_type,
        b.supplier_id,
        b.currency_id,
        b.exchange_rate,
        bd.name as item_name,
        bd.detail_qty as item_qty,
        bd.detail_total_amount as item_amount,
        bd.detail_unit_rate as item_price
    FROM
        bills b
        join bill_details bd on b.id = bd.bill_id
        LEFT JOIN LatestBillOutbox lbo ON lbo.reference_id = b.id
        LEFT JOIN pub_sub_message_records b_outbox ON b_outbox.id = lbo.max_id
    WHERE
        b.business_id = @businessId
        AND NOT b.current_status IN ('Draft', 'Void')
        AND b.bill_date BETWEEN @fromDate
        AND @toDate
        AND (b_outbox.processing_status IS NULL OR b_outbox.processing_status <> 'DEAD')
		{{- if .BranchId }} AND b.branch_id = @branchId {{- end }}
		{{- if .WarehouseId }} AND b.warehouse_id = @warehouseId {{- end }}
		{{- if .SupplierId }} AND b.supplier_id = @supplierId {{- end }}
),
CreditNoteDetail AS (
    SELECT
        sc.supplier_credit_date as payable_date,
        sc.current_status payable_status,
        sc.supplier_credit_number as transaction_number,
        "Supplier Credit" as transaction_type,
        sc.supplier_id,
        sc.currency_id,
        sc.exchange_rate,
        scd.name as item_name,
        scd.detail_qty * -1 as item_qty,
        scd.detail_total_amount *-1 as item_amount,
        scd.detail_unit_rate as item_price
    FROM
        supplier_credits sc
        join supplier_credit_details scd on sc.id = scd.supplier_credit_id
        LEFT JOIN LatestSupplierCreditOutbox lsc ON lsc.reference_id = sc.id
        LEFT JOIN pub_sub_message_records sc_outbox ON sc_outbox.id = lsc.max_id
    WHERE
        sc.business_id = @businessId
        AND sc.supplier_credit_date BETWEEN @fromDate
        AND @toDate
        AND NOT sc.current_status IN ('Draft', 'Void')
        AND (sc_outbox.processing_status IS NULL OR sc_outbox.processing_status <> 'DEAD')
		{{- if .BranchId }} AND sc.branch_id = @branchId {{- end }}
		{{- if .WarehouseId }} AND sc.warehouse_id = @warehouseId {{- end }}
		{{- if .SupplierId }} AND sc.supplier_id = @supplierId {{- end }}
),
PUnion AS (
    select
        *
    from
        BillDetail
    union
    select
        *
    from
        CreditNoteDetail
)
select
    pu.payable_date,
    pu.payable_status,
    pu.transaction_number,
    pu.transaction_type,
    pu.supplier_id,
    pu.item_name,
    pu.item_qty,
    pu.currency_id,
    (
        CASE
            WHEN pu.currency_id <> @baseCurrencyId THEN pu.item_amount
            ELSE 0
        END
    ) AS item_amount_fcy,
    (
        CASE
            WHEN pu.currency_id <> @baseCurrencyId THEN pu.item_price
            ELSE 0
        END
    ) AS item_price_fcy,
    (
        CASE
            WHEN pu.currency_id <> @baseCurrencyId THEN pu.item_amount * pu.exchange_rate
            ELSE pu.item_amount
        END
    ) AS item_amount,
    (
        CASE
            WHEN pu.currency_id <> @baseCurrencyId THEN pu.item_price * pu.exchange_rate
            ELSE pu.item_price
        END
    ) AS item_price,
    suppliers.name AS supplier_name
from
    PUnion pu
    LEFT JOIN suppliers on pu.supplier_id = suppliers.id
ORDER BY
    pu.payable_date;
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
		"SupplierId":  utils.DereferencePtr(supplierID, 0) > 0,
	})
	if err != nil {
		return nil, err
	}
	db := config.GetDB()
	var results []*PayableDetailResponse
	if err := db.WithContext(ctx).Raw(sql, map[string]interface{}{
		"businessId":     businessId,
		"baseCurrencyId": business.BaseCurrencyId,
		"branchId":       branchID,
		"warehouseId":    warehouseID,
		"supplierId":     supplierID,
		"fromDate":       startDate,
		"toDate":         endDate,
	}).Scan(&results).Error; err != nil {
		return nil, err
	}

	return results, nil
}
