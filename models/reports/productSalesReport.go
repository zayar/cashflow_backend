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

type ProductSalesReportResponse struct {
	ProductName        *string         `json:"productName,omitempty"`
	ProductSku         *string         `json:"productSku,omitempty"`
	SoldQty            decimal.Decimal `json:"soldQty"`
	TotalAmount        decimal.Decimal `json:"totalAmount"`
	TotalAmountWithTax decimal.Decimal `json:"totalAmountWithTax"`
	TotalCogs          decimal.Decimal `json:"totalCogs"`
	GrossProfit        decimal.Decimal `json:"grossProfit"`
	Margin             decimal.Decimal `json:"margin"`
}

func GetProductSalesReport(ctx context.Context, fromDate models.MyDateString, toDate models.MyDateString, branchId *int) ([]*ProductSalesReportResponse, error) {
	sqlTemplate := `
WITH InvoiceDetails as (
    SELECT
        iv_dt.product_id,
        iv_dt.product_type,
        sum(iv_dt.detail_qty) as sold_qty,
        sum(iv_dt.detail_tax_amount) as total_tax,
        sum(iv_dt.detail_total_amount) as total_amount,
        sum(iv_dt.cogs) as total_cogs
    FROM
        sales_invoices AS iv
        JOIN sales_invoice_details AS iv_dt ON iv_dt.sales_invoice_id = iv.id
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
        ) iv_outbox_latest ON iv_outbox_latest.reference_id = iv.id
        LEFT JOIN pub_sub_message_records iv_outbox ON iv_outbox.id = iv_outbox_latest.max_id
    WHERE
		business_id = @businessId
        AND invoice_date BETWEEN @fromDate AND @toDate
        {{- if .branchId }} AND branch_id = @branchId {{- end }}
        AND current_status IN ('Confirmed', 'Partial Paid', 'Paid')
        AND (iv_outbox.processing_status IS NULL OR iv_outbox.processing_status <> 'DEAD')
    group by
        iv_dt.product_id,
        iv_dt.product_type
),
AllProducts AS (
    SELECT
        id AS product_id,
        NAME AS product_name,
        unit_id AS product_unit_id,
        sku AS product_sku,
        'S' AS product_type
    FROM
        products
    UNION
    SELECT
        id AS product_id,
        NAME AS product_name,
        unit_id AS product_unit_id,
        sku AS product_sku,
        'V' AS product_type
    FROM
        product_variants
)
SELECT
    InvoiceDetails.sold_qty,
    InvoiceDetails.total_amount,
    InvoiceDetails.total_amount + InvoiceDetails.total_tax AS total_amount_with_tax,
    InvoiceDetails.total_cogs,
    AllProducts.product_name,
    AllProducts.product_sku
FROM
    InvoiceDetails
    LEFT JOIN AllProducts ON InvoiceDetails.product_id = AllProducts.product_id
    AND InvoiceDetails.product_type = AllProducts.product_type;	
`
	db := config.GetDB()
	businessId, ok := utils.GetBusinessIdFromContext(ctx)
	if !ok || businessId == "" {
		return nil, errors.New("business id is required")
	}
	business, err := models.GetBusinessById(ctx, businessId)
	if err != nil {
		return nil, err
	}
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
	sql, err := utils.ExecTemplate(sqlTemplate, map[string]interface{}{
		"branchId": utils.DereferencePtr(branchId),
	})
	if err != nil {
		return nil, err
	}

	var results []*ProductSalesReportResponse
	if err := db.WithContext(ctx).Raw(sql, map[string]interface{}{
		"businessId": businessId,
		"fromDate":   time.Time(fromDate),
		"toDate":     time.Time(toDate),
		"branchId":   branchId,
	}).Scan(&results).Error; err != nil {
		return nil, err
	}

	for _, result := range results {
		if !result.TotalAmount.IsZero() {
			margin := result.TotalAmount.Sub(result.TotalCogs).DivRound(result.TotalAmount, 4).Mul(decimal.NewFromInt(100))
			grossProfit := result.TotalAmount.Sub(result.TotalCogs)
			result.Margin = margin
			result.GrossProfit = grossProfit
		}
	}
	return results, nil
}
