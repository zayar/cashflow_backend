package models

import (
	"fmt"

	"github.com/mmdatafocus/books_backend/utils"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

// ApplyInventoryAdjustmentStockForStatusTransition applies stock changes for an InventoryAdjustment status transition.
//
// Only applies to Quantity adjustments.
// Draft -> Adjusted : apply adjustments to stock summary.
//
// NOTE: This mirrors existing hook behavior. If you need reversals (Adjusted -> Draft),
// implement a reversal policy rather than mutating historical adjustments in place.
func ApplyInventoryAdjustmentStockForStatusTransition(tx *gorm.DB, ia *InventoryAdjustment, oldStatus InventoryAdjustmentStatus) error {
	if tx == nil {
		return fmt.Errorf("tx is nil")
	}
	if ia == nil {
		return fmt.Errorf("inventory adjustment is nil")
	}
	if ia.AdjustmentType != InventoryAdjustmentTypeQuantity {
		return nil
	}
	if string(oldStatus) == string(ia.CurrentStatus) {
		return nil
	}

	apply := oldStatus == InventoryAdjustmentStatusDraft && ia.CurrentStatus == InventoryAdjustmentStatusAdjusted
	if !apply {
		return nil
	}

	ctx := tx.Statement.Context
	if err := utils.BusinessLock(ctx, ia.BusinessId, "stockLock", "stockCommands_inventoryAdjustment.go", "ApplyInventoryAdjustmentStockForStatusTransition"); err != nil {
		tx.Rollback()
		return err
	}

	for _, item := range ia.Details {
		if item.ProductId <= 0 {
			continue
		}
		product, err := GetProductOrVariant(ctx, string(item.ProductType), item.ProductId)
		if err != nil {
			tx.Rollback()
			return err
		}
		if product.GetInventoryAccountID() <= 0 {
			continue
		}

		if item.AdjustedValue.GreaterThan(decimal.NewFromFloat(0)) {
			if err := UpdateStockSummaryAdjustedQtyIn(tx, ia.BusinessId, ia.WarehouseId, item.ProductId, string(item.ProductType), item.BatchNumber, item.AdjustedValue, ia.AdjustmentDate); err != nil {
				tx.Rollback()
				return err
			}
		} else {
			if err := UpdateStockSummaryAdjustedQtyOut(tx, ia.BusinessId, ia.WarehouseId, item.ProductId, string(item.ProductType), item.BatchNumber, item.AdjustedValue, ia.AdjustmentDate); err != nil {
				tx.Rollback()
				return err
			}
		}
	}

	return nil
}
