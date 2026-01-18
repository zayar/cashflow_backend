package workflow

import (
	"fmt"
	"time"

	"github.com/mmdatafocus/books_backend/config"
	"github.com/mmdatafocus/books_backend/models"
	"github.com/mmdatafocus/books_backend/utils"
	"github.com/shopspring/decimal"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

func acquireInventoryRebuildLock(tx *gorm.DB, businessId string, warehouseId int, productId int, productType models.ProductType, batchNumber string) error {
	lockName := fmt.Sprintf("inv_rebuild:%s:%d:%d:%s:%s", businessId, warehouseId, productId, productType, batchNumber)
	var ok int
	if err := tx.Raw("SELECT GET_LOCK(?, 30)", lockName).Scan(&ok).Error; err != nil {
		return err
	}
	if ok != 1 {
		return fmt.Errorf("could not acquire rebuild lock for business_id=%s warehouse_id=%d product_id=%d product_type=%s batch=%s",
			businessId, warehouseId, productId, productType, batchNumber)
	}
	return nil
}

func releaseInventoryRebuildLock(tx *gorm.DB, businessId string, warehouseId int, productId int, productType models.ProductType, batchNumber string) {
	lockName := fmt.Sprintf("inv_rebuild:%s:%d:%d:%s:%s", businessId, warehouseId, productId, productType, batchNumber)
	var _ok int
	_ = tx.Raw("SELECT RELEASE_LOCK(?)", lockName).Scan(&_ok).Error
}

// RebuildInventoryForItemWarehouseFromDate rebuilds valuation/COGS from startDate forward for a single item+warehouse.
// This is used for backdated incoming stock to ensure deterministic FIFO/COGS and remove duplicate valuation rows.
func RebuildInventoryForItemWarehouseFromDate(
	tx *gorm.DB,
	logger *logrus.Logger,
	businessId string,
	warehouseId int,
	productId int,
	productType models.ProductType,
	batchNumber string,
	startDate time.Time,
) ([]int, error) {
	if tx == nil {
		return nil, fmt.Errorf("rebuild inventory: tx is nil")
	}
	if logger == nil {
		logger = config.GetLogger()
	}
	if businessId == "" || warehouseId <= 0 || productId <= 0 {
		return nil, fmt.Errorf("rebuild inventory: invalid scope")
	}

	if err := acquireInventoryRebuildLock(tx, businessId, warehouseId, productId, productType, batchNumber); err != nil {
		return nil, err
	}
	defer releaseInventoryRebuildLock(tx, businessId, warehouseId, productId, productType, batchNumber)

	business, err := models.GetBusinessById2(tx, businessId)
	if err != nil {
		return nil, err
	}
	normalizedStart, err := utils.ConvertToDate(startDate, business.Timezone)
	if err != nil {
		return nil, err
	}

	if logger != nil {
		logger.WithFields(logrus.Fields{
			"business_id":  businessId,
			"warehouse_id": warehouseId,
			"product_id":   productId,
			"product_type": productType,
			"batch_number": batchNumber,
			"start_date":   normalizedStart.Format(time.RFC3339),
		}).Info("inv.rebuild.start")
	}

	var beforeOutgoingCount int64
	_ = tx.Model(&models.StockHistory{}).
		Where("business_id = ? AND warehouse_id = ? AND product_id = ? AND product_type = ? AND batch_number = ? AND is_outgoing = 1 AND stock_date >= ? AND is_reversal = 0 AND reversed_by_stock_history_id IS NULL",
			businessId, warehouseId, productId, productType, batchNumber, normalizedStart).
		Count(&beforeOutgoingCount).Error

	// Find last outgoing cumulative qty before startDate to seed FIFO.
	lastCumulativeOutgoingQty := decimal.Zero
	var lastOut models.StockHistory
	err = tx.
		Where("business_id = ? AND warehouse_id = ? AND product_id = ? AND product_type = ? AND batch_number = ? AND is_outgoing = 1 AND stock_date < ? AND is_reversal = 0 AND reversed_by_stock_history_id IS NULL",
			businessId, warehouseId, productId, productType, batchNumber, normalizedStart).
		Order("stock_date DESC, is_outgoing DESC, id DESC").
		Limit(1).
		Find(&lastOut).Error
	if err != nil && err != gorm.ErrRecordNotFound {
		return nil, err
	}
	if lastOut.ID > 0 {
		lastCumulativeOutgoingQty = lastOut.CumulativeOutgoingQty
	}

	incoming, err := GetRemainingStockHistoriesByCumulativeQty(
		tx, warehouseId, productId, string(productType), batchNumber, utils.NewFalse(), lastCumulativeOutgoingQty,
	)
	if err != nil {
		return nil, err
	}
	outgoing, err := GetRemainingStockHistoriesByDate(
		tx, warehouseId, productId, string(productType), batchNumber, utils.NewTrue(), normalizedStart,
	)
	if err != nil {
		return nil, err
	}
	incoming, outgoing = FilterStockHistories(incoming, outgoing)

	if logger != nil {
		logger.WithFields(logrus.Fields{
			"business_id":               businessId,
			"warehouse_id":              warehouseId,
			"product_id":                productId,
			"product_type":              productType,
			"batch_number":              batchNumber,
			"start_date":                normalizedStart.Format(time.RFC3339),
			"incoming_count":            len(incoming),
			"outgoing_count":            len(outgoing),
			"prior_cum_outgoing_qty":    lastCumulativeOutgoingQty.String(),
			"outgoing_active_before":    beforeOutgoingCount,
		}).Info("inv.rebuild.source_count")
	}

	accountIds := make([]int, 0)
	if len(outgoing) > 0 {
		productDetail, err := GetProductDetail(tx, productId, productType)
		if err != nil {
			return nil, err
		}
		startCurrentQty := decimal.Zero
		if len(incoming) > 0 {
			startCurrentQty = incoming[0].CumulativeIncomingQty.Sub(lastCumulativeOutgoingQty)
			if startCurrentQty.IsNegative() {
				startCurrentQty = decimal.Zero
			}
		}
		accountIds, err = calculateCogs(tx, logger, productDetail, decimal.Zero, startCurrentQty, incoming, outgoing, 0, "")
		if err != nil {
			return nil, err
		}
	}

	combined := make([]*models.StockHistory, 0, len(incoming)+len(outgoing))
	combined = append(combined, incoming...)
	combined = append(combined, outgoing...)
	if len(combined) > 0 {
		lastAll, err := getLastStockHistories(tx, combined, true)
		if err != nil {
			return nil, err
		}
		if err := models.UpdateStockClosingBalances(tx, combined, lastAll); err != nil {
			return nil, err
		}
	}

	var afterOutgoingCount int64
	_ = tx.Model(&models.StockHistory{}).
		Where("business_id = ? AND warehouse_id = ? AND product_id = ? AND product_type = ? AND batch_number = ? AND is_outgoing = 1 AND stock_date >= ? AND is_reversal = 0 AND reversed_by_stock_history_id IS NULL",
			businessId, warehouseId, productId, productType, batchNumber, normalizedStart).
		Count(&afterOutgoingCount).Error

	type totals struct {
		Qty        decimal.Decimal
		AssetValue decimal.Decimal
	}
	var t totals
	_ = tx.Raw(`
		SELECT
			COALESCE(SUM(qty), 0) AS qty,
			COALESCE(SUM(qty * base_unit_value), 0) AS asset_value
		FROM stock_histories
		WHERE business_id = ? AND warehouse_id = ? AND product_id = ? AND product_type = ? AND batch_number = ?
			AND is_reversal = 0 AND reversed_by_stock_history_id IS NULL
	`, businessId, warehouseId, productId, productType, batchNumber).Scan(&t).Error

	if logger != nil {
		logger.WithFields(logrus.Fields{
			"business_id":            businessId,
			"warehouse_id":           warehouseId,
			"product_id":             productId,
			"product_type":           productType,
			"batch_number":           batchNumber,
			"start_date":             normalizedStart.Format(time.RFC3339),
			"outgoing_active_before": beforeOutgoingCount,
			"outgoing_active_after":  afterOutgoingCount,
			"final_qty":              t.Qty.String(),
			"final_asset_value":      t.AssetValue.String(),
		}).Info("inv.rebuild.end")
	}

	return accountIds, nil
}
