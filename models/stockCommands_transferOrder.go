package models

import (
	"fmt"

	"github.com/mmdatafocus/books_backend/utils"
	"gorm.io/gorm"
)

// ApplyTransferOrderStockForStatusTransition applies stock changes for a TransferOrder status transition.
//
// Draft -> Confirmed : move stock from source to destination
func ApplyTransferOrderStockForStatusTransition(tx *gorm.DB, to *TransferOrder, oldStatus TransferOrderStatus) error {
	if tx == nil {
		return fmt.Errorf("tx is nil")
	}
	if to == nil {
		return fmt.Errorf("transfer order is nil")
	}
	if string(oldStatus) == string(to.CurrentStatus) {
		return nil
	}

	apply := oldStatus == TransferOrderStatusDraft && to.CurrentStatus == TransferOrderStatusConfirmed
	if !apply {
		return nil
	}

	ctx := tx.Statement.Context
	if err := utils.BusinessLock(ctx, to.BusinessId, "stockLock", "stockCommands_transferOrder.go", "ApplyTransferOrderStockForStatusTransition"); err != nil {
		tx.Rollback()
		return err
	}

	for _, item := range to.Details {
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

		if err := UpdateStockSummaryTransferQtyOut(tx, to.BusinessId, to.SourceWarehouseId, item.ProductId, string(item.ProductType), item.BatchNumber, item.TransferQty.Neg(), to.TransferDate); err != nil {
			tx.Rollback()
			return err
		}
		if err := UpdateStockSummaryTransferQtyIn(tx, to.BusinessId, to.DestinationWarehouseId, item.ProductId, string(item.ProductType), item.BatchNumber, item.TransferQty, to.TransferDate); err != nil {
			tx.Rollback()
			return err
		}
	}
	return nil
}
