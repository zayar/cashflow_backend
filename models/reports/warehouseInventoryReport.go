package reports

import (
	"context"
	"errors"

	"github.com/mmdatafocus/books_backend/config"
	"github.com/mmdatafocus/books_backend/models"
	"github.com/mmdatafocus/books_backend/utils"
	"github.com/shopspring/decimal"
)

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

func GetWarehouseInventoryReport(ctx context.Context, toDate models.MyDateString) ([]*WarehouseInventoryResponse, error) {

	sql := `
WITH InventorySummary AS (
    SELECT
        COUNT(*) AS count,
        warehouse_id,
        product_id,
        product_type,
        batch_number,
        SUM(opening_qty) AS opening_qty,
        SUM(order_qty) AS order_qty,
        SUM(received_qty) AS received_qty,
        SUM(sale_qty) AS sale_qty,
        SUM(transfer_qty_in) AS transfer_qty_in,
        -- Normalize OUT columns as positive numbers (ledger may store as negative).
        SUM(ABS(transfer_qty_out)) AS transfer_qty_out,
        -- Normalize adjusted columns:
        -- - Adjusted Qty In should never be negative
        -- - If adjusted_qty_in is negative (e.g. delete/reversal), display it under "Adjusted Qty Out"
        SUM(CASE WHEN adjusted_qty_in > 0 THEN adjusted_qty_in ELSE 0 END) AS adjusted_qty_in,
        SUM(ABS(adjusted_qty_out)) + SUM(CASE WHEN adjusted_qty_in < 0 THEN ABS(adjusted_qty_in) ELSE 0 END) AS adjusted_qty_out,
        SUM(committed_qty) AS committed_qty,
        SUM(received_qty + adjusted_qty_in + transfer_qty_in - sale_qty - abs(adjusted_qty_out) - abs(transfer_qty_out)) AS current_qty
    FROM
        stock_summary_daily_balances
    WHERE
        transaction_date <= @toDate
        AND business_id = @businessId
    GROUP BY
        warehouse_id,
        product_id,
        product_type,
        batch_number
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
    WHERE
        business_id = @businessId
    UNION
    SELECT
        id AS product_id,
        NAME AS product_name,
        unit_id AS product_unit_id,
        sku AS product_sku,
        'V' AS product_type
    FROM
        product_variants
    WHERE
        business_id = @businessId
)
SELECT
    InventorySummary.*,
    InventorySummary.current_qty - InventorySummary.committed_qty AS available_stock,
    w.name as warehouse_name,
    AllProducts.product_name,
    -- AllProducts.product_id,
    -- AllProducts.product_type,
    AllProducts.product_unit_id,
    AllProducts.product_sku
FROM
    InventorySummary
    LEFT JOIN warehouses w ON InventorySummary.warehouse_id = w.ID
    LEFT JOIN AllProducts ON InventorySummary.product_type = AllProducts.product_type
    AND InventorySummary.product_id = AllProducts.product_id
	`

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
	var summaries []*WarehouseInventoryResponse
	if err := db.WithContext(ctx).Raw(sql, map[string]interface{}{
		"businessId": businessId,
		"toDate":     toDate,
	}).Scan(&summaries).Error; err != nil {
		return nil, err
	}
	return summaries, nil
}
