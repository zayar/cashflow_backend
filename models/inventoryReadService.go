package models

import (
	"context"
	"errors"
	"time"

	"github.com/mmdatafocus/books_backend/config"
	"github.com/mmdatafocus/books_backend/utils"
	"github.com/shopspring/decimal"
)

type InventorySummaryResponse struct {
	ProductId      int             `json:"product_id"`
	ProductName    *string         `json:"productName,omitempty"`
	ProductUnitId  int             `json:"productUnit,omitempty"`
	ProductSku     *string         `json:"productSku,omitempty"`
	OpeningQty     decimal.Decimal `json:"openingQty"`
	OrderQty       decimal.Decimal `json:"orderQty"`
	ReceivedQty    decimal.Decimal `json:"receivedQty"`
	TransferQtyIn  decimal.Decimal `json:"transfer_qty_in"`
	TransferQtyOut decimal.Decimal `json:"transfer_qty_out"`
	AdjustedQtyIn  decimal.Decimal `json:"adjusted_qty_in"`
	AdjustedQtyOut decimal.Decimal `json:"adjusted_qty_out"`
	SaleQty        decimal.Decimal `json:"saleQty"`
	CommittedQty   decimal.Decimal `json:"committedQty"`
	CurrentQty     decimal.Decimal `json:"currentQty"`
	AvailableStock decimal.Decimal `json:"availableStock"`
}

type WarehouseInventoryResponse struct {
	WarehouseId    int             `json:"warehouse_id"`
	WarehouseName  *string         `json:"warehouseName,omitempty"`
	ProductId      int             `json:"product_id"`
	ProductName    *string         `json:"productName,omitempty"`
	ProductUnitId  int             `json:"productUnit,omitempty"`
	ProductSku     *string         `json:"productSku,omitempty"`
	OpeningQty     decimal.Decimal `json:"openingQty"`
	OrderQty       decimal.Decimal `json:"orderQty"`
	ReceivedQty    decimal.Decimal `json:"receivedQty"`
	TransferQtyIn  decimal.Decimal `json:"transfer_qty_in"`
	TransferQtyOut decimal.Decimal `json:"transfer_qty_out"`
	AdjustedQtyIn  decimal.Decimal `json:"adjusted_qty_in"`
	AdjustedQtyOut decimal.Decimal `json:"adjusted_qty_out"`
	SaleQty        decimal.Decimal `json:"saleQty"`
	CommittedQty   decimal.Decimal `json:"committedQty"`
	CurrentQty     decimal.Decimal `json:"currentQty"`
	AvailableStock decimal.Decimal `json:"availableStock"`
}

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
const inventorySummarySqlAllWarehouses = `
WITH Ledger AS (
    SELECT
        sh.product_id,
        sh.product_type,
        SUM(CASE WHEN sh.reference_type IN ('POS','PGOS','PCOS') THEN sh.qty ELSE 0 END) AS opening_qty,
        SUM(CASE WHEN sh.reference_type IN ('BL','CN') AND sh.qty > 0 THEN sh.qty ELSE 0 END) AS received_qty,
        SUM(CASE WHEN sh.reference_type = 'IV' THEN ABS(sh.qty) ELSE 0 END) AS sale_qty,
        SUM(CASE WHEN sh.reference_type = 'TO' AND sh.is_transfer_in = true THEN sh.qty ELSE 0 END) AS transfer_qty_in,
        SUM(CASE WHEN sh.reference_type = 'TO' AND sh.is_transfer_in = false THEN ABS(sh.qty) ELSE 0 END) AS transfer_qty_out,
        SUM(CASE WHEN sh.reference_type = 'IVAQ' AND sh.qty > 0 THEN sh.qty ELSE 0 END) AS adjusted_qty_in,
        SUM(CASE WHEN sh.reference_type = 'IVAQ' AND sh.qty < 0 THEN ABS(sh.qty) ELSE 0 END) AS adjusted_qty_out,
        SUM(sh.qty) AS current_qty
    FROM stock_histories sh
    WHERE sh.business_id = @businessId
      AND sh.stock_date <= @toDate
      AND sh.is_reversal = 0
      AND sh.reversed_by_stock_history_id IS NULL
    GROUP BY sh.product_id, sh.product_type
),
AllProducts AS (
    SELECT id AS product_id, NAME AS product_name, unit_id AS product_unit_id, sku AS product_sku, 'S' AS product_type
    FROM products
    WHERE business_id = @businessId
    UNION
    SELECT id AS product_id, NAME AS product_name, unit_id AS product_unit_id, sku AS product_sku, 'V' AS product_type
    FROM product_variants
    WHERE business_id = @businessId
)
SELECT
    ap.product_id,
    ap.product_type,
    ap.product_name,
    ap.product_unit_id,
    ap.product_sku,
    COALESCE(l.opening_qty, 0) AS opening_qty,
    0 AS order_qty,
    COALESCE(l.received_qty, 0) AS received_qty,
    COALESCE(l.transfer_qty_in, 0) AS transfer_qty_in,
    COALESCE(l.transfer_qty_out, 0) AS transfer_qty_out,
    COALESCE(l.adjusted_qty_in, 0) AS adjusted_qty_in,
    COALESCE(l.adjusted_qty_out, 0) AS adjusted_qty_out,
    COALESCE(l.sale_qty, 0) AS sale_qty,
    0 AS committed_qty,
    COALESCE(l.current_qty, 0) AS current_qty,
    COALESCE(l.current_qty, 0) AS available_stock
FROM AllProducts ap
LEFT JOIN Ledger l ON ap.product_id = l.product_id AND ap.product_type = l.product_type
`

// SQL for SPECIFIC warehouse (with warehouse filter)
const inventorySummarySqlOneWarehouse = `
WITH Ledger AS (
    SELECT
        sh.product_id,
        sh.product_type,
        SUM(CASE WHEN sh.reference_type IN ('POS','PGOS','PCOS') THEN sh.qty ELSE 0 END) AS opening_qty,
        SUM(CASE WHEN sh.reference_type IN ('BL','CN') AND sh.qty > 0 THEN sh.qty ELSE 0 END) AS received_qty,
        SUM(CASE WHEN sh.reference_type = 'IV' THEN ABS(sh.qty) ELSE 0 END) AS sale_qty,
        SUM(CASE WHEN sh.reference_type = 'TO' AND sh.is_transfer_in = true THEN sh.qty ELSE 0 END) AS transfer_qty_in,
        SUM(CASE WHEN sh.reference_type = 'TO' AND sh.is_transfer_in = false THEN ABS(sh.qty) ELSE 0 END) AS transfer_qty_out,
        SUM(CASE WHEN sh.reference_type = 'IVAQ' AND sh.qty > 0 THEN sh.qty ELSE 0 END) AS adjusted_qty_in,
        SUM(CASE WHEN sh.reference_type = 'IVAQ' AND sh.qty < 0 THEN ABS(sh.qty) ELSE 0 END) AS adjusted_qty_out,
        SUM(sh.qty) AS current_qty
    FROM stock_histories sh
    WHERE sh.business_id = @businessId
      AND sh.stock_date <= @toDate
      AND sh.warehouse_id = @warehouseId
      AND sh.is_reversal = 0
      AND sh.reversed_by_stock_history_id IS NULL
    GROUP BY sh.product_id, sh.product_type
),
AllProducts AS (
    SELECT id AS product_id, NAME AS product_name, unit_id AS product_unit_id, sku AS product_sku, 'S' AS product_type
    FROM products
    WHERE business_id = @businessId
    UNION
    SELECT id AS product_id, NAME AS product_name, unit_id AS product_unit_id, sku AS product_sku, 'V' AS product_type
    FROM product_variants
    WHERE business_id = @businessId
)
SELECT
    ap.product_id,
    ap.product_type,
    ap.product_name,
    ap.product_unit_id,
    ap.product_sku,
    COALESCE(l.opening_qty, 0) AS opening_qty,
    0 AS order_qty,
    COALESCE(l.received_qty, 0) AS received_qty,
    COALESCE(l.transfer_qty_in, 0) AS transfer_qty_in,
    COALESCE(l.transfer_qty_out, 0) AS transfer_qty_out,
    COALESCE(l.adjusted_qty_in, 0) AS adjusted_qty_in,
    COALESCE(l.adjusted_qty_out, 0) AS adjusted_qty_out,
    COALESCE(l.sale_qty, 0) AS sale_qty,
    0 AS committed_qty,
    COALESCE(l.current_qty, 0) AS current_qty,
    COALESCE(l.current_qty, 0) AS available_stock
FROM AllProducts ap
LEFT JOIN Ledger l ON ap.product_id = l.product_id AND ap.product_type = l.product_type
`

// SQL for ALL warehouses (ledger-of-record only).
const valuationSqlAllWarehouses = `
WITH StockTotals AS (
    SELECT
        product_id,
        product_type,
        SUM(qty) AS stock_on_hand,
        SUM(qty * base_unit_value) AS asset_value
    FROM stock_histories
    WHERE business_id = @businessId
      AND stock_date <= @currentDate
      AND is_reversal = 0
      AND reversed_by_stock_history_id IS NULL
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
ORDER BY p.product_name;
`

// SQL for SPECIFIC warehouse (ledger-of-record only).
const valuationSqlOneWarehouse = `
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
      AND is_reversal = 0
      AND reversed_by_stock_history_id IS NULL
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
ORDER BY p.product_name;
`

func GetInventorySummaryLedger(ctx context.Context, toDate MyDateString, warehouseId *int) ([]*InventorySummaryResponse, error) {
	businessId, ok := utils.GetBusinessIdFromContext(ctx)
	if !ok || businessId == "" {
		return nil, errors.New("business id is required")
	}
	business, err := GetBusiness(ctx)
	if err != nil {
		return nil, errors.New("business id is required")
	}
	if err := toDate.EndOfDayUTCTime(business.Timezone); err != nil {
		return nil, err
	}

	db := config.GetDB()
	var summaries []*InventorySummaryResponse
	if warehouseId == nil || *warehouseId == 0 {
		if err := db.WithContext(ctx).Raw(inventorySummarySqlAllWarehouses, map[string]interface{}{
			"businessId": businessId,
			"toDate":     toDate,
		}).Scan(&summaries).Error; err != nil {
			return nil, err
		}
	} else {
		if err := utils.ValidateResourceId[Warehouse](ctx, businessId, *warehouseId); err != nil {
			return nil, errors.New("warehouse not found")
		}
		if err := db.WithContext(ctx).Raw(inventorySummarySqlOneWarehouse, map[string]interface{}{
			"businessId":  businessId,
			"toDate":      toDate,
			"warehouseId": *warehouseId,
		}).Scan(&summaries).Error; err != nil {
			return nil, err
		}
	}

	return summaries, nil
}

func GetWarehouseInventoryLedger(ctx context.Context, toDate MyDateString) ([]*WarehouseInventoryResponse, error) {
	businessId, ok := utils.GetBusinessIdFromContext(ctx)
	if !ok || businessId == "" {
		return nil, errors.New("business id is required")
	}
	business, err := GetBusiness(ctx)
	if err != nil {
		return nil, errors.New("business id is required")
	}
	if err := toDate.EndOfDayUTCTime(business.Timezone); err != nil {
		return nil, err
	}

	sql := `
WITH Ledger AS (
    SELECT
        sh.warehouse_id,
        sh.product_id,
        sh.product_type,
        SUM(CASE WHEN sh.reference_type IN ('POS','PGOS','PCOS') THEN sh.qty ELSE 0 END) AS opening_qty,
        SUM(CASE WHEN sh.reference_type IN ('BL','CN') AND sh.qty > 0 THEN sh.qty ELSE 0 END) AS received_qty,
        SUM(CASE WHEN sh.reference_type = 'IV' THEN ABS(sh.qty) ELSE 0 END) AS sale_qty,
        SUM(CASE WHEN sh.reference_type = 'TO' AND sh.is_transfer_in = true THEN sh.qty ELSE 0 END) AS transfer_qty_in,
        SUM(CASE WHEN sh.reference_type = 'TO' AND sh.is_transfer_in = false THEN ABS(sh.qty) ELSE 0 END) AS transfer_qty_out,
        SUM(CASE WHEN sh.reference_type = 'IVAQ' AND sh.qty > 0 THEN sh.qty ELSE 0 END) AS adjusted_qty_in,
        SUM(CASE WHEN sh.reference_type = 'IVAQ' AND sh.qty < 0 THEN ABS(sh.qty) ELSE 0 END) AS adjusted_qty_out,
        SUM(sh.qty) AS current_qty
    FROM stock_histories sh
    WHERE sh.business_id = @businessId
      AND sh.stock_date <= @toDate
      AND sh.is_reversal = 0
      AND sh.reversed_by_stock_history_id IS NULL
    GROUP BY sh.warehouse_id, sh.product_id, sh.product_type
),
AllProducts AS (
    SELECT id AS product_id, NAME AS product_name, unit_id AS product_unit_id, sku AS product_sku, 'S' AS product_type
    FROM products
    WHERE business_id = @businessId
    UNION
    SELECT id AS product_id, NAME AS product_name, unit_id AS product_unit_id, sku AS product_sku, 'V' AS product_type
    FROM product_variants
    WHERE business_id = @businessId
)
SELECT
    Ledger.warehouse_id,
    Ledger.product_id,
    Ledger.product_type,
    COALESCE(Ledger.opening_qty, 0) AS opening_qty,
    0 AS order_qty,
    COALESCE(Ledger.received_qty, 0) AS received_qty,
    COALESCE(Ledger.transfer_qty_in, 0) AS transfer_qty_in,
    COALESCE(Ledger.transfer_qty_out, 0) AS transfer_qty_out,
    COALESCE(Ledger.adjusted_qty_in, 0) AS adjusted_qty_in,
    COALESCE(Ledger.adjusted_qty_out, 0) AS adjusted_qty_out,
    COALESCE(Ledger.sale_qty, 0) AS sale_qty,
    0 AS committed_qty,
    COALESCE(Ledger.current_qty, 0) AS current_qty,
    COALESCE(Ledger.current_qty, 0) AS available_stock,
    w.name AS warehouse_name,
    ap.product_name,
    ap.product_unit_id,
    ap.product_sku
FROM Ledger
LEFT JOIN warehouses w ON Ledger.warehouse_id = w.ID
LEFT JOIN AllProducts ap ON Ledger.product_id = ap.product_id AND Ledger.product_type = ap.product_type
	`

	db := config.GetDB()
	var summaries []*WarehouseInventoryResponse
	if err := db.WithContext(ctx).Raw(sql, map[string]interface{}{
		"businessId": businessId,
		"toDate":     toDate,
	}).Scan(&summaries).Error; err != nil {
		return nil, err
	}
	return summaries, nil
}

// GetWarehouseInventoryByProduct returns per-warehouse inventory for a single product as-of the supplied date (default: today, business TZ).
func GetWarehouseInventoryByProduct(ctx context.Context, productId int, toDate *MyDateString) ([]*WarehouseInventoryResponse, error) {
	if productId <= 0 {
		return nil, errors.New("product id is required")
	}
	businessId, ok := utils.GetBusinessIdFromContext(ctx)
	if !ok || businessId == "" {
		return nil, errors.New("business id is required")
	}
	business, err := GetBusiness(ctx)
	if err != nil {
		return nil, errors.New("business id is required")
	}

	asOf := MyDateString(time.Now().In(time.UTC))
	if toDate != nil {
		asOf = *toDate
	}
	if err := asOf.EndOfDayUTCTime(business.Timezone); err != nil {
		return nil, err
	}

	db := config.GetDB()
	sql := `
WITH Ledger AS (
    SELECT
        sh.warehouse_id,
        sh.product_id,
        sh.product_type,
        SUM(CASE WHEN sh.reference_type IN ('POS','PGOS','PCOS') THEN sh.qty ELSE 0 END) AS opening_qty,
        SUM(CASE WHEN sh.reference_type IN ('BL','CN') AND sh.qty > 0 THEN sh.qty ELSE 0 END) AS received_qty,
        SUM(CASE WHEN sh.reference_type = 'IV' THEN ABS(sh.qty) ELSE 0 END) AS sale_qty,
        SUM(CASE WHEN sh.reference_type = 'TO' AND sh.is_transfer_in = true THEN sh.qty ELSE 0 END) AS transfer_qty_in,
        SUM(CASE WHEN sh.reference_type = 'TO' AND sh.is_transfer_in = false THEN ABS(sh.qty) ELSE 0 END) AS transfer_qty_out,
        SUM(CASE WHEN sh.reference_type = 'IVAQ' AND sh.qty > 0 THEN sh.qty ELSE 0 END) AS adjusted_qty_in,
        SUM(CASE WHEN sh.reference_type = 'IVAQ' AND sh.qty < 0 THEN ABS(sh.qty) ELSE 0 END) AS adjusted_qty_out,
        SUM(sh.qty) AS current_qty
    FROM stock_histories sh
    WHERE sh.business_id = @businessId
      AND sh.stock_date <= @toDate
      AND sh.is_reversal = 0
      AND sh.reversed_by_stock_history_id IS NULL
      AND sh.product_id = @productId
    GROUP BY sh.warehouse_id, sh.product_id, sh.product_type
),
AllProducts AS (
    SELECT id AS product_id, NAME AS product_name, unit_id AS product_unit_id, sku AS product_sku, 'S' AS product_type
    FROM products
    WHERE business_id = @businessId AND id = @productId
    UNION
    SELECT id AS product_id, NAME AS product_name, unit_id AS product_unit_id, sku AS product_sku, 'V' AS product_type
    FROM product_variants
    WHERE business_id = @businessId AND id = @productId
)
SELECT
    Ledger.warehouse_id,
    Ledger.product_id,
    Ledger.product_type,
    COALESCE(Ledger.opening_qty, 0) AS opening_qty,
    0 AS order_qty,
    COALESCE(Ledger.received_qty, 0) AS received_qty,
    COALESCE(Ledger.transfer_qty_in, 0) AS transfer_qty_in,
    COALESCE(Ledger.transfer_qty_out, 0) AS transfer_qty_out,
    COALESCE(Ledger.adjusted_qty_in, 0) AS adjusted_qty_in,
    COALESCE(Ledger.adjusted_qty_out, 0) AS adjusted_qty_out,
    COALESCE(Ledger.sale_qty, 0) AS sale_qty,
    0 AS committed_qty,
    COALESCE(Ledger.current_qty, 0) AS current_qty,
    COALESCE(Ledger.current_qty, 0) AS available_stock,
    w.name AS warehouse_name,
    ap.product_name,
    ap.product_unit_id,
    ap.product_sku
FROM Ledger
LEFT JOIN warehouses w ON Ledger.warehouse_id = w.ID
LEFT JOIN AllProducts ap ON Ledger.product_id = ap.product_id AND Ledger.product_type = ap.product_type
`

	var rows []*WarehouseInventoryResponse
	if err := db.WithContext(ctx).Raw(sql, map[string]interface{}{
		"businessId": businessId,
		"toDate":     asOf,
		"productId":  productId,
	}).Scan(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

func GetInventoryValuationSummaryLedger(ctx context.Context, currentDate MyDateString, warehouseId int) ([]*InventoryValuationSummaryResponse, error) {
	businessId, ok := utils.GetBusinessIdFromContext(ctx)
	if !ok || businessId == "" {
		return nil, errors.New("business id is required")
	}
	business, err := GetBusiness(ctx)
	if err != nil {
		return nil, errors.New("business id is required")
	}
	if err := currentDate.EndOfDayUTCTime(business.Timezone); err != nil {
		return nil, err
	}

	db := config.GetDB()
	var results []*InventoryValuationSummaryResponse
	if warehouseId <= 0 {
		if err := db.WithContext(ctx).Raw(valuationSqlAllWarehouses, map[string]interface{}{
			"businessId":  businessId,
			"currentDate": currentDate,
		}).Scan(&results).Error; err != nil {
			return nil, err
		}
	} else {
		if err := utils.ValidateResourceId[Warehouse](ctx, businessId, warehouseId); err != nil {
			return nil, errors.New("warehouse not found")
		}
		if err := db.WithContext(ctx).Raw(valuationSqlOneWarehouse, map[string]interface{}{
			"businessId":  businessId,
			"currentDate": currentDate,
			"warehouseId": warehouseId,
		}).Scan(&results).Error; err != nil {
			return nil, err
		}
	}
	return results, nil
}
