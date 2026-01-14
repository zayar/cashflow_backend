package reports

import (
	"context"
	"errors"

	"github.com/mmdatafocus/books_backend/config"
	"github.com/mmdatafocus/books_backend/models"
	"github.com/mmdatafocus/books_backend/utils"
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
// Compute totals from stock_histories and also include opening_stocks fallback when opening
// postings are missing from stock_histories (common in legacy/migrated datasets).
//
// This matches the behavior of Inventory Valuation By Item report which can derive opening
// balance from opening_stocks when stock_histories doesn't contain opening-stock postings.
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
      -- Only include active ledger rows (exclude reversals and rows that have been reversed).
      AND is_reversal = 0
      AND reversed_by_stock_history_id IS NULL
    GROUP BY product_id, product_type
),
HasOpeningHist AS (
    -- If stock_histories already contains opening-stock postings, do NOT add opening_stocks
    -- to avoid double counting.
    SELECT
        product_id,
        product_type
    FROM stock_histories
    WHERE business_id = @businessId
      AND stock_date <= @currentDate
      AND is_reversal = 0
      AND reversed_by_stock_history_id IS NULL
      AND reference_type IN ('POS', 'PGOS', 'PCOS')
    GROUP BY product_id, product_type
),
OpeningFallback AS (
    -- opening_stocks is not scoped by business_id, so join against products/product_variants
    -- to ensure we only include rows for this business.
    SELECT
        os.product_id,
        os.product_type,
        SUM(os.qty) AS opening_qty,
        SUM(os.qty * os.unit_value) AS opening_asset_value
    FROM opening_stocks os
    LEFT JOIN products p
        ON os.product_type = 'S'
        AND p.id = os.product_id
        AND p.business_id = @businessId
    LEFT JOIN product_variants pv
        ON os.product_type = 'V'
        AND pv.id = os.product_id
        AND pv.business_id = @businessId
    WHERE (p.id IS NOT NULL OR pv.id IS NOT NULL)
    GROUP BY os.product_id, os.product_type
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
    (
        COALESCE(s.stock_on_hand, 0)
        + CASE
            WHEN oh.product_id IS NULL THEN COALESCE(ofb.opening_qty, 0)
            ELSE 0
          END
    ) as stock_on_hand,
    (
        COALESCE(s.asset_value, 0)
        + CASE
            WHEN oh.product_id IS NULL THEN COALESCE(ofb.opening_asset_value, 0)
            ELSE 0
          END
    ) as asset_value,
    p.product_name,
    p.product_unit_id,
    p.sku
FROM
    AllProducts p
    LEFT JOIN StockTotals s ON p.product_id = s.product_id
        AND p.product_type = s.product_type
    LEFT JOIN HasOpeningHist oh ON p.product_id = oh.product_id
        AND p.product_type = oh.product_type
    LEFT JOIN OpeningFallback ofb ON p.product_id = ofb.product_id
        AND p.product_type = ofb.product_type
WHERE
    (
        COALESCE(s.stock_on_hand, 0)
        + CASE
            WHEN oh.product_id IS NULL THEN COALESCE(ofb.opening_qty, 0)
            ELSE 0
          END
    ) != 0
    OR (
        COALESCE(s.asset_value, 0)
        + CASE
            WHEN oh.product_id IS NULL THEN COALESCE(ofb.opening_asset_value, 0)
            ELSE 0
          END
    ) != 0
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
      -- Only include active ledger rows (exclude reversals and rows that have been reversed).
      AND is_reversal = 0
      AND reversed_by_stock_history_id IS NULL
    GROUP BY product_id, product_type
),
HasOpeningHist AS (
    SELECT
        product_id,
        product_type
    FROM stock_histories
    WHERE business_id = @businessId
      AND stock_date <= @currentDate
      AND warehouse_id = @warehouseId
      AND is_reversal = 0
      AND reversed_by_stock_history_id IS NULL
      AND reference_type IN ('POS', 'PGOS', 'PCOS')
    GROUP BY product_id, product_type
),
OpeningFallback AS (
    SELECT
        os.product_id,
        os.product_type,
        SUM(os.qty) AS opening_qty,
        SUM(os.qty * os.unit_value) AS opening_asset_value
    FROM opening_stocks os
    LEFT JOIN products p
        ON os.product_type = 'S'
        AND p.id = os.product_id
        AND p.business_id = @businessId
    LEFT JOIN product_variants pv
        ON os.product_type = 'V'
        AND pv.id = os.product_id
        AND pv.business_id = @businessId
    WHERE (p.id IS NOT NULL OR pv.id IS NOT NULL)
      AND os.warehouse_id = @warehouseId
    GROUP BY os.product_id, os.product_type
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
    (
        COALESCE(s.stock_on_hand, 0)
        + CASE
            WHEN oh.product_id IS NULL THEN COALESCE(ofb.opening_qty, 0)
            ELSE 0
          END
    ) as stock_on_hand,
    (
        COALESCE(s.asset_value, 0)
        + CASE
            WHEN oh.product_id IS NULL THEN COALESCE(ofb.opening_asset_value, 0)
            ELSE 0
          END
    ) as asset_value,
    p.product_name,
    p.product_unit_id,
    p.sku
FROM
    AllProducts p
    LEFT JOIN StockTotals s ON p.product_id = s.product_id
        AND p.product_type = s.product_type
    LEFT JOIN HasOpeningHist oh ON p.product_id = oh.product_id
        AND p.product_type = oh.product_type
    LEFT JOIN OpeningFallback ofb ON p.product_id = ofb.product_id
        AND p.product_type = ofb.product_type
WHERE
    (
        COALESCE(s.stock_on_hand, 0)
        + CASE
            WHEN oh.product_id IS NULL THEN COALESCE(ofb.opening_qty, 0)
            ELSE 0
          END
    ) != 0
    OR (
        COALESCE(s.asset_value, 0)
        + CASE
            WHEN oh.product_id IS NULL THEN COALESCE(ofb.opening_asset_value, 0)
            ELSE 0
          END
    ) != 0
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
