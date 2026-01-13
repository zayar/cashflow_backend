package models

import (
	"fmt"

	"bitbucket.org/mmdatafocus/books_backend/utils"
	"gorm.io/gorm"
)

// ApplySupplierCreditStockForStatusTransition applies stock changes for a SupplierCredit status transition.
//
// Draft -> Confirmed : decrease received/current qty (items returned to supplier)
// Confirmed -> Draft/Closed : reverse (increase received/current qty)
func ApplySupplierCreditStockForStatusTransition(tx *gorm.DB, sc *SupplierCredit, oldStatus SupplierCreditStatus) error {
	if tx == nil {
		return fmt.Errorf("tx is nil")
	}
	if sc == nil {
		return fmt.Errorf("supplier credit is nil")
	}
	if string(oldStatus) == string(sc.CurrentStatus) {
		return nil
	}

	applyReturn := oldStatus == SupplierCreditStatusDraft && sc.CurrentStatus == SupplierCreditStatusConfirmed
	reverseReturn := oldStatus == SupplierCreditStatusConfirmed &&
		(sc.CurrentStatus == SupplierCreditStatusDraft || sc.CurrentStatus == SupplierCreditStatusClosed || sc.CurrentStatus == SupplierCreditStatusVoid)
	if !applyReturn && !reverseReturn {
		return nil
	}

	ctx := tx.Statement.Context
	if err := utils.BusinessLock(ctx, sc.BusinessId, "stockLock", "stockCommands_supplierCredit.go", "ApplySupplierCreditStockForStatusTransition"); err != nil {
		tx.Rollback()
		return err
	}

	for _, item := range sc.Details {
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

		qty := item.DetailQty.Neg()
		if reverseReturn {
			qty = item.DetailQty
		}

		if err := UpdateStockSummaryReceivedQty(
			tx,
			sc.BusinessId,
			sc.WarehouseId,
			item.ProductId,
			string(item.ProductType),
			item.BatchNumber,
			qty,
			sc.SupplierCreditDate,
		); err != nil {
			tx.Rollback()
			return err
		}
	}
	return nil
}

