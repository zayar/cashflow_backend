package workflow

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/mmdatafocus/books_backend/config"
	"github.com/mmdatafocus/books_backend/models"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type OutboxDispatcher struct {
	DB           *gorm.DB
	Logger       *logrus.Logger
	DispatcherID string

	BatchSize      int
	PollInterval   time.Duration
	LockTimeout    time.Duration
	MaxAttempts    int
	InitialBackoff time.Duration
}

func NewOutboxDispatcher(db *gorm.DB, logger *logrus.Logger) *OutboxDispatcher {
	return &OutboxDispatcher{
		DB:             db,
		Logger:         logger,
		DispatcherID:   uuid.NewString(),
		BatchSize:      50,
		PollInterval:   500 * time.Millisecond,
		LockTimeout:    30 * time.Second,
		MaxAttempts:    20,
		InitialBackoff: 5 * time.Second,
	}
}

func (d *OutboxDispatcher) Run(ctx context.Context) {
	if ctx == nil {
		ctx = context.Background()
	}
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}
		d.dispatchOnce(ctx)
		select {
		case <-ctx.Done():
			return
		case <-time.After(d.PollInterval):
		}
	}
}

func (d *OutboxDispatcher) dispatchOnce(ctx context.Context) {
	now := time.Now().UTC()
	staleBefore := now.Add(-d.LockTimeout)
	db := d.DB
	if db == nil {
		return
	}

	var claimed []models.PubSubMessageRecord
	err := db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Eligible:
		// - PENDING / FAILED and ready to retry
		// - PROCESSING but lock is stale (dispatcher crashed mid-batch), reclaim after LockTimeout
		q := tx.
			Where("is_processed = 0").
			Where(`
				(
					publish_status IN ? AND (next_attempt_at IS NULL OR next_attempt_at <= ?)
				)
				OR
				(
					publish_status = ? AND locked_at IS NOT NULL AND locked_at <= ?
				)
			`, []string{models.OutboxPublishStatusPending, models.OutboxPublishStatusFailed}, now, models.OutboxPublishStatusProcessing, staleBefore).
			Order("id ASC").
			Limit(d.BatchSize).
			Clauses(clause.Locking{Strength: "UPDATE", Options: "SKIP LOCKED"})
		if err := q.Find(&claimed).Error; err != nil {
			return err
		}
		if len(claimed) == 0 {
			return nil
		}
		for i := range claimed {
			// Enforce max attempts: poison messages go terminal (DLQ equivalent).
			if d.MaxAttempts > 0 && claimed[i].PublishAttempts >= d.MaxAttempts {
				msg := fmt.Sprintf("max publish attempts exceeded (%d)", d.MaxAttempts)
				claimed[i].PublishStatus = models.OutboxPublishStatusDead
				if err := tx.Model(&models.PubSubMessageRecord{}).Where("id = ?", claimed[i].ID).Updates(map[string]interface{}{
					"publish_status":     models.OutboxPublishStatusDead,
					"last_publish_error": &msg,
					"next_attempt_at":    nil,
					"locked_at":          nil,
					"locked_by":          nil,
				}).Error; err != nil {
					return err
				}
				continue
			}

			// Claim for publishing.
			claimed[i].PublishStatus = models.OutboxPublishStatusProcessing
			claimed[i].LockedAt = &now
			claimed[i].LockedBy = &d.DispatcherID
			claimed[i].PublishAttempts = claimed[i].PublishAttempts + 1
			claimed[i].LastPublishError = nil
			if err := tx.Model(&models.PubSubMessageRecord{}).Where("id = ?", claimed[i].ID).Updates(map[string]interface{}{
				"publish_status":     claimed[i].PublishStatus,
				"locked_at":          claimed[i].LockedAt,
				"locked_by":          claimed[i].LockedBy,
				"publish_attempts":   gorm.Expr("publish_attempts + 1"),
				"last_publish_error": nil,
				"next_attempt_at":    nil,
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
		// Skip terminal rows that were marked DEAD in the claim transaction.
		if rec.PublishStatus == models.OutboxPublishStatusDead {
			continue
		}
		msg := models.ConvertToPubSubMessage(rec)
		pubID, pubErr := config.PublishAccountingWorkflowWithResult(ctx, rec.BusinessId, msg)
		if pubErr != nil {
			d.markPublishFailed(ctx, rec.ID, rec.BusinessId, pubErr, rec.PublishAttempts)
			continue
		}
		d.markPublishSent(ctx, rec.ID, rec.BusinessId, pubID, now)
	}
}

func (d *OutboxDispatcher) markPublishSent(ctx context.Context, recordID int, businessID string, pubsubMsgID string, now time.Time) {
	db := d.DB.WithContext(ctx)
	id := pubsubMsgID
	_ = db.Model(&models.PubSubMessageRecord{}).
		Where("id = ?", recordID).
		Updates(map[string]interface{}{
			"publish_status":     models.OutboxPublishStatusSent,
			"published_at":       &now,
			"pub_sub_message_id": &id,
			"locked_at":          nil,
			"locked_by":          nil,
			"next_attempt_at":    nil,
		}).Error
}

func (d *OutboxDispatcher) markPublishFailed(ctx context.Context, recordID int, businessID string, err error, attempt int) {
	db := d.DB.WithContext(ctx)
	now := time.Now().UTC()
	msg := err.Error()

	// Terminal after MaxAttempts (DLQ equivalent).
	if d.MaxAttempts > 0 && attempt >= d.MaxAttempts {
		_ = db.Model(&models.PubSubMessageRecord{}).
			Where("id = ?", recordID).
			Updates(map[string]interface{}{
				"publish_status":     models.OutboxPublishStatusDead,
				"last_publish_error": &msg,
				"next_attempt_at":    nil,
				"locked_at":          nil,
				"locked_by":          nil,
			}).Error

		if d.Logger != nil {
			d.Logger.WithFields(logrus.Fields{
				"field":       "OutboxDispatcher",
				"business_id": businessID,
				"record_id":   recordID,
				"attempt":     attempt,
			}).Error("outbox publish moved to DEAD after max attempts: " + fmt.Sprintf("%v", err))
		}
		return
	}

	backoff := d.InitialBackoff
	for i := 1; i < attempt; i++ {
		backoff *= 2
		if backoff > time.Minute*10 {
			backoff = time.Minute * 10
			break
		}
	}
	next := now.Add(backoff)
	_ = db.Model(&models.PubSubMessageRecord{}).
		Where("id = ?", recordID).
		Updates(map[string]interface{}{
			"publish_status":     models.OutboxPublishStatusFailed,
			"last_publish_error": &msg,
			"next_attempt_at":    &next,
			"locked_at":          nil,
			"locked_by":          nil,
		}).Error

	if d.Logger != nil {
		d.Logger.WithFields(logrus.Fields{
			"field":           "OutboxDispatcher",
			"business_id":     businessID,
			"record_id":       recordID,
			"attempt":         attempt,
			"next_attempt_at": next.Format(time.RFC3339Nano),
		}).Error("outbox publish failed: " + fmt.Sprintf("%v", err))
	}
}
