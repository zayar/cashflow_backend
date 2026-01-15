package workflow

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/mmdatafocus/books_backend/config"
	"github.com/mmdatafocus/books_backend/models"
	"github.com/mmdatafocus/books_backend/utils"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm/clause"
)

func debugIVAVReadiness() bool {
	v := strings.ToLower(strings.TrimSpace(os.Getenv("DEBUG_IVAV_READINESS")))
	return v == "1" || v == "true" || v == "yes" || v == "y"
}

// EnsureIVAVPrereqLedgerReady attempts to eliminate race conditions where:
// - stock_summaries shows on-hand (cache updated),
// - but stock_histories (valuation ledger) hasn't been created yet because the outbox worker
//   hasn't processed Product Opening Stock postings.
//
// This function is intentionally scoped to the most common source of nondeterminism: Product Opening Stock.
// It is safe to call from the GraphQL mutation path before CreateInventoryAdjustment(Value).
func EnsureIVAVPrereqLedgerReady(
	ctx context.Context,
	logger *logrus.Logger,
	businessID string,
	warehouseID int,
	productID int,
	productType models.ProductType,
	batchNumber string,
	adjustmentDate time.Time,
) (didWork bool, err error) {
	if logger == nil {
		logger = config.GetLogger()
	}
	cid, _ := utils.GetCorrelationIdFromContext(ctx)

	biz, err := models.GetBusinessById(ctx, businessID)
	if err != nil {
		return false, err
	}
	asOf, err := utils.ConvertToDate(adjustmentDate, biz.Timezone)
	if err != nil {
		return false, err
	}
	asOfExclusiveEnd := asOf.AddDate(0, 0, 1)

	db := config.GetDB()
	if db == nil {
		return false, fmt.Errorf("db is nil")
	}

	// Quick check: if ledger already exists, nothing to do.
	var ledgerExists int
	if err := db.WithContext(ctx).Raw(`
SELECT 1
FROM stock_histories
WHERE business_id = ?
  AND warehouse_id = ?
  AND product_id = ?
  AND product_type = ?
  AND COALESCE(batch_number,'') = ?
  AND stock_date < ?
  AND is_reversal = 0
  AND reversed_by_stock_history_id IS NULL
LIMIT 1
`, businessID, warehouseID, productID, productType, batchNumber, asOfExclusiveEnd).Scan(&ledgerExists).Error; err != nil {
		return false, err
	}
	if ledgerExists == 1 {
		return false, nil
	}

	// If cache doesn't show on-hand, do not try to "fix" anything.
	var cacheQty string
	_ = db.WithContext(ctx).Raw(`
SELECT COALESCE(SUM(current_qty), 0)
FROM stock_summaries
WHERE business_id = ?
  AND warehouse_id = ?
  AND product_id = ?
  AND product_type = ?
  AND COALESCE(batch_number,'') = ?
`, businessID, warehouseID, productID, productType, batchNumber).Scan(&cacheQty).Error

	if debugIVAVReadiness() {
		logger.WithFields(logrus.Fields{
			"correlation_id": cid,
			"business_id":    businessID,
			"warehouse_id":   warehouseID,
			"product_id":     productID,
			"product_type":   string(productType),
			"batch_number":   batchNumber,
			"as_of":          asOf.Format(time.RFC3339),
			"cache_qty":      cacheQty,
		}).Info("ivav_readiness: ledger missing; checking for pending opening stock outbox")
	}

	// Attempt to process pending Product Opening Stock outbox for this product (common cause of cache/ledger divergence).
	tx := db.WithContext(ctx).Begin()
	defer func() {
		// In case caller forgets; commit/rollback is handled below, but be defensive.
		if r := recover(); r != nil {
			_ = tx.Rollback().Error
			panic(r)
		}
	}()

	// Serialize within a business to avoid concurrent processing racing the background worker.
	if err := utils.BusinessLock(ctx, businessID, "stockLock", "ivavReadiness.go", "EnsureIVAVPrereqLedgerReady"); err != nil {
		_ = tx.Rollback().Error
		return false, err
	}

	var rec models.PubSubMessageRecord
	// Lock the row so a background worker cannot process it concurrently in another transaction.
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("business_id = ? AND reference_type = ? AND reference_id = ? AND action = ? AND is_processed = 0",
			businessID, models.AccountReferenceTypeProductOpeningStock, productID, models.PubSubMessageActionCreate).
		Order("id DESC").
		First(&rec).Error; err != nil {
		// Nothing to process: the ledger is missing for some OTHER reason; leave it to validation to return deterministic error.
		_ = tx.Rollback().Error
		if debugIVAVReadiness() {
			logger.WithFields(logrus.Fields{
				"correlation_id": cid,
				"business_id":    businessID,
				"warehouse_id":   warehouseID,
				"product_id":     productID,
				"product_type":   string(productType),
				"batch_number":   batchNumber,
				"as_of":          asOf.Format(time.RFC3339),
			}).WithError(err).Info("ivav_readiness: no pending opening stock outbox to process")
		}
		return false, nil
	}

	if debugIVAVReadiness() {
		logger.WithFields(logrus.Fields{
			"correlation_id": cid,
			"business_id":    businessID,
			"reference_type": rec.ReferenceType,
			"reference_id":   rec.ReferenceId,
			"message_record": rec.ID,
		}).Info("ivav_readiness: processing opening stock outbox synchronously")
	}

	if err := ProcessProductOpeningStockWorkflow(tx, logger, models.ConvertToPubSubMessage(rec)); err != nil {
		_ = tx.Rollback().Error
		return false, err
	}
	if err := tx.Commit().Error; err != nil {
		return false, err
	}

	return true, nil
}

