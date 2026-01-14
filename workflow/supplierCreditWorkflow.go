package workflow

import (
	"encoding/json"
	"slices"

	"github.com/mmdatafocus/books_backend/config"
	"github.com/mmdatafocus/books_backend/models"
	"github.com/mmdatafocus/books_backend/utils"
	"github.com/shopspring/decimal"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

func ProcessSupplierCreditWorkflow(tx *gorm.DB, logger *logrus.Logger, msg config.PubSubMessage) error {

	var accountJournalId int
	var accountIds []int
	var foreignCurrencyId int
	var stockHistories []*models.StockHistory
	business, err := models.GetBusinessById2(tx, msg.BusinessId)
	if err != nil {
		config.LogError(logger, "SupplierCreditWorkflow.go", "ProcessSupplierCreditWorkflow", "GetBusiness", msg.BusinessId, err)
		return err
	}
	if msg.Action == string(models.PubSubMessageActionCreate) {

		var supplierCredit models.SupplierCredit
		err := json.Unmarshal([]byte(msg.NewObj), &supplierCredit)
		if err != nil {
			config.LogError(logger, "SupplierCreditWorkflow.go", "ProcessSupplierCreditWorkflow > Create", "Unmarshal msg.NewObj", msg.NewObj, err)
			return err
		}
		accountJournalId, accountIds, foreignCurrencyId, stockHistories, err = CreateSupplierCredit(tx, logger, msg.ID, msg.ReferenceType, msg.BusinessId, *business, supplierCredit)
		if err != nil {
			config.LogError(logger, "SupplierCreditWorkflow.go", "ProcessSupplierCreditWorkflow > Create", "CreateSupplierCredit", nil, err)
			return err
		}
		valuationAccountIds, err := ProcessOutgoingStocks(tx, logger, stockHistories)
		if err != nil {
			config.LogError(logger, "SupplierCreditWorkflow.go", "ProcessSupplierCreditWorkflow > Create", "ProcessOutgoingStocks", stockHistories, err)
			return err
		}

		if len(valuationAccountIds) > 0 {
			for _, accId := range valuationAccountIds {
				if !slices.Contains(accountIds, accId) {
					accountIds = append(accountIds, accId)
				}
			}
		}
		err = UpdateBalances(tx, logger, msg.BusinessId, business.BaseCurrencyId, supplierCredit.BranchId, accountIds, supplierCredit.SupplierCreditDate, foreignCurrencyId)
		if err != nil {
			config.LogError(logger, "SupplierCreditWorkflow.go", "ProcessSupplierCreditWorkflow > Create", "UpdateBalances", supplierCredit, err)
			return err
		}

	} else if msg.Action == string(models.PubSubMessageActionUpdate) {

		var oldForeignCurrencyId int
		var oldAccountIds []int
		var supplierCredit models.SupplierCredit
		var oldStockHistories []*models.StockHistory
		err := json.Unmarshal([]byte(msg.NewObj), &supplierCredit)
		if err != nil {
			config.LogError(logger, "SupplierCreditWorkflow.go", "ProcessSupplierCreditWorkflow > Update", "Unmarshal msg.NewObj", msg.NewObj, err)
			return err
		}
		var oldSupplierCredit models.SupplierCredit
		err = json.Unmarshal([]byte(msg.OldObj), &oldSupplierCredit)
		if err != nil {
			config.LogError(logger, "SupplierCreditWorkflow.go", "ProcessSupplierCreditWorkflow > Update", "Unmarshal msg.OldObj", msg.OldObj, err)
			return err
		}
		_, oldAccountIds, oldForeignCurrencyId, oldStockHistories, err = DeleteSupplierCredit(tx, logger, msg.ID, msg.ReferenceType, msg.BusinessId, *business, oldSupplierCredit)
		if err != nil {
			config.LogError(logger, "SupplierCreditWorkflow.go", "ProcessSupplierCreditWorkflow > Update", "DeleteSupplierCredit", nil, err)
			return err
		}
		accountJournalId, accountIds, foreignCurrencyId, stockHistories, err = CreateSupplierCredit(tx, logger, msg.ID, msg.ReferenceType, msg.BusinessId, *business, supplierCredit)
		if err != nil {
			config.LogError(logger, "SupplierCreditWorkflow.go", "ProcessSupplierCreditWorkflow > Update", "CreateSupplierCredit", nil, err)
			return err
		}
		mergedStockHistories := mergeStockHistories(stockHistories, oldStockHistories)
		valuationAccountIds, err := ProcessStockHistories(tx, logger, mergedStockHistories)
		if err != nil {
			config.LogError(logger, "SupplierCreditWorkflow.go", "ProcessSupplierCreditWorkflow > Update", "ProcessOutgoingStocks", mergedStockHistories, err)
			return err
		}

		if len(valuationAccountIds) > 0 {
			for _, accId := range valuationAccountIds {
				if !slices.Contains(accountIds, accId) {
					accountIds = append(accountIds, accId)
				}
			}
		}
		if oldSupplierCredit.BranchId != supplierCredit.BranchId || oldSupplierCredit.SupplierCreditDate != supplierCredit.SupplierCreditDate || oldForeignCurrencyId != foreignCurrencyId {
			err = UpdateBalances(tx, logger, msg.BusinessId, business.BaseCurrencyId, oldSupplierCredit.BranchId, oldAccountIds, oldSupplierCredit.SupplierCreditDate, oldForeignCurrencyId)
			if err != nil {
				config.LogError(logger, "SupplierCreditWorkflow.go", "ProcessSupplierCreditWorkflow > Update", "UpdateBalances Old", oldSupplierCredit, err)
				return err
			}
		} else {
			accountIds = utils.MergeIntSlices(accountIds, oldAccountIds)
		}
		err = UpdateBalances(tx, logger, msg.BusinessId, business.BaseCurrencyId, supplierCredit.BranchId, accountIds, supplierCredit.SupplierCreditDate, foreignCurrencyId)
		if err != nil {
			config.LogError(logger, "SupplierCreditWorkflow.go", "ProcessSupplierCreditWorkflow > Update", "UpdateBalances", supplierCredit, err)
			return err
		}
	} else if msg.Action == string(models.PubSubMessageActionDelete) {

		var oldSupplierCredit models.SupplierCredit
		err = json.Unmarshal([]byte(msg.OldObj), &oldSupplierCredit)
		if err != nil {
			config.LogError(logger, "SupplierCreditWorkflow.go", "ProcessSupplierCreditWorkflow > Delete", "Unmarshal msg.OldObj", msg.OldObj, err)
			return err
		}
		accountJournalId, accountIds, foreignCurrencyId, stockHistories, err = DeleteSupplierCredit(tx, logger, msg.ID, msg.ReferenceType, msg.BusinessId, *business, oldSupplierCredit)
		if err != nil {
			config.LogError(logger, "SupplierCreditWorkflow.go", "ProcessSupplierCreditWorkflow > Delete", "DeleteSupplierCredit", nil, err)
			return err
		}
		valuationAccountIds, err := ProcessStockHistories(tx, logger, stockHistories)
		if err != nil {
			config.LogError(logger, "SupplierCreditWorkflow.go", "ProcessSupplierCreditWorkflow > Delete", "ProcessOutgoingStocks", stockHistories, err)
			return err
		}

		if len(valuationAccountIds) > 0 {
			for _, accId := range valuationAccountIds {
				if !slices.Contains(accountIds, accId) {
					accountIds = append(accountIds, accId)
				}
			}
		}
		err = UpdateBalances(tx, logger, msg.BusinessId, business.BaseCurrencyId, oldSupplierCredit.BranchId, accountIds, oldSupplierCredit.SupplierCreditDate, foreignCurrencyId)
		if err != nil {
			config.LogError(logger, "SupplierCreditWorkflow.go", "ProcessSupplierCreditWorkflow > Delete", "UpdateBalances", oldSupplierCredit, err)
			return err
		}
	}
	err = tx.Model(&models.PubSubMessageRecord{}).Where("id=?", msg.ID).Updates(map[string]interface{}{"account_journal_id": accountJournalId, "is_processed": true}).Error
	if err != nil {
		config.LogError(logger, "SupplierCreditWorkflow.go", "ProcessSupplierCreditWorkflow", "UpdatePubSubMessageRecord", accountJournalId, err)
		return err
	}
	return nil
}

func CreateSupplierCredit(tx *gorm.DB, logger *logrus.Logger, recordId int, recordType string, businessId string, business models.Business, supplierCredit models.SupplierCredit) (int, []int, int, []*models.StockHistory, error) {

	systemAccounts, err := models.GetSystemAccounts(businessId)
	if err != nil {
		config.LogError(logger, "SupplierCreditWorkflow.go", "CreateSupplierCredit", "GetSystemAccounts", businessId, err)
		return 0, nil, 0, nil, err
	}

	exchangeRate := supplierCredit.ExchangeRate
	transactionTime := supplierCredit.SupplierCreditDate
	baseCurrencyId := business.BaseCurrencyId
	foreignCurrencyId := supplierCredit.CurrencyId
	branchId := supplierCredit.BranchId
	baseDiscountAmount := supplierCredit.SupplierCreditTotalDiscountAmount
	baseTaxAmount := supplierCredit.SupplierCreditTotalTaxAmount
	basePayableAmount := supplierCredit.SupplierCreditTotalAmount
	baseAdjustmentAmount := supplierCredit.AdjustmentAmount
	foreignDiscountAmount := decimal.NewFromInt(0)
	foreignTaxAmount := decimal.NewFromInt(0)
	foreignPayableAmount := decimal.NewFromInt(0)
	foreignAdjustmentAmount := decimal.NewFromInt(0)

	if baseCurrencyId != foreignCurrencyId {
		foreignDiscountAmount = baseDiscountAmount
		baseDiscountAmount = foreignDiscountAmount.Mul(exchangeRate)
		foreignTaxAmount = baseTaxAmount
		baseTaxAmount = foreignTaxAmount.Mul(exchangeRate)
		foreignPayableAmount = basePayableAmount
		basePayableAmount = foreignPayableAmount.Mul(exchangeRate)
		baseAdjustmentAmount = foreignAdjustmentAmount.Mul(exchangeRate)
	}

	accountIds := make([]int, 0)
	accTransactions := make([]models.AccountTransaction, 0)
	accountIds = append(accountIds, systemAccounts[models.AccountCodeAccountsPayable])

	accountsPayable := models.AccountTransaction{
		BusinessId:          businessId,
		AccountId:           systemAccounts[models.AccountCodeAccountsPayable],
		BranchId:            branchId,
		TransactionDateTime: transactionTime,
		BaseCurrencyId:      baseCurrencyId,
		BaseDebit:           basePayableAmount,
		BaseCredit:          decimal.NewFromInt(0),
		ForeignCurrencyId:   foreignCurrencyId,
		ForeignDebit:        foreignPayableAmount,
		ForeignCredit:       decimal.NewFromInt(0),
		ExchangeRate:        exchangeRate,
	}
	accTransactions = append(accTransactions, accountsPayable)

	if !baseDiscountAmount.IsZero() {
		purchaseDiscount := models.AccountTransaction{
			BusinessId:          businessId,
			AccountId:           systemAccounts[models.AccountCodePurchaseDiscount],
			BranchId:            branchId,
			TransactionDateTime: transactionTime,
			BaseCurrencyId:      baseCurrencyId,
			BaseDebit:           baseDiscountAmount,
			BaseCredit:          decimal.NewFromInt(0),
			ForeignCurrencyId:   foreignCurrencyId,
			ForeignDebit:        foreignDiscountAmount,
			ForeignCredit:       decimal.NewFromInt(0),
			ExchangeRate:        exchangeRate,
		}
		accountIds = append(accountIds, systemAccounts[models.AccountCodePurchaseDiscount])
		accTransactions = append(accTransactions, purchaseDiscount)
	}

	if !baseTaxAmount.IsZero() {
		taxPayable := models.AccountTransaction{
			BusinessId:          businessId,
			AccountId:           systemAccounts[models.AccountCodeTaxPayable],
			BranchId:            branchId,
			TransactionDateTime: transactionTime,
			BaseCurrencyId:      baseCurrencyId,
			BaseDebit:           decimal.NewFromInt(0),
			BaseCredit:          baseTaxAmount,
			ForeignCurrencyId:   foreignCurrencyId,
			ForeignDebit:        decimal.NewFromInt(0),
			ForeignCredit:       foreignTaxAmount,
			ExchangeRate:        exchangeRate,
		}
		accountIds = append(accountIds, systemAccounts[models.AccountCodeTaxPayable])
		accTransactions = append(accTransactions, taxPayable)
	}

	if !baseAdjustmentAmount.IsZero() {
		var otherExpenses models.AccountTransaction
		if baseAdjustmentAmount.IsNegative() {
			otherExpenses = models.AccountTransaction{
				BusinessId:          businessId,
				AccountId:           systemAccounts[models.AccountCodeOtherExpenses],
				BranchId:            branchId,
				TransactionDateTime: transactionTime,
				BaseCurrencyId:      baseCurrencyId,
				BaseDebit:           baseAdjustmentAmount,
				BaseCredit:          decimal.NewFromInt(0),
				ForeignCurrencyId:   foreignCurrencyId,
				ForeignDebit:        foreignAdjustmentAmount,
				ForeignCredit:       decimal.NewFromInt(0),
				ExchangeRate:        exchangeRate,
			}
		} else {
			otherExpenses = models.AccountTransaction{
				BusinessId:          businessId,
				AccountId:           systemAccounts[models.AccountCodeOtherExpenses],
				BranchId:            branchId,
				TransactionDateTime: transactionTime,
				BaseCurrencyId:      baseCurrencyId,
				BaseDebit:           decimal.NewFromInt(0),
				BaseCredit:          baseAdjustmentAmount,
				ForeignCurrencyId:   foreignCurrencyId,
				ForeignDebit:        decimal.NewFromInt(0),
				ForeignCredit:       foreignAdjustmentAmount,
				ExchangeRate:        exchangeRate,
			}
		}
		accountIds = append(accountIds, systemAccounts[models.AccountCodeOtherExpenses])
		accTransactions = append(accTransactions, otherExpenses)
	}

	detailAccounts := make(map[int]decimal.Decimal)
	stockHistories := make([]*models.StockHistory, 0)
	productPurchaseAccounts := make(map[int]decimal.Decimal)
	productInventoryAccounts := make(map[int]decimal.Decimal)
	stockDate, err := utils.ConvertToDate(supplierCredit.SupplierCreditDate, business.Timezone)
	if err != nil {
		return 0, nil, 0, nil, err
	}

	for _, supplierCreditDetail := range supplierCredit.Details {
		amount, ok := detailAccounts[supplierCreditDetail.DetailAccountId]
		if !ok {
			amount = decimal.NewFromInt(0)
		}
		if supplierCredit.IsTaxInclusive != nil && *supplierCredit.IsTaxInclusive {
			amount = amount.Add(supplierCreditDetail.DetailTotalAmount.Add(supplierCreditDetail.DetailDiscountAmount).Sub(supplierCreditDetail.DetailTaxAmount))
		} else {
			amount = amount.Add(supplierCreditDetail.DetailTotalAmount.Add(supplierCreditDetail.DetailDiscountAmount))
		}
		detailAccounts[supplierCreditDetail.DetailAccountId] = amount

		if supplierCreditDetail.ProductId > 0 {
			productDetail, err := GetProductDetail(tx, supplierCreditDetail.ProductId, supplierCreditDetail.ProductType)
			if err != nil {
				config.LogError(logger, "SupplierCreditWorkflow.go", "CreateSupplierCredit", "GetProductDetail", supplierCreditDetail, err)
				return 0, nil, 0, nil, err
			}

			if productDetail.InventoryAccountId > 0 {
				productPurchaseAmount, ok := productPurchaseAccounts[productDetail.PurchaseAccountId]
				if !ok {
					productPurchaseAmount = decimal.NewFromInt(0)
				}
				if supplierCredit.IsTaxInclusive != nil && *supplierCredit.IsTaxInclusive {
					productPurchaseAmount = productPurchaseAmount.Add(supplierCreditDetail.DetailTotalAmount.Add(supplierCreditDetail.DetailDiscountAmount).Sub(supplierCreditDetail.DetailTaxAmount))
				} else {
					productPurchaseAmount = productPurchaseAmount.Add(supplierCreditDetail.DetailTotalAmount.Add(supplierCreditDetail.DetailDiscountAmount))
				}
				productPurchaseAccounts[productDetail.PurchaseAccountId] = productPurchaseAmount

				productInventoryAmount, ok := productInventoryAccounts[productDetail.InventoryAccountId]
				if !ok {
					productInventoryAmount = decimal.NewFromInt(0)
				}
				if supplierCredit.IsTaxInclusive != nil && *supplierCredit.IsTaxInclusive {
					productInventoryAmount = productInventoryAmount.Add(supplierCreditDetail.DetailTotalAmount.Add(supplierCreditDetail.DetailDiscountAmount).Sub(supplierCreditDetail.DetailTaxAmount))
				} else {
					productInventoryAmount = productInventoryAmount.Add(supplierCreditDetail.DetailTotalAmount.Add(supplierCreditDetail.DetailDiscountAmount))
				}
				productInventoryAccounts[productDetail.InventoryAccountId] = productInventoryAmount

				stockHistory := models.StockHistory{
					BusinessId:        supplierCredit.BusinessId,
					WarehouseId:       supplierCredit.WarehouseId,
					ProductId:         supplierCreditDetail.ProductId,
					ProductType:       supplierCreditDetail.ProductType,
					BatchNumber:       supplierCreditDetail.BatchNumber,
					StockDate:         stockDate,
					Qty:               supplierCreditDetail.DetailQty.Neg(),
					Description:       "SupplierCredit #" + supplierCredit.SupplierCreditNumber,
					ReferenceType:     models.StockReferenceTypeSupplierCredit,
					ReferenceID:       supplierCredit.ID,
					ReferenceDetailID: supplierCreditDetail.ID,
					IsOutgoing:        utils.NewTrue(),
				}
				err = tx.Exec("UPDATE supplier_credit_details SET cogs = 0 WHERE id = ?",
					stockHistory.ReferenceDetailID).Error
				if err != nil {
					tx.Rollback()
					config.LogError(logger, "SupplierCreditWorkflow.go", "CreateSupplierCredit", "ResetDetailCogs", stockHistory.ReferenceDetailID, err)
					return 0, nil, 0, nil, err
				}

				err = tx.Create(&stockHistory).Error
				if err != nil {
					tx.Rollback()
					config.LogError(logger, "SupplierCreditWorkflow.go", "CreateSupplierCredit", "CreateStockHistory", stockHistory, err)
					return 0, nil, 0, nil, err
				}
				stockHistories = append(stockHistories, &stockHistory)
			}
		}
	}

	for accId, accAmount := range detailAccounts {
		if !slices.Contains(accountIds, accId) {
			accountIds = append(accountIds, accId)
		}
		if baseCurrencyId != foreignCurrencyId {
			accTransactions = append(accTransactions, models.AccountTransaction{
				BusinessId:          businessId,
				AccountId:           accId,
				BranchId:            branchId,
				TransactionDateTime: transactionTime,
				BaseCurrencyId:      baseCurrencyId,
				BaseDebit:           decimal.NewFromInt(0),
				BaseCredit:          accAmount.Mul(exchangeRate),
				ForeignCurrencyId:   foreignCurrencyId,
				ForeignDebit:        decimal.NewFromInt(0),
				ForeignCredit:       accAmount,
				ExchangeRate:        exchangeRate,
			})
		} else {
			accTransactions = append(accTransactions, models.AccountTransaction{
				BusinessId:          businessId,
				AccountId:           accId,
				BranchId:            branchId,
				TransactionDateTime: transactionTime,
				BaseCurrencyId:      baseCurrencyId,
				BaseDebit:           decimal.NewFromInt(0),
				BaseCredit:          accAmount,
			})
		}
	}

	for purchaseAccId, _ := range productPurchaseAccounts {
		if !slices.Contains(accountIds, purchaseAccId) {
			accountIds = append(accountIds, purchaseAccId)
		}
		if baseCurrencyId != foreignCurrencyId {
			accTransactions = append(accTransactions, models.AccountTransaction{
				BusinessId:          businessId,
				AccountId:           purchaseAccId,
				BranchId:            branchId,
				TransactionDateTime: transactionTime,
				BaseCurrencyId:      baseCurrencyId,
				BaseCredit:          decimal.NewFromInt(0),
				// BaseDebit:            purchaseAmount.Mul(exchangeRate),
				BaseDebit:            decimal.NewFromInt(0),
				IsInventoryValuation: utils.NewTrue(),
			})
		} else {
			accTransactions = append(accTransactions, models.AccountTransaction{
				BusinessId:          businessId,
				AccountId:           purchaseAccId,
				BranchId:            branchId,
				TransactionDateTime: transactionTime,
				BaseCurrencyId:      baseCurrencyId,
				BaseCredit:          decimal.NewFromInt(0),
				// BaseDebit:            purchaseAmount,
				BaseDebit:            decimal.NewFromInt(0),
				IsInventoryValuation: utils.NewTrue(),
			})
		}
	}

	for inventoryAccId, _ := range productInventoryAccounts {
		if !slices.Contains(accountIds, inventoryAccId) {
			accountIds = append(accountIds, inventoryAccId)
		}
		if baseCurrencyId != foreignCurrencyId {
			accTransactions = append(accTransactions, models.AccountTransaction{
				BusinessId:          businessId,
				AccountId:           inventoryAccId,
				BranchId:            branchId,
				TransactionDateTime: transactionTime,
				BaseCurrencyId:      baseCurrencyId,
				BaseDebit:           decimal.NewFromInt(0),
				// BaseCredit:           inventoryAmount.Mul(exchangeRate),
				BaseCredit:           decimal.NewFromInt(0),
				IsInventoryValuation: utils.NewTrue(),
			})
		} else {
			accTransactions = append(accTransactions, models.AccountTransaction{
				BusinessId:          businessId,
				AccountId:           inventoryAccId,
				BranchId:            branchId,
				TransactionDateTime: transactionTime,
				BaseCurrencyId:      baseCurrencyId,
				BaseDebit:           decimal.NewFromInt(0),
				// BaseCredit:           inventoryAmount,
				BaseCredit:           decimal.NewFromInt(0),
				IsInventoryValuation: utils.NewTrue(),
			})
		}
	}

	accJournal := models.AccountJournal{
		BusinessId:          businessId,
		BranchId:            branchId,
		TransactionDateTime: transactionTime,
		TransactionNumber:   supplierCredit.SupplierCreditNumber,
		SupplierId:          supplierCredit.SupplierId,
		ReferenceId:         supplierCredit.ID,
		ReferenceType:       models.AccountReferenceTypeSupplierCredit,
		AccountTransactions: accTransactions,
	}

	err = tx.Create(&accJournal).Error
	if err != nil {
		config.LogError(logger, "SupplierCreditWorkflow.go", "CreateSupplierCredit", "CreateAccountJournal", accJournal, err)
		tx.Rollback()
		return 0, nil, 0, nil, err
	}

	return accJournal.ID, accountIds, foreignCurrencyId, stockHistories, nil
}

func DeleteSupplierCredit(tx *gorm.DB, logger *logrus.Logger, recordId int, recordType string, businessId string, business models.Business, oldSupplierCredit models.SupplierCredit) (int, []int, int, []*models.StockHistory, error) {

	foreignCurrencyId := oldSupplierCredit.CurrencyId
	accountJournal, _, accountIds, err := GetExistingAccountJournal(tx, oldSupplierCredit.ID, models.AccountReferenceTypeSupplierCredit)
	if err != nil {
		config.LogError(logger, "SupplierCreditWorkflow.go", "DeleteSupplierCredit", "GetExistingAccountJournal", oldSupplierCredit, err)
		return 0, nil, 0, nil, err
	}

	var stockHistories []*models.StockHistory
	err = tx.Where("reference_id = ? AND reference_type = ? AND is_reversal = 0 AND reversed_by_stock_history_id IS NULL", oldSupplierCredit.ID, models.StockReferenceTypeSupplierCredit).Find(&stockHistories).Error
	if err != nil {
		config.LogError(logger, "SupplierCreditWorkflow.go", "DeleteSupplierCredit", "FindStockHistories", oldSupplierCredit, err)
		return 0, nil, 0, nil, err
	}
	// Inventory ledger immutability: do not delete stock history; append reversal entries instead.
	stockReversals, err := ReverseStockHistories(tx, stockHistories, ReversalReasonSupplierCreditVoidUpdate)
	if err != nil {
		config.LogError(logger, "SupplierCreditWorkflow.go", "DeleteSupplierCredit", "ReverseStockHistories", oldSupplierCredit, err)
		return 0, nil, 0, nil, err
	}

	// detailAccounts := make(map[int]decimal.Decimal)
	// stocks := make([]*models.Stock, 0)
	// for _, supplierCreditDetail := range oldS.Details {
	// 	amount, ok := detailAccounts[supplierCreditDetail.DetailAccountId]
	// 	if !ok {
	// 		amount = decimal.NewFromInt(0)
	// 	}
	// 	if supplierCredit.IsTaxInclusive != nil && *supplierCredit.IsTaxInclusive {
	// 		amount = amount.Add(supplierCreditDetail.DetailTotalAmount.Add(supplierCreditDetail.DetailDiscountAmount).Sub(supplierCreditDetail.DetailTaxAmount))
	// 	} else {
	// 		amount = amount.Add(supplierCreditDetail.DetailTotalAmount.Add(supplierCreditDetail.DetailDiscountAmount))
	// 	}
	// 	detailAccounts[supplierCreditDetail.DetailAccountId] = amount

	// 	if supplierCreditDetail.ProductId > 0 {
	// 		productDetail, err := GetProductDetail(tx, supplierCreditDetail.ProductId, supplierCreditDetail.ProductType)
	// 		if err != nil {
	// 			return nil, err
	// 		}

	// 		if productDetail.InventoryAccountId > 0 {
	// 			stock := models.Stock{
	// 				BusinessId:    businessId,
	// 				WarehouseId:   supplierCredit.WarehouseId,
	// 				Description:   supplierCredit.SupplierCreditNumber,
	// 				ReferenceType: models.StockReferenceTypeSupplierCredit,
	// 				ReferenceID:   supplierCredit.ID,
	// 				ProductId:     supplierCreditDetail.ProductId,
	// 				ProductType:   supplierCreditDetail.ProductType,
	// 				BatchNumber:   supplierCreditDetail.BatchNumber,
	// 				ReceivedDate:  supplierCredit.SupplierCreditDate,
	// 				Qty:           supplierCreditDetail.DetailQty.Neg(),
	// 			}
	// 			if baseCurrencyId != foreignCurrencyId {
	// 				stock.BaseUnitValue = supplierCreditDetail.DetailUnitRate.Mul(supplierCredit.ExchangeRate)
	// 				stock.ForeignUnitValue = supplierCreditDetail.DetailUnitRate
	// 				stock.ExchangeRate = supplierCredit.ExchangeRate
	// 			} else {
	// 				stock.BaseUnitValue = supplierCreditDetail.DetailUnitRate
	// 			}
	// 			stocks = append(stocks, &stock)
	// 		}
	// 	}
	// }

	// stockFragments := make([]*StockFragment, 0)
	// for _, supplierCreditDetail := range oldSupplierCredit.Details {
	// 	if supplierCreditDetail.ProductId > 0 {
	// 		productDetail, err := GetProductDetail(tx, supplierCreditDetail.ProductId, supplierCreditDetail.ProductType)
	// 		if err != nil {
	// 			config.LogError(logger, "SupplierCreditWorkflow.go", "DeleteSupplierCredit", "GetProductDetail", supplierCreditDetail, err)
	// 			return 0, nil, 0, nil, err
	// 		}

	// 		if productDetail.InventoryAccountId > 0 {
	// 			stockFragments = append(stockFragments, &StockFragment{
	// 				WarehouseId:  oldSupplierCredit.WarehouseId,
	// 				ProductId:    supplierCreditDetail.ProductId,
	// 				ProductType:  supplierCreditDetail.ProductType,
	// 				BatchNumber:  supplierCreditDetail.BatchNumber,
	// 				ReceivedDate: oldSupplierCredit.SupplierCreditDate,
	// 			})
	// 		}
	// 	}
	// }

	// Phase 1: do not delete posted journals; create a reversal journal instead.
	reversalID, err := ReverseAccountJournal(tx, accountJournal, ReversalReasonSupplierCreditVoidUpdate)
	if err != nil {
		config.LogError(logger, "SupplierCreditWorkflow.go", "DeleteSupplierCredit", "ReverseAccountJournal", accountJournal, err)
		return 0, nil, 0, nil, err
	}

	return reversalID, accountIds, foreignCurrencyId, stockReversals, nil
}
