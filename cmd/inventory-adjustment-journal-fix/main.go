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
	continueOnError := flag.Bool("continue-on-error", true, "Continue when a record fails")
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

	// Load business base currency for UpdateBalances.
	biz, err := models.GetBusinessById2(db, strings.TrimSpace(*businessID))
	if err != nil {
		fmt.Fprintf(os.Stderr, "business not found: %v\n", err)
		os.Exit(1)
	}

	// Iterate adjusted inventory adjustments.
	var adjs []models.InventoryAdjustment
	if err := db.
		Where("business_id = ? AND current_status = ?", strings.TrimSpace(*businessID), models.InventoryAdjustmentStatusAdjusted).
		Order("adjustment_date ASC, id ASC").
		Find(&adjs).Error; err != nil {
		fmt.Fprintf(os.Stderr, "query adjustments: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Found %d adjusted inventory adjustments for business %s\n", len(adjs), strings.TrimSpace(*businessID))

	for _, adj := range adjs {
		fmt.Printf("Fixing adjustment id=%d type=%s date=%s\n", adj.ID, adj.AdjustmentType, adj.AdjustmentDate.UTC().Format("2006-01-02"))

		if err := db.Transaction(func(tx *gorm.DB) error {
			accountIds, branchId, _, err := workflow.RebuildInventoryAdjustmentJournalFromLedger(
				tx, logger, strings.TrimSpace(*businessID), adj.ID,
			)
			if err != nil {
				return err
			}
			if len(accountIds) == 0 {
				return nil
			}
			// Recompute balances for impacted accounts.
			return workflow.UpdateBalances(
				tx,
				logger,
				strings.TrimSpace(*businessID),
				biz.BaseCurrencyId,
				branchId,
				accountIds,
				// txTime is a MyDateString but UpdateBalances expects time.Time
				// use adjustmentDate from adj for consistency
				adj.AdjustmentDate,
				biz.BaseCurrencyId,
			)
		}); err != nil {
			if *continueOnError {
				fmt.Fprintf(os.Stderr, "  failed (skipping): %v\n", err)
				continue
			}
			fmt.Fprintf(os.Stderr, "  failed: %v\n", err)
			os.Exit(1)
		}
	}

	fmt.Println("inventory adjustment journal fix complete")
}

