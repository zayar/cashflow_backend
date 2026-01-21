package workflow

import (
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/mmdatafocus/books_backend/models"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

// Shared helpers to self-heal "insufficient FIFO layers" by running a targeted inventory rebuild.
// Used by Transfer Orders and Inventory Adjustments.

type fifoScope struct {
	warehouseId int
	productId   int
	productType models.ProductType
	batch       string
}

func parseFifoInsufficientScope(err error) (fifoScope, bool) {
	if err == nil {
		return fifoScope{}, false
	}
	msg := err.Error()
	if !strings.Contains(msg, "insufficient FIFO layers for") {
		return fifoScope{}, false
	}
	// Example:
	// "insufficient FIFO layers for product_id=106 product_type=S warehouse_id=25 batch= qty_missing=4"
	getInt := func(key string) (int, bool) {
		i := strings.Index(msg, key)
		if i < 0 {
			return 0, false
		}
		start := i + len(key)
		end := start
		for end < len(msg) && msg[end] >= '0' && msg[end] <= '9' {
			end++
		}
		if end == start {
			return 0, false
		}
		v, convErr := strconv.Atoi(msg[start:end])
		return v, convErr == nil
	}
	getStrToken := func(key string) (string, bool) {
		i := strings.Index(msg, key)
		if i < 0 {
			return "", false
		}
		start := i + len(key)
		end := start
		for end < len(msg) && msg[end] != ' ' && msg[end] != ',' {
			end++
		}
		return msg[start:end], true
	}

	pid, ok1 := getInt("product_id=")
	wid, ok2 := getInt("warehouse_id=")
	ptStr, ok3 := getStrToken("product_type=")
	batch, _ := getStrToken("batch=")
	if !ok1 || !ok2 || !ok3 || pid <= 0 || wid <= 0 || ptStr == "" {
		return fifoScope{}, false
	}
	return fifoScope{warehouseId: wid, productId: pid, productType: models.ProductType(ptStr), batch: batch}, true
}

func rebuildInventoryForScope(tx *gorm.DB, logger *logrus.Logger, businessId string, scope fifoScope, fallbackStart time.Time) error {
	if tx == nil {
		return errors.New("rebuild inventory: tx is nil")
	}
	if businessId == "" || scope.warehouseId <= 0 || scope.productId <= 0 {
		return errors.New("rebuild inventory: invalid scope")
	}
	start := fallbackStart
	type row struct{ Start time.Time }
	var r row
	_ = tx.Raw(`
		SELECT COALESCE(MIN(stock_date), ?) AS start
		FROM stock_histories
		WHERE business_id = ?
		  AND warehouse_id = ?
		  AND product_id = ?
		  AND product_type = ?
		  AND COALESCE(batch_number,'') = ?
		  AND is_reversal = 0
		  AND reversed_by_stock_history_id IS NULL
	`, fallbackStart, businessId, scope.warehouseId, scope.productId, scope.productType, scope.batch).Scan(&r).Error
	if !r.Start.IsZero() {
		start = r.Start
	}
	_, err := RebuildInventoryForItemWarehouseFromDate(
		tx, logger, businessId, scope.warehouseId, scope.productId, scope.productType, scope.batch, start,
	)
	return err
}

