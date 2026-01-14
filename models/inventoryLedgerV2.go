package models

import (
	"time"

	"github.com/shopspring/decimal"
)

// Phase 1 scaffolding (v2): append-only inventory movements ledger.
type InventoryMovement struct {
	ID            string          `gorm:"size:36;primary_key" json:"id"` // uuid
	BusinessId    string          `gorm:"index:idx_inv_move_biz_item_date,priority:1;not null" json:"business_id"`
	ProductId     int             `gorm:"index:idx_inv_move_biz_item_date,priority:2;not null" json:"product_id"`
	ProductType   ProductType     `gorm:"type:enum('S','G','C','V','I');default:S" json:"product_type"`
	QtyDelta      decimal.Decimal `gorm:"type:decimal(20,4);not null" json:"qty_delta"`
	DocType       string          `gorm:"size:20;not null" json:"doc_type"` // e.g. IV, BL, TO, IVAQ...
	DocId         int             `gorm:"index;not null" json:"doc_id"`
	DocLineId     int             `gorm:"index" json:"doc_line_id"`
	EffectiveDate time.Time       `gorm:"index:idx_inv_move_biz_item_date,priority:3;not null" json:"effective_date"`
	CreatedAt     time.Time       `gorm:"autoCreateTime" json:"created_at"`
	CorrelationId string          `gorm:"size:64;index" json:"correlation_id"`
	OutboxId      *int            `gorm:"index" json:"outbox_id"`
}

// Phase 1 scaffolding (v2): persisted COGS allocations (do not recompute from current item price).
type CogsAllocation struct {
	ID            int             `gorm:"primary_key" json:"id"`
	BusinessId    string          `gorm:"index;not null" json:"business_id"`
	InvoiceLineId int             `gorm:"index;not null" json:"invoice_line_id"`
	MovementId    string          `gorm:"size:36;index;not null" json:"movement_id"`
	Qty           decimal.Decimal `gorm:"type:decimal(20,4);not null" json:"qty"`
	UnitCost      decimal.Decimal `gorm:"type:decimal(20,4);not null" json:"unit_cost"`
	CreatedAt     time.Time       `gorm:"autoCreateTime" json:"created_at"`
	CorrelationId string          `gorm:"size:64;index" json:"correlation_id"`
}
