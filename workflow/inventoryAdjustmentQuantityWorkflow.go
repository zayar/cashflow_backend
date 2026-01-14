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
