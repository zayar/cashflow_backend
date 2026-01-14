package models

import (
	"fmt"

	"github.com/mmdatafocus/books_backend/utils"
	"gorm.io/gorm"
)

// ApplyCreditNoteStockForStatusTransition applies stock changes for a CreditNote status transition.
//
// Draft -> Confirmed : increase received/current qty (goods returned by customer)
// Confirmed -> Draft/Void : reverse
func ApplyCreditNoteStockForStatusTransition(tx *gorm.DB, cn *CreditNote, oldStatus CreditNoteStatus) error {
	if tx == nil {
		return fmt.Errorf("tx is nil")
	}
	if cn == nil {
		return fmt.Errorf("credit note is nil")
	}
	if string(oldStatus) == string(cn.CurrentStatus) {
		return nil
	}

	applyReturn := oldStatus == CreditNoteStatusDraft && cn.CurrentStatus == CreditNoteStatusConfirmed
	reverseReturn := oldStatus == CreditNoteStatusConfirmed &&
		(cn.CurrentStatus == CreditNoteStatusDraft || cn.CurrentStatus == CreditNoteStatusVoid)
	if !applyReturn && !reverseReturn {
		return nil
	}

	ctx := tx.Statement.Context
	if err := utils.BusinessLock(ctx, cn.BusinessId, "stockLock", "stockCommands_creditNote.go", "ApplyCreditNoteStockForStatusTransition"); err != nil {
		tx.Rollback()
		return err
	}

	for _, item := range cn.Details {
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
		if reverseReturn {
			qty = qty.Neg()
		}

		if err := UpdateStockSummaryReceivedQty(
			tx,
			cn.BusinessId,
			cn.WarehouseId,
			item.ProductId,
			string(item.ProductType),
			item.BatchNumber,
			qty,
			cn.CreditNoteDate,
		); err != nil {
			tx.Rollback()
			return err
		}
	}

	return nil
}
