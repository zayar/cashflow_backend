package models

import (
	"fmt"

	"github.com/mmdatafocus/books_backend/utils"
	"gorm.io/gorm"
)

// ApplySalesOrderStockForStatusTransition applies committed stock changes for a SalesOrder status transition.
//
// Draft -> Confirmed : increase committed_qty
// Confirmed -> Draft/Cancelled/Closed : reverse committed_qty
func ApplySalesOrderStockForStatusTransition(tx *gorm.DB, so *SalesOrder, oldStatus SalesOrderStatus) error {
	if tx == nil {
		return fmt.Errorf("tx is nil")
	}
	if so == nil {
		return fmt.Errorf("sales order is nil")
	}
	if string(oldStatus) == string(so.CurrentStatus) {
		return nil
	}

	applyCommit := oldStatus == SalesOrderStatusDraft && so.CurrentStatus == SalesOrderStatusConfirmed
	reverseCommit := oldStatus == SalesOrderStatusConfirmed &&
		(so.CurrentStatus == SalesOrderStatusDraft || so.CurrentStatus == SalesOrderStatusCancelled || so.CurrentStatus == SalesOrderStatusClosed)
	if !applyCommit && !reverseCommit {
		return nil
	}

	ctx := tx.Statement.Context
	if err := utils.BusinessLock(ctx, so.BusinessId, "stockLock", "stockCommands_salesOrder.go", "ApplySalesOrderStockForStatusTransition"); err != nil {
		tx.Rollback()
		return err
	}

	for _, item := range so.Details {
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

		qty := item.DetailQty
		if reverseCommit {
			qty = qty.Neg()
		}
		if err := UpdateStockSummaryCommittedQty(
			tx,
			so.BusinessId,
			so.WarehouseId,
			item.ProductId,
			string(item.ProductType),
			item.BatchNumber,
			qty,
			so.OrderDate,
		); err != nil {
			tx.Rollback()
			return err
		}
	}

	return nil
}
