package reports

import (
	"context"
	"errors"

	"github.com/mmdatafocus/books_backend/config"
	"github.com/mmdatafocus/books_backend/models"
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

func GetInventorySummaryReport(ctx context.Context, toDate models.MyDateString, warehouseId *int) ([]*InventorySummaryResponse, error) {
	businessId, ok := utils.GetBusinessIdFromContext(ctx)
	if !ok || businessId == "" {
		return nil, errors.New("business id is required")
	}
	business, err := models.GetBusiness(ctx)
	if err != nil {
		return nil, errors.New("business id is required")
	}
	if err := toDate.EndOfDayUTCTime(business.Timezone); err != nil {
		return nil, err
	}

	db := config.GetDB()
	var summaries []*InventorySummaryResponse

	// Use explicit separate queries - no templates, no conditional args
	// This eliminates any GORM named-param expansion issues
	if warehouseId == nil || *warehouseId == 0 {
		// All Warehouses: only 2 named params
		if err := db.WithContext(ctx).Raw(inventorySummarySqlAllWarehouses, map[string]interface{}{
			"businessId": businessId,
			"toDate":     toDate,
		}).Scan(&summaries).Error; err != nil {
			return nil, err
		}
	} else {
		// Specific Warehouse: 3 named params
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
