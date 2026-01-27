package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/mmdatafocus/books_backend/config"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

// no-batch-migrate enforces "no batch mode" data hygiene by flattening per-batch derived tables.
//
// It is intended to be run once per business (off-hours):
// - stock_histories.batch_number -> ” (optional but recommended)
// - stock_summaries -> merged across batches into batch_number=”
// - stock_summary_daily_balances -> merged across batches into batch_number=”
// - document detail tables -> batch_number=” (kept for cleanliness)
//
// After running, re-run inventory rebuild if you want to recompute FIFO/closing balances from history.
func main() {
	businessID := flag.String("business-id", "", "Required: business id (uuid)")
	dryRun := flag.Bool("dry-run", false, "If true, do not write; only print actions")
	flag.Parse()

	if strings.TrimSpace(*businessID) == "" {
		fmt.Fprintln(os.Stderr, "--business-id is required")
		os.Exit(1)
	}

	config.ConnectDatabaseWithRetry()
	db := config.GetDB()
	if db == nil {
		fmt.Fprintln(os.Stderr, "database not initialized")
		os.Exit(1)
	}
	logger := logrus.New()

	if *dryRun {
		fmt.Println("[dry-run] no changes will be written")
	}

	err := db.Transaction(func(tx *gorm.DB) error {
		if *dryRun {
			return nil
		}

		// 1) Stock histories: remove batch numbers.
		if err := tx.Exec(`
			UPDATE stock_histories
			SET batch_number = ''
			WHERE business_id = ? AND COALESCE(batch_number,'') <> ''
		`, *businessID).Error; err != nil {
			return err
		}

		// 2) Flatten stock_summaries into batch_number=''
		if err := tx.Exec(`DROP TEMPORARY TABLE IF EXISTS tmp_stock_summaries`).Error; err != nil {
			return err
		}
		if err := tx.Exec(`
			CREATE TEMPORARY TABLE tmp_stock_summaries AS
			SELECT
				business_id,
				warehouse_id,
				product_id,
				product_type,
				'' AS batch_number,
				COALESCE(SUM(opening_qty), 0) AS opening_qty,
				COALESCE(SUM(order_qty), 0) AS order_qty,
				COALESCE(SUM(received_qty), 0) AS received_qty,
				COALESCE(SUM(sale_qty), 0) AS sale_qty,
				COALESCE(SUM(committed_qty), 0) AS committed_qty,
				COALESCE(SUM(transfer_qty_in), 0) AS transfer_qty_in,
				COALESCE(SUM(transfer_qty_out), 0) AS transfer_qty_out,
				COALESCE(SUM(adjusted_qty_in), 0) AS adjusted_qty_in,
				COALESCE(SUM(adjusted_qty_out), 0) AS adjusted_qty_out,
				COALESCE(SUM(current_qty), 0) AS current_qty
			FROM stock_summaries
			WHERE business_id = ?
			GROUP BY business_id, warehouse_id, product_id, product_type
		`, *businessID).Error; err != nil {
			return err
		}
		if err := tx.Exec(`DELETE FROM stock_summaries WHERE business_id = ?`, *businessID).Error; err != nil {
			return err
		}
		if err := tx.Exec(`
			INSERT INTO stock_summaries (
				business_id, warehouse_id, product_id, product_type, batch_number,
				opening_qty, order_qty, received_qty, sale_qty, committed_qty,
				transfer_qty_in, transfer_qty_out, adjusted_qty_in, adjusted_qty_out, current_qty
			)
			SELECT
				business_id, warehouse_id, product_id, product_type, batch_number,
				opening_qty, order_qty, received_qty, sale_qty, committed_qty,
				transfer_qty_in, transfer_qty_out, adjusted_qty_in, adjusted_qty_out, current_qty
			FROM tmp_stock_summaries
		`).Error; err != nil {
			return err
		}
		if err := tx.Exec(`DROP TEMPORARY TABLE IF EXISTS tmp_stock_summaries`).Error; err != nil {
			return err
		}

		// 3) Flatten stock_summary_daily_balances into batch_number=''
		if err := tx.Exec(`DROP TEMPORARY TABLE IF EXISTS tmp_stock_sdb`).Error; err != nil {
			return err
		}
		if err := tx.Exec(`
			CREATE TEMPORARY TABLE tmp_stock_sdb AS
			SELECT
				business_id,
				warehouse_id,
				product_id,
				product_type,
				'' AS batch_number,
				transaction_date,
				COALESCE(SUM(opening_qty), 0) AS opening_qty,
				COALESCE(SUM(order_qty), 0) AS order_qty,
				COALESCE(SUM(received_qty), 0) AS received_qty,
				COALESCE(SUM(sale_qty), 0) AS sale_qty,
				COALESCE(SUM(committed_qty), 0) AS committed_qty,
				COALESCE(SUM(transfer_qty_in), 0) AS transfer_qty_in,
				COALESCE(SUM(transfer_qty_out), 0) AS transfer_qty_out,
				COALESCE(SUM(adjusted_qty_in), 0) AS adjusted_qty_in,
				COALESCE(SUM(adjusted_qty_out), 0) AS adjusted_qty_out,
				COALESCE(SUM(current_qty), 0) AS current_qty
			FROM stock_summary_daily_balances
			WHERE business_id = ?
			GROUP BY business_id, warehouse_id, product_id, product_type, transaction_date
		`, *businessID).Error; err != nil {
			return err
		}
		if err := tx.Exec(`DELETE FROM stock_summary_daily_balances WHERE business_id = ?`, *businessID).Error; err != nil {
			return err
		}
		if err := tx.Exec(`
			INSERT INTO stock_summary_daily_balances (
				business_id, warehouse_id, product_id, product_type, batch_number, transaction_date,
				opening_qty, order_qty, received_qty, sale_qty, committed_qty,
				transfer_qty_in, transfer_qty_out, adjusted_qty_in, adjusted_qty_out, current_qty
			)
			SELECT
				business_id, warehouse_id, product_id, product_type, batch_number, transaction_date,
				opening_qty, order_qty, received_qty, sale_qty, committed_qty,
				transfer_qty_in, transfer_qty_out, adjusted_qty_in, adjusted_qty_out, current_qty
			FROM tmp_stock_sdb
		`).Error; err != nil {
			return err
		}
		if err := tx.Exec(`DROP TEMPORARY TABLE IF EXISTS tmp_stock_sdb`).Error; err != nil {
			return err
		}

		// 4) Document detail tables: remove batch numbers (cleanliness only).
		stmts := []struct {
			name string
			sql  string
		}{
			{
				name: "sales_invoice_details",
				sql: `
					UPDATE sales_invoice_details d
					JOIN sales_invoices h ON h.id = d.sales_invoice_id
					SET d.batch_number = ''
					WHERE h.business_id = ? AND COALESCE(d.batch_number,'') <> ''
				`,
			},
			{
				name: "sales_order_details",
				sql: `
					UPDATE sales_order_details d
					JOIN sales_orders h ON h.id = d.sales_order_id
					SET d.batch_number = ''
					WHERE h.business_id = ? AND COALESCE(d.batch_number,'') <> ''
				`,
			},
			{
				name: "bill_details",
				sql: `
					UPDATE bill_details d
					JOIN bills h ON h.id = d.bill_id
					SET d.batch_number = ''
					WHERE h.business_id = ? AND COALESCE(d.batch_number,'') <> ''
				`,
			},
			{
				name: "purchase_order_details",
				sql: `
					UPDATE purchase_order_details d
					JOIN purchase_orders h ON h.id = d.purchase_order_id
					SET d.batch_number = ''
					WHERE h.business_id = ? AND COALESCE(d.batch_number,'') <> ''
				`,
			},
			{
				name: "credit_note_details",
				sql: `
					UPDATE credit_note_details d
					JOIN credit_notes h ON h.id = d.credit_note_id
					SET d.batch_number = ''
					WHERE h.business_id = ? AND COALESCE(d.batch_number,'') <> ''
				`,
			},
			{
				name: "supplier_credit_details",
				sql: `
					UPDATE supplier_credit_details d
					JOIN supplier_credits h ON h.id = d.supplier_credit_id
					SET d.batch_number = ''
					WHERE h.business_id = ? AND COALESCE(d.batch_number,'') <> ''
				`,
			},
			{
				name: "inventory_adjustment_details",
				sql: `
					UPDATE inventory_adjustment_details d
					JOIN inventory_adjustments h ON h.id = d.inventory_adjustment_id
					SET d.batch_number = ''
					WHERE h.business_id = ? AND COALESCE(d.batch_number,'') <> ''
				`,
			},
			{
				name: "transfer_order_details",
				sql: `
					UPDATE transfer_order_details d
					JOIN transfer_orders h ON h.id = d.transfer_order_id
					SET d.batch_number = ''
					WHERE h.business_id = ? AND COALESCE(d.batch_number,'') <> ''
				`,
			},
		}
		for _, s := range stmts {
			if err := tx.Exec(s.sql, *businessID).Error; err != nil {
				return fmt.Errorf("%s cleanup: %w", s.name, err)
			}
		}

		return nil
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "no-batch migrate failed: %v\n", err)
		os.Exit(1)
	}

	logger.WithField("business_id", strings.TrimSpace(*businessID)).Info("no-batch migrate complete")
	fmt.Println("no-batch migrate complete")
}
