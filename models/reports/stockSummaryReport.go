package reports

import (
	"context"
	"errors"

	"github.com/mmdatafocus/books_backend/config"
	"github.com/mmdatafocus/books_backend/models"
	"github.com/mmdatafocus/books_backend/utils"
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
WITH Ledger AS (
    SELECT
        sh.product_id,
        sh.product_type,
        SUM(CASE WHEN sh.stock_date < @fromDate THEN sh.qty ELSE 0 END) AS opening_stock,
        SUM(CASE WHEN sh.stock_date BETWEEN @fromDate AND @toDate AND sh.qty > 0 THEN sh.qty ELSE 0 END) AS qty_in,
        SUM(CASE WHEN sh.stock_date BETWEEN @fromDate AND @toDate AND sh.qty < 0 THEN ABS(sh.qty) ELSE 0 END) AS qty_out
    FROM stock_histories sh
    WHERE sh.business_id = @businessId
      AND sh.is_reversal = 0
      AND sh.reversed_by_stock_history_id IS NULL
      {{- if .warehouseId }} AND sh.warehouse_id = @warehouseId {{- end }}
    GROUP BY sh.product_id, sh.product_type
),
AllProducts AS (
    SELECT id AS product_id, name AS product_name, sku AS product_sku, 'S' AS product_type
    FROM products
    WHERE business_id = @businessId AND inventory_account_id > 0
    UNION
    SELECT id AS product_id, name AS product_name, sku AS product_sku, 'V' AS product_type
    FROM product_variants
    WHERE business_id = @businessId AND inventory_account_id > 0
)
SELECT
    ap.product_name,
    ap.product_sku,
    COALESCE(l.opening_stock, 0) AS opening_stock,
    COALESCE(l.qty_in, 0) AS qty_in,
    COALESCE(l.qty_out, 0) AS qty_out,
    COALESCE(l.opening_stock, 0) + COALESCE(l.qty_in, 0) - COALESCE(l.qty_out, 0) AS closing_stock
FROM AllProducts ap
LEFT JOIN Ledger l ON ap.product_id = l.product_id AND ap.product_type = l.product_type
ORDER BY ap.product_name;
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
