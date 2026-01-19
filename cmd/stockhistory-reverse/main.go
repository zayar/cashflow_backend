package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/mmdatafocus/books_backend/config"
	"github.com/mmdatafocus/books_backend/models"
	"github.com/mmdatafocus/books_backend/workflow"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

func main() {
	businessID := flag.String("business-id", "", "Required: business id (uuid)")
	stockHistoryID := flag.Int("stock-history-id", 0, "Required: stock_histories.id to reverse")
	reason := flag.String("reason", "Manual stock history dedup", "Reversal reason")
	dryRun := flag.Bool("dry-run", true, "Show record only (no writes)")
	confirm := flag.String("confirm", "", "Type REVERSE to proceed when dry-run=false")
	flag.Parse()

	if strings.TrimSpace(*businessID) == "" || *stockHistoryID <= 0 {
		fmt.Fprintln(os.Stderr, "--business-id and --stock-history-id are required")
		os.Exit(1)
	}
	if !*dryRun && strings.TrimSpace(*confirm) != "REVERSE" {
		fmt.Fprintln(os.Stderr, "set --confirm=REVERSE to proceed")
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

	logger := logrus.New()
	if err := db.Transaction(func(tx *gorm.DB) error {
		var sh models.StockHistory
		if err := tx.
			Where("business_id = ? AND id = ?", *businessID, *stockHistoryID).
			First(&sh).Error; err != nil {
			return err
		}
		reversals, err := workflow.ReverseStockHistories(tx, []*models.StockHistory{&sh}, *reason)
		if err != nil {
			return err
		}
		if _, err := workflow.ProcessStockHistories(tx, logger, reversals); err != nil {
			return err
		}
		return nil
	}); err != nil {
		fmt.Fprintf(os.Stderr, "reverse failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("stock history reversed")
}

func printRecord(db *gorm.DB, businessID string, stockHistoryID int) {
	var sh models.StockHistory
	if err := db.
		Where("business_id = ? AND id = ?", businessID, stockHistoryID).
		First(&sh).Error; err != nil {
		fmt.Fprintf(os.Stderr, "not found: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("id=%d business_id=%s product_id=%d product_type=%s warehouse_id=%d qty=%s unit_cost=%s stock_date=%s reference_type=%s reference_id=%d reference_detail_id=%d is_reversal=%v reversed_by=%v\n",
		sh.ID, sh.BusinessId, sh.ProductId, sh.ProductType, sh.WarehouseId, sh.Qty.String(), sh.BaseUnitValue.String(),
		sh.StockDate.Format("2006-01-02 15:04:05"), sh.ReferenceType, sh.ReferenceID, sh.ReferenceDetailID, sh.IsReversal, sh.ReversedByStockHistoryId)
}
