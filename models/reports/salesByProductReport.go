package reports

import (
	"context"
	"errors"

	"bitbucket.org/mmdatafocus/books_backend/config"
	"bitbucket.org/mmdatafocus/books_backend/models"
	"bitbucket.org/mmdatafocus/books_backend/utils"
	"github.com/shopspring/decimal"
)

type SalesByProductResponse struct {
	ProductName             *string         `json:"productName,omitempty"`
	ProductSku              *string         `json:"productSku,omitempty"`
	SoldQty                 decimal.Decimal `json:"soldQty"`
	TotalAmount             decimal.Decimal `json:"totalAmount"`
	TotalAmountWithDiscount decimal.Decimal `json:"totalAmountWithDiscount"`
	AveragePrice            decimal.Decimal `json:"averagePrice"`
}

func GetSalesByProductReport(ctx context.Context, fromDate models.MyDateString, toDate models.MyDateString, branchId *int, warehouseId *int, sku *string, productName *string) ([]*SalesByProductResponse, error) {
	sqlT := `
with InvoiceDetails as (
SELECT 
    iv_dt.product_id,
    iv_dt.product_type,
    SUM(iv_dt.detail_qty) AS sold_qty,
    AVG(iv_dt.detail_unit_rate * (CASE
        WHEN iv.currency_id = @baseCurrencyId THEN 1
        ELSE iv.exchange_rate
    END)) AS average_price,
    SUM((CASE
        WHEN iv.is_tax_inclusive = 1 THEN iv_dt.detail_total_amount - iv_dt.detail_tax_amount
        ELSE iv_dt.detail_total_amount
    END) * (CASE
        WHEN iv.currency_id = @baseCurrencyId THEN 1
        ELSE iv.exchange_rate
    END)) AS total_amount,
    SUM(iv_dt.detail_discount_amount * (CASE
        WHEN iv.currency_id = @baseCurrencyId THEN 1
        ELSE iv.exchange_rate
    END)) AS total_discount_amount
FROM
    sales_invoices AS iv
        JOIN
    sales_invoice_details AS iv_dt ON iv_dt.sales_invoice_id = iv.id
WHERE
    business_id = @businessId
        AND invoice_date BETWEEN @fromDate AND @toDate
        AND current_status IN ('Confirmed' , 'Partial Paid', 'Paid')
        {{- if .branchId }} AND branch_id = @branchId {{- end }}
        {{- if .warehouseId }} AND warehouse_id = @warehouseId {{- end }}
GROUP BY iv_dt.product_id , iv_dt.product_type
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
    WHERE
        1 = 1
    {{- if .productName }} AND name LIKE @productName {{- end }}
    {{- if .sku }} AND sku = @sku {{- end }}
    UNION
    SELECT
        id AS product_id,
        NAME AS product_name,
        -- unit_id AS product_unit_id,
        sku AS product_sku,
        'V' AS product_type
    FROM
        product_variants
    WHERE
        1 = 1
    {{- if .productName }} AND name LIKE @productName {{- end }}
    {{- if .sku }} AND sku = @sku {{- end }}
)
SELECT
    AllProducts.product_id,
    AllProducts.product_name,
    AllProducts.product_sku,
    AllProducts.product_type,
    InvoiceDetails.sold_qty,
    InvoiceDetails.average_price,
    InvoiceDetails.total_amount,
    InvoiceDetails.total_amount + InvoiceDetails.total_discount_amount AS total_amount_with_discount
FROM
    InvoiceDetails -- LEFT
    JOIN AllProducts ON InvoiceDetails.product_id = AllProducts.product_id
    AND InvoiceDetails.product_type = AllProducts.product_type;    
`
	db := config.GetDB()
	business, err := models.GetBusiness(ctx)
	if err != nil {
		return nil, errors.New("business id is required")
	}
	businessId := business.ID.String()
	if err := fromDate.StartOfDayUTCTime(business.Timezone); err != nil {
		return nil, err
	}
	if err := toDate.EndOfDayUTCTime(business.Timezone); err != nil {
		return nil, err
	}

	if branchId != nil && *branchId != 0 {
		if err := utils.ValidateResourceId[models.Branch](ctx, businessId, branchId); err != nil {
			return nil, errors.New("branch not found")
		}
	}

	// execting sql template to get raw sql
	sql, err := utils.ExecTemplate(sqlT, map[string]interface{}{
		"branchId":    utils.DereferencePtr(branchId),
		"sku":         utils.DereferencePtr(sku),
		"productName": utils.DereferencePtr(productName),
		"warehouseId": utils.DereferencePtr(warehouseId),
	})
	if err != nil {
		return nil, err
	}

	var results []*SalesByProductResponse
	if err := db.WithContext(ctx).Raw(sql, map[string]interface{}{
		"businessId":     businessId,
		"fromDate":       fromDate,
		"toDate":         toDate,
		"baseCurrencyId": business.BaseCurrencyId,
		"branchId":       branchId,
		"sku":            sku,
		"productName":    "%" + utils.DereferencePtr(productName) + "%",
		"warehouseId":    warehouseId,
	}).Scan(&results).Error; err != nil {
		return nil, err
	}

	return results, nil
}
