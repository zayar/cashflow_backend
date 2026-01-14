package models

import (
	"fmt"

	"github.com/mmdatafocus/books_backend/utils"
	"gorm.io/gorm"
)

// ApplyBillStockForStatusTransition applies stock changes for a Bill status transition.
//
// Draft -> Confirmed   : increase received_qty/current_qty
// Confirmed -> Draft/Void : reverse received_qty/current_qty
//
// This is the explicit, command-style replacement for implicit GORM model-hook side-effects.
func ApplyBillStockForStatusTransition(tx *gorm.DB, bill *Bill, oldStatus BillStatus) error {
	if tx == nil {
		return fmt.Errorf("tx is nil")
	}
	if bill == nil {
		return fmt.Errorf("bill is nil")
	}
	if string(oldStatus) == string(bill.CurrentStatus) {
		return nil
	}

	applyReceived := oldStatus == BillStatusDraft && bill.CurrentStatus == BillStatusConfirmed
	reverseReceived := oldStatus == BillStatusConfirmed && (bill.CurrentStatus == BillStatusDraft || bill.CurrentStatus == BillStatusVoid)
	if !applyReceived && !reverseReceived {
		return nil
	}

	ctx := tx.Statement.Context
	if err := utils.BusinessLock(ctx, bill.BusinessId, "stockLock", "stockCommands_bill.go", "ApplyBillStockForStatusTransition"); err != nil {
		tx.Rollback()
		return err
	}

	for _, billItem := range bill.Details {
		if billItem.ProductId <= 0 {
			continue
		}
		product, err := GetProductOrVariant(ctx, string(billItem.ProductType), billItem.ProductId)
		if err != nil {
			tx.Rollback()
			return err
		}
		if product.GetInventoryAccountID() <= 0 {
			continue
		}

		qty := billItem.DetailQty
		if reverseReceived {
			qty = qty.Neg()
		}

		if err := UpdateStockSummaryReceivedQty(
			tx,
			bill.BusinessId,
			bill.WarehouseId,
			billItem.ProductId,
			string(billItem.ProductType),
			billItem.BatchNumber,
			qty,
			bill.BillDate,
		); err != nil {
			tx.Rollback()
			return err
		}
	}

	return nil
}
