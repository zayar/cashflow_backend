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

// SQL for ALL warehouses (no warehouse filter)
const sqlAllWarehouses = `
WITH LastStockHistories AS (
    SELECT
        product_id,
        product_type,
        SUM(stock_on_hand) AS closing_qty,
        SUM(asset_value) AS closing_asset_value
    FROM (
        SELECT
            warehouse_id,
            product_id,
            product_type,
            batch_number,
            SUM(qty) OVER (
                PARTITION BY business_id, warehouse_id, product_id, product_type, batch_number
                ORDER BY stock_date, cumulative_sequence
                ROWS BETWEEN UNBOUNDED PRECEDING AND CURRENT ROW
            ) AS stock_on_hand,
            SUM(qty * base_unit_value) OVER (
                PARTITION BY business_id, warehouse_id, product_id, product_type, batch_number
                ORDER BY stock_date, cumulative_sequence
                ROWS BETWEEN UNBOUNDED PRECEDING AND CURRENT ROW
            ) AS asset_value,
            ROW_NUMBER() OVER (
                PARTITION BY business_id, warehouse_id, product_id, product_type, batch_number
                ORDER BY stock_date DESC, cumulative_sequence DESC
            ) AS rn
        FROM stock_histories
        WHERE business_id = @businessId
          AND stock_date <= @currentDate
    ) AS stock_histories_ranked
    WHERE rn = 1
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
    COALESCE(h.closing_qty, 0) as stock_on_hand,
    COALESCE(h.closing_asset_value, 0) as asset_value,
    p.product_name,
    p.product_unit_id,
    p.sku
FROM
    AllProducts p
    LEFT JOIN LastStockHistories h on p.product_id = h.product_id
        AND p.product_type = h.product_type
WHERE
    COALESCE(h.closing_qty, 0) != 0
    OR COALESCE(h.closing_asset_value, 0) != 0
ORDER BY p.product_name;
`

// SQL for SPECIFIC warehouse (with warehouse filter)
const sqlOneWarehouse = `
WITH LastStockHistories AS (
    SELECT
        product_id,
        product_type,
        SUM(stock_on_hand) AS closing_qty,
        SUM(asset_value) AS closing_asset_value
    FROM (
        SELECT
            warehouse_id,
            product_id,
            product_type,
            batch_number,
            SUM(qty) OVER (
                PARTITION BY business_id, warehouse_id, product_id, product_type, batch_number
                ORDER BY stock_date, cumulative_sequence
                ROWS BETWEEN UNBOUNDED PRECEDING AND CURRENT ROW
            ) AS stock_on_hand,
            SUM(qty * base_unit_value) OVER (
                PARTITION BY business_id, warehouse_id, product_id, product_type, batch_number
                ORDER BY stock_date, cumulative_sequence
                ROWS BETWEEN UNBOUNDED PRECEDING AND CURRENT ROW
            ) AS asset_value,
            ROW_NUMBER() OVER (
                PARTITION BY business_id, warehouse_id, product_id, product_type, batch_number
                ORDER BY stock_date DESC, cumulative_sequence DESC
            ) AS rn
        FROM stock_histories
        WHERE business_id = @businessId
          AND stock_date <= @currentDate
          AND warehouse_id = @warehouseId
    ) AS stock_histories_ranked
    WHERE rn = 1
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
    COALESCE(h.closing_qty, 0) as stock_on_hand,
    COALESCE(h.closing_asset_value, 0) as asset_value,
    p.product_name,
    p.product_unit_id,
    p.sku
FROM
    AllProducts p
    LEFT JOIN LastStockHistories h on p.product_id = h.product_id
        AND p.product_type = h.product_type
WHERE
    COALESCE(h.closing_qty, 0) != 0
    OR COALESCE(h.closing_asset_value, 0) != 0
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

	// Use explicit separate queries - no templates, no conditional args
	// This eliminates any GORM named-param expansion issues
	if warehouseId == 0 {
		// All Warehouses: only 2 named params
		if err := db.WithContext(ctx).Raw(sqlAllWarehouses, map[string]interface{}{
			"businessId":  businessId,
			"currentDate": currentDate,
		}).Scan(&results).Error; err != nil {
			return nil, err
		}
	} else {
		// Specific Warehouse: 3 named params
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
