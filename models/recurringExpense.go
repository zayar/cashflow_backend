package models

import (
	"time"

	"github.com/shopspring/decimal"
)

type RecurringExpense struct {
	ID               int             `gorm:"primary_key" json:"id"`
	BusinessId       string          `gorm:"index;not null" json:"business_id" binding:"required"`
	ProfileName      string          `gorm:"size:100;not null" json:"profile_name" binding:"required"`
	RepeatTimes      int             `gorm:"not null;default:1" json:"repeat_times" binding:"required"`
	RepeatTerms      RecurringTerms  `gorm:"type:enum('D', 'W', 'M', 'Y')" json:"repeat_terms" binding:"required"`
	StartDate        time.Time       `gorm:"not null" json:"start_date" binding:"required"`
	EndDate          time.Time       `json:"end_date"`
	IsNeverExpired   *bool           `gorm:"default:false" json:"is_never_expired"`
	ExpenseAccountId int             `gorm:"index;not null" json:"expense_account_id" binding:"required"`
	AssetAccountId   int             `gorm:"index;not null" json:"asset_account_id" binding:"required"`
	BranchId         int             `gorm:"index" json:"branch_id"`
	ExpenseDate      time.Time       `gorm:"not null" json:"expense_date" binding:"required"`
	CurrencyId       int             `gorm:"not null" json:"currency_id" binding:"required"`
	Amount           decimal.Decimal `gorm:"type:decimal(20,4);default:0" json:"amount"`
	SupplierId       int             `json:"supplier_id"`
	CustomerId       int             `json:"customer_id"`
	ReferenceNumber  string          `gorm:"size:255" json:"reference_number"`
	Notes            string          `gorm:"type:text" json:"notes"`
	ExpenseTaxId     int             `json:"expense_tax_id"`
	ExpenseTaxType   TaxType         `gorm:"type:enum('I', 'G')" json:"expense_tax_type"`
	CreatedAt        time.Time       `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt        time.Time       `gorm:"autoUpdateTime" json:"updated_at"`
}

type NewRecurringExpense struct {
	BusinessId       string          `json:"business_id" binding:"required"`
	ProfileName      string          `json:"profile_name" binding:"required"`
	RepeatTimes      int             `json:"repeat_times" binding:"required"`
	RepeatTerms      RecurringTerms  `json:"repeat_terms" binding:"required"`
	StartDate        time.Time       `json:"start_date" binding:"required"`
	EndDate          time.Time       `json:"end_date"`
	IsNeverExpired   *bool           `gorm:"default:false" json:"is_never_expired"`
	ExpenseAccountId int             `json:"expense_account_id" binding:"required"`
	AssetAccountId   int             `json:"asset_account_id" binding:"required"`
	BranchId         int             `json:"branch_id"`
	ExpenseDate      time.Time       `json:"expense_date" binding:"required"`
	CurrencyId       int             `json:"currency_id" binding:"required"`
	Amount           decimal.Decimal `json:"amount"`
	SupplierId       int             `json:"supplier_id"`
	CustomerId       int             `json:"customer_id"`
	ReferenceNumber  string          `json:"reference_number"`
	Notes            string          `json:"notes"`
	ExpenseTaxId     int             `json:"expense_tax_id"`
	ExpenseTaxType   TaxType         `json:"expense_tax_type"`
}
