package workflow

import (
	"fmt"
	"os"
	"strings"

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

// SyncTransferOrderTransferInFromOutgoing keeps destination transfer-in rows aligned
// to the latest transfer-out valuation layers (qty + unit cost) for a transfer order.
//
// This is used when FIFO/valuation is recalculated after backdated incoming edits.
// When the source-side cost layers change, the destination layers must be updated too.
func SyncTransferOrderTransferInFromOutgoing(
	tx *gorm.DB,
	logger *logrus.Logger,
	businessId string,
	transferOrderId int,
	allowCreate bool,
) (bool, error) {
	if tx == nil {
		return false, fmt.Errorf("transfer order sync: tx is nil")
	}
	if businessId == "" || transferOrderId <= 0 {
		return false, fmt.Errorf("transfer order sync: invalid args")
	}

	// Load transfer order for destination warehouse.
	var to models.TransferOrder
	if err := tx.
		Where("business_id = ? AND id = ?", businessId, transferOrderId).
		First(&to).Error; err != nil {
		return false, err
	}
	if to.DestinationWarehouseId <= 0 {
		return false, nil
	}

	// Fetch active outgoing rows for this transfer (source side).
	var outRows []*models.StockHistory
	if err := tx.
		Where("business_id = ? AND reference_type = ? AND reference_id = ? AND is_reversal = 0 AND reversed_by_stock_history_id IS NULL AND is_outgoing = 1 AND (is_transfer_in = 0 OR is_transfer_in IS NULL)",
			businessId, models.StockReferenceTypeTransferOrder, transferOrderId).
		Find(&outRows).Error; err != nil {
		return false, err
	}
	if len(outRows) == 0 {
		return false, nil
	}

	// Fetch existing active incoming rows for this transfer (destination side).
	var inRows []*models.StockHistory
	if err := tx.
		Where("business_id = ? AND reference_type = ? AND reference_id = ? AND is_reversal = 0 AND reversed_by_stock_history_id IS NULL AND is_transfer_in = 1",
			businessId, models.StockReferenceTypeTransferOrder, transferOrderId).
		Find(&inRows).Error; err != nil {
		return false, err
	}
	if len(inRows) == 0 && !allowCreate {
		// Avoid duplicating rows during initial transfer posting (destination rows created later).
		return false, nil
	}

	type key struct {
		wh     int
		pid    int
		pt     models.ProductType
		b      string
		rdi    int
		q      string
		uv     string
		date   string
		refTyp models.StockReferenceType
		refId  int
	}

	desiredCounts := make(map[key]int)
	desiredTemplate := make(map[key]*models.StockHistory)
	for _, out := range outRows {
		if out == nil {
			continue
		}
		if out.IsOutgoing == nil || !*out.IsOutgoing {
			continue
		}
		if out.IsReversal || out.ReversedByStockHistoryId != nil {
			continue
		}
		k := key{
			wh:     to.DestinationWarehouseId,
			pid:    out.ProductId,
			pt:     out.ProductType,
			b:      out.BatchNumber,
			rdi:    out.ReferenceDetailID,
			q:      out.Qty.Abs().String(),
			uv:     out.BaseUnitValue.String(),
			date:   out.StockDate.UTC().Format("2006-01-02"),
			refTyp: out.ReferenceType,
			refId:  out.ReferenceID,
		}
		desiredCounts[k]++
		if _, ok := desiredTemplate[k]; !ok {
			in := *out
			in.ID = 0
			in.WarehouseId = to.DestinationWarehouseId
			in.Qty = in.Qty.Abs()
			in.Description = "Transfer In"
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
			desiredTemplate[k] = &in
		}
	}

	toReverse := make([]*models.StockHistory, 0)
	for _, in := range inRows {
		if in == nil {
			continue
		}
		k := key{
			wh:     in.WarehouseId,
			pid:    in.ProductId,
			pt:     in.ProductType,
			b:      in.BatchNumber,
			rdi:    in.ReferenceDetailID,
			q:      in.Qty.String(),
			uv:     in.BaseUnitValue.String(),
			date:   in.StockDate.UTC().Format("2006-01-02"),
			refTyp: in.ReferenceType,
			refId:  in.ReferenceID,
		}
		if c := desiredCounts[k]; c > 0 {
			desiredCounts[k] = c - 1
			continue
		}
		toReverse = append(toReverse, in)
	}

	changed := false
	if len(toReverse) > 0 {
		if _, err := ReverseStockHistories(tx, toReverse, ReversalReasonInventoryValuationReprice); err != nil {
			return false, err
		}
		changed = true
	}

	newRows := make([]*models.StockHistory, 0)
	for k, count := range desiredCounts {
		if count <= 0 {
			continue
		}
		template := desiredTemplate[k]
		if template == nil {
			continue
		}
		for i := 0; i < count; i++ {
			row := *template
			row.ID = 0
			if err := tx.Create(&row).Error; err != nil {
				return false, err
			}
			newRows = append(newRows, &row)
		}
	}

	if len(newRows) > 0 {
		changed = true
		if _, err := ProcessIncomingStocks(tx, logger, newRows); err != nil {
			return changed, err
		}
	} else if len(toReverse) > 0 {
		// Ensure closing balances are updated when only reversals occurred.
		if _, err := ProcessStockHistories(tx, logger, toReverse); err != nil {
			return changed, err
		}
	}

	debug := strings.EqualFold(strings.TrimSpace(os.Getenv("DEBUG_TRANSFER_ORDER")), "true")
	if changed && logger != nil && debug {
		logger.WithFields(logrus.Fields{
			"business_id":        businessId,
			"transfer_order_id":  transferOrderId,
			"destination_wh_id":  to.DestinationWarehouseId,
			"created_in_rows":    len(newRows),
			"reversed_in_rows":   len(toReverse),
			"reference_type":     models.StockReferenceTypeTransferOrder,
		}).Info("transfer_order.transfer_in.synced")
	}

	return changed, nil
}

