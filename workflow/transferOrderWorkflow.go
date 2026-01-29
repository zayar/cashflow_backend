package workflow

import (
	"context"
	"encoding/json"
	"slices"
	"strconv"
	"time"

	"github.com/mmdatafocus/books_backend/config"
	"github.com/mmdatafocus/books_backend/models"
	"github.com/mmdatafocus/books_backend/utils"
	"github.com/shopspring/decimal"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

func ProcessTransferOrderWorkflow(tx *gorm.DB, logger *logrus.Logger, msg config.PubSubMessage) error {

	var accountJournalId int
	var accountIds []int
	var foreignCurrencyId int
	var stockHistories []*models.StockHistory
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
	} else if msg.Action == string(models.PubSubMessageActionDelete) {
		var oldTransferOrder models.TransferOrder
		err := json.Unmarshal([]byte(msg.OldObj), &oldTransferOrder)
		if err != nil {
			config.LogError(logger, "TransferOrderWorkflow.go", "ProcessTransferOrderWorkflow > Delete", "Unmarshal msg.OldObj", msg.OldObj, err)
			return err
		}

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		ctx = context.WithValue(ctx, utils.ContextKeyBusinessId, oldTransferOrder.BusinessId)
		sourceWarehouse, err := models.GetWarehouse(ctx, oldTransferOrder.SourceWarehouseId)
		if err != nil {
			config.LogError(logger, "TransferOrderWorkflow.go", "ProcessTransferOrderWorkflow > Delete", "GetSourceWarehouse", oldTransferOrder.SourceWarehouseId, err)
			return err
		}
		destinationWarehouse, err := models.GetWarehouse(ctx, oldTransferOrder.DestinationWarehouseId)
		if err != nil {
			config.LogError(logger, "TransferOrderWorkflow.go", "ProcessTransferOrderWorkflow > Delete", "GetDestinationWarehouse", oldTransferOrder.DestinationWarehouseId, err)
			return err
		}

		accountJournalId, accountIds, foreignCurrencyId, stockHistories, err = DeleteTransferOrder(tx, logger, msg.ID, msg.ReferenceType, msg.BusinessId, *business, oldTransferOrder)
		if err != nil {
			config.LogError(logger, "TransferOrderWorkflow.go", "ProcessTransferOrderWorkflow > Delete", "DeleteTransferOrder", nil, err)
			return err
		}
		valuationAccountIds, err := ProcessStockHistories(tx, logger, stockHistories)
		if err != nil {
			if scope, ok := parseFifoInsufficientScope(err); ok {
				if rerr := rebuildInventoryForScope(tx, logger, msg.BusinessId, scope, oldTransferOrder.TransferDate); rerr == nil {
					valuationAccountIds, err = ProcessStockHistories(tx, logger, stockHistories)
				}
			}
		}
		if err != nil {
			config.LogError(logger, "TransferOrderWorkflow.go", "ProcessTransferOrderWorkflow > Delete", "ProcessStockHistories", stockHistories, err)
			return err
		}
		if len(valuationAccountIds) > 0 {
			for _, accId := range valuationAccountIds {
				if !slices.Contains(accountIds, accId) {
					accountIds = append(accountIds, accId)
				}
			}
		}
		err = UpdateBalances(tx, logger, msg.BusinessId, business.BaseCurrencyId, sourceWarehouse.BranchId, accountIds, oldTransferOrder.TransferDate, foreignCurrencyId)
		if err != nil {
			config.LogError(logger, "TransferOrderWorkflow.go", "ProcessTransferOrderWorkflow > Delete", "UpdateBalances", nil, err)
			return err
		}
		if sourceWarehouse.BranchId != destinationWarehouse.BranchId {
			err = UpdateBalances(tx, logger, msg.BusinessId, business.BaseCurrencyId, destinationWarehouse.BranchId, accountIds, oldTransferOrder.TransferDate, foreignCurrencyId)
			if err != nil {
				config.LogError(logger, "TransferOrderWorkflow.go", "ProcessTransferOrderWorkflow > Delete", "UpdateBalancesOfDestinationBranch", nil, err)
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

	// Build existing incoming counts to make transfer-in creation idempotent.
	// This prevents duplicate transfer-in rows when rebuild/valuation syncs run
	// before this workflow creates incoming rows.
	type inKey struct {
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
	existingCounts := make(map[inKey]int)
	var existingInRows []*models.StockHistory
	if err := tx.
		Where("business_id = ? AND reference_type = ? AND reference_id = ? AND is_reversal = 0 AND reversed_by_stock_history_id IS NULL AND is_transfer_in = 1",
			businessId, models.StockReferenceTypeTransferOrder, transferOrder.ID).
		Find(&existingInRows).Error; err != nil {
		config.LogError(logger, "TransferOrderWorkflow.go", "CreateTransferOrder", "GetExistingTransferInRows", transferOrder.ID, err)
		return 0, nil, 0, err
	}
	for _, r := range existingInRows {
		if r == nil {
			continue
		}
		key := inKey{
			wh:     r.WarehouseId,
			pid:    r.ProductId,
			pt:     r.ProductType,
			b:      r.BatchNumber,
			rdi:    r.ReferenceDetailID,
			q:      r.Qty.String(),
			uv:     r.BaseUnitValue.String(),
			date:   r.StockDate.UTC().Format("2006-01-02"),
			refTyp: r.ReferenceType,
			refId:  r.ReferenceID,
		}
		existingCounts[key]++
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

		key := inKey{
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
		if c := existingCounts[key]; c > 0 {
			existingCounts[key] = c - 1
			continue
		}

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

// DeleteTransferOrder reverses ledger + journals for a transfer order.
// It appends reversal stock_histories rows and creates reversal account journals.
func DeleteTransferOrder(tx *gorm.DB, logger *logrus.Logger, recordId int, recordType string, businessId string, business models.Business, oldTransferOrder models.TransferOrder) (int, []int, int, []*models.StockHistory, error) {
	foreignCurrencyId := business.BaseCurrencyId

	// Reverse ALL active journals for this transfer order (source + destination).
	var journals []models.AccountJournal
	if err := tx.Preload("AccountTransactions").
		Where("reference_id = ? AND reference_type = ? AND is_reversal = 0 AND reversed_by_journal_id IS NULL", oldTransferOrder.ID, models.AccountReferenceTypeTransferOrder).
		Find(&journals).Error; err != nil {
		config.LogError(logger, "TransferOrderWorkflow.go", "DeleteTransferOrder", "FindAccountJournals", oldTransferOrder, err)
		return 0, nil, 0, nil, err
	}
	accountIds := make([]int, 0)
	reversalID := 0
	for _, j := range journals {
		if j.ID == 0 {
			continue
		}
		for _, t := range j.AccountTransactions {
			if !slices.Contains(accountIds, t.AccountId) {
				accountIds = append(accountIds, t.AccountId)
			}
		}
		rid, err := ReverseAccountJournal(tx, &j, ReversalReasonTransferOrderVoidUpdate)
		if err != nil {
			config.LogError(logger, "TransferOrderWorkflow.go", "DeleteTransferOrder", "ReverseAccountJournal", j, err)
			return 0, nil, 0, nil, err
		}
		if reversalID == 0 {
			reversalID = rid
		}
	}

	// Reverse stock histories (source transfer-out + destination transfer-in).
	var stockHistories []*models.StockHistory
	if err := tx.
		Where("reference_id = ? AND reference_type = ? AND is_reversal = 0 AND reversed_by_stock_history_id IS NULL", oldTransferOrder.ID, models.StockReferenceTypeTransferOrder).
		Find(&stockHistories).Error; err != nil {
		config.LogError(logger, "TransferOrderWorkflow.go", "DeleteTransferOrder", "FindStockHistories", oldTransferOrder, err)
		return 0, nil, 0, nil, err
	}
	stockReversals, err := ReverseStockHistories(tx, stockHistories, ReversalReasonTransferOrderVoidUpdate)
	if err != nil {
		config.LogError(logger, "TransferOrderWorkflow.go", "DeleteTransferOrder", "ReverseStockHistories", oldTransferOrder, err)
		return 0, nil, 0, nil, err
	}

	return reversalID, accountIds, foreignCurrencyId, stockReversals, nil
}
