package workflow

import (
	"context"
	"encoding/json"
	"slices"
	"strconv"
	"time"

	"bitbucket.org/mmdatafocus/books_backend/config"
	"bitbucket.org/mmdatafocus/books_backend/models"
	"bitbucket.org/mmdatafocus/books_backend/utils"
	"github.com/shopspring/decimal"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

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

	valuationAccountIds, err := ProcessOutgoingStocks(tx, logger, transferOutStockHistories)
	if err != nil {
		config.LogError(logger, "TransferOrderWorkflow.go", "CreateTransferOrder", "ProcessOutgoingStocks", transferOutStockHistories, err)
		return 0, nil, 0, err
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
		updatedOutStock.ID = 0
		updatedOutStock.WarehouseId = destinationWarehouse.ID
		updatedOutStock.Qty = updatedOutStock.Qty.Abs()
		updatedOutStock.Description = "Transfer In"
		updatedOutStock.IsOutgoing = utils.NewFalse()
		updatedOutStock.IsTransferIn = utils.NewTrue()
		updatedOutStock.ClosingQty = decimal.NewFromInt(0)
		updatedOutStock.ClosingAssetValue = decimal.NewFromInt(0)
		updatedOutStock.CumulativeIncomingQty = decimal.NewFromInt(0)
		updatedOutStock.CumulativeOutgoingQty = decimal.NewFromInt(0)
		err = tx.Create(&updatedOutStock).Error
		if err != nil {
			tx.Rollback()
			config.LogError(logger, "TransferOrderWorkflow.go", "CreateTransferOrder", "CreateDestinationStockHistory", updatedOutStock, err)
			return 0, nil, 0, err
		}
		transferInStockHistories = append(transferInStockHistories, updatedOutStock)
	}

	var updatedAccountTransactions []*models.AccountTransaction
	err = tx.Where("business_id = ? AND journal_id = ? AND branch_id = ?", businessId, accJournal.ID, sourceBranchId).Find(&updatedAccountTransactions).Error
	if err != nil {
		config.LogError(logger, "TransferOrderWorkflow.go", "CreateTransferOrder", "GetUpdatedAccountTransactions", transferOrder.ID, err)
		return 0, nil, 0, err
	}

	for _, updatedAccTransact := range updatedAccountTransactions {
		updatedAccTransact.ID = 0
		updatedAccTransact.BranchId = destinationBranchId
		updatedAccTransact.IsTransferIn = utils.NewTrue()

		if updatedAccTransact.AccountId == systemAccounts[models.AccountCodeGoodsInTransfer] {
			updatedAccTransact.BaseCredit = updatedAccTransact.BaseDebit
			updatedAccTransact.BaseDebit = decimal.NewFromInt(0)
		} else {
			updatedAccTransact.BaseDebit = updatedAccTransact.BaseCredit
			updatedAccTransact.BaseCredit = decimal.NewFromInt(0)
		}
		destinationAccTransactions = append(destinationAccTransactions, *updatedAccTransact)
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

	err = tx.Create(&destinationAccJournal).Error
	if err != nil {
		tx.Rollback()
		config.LogError(logger, "TransferOrderWorkflow.go", "CreateTransferOrder", "CreateDestinationAccountJournal", accJournal, err)
		return 0, nil, 0, err
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
