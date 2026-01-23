package models

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/mmdatafocus/books_backend/config"
	"github.com/mmdatafocus/books_backend/utils"
	"github.com/shopspring/decimal"
)

var (
	// ErrBusinessIdRequired is returned when the context lacks a business id.
	ErrBusinessIdRequired = fmt.Errorf("business id is required")
	// ErrDBNotInitialized is returned when the DB connection has not been established.
	ErrDBNotInitialized = fmt.Errorf("database not initialized")
)

// InventorySnapshot represents an aggregated view of stock and valuation for a product (optionally per-warehouse) as-of a timestamp.
type InventorySnapshot struct {
	ProductId    int             `json:"product_id"`
	ProductType  ProductType     `json:"product_type"`
	WarehouseId  *int            `json:"warehouse_id,omitempty"`
	StockOnHand  decimal.Decimal `json:"stock_on_hand"`
	AssetValue   decimal.Decimal `json:"asset_value"`
	UnitCostSafe decimal.Decimal `json:"unit_cost_safe"`
}

// computeLedgerSnapshots aggregates active stock_histories as-of the supplied timestamp.
// All rows are filtered by business_id and ignore reversals.
// When warehouseId is nil, results are aggregated across all warehouses (grouped only by product).
// When product filters are nil, all inventory items for the business are returned.
func computeLedgerSnapshots(ctx context.Context, asOf time.Time, warehouseId *int, productId *int, productType *ProductType, batchNumber *string) ([]InventorySnapshot, error) {
	businessId, ok := utils.GetBusinessIdFromContext(ctx)
	if !ok || businessId == "" {
		return nil, ErrBusinessIdRequired
	}
	db := config.GetDB()
	if db == nil {
		return nil, ErrDBNotInitialized
	}

	args := map[string]interface{}{
		"businessId": businessId,
		"asOf":       asOf,
	}

	where := `
		business_id = @businessId
		AND stock_date <= @asOf
		AND is_reversal = 0
		AND reversed_by_stock_history_id IS NULL
	`
	if warehouseId != nil && *warehouseId > 0 {
		where += " AND warehouse_id = @warehouseId"
		args["warehouseId"] = *warehouseId
	}
	if productId != nil && *productId > 0 {
		where += " AND product_id = @productId"
		args["productId"] = *productId
	}
	if productType != nil {
		where += " AND product_type = @productType"
		args["productType"] = *productType
	}
	// IMPORTANT: empty batch means "fungible across batches".
	// If the caller passes batchNumber="" (common for invoices), we must NOT filter by batch,
	// otherwise stock that exists under real batches (or NULL) will appear as 0 and block posting.
	if batchNumber != nil && strings.TrimSpace(*batchNumber) != "" {
		where += " AND COALESCE(batch_number, '') = @batchNumber"
		args["batchNumber"] = *batchNumber
	}

	group := "product_id, product_type"
	selectCols := `
		product_id,
		product_type,
		SUM(qty) AS stock_on_hand,
		SUM(qty * base_unit_value) AS asset_value
	`
	if warehouseId != nil && *warehouseId > 0 {
		selectCols = `
			product_id,
			product_type,
			warehouse_id,
			SUM(qty) AS stock_on_hand,
			SUM(qty * base_unit_value) AS asset_value
		`
		group = "product_id, product_type, warehouse_id"
	}

	sql := `
	SELECT
		` + selectCols + `
	FROM stock_histories
	WHERE ` + where + `
	GROUP BY ` + group + `
	`

	var rows []InventorySnapshot
	if err := db.WithContext(ctx).Raw(sql, args).Scan(&rows).Error; err != nil {
		return nil, err
	}

	for i := range rows {
		if rows[i].StockOnHand.IsZero() {
			rows[i].UnitCostSafe = decimal.Zero
			continue
		}
		rows[i].UnitCostSafe = rows[i].AssetValue.Div(rows[i].StockOnHand)
	}

	return rows, nil
}

// InventorySnapshotByProduct returns aggregated snapshots across all warehouses.
func InventorySnapshotByProduct(ctx context.Context, asOf MyDateString, productId *int, productType *ProductType) ([]InventorySnapshot, error) {
	business, err := GetBusiness(ctx)
	if err != nil {
		return nil, err
	}
	if err := asOf.EndOfDayUTCTime(business.Timezone); err != nil {
		return nil, err
	}
	return computeLedgerSnapshots(ctx, time.Time(asOf), nil, productId, productType, nil)
}

// InventorySnapshotByProductWarehouse returns snapshots per warehouse.
func InventorySnapshotByProductWarehouse(ctx context.Context, asOf MyDateString, warehouseId *int, productId *int, productType *ProductType, batchNumber *string) ([]InventorySnapshot, error) {
	business, err := GetBusiness(ctx)
	if err != nil {
		return nil, err
	}
	if err := asOf.EndOfDayUTCTime(business.Timezone); err != nil {
		return nil, err
	}
	return computeLedgerSnapshots(ctx, time.Time(asOf), warehouseId, productId, productType, batchNumber)
}

// SumSnapshot aggregates snapshot rows by product (ignoring warehouse dimension).
func SumSnapshot(rows []InventorySnapshot) map[string]InventorySnapshot {
	result := make(map[string]InventorySnapshot)
	for _, r := range rows {
		key := snapshotKey(r.ProductId, r.ProductType)
		acc := result[key]
		acc.ProductId = r.ProductId
		acc.ProductType = r.ProductType
		acc.StockOnHand = acc.StockOnHand.Add(r.StockOnHand)
		acc.AssetValue = acc.AssetValue.Add(r.AssetValue)
		if !acc.StockOnHand.IsZero() {
			acc.UnitCostSafe = acc.AssetValue.Div(acc.StockOnHand)
		}
		result[key] = acc
	}
	return result
}

func snapshotKey(pid int, ptype ProductType) string {
	return fmt.Sprintf("%d-%s", pid, string(ptype))
}
