package workflow

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/mmdatafocus/books_backend/config"
	"github.com/mmdatafocus/books_backend/models"
	"github.com/mmdatafocus/books_backend/utils"
	"github.com/shopspring/decimal"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

type TransactionRecord struct {
	Id                 int
	Type               string
	PurchaseAccountId  int
	InventoryAccountId int
	DetailStocks       []StockRecord
	TotalOldCogs       decimal.Decimal
	TotalCogs          decimal.Decimal
}

type StockRecord struct {
	Id        int
	Type      string
	Date      time.Time
	DetailId  int
	DetailQty decimal.Decimal
	OldCogs   decimal.Decimal
	Cogs      decimal.Decimal
}

type StockDetail struct {
	Id        int
	Date      time.Time
	DetailId  int
	DetailQty decimal.Decimal
	Cogs      decimal.Decimal
}

type ProductDetail struct {
	Id                 int
	Type               string
	InventoryAccountId int
	PurchaseAccountId  int
	PurchasePrice      decimal.Decimal
}

type StockFragment struct {
	WarehouseId  int
	ProductId    int
	ProductType  models.ProductType
	BatchNumber  string
	ReceivedDate time.Time
}

type StockHistoryFragment struct {
	BusinessId        string
	WarehouseId       int
	ProductId         int
	ProductType       models.ProductType
	BatchNumber       string
	ReferenceId       int
	ReferenceType     models.StockReferenceType
	ReferenceDetailId int
	TotalQty          decimal.Decimal
	TotalValue        decimal.Decimal
}

type StockHistoryDetailFragment struct {
	Id                int
	BusinessId        string
	WarehouseId       int
	ProductId         int
	ProductType       models.ProductType
	BatchNumber       string
	StockDate         time.Time
	Description       string
	ReferenceId       int
	ReferenceType     models.StockReferenceType
	ReferenceDetailId int
	Qty               decimal.Decimal
	BaseUnitValue     decimal.Decimal
}

// func mergeStockFragments(slice1, slice2 []*StockFragment) []*StockFragment {
// 	uniqueStockFragments := make(map[string]*StockFragment)

// 	// Helper function to create a unique key for each StockFragment
// 	makeKey := func(s StockFragment) string {
// 		return fmt.Sprintf("%d-%d-%s-%s", s.WarehouseId, s.ProductId, s.ProductType, s.BatchNumber)
// 	}

// 	// Add or update elements from the first slice to the map
// 	for _, stock := range slice1 {
// 		key := makeKey(*stock)
// 		if existing, found := uniqueStockFragments[key]; found {
// 			// Keep the one with the older received date
// 			if stock.ReceivedDate.Before(existing.ReceivedDate) {
// 				uniqueStockFragments[key] = stock
// 			}
// 		} else {
// 			uniqueStockFragments[key] = stock
// 		}
// 	}

// 	// Add or update elements from the second slice to the map
// 	for _, stock := range slice2 {
// 		key := makeKey(*stock)
// 		if existing, found := uniqueStockFragments[key]; found {
// 			// Keep the one with the older received date
// 			if stock.ReceivedDate.Before(existing.ReceivedDate) {
// 				uniqueStockFragments[key] = stock
// 			}
// 		} else {
// 			uniqueStockFragments[key] = stock
// 		}
// 	}

// 	// Convert the map back to a slice
// 	result := make([]*StockFragment, 0, len(uniqueStockFragments))
// 	for _, stock := range uniqueStockFragments {
// 		result = append(result, stock)
// 	}

// 	return result
// }

func mergeStockHistories(slice1, slice2 []*models.StockHistory) []*models.StockHistory {
	// Use a map to avoid duplicates and keep the one with the older stock date
	uniqueMap := make(map[string]*models.StockHistory)

	// Helper function to create a unique key for each StockHistory
	createKey := func(sh *models.StockHistory) string {
		return fmt.Sprintf("%d-%d-%s-%s-%d-%s-%d",
			sh.WarehouseId, sh.ProductId, sh.ProductType, sh.BatchNumber, sh.ReferenceID, sh.ReferenceType, sh.ReferenceDetailID)
	}

	// Add elements from the first slice to the map
	for _, sh := range slice1 {
		key := createKey(sh)
		uniqueMap[key] = sh
	}

	// Add elements from the second slice to the map, keeping the older stock date
	for _, sh := range slice2 {
		key := createKey(sh)
		if existing, found := uniqueMap[key]; found {
			if sh.StockDate.Before(existing.StockDate) {
				uniqueMap[key] = sh
			}
		} else {
			uniqueMap[key] = sh
		}
	}

	// Convert the map back to a slice
	mergedSlice := make([]*models.StockHistory, 0, len(uniqueMap))
	for _, sh := range uniqueMap {
		mergedSlice = append(mergedSlice, sh)
	}

	return mergedSlice
}

// NormalizeDate sets the time components (hour, minute, second, nanosecond) to zero.
// func NormalizeDate(t time.Time) time.Time {
// 	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
// }

// Get last stock histories before the newly added stock histories
func getLastStockHistories(tx *gorm.DB, stockHistories []*models.StockHistory, isAll bool) ([]*models.StockHistory, error) {
	var lastStockHistories []*models.StockHistory
	var err error

	if len(stockHistories) <= 0 {
		return lastStockHistories, nil
	}
	businessId := stockHistories[0].BusinessId

	// Extract the composite keys from the stockHistories slice
	type CompositeKey struct {
		ProductId   int
		ProductType string
		BatchNumber string
	}
	var compositeKeys []CompositeKey
	for _, sh := range stockHistories {
		compositeKeys = append(compositeKeys, CompositeKey{
			ProductId:   sh.ProductId,
			ProductType: string(sh.ProductType),
			BatchNumber: sh.BatchNumber,
		})
	}

	// Create a list of placeholders and parameters for the IN clause
	var placeholders []string
	var params []interface{}
	for _, key := range compositeKeys {
		placeholders = append(placeholders, "(?, ?, ?)")
		params = append(params, key.ProductId, key.ProductType, key.BatchNumber)
	}
	placeholdersStr := strings.Join(placeholders, ",")

	if isAll {
		err = tx.Raw(`
			WITH LastStockHistories AS (
				SELECT 
					*,
					ROW_NUMBER() OVER (PARTITION BY warehouse_id, product_id, product_type, batch_number ORDER BY stock_date DESC, is_outgoing DESC, id DESC) AS rn
				FROM 
					stock_histories
				WHERE 
					business_id = ?
					AND warehouse_id = ? 
					AND stock_date < ? 
					AND (product_id, product_type, batch_number) IN (`+placeholdersStr+`)
			)
			SELECT 
				*
			FROM 
				LastStockHistories
			WHERE 
				rn = 1;
			`, append([]interface{}{businessId, stockHistories[0].WarehouseId, stockHistories[0].StockDate}, params...)...).Find(&lastStockHistories).Error
	} else {
		err = tx.Raw(`
		WITH LastStockHistories AS (
			SELECT 
				*,
				ROW_NUMBER() OVER (PARTITION BY warehouse_id, product_id, product_type, batch_number ORDER BY stock_date DESC, is_outgoing DESC, id DESC) AS rn
			FROM 
				stock_histories
			WHERE 
				business_id = ?
				AND warehouse_id = ? 
				AND stock_date < ? 
				AND is_outgoing = ?
				AND (product_id, product_type, batch_number) IN (`+placeholdersStr+`)
		)
		SELECT 
			*
		FROM 
			LastStockHistories
		WHERE 
			rn = 1;
		`, append([]interface{}{businessId, stockHistories[0].WarehouseId, stockHistories[0].StockDate, stockHistories[0].IsOutgoing}, params...)...).Find(&lastStockHistories).Error
	}

	return lastStockHistories, err
}

// Get last stock histories before value-adjustment stock histories
func getLastStockHistoriesForValueAdjustment(tx *gorm.DB, stockHistories []*models.StockHistory, isAll bool) ([]*models.StockHistory, error) {
	var lastStockHistories []*models.StockHistory
	var err error

	if len(stockHistories) <= 0 {
		return lastStockHistories, nil
	}
	businessId := stockHistories[0].BusinessId

	// Extract the composite keys from the stockHistories slice
	type CompositeKey struct {
		ProductId   int
		ProductType string
		BatchNumber string
	}
	var compositeKeys []CompositeKey
	for _, sh := range stockHistories {
		compositeKeys = append(compositeKeys, CompositeKey{
			ProductId:   sh.ProductId,
			ProductType: string(sh.ProductType),
			BatchNumber: sh.BatchNumber,
		})
	}

	// Create a list of placeholders and parameters for the IN clause
	var placeholders []string
	var params []interface{}
	for _, key := range compositeKeys {
		placeholders = append(placeholders, "(?, ?, ?)")
		params = append(params, key.ProductId, key.ProductType, key.BatchNumber)
	}
	placeholdersStr := strings.Join(placeholders, ",")

	if isAll {
		err = tx.Raw(`
			WITH LastStockHistories AS (
				SELECT 
					*,
					ROW_NUMBER() OVER (PARTITION BY warehouse_id, product_id, product_type, batch_number ORDER BY stock_date DESC, is_outgoing DESC, id DESC) AS rn
				FROM 
					stock_histories
				WHERE 
					business_id = ?
					AND warehouse_id = ? 
					AND stock_date < ? 
					AND (product_id, product_type, batch_number) IN (`+placeholdersStr+`)
			)
			SELECT 
				*
			FROM 
				LastStockHistories
			WHERE 
				rn = 1;
			`, append([]interface{}{businessId, stockHistories[0].WarehouseId, stockHistories[0].StockDate}, params...)...).Find(&lastStockHistories).Error
	} else {
		err = tx.Raw(`
		WITH LastStockHistories AS (
			SELECT 
				*,
				ROW_NUMBER() OVER (PARTITION BY warehouse_id, product_id, product_type, batch_number ORDER BY stock_date DESC, is_outgoing DESC, id DESC) AS rn
			FROM 
				stock_histories
			WHERE 
				business_id = ?
				AND warehouse_id = ? 
				AND stock_date <= ? 
				AND reference_type != 'IVAV'
				AND is_outgoing = ?
				AND (product_id, product_type, batch_number) IN (`+placeholdersStr+`)
		)
		SELECT 
			*
		FROM 
			LastStockHistories
		WHERE 
			rn = 1;
		`, append([]interface{}{businessId, stockHistories[0].WarehouseId, stockHistories[0].StockDate, stockHistories[0].IsOutgoing}, params...)...).Find(&lastStockHistories).Error
	}

	return lastStockHistories, err
}

// Get Remaining StockHistories after fulfiling the given qty
func GetRemainingStockHistoriesByCumulativeQty(tx *gorm.DB, warehouseId int, productId int, productType string, batchNumber string, isOutgoing *bool, qty decimal.Decimal) ([]*models.StockHistory, error) {
	var remaininigStockHistories []*models.StockHistory
	var err error
	if isOutgoing != nil && *isOutgoing {
		err = tx.Raw(`
			SELECT * FROM stock_histories
			WHERE business_id = (SELECT business_id FROM warehouses WHERE id = ? LIMIT 1)
				AND warehouse_id = ? AND product_id = ? AND product_type = ? AND COALESCE(batch_number,'') = ? AND is_outgoing = true AND cumulative_outgoing_qty > ?
				AND is_reversal = 0 AND reversed_by_stock_history_id IS NULL
			ORDER BY stock_date, is_outgoing, id ASC;
			`, warehouseId, warehouseId, productId, productType, batchNumber, qty).Find(&remaininigStockHistories).Error
	} else {
		// For incoming stock histories, if qty is 0 (no outgoing stock yet), we need to include ALL incoming stock
		// including opening stock which might have cumulative_incoming_qty = 0 if it's the first entry.
		// Use >= 0 when qty is 0 to ensure opening stock is found.
		if qty.IsZero() {
			err = tx.Raw(`
				SELECT * FROM stock_histories
				WHERE business_id = (SELECT business_id FROM warehouses WHERE id = ? LIMIT 1)
					AND warehouse_id = ? AND product_id = ? AND product_type = ? AND COALESCE(batch_number,'') = ? AND is_outgoing = false AND cumulative_incoming_qty >= ?
					AND is_reversal = 0 AND reversed_by_stock_history_id IS NULL
				ORDER BY stock_date, is_outgoing, id ASC;
				`, warehouseId, warehouseId, productId, productType, batchNumber, qty).Find(&remaininigStockHistories).Error
		} else {
			err = tx.Raw(`
				SELECT * FROM stock_histories
				WHERE business_id = (SELECT business_id FROM warehouses WHERE id = ? LIMIT 1)
					AND warehouse_id = ? AND product_id = ? AND product_type = ? AND COALESCE(batch_number,'') = ? AND is_outgoing = false AND cumulative_incoming_qty > ?
					AND is_reversal = 0 AND reversed_by_stock_history_id IS NULL
				ORDER BY stock_date, is_outgoing, id ASC;
				`, warehouseId, warehouseId, productId, productType, batchNumber, qty).Find(&remaininigStockHistories).Error
		}
	}

	return remaininigStockHistories, err
}

// Get Remaining StockHistories after fulfiling the given qty
func GetRemainingStockHistoriesByCumulativeQtyUntilDate(tx *gorm.DB, warehouseId int, productId int, productType string, batchNumber string, isOutgoing *bool, qty decimal.Decimal, stockDate time.Time) ([]*models.StockHistory, error) {
	// date := NormalizeDate(stockDate).Add(24 * time.Hour)
	var remaininigStockHistories []*models.StockHistory
	var err error
	if isOutgoing != nil && *isOutgoing {
		err = tx.Raw(`
			SELECT * FROM stock_histories
			WHERE business_id = (SELECT business_id FROM warehouses WHERE id = ? LIMIT 1)
				AND warehouse_id = ? AND product_id = ? AND product_type = ? AND COALESCE(batch_number,'') = ? AND is_outgoing = true AND cumulative_outgoing_qty > ? AND stock_date <= ?
				AND is_reversal = 0 AND reversed_by_stock_history_id IS NULL
			ORDER BY stock_date, is_outgoing, id ASC;
			`, warehouseId, warehouseId, productId, productType, batchNumber, qty, stockDate).Find(&remaininigStockHistories).Error
	} else {
		err = tx.Raw(`
			SELECT * FROM stock_histories
			WHERE business_id = (SELECT business_id FROM warehouses WHERE id = ? LIMIT 1)
				AND warehouse_id = ? AND product_id = ? AND product_type = ? AND COALESCE(batch_number,'') = ? AND is_outgoing = false AND cumulative_incoming_qty > ? AND stock_date <= ?
				AND is_reversal = 0 AND reversed_by_stock_history_id IS NULL
			ORDER BY stock_date, is_outgoing, id ASC;
			`, warehouseId, warehouseId, productId, productType, batchNumber, qty, stockDate).Find(&remaininigStockHistories).Error
	}

	return remaininigStockHistories, err
}

// Get Remaining StockHistories equal or after the given date
func GetRemainingStockHistoriesByDate(tx *gorm.DB, warehouseId int, productId int, productType string, batchNumber string, isOutgoing *bool, stockDate time.Time) ([]*models.StockHistory, error) {
	var remaininigStockHistories []*models.StockHistory
	var err error
	if isOutgoing != nil && *isOutgoing {
		err = tx.Raw(`
			SELECT * FROM stock_histories
			WHERE business_id = (SELECT business_id FROM warehouses WHERE id = ? LIMIT 1)
				AND warehouse_id = ? AND product_id = ? AND product_type = ? AND COALESCE(batch_number,'') = ? AND is_outgoing = true AND stock_date >= ?
				AND is_reversal = 0 AND reversed_by_stock_history_id IS NULL
			ORDER BY stock_date, is_outgoing, id ASC;
			`, warehouseId, warehouseId, productId, productType, batchNumber, stockDate).Find(&remaininigStockHistories).Error
	} else {
		err = tx.Raw(`
			SELECT * FROM stock_histories
			WHERE business_id = (SELECT business_id FROM warehouses WHERE id = ? LIMIT 1)
				AND warehouse_id = ? AND product_id = ? AND product_type = ? AND COALESCE(batch_number,'') = ? AND is_outgoing = false AND stock_date >= ?
				AND is_reversal = 0 AND reversed_by_stock_history_id IS NULL
			ORDER BY stock_date, is_outgoing, id ASC;
			`, warehouseId, warehouseId, productId, productType, batchNumber, stockDate).Find(&remaininigStockHistories).Error
	}

	return remaininigStockHistories, err
}

// Calculate CostOfGoodsSold
func calculateCogs(tx *gorm.DB, logger *logrus.Logger, productDetail ProductDetail, startProcessQty decimal.Decimal, startCurrentQty decimal.Decimal, incomingStockHistories []*models.StockHistory, outgoingStockHistories []*models.StockHistory, updatedReferenceId int, updatedReferenceType models.StockReferenceType) ([]int, error) {
	accountIds := make([]int, 0)
	var err error

	if len(outgoingStockHistories) <= 0 {
		return accountIds, err
	}

	// Get the oldest stock
	currentStock := &models.StockHistory{}
	if len(incomingStockHistories) > 0 {
		currentStock = incomingStockHistories[0]
		if !startCurrentQty.IsZero() {
			currentStock.Qty = startCurrentQty
		}
	}

	uniqueStocks := make(map[string]StockHistoryFragment)
	uniqueStockDetails := make(map[string]StockHistoryDetailFragment)
	existingStocks := make(map[string]StockHistoryFragment)
	existingStockDetails := make(map[string]StockHistoryDetailFragment)

	// Query existing stock histories for references in scope so we can:
	// - compute deltas for COGS/journal reposts
	// - reverse stale valuation rows that are no longer valid
	if len(outgoingStockHistories) > 0 {
		var existingRefStockHistories []*models.StockHistory
		firstOutStock := outgoingStockHistories[0]

		if updatedReferenceId > 0 && updatedReferenceType != "" {
			// Normal path: only scope to the updated reference.
			err = tx.
				Where("business_id = ? AND reference_type = ? AND reference_id = ? AND warehouse_id = ? AND product_id = ? AND product_type = ? AND COALESCE(batch_number,'') = ? AND is_outgoing = 1 AND is_reversal = 0 AND reversed_by_stock_history_id IS NULL",
					firstOutStock.BusinessId, updatedReferenceType, updatedReferenceId, firstOutStock.WarehouseId, firstOutStock.ProductId, firstOutStock.ProductType, firstOutStock.BatchNumber).
				Find(&existingRefStockHistories).Error
			if err != nil {
				config.LogError(logger, "MainWorkflow.go", "CalculateCogs", "QueryExistingRefStockHistories", updatedReferenceId, err)
				return accountIds, err
			}
		} else {
			// Rebuild path (backdated incoming): include ALL outgoing references in scope.
			refIdsByType := make(map[models.StockReferenceType][]int)
			refSeen := make(map[models.StockReferenceType]map[int]struct{})
			for _, outStock := range outgoingStockHistories {
				if outStock == nil {
					continue
				}
				if _, ok := refSeen[outStock.ReferenceType]; !ok {
					refSeen[outStock.ReferenceType] = make(map[int]struct{})
				}
				if _, ok := refSeen[outStock.ReferenceType][outStock.ReferenceID]; ok {
					continue
				}
				refSeen[outStock.ReferenceType][outStock.ReferenceID] = struct{}{}
				refIdsByType[outStock.ReferenceType] = append(refIdsByType[outStock.ReferenceType], outStock.ReferenceID)
			}
			for refType, refIds := range refIdsByType {
				var rows []*models.StockHistory
				err = tx.
					Where("business_id = ? AND reference_type = ? AND reference_id IN ? AND warehouse_id = ? AND product_id = ? AND product_type = ? AND COALESCE(batch_number,'') = ? AND is_outgoing = 1 AND is_reversal = 0 AND reversed_by_stock_history_id IS NULL",
						firstOutStock.BusinessId, refType, refIds, firstOutStock.WarehouseId, firstOutStock.ProductId, firstOutStock.ProductType, firstOutStock.BatchNumber).
					Find(&rows).Error
				if err != nil {
					config.LogError(logger, "MainWorkflow.go", "CalculateCogs", "QueryExistingRefStockHistoriesAll", refType, err)
					return accountIds, err
				}
				if len(rows) > 0 {
					existingRefStockHistories = append(existingRefStockHistories, rows...)
				}
			}
		}

		for _, refStock := range existingRefStockHistories {
			key := fmt.Sprintf("%d-%d-%s-%s-%d-%s-%d", refStock.WarehouseId, refStock.ProductId, refStock.ProductType, refStock.BatchNumber, refStock.ReferenceID, refStock.ReferenceType, refStock.ReferenceDetailID)
			if existing, found := existingStocks[key]; found {
				existing.TotalQty = existing.TotalQty.Add(refStock.Qty.Abs())
				existing.TotalValue = existing.TotalValue.Add(refStock.Qty.Abs().Mul(refStock.BaseUnitValue))
				existingStocks[key] = existing
			} else {
				existingStocks[key] = StockHistoryFragment{
					BusinessId:        refStock.BusinessId,
					WarehouseId:       refStock.WarehouseId,
					ProductId:         refStock.ProductId,
					ProductType:       refStock.ProductType,
					BatchNumber:       refStock.BatchNumber,
					ReferenceId:       refStock.ReferenceID,
					ReferenceType:     refStock.ReferenceType,
					ReferenceDetailId: refStock.ReferenceDetailID,
					TotalQty:          refStock.Qty.Abs(),
					TotalValue:        refStock.Qty.Abs().Mul(refStock.BaseUnitValue),
				}
			}
			detailKey := fmt.Sprintf("%d-%d-%s-%s-%d-%s-%d-%s", refStock.WarehouseId, refStock.ProductId, refStock.ProductType, refStock.BatchNumber, refStock.ReferenceID, refStock.ReferenceType, refStock.ReferenceDetailID, refStock.BaseUnitValue)
			if existing, found := existingStockDetails[detailKey]; found {
				existing.Qty = existing.Qty.Add(refStock.Qty.Abs())
				existingStockDetails[detailKey] = existing
			} else {
				existingStockDetails[detailKey] = StockHistoryDetailFragment{
					Id:                refStock.ID,
					BusinessId:        refStock.BusinessId,
					WarehouseId:       refStock.WarehouseId,
					ProductId:         refStock.ProductId,
					ProductType:       refStock.ProductType,
					BatchNumber:       refStock.BatchNumber,
					StockDate:         refStock.StockDate,
					Description:       refStock.Description,
					ReferenceId:       refStock.ReferenceID,
					ReferenceType:     refStock.ReferenceType,
					ReferenceDetailId: refStock.ReferenceDetailID,
					Qty:               refStock.Qty.Abs(),
					BaseUnitValue:     refStock.BaseUnitValue,
				}
			}
		}
	}

	processQty := outgoingStockHistories[0].Qty.Abs()
	if !startProcessQty.IsZero() {
		processQty = startProcessQty
	}

	for outIndex, outStock := range outgoingStockHistories {
		// Process ALL outgoingStockHistories in the FIFO loop below for correct inventory
		// consumption tracking. The existingStocks map was already populated from DB query
		// above, so we don't need to build it from outgoingStockHistories here.
		if outIndex == 0 && !processQty.Equal(outgoingStockHistories[0].Qty.Abs()) {
			uniqueStocks[fmt.Sprintf("%d-%d-%s-%s-%d-%s-%d", outStock.WarehouseId, outStock.ProductId, outStock.ProductType, outStock.BatchNumber, outStock.ReferenceID, outStock.ReferenceType, outStock.ReferenceDetailID)] = StockHistoryFragment{
				BusinessId:        outStock.BusinessId,
				WarehouseId:       outStock.WarehouseId,
				ProductId:         outStock.ProductId,
				ProductType:       outStock.ProductType,
				BatchNumber:       outStock.BatchNumber,
				ReferenceId:       outStock.ReferenceID,
				ReferenceType:     outStock.ReferenceType,
				ReferenceDetailId: outStock.ReferenceDetailID,
				TotalQty:          outgoingStockHistories[0].Qty.Abs().Sub(processQty),
				TotalValue:        outgoingStockHistories[0].Qty.Abs().Sub(processQty).Mul(currentStock.BaseUnitValue),
			}
			uniqueStockDetails[fmt.Sprintf("%d-%d-%s-%s-%d-%s-%d-%s", outStock.WarehouseId, outStock.ProductId, outStock.ProductType, outStock.BatchNumber, outStock.ReferenceID, outStock.ReferenceType, outStock.ReferenceDetailID, outStock.BaseUnitValue)] = StockHistoryDetailFragment{
				BusinessId:        outStock.BusinessId,
				WarehouseId:       outStock.WarehouseId,
				ProductId:         outStock.ProductId,
				ProductType:       outStock.ProductType,
				BatchNumber:       outStock.BatchNumber,
				StockDate:         outStock.StockDate,
				Description:       outStock.Description,
				ReferenceId:       outStock.ReferenceID,
				ReferenceType:     outStock.ReferenceType,
				ReferenceDetailId: outStock.ReferenceDetailID,
				Qty:               outgoingStockHistories[0].Qty.Abs().Sub(processQty),
				BaseUnitValue:     outStock.BaseUnitValue,
			}
		}

		if outIndex > 0 {
			processQty = outStock.Qty.Abs()
		}
		// if len(incomingStockHistories) > 0 && !NormalizeDate(outStock.StockDate).Before(NormalizeDate(incomingStockHistories[0].StockDate)) {
		if len(incomingStockHistories) > 0 {
			for processQty.GreaterThan(decimal.Zero) && len(incomingStockHistories) > 0 {
				if currentStock.Qty.GreaterThanOrEqual(processQty) {
					// if the current stock can fulfil the process qty
					if existing, found := uniqueStocks[fmt.Sprintf("%d-%d-%s-%s-%d-%s-%d", outStock.WarehouseId, outStock.ProductId, outStock.ProductType, outStock.BatchNumber, outStock.ReferenceID, outStock.ReferenceType, outStock.ReferenceDetailID)]; found {
						existing.TotalQty = existing.TotalQty.Add(processQty)
						existing.TotalValue = existing.TotalValue.Add(processQty.Mul(currentStock.BaseUnitValue))
						uniqueStocks[fmt.Sprintf("%d-%d-%s-%s-%d-%s-%d", outStock.WarehouseId, outStock.ProductId, outStock.ProductType, outStock.BatchNumber, outStock.ReferenceID, outStock.ReferenceType, outStock.ReferenceDetailID)] = existing
					} else {
						uniqueStocks[fmt.Sprintf("%d-%d-%s-%s-%d-%s-%d", outStock.WarehouseId, outStock.ProductId, outStock.ProductType, outStock.BatchNumber, outStock.ReferenceID, outStock.ReferenceType, outStock.ReferenceDetailID)] = StockHistoryFragment{
							BusinessId:        outStock.BusinessId,
							WarehouseId:       outStock.WarehouseId,
							ProductId:         outStock.ProductId,
							ProductType:       outStock.ProductType,
							BatchNumber:       outStock.BatchNumber,
							ReferenceId:       outStock.ReferenceID,
							ReferenceType:     outStock.ReferenceType,
							ReferenceDetailId: outStock.ReferenceDetailID,
							TotalQty:          processQty,
							TotalValue:        processQty.Mul(currentStock.BaseUnitValue),
						}
					}
					if existing, found := uniqueStockDetails[fmt.Sprintf("%d-%d-%s-%s-%d-%s-%d-%s", outStock.WarehouseId, outStock.ProductId, outStock.ProductType, outStock.BatchNumber, outStock.ReferenceID, outStock.ReferenceType, outStock.ReferenceDetailID, currentStock.BaseUnitValue)]; found {
						existing.Qty = existing.Qty.Add(processQty)
						uniqueStockDetails[fmt.Sprintf("%d-%d-%s-%s-%d-%s-%d-%s", outStock.WarehouseId, outStock.ProductId, outStock.ProductType, outStock.BatchNumber, outStock.ReferenceID, outStock.ReferenceType, outStock.ReferenceDetailID, currentStock.BaseUnitValue)] = existing
					} else {
						uniqueStockDetails[fmt.Sprintf("%d-%d-%s-%s-%d-%s-%d-%s", outStock.WarehouseId, outStock.ProductId, outStock.ProductType, outStock.BatchNumber, outStock.ReferenceID, outStock.ReferenceType, outStock.ReferenceDetailID, currentStock.BaseUnitValue)] = StockHistoryDetailFragment{
							BusinessId:        outStock.BusinessId,
							WarehouseId:       outStock.WarehouseId,
							ProductId:         outStock.ProductId,
							ProductType:       outStock.ProductType,
							BatchNumber:       outStock.BatchNumber,
							StockDate:         outStock.StockDate,
							Description:       outStock.Description,
							ReferenceId:       outStock.ReferenceID,
							ReferenceType:     outStock.ReferenceType,
							ReferenceDetailId: outStock.ReferenceDetailID,
							Qty:               processQty,
							BaseUnitValue:     currentStock.BaseUnitValue,
						}
					}
					currentStock.Qty = currentStock.Qty.Sub(processQty)
					processQty = decimal.Zero
				} else {
					// if the current stock can only partially fulfil the process qty
					if existing, found := uniqueStocks[fmt.Sprintf("%d-%d-%s-%s-%d-%s-%d", outStock.WarehouseId, outStock.ProductId, outStock.ProductType, outStock.BatchNumber, outStock.ReferenceID, outStock.ReferenceType, outStock.ReferenceDetailID)]; found {
						existing.TotalQty = existing.TotalQty.Add(currentStock.Qty)
						existing.TotalValue = existing.TotalValue.Add(currentStock.Qty.Mul(currentStock.BaseUnitValue))
						uniqueStocks[fmt.Sprintf("%d-%d-%s-%s-%d-%s-%d", outStock.WarehouseId, outStock.ProductId, outStock.ProductType, outStock.BatchNumber, outStock.ReferenceID, outStock.ReferenceType, outStock.ReferenceDetailID)] = existing
					} else {
						uniqueStocks[fmt.Sprintf("%d-%d-%s-%s-%d-%s-%d", outStock.WarehouseId, outStock.ProductId, outStock.ProductType, outStock.BatchNumber, outStock.ReferenceID, outStock.ReferenceType, outStock.ReferenceDetailID)] = StockHistoryFragment{
							BusinessId:        outStock.BusinessId,
							WarehouseId:       outStock.WarehouseId,
							ProductId:         outStock.ProductId,
							ProductType:       outStock.ProductType,
							BatchNumber:       outStock.BatchNumber,
							ReferenceId:       outStock.ReferenceID,
							ReferenceType:     outStock.ReferenceType,
							ReferenceDetailId: outStock.ReferenceDetailID,
							TotalQty:          currentStock.Qty,
							TotalValue:        currentStock.Qty.Mul(currentStock.BaseUnitValue),
						}
					}
					if existing, found := uniqueStockDetails[fmt.Sprintf("%d-%d-%s-%s-%d-%s-%d-%s", outStock.WarehouseId, outStock.ProductId, outStock.ProductType, outStock.BatchNumber, outStock.ReferenceID, outStock.ReferenceType, outStock.ReferenceDetailID, currentStock.BaseUnitValue)]; found {
						existing.Qty = existing.Qty.Add(currentStock.Qty)
						uniqueStockDetails[fmt.Sprintf("%d-%d-%s-%s-%d-%s-%d-%s", outStock.WarehouseId, outStock.ProductId, outStock.ProductType, outStock.BatchNumber, outStock.ReferenceID, outStock.ReferenceType, outStock.ReferenceDetailID, currentStock.BaseUnitValue)] = existing
					} else {
						uniqueStockDetails[fmt.Sprintf("%d-%d-%s-%s-%d-%s-%d-%s", outStock.WarehouseId, outStock.ProductId, outStock.ProductType, outStock.BatchNumber, outStock.ReferenceID, outStock.ReferenceType, outStock.ReferenceDetailID, currentStock.BaseUnitValue)] = StockHistoryDetailFragment{
							BusinessId:        outStock.BusinessId,
							WarehouseId:       outStock.WarehouseId,
							ProductId:         outStock.ProductId,
							ProductType:       outStock.ProductType,
							BatchNumber:       outStock.BatchNumber,
							StockDate:         outStock.StockDate,
							Description:       outStock.Description,
							ReferenceId:       outStock.ReferenceID,
							ReferenceType:     outStock.ReferenceType,
							ReferenceDetailId: outStock.ReferenceDetailID,
							Qty:               currentStock.Qty,
							BaseUnitValue:     currentStock.BaseUnitValue,
						}
					}
					processQty = processQty.Sub(currentStock.Qty)
					currentStock.Qty = decimal.Zero
				}
				if currentStock.Qty.GreaterThan(decimal.Zero) {
					// incomingStockHistories[0] = currentStock
				} else {
					if len(incomingStockHistories) > 1 {
						incomingStockHistories = incomingStockHistories[1:]
						currentStock = incomingStockHistories[0]
					} else {
						incomingStockHistories = []*models.StockHistory{}
					}
				}
			}
		}

		if processQty.GreaterThan(decimal.Zero) {
			return accountIds, fmt.Errorf("insufficient FIFO layers for product_id=%d product_type=%s warehouse_id=%d batch=%s qty_missing=%s",
				outStock.ProductId,
				string(outStock.ProductType),
				outStock.WarehouseId,
				outStock.BatchNumber,
				processQty.String(),
			)
		}
	}

	// Aggregate valuation deltas per (reference_type, reference_id) so we only repost once per document.
	type journalDeltaKey struct {
		businessId string
		refType    models.StockReferenceType
		refId      int
		transferIn bool // only used for transfer orders; false=transfer-out
	}
	journalDeltas := make(map[journalDeltaKey]map[int]valuationDelta)

	for key, uStock := range uniqueStocks {
		eStock, found := existingStocks[key]
		if !found {
			continue
		}

		delta := uStock.TotalValue.Sub(eStock.TotalValue)
		if delta.IsZero() {
			continue
		}

		// Update persisted COGS on details (source of truth for reporting).
		if uStock.ReferenceType == models.StockReferenceTypeSupplierCredit {
			err = tx.Exec("UPDATE supplier_credit_details SET cogs = cogs + ? WHERE id = ?",
				delta, uStock.ReferenceDetailId).Error
		} else if uStock.ReferenceType == models.StockReferenceTypeInvoice {
			err = tx.Exec("UPDATE sales_invoice_details SET cogs = cogs + ? WHERE id = ?",
				delta, uStock.ReferenceDetailId).Error
		}
		if err != nil {
			config.LogError(logger, "MainWorkflow.go", "CalculateCogs", "Update Details", uStock, err)
			return accountIds, err
		}

		// Collect journal valuation deltas (append-only ledger via reversal+repost).
		//
		// NOTE: Transfer Orders have TWO journals:
		// - Source (transfer-out):   DR Goods In Transfer, CR Inventory (IsTransferIn=false)
		// - Destination (transfer-in): DR Inventory, CR Goods In Transfer (IsTransferIn=true)
		//
		// When FIFO/valuation is recalculated later (due to backdated incoming/value adjustments),
		// we must repost BOTH journals with matching deltas; otherwise "Goods In Transfer" will
		// show a residual balance (e.g. 300) and the Balance Sheet will not reconcile.
		if uStock.ReferenceType == models.StockReferenceTypeTransferOrder {
			systemAccounts, err := models.GetSystemAccounts(uStock.BusinessId)
			if err != nil {
				config.LogError(logger, "MainWorkflow.go", "CalculateCogs", "GetSystemAccounts", uStock.BusinessId, err)
				return accountIds, err
			}
			git := systemAccounts[models.AccountCodeGoodsInTransfer]
			inv := productDetail.InventoryAccountId

			// transfer-out deltas
			kOut := journalDeltaKey{businessId: uStock.BusinessId, refType: uStock.ReferenceType, refId: uStock.ReferenceId, transferIn: false}
			mOut, ok := journalDeltas[kOut]
			if !ok {
				mOut = make(map[int]valuationDelta)
				journalDeltas[kOut] = mOut
			}
			mOut[git] = valuationDelta{
				BaseDebit:  mOut[git].BaseDebit.Add(delta),
				BaseCredit: mOut[git].BaseCredit,
			}
			mOut[inv] = valuationDelta{
				BaseDebit:  mOut[inv].BaseDebit,
				BaseCredit: mOut[inv].BaseCredit.Add(delta),
			}

			// transfer-in deltas (inverse)
			kIn := journalDeltaKey{businessId: uStock.BusinessId, refType: uStock.ReferenceType, refId: uStock.ReferenceId, transferIn: true}
			mIn, ok := journalDeltas[kIn]
			if !ok {
				mIn = make(map[int]valuationDelta)
				journalDeltas[kIn] = mIn
			}
			mIn[inv] = valuationDelta{
				BaseDebit:  mIn[inv].BaseDebit.Add(delta),
				BaseCredit: mIn[inv].BaseCredit,
			}
			mIn[git] = valuationDelta{
				BaseDebit:  mIn[git].BaseDebit,
				BaseCredit: mIn[git].BaseCredit.Add(delta),
			}

			if !slices.Contains(accountIds, git) {
				accountIds = append(accountIds, git)
			}
			if !slices.Contains(accountIds, inv) {
				accountIds = append(accountIds, inv)
			}
		} else {
			k := journalDeltaKey{businessId: uStock.BusinessId, refType: uStock.ReferenceType, refId: uStock.ReferenceId, transferIn: false}

			m, ok := journalDeltas[k]
			if !ok {
				m = make(map[int]valuationDelta)
				journalDeltas[k] = m
			}
			// Outgoing valuation: DR purchase/COGS, CR inventory.
			pAcc := productDetail.PurchaseAccountId
			iAcc := productDetail.InventoryAccountId
			m[pAcc] = valuationDelta{
				BaseDebit:  m[pAcc].BaseDebit.Add(delta),
				BaseCredit: m[pAcc].BaseCredit,
			}
			m[iAcc] = valuationDelta{
				BaseDebit:  m[iAcc].BaseDebit,
				BaseCredit: m[iAcc].BaseCredit.Add(delta),
			}
			if !slices.Contains(accountIds, pAcc) {
				accountIds = append(accountIds, pAcc)
			}
			if !slices.Contains(accountIds, iAcc) {
				accountIds = append(accountIds, iAcc)
			}
		}
	}

	// Apply aggregated journal reposts.
	for k, deltas := range journalDeltas {
		var refType models.AccountReferenceType
		switch k.refType {
		case models.StockReferenceTypeInvoice:
			refType = models.AccountReferenceTypeInvoice
		case models.StockReferenceTypeSupplierCredit:
			refType = models.AccountReferenceTypeSupplierCredit
		case models.StockReferenceTypeTransferOrder:
			refType = models.AccountReferenceTypeTransferOrder
		default:
			// Unknown/unsupported reference types: skip journal repost.
			continue
		}

		var transferInFilter *bool
		if k.refType == models.StockReferenceTypeTransferOrder {
			// Repost correct journal side: transfer-out (false) or transfer-in (true).
			if k.transferIn {
				transferInFilter = utils.NewTrue()
			} else {
				transferInFilter = utils.NewFalse()
			}
		}

		if _, _, err := repostJournalWithValuationDeltas(
			tx,
			logger,
			k.businessId,
			refType,
			k.refId,
			deltas,
			transferInFilter,
			ReversalReasonInventoryValuationReprice,
		); err != nil {
			config.LogError(logger, "MainWorkflow.go", "CalculateCogs", "repostJournalWithValuationDeltas", k, err)
			return accountIds, err
		}
	}

	for key, uStockDetail := range uniqueStockDetails {
		// if uStockDetail.ReferenceType == models.StockReferenceTypeInventoryAdjustmentQuantity {
		// 	continue
		// }
		if eStockDetail, found := existingStockDetails[key]; found {
			_, err = ReplaceStockHistoryByID(tx, eStockDetail.Id, ReversalReasonInventoryValuationReprice, func(newRow *models.StockHistory) {
				newRow.Qty = uStockDetail.Qty.Neg()
				newRow.BaseUnitValue = uStockDetail.BaseUnitValue
				newRow.Description = uStockDetail.Description
				newRow.StockDate = uStockDetail.StockDate
				newRow.IsOutgoing = utils.NewTrue()
			})
			if err != nil {
				config.LogError(logger, "MainWorkflow.go", "CalculateCogs", "ReplaceStockHistoryByID", uStockDetail, err)
				return accountIds, err
			}
		} else {
			// Defensive idempotency: never create duplicate active stock_histories rows for the same
			// reference detail + base_unit_value bucket. This can happen under at-least-once processing
			// or when recalculation is re-run for the same date range.
			dup := new(models.StockHistory)
			dupErr := tx.
				Where(
					"business_id = ? AND warehouse_id = ? AND product_id = ? AND product_type = ? AND batch_number = ? AND reference_type = ? AND reference_id = ? AND reference_detail_id = ? AND is_outgoing = 1 AND qty = ? AND base_unit_value = ? AND is_reversal = 0 AND reversed_by_stock_history_id IS NULL",
					uStockDetail.BusinessId,
					uStockDetail.WarehouseId,
					uStockDetail.ProductId,
					uStockDetail.ProductType,
					uStockDetail.BatchNumber,
					uStockDetail.ReferenceType,
					uStockDetail.ReferenceId,
					uStockDetail.ReferenceDetailId,
					uStockDetail.Qty.Neg(),
					uStockDetail.BaseUnitValue,
				).
				First(dup).Error
			if dupErr == nil {
				if logger != nil {
					logger.WithFields(logrus.Fields{
						"ledgerKey": fmt.Sprintf("%d-%d-%s-%s-%d-%s-%d:%s",
							uStockDetail.WarehouseId, uStockDetail.ProductId, uStockDetail.ProductType, uStockDetail.BatchNumber,
							uStockDetail.ReferenceId, uStockDetail.ReferenceType, uStockDetail.ReferenceDetailId, uStockDetail.BaseUnitValue.String()),
						"business_id":         uStockDetail.BusinessId,
						"warehouse_id":        uStockDetail.WarehouseId,
						"product_id":          uStockDetail.ProductId,
						"product_type":        uStockDetail.ProductType,
						"batch_number":        uStockDetail.BatchNumber,
						"reference_id":        uStockDetail.ReferenceId,
						"reference_type":      uStockDetail.ReferenceType,
						"reference_detail_id": uStockDetail.ReferenceDetailId,
						"base_unit_value":     uStockDetail.BaseUnitValue.String(),
					}).Info("inv.posting.idempotency_hit")
				}
				// Already exists; do not insert another identical active row.
				continue
			}
			if dupErr != gorm.ErrRecordNotFound {
				config.LogError(logger, "MainWorkflow.go", "CalculateCogs", "FindDuplicateStockHistory", uStockDetail, dupErr)
				return accountIds, dupErr
			}

			err = tx.Create(&models.StockHistory{
				BusinessId:        uStockDetail.BusinessId,
				WarehouseId:       uStockDetail.WarehouseId,
				ProductId:         uStockDetail.ProductId,
				ProductType:       uStockDetail.ProductType,
				BatchNumber:       uStockDetail.BatchNumber,
				StockDate:         uStockDetail.StockDate,
				Qty:               uStockDetail.Qty.Neg(),
				Description:       uStockDetail.Description,
				BaseUnitValue:     uStockDetail.BaseUnitValue,
				ReferenceID:       uStockDetail.ReferenceId,
				ReferenceType:     uStockDetail.ReferenceType,
				ReferenceDetailID: uStockDetail.ReferenceDetailId,
				IsOutgoing:        utils.NewTrue(),
			}).Error
			if err != nil {
				config.LogError(logger, "MainWorkflow.go", "CalculateCogs", "CreateStockHistory", uStockDetail, err)
				return accountIds, err
			}
		}
	}

	for key, eStockDetail := range existingStockDetails {
		// if eStockDetail.ReferenceType == models.StockReferenceTypeInventoryAdjustmentQuantity {
		// 	continue
		// }
		if _, found := uniqueStockDetails[key]; !found {
			// Do not delete historical stock rows; append a reversal instead (auditability).
			orig := &models.StockHistory{
				ID:                eStockDetail.Id,
				BusinessId:        eStockDetail.BusinessId,
				WarehouseId:       eStockDetail.WarehouseId,
				ProductId:         eStockDetail.ProductId,
				ProductType:       eStockDetail.ProductType,
				BatchNumber:       eStockDetail.BatchNumber,
				StockDate:         eStockDetail.StockDate,
				Qty:               eStockDetail.Qty,
				BaseUnitValue:     eStockDetail.BaseUnitValue,
				Description:       eStockDetail.Description,
				ReferenceType:     eStockDetail.ReferenceType,
				ReferenceID:       eStockDetail.ReferenceId,
				ReferenceDetailID: eStockDetail.ReferenceDetailId,
				IsOutgoing:        utils.NewTrue(),
			}
			_, err := ReverseStockHistories(tx, []*models.StockHistory{orig}, ReversalReasonInventoryValuationReprice)
			if err != nil {
				config.LogError(logger, "MainWorkflow.go", "CalculateCogs", "ReverseStockHistory", eStockDetail, err)
				return accountIds, err
			}
		}
	}

	return accountIds, err
}

func revaluateCreditNotes(tx *gorm.DB, logger *logrus.Logger, stockHistories []*models.StockHistory, ignoreCurrentUnitValue bool, isValueAdjustment bool) error {
	var baseUnitValue decimal.Decimal
	var productDetail ProductDetail
	var err error

	// Aggregate valuation deltas per CreditNote reference, then repost journals once per reference
	// (append-only ledger).
	creditNoteJournalDeltas := make(map[int]map[int]valuationDelta) // reference_id -> (account_id -> delta)

	for _, stockHistory := range stockHistories {
		productDetail, err = GetProductDetail(tx, stockHistory.ProductId, stockHistory.ProductType)
		if err != nil {
			config.LogError(logger, "MainWorkflow.go", "RevaluateCreditNotes", "GetProductDetail", stockHistory, err)
			return err
		}
		baseUnitValue = productDetail.PurchasePrice
		var lastIncomingStockHistories []*models.StockHistory
		err = tx.Raw(`
		WITH LastStockHistories AS (
			SELECT 
				*,
				ROW_NUMBER() OVER (PARTITION BY warehouse_id, product_id, product_type, batch_number ORDER BY stock_date DESC, is_outgoing DESC, id DESC) AS rn
			FROM 
				stock_histories
			WHERE 
				warehouse_id = ? 
				AND stock_date <= ? 
				AND is_outgoing = false
				AND is_reversal = 0
				AND reversed_by_stock_history_id IS NULL
				AND reference_type != 'CN'
				AND product_id = ? AND product_type = ? AND batch_number = ?
			)
		SELECT 
			*
		FROM 
			LastStockHistories
		WHERE 
			rn = 1;
		`, stockHistory.WarehouseId, stockHistory.StockDate, stockHistory.ProductId,
			stockHistory.ProductType, stockHistory.BatchNumber).
			Find(&lastIncomingStockHistories).Error
		if err != nil {
			config.LogError(logger, "MainWorkflow.go", "RevaluateCreditNotes", "GetLastIncomingStockHistory", stockHistory, err)
			return err
		}
		if len(lastIncomingStockHistories) > 0 {
			baseUnitValue = lastIncomingStockHistories[0].BaseUnitValue
		}
		if stockHistory.ReferenceType == models.StockReferenceTypeCreditNote {
			_, err = ReplaceStockHistoryByID(tx, stockHistory.ID, ReversalReasonInventoryValuationReprice, func(newRow *models.StockHistory) {
				newRow.BaseUnitValue = baseUnitValue
			})
			if err != nil {
				config.LogError(logger, "MainWorkflow.go", "RevaluateCreditNotes", "ReplaceStockHistoryByID", stockHistory, err)
				return err
			}

			// Credit note valuation: DR inventory, CR purchase/COGS (same direction as existing code).
			delta := stockHistory.Qty.Mul(baseUnitValue)
			m, ok := creditNoteJournalDeltas[stockHistory.ReferenceID]
			if !ok {
				m = make(map[int]valuationDelta)
				creditNoteJournalDeltas[stockHistory.ReferenceID] = m
			}
			pAcc := productDetail.PurchaseAccountId
			iAcc := productDetail.InventoryAccountId
			m[pAcc] = valuationDelta{
				BaseDebit:  m[pAcc].BaseDebit,
				BaseCredit: m[pAcc].BaseCredit.Add(delta),
			}
			m[iAcc] = valuationDelta{
				BaseDebit:  m[iAcc].BaseDebit.Add(delta),
				BaseCredit: m[iAcc].BaseCredit,
			}

			err = tx.Exec("UPDATE credit_note_details SET cogs = cogs + ? WHERE id = ?",
				delta, stockHistory.ReferenceDetailID).Error
			if err != nil {
				config.LogError(logger, "MainWorkflow.go", "RevaluateCreditNotes", "Update Details", stockHistory, err)
				return err
			}
		} else {
			// check if newer stocks are CreditNotes until another incoming stock is found
			var creditNotes []*models.StockHistory
			if isValueAdjustment {
				err = tx.Raw(`
						WITH CTE AS (
							SELECT sh.*, ROW_NUMBER() OVER (ORDER BY stock_date, id) AS rn,
								CASE WHEN sh.is_outgoing = false AND sh.reference_type != 'CN' THEN 1 ELSE 0 END AS incoming_flag
							FROM stock_histories AS sh
							WHERE sh.warehouse_id = ? AND sh.product_id = ? AND sh.product_type = ?
								AND sh.batch_number = ? AND sh.stock_date > ?
								AND sh.is_reversal = 0
								AND sh.reversed_by_stock_history_id IS NULL
						)
						, IncomingLimit AS (
							SELECT COALESCE(MIN(rn), (SELECT MAX(rn) + 1 FROM CTE)) AS limit_row
							FROM CTE
							WHERE incoming_flag = 1
						)
						SELECT CTE.* FROM CTE, IncomingLimit
						WHERE CTE.rn < IncomingLimit.limit_row AND CTE.reference_type = 'CN'
						ORDER BY CTE.stock_date ASC, CTE.id ASC;
					`, stockHistory.WarehouseId, stockHistory.ProductId, stockHistory.ProductType,
					stockHistory.BatchNumber, stockHistory.StockDate).Find(&creditNotes).Error
				if err != nil {
					config.LogError(logger, "MainWorkflow.go", "RevaluateCreditNotes", "Get CreditNotes", stockHistory, err)
					return err
				}
			} else {
				err = tx.Raw(`
						WITH CTE AS (
							SELECT sh.*, ROW_NUMBER() OVER (ORDER BY stock_date, id) AS rn,
								CASE WHEN sh.is_outgoing = false AND sh.reference_type != 'CN' THEN 1 ELSE 0 END AS incoming_flag
							FROM stock_histories AS sh
							WHERE sh.warehouse_id = ? AND sh.product_id = ? AND sh.product_type = ?
								AND sh.batch_number = ? AND sh.stock_date >= ?
								AND sh.is_reversal = 0
								AND sh.reversed_by_stock_history_id IS NULL
						)
						, IncomingLimit AS (
							SELECT COALESCE(MIN(rn), (SELECT MAX(rn) + 1 FROM CTE)) AS limit_row
							FROM CTE
							WHERE incoming_flag = 1
						)
						SELECT CTE.* FROM CTE, IncomingLimit
						WHERE CTE.rn < IncomingLimit.limit_row AND CTE.reference_type = 'CN'
						ORDER BY CTE.stock_date ASC, CTE.id ASC;
				`, stockHistory.WarehouseId, stockHistory.ProductId, stockHistory.ProductType,
					stockHistory.BatchNumber, stockHistory.StockDate).Find(&creditNotes).Error
				if err != nil {
					config.LogError(logger, "MainWorkflow.go", "RevaluateCreditNotes", "Get CreditNotes", stockHistory, err)
					return err
				}
			}

			if !ignoreCurrentUnitValue {
				baseUnitValue = stockHistory.BaseUnitValue
			}
			for _, creditNote := range creditNotes {
				oldCogs := creditNote.Qty.Mul(creditNote.BaseUnitValue)
				cogs := creditNote.Qty.Mul(baseUnitValue)
				_, err = ReplaceStockHistoryByID(tx, creditNote.ID, ReversalReasonInventoryValuationReprice, func(newRow *models.StockHistory) {
					newRow.BaseUnitValue = baseUnitValue
				})
				if err != nil {
					config.LogError(logger, "MainWorkflow.go", "RevaluateCreditNotes", "ReplaceStockHistoryByID (CreditNote)", creditNote, err)
					return err
				}

				delta := cogs.Sub(oldCogs)
				m, ok := creditNoteJournalDeltas[creditNote.ReferenceID]
				if !ok {
					m = make(map[int]valuationDelta)
					creditNoteJournalDeltas[creditNote.ReferenceID] = m
				}
				pAcc := productDetail.PurchaseAccountId
				iAcc := productDetail.InventoryAccountId
				m[pAcc] = valuationDelta{
					BaseDebit:  m[pAcc].BaseDebit,
					BaseCredit: m[pAcc].BaseCredit.Add(delta),
				}
				m[iAcc] = valuationDelta{
					BaseDebit:  m[iAcc].BaseDebit.Add(delta),
					BaseCredit: m[iAcc].BaseCredit,
				}

				err = tx.Exec("UPDATE credit_note_details SET cogs = cogs + ? WHERE id = ?",
					delta, creditNote.ReferenceDetailID).Error
				if err != nil {
					config.LogError(logger, "MainWorkflow.go", "RevaluateCreditNotes", "Update CreditNoteDetails", creditNote, err)
					return err
				}
			}
		}
	}

	// Repost CreditNote journals with aggregated valuation deltas.
	for refId, deltas := range creditNoteJournalDeltas {
		businessId := ""
		if len(stockHistories) > 0 {
			businessId = stockHistories[0].BusinessId
		}
		if businessId == "" {
			// Fallback: derive tenant from the credit note stock history rows.
			for _, sh := range stockHistories {
				if sh.ReferenceID == refId {
					businessId = sh.BusinessId
					break
				}
			}
		}
		if businessId == "" {
			continue
		}
		if _, _, err := repostJournalWithValuationDeltas(
			tx,
			logger,
			businessId,
			models.AccountReferenceTypeCreditNote,
			refId,
			deltas,
			nil,
			ReversalReasonInventoryValuationReprice,
		); err != nil {
			config.LogError(logger, "MainWorkflow.go", "RevaluateCreditNotes", "repostJournalWithValuationDeltas", refId, err)
			return err
		}
	}

	return err
}

// Process Value Adjustment Stock
func ProcessValueAdjustmentStocks(tx *gorm.DB, logger *logrus.Logger, stockHistories []*models.StockHistory) ([]int, error) {
	accountIds := make([]int, 0)
	var err error

	if len(stockHistories) <= 0 {
		return accountIds, nil
	}

	lastStockHistoriesAll, err := getLastStockHistoriesForValueAdjustment(tx, stockHistories, true)
	if err != nil {
		config.LogError(logger, "MainWorkflow.go", "ProcessAdjustmentStocks", "GetLastStockHistoriesAll", stockHistories, err)
		return accountIds, err
	}

	lastIncomingStockHistories, err := getLastStockHistoriesForValueAdjustment(tx, stockHistories, false)
	if err != nil {
		config.LogError(logger, "MainWorkflow.go", "ProcessAdjustmentStocks", "GetLastStockHistories", stockHistories, err)
		return accountIds, err
	}

	err = revaluateCreditNotes(tx, logger, stockHistories, false, true)
	if err != nil {
		return accountIds, err
	}

	for _, stockHistory := range stockHistories {
		productDetail, err := GetProductDetail(tx, stockHistory.ProductId, stockHistory.ProductType)
		if err != nil {
			config.LogError(logger, "MainWorkflow.go", "ProcessOutgoingStocks", "GetProductDetail", stockHistory, err)
			return accountIds, err
		}

		lastCumulativeIncomingQty := decimal.NewFromInt(0)

		for _, lastStockHistory := range lastIncomingStockHistories {
			if lastStockHistory.WarehouseId == stockHistory.WarehouseId &&
				lastStockHistory.ProductId == stockHistory.ProductId &&
				lastStockHistory.ProductType == stockHistory.ProductType &&
				lastStockHistory.BatchNumber == stockHistory.BatchNumber {

				lastCumulativeIncomingQty = lastStockHistory.CumulativeIncomingQty
				break
			}
		}

		var maxCumulativeOutgoingQty decimal.Decimal
		err = tx.Model(&models.StockHistory{}).Where("warehouse_id = ? AND product_id = ? AND product_type = ? AND batch_number = ?",
			stockHistory.WarehouseId, stockHistory.ProductId, stockHistory.ProductType, stockHistory.BatchNumber).
			Select("COALESCE(MAX(cumulative_outgoing_qty), 0)").Scan(&maxCumulativeOutgoingQty).Error
		if err != nil {
			config.LogError(logger, "MainWorkflow.go", "ProcessIncomingStocks", "GetMaxCumulativeOutgoingQty", nil, err)
			return accountIds, err
		}

		lastCumulativeIncomingQty = lastCumulativeIncomingQty.Sub(stockHistory.Qty)

		shouldRecalc := lastCumulativeIncomingQty.LessThan(maxCumulativeOutgoingQty)
		if !shouldRecalc {
			// Backdated incoming stock can change FIFO/COGS for outgoing rows that are already posted
			// on or after this stock_date. Recalculate whenever such outgoing rows exist.
			var outgoingCount int64
			err = tx.Model(&models.StockHistory{}).
				Where("business_id = ? AND warehouse_id = ? AND product_id = ? AND product_type = ? AND batch_number = ?",
					stockHistory.BusinessId, stockHistory.WarehouseId, stockHistory.ProductId, stockHistory.ProductType, stockHistory.BatchNumber).
				Where("is_outgoing = 1").
				Where("stock_date >= ?", stockHistory.StockDate).
				Where("is_reversal = 0 AND reversed_by_stock_history_id IS NULL").
				Count(&outgoingCount).Error
			if err != nil {
				config.LogError(logger, "MainWorkflow.go", "ProcessIncomingStocks", "CountOutgoingAfterIncomingDate", stockHistory, err)
				return accountIds, err
			}
			if outgoingCount > 0 {
				shouldRecalc = true
			}
		}

		if shouldRecalc {
			remainingIncomingStockHistories, err := GetRemainingStockHistoriesByDate(tx, stockHistory.WarehouseId, stockHistory.ProductId, string(stockHistory.ProductType), stockHistory.BatchNumber, utils.NewFalse(), stockHistory.StockDate)
			if err != nil {
				config.LogError(logger, "MainWorkflow.go", "ProcessIncomingStocks", "GetRemainingStockHistoriesByDate", stockHistory, err)
				return accountIds, err
			}
			remainingOutgoingStockHistories, err := GetRemainingStockHistoriesByCumulativeQty(tx, stockHistory.WarehouseId, stockHistory.ProductId, string(stockHistory.ProductType), stockHistory.BatchNumber, utils.NewTrue(), lastCumulativeIncomingQty)
			if err != nil {
				config.LogError(logger, "MainWorkflow.go", "ProcessIncomingStocks", "GetRemainingStockHistoriesByCumulativeQty", stockHistory, err)
				return accountIds, err
			}
			remainingIncomingStockHistories, remainingOutgoingStockHistories = FilterStockHistories(remainingIncomingStockHistories, remainingOutgoingStockHistories)

			startProcessQty := decimal.NewFromInt(0)
			if len(remainingOutgoingStockHistories) > 0 {
				startProcessQty = remainingOutgoingStockHistories[0].CumulativeOutgoingQty.Sub(lastCumulativeIncomingQty)
			}
			accountIds, err = calculateCogs(tx, logger, productDetail, startProcessQty, decimal.NewFromInt(0), remainingIncomingStockHistories, remainingOutgoingStockHistories, stockHistory.ReferenceID, stockHistory.ReferenceType)
			if err != nil {
				return accountIds, err
			}
		}
	}

	err = models.UpdateStockClosingBalances(tx, stockHistories, lastStockHistoriesAll)
	if err != nil {
		config.LogError(logger, "MainWorkflow.go", "ProcessIncomingStocks", "UpdateStockClosingBalances", stockHistories, err)
		return accountIds, err
	}

	if err := ensureNonNegativeForKeys(tx, logger, stockHistories); err != nil {
		return accountIds, err
	}

	return accountIds, err
}

// Process Value Adjustment Stock Deletion
func ProcessValueAdjustmentStocksDeletion(tx *gorm.DB, logger *logrus.Logger, stockHistories []*models.StockHistory) ([]int, error) {
	accountIds := make([]int, 0)
	var err error

	if len(stockHistories) <= 0 {
		return accountIds, nil
	}

	lastStockHistoriesAll, err := getLastStockHistories(tx, stockHistories, true)
	if err != nil {
		config.LogError(logger, "MainWorkflow.go", "ProcessValueAdjustmentStocksDeletion", "GetLastStockHistoriesAll", stockHistories, err)
		return accountIds, err
	}

	lastIncomingStockHistories, err := getLastStockHistories(tx, stockHistories, false)
	if err != nil {
		config.LogError(logger, "MainWorkflow.go", "ProcessValueAdjustmentStocksDeletion", "GetLastStockHistories", stockHistories, err)
		return accountIds, err
	}

	err = revaluateCreditNotes(tx, logger, stockHistories, true, true)
	if err != nil {
		return accountIds, err
	}

	for _, stockHistory := range stockHistories {

		productDetail, err := GetProductDetail(tx, stockHistory.ProductId, stockHistory.ProductType)
		if err != nil {
			config.LogError(logger, "MainWorkflow.go", "ProcessValueAdjustmentStocksDeletion", "GetProductDetail", stockHistory, err)
			return accountIds, err
		}

		lastCumulativeOutgoingQty := decimal.NewFromInt(0)

		for _, lastStockHistory := range lastIncomingStockHistories {
			if lastStockHistory.WarehouseId == stockHistory.WarehouseId &&
				lastStockHistory.ProductId == stockHistory.ProductId &&
				lastStockHistory.ProductType == stockHistory.ProductType &&
				lastStockHistory.BatchNumber == stockHistory.BatchNumber {

				lastCumulativeOutgoingQty = lastStockHistory.CumulativeOutgoingQty
				break
			}
		}

		remainingIncomingStockHistories, err := GetRemainingStockHistoriesByCumulativeQty(tx, stockHistory.WarehouseId, stockHistory.ProductId, string(stockHistory.ProductType), stockHistory.BatchNumber, utils.NewFalse(), lastCumulativeOutgoingQty)
		if err != nil {
			config.LogError(logger, "MainWorkflow.go", "ProcessOutgoingStocks", "GetRemainingStockHistoriesByCumulativeQty", stockHistory, err)
			return accountIds, err
		}
		remainingOutgoingStockHistories, err := GetRemainingStockHistoriesByDate(tx, stockHistory.WarehouseId, stockHistory.ProductId, string(stockHistory.ProductType), stockHistory.BatchNumber, utils.NewTrue(), stockHistory.StockDate)
		if err != nil {
			config.LogError(logger, "MainWorkflow.go", "ProcessOutgoingStocks", "GetRemainingStockHistoriesByDate", stockHistory, err)
			return accountIds, err
		}
		startCurrentQty := decimal.NewFromInt(0)
		if len(remainingIncomingStockHistories) > 0 {
			startCurrentQty = remainingIncomingStockHistories[0].CumulativeIncomingQty.Sub(lastCumulativeOutgoingQty)
		}

		accountIds, err = calculateCogs(tx, logger, productDetail, decimal.NewFromInt(0), startCurrentQty, remainingIncomingStockHistories, remainingOutgoingStockHistories, stockHistory.ReferenceID, stockHistory.ReferenceType)
		if err != nil {
			return accountIds, err
		}
	}

	err = models.UpdateStockClosingBalances(tx, stockHistories, lastStockHistoriesAll)
	if err != nil {
		config.LogError(logger, "MainWorkflow.go", "ProcessOutgoingStocks", "UpdateStockClosingBalances", stockHistories, err)
		return accountIds, err
	}

	if err := ensureNonNegativeForKeys(tx, logger, stockHistories); err != nil {
		return accountIds, err
	}

	return accountIds, err
}

// ensureNonNegativeForKeys validates that cumulative quantity never drops below zero for the affected item/warehouse keys.
func ensureNonNegativeForKeys(tx *gorm.DB, logger *logrus.Logger, stockHistories []*models.StockHistory) error {
	type key struct {
		BusinessId  string
		WarehouseId int
		ProductId   int
		ProductType models.ProductType
		Batch       string
	}
	seen := make(map[key]struct{})
	for _, sh := range stockHistories {
		if sh == nil {
			continue
		}
		k := key{
			BusinessId:  sh.BusinessId,
			WarehouseId: sh.WarehouseId,
			ProductId:   sh.ProductId,
			ProductType: sh.ProductType,
			Batch:       sh.BatchNumber,
		}
		if _, ok := seen[k]; ok {
			continue
		}
		seen[k] = struct{}{}

		var minQty decimal.Decimal
		err := tx.Raw(`
			SELECT COALESCE(MIN(running_qty), 0) AS min_qty
			FROM (
				SELECT
					SUM(qty) OVER (
						PARTITION BY warehouse_id, product_id, product_type, COALESCE(batch_number,'')
						ORDER BY stock_date, cumulative_sequence, id
						ROWS BETWEEN UNBOUNDED PRECEDING AND CURRENT ROW
					) AS running_qty
				FROM stock_histories
				WHERE business_id = ?
				  AND warehouse_id = ?
				  AND product_id = ?
				  AND product_type = ?
				  AND COALESCE(batch_number,'') = ?
				  AND is_reversal = 0
				  AND reversed_by_stock_history_id IS NULL
			) t
		`, k.BusinessId, k.WarehouseId, k.ProductId, k.ProductType, k.Batch).Scan(&minQty).Error
		if err != nil {
			config.LogError(logger, "MainWorkflow.go", "ensureNonNegativeForKeys", "min_running_qty", k, err)
			return err
		}
		if minQty.LessThan(decimal.Zero) {
			return fmt.Errorf("inventory would become negative for product_id=%d product_type=%s warehouse_id=%d batch=%s", k.ProductId, string(k.ProductType), k.WarehouseId, k.Batch)
		}
	}
	return nil
}

// Process Incoming Stock
func ProcessIncomingStocks(tx *gorm.DB, logger *logrus.Logger, stockHistories []*models.StockHistory) ([]int, error) {
	accountIds := make([]int, 0)
	var err error

	if len(stockHistories) <= 0 {
		return accountIds, nil
	}

	lastStockHistoriesAll, err := getLastStockHistories(tx, stockHistories, true)
	if err != nil {
		config.LogError(logger, "MainWorkflow.go", "ProcessIncomingStocks", "GetLastStockHistoriesAll", stockHistories, err)
		return accountIds, err
	}

	lastIncomingStockHistories, err := getLastStockHistories(tx, stockHistories, false)
	if err != nil {
		config.LogError(logger, "MainWorkflow.go", "ProcessIncomingStocks", "GetLastStockHistories", stockHistories, err)
		return accountIds, err
	}

	err = revaluateCreditNotes(tx, logger, stockHistories, false, false)
	if err != nil {
		return accountIds, err
	}

	// Detect backdated incoming stock and rebuild deterministically from the earliest affected date.
	type rebuildKey struct {
		WarehouseId int
		ProductId   int
		ProductType models.ProductType
		BatchNumber string
	}
	rebuildStarts := make(map[rebuildKey]time.Time)
	for _, sh := range stockHistories {
		if sh == nil {
			continue
		}
		key := rebuildKey{
			WarehouseId: sh.WarehouseId,
			ProductId:   sh.ProductId,
			ProductType: sh.ProductType,
			BatchNumber: sh.BatchNumber,
		}
		if existing, ok := rebuildStarts[key]; !ok || sh.StockDate.Before(existing) {
			rebuildStarts[key] = sh.StockDate
		}
	}
	rebuildKeys := make(map[rebuildKey]time.Time)
	for key, startDate := range rebuildStarts {
		var outgoingCount int64
		err = tx.Model(&models.StockHistory{}).
			Where("business_id = ? AND warehouse_id = ? AND product_id = ? AND product_type = ? AND batch_number = ?",
				stockHistories[0].BusinessId, key.WarehouseId, key.ProductId, key.ProductType, key.BatchNumber).
			Where("is_outgoing = 1 AND stock_date >= ? AND is_reversal = 0 AND reversed_by_stock_history_id IS NULL", startDate).
			Count(&outgoingCount).Error
		if err != nil {
			config.LogError(logger, "MainWorkflow.go", "ProcessIncomingStocks", "CountOutgoingForRebuild", key, err)
			return accountIds, err
		}
		if outgoingCount > 0 {
			rebuildKeys[key] = startDate
		}
	}
	for key, startDate := range rebuildKeys {
		ids, err := RebuildInventoryForItemWarehouseFromDate(
			tx, logger, stockHistories[0].BusinessId, key.WarehouseId, key.ProductId, key.ProductType, key.BatchNumber, startDate,
		)
		if err != nil {
			return accountIds, err
		}
		if len(ids) > 0 {
			accountIds = utils.MergeIntSlices(accountIds, ids)
		}
	}

	for _, stockHistory := range stockHistories {
		if stockHistory == nil {
			continue
		}
		key := rebuildKey{
			WarehouseId: stockHistory.WarehouseId,
			ProductId:   stockHistory.ProductId,
			ProductType: stockHistory.ProductType,
			BatchNumber: stockHistory.BatchNumber,
		}
		if _, shouldSkip := rebuildKeys[key]; shouldSkip {
			// Already rebuilt deterministically for this key.
			continue
		}

		productDetail, err := GetProductDetail(tx, stockHistory.ProductId, stockHistory.ProductType)
		if err != nil {
			config.LogError(logger, "MainWorkflow.go", "ProcessOutgoingStocks", "GetProductDetail", stockHistory, err)
			return accountIds, err
		}

		lastCumulativeIncomingQty := decimal.NewFromInt(0)

		for _, lastStockHistory := range lastIncomingStockHistories {
			if lastStockHistory.WarehouseId == stockHistory.WarehouseId &&
				lastStockHistory.ProductId == stockHistory.ProductId &&
				lastStockHistory.ProductType == stockHistory.ProductType &&
				lastStockHistory.BatchNumber == stockHistory.BatchNumber {

				lastCumulativeIncomingQty = lastStockHistory.CumulativeIncomingQty
				break
			}
		}

		var maxCumulativeOutgoingQty decimal.Decimal
		err = tx.Model(&models.StockHistory{}).Where("warehouse_id = ? AND product_id = ? AND product_type = ? AND batch_number = ?",
			stockHistory.WarehouseId, stockHistory.ProductId, stockHistory.ProductType, stockHistory.BatchNumber).
			Select("COALESCE(MAX(cumulative_outgoing_qty), 0)").Scan(&maxCumulativeOutgoingQty).Error
		if err != nil {
			config.LogError(logger, "MainWorkflow.go", "ProcessIncomingStocks", "GetMaxCumulativeOutgoingQty", nil, err)
			return accountIds, err
		}

		shouldRecalc := lastCumulativeIncomingQty.LessThan(maxCumulativeOutgoingQty)
		if !shouldRecalc {
			// Backdated incoming stock can change FIFO/COGS for outgoing rows that are already posted
			// on or after this stock_date. Recalculate whenever such outgoing rows exist.
			var outgoingCount int64
			err = tx.Model(&models.StockHistory{}).
				Where("business_id = ? AND warehouse_id = ? AND product_id = ? AND product_type = ? AND batch_number = ?",
					stockHistory.BusinessId, stockHistory.WarehouseId, stockHistory.ProductId, stockHistory.ProductType, stockHistory.BatchNumber).
				Where("is_outgoing = 1").
				Where("stock_date >= ?", stockHistory.StockDate).
				Where("is_reversal = 0 AND reversed_by_stock_history_id IS NULL").
				Count(&outgoingCount).Error
			if err != nil {
				config.LogError(logger, "MainWorkflow.go", "ProcessIncomingStocks", "CountOutgoingAfterIncomingDate", stockHistory, err)
				return accountIds, err
			}
			if outgoingCount > 0 {
				shouldRecalc = true
			}
		}

		if shouldRecalc {
			remainingIncomingStockHistories, err := GetRemainingStockHistoriesByDate(tx, stockHistory.WarehouseId, stockHistory.ProductId, string(stockHistory.ProductType), stockHistory.BatchNumber, utils.NewFalse(), stockHistory.StockDate)
			if err != nil {
				config.LogError(logger, "MainWorkflow.go", "ProcessIncomingStocks", "GetRemainingStockHistoriesByDate", stockHistory, err)
				return accountIds, err
			}
			remainingOutgoingStockHistories, err := GetRemainingStockHistoriesByCumulativeQty(tx, stockHistory.WarehouseId, stockHistory.ProductId, string(stockHistory.ProductType), stockHistory.BatchNumber, utils.NewTrue(), lastCumulativeIncomingQty)
			if err != nil {
				config.LogError(logger, "MainWorkflow.go", "ProcessIncomingStocks", "GetRemainingStockHistoriesByCumulativeQty", stockHistory, err)
				return accountIds, err
			}
			remainingIncomingStockHistories, remainingOutgoingStockHistories = FilterStockHistories(remainingIncomingStockHistories, remainingOutgoingStockHistories)

			startProcessQty := decimal.NewFromInt(0)
			if len(remainingOutgoingStockHistories) > 0 {
				startProcessQty = remainingOutgoingStockHistories[0].CumulativeOutgoingQty.Sub(lastCumulativeIncomingQty)
			}
			// Pass 0 reference to recalc for ALL outgoing references in scope (backdated incoming).
			accountIds, err = calculateCogs(tx, logger, productDetail, startProcessQty, decimal.NewFromInt(0), remainingIncomingStockHistories, remainingOutgoingStockHistories, 0, "")
			if err != nil {
				return accountIds, err
			}
		}
	}

	err = models.UpdateStockClosingBalances(tx, stockHistories, lastStockHistoriesAll)
	if err != nil {
		config.LogError(logger, "MainWorkflow.go", "ProcessIncomingStocks", "UpdateStockClosingBalances", stockHistories, err)
		return accountIds, err
	}

	return accountIds, err
}

// Process Outgoing Stock
func ProcessOutgoingStocks(tx *gorm.DB, logger *logrus.Logger, stockHistories []*models.StockHistory) ([]int, error) {
	accountIds := make([]int, 0)
	var err error

	if len(stockHistories) <= 0 {
		return accountIds, nil
	}

	lastStockHistoriesAll, err := getLastStockHistories(tx, stockHistories, true)
	if err != nil {
		config.LogError(logger, "MainWorkflow.go", "ProcessOutgoingStocks", "GetLastStockHistoriesAll", stockHistories, err)
		return accountIds, err
	}

	lastStockHistories, err := getLastStockHistories(tx, stockHistories, false)
	if err != nil {
		config.LogError(logger, "MainWorkflow.go", "ProcessOutgoingStocks", "GetLastStockHistories", stockHistories, err)
		return accountIds, err
	}

	for _, stockHistory := range stockHistories {

		productDetail, err := GetProductDetail(tx, stockHistory.ProductId, stockHistory.ProductType)
		if err != nil {
			config.LogError(logger, "MainWorkflow.go", "ProcessOutgoingStocks", "GetProductDetail", stockHistory, err)
			return accountIds, err
		}

		lastCumulativeOutgoingQty := decimal.NewFromInt(0)

		for _, lastStockHistory := range lastStockHistories {
			if lastStockHistory.WarehouseId == stockHistory.WarehouseId &&
				lastStockHistory.ProductId == stockHistory.ProductId &&
				lastStockHistory.ProductType == stockHistory.ProductType &&
				lastStockHistory.BatchNumber == stockHistory.BatchNumber {

				lastCumulativeOutgoingQty = lastStockHistory.CumulativeOutgoingQty
				break
			}
		}

		remainingIncomingStockHistories, err := GetRemainingStockHistoriesByCumulativeQty(tx, stockHistory.WarehouseId, stockHistory.ProductId, string(stockHistory.ProductType), stockHistory.BatchNumber, utils.NewFalse(), lastCumulativeOutgoingQty)
		if err != nil {
			config.LogError(logger, "MainWorkflow.go", "ProcessOutgoingStocks", "GetRemainingStockHistoriesByCumulativeQty", stockHistory, err)
			return accountIds, err
		}
		remainingOutgoingStockHistories, err := GetRemainingStockHistoriesByDate(tx, stockHistory.WarehouseId, stockHistory.ProductId, string(stockHistory.ProductType), stockHistory.BatchNumber, stockHistory.IsOutgoing, stockHistory.StockDate)
		if err != nil {
			config.LogError(logger, "MainWorkflow.go", "ProcessOutgoingStocks", "GetRemainingStockHistoriesByDate", stockHistory, err)
			return accountIds, err
		}
		remainingIncomingStockHistories, remainingOutgoingStockHistories = FilterStockHistories(remainingIncomingStockHistories, remainingOutgoingStockHistories)

		startCurrentQty := decimal.NewFromInt(0)
		if len(remainingIncomingStockHistories) > 0 {
			startCurrentQty = remainingIncomingStockHistories[0].CumulativeIncomingQty.Sub(lastCumulativeOutgoingQty)
		}

		accountIds, err = calculateCogs(tx, logger, productDetail, decimal.NewFromInt(0), startCurrentQty, remainingIncomingStockHistories, remainingOutgoingStockHistories, stockHistory.ReferenceID, stockHistory.ReferenceType)
		if err != nil {
			return accountIds, err
		}
	}

	err = models.UpdateStockClosingBalances(tx, stockHistories, lastStockHistoriesAll)
	if err != nil {
		config.LogError(logger, "MainWorkflow.go", "ProcessOutgoingStocks", "UpdateStockClosingBalances", stockHistories, err)
		return accountIds, err
	}

	if err := ensureNonNegativeForKeys(tx, logger, stockHistories); err != nil {
		return accountIds, err
	}

	return accountIds, err
}

func FilterStockHistories(remainingIncomingStockHistories []*models.StockHistory, remainingOutgoingStockHistories []*models.StockHistory) ([]*models.StockHistory, []*models.StockHistory) {
	// Filter remainingIncomingStockHistories to remove entries until referenceType == "IVAV"
	filteredIncomingStockHistories := remainingIncomingStockHistories
	for i, history := range remainingIncomingStockHistories {
		if history.ReferenceType == "IVAV" {
			// Keep only the entries starting from this point
			filteredIncomingStockHistories = remainingIncomingStockHistories[i:]
		}
	}

	// Filter remainingIncomingStockHistories to remove entries until referenceType == "IVAV"
	filteredOutgoingStockHistories := remainingOutgoingStockHistories
	for i, history := range remainingOutgoingStockHistories {
		if history.ReferenceType == "IVAV" {
			// Keep only the entries after this point
			filteredOutgoingStockHistories = remainingOutgoingStockHistories[i+1:]
		}
	}
	return filteredIncomingStockHistories, filteredOutgoingStockHistories
}

// Calculate CostOfGoodsSold
// func ProcessInventoryValuation(tx *gorm.DB, logger *logrus.Logger, stocks []*StockFragment, updatedReferenceId int, updatedReferenceType string) ([]int, error) {
// 	// logger := config.GetLogger()

// 	accountIds := make([]int, 0)
// 	var err error
// 	var cnTransactionRecords []TransactionRecord
// 	var transactionRecords []TransactionRecord

// 	for _, stock := range stocks {
// 		var existingStocks []models.Stock
// 		err = tx.Where("warehouse_id = ? AND product_id = ? AND product_type = ? AND batch_number = ?", stock.WarehouseId, stock.ProductId, stock.ProductType, stock.BatchNumber).
// 			Order("received_date").
// 			Find(&existingStocks).Error
// 		if err != nil {
// 			config.LogError(logger, "MainWorkflow.go", "ProcessInventoryValuation", "Querying Existing Stocks", stock, err)
// 			return accountIds, err
// 		}

// 		productDetail, err := GetProductDetail(tx, stock.ProductId, stock.ProductType)
// 		if err != nil {
// 			config.LogError(logger, "MainWorkflow.go", "ProcessInventoryValuation", "GetProductDetail", stock, err)
// 			return accountIds, err
// 		}

// 		stockDate := NormalizeDate(stock.ReceivedDate)
// 		creditNoteIds := make([]int, 0)
// 		cnStockBaseUnitValue := productDetail.PurchasePrice
// 		for _, eStock := range existingStocks {
// 			if eStock.ReferenceType == models.StockReferenceTypeCreditNote && stockDate.Equal(NormalizeDate(eStock.ReceivedDate)) {
// 				creditNoteIds = append(creditNoteIds, eStock.ID)
// 			}
// 			if eStock.ReferenceType != models.StockReferenceTypeCreditNote && !stockDate.After(NormalizeDate(eStock.ReceivedDate)) {
// 				cnStockBaseUnitValue = eStock.BaseUnitValue
// 			}
// 		}

// 		if len(creditNoteIds) > 0 {
// 			for i, eStock := range existingStocks {
// 				if slices.Contains(creditNoteIds, eStock.ID) {
// 					existingStocks[i].BaseUnitValue = cnStockBaseUnitValue
// 				}
// 			}

// 			err = tx.Exec(`UPDATE stocks SET base_unit_value = ? WHERE id IN ?`,
// 				cnStockBaseUnitValue, creditNoteIds).Error
// 			if err != nil {
// 				config.LogError(logger, "MainWorkflow.go", "ProcessInventoryValuation", "Update CreditNote Stocks", creditNoteIds, err)
// 				return accountIds, nil
// 			}
// 		}

// 		var cnStockRecords []StockRecord
// 		var cnStockDetails []StockDetail
// 		err = tx.Table("credit_note_details").
// 			Joins("INNER JOIN credit_notes ON credit_notes.id = credit_note_details.credit_note_id").
// 			Select("credit_notes.id, credit_notes.credit_note_date AS date, credit_note_details.id AS detail_id, credit_note_details.detail_qty, credit_note_details.cogs").
// 			Where("DATE(credit_notes.credit_note_date) = DATE(?) AND credit_notes.warehouse_id = ? AND credit_notes.current_status IN ? AND credit_note_details.product_id = ? AND credit_note_details.product_type = ? AND credit_note_details.batch_number = ?",
// 				stock.ReceivedDate, stock.WarehouseId,
// 				[]models.CreditNoteStatus{models.CreditNoteStatusConfirmed, models.CreditNoteStatusClosed},
// 				stock.ProductId, stock.ProductType, stock.BatchNumber).
// 			Find(&cnStockDetails).Error
// 		if err != nil {
// 			config.LogError(logger, "MainWorkflow.go", "ProcessInventoryValuation", "Querying CreditNote Details", stock, err)
// 			return accountIds, err
// 		}

// 		for _, stockDetail := range cnStockDetails {
// 			cnStockRecords = append(cnStockRecords, StockRecord{
// 				Id:        stockDetail.Id,
// 				Type:      string(models.StockReferenceTypeCreditNote),
// 				Date:      stockDetail.Date,
// 				DetailId:  stockDetail.DetailId,
// 				DetailQty: stockDetail.DetailQty,
// 				OldCogs:   stockDetail.Cogs,
// 				Cogs:      cnStockBaseUnitValue,
// 			})
// 		}

// 		for _, stockRecord := range cnStockRecords {
// 			found := false
// 			for i, record := range cnTransactionRecords {
// 				if record.Id == stockRecord.Id && record.Type == stockRecord.Type {
// 					cnTransactionRecords[i].DetailStocks = append(cnTransactionRecords[i].DetailStocks, stockRecord)
// 					cnTransactionRecords[i].TotalOldCogs = cnTransactionRecords[i].TotalOldCogs.Add(stockRecord.OldCogs)
// 					cnTransactionRecords[i].TotalCogs = cnTransactionRecords[i].TotalCogs.Add(stockRecord.Cogs)
// 					found = true
// 					break
// 				}
// 			}
// 			if !found {
// 				cnTransactionRecords = append(cnTransactionRecords, TransactionRecord{
// 					Id:                 stockRecord.Id,
// 					Type:               stockRecord.Type,
// 					DetailStocks:       []StockRecord{stockRecord},
// 					TotalCogs:          stockRecord.Cogs,
// 					TotalOldCogs:       stockRecord.OldCogs,
// 					PurchaseAccountId:  productDetail.PurchaseAccountId,
// 					InventoryAccountId: productDetail.InventoryAccountId,
// 				})
// 			}
// 		}

// 		var stockRecords []StockRecord
// 		var stockDetails []StockDetail
// 		err = tx.Table("sales_invoice_details").
// 			Joins("INNER JOIN sales_invoices ON sales_invoices.id = sales_invoice_details.sales_invoice_id").
// 			Select("sales_invoices.id, sales_invoices.invoice_date AS date, sales_invoice_details.id AS detail_id, sales_invoice_details.detail_qty, sales_invoice_details.cogs").
// 			Where("sales_invoices.warehouse_id = ? AND sales_invoices.current_status IN ? AND sales_invoice_details.product_id = ? AND sales_invoice_details.product_type = ? AND sales_invoice_details.batch_number = ?",
// 				stock.WarehouseId,
// 				[]models.SalesInvoiceStatus{models.SalesInvoiceStatusConfirmed, models.SalesInvoiceStatusPaid, models.SalesInvoiceStatusPartialPaid},
// 				stock.ProductId, stock.ProductType, stock.BatchNumber).
// 			Find(&stockDetails).Error
// 		if err != nil {
// 			config.LogError(logger, "MainWorkflow.go", "ProcessInventoryValuation", "Querying Invoice Details", stock, err)
// 			return accountIds, err
// 		}

// 		for _, stockDetail := range stockDetails {
// 			stockRecords = append(stockRecords, StockRecord{
// 				Id:        stockDetail.Id,
// 				Type:      string(models.StockReferenceTypeInvoice),
// 				Date:      stockDetail.Date,
// 				DetailId:  stockDetail.DetailId,
// 				DetailQty: stockDetail.DetailQty,
// 				OldCogs:   stockDetail.Cogs,
// 				Cogs:      decimal.Zero,
// 			})
// 		}
// 		err = tx.Table("supplier_credit_details").
// 			Joins("INNER JOIN supplier_credits ON supplier_credits.id = supplier_credit_details.supplier_credit_id").
// 			Select("supplier_credits.id, supplier_credits.supplier_credit_date AS date, supplier_credit_details.id AS detail_id, supplier_credit_details.detail_qty, supplier_credit_details.cogs").
// 			Where("supplier_credits.warehouse_id = ? AND supplier_credits.current_status IN ? AND supplier_credit_details.product_id = ? AND supplier_credit_details.product_type = ? AND supplier_credit_details.batch_number = ?",
// 				stock.WarehouseId,
// 				[]models.SupplierCreditStatus{models.SupplierCreditStatusConfirmed, models.SupplierCreditStatusClosed},
// 				stock.ProductId, stock.ProductType, stock.BatchNumber).
// 			Find(&stockDetails).Error
// 		if err != nil {
// 			config.LogError(logger, "MainWorkflow.go", "ProcessInventoryValuation", "Querying SupplierCredit Details", stock, err)
// 			return accountIds, err
// 		}

// 		for _, stockDetail := range stockDetails {
// 			stockRecords = append(stockRecords, StockRecord{
// 				Id:        stockDetail.Id,
// 				Type:      string(models.StockReferenceTypeSupplierCredit),
// 				Date:      stockDetail.Date,
// 				DetailId:  stockDetail.DetailId,
// 				DetailQty: stockDetail.DetailQty,
// 				OldCogs:   stockDetail.Cogs,
// 				Cogs:      decimal.Zero,
// 			})
// 		}

// 		// Sort stock records by Date
// 		sort.Slice(stockRecords, func(i, j int) bool {
// 			return stockRecords[i].Date.Before(stockRecords[j].Date)
// 		})

// 		// Calculate COGS
// 		var cogs decimal.Decimal
// 		for index, stockRecord := range stockRecords {
// 			processQty := stockRecord.DetailQty

// 			if len(existingStocks) > 0 && !NormalizeDate(stockRecord.Date).Before(NormalizeDate(existingStocks[0].ReceivedDate)) {
// 				for processQty.GreaterThan(decimal.Zero) && len(existingStocks) > 0 {
// 					// Get the oldest stock
// 					currentStock := existingStocks[0]

// 					if currentStock.Qty.GreaterThanOrEqual(processQty) {
// 						// If the current stock can fulfil the record
// 						cogs = cogs.Add(processQty.Mul(currentStock.BaseUnitValue))
// 						currentStock.Qty = currentStock.Qty.Sub(processQty)
// 						processQty = decimal.Zero
// 					} else {
// 						// If the current stock can only partially fulfil the record
// 						cogs = cogs.Add(currentStock.Qty.Mul(currentStock.BaseUnitValue))
// 						processQty = processQty.Sub(currentStock.Qty)
// 						currentStock.Qty = decimal.Zero
// 					}

// 					if currentStock.Qty.GreaterThan(decimal.Zero) {
// 						existingStocks[0] = currentStock
// 					} else {
// 						if len(existingStocks) > 1 {
// 							existingStocks = existingStocks[1:]
// 						} else {
// 							existingStocks = []models.Stock{}
// 						}

// 					}
// 				}
// 			}

// 			if processQty.GreaterThan(decimal.Zero) {
// 				cogs = cogs.Add(processQty.Mul(productDetail.PurchasePrice))
// 			}

// 			stockRecords[index].Cogs = cogs
// 			cogs = decimal.Zero
// 		}

// 		for _, stockRecord := range stockRecords {
// 			found := false
// 			for i, record := range transactionRecords {
// 				if record.Id == stockRecord.Id && record.Type == stockRecord.Type {
// 					transactionRecords[i].DetailStocks = append(transactionRecords[i].DetailStocks, stockRecord)
// 					transactionRecords[i].TotalOldCogs = transactionRecords[i].TotalOldCogs.Add(stockRecord.OldCogs)
// 					transactionRecords[i].TotalCogs = transactionRecords[i].TotalCogs.Add(stockRecord.Cogs)
// 					found = true
// 					break
// 				}
// 			}
// 			if !found {
// 				transactionRecords = append(transactionRecords, TransactionRecord{
// 					Id:                 stockRecord.Id,
// 					Type:               stockRecord.Type,
// 					DetailStocks:       []StockRecord{stockRecord},
// 					TotalCogs:          stockRecord.Cogs,
// 					TotalOldCogs:       stockRecord.OldCogs,
// 					PurchaseAccountId:  productDetail.PurchaseAccountId,
// 					InventoryAccountId: productDetail.InventoryAccountId,
// 				})
// 			}
// 		}
// 	}

// 	for _, record := range transactionRecords {
// 		if updatedReferenceId != 0 && record.Id == updatedReferenceId && record.Type == updatedReferenceType {
// 			err = tx.Exec(`
// 				UPDATE account_transactions INNER JOIN account_journals ON account_journals.id = account_transactions.journal_id
// 				SET base_debit = ?
// 				WHERE account_journals.reference_id = ?
// 				AND account_journals.reference_type = ?
// 				AND account_transactions.account_id = ?
// 				AND account_transactions.is_inventory_valuation = true
// 			`, record.TotalCogs, record.Id, record.Type, record.PurchaseAccountId).Error
// 			if !slices.Contains(accountIds, record.PurchaseAccountId) {
// 				accountIds = append(accountIds, record.PurchaseAccountId)
// 			}
// 			if err != nil {
// 				config.LogError(logger, "MainWorkflow.go", "ProcessInventoryValuation", "Update AccountTransactions (Purchase)", record, err)
// 				return accountIds, err
// 			}

// 			err = tx.Exec(`
// 				UPDATE account_transactions INNER JOIN account_journals ON account_journals.id = account_transactions.journal_id
// 				SET base_credit = ?
// 				WHERE account_journals.reference_id = ?
// 				AND account_journals.reference_type = ?
// 				AND account_transactions.account_id = ?
// 				AND account_transactions.is_inventory_valuation = true
// 			`, record.TotalCogs, record.Id, record.Type, record.InventoryAccountId).Error
// 			if !slices.Contains(accountIds, record.InventoryAccountId) {
// 				accountIds = append(accountIds, record.InventoryAccountId)
// 			}
// 			if err != nil {
// 				config.LogError(logger, "MainWorkflow.go", "ProcessInventoryValuation", "Update AccountTransactions (Inventory)", record, err)
// 				return accountIds, err
// 			}
// 		} else if !record.TotalCogs.Equals(record.TotalOldCogs) {
// 			err = tx.Exec(`
// 				UPDATE account_transactions INNER JOIN account_journals ON account_journals.id = account_transactions.journal_id
// 				SET base_debit = base_debit + ?
// 				WHERE account_journals.reference_id = ?
// 				AND account_journals.reference_type = ?
// 				AND account_transactions.account_id = ?
// 				AND account_transactions.is_inventory_valuation = true
// 			`, record.TotalCogs.Sub(record.TotalOldCogs), record.Id, record.Type, record.PurchaseAccountId).Error
// 			if !slices.Contains(accountIds, record.PurchaseAccountId) {
// 				accountIds = append(accountIds, record.PurchaseAccountId)
// 			}
// 			if err != nil {
// 				config.LogError(logger, "MainWorkflow.go", "ProcessInventoryValuation", "Update AccountTransactions (Purchase2)", record, err)
// 				return accountIds, err
// 			}

// 			err = tx.Exec(`
// 				UPDATE account_transactions INNER JOIN account_journals ON account_journals.id = account_transactions.journal_id
// 				SET base_credit = base_credit + ?
// 				WHERE account_journals.reference_id = ?
// 				AND account_journals.reference_type = ?
// 				AND account_transactions.account_id = ?
// 				AND account_transactions.is_inventory_valuation = true
// 			`, record.TotalCogs.Sub(record.TotalOldCogs), record.Id, record.Type, record.InventoryAccountId).Error
// 			if !slices.Contains(accountIds, record.InventoryAccountId) {
// 				accountIds = append(accountIds, record.InventoryAccountId)
// 			}
// 			if err != nil {
// 				config.LogError(logger, "MainWorkflow.go", "ProcessInventoryValuation", "Update AccountTransactions (Inventory2)", record, err)
// 				return accountIds, err
// 			}
// 		}

// 		for _, detailStock := range record.DetailStocks {
// 			if !detailStock.Cogs.Equals(detailStock.OldCogs) {
// 				if detailStock.Type == string(models.StockReferenceTypeSupplierCredit) {
// 					err = tx.Exec("UPDATE supplier_credit_details SET cogs = ? WHERE id = ?",
// 						detailStock.Cogs, detailStock.DetailId).Error
// 				}
// 				if detailStock.Type == string(models.StockReferenceTypeInvoice) {
// 					err = tx.Exec("UPDATE sales_invoice_details SET cogs = ? WHERE id = ?",
// 						detailStock.Cogs, detailStock.DetailId).Error
// 				}
// 				if err != nil {
// 					config.LogError(logger, "MainWorkflow.go", "ProcessInventoryValuation", "Update SupplierCredit/Invoice", detailStock, err)
// 					return accountIds, err
// 				}
// 			}
// 		}
// 	}

// 	for _, record := range cnTransactionRecords {
// 		if updatedReferenceId != 0 && record.Id == updatedReferenceId && record.Type == updatedReferenceType {
// 			err = tx.Exec(`
// 				UPDATE account_transactions INNER JOIN account_journals ON account_journals.id = account_transactions.journal_id
// 				SET base_credit = ?
// 				WHERE account_journals.reference_id = ?
// 				AND account_journals.reference_type = ?
// 				AND account_transactions.account_id = ?
// 				AND account_transactions.is_inventory_valuation = true
// 			`, record.TotalCogs, record.Id, record.Type, record.PurchaseAccountId).Error
// 			if !slices.Contains(accountIds, record.PurchaseAccountId) {
// 				accountIds = append(accountIds, record.PurchaseAccountId)
// 			}
// 			if err != nil {
// 				config.LogError(logger, "MainWorkflow.go", "ProcessInventoryValuation", "Update AccountTransactions (CN Purchase)", record, err)
// 				return accountIds, err
// 			}

// 			err = tx.Exec(`
// 				UPDATE account_transactions INNER JOIN account_journals ON account_journals.id = account_transactions.journal_id
// 				SET base_debit = ?
// 				WHERE account_journals.reference_id = ?
// 				AND account_journals.reference_type = ?
// 				AND account_transactions.account_id = ?
// 				AND account_transactions.is_inventory_valuation = true
// 			`, record.TotalCogs, record.Id, record.Type, record.InventoryAccountId).Error
// 			if !slices.Contains(accountIds, record.InventoryAccountId) {
// 				accountIds = append(accountIds, record.InventoryAccountId)
// 			}
// 			if err != nil {
// 				config.LogError(logger, "MainWorkflow.go", "ProcessInventoryValuation", "Update AccountTransactions (CN Inventory)", record, err)
// 				return accountIds, err
// 			}
// 		} else if !record.TotalCogs.Equals(record.TotalOldCogs) {
// 			err = tx.Exec(`
// 				UPDATE account_transactions INNER JOIN account_journals ON account_journals.id = account_transactions.journal_id
// 				SET base_credit = base_credit + ?
// 				WHERE account_journals.reference_id = ?
// 				AND account_journals.reference_type = ?
// 				AND account_transactions.account_id = ?
// 				AND account_transactions.is_inventory_valuation = true
// 			`, record.TotalCogs.Sub(record.TotalOldCogs), record.Id, record.Type, record.PurchaseAccountId).Error
// 			if !slices.Contains(accountIds, record.PurchaseAccountId) {
// 				accountIds = append(accountIds, record.PurchaseAccountId)
// 			}
// 			if err != nil {
// 				config.LogError(logger, "MainWorkflow.go", "ProcessInventoryValuation", "Update AccountTransactions (CN Purchase2)", record, err)
// 				return accountIds, err
// 			}

// 			err = tx.Exec(`
// 				UPDATE account_transactions INNER JOIN account_journals ON account_journals.id = account_transactions.journal_id
// 				SET base_debit = base_debit + ?
// 				WHERE account_journals.reference_id = ?
// 				AND account_journals.reference_type = ?
// 				AND account_transactions.account_id = ?
// 				AND account_transactions.is_inventory_valuation = true
// 			`, record.TotalCogs.Sub(record.TotalOldCogs), record.Id, record.Type, record.InventoryAccountId).Error
// 			if !slices.Contains(accountIds, record.InventoryAccountId) {
// 				accountIds = append(accountIds, record.InventoryAccountId)
// 			}
// 			if err != nil {
// 				config.LogError(logger, "MainWorkflow.go", "ProcessInventoryValuation", "Update AccountTransactions (CN Inventory 2)", record, err)
// 				return accountIds, err
// 			}
// 		}

// 		for _, detailStock := range record.DetailStocks {
// 			if !detailStock.Cogs.Equals(detailStock.OldCogs) {
// 				if detailStock.Type == string(models.StockReferenceTypeCreditNote) {
// 					err = tx.Exec("UPDATE credit_note_details SET cogs = ? WHERE id = ?",
// 						detailStock.Cogs, detailStock.DetailId).Error
// 				}
// 				if err != nil {
// 					config.LogError(logger, "MainWorkflow.go", "ProcessInventoryValuation", "Update CN", detailStock, err)
// 					return accountIds, err
// 				}
// 			}
// 		}
// 	}

// 	return accountIds, nil
// }

func GetProductDetail(tx *gorm.DB, productId int, productType models.ProductType) (ProductDetail, error) {
	var productDetail ProductDetail
	var err error
	if productType == models.ProductTypeSingle {
		err = tx.Table("products").
			Select("id, inventory_account_id, purchase_account_id, purchase_price").
			Where("id = ?", productId).First(&productDetail).Error
	} else if productType == models.ProductTypeVariant {
		err = tx.Table("product_variants").
			Select("id, inventory_account_id, purchase_account_id, purchase_price").
			Where("id = ?", productId).First(&productDetail).Error
	}
	return productDetail, err
}

func CheckIfStockNeedsInventoryTracking(tx *gorm.DB, productId int, productType models.ProductType) bool {
	if productType == models.ProductTypeSingle {
		var product models.Product
		err := tx.Where("id = ?", productId).First(&product).Error
		if err != nil {
			return false
		}
		if product.InventoryAccountId > 0 {
			return true
		}
	} else if productType == models.ProductTypeVariant {
		var product models.ProductVariant
		err := tx.Where("id = ?", productId).First(&product).Error
		if err != nil {
			return false
		}
		if product.InventoryAccountId > 0 {
			return true
		}
	}
	return false
}

func GetExistingAccountJournal(tx *gorm.DB, journalId int, journalType models.AccountReferenceType) (*models.AccountJournal, []int, []int, error) {
	var accountJournal models.AccountJournal
	// Phase 1: prefer the active journal for the reference.
	// Active journal: is_reversal=false AND reversed_by_journal_id IS NULL.
	err := tx.Preload("AccountTransactions").
		Where("reference_id = ? AND reference_type = ? AND is_reversal = 0 AND reversed_by_journal_id IS NULL", journalId, journalType).
		Order("id DESC").
		First(&accountJournal).Error
	if err != nil {
		return nil, nil, nil, err
	}
	transactionIds := make([]int, len(accountJournal.AccountTransactions))
	accountIds := make([]int, 0)
	for i, transaction := range accountJournal.AccountTransactions {
		transactionIds[i] = transaction.ID
		if !slices.Contains(accountIds, transaction.AccountId) {
			accountIds = append(accountIds, transaction.AccountId)
		}
	}

	return &accountJournal, transactionIds, accountIds, nil
}

// Get Exchange Rate
// func GetExchangeRate(tx *gorm.DB, currencyId int, transactionDateTime time.Time) (decimal.Decimal, error) {
// 	var currencyExchanges []*models.CurrencyExchange
// 	exchangeRate := decimal.NewFromInt(0)
// 	err := tx.Where("foreign_currency_id = ? AND exchange_date <= ?", currencyId, transactionDateTime).Order("exchange_date DESC").Limit(1).Find(&currencyExchanges).Error
// 	if err != nil {
// 		return exchangeRate, err
// 	}
// 	if len(currencyExchanges) > 0 {
// 		exchangeRate = currencyExchanges[0].ExchangeRate
// 	}
// 	// if exchangeRate.IsZero() {
// 	// 	var currency *models.Currency
// 	// 	err = tx.Where("id", currencyId).First(&currency).Error
// 	// 	if err != nil {
// 	// 		return exchangeRate, err
// 	// 	}
// 	// 	exchangeRate = currency.ExchangeRate
// 	// }
// 	return exchangeRate, err
// }

// Get latest base transactions for each account to get the closing balances
func GetLatestBaseTransactionByAccount(tx *gorm.DB, currency_id int, accountIds []int, transactionDateTime time.Time) ([]*models.AccountTransaction, error) {
	var lastAccTransacts []*models.AccountTransaction
	err := tx.Raw(`
		WITH LatestValues AS (
			SELECT
				*,
				ROW_NUMBER() OVER (PARTITION BY base_currency_id, account_id ORDER BY transaction_date_time DESC, id DESC) AS rn
			FROM
				account_transactions
			WHERE base_currency_id = ? AND account_id IN ? AND transaction_date_time < ?
		)
		SELECT
			*
		FROM
			LatestValues
		WHERE
			rn = 1
		`, currency_id, accountIds, transactionDateTime).Find(&lastAccTransacts).Error
	return lastAccTransacts, err
}

// Get latest foreign transactions for each account to get the closing balances
func GetLatestForeignTransactionByAccount(tx *gorm.DB, currencyId int, accountIds []int, transactionDateTime time.Time) ([]*models.AccountTransaction, error) {
	var lastAccTransacts []*models.AccountTransaction
	err := tx.Raw(`
		WITH LatestValues AS (
			SELECT
				*,
				ROW_NUMBER() OVER (PARTITION BY foreign_currency_id, account_id ORDER BY transaction_date_time DESC, id DESC) AS rn
			FROM
				account_transactions
			WHERE foreign_currency_id = ? AND account_id IN ? AND transaction_date_time < ?
		)
		SELECT
			*
		FROM
			LatestValues
		WHERE
			rn = 1
		`, currencyId, accountIds, transactionDateTime).Find(&lastAccTransacts).Error
	return lastAccTransacts, err
}

// func UpdateBalances(tx *gorm.DB, businessId string, baseCurrencyId int, branchId int, accountIds []int, transactionDateTime time.Time, latestBaseTransactions []*models.AccountTransaction, foreignCurrencyId int, latestForeignTransactions []*models.AccountTransaction) error {
func UpdateBalances(tx *gorm.DB, logger *logrus.Logger, businessId string, baseCurrencyId int, branchId int, accountIds []int, transactionDateTime time.Time, foreignCurrencyId int) error {
	// err := updateClosingBaseBalances(tx, baseCurrencyId, branchId, accountIds, transactionDateTime, latestBaseTransactions)
	// if err != nil {
	// 	return err
	// }

	// if baseCurrencyId != foreignCurrencyId {
	// 	err = updateClosingForeignBalances(tx, foreignCurrencyId, branchId, accountIds, transactionDateTime, latestForeignTransactions)
	// 	if err != nil {
	// 		return err
	// 	}
	// }

	business, err := models.GetBusinessById2(tx, businessId)
	if err != nil {
		return errors.New("business id is required")
	}
	timezone := "Asia/Yangon"
	if business.Timezone != "" {
		timezone = business.Timezone
	}

	systemAccounts, err := models.GetSystemAccounts(businessId)
	if err != nil {
		config.LogError(logger, "MainWorkflow.go", "UpdateBalances", "GetSystemAccounts", businessId, err)
		return err
	}

	closingBalanceUpdateAccountIds := make([]int, 0)
	for _, accId := range accountIds {
		if accId == systemAccounts[models.AccountCodeAccountsPayable] || accId == systemAccounts[models.AccountCodeAccountsReceivable] {
			closingBalanceUpdateAccountIds = append(closingBalanceUpdateAccountIds, accId)
		}
	}

	if len(closingBalanceUpdateAccountIds) > 0 {
		latestBaseTransactions, err := GetLatestBaseTransactionByAccount(tx, baseCurrencyId, closingBalanceUpdateAccountIds, transactionDateTime)
		if err != nil {
			return err
		}
		err = updateClosingBaseBalances(tx, baseCurrencyId, closingBalanceUpdateAccountIds, transactionDateTime, latestBaseTransactions)
		if err != nil {
			return err
		}

		if foreignCurrencyId != 0 && baseCurrencyId != foreignCurrencyId {
			latestForeignTransactions, err := GetLatestForeignTransactionByAccount(tx, foreignCurrencyId, closingBalanceUpdateAccountIds, transactionDateTime)
			if err != nil {
				return err
			}
			err = updateClosingForeignBalances(tx, foreignCurrencyId, closingBalanceUpdateAccountIds, transactionDateTime, latestForeignTransactions)
			if err != nil {
				return err
			}
		}
	}

	err = upsertBaseCurrencyDailyBalances(tx, timezone, businessId, baseCurrencyId, branchId, accountIds, transactionDateTime)
	if err != nil {
		config.LogError(logger, "MainWorkflow.go", "UpdateBalances", "UpsertBaseCurrencyDailyBalances", nil, err)
		return err
	}

	err = deleteBaseCurrencyDailyBalances(tx, timezone, businessId, baseCurrencyId, branchId, accountIds, transactionDateTime)
	if err != nil {
		config.LogError(logger, "MainWorkflow.go", "UpdateBalances", "DeleteBaseCurrencyDailyBalances", nil, err)
		return err
	}

	// Phase B: maintain DailySummary (derived aggregate) for fast dashboards.
	// We update for both branch 0 (company-wide) and the specific branch.
	//
	// IMPORTANT: derived aggregates must not block ledger posting.
	// If these fail (e.g. missing table during rollout), we log and continue.
	if err := upsertDailySummaries(tx, timezone, businessId, baseCurrencyId, []int{0, branchId}, transactionDateTime); err != nil {
		config.LogError(logger, "MainWorkflow.go", "UpdateBalances", "UpsertDailySummaries (non-fatal)", nil, err)
	}
	if err := deleteDailySummaries(tx, timezone, businessId, baseCurrencyId, []int{0, branchId}, transactionDateTime); err != nil {
		config.LogError(logger, "MainWorkflow.go", "UpdateBalances", "DeleteDailySummaries (non-fatal)", nil, err)
	}

	if foreignCurrencyId != 0 && baseCurrencyId != foreignCurrencyId {
		err = upsertForeignCurrencyDailyBalances(tx, timezone, businessId, foreignCurrencyId, branchId, accountIds, transactionDateTime)
		if err != nil {
			config.LogError(logger, "MainWorkflow.go", "UpdateBalances", "UpsertForeignCurrencyDailyBalances", nil, err)
			return err
		}

		err = deleteForeignCurrencyDailyBalances(tx, timezone, businessId, foreignCurrencyId, branchId, accountIds, transactionDateTime)
		if err != nil {
			config.LogError(logger, "MainWorkflow.go", "UpdateBalances", "DeleteForeignCurrencyDailyBalances", nil, err)
			return err
		}

	}
	return nil
}

// Daily summaries are derived from account_currency_daily_balances + account main types.
// This keeps dashboard income/expense queries fast and consistent.
func upsertDailySummaries(tx *gorm.DB, timezone string, businessId string, currencyId int, branchIds []int, transactionDateTime time.Time) error {
	return tx.Exec(`
		INSERT INTO daily_summaries (business_id, currency_id, branch_id, transaction_date, total_income, total_expense, created_at, updated_at)
		SELECT
			acb.business_id,
			acb.currency_id,
			acb.branch_id,
			acb.transaction_date,
			COALESCE(SUM(CASE WHEN a.main_type = 'Income' THEN -acb.balance ELSE 0 END), 0) AS total_income,
			COALESCE(SUM(CASE WHEN a.main_type = 'Expense' THEN  acb.balance ELSE 0 END), 0) AS total_expense,
			NOW(),
			NOW()
		FROM account_currency_daily_balances acb
		JOIN accounts a ON a.id = acb.account_id
		WHERE
			acb.business_id = ?
			AND acb.currency_id = ?
			AND acb.branch_id IN ?
			AND acb.transaction_date >= DATE(CONVERT_TZ(?, 'UTC', ?))
			AND a.main_type IN ('Income', 'Expense')
		GROUP BY
			acb.business_id, acb.currency_id, acb.branch_id, acb.transaction_date
		ON DUPLICATE KEY UPDATE
			total_income = VALUES(total_income),
			total_expense = VALUES(total_expense),
			updated_at = NOW()
	`, businessId, currencyId, branchIds, transactionDateTime, timezone).Error
}

// Remove stale daily summary rows when activity for a date disappears due to reversals/updates.
func deleteDailySummaries(tx *gorm.DB, timezone string, businessId string, currencyId int, branchIds []int, transactionDateTime time.Time) error {
	return tx.Exec(`
		DELETE ds
		FROM daily_summaries ds
		LEFT JOIN (
			SELECT
				acb.business_id,
				acb.currency_id,
				acb.branch_id,
				acb.transaction_date
			FROM account_currency_daily_balances acb
			JOIN accounts a ON a.id = acb.account_id
			WHERE
				acb.business_id = ?
				AND acb.currency_id = ?
				AND acb.branch_id IN ?
				AND acb.transaction_date >= DATE(CONVERT_TZ(?, 'UTC', ?))
				AND a.main_type IN ('Income', 'Expense')
			GROUP BY
				acb.business_id, acb.currency_id, acb.branch_id, acb.transaction_date
		) agg
			ON agg.business_id = ds.business_id
			AND agg.currency_id = ds.currency_id
			AND agg.branch_id = ds.branch_id
			AND agg.transaction_date = ds.transaction_date
		WHERE
			ds.business_id = ?
			AND ds.currency_id = ?
			AND ds.branch_id IN ?
			AND ds.transaction_date >= DATE(CONVERT_TZ(?, 'UTC', ?))
			AND agg.transaction_date IS NULL
	`, businessId, currencyId, branchIds, transactionDateTime, timezone, businessId, currencyId, branchIds, transactionDateTime, timezone).Error
}

// Update closing base balances for current transaction and the following transactions
func updateClosingBaseBalances(tx *gorm.DB, currencyId int, accountIds []int, transactionDateTime time.Time, latestTransactions []*models.AccountTransaction) error {
	for _, accountId := range accountIds {

		baseClosingBalance := decimal.NewFromInt(0)

		for _, transact := range latestTransactions {
			if transact.AccountId == accountId {
				baseClosingBalance = transact.BaseClosingBalance
				break
			}
		}

		err := tx.Exec(`
		UPDATE account_transactions AS t
		JOIN (SELECT
			id,
			transaction_date_time,
			account_id,
			base_debit,
			base_credit,
			? + SUM(base_debit - base_credit) OVER (PARTITION BY account_id 
				ORDER BY transaction_date_time, id ROWS BETWEEN UNBOUNDED PRECEDING AND CURRENT ROW) AS closing_base_balance
		FROM account_transactions
		WHERE base_currency_id = ? AND account_id = ? AND transaction_date_time >= ?
		) AS temp
		ON t.id = temp.id
		SET 
		t.base_closing_balance = temp.closing_base_balance
		WHERE t.id > 0
		`, baseClosingBalance, currencyId, accountId, transactionDateTime).Error
		if err != nil {
			return err
		}
	}
	return nil
}

// Update closing foreign balances for current transaction and the following transactions
func updateClosingForeignBalances(tx *gorm.DB, currencyId int, accountIds []int, transactionDateTime time.Time, latestTransactions []*models.AccountTransaction) error {
	for _, accountId := range accountIds {

		foreignClosingBalance := decimal.NewFromInt(0)

		for _, transact := range latestTransactions {
			if transact.AccountId == accountId {
				foreignClosingBalance = transact.ForeignClosingBalance
				break
			}
		}

		err := tx.Exec(`
		UPDATE account_transactions AS t
		JOIN (SELECT
			id,
			transaction_date_time,
			account_id,
			foreign_debit,
			foreign_credit,
			? + SUM(foreign_debit - foreign_credit) OVER (PARTITION BY foreign_currency_id, account_id 
				ORDER BY transaction_date_time,id ROWS BETWEEN UNBOUNDED PRECEDING AND CURRENT ROW) AS closing_foreign_balance
		FROM account_transactions
		WHERE foreign_currency_id = ? AND account_id = ? AND transaction_date_time >= ?
		) AS temp
		ON t.id = temp.id
		SET 
		t.foreign_closing_balance = temp.closing_foreign_balance
		WHERE t.id > 0
		`, foreignClosingBalance, currencyId, accountId, transactionDateTime).Error
		if err != nil {
			return err
		}
	}
	return nil
}

func upsertBaseCurrencyDailyBalances(tx *gorm.DB, timezone string, businessId string, currencyId int, branchId int, accountIds []int, transactionDateTime time.Time) error {
	branchIds := []int{0, branchId}
	var lastDailyBalances []*models.AccountCurrencyDailyBalance
	// tx = tx.Session(&gorm.Session{
	// 	Logger: config.WriteGormLog(), // Apply the custom logger
	// })
	err := tx.Raw(`
	WITH LatestValues AS (
		SELECT
			*,
			ROW_NUMBER() OVER (PARTITION BY currency_id, branch_id, account_id ORDER BY transaction_date DESC) AS rn
		FROM
			account_currency_daily_balances
		WHERE currency_id = ? AND branch_id IN ? AND account_id IN ? AND transaction_date < DATE(CONVERT_TZ(?, 'UTC', ?))
	)
	SELECT
		*
	FROM
		LatestValues
	WHERE
		rn = 1
	`, currencyId, branchIds, accountIds, transactionDateTime, timezone).Find(&lastDailyBalances).Error
	if err != nil {
		return err
	}

	for _, brId := range branchIds {
		for _, accountId := range accountIds {

			prevBalance := decimal.NewFromInt(0)

			for _, balance := range lastDailyBalances {
				if balance.CurrencyId == currencyId && balance.BranchId == brId && balance.AccountId == accountId {
					prevBalance = balance.RunningBalance
					break
				}
			}

			if brId == 0 {
				err = tx.Exec(`
				INSERT INTO account_currency_daily_balances (business_id, currency_id, account_id, transaction_date, branch_id, 
					debit, credit, balance, running_balance) 
			SELECT * FROM (
				SELECT *, t1.total_debit - t1.total_credit AS total,
				? + SUM(t1.total_debit - t1.total_credit) OVER (PARTITION BY business_id, account_id, currency_id ORDER BY transaction_date) AS running_total
			FROM
			(SELECT 
				business_id,
				base_currency_id AS currency_id,
				account_id, 
				DATE(CONVERT_TZ(transaction_date_time, 'UTC', ?)) AS transaction_date, 
				0, 
				SUM(base_debit) AS total_debit, 
				SUM(base_credit) AS total_credit
			FROM 
				account_transactions
			WHERE
				business_id = ?
				AND base_currency_id = ?
				AND account_id = ?
				AND DATE(CONVERT_TZ(transaction_date_time, 'UTC', ?)) >= DATE(CONVERT_TZ(?, 'UTC', ?))
			GROUP BY 
				business_id,
				base_currency_id,
				account_id, 
				transaction_date
			) AS t1
			) AS t
			ON DUPLICATE KEY UPDATE debit = total_debit, credit = total_credit, balance = total, running_balance = running_total
					`, prevBalance, timezone, businessId, currencyId, accountId, timezone, transactionDateTime, timezone).Error
				if err != nil {
					return err
				}
			} else {
				err = tx.Exec(`
				INSERT INTO account_currency_daily_balances (business_id, currency_id, account_id, transaction_date, branch_id, 
					debit, credit, balance, running_balance) 
			SELECT * FROM (
				SELECT *, t1.total_debit - t1.total_credit AS total,
				? + SUM(t1.total_debit - t1.total_credit) OVER (PARTITION BY business_id, branch_id, account_id, currency_id ORDER BY transaction_date) AS running_total
			FROM
			(SELECT 
				business_id,
				base_currency_id AS currency_id,
				account_id, 
				DATE(CONVERT_TZ(transaction_date_time, 'UTC', ?)) AS transaction_date, 
				branch_id, 
				SUM(base_debit) AS total_debit, 
				SUM(base_credit) AS total_credit
			FROM 
				account_transactions
			WHERE
				business_id = ?
				AND base_currency_id = ?
				AND branch_id = ?
				AND account_id = ?
				AND DATE(CONVERT_TZ(transaction_date_time, 'UTC', ?)) >= DATE(CONVERT_TZ(?, 'UTC', ?))
			GROUP BY 
				business_id,
				base_currency_id,
				account_id, 
				branch_id, 
				transaction_date
			) AS t1
			) AS t
			ON DUPLICATE KEY UPDATE debit = total_debit, credit = total_credit, balance = total, running_balance = running_total
					`, prevBalance, timezone, businessId, currencyId, brId, accountId, timezone, transactionDateTime, timezone).Error
				if err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func deleteBaseCurrencyDailyBalances(tx *gorm.DB, timezone string, businessId string, currencyId int, branchId int, accountIds []int, transactionDateTime time.Time) error {
	// tx = tx.Session(&gorm.Session{
	// 	Logger: config.WriteGormLog(), // Apply the custom logger
	// })
	err := tx.Exec(`
	DELETE acdb
FROM account_currency_daily_balances AS acdb
LEFT JOIN account_transactions AS act ON acdb.account_id = act.account_id
    AND acdb.transaction_date = DATE(CONVERT_TZ(act.transaction_date_time, 'UTC', ?))
    AND acdb.branch_id = act.branch_id
    AND acdb.currency_id = act.base_currency_id
WHERE 
    acdb.business_id = ? AND acdb.currency_id = ? AND acdb.branch_id = ? 
	AND acdb.account_id IN ? AND acdb.transaction_date >= DATE(CONVERT_TZ(?, 'UTC', ?)) AND act.id IS NULL;
		`, timezone, businessId, currencyId, branchId, accountIds, transactionDateTime, timezone).Error
	if err != nil {
		return err
	}

	err = tx.Exec(`
	DELETE acdb
FROM account_currency_daily_balances AS acdb
LEFT JOIN account_transactions AS act ON acdb.account_id = act.account_id
    AND acdb.transaction_date = DATE(CONVERT_TZ(act.transaction_date_time, 'UTC', ?))
    AND acdb.currency_id = act.base_currency_id
WHERE 
    acdb.business_id = ? AND acdb.currency_id = ? 
	AND acdb.account_id IN ? AND acdb.transaction_date >= DATE(CONVERT_TZ(?, 'UTC', ?)) AND act.id IS NULL;
		`, timezone, businessId, currencyId, accountIds, transactionDateTime, timezone).Error
	if err != nil {
		return err
	}
	return nil
}

func upsertForeignCurrencyDailyBalances(tx *gorm.DB, timezone string, businessId string, currencyId int, branchId int, accountIds []int, transactionDateTime time.Time) error {
	branchIds := []int{0, branchId}
	var lastDailyBalances []*models.AccountCurrencyDailyBalance
	err := tx.Raw(`
	WITH LatestValues AS (
		SELECT
			*,
			ROW_NUMBER() OVER (PARTITION BY currency_id, branch_id, account_id ORDER BY transaction_date DESC) AS rn
		FROM
			account_currency_daily_balances
		WHERE currency_id = ? AND branch_id IN ? AND account_id IN ? AND transaction_date < DATE(CONVERT_TZ(?, 'UTC', ?))
	)
	SELECT
		*
	FROM
		LatestValues
	WHERE
		rn = 1
	`, currencyId, branchIds, accountIds, transactionDateTime, timezone).Find(&lastDailyBalances).Error
	if err != nil {
		return err
	}

	for _, brId := range branchIds {
		for _, accountId := range accountIds {

			prevBalance := decimal.NewFromInt(0)
			prevBaseBalance := decimal.NewFromInt(0)

			for _, balance := range lastDailyBalances {
				if balance.CurrencyId == currencyId && balance.BranchId == brId && balance.AccountId == accountId {
					prevBalance = balance.RunningBalance
					prevBaseBalance = balance.RunningBaseBalance
					break
				}
			}

			if brId == 0 {
				err := tx.Exec(`
					INSERT INTO account_currency_daily_balances (business_id, currency_id, account_id, transaction_date, branch_id, 
						debit, credit, base_debit, base_credit, balance, running_balance, running_base_balance) 
				SELECT * FROM (
					SELECT *, t1.total_debit - t1.total_credit AS total,
					? + SUM(t1.total_debit - t1.total_credit) OVER (PARTITION BY business_id, account_id, currency_id ORDER BY transaction_date) AS running_total,
					? + SUM(t1.total_base_debit - t1.total_base_credit) OVER (PARTITION BY business_id, account_id, currency_id ORDER BY transaction_date) AS running_base_total
				FROM
				(SELECT 
					business_id,
					foreign_currency_id AS currency_id,
					account_id, 
					DATE(CONVERT_TZ(transaction_date_time, 'UTC', ?)) AS transaction_date, 
					0, 
					SUM(foreign_debit) AS total_debit, 
					SUM(foreign_credit) AS total_credit,
					SUM(foreign_debit * exchange_rate) AS total_base_debit, 
        			SUM(foreign_credit * exchange_rate) AS total_base_credit
				FROM 
					account_transactions
				WHERE
					business_id = ?
					AND foreign_currency_id = ?
					AND account_id = ?
					AND DATE(CONVERT_TZ(transaction_date_time, 'UTC', ?)) >= DATE(CONVERT_TZ(?, 'UTC', ?))
				GROUP BY 
					business_id,
					foreign_currency_id,
					account_id, 
					transaction_date
				) AS t1
				) AS t
				ON DUPLICATE KEY UPDATE debit = total_debit, credit = total_credit, 
				base_debit = total_base_debit, base_credit = total_base_credit,
				balance = total, running_balance = running_total, running_base_balance = running_base_total
				`, prevBalance, prevBaseBalance, timezone, businessId, currencyId, accountId, timezone, transactionDateTime, timezone).Error
				if err != nil {
					return err
				}
			} else {
				err := tx.Exec(`
					INSERT INTO account_currency_daily_balances (business_id, currency_id, account_id, transaction_date, branch_id, 
						debit, credit, base_debit, base_credit, balance, running_balance, running_base_balance) 
				SELECT * FROM (
					SELECT *, t1.total_debit - t1.total_credit AS total,
					? + SUM(t1.total_debit - t1.total_credit) OVER (PARTITION BY business_id, branch_id, account_id, currency_id ORDER BY transaction_date) AS running_total,
					? + SUM(t1.total_base_debit - t1.total_base_credit) OVER (PARTITION BY business_id, account_id, currency_id ORDER BY transaction_date) AS running_base_total
				FROM
				(SELECT 
					business_id,
					foreign_currency_id AS currency_id,
					account_id, 
					DATE(CONVERT_TZ(transaction_date_time, 'UTC', ?)) AS transaction_date, 
					branch_id, 
					SUM(foreign_debit) AS total_debit, 
					SUM(foreign_credit) AS total_credit,
					SUM(foreign_debit * exchange_rate) AS total_base_debit, 
        			SUM(foreign_credit * exchange_rate) AS total_base_credit
				FROM 
					account_transactions
				WHERE
					business_id = ?
					AND foreign_currency_id = ?
					AND branch_id = ?
					AND account_id = ?
					AND DATE(CONVERT_TZ(transaction_date_time, 'UTC', ?)) >= DATE(CONVERT_TZ(?, 'UTC', ?))
				GROUP BY 
					business_id,
					foreign_currency_id,
					account_id, 
					branch_id, 
					transaction_date
				) AS t1
				) AS t
				ON DUPLICATE KEY UPDATE debit = total_debit, credit = total_credit, 
				base_debit = total_base_debit, base_credit = total_base_credit,
				balance = total, running_balance = running_total, running_base_balance = running_base_total
				`, prevBalance, prevBaseBalance, timezone, businessId, currencyId, brId, accountId, timezone, transactionDateTime, timezone).Error
				if err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func deleteForeignCurrencyDailyBalances(tx *gorm.DB, timezone string, businessId string, currencyId int, branchId int, accountIds []int, transactionDateTime time.Time) error {
	err := tx.Exec(`
	DELETE acdb
FROM account_currency_daily_balances AS acdb
LEFT JOIN account_transactions AS act ON acdb.account_id = act.account_id
    AND acdb.transaction_date = DATE(CONVERT_TZ(act.transaction_date_time, 'UTC', ?))
    AND acdb.branch_id = act.branch_id
    AND acdb.currency_id = act.foreign_currency_id
WHERE 
    acdb.business_id = ? AND acdb.currency_id = ? AND acdb.branch_id = ? 
	AND acdb.account_id IN ? AND acdb.transaction_date >= DATE(CONVERT_TZ(?, 'UTC', ?)) AND act.id IS NULL;
		`, timezone, businessId, currencyId, branchId, accountIds, transactionDateTime, timezone).Error
	if err != nil {
		return err
	}

	err = tx.Exec(`
	DELETE acdb
FROM account_currency_daily_balances AS acdb
LEFT JOIN account_transactions AS act ON acdb.account_id = act.account_id
    AND acdb.transaction_date = DATE(CONVERT_TZ(act.transaction_date_time, 'UTC', ?))
    AND acdb.currency_id = act.foreign_currency_id
WHERE 
    acdb.business_id = ? AND acdb.currency_id = ?
	AND acdb.account_id IN ? AND acdb.transaction_date >= DATE(CONVERT_TZ(?, 'UTC', ?)) AND act.id IS NULL;
		`, timezone, businessId, currencyId, accountIds, transactionDateTime, timezone).Error
	if err != nil {
		return err
	}
	return nil
}

func updateBankingClosingBaseBalances(tx *gorm.DB, accountIds []int, transactionDateTime time.Time) error {
	err := tx.Exec(`
		UPDATE banking_transactions bt
		JOIN account_transactions at
		ON bt.from_account_id = at.account_id
		AND bt.business_id = at.business_id
		SET bt.from_account_closing_balance = at.base_closing_balance
		WHERE at.banking_transaction_id = bt.ID
		AND bt.ID > 0
		AND at.account_id IN ? AND at.transaction_date_time >= ?
	`, accountIds, transactionDateTime).Error
	if err != nil {
		return err
	}
	err = tx.Exec(`
		UPDATE banking_transactions bt
		JOIN account_transactions at
		ON bt.to_account_id = at.account_id
		AND bt.business_id = at.business_id
		SET bt.to_account_closing_balance = at.base_closing_balance
		WHERE at.banking_transaction_id = bt.ID
		AND bt.ID > 0
		AND at.account_id IN ? AND at.transaction_date_time >= ?
	`, accountIds, transactionDateTime).Error
	if err != nil {
		return err
	}
	return nil
}

func updateBankingClosingForeignBalances(tx *gorm.DB, accountIds []int, transactionDateTime time.Time) error {
	err := tx.Exec(`
		UPDATE banking_transactions bt
		JOIN account_transactions at
		ON bt.from_account_id = at.account_id
		AND bt.business_id = at.business_id
		SET bt.from_account_closing_balance = at.foreign_closing_balance
		WHERE at.banking_transaction_id = bt.ID
		AND bt.ID > 0
		AND at.account_id IN ? AND at.transaction_date_time >= ?
	`, accountIds, transactionDateTime).Error
	if err != nil {
		return err
	}
	err = tx.Exec(`
		UPDATE banking_transactions bt
		JOIN account_transactions at
		ON bt.to_account_id = at.account_id
		AND bt.business_id = at.business_id
		SET bt.to_account_closing_balance = at.foreign_closing_balance
		WHERE at.banking_transaction_id = bt.ID
		AND bt.ID > 0
		AND at.account_id IN ? AND at.transaction_date_time >= ?
	`, accountIds, transactionDateTime).Error
	if err != nil {
		return err
	}
	return nil
}

func GetAccount(tx *gorm.DB, id int) (*models.Account, error) {
	// find in redis
	var acc *models.Account
	var err error
	acc, err = utils.RetrieveRedis[models.Account](id)
	if err != nil {
		return nil, err
	}
	err = tx.First(&acc, id).Error
	return acc, err
}

func UpdateBankBalances(tx *gorm.DB, baseCurrencyId int, branchId int, accountIds []int, transactionDateTime time.Time) error {

	baseAccountIds := make([]int, 0)
	foreignAccountIds := make(map[int][]int)

	for _, accId := range accountIds {
		acc, err := GetAccount(tx, accId)
		if err != nil {
			return err
		}
		if acc.DetailType == models.AccountDetailTypeBank || acc.DetailType == models.AccountDetailTypeCash {
			if acc.CurrencyId != baseCurrencyId {
				// add to foreignAccountIds according to acc.CurrencyId
				foreignAccountIds[acc.CurrencyId] = append(foreignAccountIds[acc.CurrencyId], acc.ID)
			} else {
				baseAccountIds = append(baseAccountIds, acc.ID)
			}
		}
	}

	// Process base currency accounts
	if len(baseAccountIds) > 0 {
		latestBaseTransactions, err := GetLatestBaseTransactionByAccount(tx, baseCurrencyId, baseAccountIds, transactionDateTime)
		if err != nil {
			return err
		}
		err = updateClosingBaseBalances(tx, baseCurrencyId, baseAccountIds, transactionDateTime, latestBaseTransactions)
		if err != nil {
			return err
		}
		err = updateBankingClosingBaseBalances(tx, baseAccountIds, transactionDateTime)
		if err != nil {
			return err
		}
	}

	// Process foreign currency accounts
	for currencyId, fAccountIds := range foreignAccountIds {
		if len(fAccountIds) > 0 {
			latestForeignTransactions, err := GetLatestForeignTransactionByAccount(tx, currencyId, fAccountIds, transactionDateTime)
			if err != nil {
				return err
			}
			err = updateClosingForeignBalances(tx, currencyId, fAccountIds, transactionDateTime, latestForeignTransactions)
			if err != nil {
				return err
			}
			err = updateBankingClosingForeignBalances(tx, fAccountIds, transactionDateTime)
			if err != nil {
				return err
			}
		}
	}
	return nil
}
