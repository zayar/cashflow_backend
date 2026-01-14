package workflow

import (
	"time"

	"github.com/mmdatafocus/books_backend/models"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

// ReplaceStockHistoryByID keeps inventory append-only by:
// - appending a reversal row for the old stock history
// - inserting a new replacement row (non-reversal) with adjusted values
//
// This is used when historical rows were previously mutated in-place (qty/base_unit_value).
func ReplaceStockHistoryByID(
	tx *gorm.DB,
	oldID int,
	reason string,
	apply func(newRow *models.StockHistory),
) (*models.StockHistory, error) {
	if tx == nil {
		return nil, gorm.ErrInvalidDB
	}
	if oldID <= 0 {
		return nil, gorm.ErrRecordNotFound
	}

	var old models.StockHistory
	if err := tx.Where("id = ?", oldID).First(&old).Error; err != nil {
		return nil, err
	}

	// Build replacement from old (preserve reference keys) and reset derived fields.
	newRow := old
	newRow.ID = 0
	newRow.IsReversal = false
	newRow.ReversesStockHistoryId = nil
	newRow.ReversedByStockHistoryId = nil
	newRow.ReversalReason = nil
	newRow.ReversedAt = nil
	newRow.ClosingQty = decimal.Zero
	newRow.ClosingAssetValue = decimal.Zero
	newRow.CumulativeIncomingQty = decimal.Zero
	newRow.CumulativeOutgoingQty = decimal.Zero
	newRow.CumulativeSequence = 0
	newRow.CreatedAt = time.Time{}
	newRow.UpdatedAt = time.Time{}

	if apply != nil {
		apply(&newRow)
	}

	// No-op if nothing changed.
	if old.Qty.Equal(newRow.Qty) && old.BaseUnitValue.Equal(newRow.BaseUnitValue) && old.Description == newRow.Description {
		return &old, nil
	}

	if _, err := ReverseStockHistories(tx, []*models.StockHistory{&old}, reason); err != nil {
		return nil, err
	}
	if err := tx.Create(&newRow).Error; err != nil {
		return nil, err
	}
	return &newRow, nil
}

