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

// Outbox processing statuses for PubSubMessageRecord.ProcessingStatus.
// These represent worker-side handling state (distinct from PublishStatus).
const (
	OutboxProcessStatusPending    = "PENDING"
	OutboxProcessStatusProcessing = "PROCESSING"
	OutboxProcessStatusSucceeded  = "SUCCEEDED"
	OutboxProcessStatusFailed     = "FAILED"
	OutboxProcessStatusDead       = "DEAD"
)
