package reports

import (
	"context"
	"errors"

	"bitbucket.org/mmdatafocus/books_backend/config"
	"bitbucket.org/mmdatafocus/books_backend/models"
	"bitbucket.org/mmdatafocus/books_backend/utils"
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
WITH InventorySummary AS (
    SELECT
        COUNT(*) AS count,
        product_id,
        product_type,
        batch_number,
        SUM(opening_qty) AS opening_qty,
        SUM(order_qty) AS order_qty,
        SUM(received_qty) AS received_qty,
        SUM(sale_qty) AS sale_qty,
        SUM(transfer_qty_in) AS transfer_qty_in,
        SUM(ABS(transfer_qty_out)) AS transfer_qty_out,
        SUM(adjusted_qty_in) AS adjusted_qty_in,
        SUM(ABS(adjusted_qty_out)) AS adjusted_qty_out,
        SUM(committed_qty) AS committed_qty,
        SUM(received_qty + adjusted_qty_in + transfer_qty_in - sale_qty - abs(adjusted_qty_out) - abs(transfer_qty_out)) AS current_qty
    FROM stock_summary_daily_balances
    WHERE transaction_date <= @toDate
        AND business_id = @businessId
    GROUP BY product_id, product_type, batch_number
),
AllProducts AS (
    SELECT
        id AS product_id,
        NAME AS product_name,
        unit_id AS product_unit_id,
        sku AS product_sku,
        'S' AS product_type
    FROM products
    WHERE business_id = @businessId
    UNION
    SELECT
        id AS product_id,
        NAME AS product_name,
        unit_id AS product_unit_id,
        sku AS product_sku,
        'V' AS product_type
    FROM product_variants
    WHERE business_id = @businessId
)
SELECT
    InventorySummary.*,
    AllProducts.product_name,
    AllProducts.product_unit_id,
    AllProducts.product_sku,
    current_qty - committed_qty AS available_stock
FROM InventorySummary
LEFT JOIN AllProducts ON InventorySummary.product_type = AllProducts.product_type
    AND InventorySummary.product_id = AllProducts.product_id
`

// SQL for SPECIFIC warehouse (with warehouse filter)
const inventorySummarySqlOneWarehouse = `
WITH InventorySummary AS (
    SELECT
        COUNT(*) AS count,
        product_id,
        product_type,
        batch_number,
        SUM(opening_qty) AS opening_qty,
        SUM(order_qty) AS order_qty,
        SUM(received_qty) AS received_qty,
        SUM(sale_qty) AS sale_qty,
        SUM(transfer_qty_in) AS transfer_qty_in,
        SUM(ABS(transfer_qty_out)) AS transfer_qty_out,
        SUM(adjusted_qty_in) AS adjusted_qty_in,
        SUM(ABS(adjusted_qty_out)) AS adjusted_qty_out,
        SUM(committed_qty) AS committed_qty,
        SUM(received_qty + adjusted_qty_in + transfer_qty_in - sale_qty - abs(adjusted_qty_out) - abs(transfer_qty_out)) AS current_qty
    FROM stock_summary_daily_balances
    WHERE transaction_date <= @toDate
        AND business_id = @businessId
        AND warehouse_id = @warehouseId
    GROUP BY product_id, product_type, batch_number
),
AllProducts AS (
    SELECT
        id AS product_id,
        NAME AS product_name,
        unit_id AS product_unit_id,
        sku AS product_sku,
        'S' AS product_type
    FROM products
    WHERE business_id = @businessId
    UNION
    SELECT
        id AS product_id,
        NAME AS product_name,
        unit_id AS product_unit_id,
        sku AS product_sku,
        'V' AS product_type
    FROM product_variants
    WHERE business_id = @businessId
)
SELECT
    InventorySummary.*,
    AllProducts.product_name,
    AllProducts.product_unit_id,
    AllProducts.product_sku,
    current_qty - committed_qty AS available_stock
FROM InventorySummary
LEFT JOIN AllProducts ON InventorySummary.product_type = AllProducts.product_type
    AND InventorySummary.product_id = AllProducts.product_id
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
