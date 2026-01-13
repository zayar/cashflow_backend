package workflow

import (
	"fmt"
	"time"

	"bitbucket.org/mmdatafocus/books_backend/models"
	"bitbucket.org/mmdatafocus/books_backend/utils"
	"gorm.io/gorm"
)

// ReverseStockHistories appends reversal StockHistory rows for the provided originals.
//
// This preserves inventory auditability by never deleting original rows.
// Closing balances are recalculated by existing ProcessIncomingStocks/ProcessOutgoingStocks.
func ReverseStockHistories(tx *gorm.DB, originals []*models.StockHistory, reason string) ([]*models.StockHistory, error) {
	if tx == nil {
		return nil, fmt.Errorf("reverse stock: tx is nil")
	}
	if len(originals) == 0 {
		return []*models.StockHistory{}, nil
	}
	now := time.Now().UTC()
	reasonCopy := reason

	reversals := make([]*models.StockHistory, 0, len(originals))
	for _, o := range originals {
		if o == nil {
			continue
		}
		// If already reversed, skip quietly.
		if o.ReversedByStockHistoryId != nil && *o.ReversedByStockHistoryId > 0 {
			continue
		}

		isOutgoing := false
		if o.IsOutgoing != nil {
			isOutgoing = *o.IsOutgoing
		}

		// Reverse direction and qty.
		reversalIsOutgoing := !isOutgoing
		reversalQty := o.Qty.Neg()

		isOutgoingPtr := utils.NewFalse()
		if reversalIsOutgoing {
			isOutgoingPtr = utils.NewTrue()
		}

		rev := &models.StockHistory{
			BusinessId:        o.BusinessId,
			WarehouseId:       o.WarehouseId,
			ProductId:         o.ProductId,
			ProductType:       o.ProductType,
			BatchNumber:       o.BatchNumber,
			StockDate:         o.StockDate,
			Qty:               reversalQty,
			Description:       "REV: " + o.Description,
			BaseUnitValue:     o.BaseUnitValue,
			ReferenceType:     o.ReferenceType,
			ReferenceID:       o.ReferenceID,
			ReferenceDetailID: o.ReferenceDetailID,
			IsOutgoing:        isOutgoingPtr,
			IsTransferIn:      o.IsTransferIn,
			IsReversal:        true,
			ReversesStockHistoryId: &o.ID,
			ReversalReason:         &reasonCopy,
		}

		if err := tx.Create(rev).Error; err != nil {
			return nil, err
		}

		// Mark original reversed (metadata-only update).
		if err := tx.Model(&models.StockHistory{}).
			Where("id = ?", o.ID).
			Updates(map[string]interface{}{
				"reversed_by_stock_history_id": rev.ID,
				"reversal_reason":              &reasonCopy,
				"reversed_at":                  &now,
			}).Error; err != nil {
			return nil, err
		}

		reversals = append(reversals, rev)
	}

	return reversals, nil
}

