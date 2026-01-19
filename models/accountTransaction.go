package models

import (
	"context"
	"errors"
	"time"

	"github.com/mmdatafocus/books_backend/config"
	"github.com/mmdatafocus/books_backend/utils"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

type AccountJournalReferenceUnion interface {
	// Common method for getting ID
	GetId() int
}

type PubSubMessageRecord struct {
	ID                  int                  `gorm:"primary_key;index:idx_outbox_dispatch,priority:3;index:idx_outbox_reconcile,priority:3" json:"id"`
	AccountJournalId    int                  `json:"account_journal_id"`
	BusinessId          string               `gorm:"size:64;not null;index;index:idx_outbox_reconcile,priority:1" json:"business_id"`
	TransactionDateTime time.Time            `gorm:"index;not null" json:"transaction_date_time"`
	ReferenceId         int                  `json:"reference_id"`
	ReferenceType       AccountReferenceType `gorm:"type:enum('JN','IV','CP','CN','CNA','CNR','EP','ER','BL','SP','POS', 'PVOS','IVAQ','IVAV','IWO','ACP','ASP','COB','SOB','OB','AC','AD','SCR','OI','TO','SC','SCA','OD','OC','SAA','SAR','CAA','CAR','PGOS','POSIVP')" json:"reference_type"`
	Action              PubSubMessageAction  `gorm:"type:enum('C','U','D')" json:"action"`
	OldObj              []byte               `gorm:"type:blob" json:"old_obj"`
	NewObj              []byte               `gorm:"type:blob" json:"new_obj"`
	IsProcessed         bool                 `gorm:"index;not null;index:idx_outbox_reconcile,priority:2" json:"is_processed"`
	// Phase 0 outbox metadata (publish happens after commit via dispatcher).
	PublishStatus    string     `gorm:"size:20;index;not null;default:'PENDING';index:idx_outbox_dispatch,priority:1" json:"publish_status"` // PENDING|PROCESSING|SENT|FAILED|DEAD
	PublishedAt      *time.Time `gorm:"index" json:"published_at"`
	PubSubMessageId  *string    `gorm:"size:255" json:"pubsub_message_id"`
	PublishAttempts  int        `gorm:"not null;default:0" json:"publish_attempts"`
	NextAttemptAt    *time.Time `gorm:"index;index:idx_outbox_dispatch,priority:2" json:"next_attempt_at"`
	LockedAt         *time.Time `gorm:"index" json:"locked_at"`
	LockedBy         *string    `gorm:"size:100" json:"locked_by"`
	LastPublishError *string    `gorm:"type:text" json:"last_publish_error"`
	// Processing metadata (consumer/worker)
	LastProcessError *string    `gorm:"type:text" json:"last_process_error"`
	ProcessedAt      *time.Time `gorm:"index" json:"processed_at"`
	CorrelationId    string     `gorm:"size:64;index" json:"correlation_id"`
	CreatedAt        time.Time  `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt        time.Time  `gorm:"autoUpdateTime" json:"updated_at"`
}

func ConvertToPubSubMessage(record PubSubMessageRecord) config.PubSubMessage {
	return config.PubSubMessage{
		ID:                  record.ID,
		BusinessId:          record.BusinessId,
		TransactionDateTime: record.TransactionDateTime,
		ReferenceId:         record.ReferenceId,
		ReferenceType:       string(record.ReferenceType),
		Action:              string(record.Action),
		OldObj:              record.OldObj,
		NewObj:              record.NewObj,
		CorrelationId:       record.CorrelationId,
	}
}

type AccountJournal struct {
	ID                  int                  `gorm:"primary_key" json:"id"`
	BusinessId          string               `gorm:"index;not null;index:idx_aj_biz_date,priority:1;index:idx_aj_biz_ref,priority:1" json:"business_id"`
	BranchId            int                  `gorm:"index;not null" json:"branch_id"`
	TransactionDateTime time.Time            `gorm:"index;not null;index:idx_aj_biz_date,priority:2" json:"transaction_date_time"`
	TransactionNumber   string               `gorm:"size:255" json:"transaction_number"`
	TransactionDetails  string               `gorm:"type:text" json:"transaction_details"`
	ReferenceNumber     string               `gorm:"size:255" json:"reference_number"`
	CustomerId          int                  `gorm:"index" json:"customer_id"`
	SupplierId          int                  `gorm:"index" json:"supplier_id"`
	ReferenceId         int                  `gorm:"index:idx_aj_biz_ref,priority:3" json:"reference_id"`
	ReferenceType       AccountReferenceType `gorm:"type:enum('JN','IV','CP','CN','CNA','CNR','EP','ER','BL','SP','POS', 'PVOS','IVAQ','IVAV','IWO','ACP','ASP','COB','SOB','OB','AC','AD','SCR','OI','TO','SC','SCA','OD','OC','SAA','SAR','CAA','CAR','PGOS','POSIVP');index:idx_aj_biz_ref,priority:2" json:"reference_type"`
	// Composite indexes (Phase A):
	// - idx_aj_biz_ref:  (business_id, reference_type, reference_id)
	// - idx_aj_biz_date: (business_id, transaction_date_time)
	// Phase 1: ledger immutability & reversals
	// - Posted journals are never deleted; changes are done by inserting a reversal journal.
	// - For a given (reference_type, reference_id), there should be at most one "active" journal where:
	//   is_reversal = false AND reversed_by_journal_id IS NULL
	IsReversal          bool                         `gorm:"not null;default:false;index" json:"is_reversal"`
	ReversesJournalId   *int                         `gorm:"index" json:"reverses_journal_id"`
	ReversedByJournalId *int                         `gorm:"index" json:"reversed_by_journal_id"`
	ReversalReason      *string                      `gorm:"type:text" json:"reversal_reason"`
	ReversedAt          *time.Time                   `gorm:"index" json:"reversed_at"`
	AccountTransactions []AccountTransaction         `gorm:"foreignKey:JournalId" json:"account_transactions"`
	ReferenceData       AccountJournalReferenceUnion `gorm:"-" json:"referenceData"`
	CreatedAt           time.Time                    `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt           time.Time                    `gorm:"autoUpdateTime" json:"updated_at"`
}

type AccountTransaction struct {
	ID         int    `gorm:"primary_key" json:"id"`
	BusinessId string `gorm:"index;index:idx_at_biz_date,priority:1;index:idx_at_biz_acct_date,priority:1;index:idx_at_biz_journal,priority:1" json:"business_id"`
	JournalId  int    `gorm:"index;not null;index:idx_at_biz_journal,priority:2" json:"journal_id" binding:"required"`
	AccountId  int    `gorm:"index;not null;index:idx_at_biz_acct_date,priority:2" json:"account_id" binding:"required"`
	// Account               *Account        `gorm:"foreignKey:AccountId" json:"account"`
	BranchId              int             `gorm:"index;not null" json:"branch_id"`
	TransactionDateTime   time.Time       `gorm:"index;not null;index:idx_at_biz_date,priority:2;index:idx_at_biz_acct_date,priority:3" json:"transaction_date_time"`
	Description           string          `gorm:"size:255" json:"description"`
	BaseCurrencyId        int             `gorm:"index;not null" json:"base_currency_id" binding:"required"`
	BaseDebit             decimal.Decimal `gorm:"type:decimal(20,4);default:0" json:"base_debit"`
	BaseCredit            decimal.Decimal `gorm:"type:decimal(20,4);default:0" json:"base_credit"`
	BaseClosingBalance    decimal.Decimal `gorm:"type:decimal(20,4);default:0" json:"base_closing_balance"`
	ForeignCurrencyId     int             `gorm:"index;not null" json:"foreign_currency_id" binding:"required"`
	ForeignDebit          decimal.Decimal `gorm:"type:decimal(20,4);default:0" json:"foreign_debit"`
	ForeignCredit         decimal.Decimal `gorm:"type:decimal(20,4);default:0" json:"foreign_credit"`
	ForeignClosingBalance decimal.Decimal `gorm:"type:decimal(20,4);default:0" json:"foreign_closing_balance"`
	ExchangeRate          decimal.Decimal `gorm:"type:decimal(20,4);default:0" json:"exchange_rate"`
	IsInventoryValuation  *bool           `gorm:"not null;default:false" json:"is_inventory_valuation"`
	IsTransferIn          *bool           `gorm:"not null;default:false" json:"is_transfer_in"`
	BankingTransactionId  int             `gorm:"index;default:0" json:"banking_transaction_id"`
	RealisedAmount        decimal.Decimal `gorm:"type:decimal(20,4);default:0" json:"realised_amount"`
	CreatedAt             time.Time       `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt             time.Time       `gorm:"autoUpdateTime" json:"updated_at"`
}

// Ledger immutability guardrails:
// - account_transactions are append-only (no updates/deletes).
// - account_journals must never be deleted; limited updates are allowed only for reversal linkage fields.

func (t *AccountTransaction) BeforeUpdate(tx *gorm.DB) error {
	return errors.New("immutable ledger: account_transactions cannot be updated")
}

func (t *AccountTransaction) BeforeDelete(tx *gorm.DB) error {
	return errors.New("immutable ledger: account_transactions cannot be deleted")
}

func (j *AccountJournal) BeforeDelete(tx *gorm.DB) error {
	return errors.New("immutable ledger: account_journals cannot be deleted")
}

func (j *AccountJournal) BeforeUpdate(tx *gorm.DB) error {
	// Allow only reversal linkage fields to be updated.
	allowed := map[string]bool{
		"IsReversal":          true,
		"ReversesJournalId":   true,
		"ReversedByJournalId": true,
		"ReversalReason":      true,
		"ReversedAt":          true,
		"UpdatedAt":           true,
	}
	if tx == nil || tx.Statement == nil || tx.Statement.Schema == nil {
		return nil
	}
	for _, f := range tx.Statement.Schema.Fields {
		if tx.Statement.Changed(f.Name) && !allowed[f.Name] {
			return errors.New("immutable ledger: only reversal linkage fields may be updated on account_journals")
		}
	}
	return nil
}

type AccountJournalTransaction struct {
	Account    *Account        `json:"account"`
	Branch     *AllBranch      `json:"branch"`
	AccountId  int             `json:"accountId"`
	BranchId   int             `json:"branchId"`
	BaseDebit  decimal.Decimal `json:"baseDebit"`
	BaseCredit decimal.Decimal `json:"baseCredit"`
}

// type AccountJournalTransaction struct {
// }

func GetAccountJournalTransactions(ctx context.Context, referenceId int, referenceType AccountReferenceType, accountId *int) ([]*AccountJournalTransaction, error) {
	businessId, ok := utils.GetBusinessIdFromContext(ctx)
	if !ok || businessId == "" {
		return nil, errors.New("business id is required")
	}

	// if err := ValidateAccountReference(ctx, businessId, referenceId, referenceType); err != nil {
	// 	return nil, err
	// }

	var results []*AccountJournalTransaction
	db := config.GetDB()
	// CRITICAL: Only return transactions from the active (non-reversed) journal.
	// After schema update, we must filter for is_reversal = false AND reversed_by_journal_id IS NULL
	// to ensure we only get the current active journal entry, not reversals or inactive ones.
	sql := `
		SELECT
	    at.account_id,
	    at.base_currency_id,
	    at.branch_id,
	    at.base_debit,
	    at.base_credit
	FROM
	    account_journals AS aj
	        LEFT JOIN
	    account_transactions at ON aj.id = at.journal_id
	WHERE
	    aj.business_id = ?
	        AND aj.reference_type = ?
	        AND aj.reference_id = ?
	        AND aj.is_reversal = false
	        AND aj.reversed_by_journal_id IS NULL
	`
	args := []interface{}{businessId, referenceType, referenceId}
	if accountId != nil && *accountId > 0 {
		sql += " AND at.account_id = ?"
		args = append(args, *accountId)
	}
	if err := db.WithContext(ctx).Raw(sql, args...).Scan(&results).Error; err != nil {
		return nil, err
	}
	return results, nil
}
