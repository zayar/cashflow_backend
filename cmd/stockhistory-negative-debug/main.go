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

// stockhistory-negative-debug prints the inventory ledger running balance for a single
// (business_id, warehouse_id, product_id, product_type) key so you can see exactly
// which row makes inventory go negative.
//
// Example:
//
//	go run ./cmd/stockhistory-negative-debug/ \
//	  -business-id=a195a02a-ee0c-4047-a6f4-443633d0aca4 \
//	  -warehouse-id=30 \
//	  -product-id=137 \
//	  -product-type=S
func main() {
	businessID := flag.String("business-id", "", "Required: business id (uuid)")
	warehouseID := flag.Int("warehouse-id", 0, "Required: warehouse id")
	productID := flag.Int("product-id", 0, "Required: product id")
	productType := flag.String("product-type", "S", "Product type enum (S,G,C,V,I)")
	showAll := flag.Bool("show-all", false, "Include reversals and reversed rows")
	limit := flag.Int("limit", 500, "Max rows to print (0 = no limit)")
	flag.Parse()

	if strings.TrimSpace(*businessID) == "" || *warehouseID <= 0 || *productID <= 0 {
		fmt.Fprintln(os.Stderr, "--business-id, --warehouse-id, and --product-id are required")
		os.Exit(1)
	}
	pt := models.ProductType(strings.TrimSpace(*productType))
	if pt == "" {
		fmt.Fprintln(os.Stderr, "--product-type is required")
		os.Exit(1)
	}

	config.ConnectDatabaseWithRetry()
	db := config.GetDB()
	if db == nil {
		fmt.Fprintln(os.Stderr, "database not initialized")
		os.Exit(1)
	}

	var whName string
	_ = db.Raw("SELECT name FROM warehouses WHERE business_id = ? AND id = ? LIMIT 1", *businessID, *warehouseID).Scan(&whName).Error
	fmt.Printf("business_id=%s warehouse_id=%d warehouse_name=%q product_id=%d product_type=%s\n", *businessID, *warehouseID, whName, *productID, string(pt))

	type row struct {
		ID          int
		StockDate   time.Time
		RefType     string
		RefID       int
		Description string
		Qty         string
		IsOutgoing  *bool
		IsReversal  bool
		ReversedBy  *int
		RunningQty  string
	}

	whereExtra := ""
	if !*showAll {
		whereExtra = " AND is_reversal = 0 AND reversed_by_stock_history_id IS NULL "
	}
	limitSQL := ""
	if *limit > 0 {
		limitSQL = fmt.Sprintf(" LIMIT %d ", *limit)
	}

	sql := fmt.Sprintf(`
SELECT
  id,
  stock_date,
  reference_type AS ref_type,
  reference_id   AS ref_id,
  description,
  qty,
  is_outgoing,
  is_reversal,
  reversed_by_stock_history_id AS reversed_by,
  SUM(qty) OVER (
    PARTITION BY warehouse_id, product_id, product_type
    ORDER BY stock_date, CASE WHEN qty < 0 THEN 1 ELSE 0 END, id
    ROWS BETWEEN UNBOUNDED PRECEDING AND CURRENT ROW
  ) AS running_qty
FROM stock_histories
WHERE business_id = ?
  AND warehouse_id = ?
  AND product_id = ?
  AND product_type = ?
%s
ORDER BY stock_date, CASE WHEN qty < 0 THEN 1 ELSE 0 END, id
%s
`, whereExtra, limitSQL)

	var rows []row
	if err := db.Raw(sql, *businessID, *warehouseID, *productID, pt).Scan(&rows).Error; err != nil {
		fmt.Fprintf(os.Stderr, "query failed: %v\n", err)
		os.Exit(1)
	}
	if len(rows) == 0 {
		fmt.Println("no rows found")
		return
	}

	fmt.Printf("rows=%d\n", len(rows))
	minFound := false
	var minID int
	var minRunning string
	for _, r := range rows {
		fmt.Printf("id=%d date=%s ref=%s/%d qty=%s out=%v running=%s desc=%q reversal=%v reversed_by=%v\n",
			r.ID,
			r.StockDate.Format("2006-01-02T15:04:05Z07:00"),
			r.RefType,
			r.RefID,
			r.Qty,
			boolPtr(r.IsOutgoing),
			r.RunningQty,
			r.Description,
			r.IsReversal,
			intPtr(r.ReversedBy),
		)
		// Track min by lexicographic compare isn't safe; we just detect first negative by string prefix.
		// If you need exact numeric compare, use the main workflow's decimal parsing.
		if !minFound && strings.HasPrefix(strings.TrimSpace(r.RunningQty), "-") {
			minFound = true
			minID = r.ID
			minRunning = r.RunningQty
		}
	}
	if minFound {
		fmt.Printf("FIRST_NEGATIVE: id=%d running_qty=%s\n", minID, minRunning)
		fmt.Println("This is the earliest row where the warehouse running balance went negative.")
		fmt.Println("Fix options are usually: reverse/delete the outgoing row, or add/move missing incoming stock before it.")
	} else {
		fmt.Println("OK: no negative running balance detected in printed rows.")
	}
}

func boolPtr(b *bool) bool {
	if b == nil {
		return false
	}
	return *b
}

func intPtr(i *int) int {
	if i == nil {
		return 0
	}
	return *i
}

// Ensure main compiles with gorm imported (used by other cmds in this repo).
var _ *gorm.DB

