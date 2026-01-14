package models

import "time"

// Phase 0 drift detection output (nightly/admin-triggered).
type ReconciliationReport struct {
	ID            int       `gorm:"primary_key" json:"id"`
	BusinessId    string    `gorm:"index;not null" json:"business_id"`
	CheckType     string    `gorm:"size:50;index;not null" json:"check_type"`  // e.g. INVOICE_JOURNAL, STOCK_SUMMARY
	EntityType    string    `gorm:"size:50;index;not null" json:"entity_type"` // e.g. SalesInvoice, StockSummary
	EntityId      int       `gorm:"index;not null" json:"entity_id"`           // invoice_id, stock_summary_id, etc
	Details       string    `gorm:"type:text" json:"details"`                  // human-readable mismatch detail
	CorrelationId string    `gorm:"size:64;index" json:"correlation_id"`
	CreatedAt     time.Time `gorm:"autoCreateTime" json:"created_at"`
}
