package models

// Outbox publish statuses for PubSubMessageRecord.PublishStatus.
// Keep these as strings (DB values) for backwards compatibility.
const (
	OutboxPublishStatusPending    = "PENDING"
	OutboxPublishStatusProcessing = "PROCESSING"
	OutboxPublishStatusSent       = "SENT"
	OutboxPublishStatusFailed     = "FAILED"
	OutboxPublishStatusDead       = "DEAD"
)
