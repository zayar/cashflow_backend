package workflow

import (
	"context"
	"encoding/json"
	"errors"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/mmdatafocus/books_backend/config"
	"github.com/mmdatafocus/books_backend/models"
	"github.com/mmdatafocus/books_backend/utils"
	"github.com/shopspring/decimal"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

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
		// token ends at space or comma
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
	return fifoScope{
		warehouseId: wid,
		productId:   pid,
		productType: models.ProductType(ptStr),
		batch:       batch,
	}, true
}

func rebuildInventoryForScope(tx *gorm.DB, logger *logrus.Logger, businessId string, scope fifoScope, fallbackStart time.Time) error {
	if tx == nil {
		return errors.New("rebuild inventory: tx is nil")
	}
	if businessId == "" || scope.warehouseId <= 0 || scope.productId <= 0 {
		return errors.New("rebuild inventory: invalid scope")
	}
	// Best-effort: rebuild from earliest ledger date for this key; fall back to document date.
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

func ProcessTransferOrderWorkflow(tx *gorm.DB, logger *logrus.Logger, msg config.PubSubMessage) error {

	var accountJournalId int
	var accountIds []int
	var foreignCurrencyId int
	business, err := models.GetBusinessById2(tx, msg.BusinessId)
	if err != nil {
		config.LogError(logger, "TransferOrderWorkflow.go", "ProcessTransferOrderWorkflow", "GetBusiness", msg.BusinessId, err)
		return err
	}
	if msg.Action == string(models.PubSubMessageActionCreate) {

		var transferOrder models.TransferOrder
		err := json.Unmarshal([]byte(msg.NewObj), &transferOrder)
		if err != nil {
			config.LogError(logger, "TransferOrderWorkflow.go", "ProcessTransferOrderWorkflow > Create", "Unmarshal msg.NewObj", msg.NewObj, err)
			return err
		}
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		ctx = context.WithValue(ctx, utils.ContextKeyBusinessId, transferOrder.BusinessId)
		sourceWarehouse, err := models.GetWarehouse(ctx, transferOrder.SourceWarehouseId)
		if err != nil {
			config.LogError(logger, "TransferOrderWorkflow.go", "ProcessTransferOrderWorkflow > Create", "GetSourceWarehouse", transferOrder.SourceWarehouseId, err)
			return err
		}
		destinationWarehouse, err := models.GetWarehouse(ctx, transferOrder.DestinationWarehouseId)
		if err != nil {
			config.LogError(logger, "TransferOrderWorkflow.go", "ProcessTransferOrderWorkflow > Create", "GetDestinationWarehouse", transferOrder.DestinationWarehouseId, err)
			return err
		}

		accountJournalId, accountIds, foreignCurrencyId, err = CreateTransferOrder(tx, logger, msg.ID, msg.ReferenceType, msg.BusinessId, *business, transferOrder, *sourceWarehouse, *destinationWarehouse)
		if err != nil {
			config.LogError(logger, "TransferOrderWorkflow.go", "ProcessTransferOrderWorkflow > Create", "CreateTransferOrder", nil, err)
			return err
		}

		err = UpdateBalances(tx, logger, msg.BusinessId, business.BaseCurrencyId, sourceWarehouse.BranchId, accountIds, transferOrder.TransferDate, foreignCurrencyId)
		if err != nil {
			config.LogError(logger, "TransferOrderWorkflow.go", "ProcessTransferOrderWorkflow > Create", "UpdateBalances", nil, err)
			return err
		}
		if sourceWarehouse.BranchId != destinationWarehouse.BranchId {
			err = UpdateBalances(tx, logger, msg.BusinessId, business.BaseCurrencyId, destinationWarehouse.BranchId, accountIds, transferOrder.TransferDate, foreignCurrencyId)
			if err != nil {
				config.LogError(logger, "TransferOrderWorkflow.go", "ProcessTransferOrderWorkflow > Create", "UpdateBalancesOfDestinationBranch", nil, err)
				return err
			}
		}
	}
	err = tx.Model(&models.PubSubMessageRecord{}).Where("id=?", msg.ID).Updates(map[string]interface{}{"account_journal_id": accountJournalId, "is_processed": true}).Error
	if err != nil {
		config.LogError(logger, "TransferOrderWorkflow.go", "ProcessTransferOrderWorkflow", "UpdatePubSubMessageRecord", accountJournalId, err)
		return err
	}
	return nil
}

func CreateTransferOrder(tx *gorm.DB, logger *logrus.Logger, recordId int, recordType string, businessId string, business models.Business, transferOrder models.TransferOrder, sourceWarehouse models.Warehouse, destinationWarehouse models.Warehouse) (int, []int, int, error) {

	systemAccounts, err := models.GetSystemAccounts(businessId)
	if err != nil {
		config.LogError(logger, "TransferOrderWorkflow.go", "CreateTransferOrder", "GetSystemAccounts", businessId, err)
		return 0, nil, 0, err
	}

	transactionTime := transferOrder.TransferDate
	baseCurrencyId := business.BaseCurrencyId
	foreignCurrencyId := business.BaseCurrencyId
	sourceBranchId := sourceWarehouse.BranchId
	destinationBranchId := destinationWarehouse.BranchId

	accountIds := make([]int, 0)
	sourceAccTransactions := make([]models.AccountTransaction, 0)
	destinationAccTransactions := make([]models.AccountTransaction, 0)

	transferOutStockHistories := make([]*models.StockHistory, 0)
	transferInStockHistories := make([]*models.StockHistory, 0)
	sourceProductInventoryAccounts := make(map[int]decimal.Decimal)
	destinationProductInventoryAccounts := make(map[int]decimal.Decimal)
	stockDate, err := utils.ConvertToDate(transferOrder.TransferDate, business.Timezone)
	if err != nil {
		return 0, nil, 0, err
	}

	for _, transferOrderDetail := range transferOrder.Details {
		if transferOrderDetail.ProductId > 0 {
			productDetail, err := GetProductDetail(tx, transferOrderDetail.ProductId, transferOrderDetail.ProductType)
			if err != nil {
				config.LogError(logger, "TransferOrderWorkflow.go", "CreateTransferOrder", "GetProductDetail", transferOrderDetail, err)
				return 0, nil, 0, err
			}

			sourceProductInventoryAmount, ok := sourceProductInventoryAccounts[productDetail.InventoryAccountId]
			if !ok {
				sourceProductInventoryAmount = decimal.NewFromInt(0)
			}
			sourceProductInventoryAccounts[productDetail.InventoryAccountId] = sourceProductInventoryAmount

			destinationProductInventoryAmount, ok := destinationProductInventoryAccounts[productDetail.InventoryAccountId]
			if !ok {
				destinationProductInventoryAmount = decimal.NewFromInt(0)
			}
			destinationProductInventoryAccounts[productDetail.InventoryAccountId] = destinationProductInventoryAmount

			sourceStockHistory := models.StockHistory{
				BusinessId:        transferOrder.BusinessId,
				WarehouseId:       sourceWarehouse.ID,
				ProductId:         transferOrderDetail.ProductId,
				ProductType:       transferOrderDetail.ProductType,
				BatchNumber:       transferOrderDetail.BatchNumber,
				StockDate:         stockDate,
				Qty:               transferOrderDetail.TransferQty.Neg(),
				BaseUnitValue:     decimal.NewFromInt(0),
				Description:       "Transfer Out",
				ReferenceType:     models.StockReferenceTypeTransferOrder,
				ReferenceID:       transferOrder.ID,
				ReferenceDetailID: transferOrderDetail.ID,
				IsOutgoing:        utils.NewTrue(),
				IsTransferIn:      utils.NewFalse(),
			}
			err = tx.Create(&sourceStockHistory).Error
			if err != nil {
				tx.Rollback()
				config.LogError(logger, "TransferOrderWorkflow.go", "CreateTransferOrder", "CreateSourceStockHistory", sourceStockHistory, err)
				return 0, nil, 0, err
			}
			transferOutStockHistories = append(transferOutStockHistories, &sourceStockHistory)
		}
	}

	accountIds = append(accountIds, systemAccounts[models.AccountCodeGoodsInTransfer])
	sourceAccTransactions = append(sourceAccTransactions, models.AccountTransaction{
		BusinessId:           businessId,
		AccountId:            systemAccounts[models.AccountCodeGoodsInTransfer],
		BranchId:             sourceBranchId,
		TransactionDateTime:  transactionTime,
		BaseCurrencyId:       baseCurrencyId,
		BaseCredit:           decimal.NewFromInt(0),
		BaseDebit:            decimal.NewFromInt(0),
		IsInventoryValuation: utils.NewTrue(),
		IsTransferIn:         utils.NewFalse(),
	})

	for inventoryAccId, inventoryAmount := range sourceProductInventoryAccounts {
		if !slices.Contains(accountIds, inventoryAccId) {
			accountIds = append(accountIds, inventoryAccId)
		}
		sourceAccTransactions = append(sourceAccTransactions, models.AccountTransaction{
			BusinessId:           businessId,
			AccountId:            inventoryAccId,
			BranchId:             sourceBranchId,
			TransactionDateTime:  transactionTime,
			BaseCurrencyId:       baseCurrencyId,
			BaseDebit:            inventoryAmount,
			BaseCredit:           decimal.NewFromInt(0),
			IsInventoryValuation: utils.NewTrue(),
			IsTransferIn:         utils.NewFalse(),
		})
	}

	accJournal := models.AccountJournal{
		BusinessId:          businessId,
		BranchId:            sourceBranchId,
		TransactionDateTime: transactionTime,
		TransactionNumber:   strconv.Itoa(transferOrder.ID),
		ReferenceId:         transferOrder.ID,
		ReferenceType:       models.AccountReferenceTypeTransferOrder,
		AccountTransactions: sourceAccTransactions,
	}

	err = tx.Create(&accJournal).Error
	if err != nil {
		tx.Rollback()
		config.LogError(logger, "TransferOrderWorkflow.go", "CreateTransferOrder", "CreateSourceAccountJournal", accJournal, err)
		return 0, nil, 0, err
	}

	// IMPORTANT:
	// Transfer Orders have TWO journals (source transfer-out + destination transfer-in).
	// Outgoing stock valuation (ProcessOutgoingStocks -> CalculateCogs) may trigger a journal repost
	// (reverse+replace) for BOTH sides to keep "Goods In Transfer" balanced.
	//
	// If the destination journal doesn't exist yet, reposting the transfer-in side will fail with:
	//   "repost journal: matching active journal not found"
	//
	// To avoid that, create a placeholder destination journal now (with IsTransferIn=true lines).
	// The valuation repost will update both journals once the true COGS is known.
	for _, srcTx := range sourceAccTransactions {
		dt := srcTx
		dt.ID = 0
		dt.BranchId = destinationBranchId
		dt.IsTransferIn = utils.NewTrue()
		// Mirror debit/credit direction (even if 0 at this stage).
		if dt.AccountId == systemAccounts[models.AccountCodeGoodsInTransfer] {
			dt.BaseCredit = dt.BaseDebit
			dt.BaseDebit = decimal.NewFromInt(0)
		} else {
			dt.BaseDebit = dt.BaseCredit
			dt.BaseCredit = decimal.NewFromInt(0)
		}
		destinationAccTransactions = append(destinationAccTransactions, dt)
	}

	destinationAccJournal := models.AccountJournal{
		BusinessId:          businessId,
		BranchId:            destinationBranchId,
		TransactionDateTime: transactionTime,
		TransactionNumber:   strconv.Itoa(transferOrder.ID),
		ReferenceId:         transferOrder.ID,
		ReferenceType:       models.AccountReferenceTypeTransferOrder,
		AccountTransactions: destinationAccTransactions,
	}
	if err := tx.Create(&destinationAccJournal).Error; err != nil {
		tx.Rollback()
		config.LogError(logger, "TransferOrderWorkflow.go", "CreateTransferOrder", "CreateDestinationAccountJournal", destinationAccJournal, err)
		return 0, nil, 0, err
	}

	valuationAccountIds, err := ProcessOutgoingStocks(tx, logger, transferOutStockHistories)
	if err != nil {
		// Self-heal: if FIFO layers are inconsistent due to backdated postings, rebuild and retry once.
		if scope, ok := parseFifoInsufficientScope(err); ok {
			if rerr := rebuildInventoryForScope(tx, logger, businessId, scope, transferOrder.TransferDate); rerr == nil {
				valuationAccountIds, err = ProcessOutgoingStocks(tx, logger, transferOutStockHistories)
			}
		}
		if err != nil {
		config.LogError(logger, "TransferOrderWorkflow.go", "CreateTransferOrder", "ProcessOutgoingStocks", transferOutStockHistories, err)
		return 0, nil, 0, err
		}
	}

	if len(valuationAccountIds) > 0 {
		for _, accId := range valuationAccountIds {
			if !slices.Contains(accountIds, accId) {
				accountIds = append(accountIds, accId)
			}
		}
	}

	var updatedTransferOutStockHistories []*models.StockHistory
	err = tx.Where("business_id = ? AND reference_id = ? AND reference_type = ?", businessId, transferOrder.ID, models.StockReferenceTypeTransferOrder).Find(&updatedTransferOutStockHistories).Error
	if err != nil {
		config.LogError(logger, "TransferOrderWorkflow.go", "CreateTransferOrder", "GetUpdatedTransferOutStockHistories", transferOrder.ID, err)
		return 0, nil, 0, err
	}

	for _, updatedOutStock := range updatedTransferOutStockHistories {
		// Clone each outgoing valuation row into an incoming row for destination.
		//
		// IMPORTANT:
		// - Outgoing processing can split a transfer line into multiple rows (FIFO layers).
		// - Destination MUST receive ALL those rows (qty and unit cost) to keep valuation consistent.
		// - Do NOT mutate the original outgoing rows in-place.
		if updatedOutStock == nil {
			continue
		}
		if updatedOutStock.IsOutgoing == nil || !*updatedOutStock.IsOutgoing {
			// Defensive: only clone outgoing rows.
			continue
		}
		if updatedOutStock.IsReversal || updatedOutStock.ReversedByStockHistoryId != nil {
			// Defensive: never clone reversal rows.
			continue
		}
		if updatedOutStock.WarehouseId != sourceWarehouse.ID {
			// Defensive: only clone from source warehouse postings.
			continue
		}

		in := *updatedOutStock
		in.ID = 0
		in.WarehouseId = destinationWarehouse.ID
		in.Qty = in.Qty.Abs()
		in.Description = "Transfer In"
		in.IsOutgoing = utils.NewFalse()
		in.IsTransferIn = utils.NewTrue()
		in.ClosingQty = decimal.NewFromInt(0)
		in.ClosingAssetValue = decimal.NewFromInt(0)
		in.CumulativeIncomingQty = decimal.NewFromInt(0)
		in.CumulativeOutgoingQty = decimal.NewFromInt(0)
		in.CumulativeSequence = 0
		// Ensure transfer-in row isn't treated as a reversal.
		in.IsReversal = false
		in.ReversesStockHistoryId = nil
		in.ReversedByStockHistoryId = nil
		in.ReversalReason = nil
		in.ReversedAt = nil

		err = tx.Create(&in).Error
		if err != nil {
			tx.Rollback()
			config.LogError(logger, "TransferOrderWorkflow.go", "CreateTransferOrder", "CreateDestinationStockHistory", in, err)
			return 0, nil, 0, err
		}
		transferInStockHistories = append(transferInStockHistories, &in)
	}

	valuationAccountIds, err = ProcessIncomingStocks(tx, logger, transferInStockHistories)
	if err != nil {
		config.LogError(logger, "TransferOrderWorkflow.go", "CreateTransferOrder", "ProcessIncomingStocks", transferInStockHistories, err)
		return 0, nil, 0, err
	}

	if len(valuationAccountIds) > 0 {
		for _, accId := range valuationAccountIds {
			if !slices.Contains(accountIds, accId) {
				accountIds = append(accountIds, accId)
			}
		}
	}

	return accJournal.ID, accountIds, foreignCurrencyId, nil

}
