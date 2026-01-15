package workflow

import (
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

func ProcessInventoryAdjustmentValueWorkflow(tx *gorm.DB, logger *logrus.Logger, msg config.PubSubMessage) error {

	var accountJournalId int
	var accountIds []int
	var foreignCurrencyId int
	var stockHistories []*models.StockHistory
	business, err := models.GetBusinessById2(tx, msg.BusinessId)
	if err != nil {
		config.LogError(logger, "InventoryAdjustmentValueWorkflow.go", "ProcessInventoryAdjustmentValueWorkflow", "GetBusiness", msg.BusinessId, err)
		return err
	}
	if msg.Action == string(models.PubSubMessageActionCreate) {

		var inventoryAdjustment models.InventoryAdjustment
		err := json.Unmarshal([]byte(msg.NewObj), &inventoryAdjustment)
		if err != nil {
			config.LogError(logger, "InventoryAdjustmentValueWorkflow.go", "ProcessInventoryAdjustmentValueWorkflow > Create", "Unmarshal msg.NewObj", msg.NewObj, err)
			return err
		}
		accountJournalId, accountIds, foreignCurrencyId, stockHistories, err = CreateInventoryAdjustmentValue(tx, logger, msg.ID, msg.ReferenceType, msg.BusinessId, *business, inventoryAdjustment)
		if err != nil {
			config.LogError(logger, "InventoryAdjustmentValueWorkflow.go", "ProcessInventoryAdjustmentValueWorkflow > Create", "CreateInventoryAdjustmentValue", nil, err)
			return err
		}
		valuationAccountIds, err := ProcessValueAdjustmentStocks(tx, logger, stockHistories)
		if err != nil {
			config.LogError(logger, "InventoryAdjustmentValueWorkflow.go", "ProcessInventoryAdjustmentValueWorkflow > Create", "ProcessValueAdjustmentStocks", stockHistories, err)
			return err
		}

		if len(valuationAccountIds) > 0 {
			for _, accId := range valuationAccountIds {
				if !slices.Contains(accountIds, accId) {
					accountIds = append(accountIds, accId)
				}
			}
		}
		err = UpdateBalances(tx, logger, msg.BusinessId, business.BaseCurrencyId, inventoryAdjustment.BranchId, accountIds, inventoryAdjustment.AdjustmentDate, foreignCurrencyId)
		if err != nil {
			config.LogError(logger, "InventoryAdjustmentValueWorkflow.go", "ProcessInventoryAdjustmentValueWorkflow > Create", "UpdateBalances", nil, err)
			return err
		}
	} else if msg.Action == string(models.PubSubMessageActionDelete) {
		var oldInventoryAdjustment models.InventoryAdjustment
		err := json.Unmarshal([]byte(msg.OldObj), &oldInventoryAdjustment)
		if err != nil {
			config.LogError(logger, "InventoryAdjustmentValueWorkflow.go", "ProcessInventoryAdjustmentValueWorkflow > Delete", "Unmarshal msg.OldObj", msg.OldObj, err)
			return err
		}
		accountJournalId, accountIds, foreignCurrencyId, stockHistories, err = DeleteInventoryAdjustmentValue(tx, logger, msg.ID, msg.ReferenceType, msg.BusinessId, *business, oldInventoryAdjustment)
		if err != nil {
			config.LogError(logger, "InventoryAdjustmentValueWorkflow.go", "ProcessInventoryAdjustmentValueWorkflow > Delete", "DeleteInventoryAdjustmentValue", nil, err)
			return err
		}
		// Inventory ledger immutability: delete is modeled as a reversal IVAV event.
		// Process the incoming reversal rows using the normal value-adjustment processor.
		valuationAccountIds, err := ProcessValueAdjustmentStocks(tx, logger, stockHistories)
		if err != nil {
			config.LogError(logger, "InventoryAdjustmentValueWorkflow.go", "ProcessInventoryAdjustmentValueWorkflow > Delete", "ProcessValueAdjustmentStocks", stockHistories, err)
			return err
		}

		if len(valuationAccountIds) > 0 {
			for _, accId := range valuationAccountIds {
				if !slices.Contains(accountIds, accId) {
					accountIds = append(accountIds, accId)
				}
			}
		}
		err = UpdateBalances(tx, logger, msg.BusinessId, business.BaseCurrencyId, oldInventoryAdjustment.BranchId, accountIds, oldInventoryAdjustment.AdjustmentDate, foreignCurrencyId)
		if err != nil {
			config.LogError(logger, "InventoryAdjustmentValueWorkflow.go", "ProcessInventoryAdjustmentValueWorkflow > Delete", "UpdateBalances", nil, err)
			return err
		}
	}
	err = tx.Model(&models.PubSubMessageRecord{}).Where("id=?", msg.ID).Updates(map[string]interface{}{"account_journal_id": accountJournalId, "is_processed": true}).Error
	if err != nil {
		config.LogError(logger, "InventoryAdjustmentValueWorkflow.go", "ProcessInventoryAdjustmentValueWorkflow", "UpdatePubSubMessageRecord", accountJournalId, err)
		return err
	}
	return nil
}

func CreateInventoryAdjustmentValue(tx *gorm.DB, logger *logrus.Logger, recordId int, recordType string, businessId string, business models.Business, inventoryAdjustment models.InventoryAdjustment) (int, []int, int, []*models.StockHistory, error) {

	transactionTime := inventoryAdjustment.AdjustmentDate
	baseCurrencyId := business.BaseCurrencyId
	foreignCurrencyId := business.BaseCurrencyId
	branchId := inventoryAdjustment.BranchId

	accountIds := make([]int, 0)
	accTransactions := make([]models.AccountTransaction, 0)

	productPurchaseAccounts := make(map[int]decimal.Decimal)
	productInventoryAccounts := make(map[int]decimal.Decimal)
	stockHistories := make([]*models.StockHistory, 0)
	stockDate, err := utils.ConvertToDate(inventoryAdjustment.AdjustmentDate, business.Timezone)
	if err != nil {
		return 0, nil, 0, nil, err
	}
	// ConvertToDate() returns start-of-day. stock_histories.stock_date is a timestamp, so use
	// an inclusive end-of-day for "until date" semantics and an exclusive next-day bound for "<".
	stockDateExclusiveEnd := stockDate.AddDate(0, 0, 1)
	stockDateInclusiveEnd := time.Date(stockDate.Year(), stockDate.Month(), stockDate.Day(), 23, 59, 59, 0, stockDate.Location())

	for _, inventoryAdjustmentDetail := range inventoryAdjustment.Details {
		if inventoryAdjustmentDetail.ProductId > 0 {
			productDetail, err := GetProductDetail(tx, inventoryAdjustmentDetail.ProductId, inventoryAdjustmentDetail.ProductType)
			if err != nil {
				config.LogError(logger, "InventoryAdjustmentValueWorkflow.go", "CreateInventoryAdjustmentValue", "GetProductDetail", inventoryAdjustmentDetail, err)
				return 0, nil, 0, nil, err
			}

			var lastStockHistory models.StockHistory
			// IMPORTANT: only consider ACTIVE ledger rows and always scope by business_id.
			// Otherwise, we can:
			// - pick reversal/inactive rows (closing balances may be 0)
			// - pick rows from another tenant in rare cases (warehouse IDs are not globally unique assumptions)
			err = tx.
				Where("business_id = ? AND warehouse_id = ? AND product_id = ? AND product_type = ? AND COALESCE(batch_number,'') = ? AND stock_date < ? AND is_reversal = 0 AND reversed_by_stock_history_id IS NULL",
					businessId, inventoryAdjustment.WarehouseId, inventoryAdjustmentDetail.ProductId, inventoryAdjustmentDetail.ProductType, inventoryAdjustmentDetail.BatchNumber, stockDateExclusiveEnd).
				Order("stock_date DESC, cumulative_sequence DESC, id DESC").
				Limit(1).
				Find(&lastStockHistory).Error
			if err != nil {
				config.LogError(logger, "InventoryAdjustmentValueWorkflow.go", "CreateInventoryAdjustmentValue", "GetLastStockHistory", inventoryAdjustmentDetail, err)
				return 0, nil, 0, nil, err
			}
			if lastStockHistory.ID <= 0 {
				config.LogError(logger, "InventoryAdjustmentValueWorkflow.go", "CreateInventoryAdjustmentValue", "LastStockHistory not found", inventoryAdjustmentDetail, err)
				return 0, nil, 0, nil, err
			}
			remainingIncomingStockHistories, err := GetRemainingStockHistoriesByCumulativeQtyUntilDate(tx, inventoryAdjustment.WarehouseId, inventoryAdjustmentDetail.ProductId, string(inventoryAdjustmentDetail.ProductType), inventoryAdjustmentDetail.BatchNumber, utils.NewFalse(), lastStockHistory.CumulativeOutgoingQty, stockDateInclusiveEnd)
			if err != nil {
				config.LogError(logger, "InventoryAdjustmentValueWorkflow.go", "CreateInventoryAdjustmentValue", "GetRemainingStockHistoriesByCumulativeQty", inventoryAdjustmentDetail, err)
				return 0, nil, 0, nil, err
			}
			totalQty := decimal.NewFromInt(0)
			totalValue := decimal.NewFromInt(0)
			for index, remainingIncomingStockHistory := range remainingIncomingStockHistories {
				stockHistory := models.StockHistory{
					BusinessId:        inventoryAdjustment.BusinessId,
					WarehouseId:       inventoryAdjustment.WarehouseId,
					ProductId:         inventoryAdjustmentDetail.ProductId,
					ProductType:       inventoryAdjustmentDetail.ProductType,
					BatchNumber:       inventoryAdjustmentDetail.BatchNumber,
					StockDate:         stockDate,
					Qty:               remainingIncomingStockHistory.Qty.Neg(),
					BaseUnitValue:     remainingIncomingStockHistory.BaseUnitValue,
					Description:       "InventoryAdjustment By Value",
					ReferenceType:     models.StockReferenceTypeInventoryAdjustmentValue,
					ReferenceID:       inventoryAdjustment.ID,
					ReferenceDetailID: inventoryAdjustmentDetail.ID,
					IsOutgoing:        utils.NewTrue(),
				}
				if index == 0 {
					stockHistory.Qty = remainingIncomingStockHistory.CumulativeIncomingQty.Sub(lastStockHistory.CumulativeOutgoingQty).Neg()
				}
				totalQty = totalQty.Add(stockHistory.Qty.Abs())
				totalValue = totalValue.Add(stockHistory.Qty.Abs().Mul(stockHistory.BaseUnitValue))
				err = tx.Create(&stockHistory).Error
				if err != nil {
					config.LogError(logger, "InventoryAdjustmentValueWorkflow.go", "CreateInventoryAdjustmentValue", "CreateOutgoingStockHistory", stockHistory, err)
					return 0, nil, 0, nil, err
				}
			}

			newBaseUnitValue := totalValue.Add(inventoryAdjustmentDetail.AdjustedValue).DivRound(totalQty, 4)
			stockHistory := models.StockHistory{
				BusinessId:        inventoryAdjustment.BusinessId,
				WarehouseId:       inventoryAdjustment.WarehouseId,
				ProductId:         inventoryAdjustmentDetail.ProductId,
				ProductType:       inventoryAdjustmentDetail.ProductType,
				BatchNumber:       inventoryAdjustmentDetail.BatchNumber,
				StockDate:         stockDate,
				Qty:               totalQty,
				BaseUnitValue:     newBaseUnitValue,
				Description:       "InventoryAdjustment By Value",
				ReferenceType:     models.StockReferenceTypeInventoryAdjustmentValue,
				ReferenceID:       inventoryAdjustment.ID,
				ReferenceDetailID: inventoryAdjustmentDetail.ID,
				IsOutgoing:        utils.NewFalse(),
			}
			err = tx.Create(&stockHistory).Error
			if err != nil {
				config.LogError(logger, "InventoryAdjustmentValueWorkflow.go", "CreateInventoryAdjustmentValue", "CreateIncomingStockHistory", stockHistory, err)
				return 0, nil, 0, nil, err
			}
			stockHistories = append(stockHistories, &stockHistory)

			productPurchaseAmount, ok := productPurchaseAccounts[inventoryAdjustment.AccountId]
			if !ok {
				productPurchaseAmount = decimal.NewFromInt(0)
			}
			productPurchaseAccounts[inventoryAdjustment.AccountId] = productPurchaseAmount.Add(newBaseUnitValue.Mul(totalQty).Sub(totalValue))

			productInventoryAmount, ok := productInventoryAccounts[productDetail.InventoryAccountId]
			if !ok {
				productInventoryAmount = decimal.NewFromInt(0)
			}
			productInventoryAccounts[productDetail.InventoryAccountId] = productInventoryAmount.Add(newBaseUnitValue.Mul(totalQty).Sub(totalValue))
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
		ReferenceType:       models.AccountReferenceTypeInventoryAdjustmentValue,
		AccountTransactions: accTransactions,
	}

	err = tx.Create(&accJournal).Error
	if err != nil {
		tx.Rollback()
		config.LogError(logger, "InventoryAdjustmentValueWorkflow.go", "CreateInventoryAdjustmentValue", "CreateAccountJournal", accJournal, err)
		return 0, nil, 0, nil, err
	}

	return accJournal.ID, accountIds, foreignCurrencyId, stockHistories, nil

}

func DeleteInventoryAdjustmentValue(tx *gorm.DB, logger *logrus.Logger, recordId int, recordType string, businessId string, business models.Business, oldInventoryAdjustment models.InventoryAdjustment) (int, []int, int, []*models.StockHistory, error) {

	foreignCurrencyId := business.BaseCurrencyId
	accountJournal, _, accountIds, err := GetExistingAccountJournal(tx, oldInventoryAdjustment.ID, models.AccountReferenceTypeInventoryAdjustmentValue)
	if err != nil {
		config.LogError(logger, "InventoryAdjustmentValueWorkflow.go", "DeleteInventoryAdjustmentValue", "GetExistingAccountJournal", oldInventoryAdjustment, err)
		return 0, nil, 0, nil, err
	}

	var stockHistories []*models.StockHistory
	err = tx.Where("reference_id = ? AND reference_type = ? AND is_reversal = 0 AND reversed_by_stock_history_id IS NULL", oldInventoryAdjustment.ID, models.AccountReferenceTypeInventoryAdjustmentValue).Find(&stockHistories).Error
	if err != nil {
		config.LogError(logger, "InventoryAdjustmentValueWorkflow.go", "DeleteInventoryAdjustmentValue", "FindStockHistories", oldInventoryAdjustment, err)
		return 0, nil, 0, nil, err
	}
	// Inventory ledger immutability: do not delete IVAV stock history; append reversal entries instead.
	stockReversals, err := ReverseStockHistories(tx, stockHistories, ReversalReasonInventoryAdjustValueVoidUpdate)
	if err != nil {
		config.LogError(logger, "InventoryAdjustmentValueWorkflow.go", "DeleteInventoryAdjustmentValue", "ReverseStockHistories", oldInventoryAdjustment, err)
		return 0, nil, 0, nil, err
	}

	// Phase 1: do not delete posted journals; create a reversal journal instead.
	reversalID, err := ReverseAccountJournal(tx, accountJournal, ReversalReasonInventoryAdjustValueVoidUpdate)
	if err != nil {
		config.LogError(logger, "InventoryAdjustmentValueWorkflow.go", "DeleteInventoryAdjustmentValue", "ReverseAccountJournal", accountJournal, err)
		return 0, nil, 0, nil, err
	}

	// For IVAV processing, we only pass the incoming side (is_outgoing=false).
	incomingStockHistories := make([]*models.StockHistory, 0)
	for _, stockHistory := range stockReversals {
		if stockHistory.IsOutgoing != nil && *stockHistory.IsOutgoing == false {
			incomingStockHistories = append(incomingStockHistories, stockHistory)
		}
	}

	return reversalID, accountIds, foreignCurrencyId, incomingStockHistories, nil
}
