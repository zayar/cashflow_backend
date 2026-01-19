package main

import (
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/mmdatafocus/books_backend/config"
	"github.com/mmdatafocus/books_backend/models"
	"github.com/mmdatafocus/books_backend/utils"
	"gorm.io/gorm"
)

// Simple tool to mark a duplicate stock_history row as reversed without triggering FIFO rebuild.
// This is useful when you have duplicate active rows and just want to mark one as invalid.
func main() {
	businessID := flag.String("business-id", "", "Required: business id (uuid)")
	stockHistoryID := flag.Int("stock-history-id", 0, "Required: stock_histories.id to mark as reversed")
	reason := flag.String("reason", "Manual duplicate removal", "Reversal reason")
	dryRun := flag.Bool("dry-run", true, "Show record only (no writes)")
	confirm := flag.String("confirm", "", "Type MARK_REVERSED to proceed when dry-run=false")
	flag.Parse()

	if strings.TrimSpace(*businessID) == "" || *stockHistoryID <= 0 {
		fmt.Fprintln(os.Stderr, "--business-id and --stock-history-id are required")
		os.Exit(1)
	}
	if !*dryRun && strings.TrimSpace(*confirm) != "MARK_REVERSED" {
		fmt.Fprintln(os.Stderr, "set --confirm=MARK_REVERSED to proceed")
		os.Exit(1)
	}

	config.ConnectDatabaseWithRetry()
	db := config.GetDB()
	if db == nil {
		fmt.Fprintln(os.Stderr, "database not initialized")
		os.Exit(1)
	}

	if *dryRun {
		printRecord(db, *businessID, *stockHistoryID)
		return
	}

	// Simple approach: create a reversal row and mark original as reversed, but skip FIFO processing.
	// This just marks it as invalid in the ledger without recalculating costs.
	if err := db.Transaction(func(tx *gorm.DB) error {
		var original models.StockHistory
		if err := tx.
			Where("business_id = ? AND id = ? AND is_reversal = 0 AND reversed_by_stock_history_id IS NULL", *businessID, *stockHistoryID).
			First(&original).Error; err != nil {
			return fmt.Errorf("stock history not found or already reversed: %w", err)
		}

		// Create reversal row (opposite direction)
		reversalQty := original.Qty.Neg()
		isOutgoing := false
		if original.IsOutgoing != nil {
			isOutgoing = *original.IsOutgoing
		}
		reversalIsOutgoing := !isOutgoing
		reversalIsOutgoingPtr := utils.NewFalse()
		if reversalIsOutgoing {
			reversalIsOutgoingPtr = utils.NewTrue()
		}

		now := time.Now().UTC()
		reversal := models.StockHistory{
			BusinessId:             original.BusinessId,
			WarehouseId:            original.WarehouseId,
			ProductId:              original.ProductId,
			ProductType:            original.ProductType,
			BatchNumber:            original.BatchNumber,
			StockDate:              original.StockDate,
			Qty:                    reversalQty,
			Description:            "REV: " + original.Description,
			BaseUnitValue:          original.BaseUnitValue,
			ReferenceType:          original.ReferenceType,
			ReferenceID:            original.ReferenceID,
			ReferenceDetailID:      original.ReferenceDetailID,
			IsOutgoing:             reversalIsOutgoingPtr,
			IsTransferIn:           original.IsTransferIn,
			IsReversal:             true,
			ReversesStockHistoryId: &original.ID,
			ReversalReason:         reason,
		}

		if err := tx.Create(&reversal).Error; err != nil {
			return fmt.Errorf("create reversal row: %w", err)
		}

		// Mark original as reversed (metadata only - no FIFO processing)
		if err := tx.Model(&models.StockHistory{}).
			Where("id = ?", original.ID).
			Updates(map[string]interface{}{
				"reversed_by_stock_history_id": reversal.ID,
				"reversal_reason":              reason,
				"reversed_at":                  &now,
			}).Error; err != nil {
			return fmt.Errorf("mark original as reversed: %w", err)
		}

		fmt.Printf("✓ Created reversal row id=%d for original id=%d\n", reversal.ID, original.ID)
		return nil
	}); err != nil {
		fmt.Fprintf(os.Stderr, "mark reversed failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("✓ Stock history marked as reversed (duplicate removed from active ledger)")
	fmt.Println("  Note: Run inventory rebuild to recalculate closing balances if needed")
}

func printRecord(db *gorm.DB, businessID string, stockHistoryID int) {
	var sh models.StockHistory
	if err := db.
		Where("business_id = ? AND id = ?", businessID, stockHistoryID).
		First(&sh).Error; err != nil {
		fmt.Fprintf(os.Stderr, "not found: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Will mark as reversed:\n")
	fmt.Printf("  id=%d\n", sh.ID)
	fmt.Printf("  business_id=%s\n", sh.BusinessId)
	fmt.Printf("  product_id=%d product_type=%s warehouse_id=%d\n", sh.ProductId, sh.ProductType, sh.WarehouseId)
	fmt.Printf("  qty=%s unit_cost=%s\n", sh.Qty.String(), sh.BaseUnitValue.String())
	fmt.Printf("  stock_date=%s\n", sh.StockDate.Format("2006-01-02 15:04:05"))
	fmt.Printf("  reference_type=%s reference_id=%d reference_detail_id=%d\n", sh.ReferenceType, sh.ReferenceID, sh.ReferenceDetailID)
	fmt.Printf("  is_outgoing=%v is_reversal=%v reversed_by=%v\n", sh.IsOutgoing, sh.IsReversal, sh.ReversedByStockHistoryId)
	if sh.ReversedByStockHistoryId != nil {
		fmt.Fprintf(os.Stderr, "\n⚠ WARNING: This record is already reversed!\n")
		os.Exit(1)
	}
}
