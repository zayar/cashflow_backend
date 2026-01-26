package workflow

import (
	"fmt"

	"github.com/mmdatafocus/books_backend/models"
	"github.com/mmdatafocus/books_backend/utils"
	"github.com/shopspring/decimal"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

// BackfillTransferOrderMissingTransferIn ensures that for a Transfer Order (reference_type=TO),
// every active outgoing stock_histories row (source warehouse) has a corresponding active
// incoming row (destination warehouse) with the same qty abs + base_unit_value.
//
// Why:
// Some early data can contain transfer-out stock ledger rows without the matching transfer-in rows.
// That makes Inventory Valuation Summary (stock_histories) lower than Balance Sheet (GL) by the missing value.
func BackfillTransferOrderMissingTransferIn(
	tx *gorm.DB,
	logger *logrus.Logger,
	businessId string,
	transferOrderId int,
) (created int, affectedAccountIds []int, err error) {
	if tx == nil {
		return 0, nil, fmt.Errorf("transfer order ledger fix: tx is nil")
	}
	if businessId == "" || transferOrderId <= 0 {
		return 0, nil, fmt.Errorf("transfer order ledger fix: invalid args")
	}

	// Load transfer order for destination warehouse.
	var to models.TransferOrder
	if err := tx.
		Where("business_id = ? AND id = ?", businessId, transferOrderId).
		First(&to).Error; err != nil {
		return 0, nil, err
	}

	// Fetch active outgoing rows for this transfer (source side).
	var outRows []*models.StockHistory
	if err := tx.
		Where("business_id = ? AND reference_type = ? AND reference_id = ? AND is_reversal = 0 AND reversed_by_stock_history_id IS NULL AND (is_transfer_in = 0 OR is_transfer_in IS NULL)",
			businessId, models.StockReferenceTypeTransferOrder, transferOrderId).
		Find(&outRows).Error; err != nil {
		return 0, nil, err
	}
	if len(outRows) == 0 {
		return 0, nil, nil
	}

	// Fetch existing active incoming rows for this transfer (destination side).
	var inRows []*models.StockHistory
	if err := tx.
		Where("business_id = ? AND reference_type = ? AND reference_id = ? AND is_reversal = 0 AND reversed_by_stock_history_id IS NULL AND is_transfer_in = 1",
			businessId, models.StockReferenceTypeTransferOrder, transferOrderId).
		Find(&inRows).Error; err != nil {
		return 0, nil, err
	}

	// Build a quick existence set for incoming rows.
	// Key includes warehouse + product + batch + reference_detail + qty + unit cost.
	type k struct {
		wh   int
		pid  int
		pt   models.ProductType
		b    string
		rdi  int
		q    string
		uv   string
		date string
	}
	exists := make(map[k]bool, len(inRows))
	for _, r := range inRows {
		if r == nil {
			continue
		}
		exists[k{
			wh:   r.WarehouseId,
			pid:  r.ProductId,
			pt:   r.ProductType,
			b:    r.BatchNumber,
			rdi:  r.ReferenceDetailID,
			q:    r.Qty.String(),
			uv:   r.BaseUnitValue.String(),
			date: r.StockDate.UTC().Format("2006-01-02"),
		}] = true
	}

	toCreate := make([]*models.StockHistory, 0)
	for _, out := range outRows {
		if out == nil {
			continue
		}
		// Only clone true outgoing rows.
		if out.IsOutgoing == nil || !*out.IsOutgoing {
			continue
		}
		if out.IsReversal || out.ReversedByStockHistoryId != nil {
			continue
		}
		// Create transfer-in row at destination warehouse with same valuation.
		in := *out
		in.ID = 0
		in.WarehouseId = to.DestinationWarehouseId
		in.Qty = in.Qty.Abs()
		in.Description = "Transfer In (backfill)"
		in.IsOutgoing = utils.NewFalse()
		in.IsTransferIn = utils.NewTrue()
		in.ClosingQty = decimal.Zero
		in.ClosingAssetValue = decimal.Zero
		in.CumulativeIncomingQty = decimal.Zero
		in.CumulativeOutgoingQty = decimal.Zero
		in.CumulativeSequence = 0
		in.IsReversal = false
		in.ReversesStockHistoryId = nil
		in.ReversedByStockHistoryId = nil
		in.ReversalReason = nil
		in.ReversedAt = nil

		key := k{
			wh:   in.WarehouseId,
			pid:  in.ProductId,
			pt:   in.ProductType,
			b:    in.BatchNumber,
			rdi:  in.ReferenceDetailID,
			q:    in.Qty.String(),
			uv:   in.BaseUnitValue.String(),
			date: in.StockDate.UTC().Format("2006-01-02"),
		}
		if exists[key] {
			continue
		}
		exists[key] = true
		toCreate = append(toCreate, &in)
	}

	if len(toCreate) == 0 {
		return 0, nil, nil
	}

	for _, r := range toCreate {
		if err := tx.Create(r).Error; err != nil {
			return created, affectedAccountIds, err
		}
		created++
	}

	// Ensure FIFO/closing fields are consistent for the created incoming rows.
	if _, err := ProcessIncomingStocks(tx, logger, toCreate); err != nil {
		return created, affectedAccountIds, err
	}

	if logger != nil {
		logger.WithFields(logrus.Fields{
			"business_id":        businessId,
			"transfer_order_id":  transferOrderId,
			"created_in_rows":    created,
			"destination_wh_id":  to.DestinationWarehouseId,
			"reference_type":     models.StockReferenceTypeTransferOrder,
		}).Info("transfer_order.transfer_in.backfilled")
	}

	return created, affectedAccountIds, nil
}

