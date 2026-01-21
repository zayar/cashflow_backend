package models

import "time"

// OutboxPostingStatus is the worker/processing-side status exposed via GraphQL.
// It intentionally does not include publish states like SENT.
type OutboxPostingStatus string

const (
	OutboxPostingStatusPending    OutboxPostingStatus = "PENDING"
	OutboxPostingStatusProcessing OutboxPostingStatus = "PROCESSING"
	OutboxPostingStatusFailed     OutboxPostingStatus = "FAILED"
	OutboxPostingStatusDead       OutboxPostingStatus = "DEAD"
	OutboxPostingStatusSucceeded  OutboxPostingStatus = "SUCCEEDED"
)

// OutboxStatus is a UI-facing view of the latest outbox row for a document.
type OutboxStatus struct {
	RecordId            int                  `json:"record_id"`
	ReferenceType       AccountReferenceType `json:"reference_type"`
	ReferenceId         int                  `json:"reference_id"`
	PublishStatus       string               `json:"publish_status"`
	ProcessingStatus    OutboxPostingStatus  `json:"processing_status"`
	IsProcessed         bool                 `json:"is_processed"`
	PublishAttempts     int                  `json:"publish_attempts"`
	ProcessAttempts     int                  `json:"process_attempts"`
	NextAttemptAt       *time.Time           `json:"next_attempt_at"`
	NextProcessAttemptAt *time.Time          `json:"next_process_attempt_at"`
	LastPublishError    *string              `json:"last_publish_error"`
	LastProcessError    *string              `json:"last_process_error"`
	CreatedAt           time.Time            `json:"created_at"`
	PublishedAt         *time.Time           `json:"published_at"`
	ProcessedAt         *time.Time           `json:"processed_at"`
}

