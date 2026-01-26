package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/mmdatafocus/books_backend/config"
	"github.com/mmdatafocus/books_backend/workflow"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

func main() {
	businessID := flag.String("business-id", "", "Required: business id (uuid)")
	transferOrderID := flag.Int("transfer-order-id", 0, "Optional: fix a single transfer order id")
	continueOnError := flag.Bool("continue-on-error", true, "Continue when a record fails")
	flag.Parse()

	if strings.TrimSpace(*businessID) == "" {
		fmt.Fprintln(os.Stderr, "--business-id is required")
		os.Exit(2)
	}

	config.ConnectDatabaseWithRetry()
	db := config.GetDB()
	if db == nil {
		fmt.Fprintln(os.Stderr, "database not initialized")
		os.Exit(1)
	}

	logger := logrus.New()

	// Fix one TO
	if *transferOrderID > 0 {
		if err := db.Transaction(func(tx *gorm.DB) error {
			created, _, err := workflow.BackfillTransferOrderMissingTransferIn(tx, logger, strings.TrimSpace(*businessID), *transferOrderID)
			if err != nil {
				return err
			}
			fmt.Printf("transfer_order_id=%d created_transfer_in_rows=%d\n", *transferOrderID, created)
			return nil
		}); err != nil {
			fmt.Fprintf(os.Stderr, "failed: %v\n", err)
			os.Exit(1)
		}
		return
	}

	// Scan: find transfer orders where stock ledger doesn't net to 0 (sum(qty*unit_cost) != 0).
	type row struct {
		RefId int `gorm:"column:reference_id"`
	}
	var rows []row
	if err := db.Raw(`
		SELECT reference_id
		FROM stock_histories
		WHERE business_id = ?
		  AND reference_type = 'TO'
		  AND is_reversal = 0
		  AND reversed_by_stock_history_id IS NULL
		GROUP BY reference_id
		HAVING ABS(COALESCE(SUM(qty * base_unit_value), 0)) > 0
		ORDER BY reference_id ASC
	`, strings.TrimSpace(*businessID)).Scan(&rows).Error; err != nil {
		fmt.Fprintf(os.Stderr, "scan failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Found %d transfer orders with non-zero net ledger value\n", len(rows))
	for _, r := range rows {
		if err := db.Transaction(func(tx *gorm.DB) error {
			created, _, err := workflow.BackfillTransferOrderMissingTransferIn(tx, logger, strings.TrimSpace(*businessID), r.RefId)
			if err != nil {
				return err
			}
			if created > 0 {
				fmt.Printf("transfer_order_id=%d created_transfer_in_rows=%d\n", r.RefId, created)
			}
			return nil
		}); err != nil {
			if *continueOnError {
				fmt.Fprintf(os.Stderr, "transfer_order_id=%d failed (skipping): %v\n", r.RefId, err)
				continue
			}
			fmt.Fprintf(os.Stderr, "transfer_order_id=%d failed: %v\n", r.RefId, err)
			os.Exit(1)
		}
	}
}

