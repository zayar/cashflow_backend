package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/mmdatafocus/books_backend/config"
	"github.com/mmdatafocus/books_backend/models"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

func main() {
	businessID := flag.String("business-id", "", "Required: business id (uuid)")
	dryRun := flag.Bool("dry-run", true, "Show counts only (no writes)")
	confirm := flag.String("confirm", "", "Type RESET to proceed when dry-run=false")
	resetAccounting := flag.Bool("reset-accounting", true, "Delete inventory-related journals/transactions")
	resetDocuments := flag.Bool("reset-documents", false, "Delete inventory documents (destructive)")
	resetOutbox := flag.Bool("reset-outbox", true, "Delete inventory outbox records")
	flag.Parse()

	if strings.TrimSpace(*businessID) == "" {
		fmt.Fprintln(os.Stderr, "--business-id is required")
		os.Exit(1)
	}
	if !*dryRun && strings.TrimSpace(*confirm) != "RESET" {
		fmt.Fprintln(os.Stderr, "set --confirm=RESET to proceed")
		os.Exit(1)
	}

	config.ConnectDatabaseWithRetry()
	db := config.GetDB()
	if db == nil {
		fmt.Fprintln(os.Stderr, "database not initialized")
		os.Exit(1)
	}
	logger := logrus.New()

	var biz models.Business
	if err := db.Where("id = ?", *businessID).First(&biz).Error; err != nil {
		fmt.Fprintf(os.Stderr, "business not found: %v\n", err)
		os.Exit(1)
	}

	if *dryRun {
		printCounts(db, *businessID)
		return
	}

	if err := db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("business_id = ?", *businessID).Delete(&models.StockSummaryDailyBalance{}).Error; err != nil {
			return err
		}
		if err := tx.Where("business_id = ?", *businessID).Delete(&models.StockSummary{}).Error; err != nil {
			return err
		}
		if err := tx.Where("business_id = ?", *businessID).Delete(&models.StockHistory{}).Error; err != nil {
			return err
		}
		if err := tx.Where("business_id = ?", *businessID).Delete(&models.InventoryMovement{}).Error; err != nil {
			return err
		}
		if err := tx.Where("business_id = ?", *businessID).Delete(&models.CogsAllocation{}).Error; err != nil {
			return err
		}

		// Reset COGS on details.
		if err := tx.Exec(`
			UPDATE sales_invoice_details d
			INNER JOIN sales_invoices i ON i.id = d.sales_invoice_id
			SET d.cogs = 0
			WHERE i.business_id = ?`, *businessID).Error; err != nil {
			return err
		}
		if err := tx.Exec(`
			UPDATE supplier_credit_details d
			INNER JOIN supplier_credits s ON s.id = d.supplier_credit_id
			SET d.cogs = 0
			WHERE s.business_id = ?`, *businessID).Error; err != nil {
			return err
		}
		if err := tx.Exec(`
			UPDATE credit_note_details d
			INNER JOIN credit_notes c ON c.id = d.credit_note_id
			SET d.cogs = 0
			WHERE c.business_id = ?`, *businessID).Error; err != nil {
			return err
		}

		if *resetOutbox {
			if err := tx.Where("business_id = ? AND reference_type IN ('POS','PGOS','PCOS','BL','IV','CN','SC','TO','IVAQ','IVAV')", *businessID).
				Delete(&models.PubSubMessageRecord{}).Error; err != nil {
				return err
			}
		}

		if *resetAccounting {
			var journalIDs []int
			if err := tx.Model(&models.AccountJournal{}).
				Where("business_id = ? AND reference_type IN ('POS','PGOS','PCOS','BL','IV','CN','SC','TO','IVAQ','IVAV')", *businessID).
				Pluck("id", &journalIDs).Error; err != nil {
				return err
			}
			if len(journalIDs) > 0 {
				if err := tx.Session(&gorm.Session{SkipHooks: true}).
					Where("business_id = ? AND journal_id IN ?", *businessID, journalIDs).
					Delete(&models.AccountTransaction{}).Error; err != nil {
					return err
				}
				if err := tx.Session(&gorm.Session{SkipHooks: true}).
					Where("business_id = ? AND id IN ?", *businessID, journalIDs).
					Delete(&models.AccountJournal{}).Error; err != nil {
					return err
				}
			}
		}

		if *resetDocuments {
			if err := tx.Where("business_id = ?", *businessID).Delete(&models.InventoryAdjustmentDetail{}).Error; err != nil {
				return err
			}
			if err := tx.Where("business_id = ?", *businessID).Delete(&models.InventoryAdjustment{}).Error; err != nil {
				return err
			}
			if err := tx.Where("business_id = ?", *businessID).Delete(&models.TransferOrderDetail{}).Error; err != nil {
				return err
			}
			if err := tx.Where("business_id = ?", *businessID).Delete(&models.TransferOrder{}).Error; err != nil {
				return err
			}
			if err := tx.Where("business_id = ?", *businessID).Delete(&models.BillDetail{}).Error; err != nil {
				return err
			}
			if err := tx.Where("business_id = ?", *businessID).Delete(&models.Bill{}).Error; err != nil {
				return err
			}
			if err := tx.Where("business_id = ?", *businessID).Delete(&models.SupplierCreditDetail{}).Error; err != nil {
				return err
			}
			if err := tx.Where("business_id = ?", *businessID).Delete(&models.SupplierCredit{}).Error; err != nil {
				return err
			}
			if err := tx.Where("business_id = ?", *businessID).Delete(&models.CreditNoteDetail{}).Error; err != nil {
				return err
			}
			if err := tx.Where("business_id = ?", *businessID).Delete(&models.CreditNote{}).Error; err != nil {
				return err
			}
			if err := tx.Where("business_id = ?", *businessID).Delete(&models.SalesInvoiceDetail{}).Error; err != nil {
				return err
			}
			if err := tx.Where("business_id = ?", *businessID).Delete(&models.SalesInvoice{}).Error; err != nil {
				return err
			}
		}

		return nil
	}); err != nil {
		logger.WithError(err).Error("inventory reset failed")
		os.Exit(1)
	}

	fmt.Println("inventory reset completed")
}

func printCounts(db *gorm.DB, businessID string) {
	type countRow struct {
		Name  string
		Count int64
	}
	var counts []countRow
	pushCount := func(name string, query string, args ...interface{}) {
		var c int64
		_ = db.Raw(query, args...).Scan(&c).Error
		counts = append(counts, countRow{Name: name, Count: c})
	}

	pushCount("stock_histories", "SELECT COUNT(*) FROM stock_histories WHERE business_id = ?", businessID)
	pushCount("stock_summaries", "SELECT COUNT(*) FROM stock_summaries WHERE business_id = ?", businessID)
	pushCount("stock_summary_daily_balances", "SELECT COUNT(*) FROM stock_summary_daily_balances WHERE business_id = ?", businessID)
	pushCount("inventory_movements", "SELECT COUNT(*) FROM inventory_movements WHERE business_id = ?", businessID)
	pushCount("cogs_allocations", "SELECT COUNT(*) FROM cogs_allocations WHERE business_id = ?", businessID)
	pushCount("invoice_details", `
		SELECT COUNT(*) FROM sales_invoice_details d
		INNER JOIN sales_invoices i ON i.id = d.sales_invoice_id
		WHERE i.business_id = ?`, businessID)
	pushCount("inventory_outbox", `
		SELECT COUNT(*) FROM pub_sub_message_records
		WHERE business_id = ? AND reference_type IN ('POS','PGOS','PCOS','BL','IV','CN','SC','TO','IVAQ','IVAV')`, businessID)

	for _, row := range counts {
		fmt.Printf("%s: %d\n", row.Name, row.Count)
	}
}
