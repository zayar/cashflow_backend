package models

import (
	"context"
	"errors"
	"time"

	"github.com/mmdatafocus/books_backend/config"
	"github.com/mmdatafocus/books_backend/utils"
	"gorm.io/gorm"
)

func ReprocessOutbox(ctx context.Context, referenceType AccountReferenceType, referenceId int) (*OutboxStatus, error) {
	businessId, ok := utils.GetBusinessIdFromContext(ctx)
	if !ok || businessId == "" {
		return nil, errors.New("business id is required")
	}

	now := time.Now().UTC()
	db := config.GetDB()

	res := db.WithContext(ctx).
		Model(&PubSubMessageRecord{}).
		Where("business_id = ? AND reference_type = ? AND reference_id = ? AND is_processed = 0", businessId, referenceType, referenceId).
		Updates(map[string]interface{}{
			"locked_at":               nil,
			"locked_by":               nil,
			"publish_status":          OutboxPublishStatusPending,
			"next_attempt_at":         nil,
			"processing_status":       OutboxProcessStatusPending,
			"next_process_attempt_at": &now,
			"last_process_error":      nil,
		})
	if res.Error != nil {
		return nil, res.Error
	}
	if res.RowsAffected == 0 {
		return nil, gorm.ErrRecordNotFound
	}

	return GetOutboxStatus(ctx, referenceType, referenceId)
}

