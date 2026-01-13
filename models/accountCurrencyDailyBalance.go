package models

import (
	"time"

	"github.com/shopspring/decimal"
)

type AccountCurrencyDailyBalance struct {
	BusinessId         string          `gorm:"primaryKey;index:idx_acb_biz_date_branch_curr,priority:1;index:idx_acb_biz_acct_date,priority:1" json:"business_id"`
	AccountId          int             `gorm:"primaryKey;index:idx_acb_biz_acct_date,priority:2" json:"account_id"`
	Account            *Account        `gorm:"foreignKey:AccountId" json:"account"`
	TransactionDate    time.Time       `gorm:"primaryKey;index:idx_acb_biz_date_branch_curr,priority:2;index:idx_acb_biz_acct_date,priority:3" json:"transaction_date"`
	BranchId           int             `gorm:"primaryKey;index:idx_acb_biz_date_branch_curr,priority:3" json:"branch_id"`
	CurrencyId         int             `gorm:"primaryKey;index:idx_acb_biz_date_branch_curr,priority:4" json:"currency_id"`
	Debit              decimal.Decimal `gorm:"type:decimal(20,4);default:0" json:"debit"`
	Credit             decimal.Decimal `gorm:"type:decimal(20,4);default:0" json:"credit"`
	BaseDebit          decimal.Decimal `gorm:"type:decimal(20,4);default:0" json:"base_debit"`
	BaseCredit         decimal.Decimal `gorm:"type:decimal(20,4);default:0" json:"base_credit"`
	Balance            decimal.Decimal `gorm:"type:decimal(20,4);default:0" json:"balance"`
	RunningBalance     decimal.Decimal `gorm:"type:decimal(20,4);default:0" json:"running_balance"`
	RunningBaseBalance decimal.Decimal `gorm:"type:decimal(20,4);default:0" json:"running_base_balance"`
}

// ListAccountCurrencyDailyBalance(businessId, accountId, branchId, transactionDate, currencyId) ([]AccountCurrencyDailyBalance,error) <Owner/Custom>
