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
	syncTransferIn := flag.Bool("sync-transfer-in", false, "Re-sync transfer-in valuation layers to latest transfer-out")
	continueOnError := flag.Bool("continue-on-error", true, "Continue when a record fails")
	dryRun := flag.Bool("dry-run", false, "Scan only (no writes)")
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
		if *syncTransferIn {
			tx := db.Begin()
			changed, err := workflow.SyncTransferOrderTransferInFromOutgoing(tx, logger, strings.TrimSpace(*businessID), *transferOrderID, true)
			if err != nil {
				_ = tx.Rollback()
				fmt.Fprintf(os.Stderr, "failed: %v\n", err)
				os.Exit(1)
			}
			if *dryRun {
				_ = tx.Rollback()
			} else if err := tx.Commit().Error; err != nil {
				fmt.Fprintf(os.Stderr, "failed: %v\n", err)
				os.Exit(1)
			}
			fmt.Printf("transfer_order_id=%d synced_transfer_in=%t\n", *transferOrderID, changed)
			return
		}
		// Dry-run: show whether it nets to 0 (and current net value)
		if *dryRun {
			type oneRow struct {
				Net decimalString `gorm:"column:net"`
				Cnt int64         `gorm:"column:cnt"`
			}
			var r oneRow
			if err := db.Raw(`
				SELECT
					COALESCE(SUM(qty * base_unit_value), 0) AS net,
					COUNT(*) AS cnt
				FROM stock_histories
				WHERE business_id = ?
				  AND reference_type = 'TO'
				  AND reference_id = ?
				  AND is_reversal = 0
				  AND reversed_by_stock_history_id IS NULL
			`, strings.TrimSpace(*businessID), *transferOrderID).Scan(&r).Error; err != nil {
				fmt.Fprintf(os.Stderr, "failed: %v\n", err)
				os.Exit(1)
			}
			fmt.Printf("transfer_order_id=%d rows=%d net_ledger_value=%s\n", *transferOrderID, r.Cnt, r.Net)

			type shRow struct {
				ID              int           `gorm:"column:id"`
				WarehouseId     int           `gorm:"column:warehouse_id"`
				ProductId       int           `gorm:"column:product_id"`
				ProductType     string        `gorm:"column:product_type"`
				BatchNumber     string        `gorm:"column:batch_number"`
				Qty             decimalString `gorm:"column:qty"`
				BaseUnitValue   decimalString `gorm:"column:base_unit_value"`
				IsOutgoing      bool          `gorm:"column:is_outgoing"`
				IsTransferIn    bool          `gorm:"column:is_transfer_in"`
				IsReversal      bool          `gorm:"column:is_reversal"`
				ReversedById    *int          `gorm:"column:reversed_by_stock_history_id"`
				ReversesId      *int          `gorm:"column:reverses_stock_history_id"`
				ReferenceDetail int           `gorm:"column:reference_detail_id"`
				StockDate       string        `gorm:"column:stock_date"`
				Description     string        `gorm:"column:description"`
			}
			var rows []shRow
			if err := db.Raw(`
				SELECT
					id,
					warehouse_id,
					product_id,
					product_type,
					COALESCE(batch_number,'') AS batch_number,
					CAST(qty AS CHAR) AS qty,
					CAST(base_unit_value AS CHAR) AS base_unit_value,
					COALESCE(is_outgoing, 0) AS is_outgoing,
					COALESCE(is_transfer_in, 0) AS is_transfer_in,
					COALESCE(is_reversal, 0) AS is_reversal,
					reversed_by_stock_history_id,
					reverses_stock_history_id,
					reference_detail_id,
					DATE_FORMAT(stock_date, '%Y-%m-%d %H:%i:%s') AS stock_date,
					description
				FROM stock_histories
				WHERE business_id = ?
				  AND reference_type = 'TO'
				  AND reference_id = ?
				  AND is_reversal = 0
				  AND reversed_by_stock_history_id IS NULL
				ORDER BY id ASC
			`, strings.TrimSpace(*businessID), *transferOrderID).Scan(&rows).Error; err != nil {
				fmt.Fprintf(os.Stderr, "failed: %v\n", err)
				os.Exit(1)
			}
			for _, rr := range rows {
				fmt.Printf("  sh_id=%d wh=%d product=%s-%d batch=%q qty=%s unit=%s outgoing=%t transfer_in=%t detail_id=%d date=%s desc=%q\n",
					rr.ID, rr.WarehouseId, rr.ProductType, rr.ProductId, rr.BatchNumber, rr.Qty, rr.BaseUnitValue, rr.IsOutgoing, rr.IsTransferIn, rr.ReferenceDetail, rr.StockDate, rr.Description)
			}

			// Also print ALL rows (including reversals), so we can see historical broken pairs.
			fmt.Println("  -- all stock_histories rows for this TO (including reversals) --")
			var allRows []shRow
			if err := db.Raw(`
				SELECT
					id,
					warehouse_id,
					product_id,
					product_type,
					COALESCE(batch_number,'') AS batch_number,
					CAST(qty AS CHAR) AS qty,
					CAST(base_unit_value AS CHAR) AS base_unit_value,
					COALESCE(is_outgoing, 0) AS is_outgoing,
					COALESCE(is_transfer_in, 0) AS is_transfer_in,
					COALESCE(is_reversal, 0) AS is_reversal,
					reversed_by_stock_history_id,
					reverses_stock_history_id,
					reference_detail_id,
					DATE_FORMAT(stock_date, '%Y-%m-%d %H:%i:%s') AS stock_date,
					description
				FROM stock_histories
				WHERE business_id = ?
				  AND reference_type = 'TO'
				  AND reference_id = ?
				ORDER BY id ASC
			`, strings.TrimSpace(*businessID), *transferOrderID).Scan(&allRows).Error; err != nil {
				fmt.Fprintf(os.Stderr, "failed: %v\n", err)
				os.Exit(1)
			}
			for _, rr := range allRows {
				fmt.Printf("  sh_id=%d wh=%d product=%s-%d qty=%s unit=%s outgoing=%t transfer_in=%t is_reversal=%t reverses=%v reversed_by=%v detail_id=%d date=%s desc=%q\n",
					rr.ID, rr.WarehouseId, rr.ProductType, rr.ProductId, rr.Qty, rr.BaseUnitValue, rr.IsOutgoing, rr.IsTransferIn, rr.IsReversal, rr.ReversesId, rr.ReversedById, rr.ReferenceDetail, rr.StockDate, rr.Description)
			}
			return
		}

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

	// Batch sync: re-align transfer-in rows for all transfer orders.
	if *syncTransferIn {
		type row struct {
			ID int `gorm:"column:id"`
		}
		var rows []row
		if err := db.Raw(`
			SELECT id
			FROM transfer_orders
			WHERE business_id = ?
			ORDER BY id ASC
		`, strings.TrimSpace(*businessID)).Scan(&rows).Error; err != nil {
			fmt.Fprintf(os.Stderr, "scan failed: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Found %d transfer orders to sync\n", len(rows))
		for _, r := range rows {
			tx := db.Begin()
			changed, err := workflow.SyncTransferOrderTransferInFromOutgoing(tx, logger, strings.TrimSpace(*businessID), r.ID, true)
			if err != nil {
				_ = tx.Rollback()
				if *continueOnError {
					fmt.Fprintf(os.Stderr, "transfer_order_id=%d failed (skipping): %v\n", r.ID, err)
					continue
				}
				fmt.Fprintf(os.Stderr, "transfer_order_id=%d failed: %v\n", r.ID, err)
				os.Exit(1)
			}
			if *dryRun {
				_ = tx.Rollback()
			} else if err := tx.Commit().Error; err != nil {
				if *continueOnError {
					fmt.Fprintf(os.Stderr, "transfer_order_id=%d failed (skipping): %v\n", r.ID, err)
					continue
				}
				fmt.Fprintf(os.Stderr, "transfer_order_id=%d failed: %v\n", r.ID, err)
				os.Exit(1)
			}
			if changed {
				fmt.Printf("transfer_order_id=%d synced_transfer_in=%t\n", r.ID, changed)
			}
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

	if *dryRun {
		// Print net values without modifying anything.
		type netRow struct {
			RefId int          `gorm:"column:reference_id"`
			Net   decimalString `gorm:"column:net"`
			Cnt   int64        `gorm:"column:cnt"`
		}
		var nets []netRow
		if err := db.Raw(`
			SELECT
				reference_id,
				COALESCE(SUM(qty * base_unit_value), 0) AS net,
				COUNT(*) AS cnt
			FROM stock_histories
			WHERE business_id = ?
			  AND reference_type = 'TO'
			  AND is_reversal = 0
			  AND reversed_by_stock_history_id IS NULL
			GROUP BY reference_id
			HAVING ABS(COALESCE(SUM(qty * base_unit_value), 0)) > 0
			ORDER BY ABS(COALESCE(SUM(qty * base_unit_value), 0)) DESC, reference_id ASC
		`, strings.TrimSpace(*businessID)).Scan(&nets).Error; err != nil {
			fmt.Fprintf(os.Stderr, "scan failed: %v\n", err)
			os.Exit(1)
		}
		for _, n := range nets {
			fmt.Printf("transfer_order_id=%d rows=%d net_ledger_value=%s\n", n.RefId, n.Cnt, n.Net)
		}
		return
	}

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

// decimalString is a tiny helper to scan DECIMAL into string without float rounding in fmt.
type decimalString string

