package models

import "time"

// DocumentTemplate stores per-business document template configuration (UI only).
//
// IMPORTANT: This is NOT a financial “second source of truth”.
// It only affects rendering (print/PDF/email previews), never posting/COGS/valuation logic.
type DocumentTemplate struct {
	ID int `gorm:"primary_key" json:"id"`

	BusinessId   string `gorm:"not null;index:idx_dt_biz_doc,priority:1" json:"business_id"`
	DocumentType string `gorm:"not null;index:idx_dt_biz_doc,priority:2;size:50" json:"document_type"`

	Name      string `gorm:"not null;size:150" json:"name"`
	IsDefault bool   `gorm:"not null;default:false" json:"is_default"`

	// ConfigJson is stored as JSON text to avoid requiring MySQL JSON column support.
	// The API validates that it is valid JSON.
	ConfigJson string `gorm:"type:longtext" json:"config_json"`

	CreatedAt time.Time `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt time.Time `gorm:"autoUpdateTime" json:"updated_at"`
}

const (
	DocumentTypeInvoice        = "invoice"
	DocumentTypeCreditNote     = "credit_note"
	DocumentTypePaymentReceipt = "payment_receipt"
	DocumentTypeBill           = "bill"
)

func IsAllowedDocumentTemplateType(t string) bool {
	switch t {
	case DocumentTypeInvoice, DocumentTypeCreditNote, DocumentTypePaymentReceipt, DocumentTypeBill:
		return true
	default:
		return false
	}
}
