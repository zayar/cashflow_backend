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

type PayableSummaryResponse struct {
	PayableDate       time.Time       `json:"payableDate"`
	PayableStatus     string          `json:"payableStatus"`
	TransactionNumber string          `json:"transactionNumber"`
	TransactionType   string          `json:"transactionType"`
	SupplierID        *int            `json:"supplierId,omitempty"`
	SupplierName      *string         `json:"supplierName,omitempty"`
	CurrencyId        int             `json:"currencyId"`
	PayableAmount     decimal.Decimal `json:"payableAmount"`
	PayableBalance    decimal.Decimal `json:"payableBalance"`
	PayableAmountFcy  decimal.Decimal `json:"payableAmountFcy"`
	PayableBalanceFcy decimal.Decimal `json:"payableBalanceFcy"`
}

func GetPayableSummaryReport(ctx context.Context, startDate models.MyDateString, endDate models.MyDateString, supplierID *int, branchID *int, warehouseID *int) ([]*PayableSummaryResponse, error) {
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
BillSummary as (
    SELECT
        b.bill_date as payable_date,
        CASE
            WHEN NOT b.current_status IN ('Draft', 'Void') AND b.remaining_balance > 0
            AND DATEDIFF(UTC_TIMESTAMP(), b.bill_due_date) > 0 THEN "Overdue"
            ELSE b.current_status
        END AS payable_status,
        b.bill_number as transaction_number,
        "Bill" as transaction_type,
        b.supplier_id,
        b.bill_total_amount payable_amount,
        b.remaining_balance payable_balance,
        b.currency_id,
        b.exchange_rate
    FROM
        bills b
        LEFT JOIN LatestBillOutbox lbo ON lbo.reference_id = b.id
        LEFT JOIN pub_sub_message_records b_outbox ON b_outbox.id = lbo.max_id
    WHERE
        b.business_id = @businessId
        AND NOT b.current_status IN ('Draft', 'Void')
        AND b.bill_date BETWEEN @fromDate AND @toDate
        AND (b_outbox.processing_status IS NULL OR b_outbox.processing_status <> 'DEAD')
		{{- if .BranchId }} AND b.branch_id = @branchId {{- end }}
		{{- if .WarehouseId }} AND b.warehouse_id = @warehouseId {{- end }}
		{{- if .SupplierId }} AND b.supplier_id = @supplierId {{- end }}
),

SupplierCreditSummary as (
    SELECT
        sc.supplier_credit_date as payable_date,
        sc.current_status AS payable_status,
        sc.supplier_credit_number as transaction_number,
        "Supplier Credit" as transaction_type,
        sc.supplier_id,
        sc.supplier_credit_total_amount * -1 AS payable_amount,
        sc.remaining_balance * -1 AS payable_balance,
        sc.currency_id,
        sc.exchange_rate
    FROM
        supplier_credits sc
        LEFT JOIN LatestSupplierCreditOutbox lsc ON lsc.reference_id = sc.id
        LEFT JOIN pub_sub_message_records sc_outbox ON sc_outbox.id = lsc.max_id
    WHERE
        sc.business_id = @businessId
        AND NOT sc.current_status IN ('Draft', 'Void')
        AND sc.supplier_credit_date BETWEEN @fromDate AND @toDate
        AND (sc_outbox.processing_status IS NULL OR sc_outbox.processing_status <> 'DEAD')
		{{- if .BranchId }} AND sc.branch_id = @branchId {{- end }}
		{{- if .WarehouseId }} AND sc.warehouse_id = @warehouseId {{- end }}
		{{- if .SupplierId }} AND sc.supplier_id = @supplierId {{- end }}
),

PUnion AS (
        SELECT
            *
        from
            BillSummary
        UNION
        SELECT
            *
        from
            SupplierCreditSummary
)

select
pu.payable_date,
pu.payable_status,
pu.transaction_number,
pu.transaction_type,
pu.supplier_id,
suppliers.name supplier_name,
    (
        CASE
            WHEN pu.currency_id <> @baseCurrencyId THEN pu.payable_amount
            ELSE 0
        END
    ) payable_amount_fcy,
    (
        CASE
            WHEN pu.currency_id <> @baseCurrencyId THEN pu.payable_balance
            ELSE 0
        END
    ) payable_balance_fcy,
    (
        CASE
            WHEN pu.currency_id <> @baseCurrencyId THEN pu.payable_amount * pu.exchange_rate
            ELSE pu.payable_amount
        END
    ) AS payable_amount,
    (
        CASE
            WHEN pu.currency_id <> @baseCurrencyId THEN pu.payable_balance * pu.exchange_rate
            ELSE pu.payable_balance
        END
    ) AS payable_balance,
pu.currency_id
from PUnion pu
left join suppliers on suppliers.id = pu.supplier_id
order by pu.payable_date
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
	var results []*PayableSummaryResponse
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
