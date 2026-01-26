package workflow

import (
	"fmt"
	"slices"

	"github.com/mmdatafocus/books_backend/models"
	"github.com/shopspring/decimal"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

// RebuildInventoryAdjustmentJournalFromLedger rewrites the active inventory adjustment journal
// purely from the stock ledger (stock_histories) and the adjustment's selected account.
//
// Why:
// Older builds could create/leave extra valuation lines (e.g. COGS) or even negative amounts.
// If valuation deltas later become zero, inventory rebuild may not re-touch the journal, leaving
// the bad line visible forever. This function forces a clean 2-sided posting:
// - Inventory accounts (one or many): aggregate from stock_histories per product's inventory_account_id
// - Adjustment account (inventory_adjustments.account_id): balancing line (opposite sign)
//
// It keeps the ledger append-only by reversing the current active journal and inserting a replacement.
func RebuildInventoryAdjustmentJournalFromLedger(
	tx *gorm.DB,
	logger *logrus.Logger,
	businessId string,
	adjustmentId int,
) (accountIds []int, branchId int, txTime models.MyDateString, err error) {
	if tx == nil {
		return nil, 0, models.MyDateString{}, fmt.Errorf("inv adj journal rebuild: tx is nil")
	}
	if businessId == "" || adjustmentId <= 0 {
		return nil, 0, models.MyDateString{}, fmt.Errorf("inv adj journal rebuild: invalid reference")
	}

	// Load adjustment (selected account + meta)
	var adj models.InventoryAdjustment
	if err := tx.
		Where("business_id = ? AND id = ?", businessId, adjustmentId).
		First(&adj).Error; err != nil {
		return nil, 0, models.MyDateString{}, err
	}

	var refType models.AccountReferenceType
	var stockRef models.StockReferenceType
	var reason string
	switch adj.AdjustmentType {
	case models.InventoryAdjustmentTypeQuantity:
		refType = models.AccountReferenceTypeInventoryAdjustmentQuantity
		stockRef = models.StockReferenceTypeInventoryAdjustmentQuantity
		reason = ReversalReasonInventoryAdjustQtyVoidUpdate
	case models.InventoryAdjustmentTypeValue:
		refType = models.AccountReferenceTypeInventoryAdjustmentValue
		stockRef = models.StockReferenceTypeInventoryAdjustmentValue
		reason = ReversalReasonInventoryAdjustValueVoidUpdate
	default:
		return nil, 0, models.MyDateString{}, fmt.Errorf("inv adj journal rebuild: unsupported adjustment_type=%s", adj.AdjustmentType)
	}

	// Fetch active stock ledger rows for this adjustment.
	var shRows []*models.StockHistory
	if err := tx.
		Where("business_id = ? AND reference_type = ? AND reference_id = ? AND is_reversal = 0 AND reversed_by_stock_history_id IS NULL",
			businessId, stockRef, adjustmentId).
		Find(&shRows).Error; err != nil {
		return nil, 0, models.MyDateString{}, err
	}
	if len(shRows) == 0 {
		return nil, adj.BranchId, models.MyDateString(adj.AdjustmentDate), nil
	}

	// Aggregate per inventory account.
	// NOTE: Inventory adjustments can contain many products; each product can map to a different inventory account.
	invByProduct := make(map[string]int) // "<type>:<id>" -> inventory_account_id
	invAmounts := make(map[int]decimal.Decimal)
	totalInv := decimal.Zero

	for _, sh := range shRows {
		if sh == nil || sh.ProductId <= 0 {
			continue
		}
		key := fmt.Sprintf("%s:%d", sh.ProductType, sh.ProductId)
		invAcc, ok := invByProduct[key]
		if !ok {
			pd, perr := GetProductDetail(tx, sh.ProductId, sh.ProductType)
			if perr != nil {
				return nil, 0, models.MyDateString{}, perr
			}
			invAcc = pd.InventoryAccountId
			invByProduct[key] = invAcc
		}
		if invAcc <= 0 {
			// Non-inventory item; skip.
			continue
		}
		val := sh.Qty.Mul(sh.BaseUnitValue) // signed
		invAmounts[invAcc] = invAmounts[invAcc].Add(val)
		totalInv = totalInv.Add(val)
	}

	// If nothing mapped to inventory accounts, nothing to rebuild.
	if totalInv.IsZero() {
		return nil, adj.BranchId, models.MyDateString(adj.AdjustmentDate), nil
	}

	// Load current active journal (business-scoped).
	var active models.AccountJournal
	if err := tx.
		Preload("AccountTransactions").
		Where("business_id = ? AND reference_id = ? AND reference_type = ? AND is_reversal = 0 AND reversed_by_journal_id IS NULL",
			businessId, adjustmentId, refType).
		Order("id DESC").
		First(&active).Error; err != nil {
		return nil, 0, models.MyDateString{}, err
	}

	// Reverse current journal (append-only ledger).
	if _, err := ReverseAccountJournal(tx, &active, "Inventory adjustment journal rebuild"); err != nil {
		return nil, 0, models.MyDateString{}, err
	}

	isValTrue := true
	newTxs := make([]models.AccountTransaction, 0, len(invAmounts)+1)

	// Inventory lines
	for invAcc, amt := range invAmounts {
		if amt.IsZero() {
			continue
		}
		t := models.AccountTransaction{
			BusinessId:           businessId,
			AccountId:            invAcc,
			BranchId:             adj.BranchId,
			TransactionDateTime:  adj.AdjustmentDate,
			BaseCurrencyId:       active.AccountTransactions[0].BaseCurrencyId,
			BaseDebit:            decimal.Zero,
			BaseCredit:           decimal.Zero,
			ForeignCurrencyId:    active.AccountTransactions[0].ForeignCurrencyId,
			ForeignDebit:         decimal.Zero,
			ForeignCredit:        decimal.Zero,
			ExchangeRate:         active.AccountTransactions[0].ExchangeRate,
			IsInventoryValuation: &isValTrue,
		}
		if amt.GreaterThan(decimal.Zero) {
			t.BaseDebit = amt
		} else {
			t.BaseCredit = amt.Abs()
		}
		newTxs = append(newTxs, t)

		if !slices.Contains(accountIds, invAcc) {
			accountIds = append(accountIds, invAcc)
		}
	}

	// Adjustment account balancing line (opposite of total inventory delta)
	adjAmt := totalInv.Neg()
	adjTx := models.AccountTransaction{
		BusinessId:           businessId,
		AccountId:            adj.AccountId,
		BranchId:             adj.BranchId,
		TransactionDateTime:  adj.AdjustmentDate,
		BaseCurrencyId:       active.AccountTransactions[0].BaseCurrencyId,
		BaseDebit:            decimal.Zero,
		BaseCredit:           decimal.Zero,
		ForeignCurrencyId:    active.AccountTransactions[0].ForeignCurrencyId,
		ForeignDebit:         decimal.Zero,
		ForeignCredit:        decimal.Zero,
		ExchangeRate:         active.AccountTransactions[0].ExchangeRate,
		IsInventoryValuation: &isValTrue,
	}
	if adjAmt.GreaterThan(decimal.Zero) {
		adjTx.BaseDebit = adjAmt
	} else {
		adjTx.BaseCredit = adjAmt.Abs()
	}
	newTxs = append(newTxs, adjTx)
	if !slices.Contains(accountIds, adj.AccountId) {
		accountIds = append(accountIds, adj.AccountId)
	}

	replacement := models.AccountJournal{
		BusinessId:          active.BusinessId,
		BranchId:            active.BranchId,
		TransactionDateTime: active.TransactionDateTime,
		TransactionNumber:   active.TransactionNumber,
		TransactionDetails:  active.TransactionDetails,
		ReferenceNumber:     active.ReferenceNumber,
		CustomerId:          active.CustomerId,
		SupplierId:          active.SupplierId,
		ReferenceId:         active.ReferenceId,
		ReferenceType:       active.ReferenceType,
		IsReversal:          false,
		ReversesJournalId:   nil,
		ReversedByJournalId: nil,
		ReversalReason:      nil,
		ReversedAt:          nil,
		AccountTransactions: newTxs,
	}

	if err := tx.Create(&replacement).Error; err != nil {
		return nil, 0, models.MyDateString{}, err
	}

	// Return meta so callers can UpdateBalances.
	if logger != nil {
		logger.WithFields(logrus.Fields{
			"business_id":  businessId,
			"adjustment_id": adjustmentId,
			"ref_type":     refType,
			"accounts":     accountIds,
			"reason":       reason,
		}).Info("inv.adjustment.journal.rebuilt")
	}

	return accountIds, adj.BranchId, models.MyDateString(adj.AdjustmentDate), nil
}

