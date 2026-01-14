package workflow

import (
	"encoding/json"
	"slices"
	"strconv"

	"github.com/mmdatafocus/books_backend/config"
	"github.com/mmdatafocus/books_backend/models"
	"github.com/mmdatafocus/books_backend/utils"
	"github.com/shopspring/decimal"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

// applyInventoryAdjustmentQuantityToStockSummary updates stock_summaries + stock_summary_daily_balances
// from the stock ledger rows created by the inventory adjustment workflow.
//
// Why this exists:
// - Inventory valuation is driven by stock_histories (ledger).
// - Product Overview / Transfer Order availability / Warehouse Report are driven by stock_summaries
//   and stock_summary_daily_balances (cache/derived tables).
// - When INVENTORY_ADJUSTMENT stock commands are disabled, creating an adjustment as "Adjusted"
//   can post to the ledger (async) without updating the cache, causing UI mismatch.
//
// IMPORTANT: Avoid double-applying when INVENTORY_ADJUSTMENT stock commands are enabled
// (those update stock_summaries at status transition time).
func applyInventoryAdjustmentQuantityToStockSummary(tx *gorm.DB, businessId string, stockHistories []*models.StockHistory) error {
	if tx == nil {
		return gorm.ErrInvalidDB
	}
	if len(stockHistories) == 0 {
		return nil
	}
	// If stock commands are enabled for inventory adjustments, cache updates are already applied
	// during the Draft -> Adjusted transition in the API layer.
	if config.UseStockCommandsFor("INVENTORY_ADJUSTMENT") {
		return nil
	}

	ctx := tx.Statement.Context
	if err := utils.BusinessLock(ctx, businessId, "stockLock", "inventoryAdjustmentQuantityWorkflow.go", "applyInventoryAdjustmentQuantityToStockSummary"); err != nil {
		tx.Rollback()
		return err
	}

	for _, sh := range stockHistories {
		if sh == nil {
			continue
		}
		if sh.ProductId <= 0 {
			continue
		}
		if sh.ReferenceType != models.StockReferenceTypeInventoryAdjustmentQuantity {
			// Defensive: this helper is only intended for IVAQ rows.
			continue
		}

		// NOTE: worker tx context does not always carry business_id, so do NOT use
		// context-scoped product fetches here. Use the workflow helper instead.
		productDetail, err := GetProductDetail(tx, sh.ProductId, sh.ProductType)
		if err != nil {
			tx.Rollback()
			return err
		}
		if productDetail.InventoryAccountId <= 0 {
			continue
		}

		isOutgoing := false
		if sh.IsOutgoing != nil {
			isOutgoing = *sh.IsOutgoing
		}

		// For reversals, we want to reverse the same "bucket" the original affected:
		// - original incoming adjustment -> decrement adjusted_qty_in (pass negative qty to AdjustedQtyIn)
		// - original outgoing adjustment -> decrement adjusted_qty_out (pass positive qty to AdjustedQtyOut)
		applyToOutBucket := isOutgoing
		if sh.IsReversal {
			originalWasOutgoing := !isOutgoing
			applyToOutBucket = originalWasOutgoing
		}

		if applyToOutBucket {
			if err := models.UpdateStockSummaryAdjustedQtyOut(tx, sh.BusinessId, sh.WarehouseId, sh.ProductId, string(sh.ProductType), sh.BatchNumber, sh.Qty, sh.StockDate); err != nil {
				tx.Rollback()
				return err
			}
		} else {
			if err := models.UpdateStockSummaryAdjustedQtyIn(tx, sh.BusinessId, sh.WarehouseId, sh.ProductId, string(sh.ProductType), sh.BatchNumber, sh.Qty, sh.StockDate); err != nil {
				tx.Rollback()
				return err
			}
		}
	}
	return nil
}

func ProcessInventoryAdjustmentQuantityWorkflow(tx *gorm.DB, logger *logrus.Logger, msg config.PubSubMessage) error {

	var accountJournalId int
	var accountIds []int
	var foreignCurrencyId int
	var increasedStockHistories []*models.StockHistory
	var decreasedStockHistories []*models.StockHistory
	business, err := models.GetBusinessById2(tx, msg.BusinessId)
	if err != nil {
		config.LogError(logger, "InventoryAdjustmentQuantityWorkflow.go", "ProcessInventoryAdjustmentQuantityWorkflow", "GetBusiness", msg.BusinessId, err)
		return err
	}
	if msg.Action == string(models.PubSubMessageActionCreate) {

		var inventoryAdjustment models.InventoryAdjustment
		err := json.Unmarshal([]byte(msg.NewObj), &inventoryAdjustment)
		if err != nil {
			config.LogError(logger, "InventoryAdjustmentQuantityWorkflow.go", "ProcessInventoryAdjustmentQuantityWorkflow > Create", "Unmarshal msg.NewObj", msg.NewObj, err)
			return err
		}
		accountJournalId, accountIds, foreignCurrencyId, increasedStockHistories, decreasedStockHistories, err = CreateInventoryAdjustmentQuantity(tx, logger, msg.ID, msg.ReferenceType, msg.BusinessId, *business, inventoryAdjustment)
		if err != nil {
			config.LogError(logger, "InventoryAdjustmentQuantityWorkflow.go", "ProcessInventoryAdjustmentQuantityWorkflow > Create", "CreateCreditNote", nil, err)
			return err
		}
		merged := append(increasedStockHistories, decreasedStockHistories...)
		// Keep cache tables in sync with ledger for UI stock availability queries.
		if err := applyInventoryAdjustmentQuantityToStockSummary(tx, msg.BusinessId, merged); err != nil {
			config.LogError(logger, "InventoryAdjustmentQuantityWorkflow.go", "ProcessInventoryAdjustmentQuantityWorkflow > Create", "applyInventoryAdjustmentQuantityToStockSummary", merged, err)
			return err
		}
		valuationAccountIds, err := ProcessStockHistories(tx, logger, merged)
		if err != nil {
			config.LogError(logger, "InventoryAdjustmentQuantityWorkflow.go", "ProcessInventoryAdjustmentQuantityWorkflow > Create", "ProcessStockHistories", merged, err)
			return err
		}
		accountIds = utils.MergeIntSlices(accountIds, valuationAccountIds)
		err = UpdateBalances(tx, logger, msg.BusinessId, business.BaseCurrencyId, inventoryAdjustment.BranchId, accountIds, inventoryAdjustment.AdjustmentDate, foreignCurrencyId)
		if err != nil {
			config.LogError(logger, "InventoryAdjustmentQuantityWorkflow.go", "ProcessInventoryAdjustmentQuantityWorkflow > Create", "UpdateBalances", nil, err)
			return err
		}
	} else if msg.Action == string(models.PubSubMessageActionDelete) {
		var oldInventoryAdjustment models.InventoryAdjustment
		err = json.Unmarshal([]byte(msg.OldObj), &oldInventoryAdjustment)
		if err != nil {
			config.LogError(logger, "InventoryAdjustmentQuantityWorkflow.go", "ProcessInventoryAdjustmentQuantityWorkflow > Delete", "Unmarshal OldObj", msg.OldObj, err)
			return err
		}
		accountJournalId, accountIds, foreignCurrencyId, increasedStockHistories, decreasedStockHistories, err = DeleteInventoryAdjustmentQuantity(tx, logger, msg.ID, msg.ReferenceType, msg.BusinessId, *business, oldInventoryAdjustment)
		if err != nil {
			config.LogError(logger, "InventoryAdjustmentQuantityWorkflow.go", "ProcessInventoryAdjustmentQuantityWorkflow > Delete", "DeleteBill", nil, err)
			return err
		}
		merged := append(increasedStockHistories, decreasedStockHistories...)
		// Apply reversal effects to cache tables as well, so availability reverts.
		if err := applyInventoryAdjustmentQuantityToStockSummary(tx, msg.BusinessId, merged); err != nil {
			config.LogError(logger, "InventoryAdjustmentQuantityWorkflow.go", "ProcessInventoryAdjustmentQuantityWorkflow > Delete", "applyInventoryAdjustmentQuantityToStockSummary", merged, err)
			return err
		}
		valuationAccountIds, err := ProcessStockHistories(tx, logger, merged)
		if err != nil {
			config.LogError(logger, "InventoryAdjustmentQuantityWorkflow.go", "ProcessInventoryAdjustmentQuantityWorkflow > Delete", "ProcessStockHistories", merged, err)
			return err
		}
		accountIds = utils.MergeIntSlices(accountIds, valuationAccountIds)
		err = UpdateBalances(tx, logger, msg.BusinessId, business.BaseCurrencyId, oldInventoryAdjustment.BranchId, accountIds, oldInventoryAdjustment.AdjustmentDate, foreignCurrencyId)
		if err != nil {
			config.LogError(logger, "InventoryAdjustmentQuantityWorkflow.go", "ProcessInventoryAdjustmentQuantityWorkflow > Delete", "UpdateBalances", oldInventoryAdjustment, err)
			return err
		}
	}
	err = tx.Model(&models.PubSubMessageRecord{}).Where("id=?", msg.ID).Updates(map[string]interface{}{"account_journal_id": accountJournalId, "is_processed": true}).Error
	if err != nil {
		config.LogError(logger, "InventoryAdjustmentQuantityWorkflow.go", "ProcessInventoryAdjustmentQuantityWorkflow", "UpdatePubSubMessageRecord", accountJournalId, err)
		return err
	}
	return nil
}

func CreateInventoryAdjustmentQuantity(tx *gorm.DB, logger *logrus.Logger, recordId int, recordType string, businessId string, business models.Business, inventoryAdjustment models.InventoryAdjustment) (int, []int, int, []*models.StockHistory, []*models.StockHistory, error) {

	transactionTime := inventoryAdjustment.AdjustmentDate
	baseCurrencyId := business.BaseCurrencyId
	foreignCurrencyId := business.BaseCurrencyId
	branchId := inventoryAdjustment.BranchId

	accountIds := make([]int, 0)
	accTransactions := make([]models.AccountTransaction, 0)

	increasedStockHistories := make([]*models.StockHistory, 0)
	decreasedStockHistories := make([]*models.StockHistory, 0)
	productPurchaseAccounts := make(map[int]decimal.Decimal)
	productInventoryAccounts := make(map[int]decimal.Decimal)
	stockDate, err := utils.ConvertToDate(inventoryAdjustment.AdjustmentDate, business.Timezone)
	if err != nil {
		return 0, nil, 0, nil, nil, err
	}

	for _, inventoryAdjustmentDetail := range inventoryAdjustment.Details {
		if inventoryAdjustmentDetail.ProductId > 0 {
			productDetail, err := GetProductDetail(tx, inventoryAdjustmentDetail.ProductId, inventoryAdjustmentDetail.ProductType)
			if err != nil {
				config.LogError(logger, "InventoryAdjustmentQuantityWorkflow.go", "CreateInventoryAdjustmentQuantity", "GetProductDetail", inventoryAdjustmentDetail, err)
				return 0, nil, 0, nil, nil, err
			}

			productPurchaseAmount, ok := productPurchaseAccounts[inventoryAdjustment.AccountId]
			if !ok {
				productPurchaseAmount = decimal.NewFromInt(0)
			}
			productPurchaseAccounts[inventoryAdjustment.AccountId] = productPurchaseAmount.Add(inventoryAdjustmentDetail.AdjustedValue.Mul(inventoryAdjustmentDetail.CostPrice))

			productInventoryAmount, ok := productInventoryAccounts[productDetail.InventoryAccountId]
			if !ok {
				productInventoryAmount = decimal.NewFromInt(0)
			}
			productInventoryAccounts[productDetail.InventoryAccountId] = productInventoryAmount.Add(inventoryAdjustmentDetail.AdjustedValue.Mul(inventoryAdjustmentDetail.CostPrice))

			stockHistory := models.StockHistory{
				BusinessId:        inventoryAdjustment.BusinessId,
				WarehouseId:       inventoryAdjustment.WarehouseId,
				ProductId:         inventoryAdjustmentDetail.ProductId,
				ProductType:       inventoryAdjustmentDetail.ProductType,
				BatchNumber:       inventoryAdjustmentDetail.BatchNumber,
				StockDate:         stockDate,
				Qty:               inventoryAdjustmentDetail.AdjustedValue,
				BaseUnitValue:     inventoryAdjustmentDetail.CostPrice,
				Description:       "InventoryAdjustment By Quantity",
				ReferenceType:     models.StockReferenceTypeInventoryAdjustmentQuantity,
				ReferenceID:       inventoryAdjustment.ID,
				ReferenceDetailID: inventoryAdjustmentDetail.ID,
			}
			if inventoryAdjustmentDetail.AdjustedValue.IsNegative() {
				stockHistory.IsOutgoing = utils.NewTrue()
			} else {
				stockHistory.IsOutgoing = utils.NewFalse()
			}
			err = tx.Create(&stockHistory).Error
			if err != nil {
				tx.Rollback()
				config.LogError(logger, "InventoryAdjustmentQuantityWorkflow.go", "CreateInventoryAdjustmentQuantity", "CreateStockHistory", stockHistory, err)
				return 0, nil, 0, nil, nil, err
			}
			if inventoryAdjustmentDetail.AdjustedValue.IsNegative() {
				decreasedStockHistories = append(decreasedStockHistories, &stockHistory)
			} else {
				increasedStockHistories = append(increasedStockHistories, &stockHistory)
			}
		}
	}

	for purchaseAccId, purchaseAmount := range productPurchaseAccounts {
		if !slices.Contains(accountIds, purchaseAccId) {
			accountIds = append(accountIds, purchaseAccId)
		}
		if purchaseAmount.IsNegative() {
			accTransactions = append(accTransactions, models.AccountTransaction{
				BusinessId:           businessId,
				AccountId:            purchaseAccId,
				BranchId:             branchId,
				TransactionDateTime:  transactionTime,
				BaseCurrencyId:       baseCurrencyId,
				BaseDebit:            purchaseAmount.Abs(),
				BaseCredit:           decimal.NewFromInt(0),
				IsInventoryValuation: utils.NewTrue(),
			})
		} else {
			accTransactions = append(accTransactions, models.AccountTransaction{
				BusinessId:           businessId,
				AccountId:            purchaseAccId,
				BranchId:             branchId,
				TransactionDateTime:  transactionTime,
				BaseCurrencyId:       baseCurrencyId,
				BaseCredit:           purchaseAmount,
				BaseDebit:            decimal.NewFromInt(0),
				IsInventoryValuation: utils.NewTrue(),
			})
		}
	}

	for inventoryAccId, inventoryAmount := range productInventoryAccounts {
		if !slices.Contains(accountIds, inventoryAccId) {
			accountIds = append(accountIds, inventoryAccId)
		}
		if inventoryAmount.IsNegative() {
			accTransactions = append(accTransactions, models.AccountTransaction{
				BusinessId:           businessId,
				AccountId:            inventoryAccId,
				BranchId:             branchId,
				TransactionDateTime:  transactionTime,
				BaseCurrencyId:       baseCurrencyId,
				BaseCredit:           inventoryAmount.Abs(),
				BaseDebit:            decimal.NewFromInt(0),
				IsInventoryValuation: utils.NewTrue(),
			})
		} else {
			accTransactions = append(accTransactions, models.AccountTransaction{
				BusinessId:           businessId,
				AccountId:            inventoryAccId,
				BranchId:             branchId,
				TransactionDateTime:  transactionTime,
				BaseCurrencyId:       baseCurrencyId,
				BaseDebit:            inventoryAmount,
				BaseCredit:           decimal.NewFromInt(0),
				IsInventoryValuation: utils.NewTrue(),
			})
		}
	}

	accJournal := models.AccountJournal{
		BusinessId:          businessId,
		BranchId:            branchId,
		TransactionDateTime: transactionTime,
		TransactionNumber:   strconv.Itoa(inventoryAdjustment.ID),
		ReferenceId:         inventoryAdjustment.ID,
		ReferenceType:       models.AccountReferenceTypeInventoryAdjustmentQuantity,
		AccountTransactions: accTransactions,
	}

	err = tx.Create(&accJournal).Error
	if err != nil {
		tx.Rollback()
		config.LogError(logger, "InventoryAdjustmentQuantityWorkflow.go", "CreateInventoryAdjustmentQuantity", "CreateAccountJournal", accJournal, err)
		return 0, nil, 0, nil, nil, err
	}

	return accJournal.ID, accountIds, foreignCurrencyId, increasedStockHistories, decreasedStockHistories, nil

}

func DeleteInventoryAdjustmentQuantity(tx *gorm.DB, logger *logrus.Logger, recordId int, recordType string, businessId string, business models.Business, oldInventoryAdjustment models.InventoryAdjustment) (int, []int, int, []*models.StockHistory, []*models.StockHistory, error) {

	foreignCurrencyId := business.BaseCurrencyId
	accountJournal, _, accountIds, err := GetExistingAccountJournal(tx, oldInventoryAdjustment.ID, models.AccountReferenceTypeInventoryAdjustmentQuantity)
	if err != nil {
		config.LogError(logger, "InventoryAdjustmentQuantityWorkflow.go", "DeleteInventoryAdjustmentQuantity", "GetExistingAccountJournal", oldInventoryAdjustment, err)
		return 0, nil, 0, nil, nil, err
	}

	var stockHistories []*models.StockHistory
	err = tx.Where("reference_id = ? AND reference_type = ? AND is_reversal = 0 AND reversed_by_stock_history_id IS NULL", oldInventoryAdjustment.ID, models.StockReferenceTypeInventoryAdjustmentQuantity).Find(&stockHistories).Error
	if err != nil {
		config.LogError(logger, "InventoryAdjustmentQuantityWorkflow.go", "DeleteInventoryAdjustmentQuantity", "FindStockHistories", oldInventoryAdjustment, err)
		return 0, nil, 0, nil, nil, err
	}
	// Inventory ledger immutability: do not delete stock history; append reversal entries instead.
	stockReversals, err := ReverseStockHistories(tx, stockHistories, ReversalReasonInventoryAdjustQtyVoidUpdate)
	if err != nil {
		config.LogError(logger, "InventoryAdjustmentQuantityWorkflow.go", "DeleteInventoryAdjustmentQuantity", "ReverseStockHistories", oldInventoryAdjustment, err)
		return 0, nil, 0, nil, nil, err
	}

	// Phase 1: do not delete posted journals; create a reversal journal instead.
	reversalID, err := ReverseAccountJournal(tx, accountJournal, ReversalReasonInventoryAdjustQtyVoidUpdate)
	if err != nil {
		config.LogError(logger, "InventoryAdjustmentQuantityWorkflow.go", "DeleteInventoryAdjustmentQuantity", "ReverseAccountJournal", accountJournal, err)
		return 0, nil, 0, nil, nil, err
	}

	increasedStockHistories := make([]*models.StockHistory, 0)
	decreasedStockHistories := make([]*models.StockHistory, 0)
	for _, stockHistory := range stockReversals {
		if stockHistory.Qty.IsNegative() {
			decreasedStockHistories = append(decreasedStockHistories, stockHistory)
		} else {
			increasedStockHistories = append(increasedStockHistories, stockHistory)
		}
	}

	return reversalID, accountIds, foreignCurrencyId, increasedStockHistories, decreasedStockHistories, nil
}
