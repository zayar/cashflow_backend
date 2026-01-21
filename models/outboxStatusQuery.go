package models

import (
	"context"
	"errors"

	"github.com/mmdatafocus/books_backend/config"
	"github.com/mmdatafocus/books_backend/utils"
)

func GetOutboxStatus(ctx context.Context, referenceType AccountReferenceType, referenceId int) (*OutboxStatus, error) {
	businessId, ok := utils.GetBusinessIdFromContext(ctx)
	if !ok || businessId == "" {
		return nil, errors.New("business id is required")
	}

	db := config.GetDB()
	var rec PubSubMessageRecord
	if err := db.WithContext(ctx).
		Where("business_id = ? AND reference_type = ? AND reference_id = ?", businessId, referenceType, referenceId).
		Order("id DESC").
		First(&rec).Error; err != nil {
		return nil, err
	}

	processing := rec.ProcessingStatus
	if processing == "" {
		if rec.IsProcessed {
			processing = OutboxProcessStatusSucceeded
		} else {
			processing = OutboxProcessStatusPending
		}
	}

	var postingStatus OutboxPostingStatus
	switch processing {
	case OutboxProcessStatusProcessing:
		postingStatus = OutboxPostingStatusProcessing
	case OutboxProcessStatusFailed:
		postingStatus = OutboxPostingStatusFailed
	case OutboxProcessStatusDead:
		postingStatus = OutboxPostingStatusDead
	case OutboxProcessStatusSucceeded:
		postingStatus = OutboxPostingStatusSucceeded
	default:
		// If the row is already processed, always show SUCCEEDED (even if older rows have legacy values).
		if rec.IsProcessed {
			postingStatus = OutboxPostingStatusSucceeded
		} else {
			postingStatus = OutboxPostingStatusPending
		}
	}

	return &OutboxStatus{
		RecordId:             rec.ID,
		ReferenceType:        rec.ReferenceType,
		ReferenceId:          rec.ReferenceId,
		PublishStatus:        rec.PublishStatus,
		ProcessingStatus:     postingStatus,
		IsProcessed:          rec.IsProcessed,
		PublishAttempts:      rec.PublishAttempts,
		ProcessAttempts:      rec.ProcessAttempts,
		NextAttemptAt:        rec.NextAttemptAt,
		NextProcessAttemptAt: rec.NextProcessAttemptAt,
		LastPublishError:     rec.LastPublishError,
		LastProcessError:     rec.LastProcessError,
		CreatedAt:            rec.CreatedAt,
		PublishedAt:          rec.PublishedAt,
		ProcessedAt:          rec.ProcessedAt,
	}, nil
}

