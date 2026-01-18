package workflow

import (
	"encoding/json"
	"errors"
	"strconv"

	"github.com/mmdatafocus/books_backend/config"
	"github.com/mmdatafocus/books_backend/models"
	"github.com/mmdatafocus/books_backend/utils"
	"github.com/shopspring/decimal"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

func ProcessProductOpeningStockWorkflow(tx *gorm.DB, logger *logrus.Logger, msg config.PubSubMessage) error {
	var accountIds []int
	var foreignCurrencyId int
	var branchIds []int
	business, err := models.GetBusinessById2(tx, msg.BusinessId)
	if err != nil {
		config.LogError(logger, "ProductOpeningStockWorkflow.go", "ProcessProductOpeningStockWorkflow", "GetBusiness", msg.BusinessId, err)
		return err
	}
	if msg.Action == string(models.PubSubMessageActionCreate) {

		var openingStock models.ProductOpeningStock
		err := json.Unmarshal([]byte(msg.NewObj), &openingStock)
		if err != nil {
			config.LogError(logger, "ProductOpeningStockWorkflow.go", "ProcessProductOpeningStockWorkflow > Create", "Unmarshal msg.NewObj", msg.NewObj, err)
			return err
		}
		accountIds, foreignCurrencyId, branchIds, err = CreateProductOpeningStock(tx, logger, msg.ID, msg.ReferenceType, msg.BusinessId, *business, openingStock)
		if err != nil {
			config.LogError(logger, "ProductOpeningStockWorkflow.go", "ProcessProductOpeningStockWorkflow > Create", "CreateProductOpeningStock", nil, err)
			return err
		}
		for _, branchId := range branchIds {
			err = UpdateBalances(tx, logger, msg.BusinessId, business.BaseCurrencyId, branchId, accountIds, business.MigrationDate, foreignCurrencyId)
			if err != nil {
				config.LogError(logger, "ProductOpeningStockWorkflow.go", "ProcessProductOpeningStockWorkflow > Create", "UpdateBalances", msg, err)
				return err
			}
		}
	}

	err = tx.Model(&models.PubSubMessageRecord{}).Where("id=?", msg.ID).Updates(map[string]interface{}{"is_processed": true}).Error
	if err != nil {
		config.LogError(logger, "ProductOpeningStockWorkflow.go", "ProcessProductOpeningStockWorkflow", "UpdatePubSubMessageRecord", msg.ID, err)
		return err
	}
	return nil
}

func CreateProductOpeningStock(tx *gorm.DB, logger *logrus.Logger, recordId int, recordType string, businessId string, business models.Business, openingStock models.ProductOpeningStock) ([]int, int, []int, error) {

	systemAccounts, err := models.GetSystemAccounts(businessId)
	if err != nil {
		config.LogError(logger, "ProductOpeningStockWorkflow.go", "CreateProductOpeningStock", "GetSystemAccounts", businessId, err)
		return nil, 0, nil, err
	}

	warehouseTotalAmounts := make(map[int]decimal.Decimal)
	stockHistories := make([]*models.StockHistory, 0)
	stockDate, err := utils.ConvertToDate(business.MigrationDate, business.Timezone)
	if err != nil {
		return nil, 0, nil, err
	}

	for _, detail := range openingStock.Details {
		amount, ok := warehouseTotalAmounts[detail.WarehouseId]
		if !ok {
			amount = decimal.NewFromInt(0)
		}
		warehouseTotalAmounts[detail.WarehouseId] = amount.Add(detail.Qty.Mul(detail.UnitValue))

		existing := new(models.StockHistory)
		findErr := tx.
			Where("business_id = ? AND warehouse_id = ? AND product_id = ? AND product_type = ? AND COALESCE(batch_number,'') = ? AND reference_type = ? AND reference_id = ? AND is_outgoing = false AND is_reversal = 0 AND reversed_by_stock_history_id IS NULL",
				businessId, detail.WarehouseId, openingStock.ProductId, models.ProductTypeSingle, detail.BatchNumber, models.StockReferenceTypeProductOpeningStock, openingStock.ProductId).
			First(existing).Error
		if findErr == nil {
			stockHistories = append(stockHistories, existing)
			continue
		}
		if !errors.Is(findErr, gorm.ErrRecordNotFound) {
			return nil, 0, nil, findErr
		}

		stockHistory := models.StockHistory{
			BusinessId:        businessId,
			WarehouseId:       detail.WarehouseId,
			ProductId:         openingStock.ProductId,
			ProductType:       models.ProductTypeSingle,
			BatchNumber:       detail.BatchNumber,
			StockDate:         stockDate,
			Qty:               detail.Qty,
			Description:       "Opening Stock",
			ReferenceType:     models.StockReferenceTypeProductOpeningStock,
			ReferenceID:       openingStock.ProductId,
			ReferenceDetailID: 0,
			IsOutgoing:        utils.NewFalse(),
			BaseUnitValue:     detail.UnitValue,
		}
		err = tx.Create(&stockHistory).Error
		if err != nil {
			config.LogError(logger, "ProductOpeningStockWorkflow.go", "CreateProductOpeningStock", "CreateStockHistory", stockHistory, err)
			return nil, 0, nil, err
		}
		stockHistories = append(stockHistories, &stockHistory)

		// Keep stock_summaries in sync with stock_histories for opening stock.
		if err := models.UpdateStockSummaryOpeningQty(tx, businessId, detail.WarehouseId, openingStock.ProductId, string(models.ProductTypeSingle), detail.BatchNumber, detail.Qty, stockDate); err != nil {
			config.LogError(logger, "ProductOpeningStockWorkflow.go", "CreateProductOpeningStock", "UpdateStockSummaryOpeningQty", detail, err)
			return nil, 0, nil, err
		}
	}

	branchIds := make([]int, 0)
	branchTotalAmounts := make(map[int]decimal.Decimal)
	for warehouseId, totalAmount := range warehouseTotalAmounts {
		var warehouse models.Warehouse
		err = tx.Where("id = ?", warehouseId).First(&warehouse).Error
		if err != nil {
			return nil, 0, nil, err
		}
		amount, ok := branchTotalAmounts[warehouse.BranchId]
		if !ok {
			amount = decimal.NewFromInt(0)
		}
		branchTotalAmounts[warehouse.BranchId] = amount.Add(totalAmount)
	}

	for branchId, totalAmount := range branchTotalAmounts {
		branchIds = append(branchIds, branchId)

		inventory := models.AccountTransaction{
			BusinessId:          businessId,
			AccountId:           openingStock.InventoryAccountId,
			BranchId:            branchId,
			TransactionDateTime: business.MigrationDate,
			BaseCurrencyId:      business.BaseCurrencyId,
			BaseDebit:           totalAmount,
			BaseCredit:          decimal.NewFromInt(0),
			ForeignCurrencyId:   business.BaseCurrencyId,
			ForeignDebit:        decimal.NewFromInt(0),
			ForeignCredit:       decimal.NewFromInt(0),
			ExchangeRate:        decimal.NewFromInt(0),
		}

		openingBalanceAdjustments := models.AccountTransaction{
			BusinessId:          businessId,
			AccountId:           systemAccounts[models.AccountCodeOpeningBalanceAdjustments],
			BranchId:            branchId,
			TransactionDateTime: business.MigrationDate,
			BaseCurrencyId:      business.BaseCurrencyId,
			BaseDebit:           decimal.NewFromInt(0),
			BaseCredit:          totalAmount,
			ForeignCurrencyId:   business.BaseCurrencyId,
			ForeignCredit:       decimal.NewFromInt(0),
			ForeignDebit:        decimal.NewFromInt(0),
			ExchangeRate:        decimal.NewFromInt(0),
		}

		journal := models.AccountJournal{
			BusinessId:          businessId,
			BranchId:            branchId,
			TransactionDateTime: business.MigrationDate,
			TransactionNumber:   strconv.Itoa(openingStock.ProductId),
			TransactionDetails:  "Product Opening Stock",
			ReferenceId:         openingStock.ProductId,
			ReferenceType:       models.AccountReferenceTypeProductOpeningStock,
			AccountTransactions: []models.AccountTransaction{inventory, openingBalanceAdjustments},
		}

		err = tx.Create(&journal).Error
		if err != nil {
			config.LogError(logger, "ProductOpeningStockWorkflow.go", "CreateProductOpeningStock", "CreateAccountJournal", journal, err)
			return nil, 0, nil, err
		}

		err = models.UpdateStockClosingBalances(tx, stockHistories, nil)
		if err != nil {
			config.LogError(logger, "ProductOpeningStockWorkflow.go", "CreateProductOpeningStock", "UpdateStockClosingBalances", journal, err)
			return nil, 0, nil, err
		}
	}

	accountIds := []int{openingStock.InventoryAccountId, systemAccounts[models.AccountCodeOpeningBalanceAdjustments]}

	return accountIds, business.BaseCurrencyId, branchIds, nil
}
