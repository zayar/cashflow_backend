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
	"gorm.io/gorm"
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
	ledgerExists, err := ledgerExistsForIVAV(ctx, db, businessID, warehouseID, productID, productType, batchNumber, asOfExclusiveEnd)
	if err != nil {
		return false, err
	}
	if ledgerExists {
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
		}).Info("ivav_readiness: ledger missing; attempting deterministic repair")
	}

	// Attempt to process pending Product Opening Stock outbox for this product (common cause of cache/ledger divergence).
	tx := db.WithContext(ctx).Begin()
	defer func() {
		if r := recover(); r != nil {
			_ = tx.Rollback().Error
			panic(r)
		}
	}()

	// Serialize per business, matching worker behavior.
	if err := AcquireBusinessPostingLock(tx, businessID); err != nil {
		_ = tx.Rollback().Error
		return false, err
	}
	defer ReleaseBusinessPostingLock(tx, businessID)

	// First try: process opening stock outbox (fast path).
	did, err := processOpeningStockOutboxIfPresent(tx.WithContext(ctx), logger, businessID, productID, cid)
	if err != nil {
		_ = tx.Rollback().Error
		return false, err
	}
	if did {
		// Re-check ledger after processing.
		ok, err := ledgerExistsForIVAV(ctx, tx, businessID, warehouseID, productID, productType, batchNumber, asOfExclusiveEnd)
		if err != nil {
			_ = tx.Rollback().Error
			return false, err
		}
		if ok {
			if err := tx.Commit().Error; err != nil {
				return false, err
			}
			return true, nil
		}
	}

	// Second try: bounded "reconcile" pass for this business to process pending outbox records.
	// This fixes nondeterminism when other stock-affecting docs updated cache first but worker hasn't posted ledger yet.
	processedAny, err := processPendingOutboxBounded(tx.WithContext(ctx), logger, businessID, cid, func(tx2 *gorm.DB) (bool, error) {
		return ledgerExistsForIVAV(ctx, tx2, businessID, warehouseID, productID, productType, batchNumber, asOfExclusiveEnd)
	})
	if err != nil {
		_ = tx.Rollback().Error
		return false, err
	}
	if err := tx.Commit().Error; err != nil {
		return false, err
	}
	return did || processedAny, nil
}

func ledgerExistsForIVAV(
	ctx context.Context,
	db *gorm.DB,
	businessID string,
	warehouseID int,
	productID int,
	productType models.ProductType,
	batchNumber string,
	asOfExclusiveEnd time.Time,
) (bool, error) {
	var exists int
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
`, businessID, warehouseID, productID, productType, batchNumber, asOfExclusiveEnd).Scan(&exists).Error; err != nil {
		return false, err
	}
	return exists == 1, nil
}

func processOpeningStockOutboxIfPresent(tx *gorm.DB, logger *logrus.Logger, businessID string, productID int, cid string) (bool, error) {
	var rec models.PubSubMessageRecord
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("business_id = ? AND reference_type = ? AND reference_id = ? AND action = ? AND is_processed = 0",
			businessID, models.AccountReferenceTypeProductOpeningStock, productID, models.PubSubMessageActionCreate).
		Order("id DESC").
		First(&rec).Error; err != nil {
		return false, nil
	}
	if debugIVAVReadiness() && logger != nil {
		logger.WithFields(logrus.Fields{
			"correlation_id": cid,
			"business_id":    businessID,
			"reference_type": rec.ReferenceType,
			"reference_id":   rec.ReferenceId,
			"message_record": rec.ID,
		}).Info("ivav_readiness: processing opening stock outbox synchronously")
	}
	if err := ProcessProductOpeningStockWorkflow(tx, logger, models.ConvertToPubSubMessage(rec)); err != nil {
		return false, err
	}
	return true, nil
}

func processPendingOutboxBounded(
	tx *gorm.DB,
	logger *logrus.Logger,
	businessID string,
	cid string,
	readyCheck func(tx *gorm.DB) (bool, error),
) (bool, error) {
	// Process up to N unprocessed outbox records, oldest first.
	const maxRecords = 50
	var records []models.PubSubMessageRecord
	if err := tx.
		Where("business_id = ? AND is_processed = 0", businessID).
		Order("id ASC").
		Limit(maxRecords).
		Find(&records).Error; err != nil {
		return false, err
	}
	if len(records) == 0 {
		return false, nil
	}
	if debugIVAVReadiness() && logger != nil {
		logger.WithFields(logrus.Fields{
			"correlation_id": cid,
			"business_id":    businessID,
			"count":          len(records),
		}).Warn("ivav_readiness: running bounded reconcile over pending outbox")
	}

	processedAny := false
	for _, record := range records {
		msg := config.PubSubMessage{
			ID:                  record.ID,
			BusinessId:          record.BusinessId,
			TransactionDateTime: record.TransactionDateTime,
			ReferenceId:         record.ReferenceId,
			ReferenceType:       string(record.ReferenceType),
			Action:              string(record.Action),
			OldObj:              record.OldObj,
			NewObj:              record.NewObj,
			CorrelationId:       record.CorrelationId,
		}

		handlerName := msg.ReferenceType
		messageID := fmt.Sprintf("%d", msg.ID)
		skip, err := BeginIdempotency(tx, businessID, handlerName, messageID)
		if err != nil {
			return processedAny, err
		}
		if skip {
			continue
		}
		// Apply posting gate (same safety as worker). If blocked, treat as processed (do not loop here).
		if msg.ReferenceType != "Reconcile" {
			if err := EnforcePostingGate(tx.Statement.Context, msg); err != nil {
				_ = MarkIdempotencySucceeded(tx, businessID, handlerName, messageID)
				continue
			}
		}

		if err := processOutboxMessage(tx, logger, msg); err != nil {
			_ = MarkIdempotencyFailed(tx, businessID, handlerName, messageID, err)
			return processedAny, err
		}
		if err := MarkIdempotencySucceeded(tx, businessID, handlerName, messageID); err != nil {
			return processedAny, err
		}
		processedAny = true

		ok, err := readyCheck(tx)
		if err != nil {
			return processedAny, err
		}
		if ok {
			return processedAny, nil
		}
	}
	return processedAny, nil
}

// processOutboxMessage is a local dispatcher equivalent to the worker's ProcessWorkflow switch.
// It is used only for bounded readiness repair and does not publish/dispatch.
func processOutboxMessage(tx *gorm.DB, logger *logrus.Logger, msg config.PubSubMessage) error {
	switch models.AccountReferenceType(msg.ReferenceType) {
	case models.AccountReferenceTypeOpeningBalance:
		return ProcessOpeningBalanceWorkflow(tx, logger, msg)
	case models.AccountReferenceTypeProductOpeningStock:
		return ProcessProductOpeningStockWorkflow(tx, logger, msg)
	case models.AccountReferenceTypeProductGroupOpeningStock:
		return ProcessProductGroupOpeningStockWorkflow(tx, logger, msg)
	case models.AccountReferenceTypeInventoryAdjustmentQuantity:
		return ProcessInventoryAdjustmentQuantityWorkflow(tx, logger, msg)
	case models.AccountReferenceTypeInventoryAdjustmentValue:
		return ProcessInventoryAdjustmentValueWorkflow(tx, logger, msg)
	case models.AccountReferenceTypeTransferOrder:
		return ProcessTransferOrderWorkflow(tx, logger, msg)
	case models.AccountReferenceTypeBill:
		return ProcessBillWorkflow(tx, logger, msg)
	case models.AccountReferenceTypeInvoice:
		return ProcessInvoiceWorkflow(tx, logger, msg)
	case models.AccountReferenceTypeInvoiceWriteOff:
		return ProcessInvoiceWriteOffWorkflow(tx, logger, msg)
	case models.AccountReferenceTypeCreditNote:
		return ProcessCreditNoteWorkflow(tx, logger, msg)
	case models.AccountReferenceTypeSupplierCredit:
		return ProcessSupplierCreditWorkflow(tx, logger, msg)
	default:
		// Many other outbox types exist; they don't affect stock ledger prerequisites for IVAV.
		// We skip them rather than failing.
		return nil
	}
}

