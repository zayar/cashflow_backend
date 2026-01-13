package models

import (
	"time"

	"github.com/shopspring/decimal"
)

type MoneyTransaction struct {
	ID                    int             `gorm:"primary_key" json:"id"`
	BusinessId            string          `gorm:"index;not null" json:"business_id"`
	MoneyAccountId        int             `gorm:"index;not null" json:"money_account_id"`
	TransactionDateTime   time.Time       `gorm:"index;not null" json:"transaction_date_time"`
	TransactionType       string          `gorm:"size:255" json:"transaction_type"`
	Description           string          `gorm:"size:255" json:"description"`
	BranchId              int             `json:"branch_id"`
	BaseDeposit           decimal.Decimal `gorm:"type:decimal(20,4);default:0" json:"base_deposit"`
	BaseWithdrawal        decimal.Decimal `gorm:"type:decimal(20,4);default:0" json:"base_withdrawal"`
	BaseClosingBalance    decimal.Decimal `gorm:"type:decimal(20,4);default:0" json:"base_closing_balance"`
	ForeignDeposit        decimal.Decimal `gorm:"type:decimal(20,4);default:0" json:"foreign_deposit"`
	ForeignWithdrawal     decimal.Decimal `gorm:"type:decimal(20,4);default:0" json:"foreign_withdrawal"`
	ForeignClosingBalance decimal.Decimal `gorm:"type:decimal(20,4);default:0" json:"foreign_closing_balance"`
	ExchangeRate          decimal.Decimal `gorm:"type:decimal(20,4);default:0" json:"exchange_rate"`
}

// ListMoneyTransaction(businessId, moneyAccountId, branchId, transactionDateTime, transactionType) ([]MoneyTransaction,error) <Owner/Custom>
