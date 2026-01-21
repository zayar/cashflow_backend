package main

import (
	"context"
	"math"
	"os"
	"strconv"
	"time"

	"github.com/mmdatafocus/books_backend/config"
	"github.com/mmdatafocus/books_backend/models"
	"github.com/sirupsen/logrus"
)

type outboxProcessRetryConfig struct {
	maxAttempts int
	baseBackoff time.Duration
	maxBackoff  time.Duration
}

func getOutboxProcessRetryConfig() outboxProcessRetryConfig {
	cfg := outboxProcessRetryConfig{
		maxAttempts: 10,
		baseBackoff: 5 * time.Second,
		maxBackoff:  10 * time.Minute,
	}

	if v := os.Getenv("OUTBOX_PROCESS_MAX_ATTEMPTS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			cfg.maxAttempts = n
		}
	}
	if v := os.Getenv("OUTBOX_PROCESS_BASE_BACKOFF_SECONDS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			cfg.baseBackoff = time.Duration(n) * time.Second
		}
	}
	if v := os.Getenv("OUTBOX_PROCESS_MAX_BACKOFF_SECONDS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			cfg.maxBackoff = time.Duration(n) * time.Second
		}
	}

	return cfg
}

func outboxProcessBackoff(attempt int, cfg outboxProcessRetryConfig) time.Duration {
	if attempt <= 0 {
		return cfg.baseBackoff
	}
	// base * 2^(attempt-1), capped.
	exp := float64(attempt - 1)
	delay := time.Duration(float64(cfg.baseBackoff) * math.Pow(2, exp))
	if delay > cfg.maxBackoff {
		return cfg.maxBackoff
	}
	return delay
}

func markOutboxProcessing(ctx context.Context, id int) {
	if id <= 0 {
		return
	}
	db := config.GetDB()
	_ = db.WithContext(ctx).
		Model(&models.PubSubMessageRecord{}).
		Where("id = ? AND processing_status <> ?", id, models.OutboxProcessStatusDead).
		Updates(map[string]interface{}{
			"processing_status": models.OutboxProcessStatusProcessing,
		}).Error
}

// markOutboxProcessFailure returns whether the record is now DEAD.
func markOutboxProcessFailure(ctx context.Context, logger *logrus.Logger, m config.PubSubMessage, err error) bool {
	if m.ID <= 0 {
		return false
	}

	cfg := getOutboxProcessRetryConfig()
	now := time.Now().UTC()
	errMsg := ""
	if err != nil {
		errMsg = err.Error()
	}

	db := config.GetDB()

	// Fetch current attempts for stable backoff and DEAD cutoff.
	var rec models.PubSubMessageRecord
	if qerr := db.WithContext(ctx).
		Select("id,business_id,reference_type,reference_id,process_attempts").
		Where("id = ?", m.ID).
		First(&rec).Error; qerr != nil {
		// Still try to record the error even if we can't read attempts.
		_ = db.WithContext(ctx).Model(&models.PubSubMessageRecord{}).
			Where("id = ?", m.ID).
			Updates(map[string]interface{}{
				"last_process_error": &errMsg,
				"locked_at":          nil,
				"locked_by":          nil,
				"processing_status":  models.OutboxProcessStatusFailed,
			}).Error
		return false
	}

	attempts := rec.ProcessAttempts + 1
	status := models.OutboxProcessStatusFailed

	var nextAttemptAt *time.Time
	if attempts >= cfg.maxAttempts {
		status = models.OutboxProcessStatusDead
		nextAttemptAt = nil
	} else {
		t := now.Add(outboxProcessBackoff(attempts, cfg))
		nextAttemptAt = &t
	}

	_ = db.WithContext(ctx).Model(&models.PubSubMessageRecord{}).
		Where("id = ?", m.ID).
		Updates(map[string]interface{}{
			"last_process_error":      &errMsg,
			"process_attempts":        attempts,
			"next_process_attempt_at": nextAttemptAt,
			"processing_status":       status,
			"locked_at":               nil,
			"locked_by":               nil,
		}).Error

	if logger != nil {
		logger.WithFields(logrus.Fields{
			"field":             "OutboxProcessing",
			"business_id":       rec.BusinessId,
			"reference_type":    rec.ReferenceType,
			"reference_id":      rec.ReferenceId,
			"record_id":         rec.ID,
			"processing_status": status,
			"process_attempts":  attempts,
		}).Error("outbox processing failed: " + errMsg)
	}

	return status == models.OutboxProcessStatusDead
}

func markOutboxProcessSuccess(ctx context.Context, logger *logrus.Logger, m config.PubSubMessage) {
	if m.ID <= 0 {
		return
	}
	now := time.Now().UTC()
	db := config.GetDB()

	// Do not override terminal DEAD rows (e.g. posting gate drop).
	_ = db.WithContext(ctx).Model(&models.PubSubMessageRecord{}).
		Where("id = ? AND processing_status <> ?", m.ID, models.OutboxProcessStatusDead).
		Updates(map[string]interface{}{
			"processing_status":       models.OutboxProcessStatusSucceeded,
			"processed_at":            &now,
			"next_process_attempt_at": nil,
			"last_process_error":      nil,
			"locked_at":               nil,
			"locked_by":               nil,
		}).Error

	if logger != nil {
		logger.WithFields(logrus.Fields{
			"field":             "OutboxProcessing",
			"business_id":       m.BusinessId,
			"reference_type":    m.ReferenceType,
			"reference_id":      m.ReferenceId,
			"record_id":         m.ID,
			"processing_status": models.OutboxProcessStatusSucceeded,
		}).Info("outbox processed successfully")
	}
}

