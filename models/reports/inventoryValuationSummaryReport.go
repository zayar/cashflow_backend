package reports

import (
	"context"
	"errors"

	"bitbucket.org/mmdatafocus/books_backend/config"
	"bitbucket.org/mmdatafocus/books_backend/models"
	"bitbucket.org/mmdatafocus/books_backend/utils"
	"github.com/shopspring/decimal"
)

type InventoryValuationSummaryResponse struct {
	ProductID     int             `json:"productId"`
	ProductType   string          `json:"productType"`
	ProductName   string          `json:"productName"`
	Sku           string          `json:"sku"`
	ProductUnitId int             `json:"productUnitIt"`
	StockOnHand   decimal.Decimal `json:"stockOnHand"`
	AssetValue    decimal.Decimal `json:"assetValue"`
}

// SQL for ALL warehouses
// Simply SUM all qty and qty*base_unit_value up to the report date.
// This is the most reliable approach - doesn't depend on closing_qty being updated.
const sqlAllWarehouses = `
WITH StockTotals AS (
    SELECT
        product_id,
        product_type,
        SUM(qty) AS stock_on_hand,
        SUM(qty * base_unit_value) AS asset_value
    FROM stock_histories
    WHERE business_id = @businessId
      AND stock_date <= @currentDate
    GROUP BY product_id, product_type
),
AllProducts AS (
    SELECT
        id AS product_id,
        name AS product_name,
        unit_id AS product_unit_id,
        sku,
        'S' AS product_type
    FROM products
    WHERE business_id = @businessId
    UNION
    SELECT
        id AS product_id,
        name AS product_name,
        unit_id AS product_unit_id,
        sku,
        'V' AS product_type
    FROM product_variants
    WHERE business_id = @businessId
)
SELECT
    p.product_id,
    p.product_type,
    COALESCE(s.stock_on_hand, 0) as stock_on_hand,
    COALESCE(s.asset_value, 0) as asset_value,
    p.product_name,
    p.product_unit_id,
    p.sku
FROM
    AllProducts p
    LEFT JOIN StockTotals s ON p.product_id = s.product_id
        AND p.product_type = s.product_type
WHERE
    COALESCE(s.stock_on_hand, 0) != 0
    OR COALESCE(s.asset_value, 0) != 0
ORDER BY p.product_name;
`

// SQL for SPECIFIC warehouse
const sqlOneWarehouse = `
WITH StockTotals AS (
    SELECT
        product_id,
        product_type,
        SUM(qty) AS stock_on_hand,
        SUM(qty * base_unit_value) AS asset_value
    FROM stock_histories
    WHERE business_id = @businessId
      AND stock_date <= @currentDate
      AND warehouse_id = @warehouseId
    GROUP BY product_id, product_type
),
AllProducts AS (
    SELECT
        id AS product_id,
        name AS product_name,
        unit_id AS product_unit_id,
        sku,
        'S' AS product_type
    FROM products
    WHERE business_id = @businessId
    UNION
    SELECT
        id AS product_id,
        name AS product_name,
        unit_id AS product_unit_id,
        sku,
        'V' AS product_type
    FROM product_variants
    WHERE business_id = @businessId
)
SELECT
    p.product_id,
    p.product_type,
    COALESCE(s.stock_on_hand, 0) as stock_on_hand,
    COALESCE(s.asset_value, 0) as asset_value,
    p.product_name,
    p.product_unit_id,
    p.sku
FROM
    AllProducts p
    LEFT JOIN StockTotals s ON p.product_id = s.product_id
        AND p.product_type = s.product_type
WHERE
    COALESCE(s.stock_on_hand, 0) != 0
    OR COALESCE(s.asset_value, 0) != 0
ORDER BY p.product_name;
`

func GetInventoryValuationSummaryReport(ctx context.Context, currentDate models.MyDateString, warehouseId int) ([]*InventoryValuationSummaryResponse, error) {
	businessId, ok := utils.GetBusinessIdFromContext(ctx)
	if !ok || businessId == "" {
		return nil, errors.New("business id is required")
	}
	business, err := models.GetBusiness(ctx)
	if err != nil {
		return nil, errors.New("business id is required")
	}
	if err := currentDate.EndOfDayUTCTime(business.Timezone); err != nil {
		return nil, err
	}

	var results []*InventoryValuationSummaryResponse
	db := config.GetDB()

	if warehouseId == 0 {
		if err := db.WithContext(ctx).Raw(sqlAllWarehouses, map[string]interface{}{
			"businessId":  businessId,
			"currentDate": currentDate,
		}).Scan(&results).Error; err != nil {
			return nil, err
		}
	} else {
		if err := db.WithContext(ctx).Raw(sqlOneWarehouse, map[string]interface{}{
			"businessId":  businessId,
			"currentDate": currentDate,
			"warehouseId": warehouseId,
		}).Scan(&results).Error; err != nil {
			return nil, err
		}
	}

	return results, nil
}
