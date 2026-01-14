package workflow

import (
	"fmt"
	"time"

	"github.com/mmdatafocus/books_backend/models"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

// ReverseAccountJournal creates a reversal journal that negates the original journal's transactions.
//
// Design:
// - We do NOT delete posted journals.
// - We insert a reversal journal (is_reversal=true) and mark the original as reversed_by_journal_id=<reversal>.
//
// NOTE: This only handles financial journal reversal. Inventory/banking side-effects (if any)
// are managed by their respective workflows.
func ReverseAccountJournal(tx *gorm.DB, original *models.AccountJournal, reason string) (reversalJournalID int, err error) {
	if tx == nil || original == nil {
		return 0, fmt.Errorf("reverse journal: tx/original is nil")
	}

	// Idempotent behavior: if already reversed, return existing reversal id.
	if original.ReversedByJournalId != nil && *original.ReversedByJournalId > 0 {
		return *original.ReversedByJournalId, nil
	}

	// Ensure transactions are loaded.
	if original.AccountTransactions == nil {
		var loaded models.AccountJournal
		if err := tx.Preload("AccountTransactions").Where("id = ?", original.ID).First(&loaded).Error; err != nil {
			return 0, err
		}
		original = &loaded
	}

	reasonCopy := reason
	now := time.Now().UTC()

	reversedTxs := make([]models.AccountTransaction, 0, len(original.AccountTransactions))
	for _, t := range original.AccountTransactions {
		reversedTxs = append(reversedTxs, models.AccountTransaction{
			BusinessId:            t.BusinessId,
			AccountId:             t.AccountId,
			BranchId:              t.BranchId,
			TransactionDateTime:   t.TransactionDateTime,
			Description:           t.Description,
			BaseCurrencyId:        t.BaseCurrencyId,
			BaseDebit:             t.BaseCredit,
			BaseCredit:            t.BaseDebit,
			BaseClosingBalance:    decimal.Zero, // will be recalculated by UpdateBalances()
			ForeignCurrencyId:     t.ForeignCurrencyId,
			ForeignDebit:          t.ForeignCredit,
			ForeignCredit:         t.ForeignDebit,
			ForeignClosingBalance: decimal.Zero, // will be recalculated by UpdateBalances()
			ExchangeRate:          t.ExchangeRate,
			IsInventoryValuation:  t.IsInventoryValuation,
			IsTransferIn:          t.IsTransferIn,
			BankingTransactionId:  0, // do not link reversal to historical banking txn rows
			RealisedAmount:        t.RealisedAmount.Neg(),
		})
	}

	reversal := models.AccountJournal{
		BusinessId:          original.BusinessId,
		BranchId:            original.BranchId,
		TransactionDateTime: original.TransactionDateTime,
		TransactionNumber:   "REV-" + original.TransactionNumber,
		TransactionDetails:  "Reversal: " + reasonCopy,
		ReferenceNumber:     original.ReferenceNumber,
		CustomerId:          original.CustomerId,
		SupplierId:          original.SupplierId,
		ReferenceId:         original.ReferenceId,
		ReferenceType:       original.ReferenceType,
		IsReversal:          true,
		ReversesJournalId:   &original.ID,
		ReversalReason:      &reasonCopy,
		AccountTransactions: reversedTxs,
	}

	if err := tx.Create(&reversal).Error; err != nil {
		return 0, err
	}

	// Mark original as reversed (metadata-only update).
	if err := tx.Model(&models.AccountJournal{}).
		Where("id = ?", original.ID).
		Updates(map[string]interface{}{
			"reversed_by_journal_id": reversal.ID,
			"reversal_reason":        &reasonCopy,
			"reversed_at":            &now,
		}).Error; err != nil {
		return 0, err
	}

	return reversal.ID, nil
}
