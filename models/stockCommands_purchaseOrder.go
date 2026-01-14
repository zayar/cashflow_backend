package models

import (
	"fmt"

	"github.com/mmdatafocus/books_backend/utils"
	"gorm.io/gorm"
)

// ApplyPurchaseOrderStockForStatusTransition applies stock changes for a PurchaseOrder status transition.
//
// Draft -> Confirmed : increase order_qty
// Confirmed -> Draft/Closed/Cancelled : reverse order_qty
//
// This is the explicit, command-style replacement for implicit GORM model-hook side-effects.
func ApplyPurchaseOrderStockForStatusTransition(tx *gorm.DB, po *PurchaseOrder, oldStatus PurchaseOrderStatus) error {
	if tx == nil {
		return fmt.Errorf("tx is nil")
	}
	if po == nil {
		return fmt.Errorf("purchase order is nil")
	}
	if string(oldStatus) == string(po.CurrentStatus) {
		return nil
	}

	applyOrder := oldStatus == PurchaseOrderStatusDraft && po.CurrentStatus == PurchaseOrderStatusConfirmed
	reverseOrder := oldStatus == PurchaseOrderStatusConfirmed &&
		(po.CurrentStatus == PurchaseOrderStatusDraft || po.CurrentStatus == PurchaseOrderStatusClosed || po.CurrentStatus == PurchaseOrderStatusCancelled)
	if !applyOrder && !reverseOrder {
		return nil
	}

	ctx := tx.Statement.Context
	if err := utils.BusinessLock(ctx, po.BusinessId, "stockLock", "stockCommands_purchaseOrder.go", "ApplyPurchaseOrderStockForStatusTransition"); err != nil {
		tx.Rollback()
		return err
	}

	for _, poItem := range po.Details {
		if poItem.ProductId <= 0 {
			continue
		}
		product, err := GetProductOrVariant(ctx, string(poItem.ProductType), poItem.ProductId)
		if err != nil {
			tx.Rollback()
			return err
		}
		if product.GetInventoryAccountID() <= 0 {
			continue
		}

		qty := poItem.DetailQty
		if reverseOrder {
			qty = qty.Neg()
		}
		if err := UpdateStockSummaryOrderQty(
			tx,
			po.BusinessId,
			po.WarehouseId,
			poItem.ProductId,
			string(poItem.ProductType),
			poItem.BatchNumber,
			qty,
			po.OrderDate,
		); err != nil {
			tx.Rollback()
			return err
		}
	}

	return nil
}
