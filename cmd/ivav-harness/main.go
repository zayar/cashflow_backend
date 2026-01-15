package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/mmdatafocus/books_backend/config"
	"github.com/mmdatafocus/books_backend/models"
	"github.com/mmdatafocus/books_backend/utils"
	"github.com/mmdatafocus/books_backend/workflow"
	"github.com/shopspring/decimal"
	"github.com/sirupsen/logrus"
)

// ivav-harness is a reproducibility harness for nondeterministic IVAV readiness bugs.
//
// Example:
//   go run ./cmd/ivav-harness \
//     --business_id=... --user_id=1 --warehouse_id=1 --branch_id=1 \
//     --product_id=123 --product_type=S --unit_cost=100 \
//     --date=2026-01-15T12:00:00Z --attempts=20
func main() {
	var (
		businessID  = flag.String("business_id", "", "business_id (required)")
		userID      = flag.Int("user_id", 1, "user_id")
		username    = flag.String("username", "ivav-harness", "username")
		warehouseID = flag.Int("warehouse_id", 0, "warehouse_id (required)")
		branchID    = flag.Int("branch_id", 0, "branch_id (required)")
		reasonID    = flag.Int("reason_id", 0, "reason_id (required)")
		accountID   = flag.Int("account_id", 0, "account_id (required) - typically COGS for IVAV")

		productID   = flag.Int("product_id", 0, "product_id (required)")
		productType = flag.String("product_type", "S", "product_type (S/V/...)")
		batch       = flag.String("batch_number", "", "batch_number (optional)")

		unitCost  = flag.Float64("unit_cost", 0, "NEW unit cost for IVAV (required)")
		dateStr   = flag.String("date", time.Now().UTC().Format(time.RFC3339), "adjustment date (RFC3339)")
		attempts  = flag.Int("attempts", 20, "attempt count")
		sleepMS   = flag.Int("sleep_ms", 0, "sleep between attempts (ms)")
		doEnsure  = flag.Bool("ensure", true, "call EnsureIVAVPrereqLedgerReady before each attempt")
		doCreate  = flag.Bool("create", false, "actually create an IVAV document (default: validate only)")
	)
	flag.Parse()

	if *businessID == "" || *warehouseID == 0 || *branchID == 0 || *reasonID == 0 || *accountID == 0 || *productID == 0 {
		fmt.Fprintln(os.Stderr, "missing required flags")
		flag.Usage()
		os.Exit(2)
	}

	adjDate, err := time.Parse(time.RFC3339, *dateStr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "invalid --date: %v\n", err)
		os.Exit(2)
	}

	// Connect to DB/Redis using env config (same as server).
	config.ConnectDatabaseWithRetry()
	config.ConnectRedisWithRetry()

	logger := logrus.New()

	baseCtx := context.Background()
	baseCtx = utils.SetBusinessIdInContext(baseCtx, *businessID)
	baseCtx = utils.SetUserIdInContext(baseCtx, *userID)
	baseCtx = utils.SetUserNameInContext(baseCtx, *username)
	baseCtx = utils.SetUsernameInContext(baseCtx, *username)

	success := 0
	fail := 0
	for i := 1; i <= *attempts; i++ {
		cid := fmt.Sprintf("ivav-harness-%02d-%d", i, time.Now().UnixNano())
		ctx := utils.SetCorrelationIdInContext(baseCtx, cid)

		if *doEnsure {
			_, _ = workflow.EnsureIVAVPrereqLedgerReady(ctx, logger, *businessID, *warehouseID, *productID, models.ProductType(*productType), *batch, adjDate)
		}

		input := &models.NewInventoryAdjustment{
			AdjustmentType: models.InventoryAdjustmentTypeValue,
			AdjustmentDate: adjDate,
			AccountId:      *accountID,
			BranchId:       *branchID,
			WarehouseId:    *warehouseID,
			ReasonId:       *reasonID,
			CurrentStatus:  models.InventoryAdjustmentStatusDraft,
			Details: []models.NewInventoryAdjustmentDetail{
				{
					ProductId:     *productID,
					ProductType:   models.ProductType(*productType),
					BatchNumber:   *batch,
					Name:          fmt.Sprintf("productId=%d", *productID),
					AdjustedValue: decimal.NewFromFloat(*unitCost),
					CostPrice:     decimal.NewFromFloat(*unitCost),
				},
			},
		}

		var attemptErr error
		if *doCreate {
			_, attemptErr = models.CreateInventoryAdjustment(ctx, input)
		} else {
			attemptErr = models.ValidateInventoryAdjustmentInput(ctx, input)
		}

		if attemptErr != nil {
			fail++
			fmt.Printf("%02d cid=%s FAIL: %s\n", i, cid, attemptErr.Error())
		} else {
			success++
			fmt.Printf("%02d cid=%s OK\n", i, cid)
		}

		if *sleepMS > 0 {
			time.Sleep(time.Duration(*sleepMS) * time.Millisecond)
		}
	}

	fmt.Printf("\nRESULT: ok=%d fail=%d attempts=%d\n", success, fail, *attempts)
}

