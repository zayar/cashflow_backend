package reports

import (
	"context"

	"github.com/mmdatafocus/books_backend/config"
	"github.com/mmdatafocus/books_backend/models"
	"github.com/mmdatafocus/books_backend/utils"
	"github.com/shopspring/decimal"
)

type PurchasesByProductResponse struct {
	ProductID               int             `json:"productId"`
	ProductName             *string         `json:"productName,omitempty"`
	ProductSku              *string         `json:"productSku,omitempty"`
	QtyPurchased            decimal.Decimal `json:"qtyPurchased"`
	TotalAmount             decimal.Decimal `json:"totalAmount"`
	TotalAmountWithDiscount decimal.Decimal `json:"totalAmountWithDiscount"`
	AveragePrice            decimal.Decimal `json:"averagePrice"`
}

func GetPurchasesByProductReport(ctx context.Context, fromDate models.MyDateString, toDate models.MyDateString, branchId *int, warehouseId *int) ([]*PurchasesByProductResponse, error) {
	sqlT := `
WITH BillDetails AS (
    SELECT
        sum(bd.detail_qty) qty,
        sum(
            (
                CASE
                    WHEN b.is_tax_inclusive = 1 THEN bd.detail_total_amount - bd.detail_tax_amount
                    ELSE bd.detail_total_amount
                END
            ) * (
                CASE
                    WHEN b.currency_id <> @baseCurrencyId THEN b.exchange_rate
                    ELSE 1
                END
            )
        ) adjusted_total_amount,
        sum(
            CASE
                when b.currency_id <> @baseCurrencyId THEN bd.detail_discount_amount * b.exchange_rate
                ELSE bd.detail_discount_amount
            END
        ) adjusted_discount_amount,
        avg(
            CASE
                when b.currency_id <> @baseCurrencyId THEN bd.detail_unit_rate * b.exchange_rate
                ELSE bd.detail_unit_rate
            END
        ) adjusted_average_price,
        bd.product_id,
        bd.product_type
    FROM
        bills b
        JOIN bill_details bd on bd.bill_id = b.id
    WHERE
		b.business_id = @businessId
		AND b.bill_date BETWEEN @fromDate AND @toDate
        AND b.current_status IN ('Confirmed', 'Paid', 'Partial Paid')
		{{- if .branchId }} AND branch_id = @branchId {{- end }}
		{{- if .warehouseId }} AND warehouse_id = @warehouseId {{- end }}
    GROUP BY
        bd.product_id,
        bd.product_type
),
AllProducts AS (
    SELECT
        id AS product_id,
        NAME AS product_name,
       -- unit_id AS product_unit_id,
        sku AS product_sku,
        'S' AS product_type
    FROM
        products
    UNION
    SELECT
        id AS product_id,
        NAME AS product_name,
       -- unit_id AS product_unit_id,
        sku AS product_sku,
        'V' AS product_type
    FROM
        product_variants
)

select
    p.product_id,
    p.product_type,
    p.product_name,
    p.product_sku,
    d.qty qty_purchased,
    d.adjusted_total_amount total_amount,
    d.adjusted_total_amount + d.adjusted_discount_amount total_amount_with_discount,
    d.adjusted_average_price average_price
from
    BillDetails d
    LEFT JOIN AllProducts p ON p.product_id = d.product_id AND p.product_type = d.product_type
ORDER BY p.product_name
	`
	var results []*PurchasesByProductResponse
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
		"branchId":    utils.DereferencePtr(branchId, 0) > 0,
		"warehouseId": utils.DereferencePtr(warehouseId, 0) > 0,
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
		"warehouseId":    warehouseId,
	}).Scan(&results).Error; err != nil {
		return nil, err
	}
	return results, nil
}
