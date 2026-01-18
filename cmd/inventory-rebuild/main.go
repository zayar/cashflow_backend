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

func main() {
	businessID := flag.String("business-id", "", "Required: business id (uuid)")
	productID := flag.Int("product-id", 0, "Optional: product id")
	productType := flag.String("product-type", "S", "Optional: product type (S/V)")
	warehouseID := flag.Int("warehouse-id", 0, "Optional: warehouse id")
	batchNumber := flag.String("batch", "", "Optional: batch number (default empty)")
	fromDateStr := flag.String("from", "", "Optional: rebuild from date (YYYY-MM-DD). Defaults to earliest ledger date for the key.")
	continueOnError := flag.Bool("continue-on-error", false, "Skip failing keys and continue rebuilding others")
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

	var scopes []struct {
		WarehouseId int
		ProductId   int
		ProductType models.ProductType
		Batch       string
		StartDate   time.Time
	}

	if *productID > 0 && *warehouseID > 0 {
		start := time.Now().UTC()
		if strings.TrimSpace(*fromDateStr) != "" {
			d, err := time.Parse("2006-01-02", strings.TrimSpace(*fromDateStr))
			if err != nil {
				fmt.Fprintf(os.Stderr, "invalid from date: %v\n", err)
				os.Exit(1)
			}
			start = d
		} else {
			// Earliest ledger date for the key
			db.Raw(`
				SELECT COALESCE(MIN(stock_date), NOW()) AS start_date
				FROM stock_histories
				WHERE business_id = ? AND warehouse_id = ? AND product_id = ? AND product_type = ? AND COALESCE(batch_number,'') = ?
			`, *businessID, *warehouseID, *productID, models.ProductType(*productType), *batchNumber).Scan(&start)
		}
		scopes = append(scopes, struct {
			WarehouseId int
			ProductId   int
			ProductType models.ProductType
			Batch       string
			StartDate   time.Time
		}{*warehouseID, *productID, models.ProductType(*productType), strings.TrimSpace(*batchNumber), start})
	} else {
		// Discover all keys for the business.
		type row struct {
			WarehouseId int
			ProductId   int
			ProductType models.ProductType
			Batch       string
			StartDate   time.Time
		}
		var rows []row
		if err := db.Raw(`
			SELECT warehouse_id, product_id, product_type, COALESCE(batch_number,'') AS batch, MIN(stock_date) AS start_date
			FROM stock_histories
			WHERE business_id = ?
			GROUP BY warehouse_id, product_id, product_type, COALESCE(batch_number,'')
		`, *businessID).Scan(&rows).Error; err != nil {
			fmt.Fprintf(os.Stderr, "discover scopes: %v\n", err)
			os.Exit(1)
		}
		for _, r := range rows {
			scopes = append(scopes, struct {
				WarehouseId int
				ProductId   int
				ProductType models.ProductType
				Batch       string
				StartDate   time.Time
			}{r.WarehouseId, r.ProductId, r.ProductType, r.Batch, r.StartDate})
		}
	}

	for _, s := range scopes {
		fmt.Printf("Rebuilding business=%s warehouse=%d product=%d type=%s batch=%q from=%s\n",
			*businessID, s.WarehouseId, s.ProductId, string(s.ProductType), s.Batch, s.StartDate.Format(time.RFC3339))
		if err := db.Transaction(func(tx *gorm.DB) error {
			_, err := workflow.RebuildInventoryForItemWarehouseFromDate(
				tx, logger, *businessID, s.WarehouseId, s.ProductId, s.ProductType, s.Batch, s.StartDate,
			)
			if err != nil {
				return err
			}
			return nil
		}); err != nil {
			if *continueOnError {
				fmt.Fprintf(os.Stderr, "rebuild failed (skipping): %v\n", err)
				continue
			}
			fmt.Fprintf(os.Stderr, "rebuild failed: %v\n", err)
			os.Exit(1)
		}
	}

	fmt.Println("inventory rebuild complete")
}
