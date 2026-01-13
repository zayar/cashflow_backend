package models

import (
	"time"

	"github.com/shopspring/decimal"
)

// DailySummary is a small, query-friendly aggregate table used by dashboards.
//
// Grain: (business_id, currency_id, branch_id, transaction_date).
// Values are stored as positive numbers:
// - total_income: positive income amount for the day
// - total_expense: positive expense amount for the day
//
// NOTE: This table is derived data and can be rebuilt from the ledger/daily balances.
type DailySummary struct {
	BusinessId       string    `gorm:"primaryKey;size:64;index:idx_ds_biz_date,priority:1;index:idx_ds_biz_curr_date,priority:1" json:"business_id"`
	CurrencyId       int       `gorm:"primaryKey;index:idx_ds_biz_curr_date,priority:2" json:"currency_id"`
	BranchId         int       `gorm:"primaryKey" json:"branch_id"`
	TransactionDate  time.Time `gorm:"primaryKey;index:idx_ds_biz_date,priority:2;index:idx_ds_biz_curr_date,priority:3" json:"transaction_date"`

	TotalIncome  decimal.Decimal `gorm:"type:decimal(20,4);default:0" json:"total_income"`
	TotalExpense decimal.Decimal `gorm:"type:decimal(20,4);default:0" json:"total_expense"`

	CreatedAt time.Time `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt time.Time `gorm:"autoUpdateTime" json:"updated_at"`
}

