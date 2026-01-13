package models

import "time"

type IdempotencyStatus string

const (
	IdempotencyStatusStarted   IdempotencyStatus = "STARTED"
	IdempotencyStatusSucceeded IdempotencyStatus = "SUCCEEDED"
	IdempotencyStatusFailed    IdempotencyStatus = "FAILED"
)

// IdempotencyKey provides durable, DB-backed idempotency for worker handlers.
// Unique constraint: (business_id, handler_name, message_id).
type IdempotencyKey struct {
	ID          int              `gorm:"primary_key" json:"id"`
	BusinessId  string           `gorm:"size:64;not null;index:uniq_idem,unique" json:"business_id"`
	HandlerName string           `gorm:"size:100;not null;index:uniq_idem,unique" json:"handler_name"`
	MessageId   string           `gorm:"size:255;not null;index:uniq_idem,unique" json:"message_id"`
	Status      IdempotencyStatus `gorm:"size:20;not null;index" json:"status"`
	LastError   *string          `gorm:"type:text" json:"last_error"`
	CreatedAt   time.Time        `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt   time.Time        `gorm:"autoUpdateTime" json:"updated_at"`
}

