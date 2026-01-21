package workflow

import (
	"fmt"

	"github.com/mmdatafocus/books_backend/config"
	"github.com/mmdatafocus/books_backend/models"
	"github.com/shopspring/decimal"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

type valuationDelta struct {
	BaseDebit  decimal.Decimal
	BaseCredit decimal.Decimal
}

// repostJournalWithValuationDeltas rewrites the active journal for a reference by:
// - inserting a reversal journal for the current active journal
// - inserting a new replacement journal with valuation deltas applied
//
// This keeps the ledger append-only (auditable) while allowing inventory valuation to change
// due to backdated stock movements.
//
// transferInFilter:
// - nil: apply to all valuation lines
// - non-nil: apply only to valuation lines matching IsTransferIn == *transferInFilter
func repostJournalWithValuationDeltas(
	tx *gorm.DB,
	logger *logrus.Logger,
	businessId string,
	refType models.AccountReferenceType,
	refId int,
	deltas map[int]valuationDelta, // account_id -> delta
	transferInFilter *bool,
	reason string,
) (replacementJournalId int, accountIds []int, err error) {
	if tx == nil {
		return 0, nil, fmt.Errorf("repost journal: tx is nil")
	}
	if businessId == "" || refId <= 0 || refType == "" {
		return 0, nil, fmt.Errorf("repost journal: invalid reference")
	}
	if len(deltas) == 0 {
		return 0, nil, nil
	}

	var aj *models.AccountJournal
	var existingAccountIds []int

	// Transfer orders have multiple active journals for the same reference (source + destination).
	// When transferInFilter is provided, select the active journal that matches that transfer direction.
	if transferInFilter != nil {
		var candidates []*models.AccountJournal
		if err := tx.
			Preload("AccountTransactions").
			// CRITICAL: always scope by business_id. reference_id is not globally unique.
			Where("business_id = ? AND reference_id = ? AND reference_type = ? AND is_reversal = 0 AND reversed_by_journal_id IS NULL", businessId, refId, refType).
			Find(&candidates).Error; err != nil {
			return 0, nil, err
		}
		for _, c := range candidates {
			for _, t := range c.AccountTransactions {
				if t.IsInventoryValuation != nil && *t.IsInventoryValuation {
					tv := false
					if t.IsTransferIn != nil {
						tv = *t.IsTransferIn
					}
					if tv == *transferInFilter {
						aj = c
						break
					}
				}
			}
			if aj != nil {
				break
			}
		}
		if aj == nil {
			return 0, nil, fmt.Errorf("repost journal: matching active journal not found")
		}
		for _, t := range aj.AccountTransactions {
			existingAccountIds = append(existingAccountIds, t.AccountId)
		}
	} else {
		var err error
		aj, _, existingAccountIds, err = GetExistingAccountJournal(tx, refId, refType)
		if err != nil {
			return 0, nil, err
		}
		if aj == nil {
			return 0, nil, fmt.Errorf("repost journal: active journal not found")
		}
	}

	// Build replacement transactions from existing ones.
	remaining := make(map[int]valuationDelta, len(deltas))
	for k, v := range deltas {
		remaining[k] = v
	}

	newTxs := make([]models.AccountTransaction, 0, len(aj.AccountTransactions)+len(remaining))
	for _, t := range aj.AccountTransactions {
		nt := models.AccountTransaction{
			BusinessId:           t.BusinessId,
			AccountId:            t.AccountId,
			BranchId:             t.BranchId,
			TransactionDateTime:  t.TransactionDateTime,
			Description:          t.Description,
			BaseCurrencyId:       t.BaseCurrencyId,
			BaseDebit:            t.BaseDebit,
			BaseCredit:           t.BaseCredit,
			ForeignCurrencyId:    t.ForeignCurrencyId,
			ForeignDebit:         t.ForeignDebit,
			ForeignCredit:        t.ForeignCredit,
			ExchangeRate:         t.ExchangeRate,
			IsInventoryValuation: t.IsInventoryValuation,
			IsTransferIn:         t.IsTransferIn,
			BankingTransactionId: t.BankingTransactionId,
			RealisedAmount:       t.RealisedAmount,
		}

		isVal := t.IsInventoryValuation != nil && *t.IsInventoryValuation
		transferMatch := true
		if transferInFilter != nil {
			tv := false
			if t.IsTransferIn != nil {
				tv = *t.IsTransferIn
			}
			transferMatch = tv == *transferInFilter
		}

		if isVal && transferMatch {
			if d, ok := remaining[t.AccountId]; ok {
				nt.BaseDebit = nt.BaseDebit.Add(d.BaseDebit)
				nt.BaseCredit = nt.BaseCredit.Add(d.BaseCredit)
				delete(remaining, t.AccountId)
			}
		}

		newTxs = append(newTxs, nt)
	}

	// Append missing valuation lines (should be rare; usually placeholder exists).
	baseCurrencyId := 0
	foreignCurrencyId := 0
	exchangeRate := decimal.Zero
	branchId := aj.BranchId
	if len(aj.AccountTransactions) > 0 {
		baseCurrencyId = aj.AccountTransactions[0].BaseCurrencyId
		foreignCurrencyId = aj.AccountTransactions[0].ForeignCurrencyId
		exchangeRate = aj.AccountTransactions[0].ExchangeRate
		branchId = aj.AccountTransactions[0].BranchId
	}
	isValTrue := true
	for accountId, d := range remaining {
		newTxs = append(newTxs, models.AccountTransaction{
			BusinessId:           businessId,
			AccountId:            accountId,
			BranchId:             branchId,
			TransactionDateTime:  aj.TransactionDateTime,
			BaseCurrencyId:       baseCurrencyId,
			BaseDebit:            d.BaseDebit,
			BaseCredit:           d.BaseCredit,
			ForeignCurrencyId:    foreignCurrencyId,
			ForeignDebit:         decimal.Zero,
			ForeignCredit:        decimal.Zero,
			ExchangeRate:         exchangeRate,
			IsInventoryValuation: &isValTrue,
			IsTransferIn:         transferInFilter,
		})
	}

	// Reverse the current active journal.
	if _, err := ReverseAccountJournal(tx, aj, reason); err != nil {
		config.LogError(logger, "ValuationRepost.go", "repostJournalWithValuationDeltas", "ReverseAccountJournal", aj, err)
		return 0, nil, err
	}

	// Create replacement active journal for the same reference.
	reasonCopy := reason
	replacement := models.AccountJournal{
		BusinessId:          aj.BusinessId,
		BranchId:            aj.BranchId,
		TransactionDateTime: aj.TransactionDateTime,
		TransactionNumber:   aj.TransactionNumber,
		TransactionDetails:  aj.TransactionDetails,
		ReferenceNumber:     aj.ReferenceNumber,
		CustomerId:          aj.CustomerId,
		SupplierId:          aj.SupplierId,
		ReferenceId:         aj.ReferenceId,
		ReferenceType:       aj.ReferenceType,
		IsReversal:          false,
		ReversesJournalId:   nil,
		ReversedByJournalId: nil,
		ReversalReason:      nil,
		ReversedAt:          nil,
		AccountTransactions: newTxs,
	}

	if err := tx.Create(&replacement).Error; err != nil {
		config.LogError(logger, "ValuationRepost.go", "repostJournalWithValuationDeltas", "CreateReplacementJournal", replacement, err)
		return 0, nil, err
	}

	_ = reasonCopy // reserved for future: attach reason to replacement details if needed

	// Return impacted accounts (existing + delta accounts).
	accountIds = make([]int, 0, len(existingAccountIds)+len(deltas))
	accountIds = append(accountIds, existingAccountIds...)
	for accId := range deltas {
		found := false
		for _, existing := range existingAccountIds {
			if existing == accId {
				found = true
				break
			}
		}
		if !found {
			accountIds = append(accountIds, accId)
		}
	}

	return replacement.ID, accountIds, nil
}
