package models

import (
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

// UpdateStockClosingBalances recalculates closing qty/asset and cumulative quantities
// for the given stock histories and all following rows in the same key scope.
func UpdateStockClosingBalances(tx *gorm.DB, newStockHistories []*StockHistory, lastStockHistories []*StockHistory) error {
	for _, stockHistory := range newStockHistories {
		if stockHistory == nil {
			continue
		}

		closingQty := decimal.NewFromInt(0)
		closingAssetValue := decimal.NewFromInt(0)
		cumulativeIncomingQty := decimal.NewFromInt(0)
		cumulativeOutgoingQty := decimal.NewFromInt(0)
		cumulativeSequence := 0

		for _, lastStockHistory := range lastStockHistories {
			if lastStockHistory == nil {
				continue
			}
			if lastStockHistory.WarehouseId == stockHistory.WarehouseId &&
				lastStockHistory.ProductId == stockHistory.ProductId &&
				lastStockHistory.ProductType == stockHistory.ProductType &&
				lastStockHistory.BatchNumber == stockHistory.BatchNumber {

				closingQty = lastStockHistory.ClosingQty
				closingAssetValue = lastStockHistory.ClosingAssetValue
				cumulativeIncomingQty = lastStockHistory.CumulativeIncomingQty
				cumulativeOutgoingQty = lastStockHistory.CumulativeOutgoingQty
				cumulativeSequence = lastStockHistory.CumulativeSequence
				break
			}
		}

		err := tx.Exec(`
			UPDATE stock_histories AS t
			JOIN (SELECT
				id,
				warehouse_id,
				product_id,
				product_type,
				batch_number,
				? + SUM(qty) OVER (PARTITION BY business_id, warehouse_id, product_id, product_type, batch_number ORDER BY stock_date, is_outgoing, id ROWS BETWEEN UNBOUNDED PRECEDING AND CURRENT ROW) AS closing_qty_balance,
				? + SUM(qty * base_unit_value) OVER (PARTITION BY business_id, warehouse_id, product_id, product_type, batch_number ORDER BY stock_date, is_outgoing, id ROWS BETWEEN UNBOUNDED PRECEDING AND CURRENT ROW) AS closing_asset_value_balance,
				? + SUM(CASE WHEN is_outgoing THEN 0 ELSE qty END) OVER (PARTITION BY business_id, warehouse_id, product_id, product_type, batch_number ORDER BY stock_date, is_outgoing, id ROWS BETWEEN UNBOUNDED PRECEDING AND CURRENT ROW) AS cumulative_incoming_qty_balance,
				? + SUM(CASE WHEN is_outgoing THEN ABS(qty) ELSE 0 END)  OVER (PARTITION BY business_id, warehouse_id, product_id, product_type, batch_number ORDER BY stock_date, is_outgoing, id ROWS BETWEEN UNBOUNDED PRECEDING AND CURRENT ROW) AS cumulative_outgoing_qty_balance,
				? + ROW_NUMBER() OVER (PARTITION BY business_id, warehouse_id, product_id, product_type, batch_number ORDER BY stock_date, is_outgoing, id) AS cumulative_sequence
			FROM stock_histories
			WHERE business_id = ? AND warehouse_id = ? AND product_id = ? AND product_type = ? AND batch_number = ? AND stock_date >= ?
			) AS temp
			ON t.id = temp.id
			SET
			t.closing_qty = temp.closing_qty_balance,
			t.closing_asset_value = temp.closing_asset_value_balance,
			t.cumulative_incoming_qty = temp.cumulative_incoming_qty_balance,
			t.cumulative_outgoing_qty = temp.cumulative_outgoing_qty_balance,
			t.cumulative_sequence = temp.cumulative_sequence
			WHERE t.id > 0
			`, closingQty, closingAssetValue, cumulativeIncomingQty, cumulativeOutgoingQty, cumulativeSequence, stockHistory.BusinessId, stockHistory.WarehouseId, stockHistory.ProductId, stockHistory.ProductType, stockHistory.BatchNumber, stockHistory.StockDate).Error
		if err != nil {
			return err
		}
	}
	return nil
}
