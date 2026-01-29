package main

import (
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/mmdatafocus/books_backend/config"
	"github.com/mmdatafocus/books_backend/models"
	"github.com/mmdatafocus/books_backend/workflow"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

// transfer-order-delete-cleanup reverses ledger rows for transfer orders that were deleted
// in the UI but never reversed in the ledger (historical bug: no delete workflow).
//
// Dry-run (default): show counts only
//   go run ./cmd/transfer-order-delete-cleanup -business-id=... -dry-run=true
//
// Execute:
//   go run ./cmd/transfer-order-delete-cleanup -business-id=... -dry-run=false -confirm=DELETE
//
// Single transfer order:
//   go run ./cmd/transfer-order-delete-cleanup -business-id=... -transfer-order-id=71 -dry-run=false -confirm=DELETE
//
// ALL transfer orders (dangerous; reverses every TO ledger row):
//   go run ./cmd/transfer-order-delete-cleanup -business-id=... -all -dry-run=false -confirm=DELETE
func main() {
	businessID := flag.String("business-id", "", "Required: business id (uuid)")
	transferOrderID := flag.Int("transfer-order-id", 0, "Optional: clean up a single transfer order id")
	all := flag.Bool("all", false, "Clean up ALL transfer orders (even if still exists)")
	dryRun := flag.Bool("dry-run", true, "List only (no writes)")
	confirm := flag.String("confirm", "", "Type DELETE to proceed when dry-run=false")
	force := flag.Bool("force", false, "Allow cleanup even if transfer order still exists")
	flag.Parse()

	if strings.TrimSpace(*businessID) == "" {
		fmt.Fprintln(os.Stderr, "--business-id is required")
		os.Exit(1)
	}
	if !*dryRun && strings.TrimSpace(*confirm) != "DELETE" {
		fmt.Fprintln(os.Stderr, "set --confirm=DELETE to proceed")
		os.Exit(1)
	}

	config.ConnectDatabaseWithRetry()
	db := config.GetDB()
	if db == nil {
		fmt.Fprintln(os.Stderr, "database not initialized")
		os.Exit(1)
	}
	logger := logrus.New()

	if *transferOrderID > 0 {
		if err := cleanupOne(db, logger, *businessID, *transferOrderID, *dryRun, *force); err != nil {
			fmt.Fprintf(os.Stderr, "cleanup failed: %v\n", err)
			os.Exit(1)
		}
		return
	}

	if *all {
		// Clean ALL transfer orders (by ledger reference_id).
		type row struct {
			RefID int `gorm:"column:ref_id"`
		}
		var refs []row
		if err := db.Raw(`
			SELECT DISTINCT sh.reference_id AS ref_id
			FROM stock_histories sh
			WHERE sh.business_id = ?
			  AND sh.reference_type = 'TO'
			  AND sh.is_reversal = 0
			  AND sh.reversed_by_stock_history_id IS NULL
			ORDER BY sh.reference_id
		`, strings.TrimSpace(*businessID)).Scan(&refs).Error; err != nil {
			fmt.Fprintf(os.Stderr, "scan failed: %v\n", err)
			os.Exit(1)
		}
		if len(refs) == 0 {
			fmt.Println("no transfer order ledger rows found")
			return
		}
		fmt.Printf("found %d transfer orders (ledger refs)\n", len(refs))
		for _, r := range refs {
			if err := cleanupOne(db, logger, *businessID, r.RefID, *dryRun, true); err != nil {
				fmt.Fprintf(os.Stderr, "cleanup failed for transfer_order_id=%d: %v\n", r.RefID, err)
				os.Exit(1)
			}
		}
		return
	}

	// Find orphan transfer orders: stock_histories exist but transfer_orders row is missing.
	type row struct {
		RefID int `gorm:"column:ref_id"`
	}
	var refs []row
	if err := db.Raw(`
		SELECT DISTINCT sh.reference_id AS ref_id
		FROM stock_histories sh
		LEFT JOIN transfer_orders t
		  ON t.id = sh.reference_id AND t.business_id = sh.business_id
		WHERE sh.business_id = ?
		  AND sh.reference_type = 'TO'
		  AND sh.is_reversal = 0
		  AND sh.reversed_by_stock_history_id IS NULL
		  AND t.id IS NULL
		ORDER BY sh.reference_id
	`, strings.TrimSpace(*businessID)).Scan(&refs).Error; err != nil {
		fmt.Fprintf(os.Stderr, "scan failed: %v\n", err)
		os.Exit(1)
	}
	if len(refs) == 0 {
		fmt.Println("no orphan transfer orders found")
		return
	}

	fmt.Printf("found %d orphan transfer orders\n", len(refs))
	for _, r := range refs {
		if err := cleanupOne(db, logger, *businessID, r.RefID, *dryRun, true); err != nil {
			fmt.Fprintf(os.Stderr, "cleanup failed for transfer_order_id=%d: %v\n", r.RefID, err)
			os.Exit(1)
		}
	}
}

func cleanupOne(db *gorm.DB, logger *logrus.Logger, businessID string, transferOrderID int, dryRun bool, force bool) error {
	if transferOrderID <= 0 {
		return fmt.Errorf("invalid transfer_order_id")
	}

	// If transfer order exists, skip unless forced.
	var count int64
	if err := db.Model(&models.TransferOrder{}).
		Where("business_id = ? AND id = ?", businessID, transferOrderID).
		Count(&count).Error; err != nil {
		return err
	}
	if count > 0 && !force {
		fmt.Printf("transfer_order_id=%d exists; skipping (use --force to override)\n", transferOrderID)
		return nil
	}

	// Gather ledger stats for reporting. (row_count avoids MySQL reserved word "rows")
	type stats struct {
		Rows int64  `gorm:"column:row_count"`
		Net  string `gorm:"column:net"`
	}
	var s stats
	if err := db.Raw(`
		SELECT
			COUNT(*) AS row_count,
			COALESCE(SUM(qty * base_unit_value), 0) AS net
		FROM stock_histories
		WHERE business_id = ?
		  AND reference_type = 'TO'
		  AND reference_id = ?
		  AND is_reversal = 0
		  AND reversed_by_stock_history_id IS NULL
	`, businessID, transferOrderID).Scan(&s).Error; err != nil {
		return err
	}
	var journalCount int64
	if err := db.Model(&models.AccountJournal{}).
		Where("reference_id = ? AND reference_type = ? AND is_reversal = 0 AND reversed_by_journal_id IS NULL",
			transferOrderID, models.AccountReferenceTypeTransferOrder).
		Count(&journalCount).Error; err != nil {
		return err
	}

	fmt.Printf("transfer_order_id=%d ledger_rows=%d net_ledger_value=%s active_journals=%d\n",
		transferOrderID, s.Rows, s.Net, journalCount)

	if dryRun {
		return nil
	}

	return db.Transaction(func(tx *gorm.DB) error {
		business, err := models.GetBusinessById2(tx, businessID)
		if err != nil {
			return err
		}

		// Reverse journals (source + destination).
		var journals []models.AccountJournal
		if err := tx.Preload("AccountTransactions").
			Where("reference_id = ? AND reference_type = ? AND is_reversal = 0 AND reversed_by_journal_id IS NULL",
				transferOrderID, models.AccountReferenceTypeTransferOrder).
			Find(&journals).Error; err != nil {
			return err
		}
		accountIds := make([]int, 0)
		branchIds := make(map[int]struct{})
		var txnDate time.Time
		for _, j := range journals {
			if j.ID == 0 {
				continue
			}
			branchIds[j.BranchId] = struct{}{}
			if txnDate.IsZero() || j.TransactionDateTime.Before(txnDate) {
				txnDate = j.TransactionDateTime
			}
			for _, t := range j.AccountTransactions {
				if !containsInt(accountIds, t.AccountId) {
					accountIds = append(accountIds, t.AccountId)
				}
			}
			if _, err := workflow.ReverseAccountJournal(tx, &j, workflow.ReversalReasonTransferOrderVoidUpdate); err != nil {
				return err
			}
		}

		// Reverse stock histories.
		var stockHistories []*models.StockHistory
		if err := tx.
			Where("reference_id = ? AND reference_type = ? AND is_reversal = 0 AND reversed_by_stock_history_id IS NULL",
				transferOrderID, models.StockReferenceTypeTransferOrder).
			Find(&stockHistories).Error; err != nil {
			return err
		}
		stockReversals, err := workflow.ReverseStockHistories(tx, stockHistories, workflow.ReversalReasonTransferOrderVoidUpdate)
		if err != nil {
			return err
		}
		if len(stockReversals) > 0 {
			if _, err := workflow.ProcessStockHistories(tx, logger, stockReversals); err != nil {
				return err
			}
		}

		// Update balances for all affected branches (if journals existed).
		if len(accountIds) > 0 {
			if txnDate.IsZero() {
				txnDate = time.Now().UTC()
			}
			for bid := range branchIds {
				if err := workflow.UpdateBalances(tx, logger, businessID, business.BaseCurrencyId, bid, accountIds, txnDate, business.BaseCurrencyId); err != nil {
					return err
				}
			}
		}
		return nil
	})
}

func containsInt(items []int, v int) bool {
	for _, i := range items {
		if i == v {
			return true
		}
	}
	return false
}

