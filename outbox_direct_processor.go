package main

import (
	"context"
	"os"
	"strings"
	"time"

	"github.com/mmdatafocus/books_backend/models"
	"github.com/mmdatafocus/books_backend/utils"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// OutboxDirectProcessor processes unhandled outbox records without Pub/Sub.
// This is intended for local/dev environments where Pub/Sub is not configured.
type OutboxDirectProcessor struct {
	DB         *gorm.DB
	Logger     *logrus.Logger
	WorkerID   string
	BatchSize  int
	Interval   time.Duration
	LockTTL    time.Duration
	Processing string
}

func NewOutboxDirectProcessor(db *gorm.DB, logger *logrus.Logger) *OutboxDirectProcessor {
	return &OutboxDirectProcessor{
		DB:        db,
		Logger:    logger,
		WorkerID:  "direct-" + time.Now().Format("20060102-150405.000"),
		BatchSize: 50,
		Interval:  2 * time.Second,
		LockTTL:   30 * time.Second,
	}
}

func shouldRunDirectOutboxProcessor() bool {
	val := strings.ToLower(strings.TrimSpace(os.Getenv("OUTBOX_DIRECT_PROCESSING")))
	if val == "true" {
		return true
	}
	if val == "false" {
		return false
	}
	// Default: run as a safety-net even when Pub/Sub is configured.
	//
	// Why:
	// - In some environments Pub/Sub settings may exist but delivery/permissions can be misconfigured,
	//   leaving outbox rows stuck in PENDING/FAILED/SENT without journals ever being created.
	// - Running the direct processor as a background "backup worker" ensures journals are eventually created.
	// - Processing is protected by DB idempotency keys + ledger immutability, so at-least-once delivery is safe.
	//
	// To disable in production, explicitly set OUTBOX_DIRECT_PROCESSING=false.
	return true
}

func (p *OutboxDirectProcessor) Run(ctx context.Context) {
	if p == nil || p.DB == nil {
		return
	}
	if ctx == nil {
		ctx = context.Background()
	}
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}
		p.processOnce(ctx)
		select {
		case <-ctx.Done():
			return
		case <-time.After(p.Interval):
		}
	}
}

func (p *OutboxDirectProcessor) processOnce(ctx context.Context) {
	now := time.Now().UTC()
	staleBefore := now.Add(-p.LockTTL)

	var claimed []models.PubSubMessageRecord
	err := p.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		q := tx.
			Where("is_processed = 0").
			Where("(locked_at IS NULL OR locked_at <= ?)", staleBefore).
			Order("id ASC").
			Limit(p.BatchSize).
			Clauses(clause.Locking{Strength: "UPDATE", Options: "SKIP LOCKED"})
		if err := q.Find(&claimed).Error; err != nil {
			return err
		}
		if len(claimed) == 0 {
			return nil
		}
		for i := range claimed {
			claimed[i].LockedAt = &now
			claimed[i].LockedBy = &p.WorkerID
			if err := tx.Model(&models.PubSubMessageRecord{}).
				Where("id = ?", claimed[i].ID).
				Updates(map[string]interface{}{
					"locked_at": claimed[i].LockedAt,
					"locked_by": claimed[i].LockedBy,
				}).Error; err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil || len(claimed) == 0 {
		return
	}

	for _, rec := range claimed {
		msg := models.ConvertToPubSubMessage(rec)
		procCtx := context.WithValue(ctx, utils.ContextKeyBusinessId, rec.BusinessId)
		procCtx = context.WithValue(procCtx, utils.ContextKeyUserId, 0)
		procCtx = context.WithValue(procCtx, utils.ContextKeyUserName, "System")
		procCtx = utils.SetCorrelationIdInContext(procCtx, rec.CorrelationId)

		if err := ProcessMessage(procCtx, p.Logger, msg); err != nil {
			errMsg := err.Error()
			_ = p.DB.WithContext(ctx).Model(&models.PubSubMessageRecord{}).
				Where("id = ?", rec.ID).
				Updates(map[string]interface{}{
					"last_process_error": &errMsg,
					"locked_at":          nil,
					"locked_by":          nil,
				}).Error
			if p.Logger != nil {
				p.Logger.WithFields(logrus.Fields{
					"field":          "OutboxDirectProcessor",
					"business_id":    rec.BusinessId,
					"reference_type": rec.ReferenceType,
					"reference_id":   rec.ReferenceId,
					"record_id":      rec.ID,
				}).Error("direct processing failed: " + errMsg)
			}
			continue
		}

		_ = p.DB.WithContext(ctx).Model(&models.PubSubMessageRecord{}).
			Where("id = ?", rec.ID).
			Updates(map[string]interface{}{
				"locked_at": nil,
				"locked_by": nil,
			}).Error
	}
}
