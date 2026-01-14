package models

import (
	"fmt"

	"github.com/mmdatafocus/books_backend/utils"
	"gorm.io/gorm"
)

// ApplySalesInvoiceStockForStatusTransition applies stock changes for a SalesInvoice status transition.
//
// This is the explicit, command-style replacement for implicit GORM model-hook side-effects.
// It is intended to be called from the SalesInvoice write paths (create/status update) inside the same DB transaction.
func ApplySalesInvoiceStockForStatusTransition(tx *gorm.DB, sale *SalesInvoice, oldStatus SalesInvoiceStatus) error {
	if tx == nil {
		return fmt.Errorf("tx is nil")
	}
	if sale == nil {
		return fmt.Errorf("sale invoice is nil")
	}
	if string(oldStatus) == string(sale.CurrentStatus) {
		return nil
	}

	// Only handle the states that affect inventory.
	applySale := oldStatus == SalesInvoiceStatusDraft && sale.CurrentStatus == SalesInvoiceStatusConfirmed
	reverseSale := oldStatus == SalesInvoiceStatusConfirmed && (sale.CurrentStatus == SalesInvoiceStatusDraft || sale.CurrentStatus == SalesInvoiceStatusVoid)
	if !applySale && !reverseSale {
		return nil
	}

	ctx := tx.Statement.Context

	// Serialize stock summary writes per business to avoid racy interleavings.
	if err := utils.BusinessLock(ctx, sale.BusinessId, "stockLock", "stockCommands_salesInvoice.go", "ApplySalesInvoiceStockForStatusTransition"); err != nil {
		tx.Rollback()
		return err
	}

	for _, saleItem := range sale.Details {
		if saleItem.ProductId <= 0 {
			continue
		}
		product, err := GetProductOrVariant(ctx, string(saleItem.ProductType), saleItem.ProductId)
		if err != nil {
			tx.Rollback()
			return err
		}
		if product.GetInventoryAccountID() <= 0 {
			continue
		}

		qty := saleItem.DetailQty
		if reverseSale {
			qty = qty.Neg()
		}
		if err := UpdateStockSummarySaleQty(
			tx,
			sale.BusinessId,
			sale.WarehouseId,
			saleItem.ProductId,
			string(saleItem.ProductType),
			saleItem.BatchNumber,
			qty,
			sale.InvoiceDate,
		); err != nil {
			tx.Rollback()
			return err
		}
	}

	return nil
}
