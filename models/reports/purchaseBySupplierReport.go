package reports

import (
	"context"

	"github.com/mmdatafocus/books_backend/config"
	"github.com/mmdatafocus/books_backend/models"
	"github.com/mmdatafocus/books_backend/utils"
	"github.com/shopspring/decimal"
)

type PurchasesBySupplierResponse struct {
	SupplierID            int             `json:"SupplierId"`
	SupplierName          *string         `json:"SupplierName,omitempty"`
	BillCount             int             `json:"BillCount"`
	TotalPurchases        decimal.Decimal `json:"TotalPurchases"`
	TotalPurchasesWithTax decimal.Decimal `json:"TotalPurchasesWithTax"`
}

func GetPurchasesBySupplierReport(ctx context.Context, fromDate models.MyDateString, toDate models.MyDateString, branchId *int) ([]*PurchasesBySupplierResponse, error) {
	sqlT := `
WITH BillDetails AS (
    select
        b.supplier_id,
        count(b.id) bill_count,
        sum(
            (
                CASE
                    WHEN b.currency_id <> @baseCurrencyId THEN b.bill_total_amount * b.exchange_rate
                    ELSE b.bill_total_amount
                END
            )
        ) adjustedAmount,
        sum(
            (
                CASE
                    WHEN b.currency_id <> @baseCurrencyId THEN b.bill_total_tax_amount * b.exchange_rate
                    ELSE b.bill_total_tax_amount
                END
            )
        ) adjustedTaxAmount
    from
        bills b
    WHERE
		b.business_id = @businessId
        AND b.current_status IN ('Confirmed', 'Paid', 'Partial Paid')
		AND b.bill_date BETWEEN @fromDate AND @toDate
		{{- if .branchId }} AND branch_id = @branchId {{- end }}
    GROUP BY
        supplier_id
)
SELECT
    suppliers.name supplier_name,
    bd.supplier_id,
    bd.bill_count,
    bd.adjustedAmount total_purchases_with_tax,
    bd.adjustedAmount - bd.adjustedTaxAmount total_purchases
from
    BillDetails bd
    LEFT JOIN suppliers ON suppliers.id = bd.supplier_id
ORDER BY suppliers.name;
	`
	var results []*PurchasesBySupplierResponse
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
	sql, err := utils.ExecTemplate(sqlT, map[string]interface{}{
		"branchId": utils.DereferencePtr(branchId, 0) > 0,
	})
	if err != nil {
		return nil, err
	}
	db := config.GetDB()
	if err := db.WithContext(ctx).Raw(sql, map[string]interface{}{
		"businessId":     business.ID,
		"baseCurrencyId": business.BaseCurrencyId,
		"fromDate":       fromDate,
		"toDate":         toDate,
		"branchId":       branchId,
	}).Scan(&results).Error; err != nil {
		return nil, err
	}
	return results, nil
}
