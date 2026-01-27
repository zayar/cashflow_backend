// inventory-adjustment-ledger-cleanup removes orphan stock_histories rows that remain
// after an Inventory Adjustment (By Quantity) was deleted in the UI but the ledger
// was never reversed (e.g. due to the DeleteInventoryAdjustment OldObj missing details
// before the fix). Such rows still show on the inventory valuation report.
//
// Usage (dry-run, list matching rows):
//
//	go run ./cmd/inventory-adjustment-ledger-cleanup \
//	  -business-id=a195a02a-ee0c-4047-a6f4-443633d0aca4 \
//	  -warehouse-name="Zoo Warehouse" \
//	  -stock-date=2026-01-07 \
//	  -qty=2
//
// If warehouse name is not found, the tool lists warehouses and recent IVAQ rows;
// then run again with -warehouse-id=N.
//
// To delete:
//
//	go run ./cmd/inventory-adjustment-ledger-cleanup \
//	  -business-id=... -warehouse-id=N -stock-date=2026-01-07 -qty=2 \
//	  -dry-run=false -confirm=DELETE
package main

import (
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/mmdatafocus/books_backend/config"
	"github.com/mmdatafocus/books_backend/models"
	"gorm.io/gorm"
)

func main() {
	businessID := flag.String("business-id", "", "Required: business id (uuid)")
	stockHistoryID := flag.Int("stock-history-id", 0, "If set, delete only this stock_histories.id (skip warehouse/date/qty filters)")
	warehouseID := flag.Int("warehouse-id", 0, "Warehouse id (use when name lookup fails or to skip name lookup)")
	warehouseName := flag.String("warehouse-name", "Zoo Warehouse", "Warehouse name (ignored if -warehouse-id or -stock-history-id is set)")
	stockDate := flag.String("stock-date", "2026-01-07", "Stock date (YYYY-MM-DD) of the orphan IVAQ row")
	qty := flag.Float64("qty", 2, "Quantity of the orphan row (use absolute value if you pass signed qty)")
	dryRun := flag.Bool("dry-run", true, "List matching rows only (no deletes)")
	confirm := flag.String("confirm", "", "Type DELETE to proceed when dry-run=false")
	flag.Parse()

	if strings.TrimSpace(*businessID) == "" {
		fmt.Fprintln(os.Stderr, "--business-id is required")
		os.Exit(1)
	}
	if !*dryRun && strings.TrimSpace(*confirm) != "DELETE" {
		fmt.Fprintln(os.Stderr, "set --confirm=DELETE to proceed when -dry-run=false")
		os.Exit(1)
	}

	config.ConnectDatabaseWithRetry()
	db := config.GetDB()
	if db == nil {
		fmt.Fprintln(os.Stderr, "database not initialized")
		os.Exit(1)
	}

	// Mode: delete by primary key (most reliable when date/qty filters miss due to timezone etc.)
	if *stockHistoryID > 0 {
		runByID(db, *businessID, *stockHistoryID, *dryRun)
		return
	}

	parsedDate, err := time.Parse("2006-01-02", *stockDate)
	if err != nil {
		fmt.Fprintf(os.Stderr, "invalid -stock-date %q: %v\n", *stockDate, err)
		os.Exit(1)
	}
	dateOnly := parsedDate.Format("2006-01-02")

	// Resolve warehouse: use -warehouse-id if set, else lookup by name
	var whID int
	if *warehouseID > 0 {
		var count int64
		if err := db.Model(&models.Warehouse{}).Where("business_id = ? AND id = ?", *businessID, *warehouseID).Count(&count).Error; err != nil || count == 0 {
			fmt.Fprintf(os.Stderr, "warehouse id %d not found for business %s\n", *warehouseID, *businessID)
			listWarehouses(db, *businessID)
			os.Exit(1)
		}
		whID = *warehouseID
	} else {
		name := strings.TrimSpace(*warehouseName)
		if err := db.Model(&models.Warehouse{}).
			Where("business_id = ? AND name = ?", *businessID, name).
			Pluck("id", &whID).Error; err != nil || whID == 0 {
			// Fallback: case-insensitive match (e.g. "zoo warehouse" vs "Zoo Warehouse")
			if err := db.Model(&models.Warehouse{}).
				Where("business_id = ? AND LOWER(TRIM(name)) = LOWER(?)", *businessID, name).
				Pluck("id", &whID).Error; err != nil || whID == 0 {
				fmt.Fprintf(os.Stderr, "warehouse %q not found for business %s\n", *warehouseName, *businessID)
				listWarehouses(db, *businessID)
				os.Exit(1)
			}
		}
	}

	if *dryRun {
		listMatching(db, *businessID, whID, dateOnly, *qty)
		return
	}

	var deleted int64
	if err := db.Transaction(func(tx *gorm.DB) error {
		res := tx.Where(
			"business_id = ? AND reference_type = ? AND warehouse_id = ? AND is_reversal = 0 AND reversed_by_stock_history_id IS NULL",
			*businessID, models.StockReferenceTypeInventoryAdjustmentQuantity, whID,
		).
			Where("DATE(stock_date) BETWEEN DATE_SUB(?, INTERVAL 2 DAY) AND DATE_ADD(?, INTERVAL 2 DAY)", dateOnly, dateOnly).
			Where("(qty = ? OR qty = ?)", *qty, -(*qty)).
			Delete(&models.StockHistory{})
		if res.Error != nil {
			return res.Error
		}
		deleted = res.RowsAffected
		return nil
	}); err != nil {
		fmt.Fprintf(os.Stderr, "delete failed: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("deleted %d stock_histories row(s)\n", deleted)
}

func runByID(db *gorm.DB, businessID string, id int, dryRun bool) {
	var sh models.StockHistory
	if err := db.Where("business_id = ? AND id = ?", businessID, id).First(&sh).Error; err != nil {
		fmt.Fprintf(os.Stderr, "stock_history id=%d not found for business %s: %v\n", id, businessID, err)
		os.Exit(1)
	}
	if sh.ReferenceType != models.StockReferenceTypeInventoryAdjustmentQuantity {
		fmt.Fprintf(os.Stderr, "stock_history id=%d has reference_type=%s (expected IVAQ)\n", id, sh.ReferenceType)
		os.Exit(1)
	}
	if dryRun {
		fmt.Printf("would delete: id=%d stock_date=%s warehouse_id=%d description=%q qty=%s base_unit_value=%s reference_id=%d\n",
			sh.ID, sh.StockDate.Format("2006-01-02"), sh.WarehouseId, sh.Description, sh.Qty.String(), sh.BaseUnitValue.String(), sh.ReferenceID)
		fmt.Println("run with -dry-run=false -confirm=DELETE to delete")
		return
	}
	res := db.Where("business_id = ? AND id = ?", businessID, id).Delete(&models.StockHistory{})
	if res.Error != nil {
		fmt.Fprintf(os.Stderr, "delete failed: %v\n", res.Error)
		os.Exit(1)
	}
	fmt.Printf("deleted stock_history id=%d\n", id)
}

func listMatching(db *gorm.DB, businessID string, warehouseID int, dateOnly string, qty float64) {
	// Try exact date first, then 3-day window (catches timezone-off-by-one)
	for _, useWindow := range []bool{false, true} {
		var rows []models.StockHistory
		q := db.Model(&models.StockHistory{}).
			Where("business_id = ? AND reference_type = ? AND warehouse_id = ?", businessID, models.StockReferenceTypeInventoryAdjustmentQuantity, warehouseID).
			Where("is_reversal = 0 AND reversed_by_stock_history_id IS NULL")
		if useWindow {
			q = q.Where("DATE(stock_date) BETWEEN DATE_SUB(?, INTERVAL 2 DAY) AND DATE_ADD(?, INTERVAL 2 DAY)", dateOnly, dateOnly)
		} else {
			q = q.Where("DATE(stock_date) = ?", dateOnly)
		}
		q = q.Where("(qty = ? OR qty = ?)", qty, -qty).Order("id")
		if err := q.Find(&rows).Error; err != nil {
			fmt.Fprintf(os.Stderr, "query failed: %v\n", err)
			os.Exit(1)
		}
		if len(rows) == 0 {
			continue
		}
		if useWindow {
			fmt.Printf("matching rows in ±2 day window (%d):\n", len(rows))
		} else {
			fmt.Printf("matching rows (%d):\n", len(rows))
		}
		for _, r := range rows {
			fmt.Printf("  id=%d stock_date=%s product_id=%d description=%q qty=%s base_unit_value=%s reference_id=%d\n",
				r.ID, r.StockDate.Format("2006-01-02"), r.ProductId, r.Description, r.Qty.String(), r.BaseUnitValue.String(), r.ReferenceID)
		}
		fmt.Println("run with -dry-run=false -confirm=DELETE to delete these rows")
		return
	}
	// No rows found: discover IVAQ rows in this warehouse in a date window so user can use -stock-history-id
	fmt.Println("no matching stock_histories rows")
	discoverIVAQRows(db, businessID, warehouseID, dateOnly)
}

func discoverIVAQRows(db *gorm.DB, businessID string, warehouseID int, dateOnly string) {
	var rows []models.StockHistory
	err := db.Model(&models.StockHistory{}).
		Where("business_id = ? AND reference_type = ? AND warehouse_id = ?", businessID, models.StockReferenceTypeInventoryAdjustmentQuantity, warehouseID).
		Where("is_reversal = 0 AND reversed_by_stock_history_id IS NULL").
		Where("DATE(stock_date) BETWEEN DATE_SUB(?, INTERVAL 3 DAY) AND DATE_ADD(?, INTERVAL 3 DAY)", dateOnly, dateOnly).
		Order("stock_date, id").
		Find(&rows).Error
	if err != nil || len(rows) == 0 {
		fmt.Fprintln(os.Stderr, "no IVAQ rows in this warehouse in ±3 day window; check -warehouse-id and -stock-date")
		return
	}
	fmt.Fprintln(os.Stderr, "IVAQ rows in this warehouse near that date (use -stock-history-id=ID to delete one):")
	for _, r := range rows {
		fmt.Fprintf(os.Stderr, "  id=%d stock_date=%s product_id=%d qty=%s base_unit_value=%s — use: -stock-history-id=%d\n",
			r.ID, r.StockDate.Format("2006-01-02"), r.ProductId, r.Qty.String(), r.BaseUnitValue.String(), r.ID)
	}
}

func listWarehouses(db *gorm.DB, businessID string) {
	var whs []struct {
		ID   int
		Name string
	}
	if err := db.Model(&models.Warehouse{}).Where("business_id = ?", businessID).Select("id", "name").Find(&whs).Error; err != nil {
		return
	}
	if len(whs) == 0 {
		fmt.Fprintln(os.Stderr, "no warehouses for this business")
		return
	}
	fmt.Fprintln(os.Stderr, "warehouses for this business (use -warehouse-id=N):")
	for _, w := range whs {
		fmt.Fprintf(os.Stderr, "  id=%d name=%q\n", w.ID, w.Name)
	}
	// Hint: show IVAQ rows for this business on common dates so they can pick warehouse_id
	var refs []struct {
		WarehouseID int
		StockDate   string
		Qty         string
	}
	_ = db.Raw(`
		SELECT warehouse_id, DATE(stock_date) AS stock_date, qty
		FROM stock_histories
		WHERE business_id = ? AND reference_type = ? AND is_reversal = 0 AND reversed_by_stock_history_id IS NULL
		ORDER BY stock_date DESC
		LIMIT 20
	`, businessID, models.StockReferenceTypeInventoryAdjustmentQuantity).Scan(&refs).Error
	if len(refs) > 0 {
		fmt.Fprintln(os.Stderr, "recent IVAQ rows (pick -warehouse-id and -stock-date from here if needed):")
		for _, r := range refs {
			fmt.Fprintf(os.Stderr, "  warehouse_id=%d stock_date=%s qty=%s\n", r.WarehouseID, r.StockDate, r.Qty)
		}
	}
}
