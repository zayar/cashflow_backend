package reports

import (
	"context"
	"errors"

	"bitbucket.org/mmdatafocus/books_backend/config"
	"bitbucket.org/mmdatafocus/books_backend/models"
	"bitbucket.org/mmdatafocus/books_backend/utils"
	"github.com/shopspring/decimal"
)

type StockSummaryReportResponse struct {
	ProductName  string          `json:"productName,omitempty"`
	ProductSku   string          `json:"productSku,omitempty"`
	OpeningStock decimal.Decimal `json:"openingStock"`
	QtyIn        decimal.Decimal `json:"qtyIn"`
	QtyOut       decimal.Decimal `json:"qtyOut"`
	ClosingStock decimal.Decimal `json:"closingStock"`
}

func GetStockSummaryReport(ctx context.Context, fromDate models.MyDateString, toDate models.MyDateString, warehouseId *int) ([]*StockSummaryReportResponse, error) {

	sqlT := `
WITH InOutQty AS (
    SELECT
        product_id,
        product_type,
        SUM(opening_qty) + SUM(received_qty) + SUM(adjusted_qty_in) + SUM(transfer_qty_in) AS qty_in,
        SUM(sale_qty) + SUM(ABS(adjusted_qty_out)) + SUM(ABS(transfer_qty_out)) AS qty_out
    FROM
        stock_summary_daily_balances
    WHERE
        business_id = @businessId
        AND transaction_date BETWEEN @fromDate
        AND @toDate
        {{- if .warehouseId }} AND warehouse_id = @warehouseId {{- end }}
    GROUP BY
        product_id,
        product_type
),
OpeningStock AS (
    SELECT
        product_id,
        product_type,
        SUM(current_qty) AS opening_stock
    FROM
        stock_summary_daily_balances
    WHERE
        business_id = @businessId
        AND transaction_date < @fromDate
        {{- if .warehouseId }} AND warehouse_id = @warehouseId {{- end }}
    GROUP BY
        product_id,
        product_type
),
CombinedProducts AS (
    SELECT product_id, product_type FROM InOutQty
    UNION
    SELECT product_id, product_type FROM OpeningStock
),
AllProducts AS (
    SELECT
        id AS product_id,
        name AS product_name,
        sku AS product_sku,
        'S' AS product_type
    FROM
        products
    WHERE
        business_id = @businessId 
        AND inventory_account_id > 0
    UNION
    SELECT
        id AS product_id,
        name AS product_name,
        sku AS product_sku,
        'V' AS product_type
    FROM
        product_variants
    WHERE
        business_id = @businessId 
        AND inventory_account_id > 0
)
SELECT
    CombinedProducts.product_id,
    CombinedProducts.product_type,
    AllProducts.product_sku,
    AllProducts.product_name,
    COALESCE(InOutQty.qty_in, 0) AS qty_in,
    COALESCE(InOutQty.qty_out, 0) AS qty_out,
    COALESCE(OpeningStock.opening_stock, 0) AS opening_stock,
    COALESCE(OpeningStock.opening_stock, 0) + COALESCE(InOutQty.qty_in, 0) - COALESCE(InOutQty.qty_out, 0) AS closing_stock
FROM
    CombinedProducts
LEFT JOIN InOutQty
    ON CombinedProducts.product_id = InOutQty.product_id
    AND CombinedProducts.product_type = InOutQty.product_type
LEFT JOIN OpeningStock
    ON CombinedProducts.product_id = OpeningStock.product_id
    AND CombinedProducts.product_type = OpeningStock.product_type
LEFT JOIN AllProducts
    ON CombinedProducts.product_id = AllProducts.product_id
    AND CombinedProducts.product_type = AllProducts.product_type
ORDER BY AllProducts.product_name;

`
	var _ = `
WITH inOutQty AS (
    SELECT
        product_id,
        product_type,
        SUM(opening_qty) + SUM(received_qty) + SUM(adjusted_qty_in) + SUM(transfer_qty_in) AS qty_in,
        SUM(sale_qty) + SUM(ABS(adjusted_qty_out)) + SUM(ABS(transfer_qty_out)) AS qty_out
    FROM
        stock_summary_daily_balances
    WHERE
        business_id = @businessId
        AND transaction_date BETWEEN @fromDate AND @toDate
        {{- if .warehouseId }} AND warehouse_id = @warehouseId {{- end }}
    GROUP BY
        product_id, product_type
),
StockSummary AS (
    SELECT
        inOutQty.*,
        COALESCE(openingStocks.opening_stock, 0) AS opening_stock,
        COALESCE(openingStocks.opening_stock, 0) + inOutQty.qty_in - inOutQty.qty_out AS closing_stock
    FROM
        (
        ) AS inOutQty
        LEFT JOIN (
            SELECT
                product_id,
                product_type,
                SUM(current_qty) AS opening_stock
            FROM
                stock_summary_daily_balances
            WHERE
				business_id = @businessId
                AND transaction_date < @fromDate
                {{- if .warehouseId }} AND warehouse_id = @warehouseId {{- end }}
            GROUP BY
                product_id, product_type
        ) AS openingStocks ON openingStocks.product_id = inOutQty.product_id
        AND openingStocks.product_type = inOutQty.product_type
),
AllProducts AS (
    SELECT
        id AS product_id,
        name AS product_name,
        sku AS product_sku,
        'S' AS product_type
    FROM
        products
    UNION
    SELECT
        id AS product_id,
        name AS product_name,
        sku AS product_sku,
        'V' AS product_type
    FROM
        product_variants
)
SELECT
    AllProducts.product_id,
    AllProducts.product_type,
    AllProducts.product_sku,
    AllProducts.product_name,
    StockSummary.opening_stock,
    StockSummary.qty_in,
    StockSummary.qty_out,
    StockSummary.closing_stock
FROM
    StockSummary
    LEFT JOIN AllProducts ON StockSummary.product_id = AllProducts.product_id
    AND StockSummary.product_type = AllProducts.product_type
`
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

	if warehouseId != nil && *warehouseId > 0 {
		if err := utils.ValidateResourceId[models.Warehouse](ctx, businessId, *warehouseId); err != nil {
			return nil, errors.New("warehouse not found")
		}
	}

	sql, err := utils.ExecTemplate(sqlT, map[string]interface{}{
		"warehouseId": utils.DereferencePtr(warehouseId),
	})
	if err != nil {
		return nil, err
	}

	var results []*StockSummaryReportResponse
	db := config.GetDB()
	// IMPORTANT:
	// The SQL template conditionally removes the warehouse filter when warehouseId is nil/0.
	// GORM expands named params to positional placeholders per-occurrence. If we pass a named param
	// that no longer exists in the final SQL (e.g. warehouseId), the driver can error with:
	// "sql: expected N arguments, got N+1".
	//
	// Therefore, only include warehouseId when the placeholder is present.
	args := map[string]interface{}{
		"fromDate":   fromDate,
		"toDate":     toDate,
		"businessId": businessId,
	}
	if warehouseId != nil && *warehouseId != 0 {
		args["warehouseId"] = warehouseId
	}
	if err := db.WithContext(ctx).Raw(sql, args).Scan(&results).Error; err != nil {
		return nil, err
	}

	return results, nil
}
