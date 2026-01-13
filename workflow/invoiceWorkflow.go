package workflow

import (
	"encoding/json"
	"slices"

	"bitbucket.org/mmdatafocus/books_backend/config"
	"bitbucket.org/mmdatafocus/books_backend/models"
	"bitbucket.org/mmdatafocus/books_backend/utils"
	"github.com/shopspring/decimal"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

func ProcessInvoiceWorkflow(tx *gorm.DB, logger *logrus.Logger, msg config.PubSubMessage) error {

	var accountJournalId int
	var accountIds []int
	var foreignCurrencyId int
	var stockHistories []*models.StockHistory
	business, err := models.GetBusinessById2(tx, msg.BusinessId)
	if err != nil {
		config.LogError(logger, "InvoiceWorkflow.go", "ProcessInvoiceWorkflow", "GetBusiness", msg.BusinessId, err)
		return err
	}
	if msg.Action == string(models.PubSubMessageActionCreate) {

		var invoice models.SalesInvoice
		err := json.Unmarshal([]byte(msg.NewObj), &invoice)
		if err != nil {
			config.LogError(logger, "InvoiceWorkflow.go", "ProcessInvoiceWorkflow > Create", "Unmarshal msg.NewObj", msg.NewObj, err)
			return err
		}
		accountJournalId, accountIds, foreignCurrencyId, stockHistories, err = CreateInvoice(tx, logger, msg.ID, msg.ReferenceType, msg.BusinessId, *business, invoice)
		if err != nil {
			config.LogError(logger, "InvoiceWorkflow.go", "ProcessInvoiceWorkflow > Create", "CreateInvoice", nil, err)
			return err
		}
		valuationAccountIds, err := ProcessOutgoingStocks(tx, logger, stockHistories)
		if err != nil {
			config.LogError(logger, "InvoiceWorkflow.go", "ProcessInvoiceWorkflow > Create", "ProcessInventoryValuation", stockHistories, err)
			return err
		}

		if len(valuationAccountIds) > 0 {
			for _, accId := range valuationAccountIds {
				if !slices.Contains(accountIds, accId) {
					accountIds = append(accountIds, accId)
				}
			}
		}
		err = UpdateBalances(tx, logger, msg.BusinessId, business.BaseCurrencyId, invoice.BranchId, accountIds, invoice.InvoiceDate, foreignCurrencyId)
		if err != nil {
			config.LogError(logger, "InvoiceWorkflow.go", "ProcessInvoiceWorkflow > Create", "UpdateBalances", invoice, err)
			return err
		}
	} else if msg.Action == string(models.PubSubMessageActionUpdate) {

		var oldForeignCurrencyId int
		var oldAccountIds []int
		var invoice models.SalesInvoice
		var oldStockHistories []*models.StockHistory
		err := json.Unmarshal([]byte(msg.NewObj), &invoice)
		if err != nil {
			config.LogError(logger, "InvoiceWorkflow.go", "ProcessInvoiceWorkflow > Update", "Unmarshal msg.NewObj", msg.NewObj, err)
			return err
		}
		var oldInvoice models.SalesInvoice
		err = json.Unmarshal([]byte(msg.OldObj), &oldInvoice)
		if err != nil {
			config.LogError(logger, "InvoiceWorkflow.go", "ProcessInvoiceWorkflow > Update", "Unmarshal msg.OldObj", msg.OldObj, err)
			return err
		}
		_, oldAccountIds, oldForeignCurrencyId, oldStockHistories, err = DeleteInvoice(tx, logger, msg.ID, msg.ReferenceType, msg.BusinessId, *business, oldInvoice)
		if err != nil {
			config.LogError(logger, "InvoiceWorkflow.go", "ProcessInvoiceWorkflow > Update", "DeleteInvoice", nil, err)
			return err
		}
		accountJournalId, accountIds, foreignCurrencyId, stockHistories, err = CreateInvoice(tx, logger, msg.ID, msg.ReferenceType, msg.BusinessId, *business, invoice)
		if err != nil {
			config.LogError(logger, "InvoiceWorkflow.go", "ProcessInvoiceWorkflow > Update", "CreateInvoice", nil, err)
			return err
		}
		mergedStockHistories := mergeStockHistories(stockHistories, oldStockHistories)
		valuationAccountIds, err := ProcessStockHistories(tx, logger, mergedStockHistories)
		if err != nil {
			config.LogError(logger, "InvoiceWorkflow.go", "ProcessInvoiceWorkflow > Update", "ProcessInventoryValuation", mergedStockHistories, err)
			return err
		}

		if len(valuationAccountIds) > 0 {
			for _, accId := range valuationAccountIds {
				if !slices.Contains(accountIds, accId) {
					accountIds = append(accountIds, accId)
				}
			}
		}
		if oldInvoice.BranchId != invoice.BranchId || oldInvoice.InvoiceDate != invoice.InvoiceDate || oldForeignCurrencyId != foreignCurrencyId {
			err = UpdateBalances(tx, logger, msg.BusinessId, business.BaseCurrencyId, oldInvoice.BranchId, oldAccountIds, oldInvoice.InvoiceDate, oldForeignCurrencyId)
			if err != nil {
				config.LogError(logger, "InvoiceWorkflow.go", "ProcessInvoiceWorkflow > Update", "UpdateBalances Old", oldInvoice, err)
				return err
			}
		} else {
			accountIds = utils.MergeIntSlices(accountIds, oldAccountIds)
		}
		err = UpdateBalances(tx, logger, msg.BusinessId, business.BaseCurrencyId, invoice.BranchId, accountIds, invoice.InvoiceDate, foreignCurrencyId)
		if err != nil {
			config.LogError(logger, "InvoiceWorkflow.go", "ProcessInvoiceWorkflow > Update", "UpdateBalances", invoice, err)
			return err
		}
	} else if msg.Action == string(models.PubSubMessageActionDelete) {

		var oldInvoice models.SalesInvoice
		err = json.Unmarshal([]byte(msg.OldObj), &oldInvoice)
		if err != nil {
			config.LogError(logger, "InvoiceWorkflow.go", "ProcessInvoiceWorkflow > Delete", "Unmarshal msg.OldObj", msg.OldObj, err)
			return err
		}
		accountJournalId, accountIds, foreignCurrencyId, stockHistories, err = DeleteInvoice(tx, logger, msg.ID, msg.ReferenceType, msg.BusinessId, *business, oldInvoice)
		if err != nil {
			config.LogError(logger, "InvoiceWorkflow.go", "ProcessInvoiceWorkflow > Delete", "DeleteInvoice", nil, err)
			return err
		}
		valuationAccountIds, err := ProcessStockHistories(tx, logger, stockHistories)
		if err != nil {
			config.LogError(logger, "InvoiceWorkflow.go", "ProcessInvoiceWorkflow > Delete", "ProcessOutgoingStocks", stockHistories, err)
			return err
		}

		if len(valuationAccountIds) > 0 {
			for _, accId := range valuationAccountIds {
				if !slices.Contains(accountIds, accId) {
					accountIds = append(accountIds, accId)
				}
			}
		}
		err = UpdateBalances(tx, logger, msg.BusinessId, business.BaseCurrencyId, oldInvoice.BranchId, accountIds, oldInvoice.InvoiceDate, foreignCurrencyId)
		if err != nil {
			config.LogError(logger, "InvoiceWorkflow.go", "ProcessInvoiceWorkflow > Delete", "UpdateBalances", oldInvoice, err)
			return err
		}
	}
	err = tx.Model(&models.PubSubMessageRecord{}).Where("id=?", msg.ID).Updates(map[string]interface{}{"account_journal_id": accountJournalId, "is_processed": true}).Error
	if err != nil {
		config.LogError(logger, "InvoiceWorkflow.go", "ProcessInvoiceWorkflow", "UpdatePubSubMessageRecord", accountJournalId, err)
		return err
	}
	return nil
}

func CreateInvoice(tx *gorm.DB, logger *logrus.Logger, recordId int, recordType string, businessId string, business models.Business, invoice models.SalesInvoice) (int, []int, int, []*models.StockHistory, error) {

	systemAccounts, err := models.GetSystemAccounts(businessId)
	if err != nil {
		config.LogError(logger, "InvoiceWorkflow.go", "CreateInvoice", "GetSystemAccounts", businessId, err)
		return 0, nil, 0, nil, err
	}

	exchangeRate := invoice.ExchangeRate
	transactionTime := invoice.InvoiceDate
	baseCurrencyId := business.BaseCurrencyId
	foreignCurrencyId := invoice.CurrencyId
	branchId := invoice.BranchId
	baseDiscountAmount := invoice.InvoiceTotalDiscountAmount
	baseTaxAmount := invoice.InvoiceTotalTaxAmount
	baseReceivableAmount := invoice.InvoiceTotalAmount
	baseAdjustmentAmount := invoice.AdjustmentAmount
	baseShippingCharges := invoice.ShippingCharges
	foreignDiscountAmount := decimal.NewFromInt(0)
	foreignTaxAmount := decimal.NewFromInt(0)
	foreignReceivableAmount := decimal.NewFromInt(0)
	foreignAdjustmentAmount := decimal.NewFromInt(0)
	foreignShippingCharges := decimal.NewFromInt(0)

	if baseCurrencyId != foreignCurrencyId {
		foreignDiscountAmount = baseDiscountAmount
		baseDiscountAmount = foreignDiscountAmount.Mul(exchangeRate)
		foreignTaxAmount = baseTaxAmount
		baseTaxAmount = foreignTaxAmount.Mul(exchangeRate)
		foreignReceivableAmount = baseReceivableAmount
		foreignAdjustmentAmount = baseAdjustmentAmount
		foreignShippingCharges = baseShippingCharges
		baseReceivableAmount = foreignReceivableAmount.Mul(exchangeRate)
		baseAdjustmentAmount = foreignAdjustmentAmount.Mul(exchangeRate)
		baseShippingCharges = foreignShippingCharges.Mul(exchangeRate)
	}

	accountIds := make([]int, 0)
	accTransactions := make([]models.AccountTransaction, 0)
	accountIds = append(accountIds, systemAccounts[models.AccountCodeAccountsReceivable])

	accountsReceivable := models.AccountTransaction{
		BusinessId:          businessId,
		AccountId:           systemAccounts[models.AccountCodeAccountsReceivable],
		BranchId:            branchId,
		TransactionDateTime: transactionTime,
		BaseCurrencyId:      baseCurrencyId,
		BaseCredit:          decimal.NewFromInt(0),
		BaseDebit:           baseReceivableAmount,
		ForeignCurrencyId:   foreignCurrencyId,
		ForeignCredit:       decimal.NewFromInt(0),
		ForeignDebit:        foreignReceivableAmount,
		ExchangeRate:        exchangeRate,
	}
	accTransactions = append(accTransactions, accountsReceivable)

	if !baseDiscountAmount.IsZero() {
		discount := models.AccountTransaction{
			BusinessId:          businessId,
			AccountId:           systemAccounts[models.AccountCodeDiscount],
			BranchId:            branchId,
			TransactionDateTime: transactionTime,
			BaseCurrencyId:      baseCurrencyId,
			BaseCredit:          decimal.NewFromInt(0),
			BaseDebit:           baseDiscountAmount,
			ForeignCurrencyId:   foreignCurrencyId,
			ForeignCredit:       decimal.NewFromInt(0),
			ForeignDebit:        foreignDiscountAmount,
			ExchangeRate:        exchangeRate,
		}
		accountIds = append(accountIds, systemAccounts[models.AccountCodeDiscount])
		accTransactions = append(accTransactions, discount)
	}

	if !baseTaxAmount.IsZero() {
		taxPayable := models.AccountTransaction{
			BusinessId:          businessId,
			AccountId:           systemAccounts[models.AccountCodeTaxPayable],
			BranchId:            branchId,
			TransactionDateTime: transactionTime,
			BaseCurrencyId:      baseCurrencyId,
			BaseCredit:          baseTaxAmount,
			BaseDebit:           decimal.NewFromInt(0),
			ForeignCurrencyId:   foreignCurrencyId,
			ForeignCredit:       foreignTaxAmount,
			ForeignDebit:        decimal.NewFromInt(0),
			ExchangeRate:        exchangeRate,
		}
		accountIds = append(accountIds, systemAccounts[models.AccountCodeTaxPayable])
		accTransactions = append(accTransactions, taxPayable)
	}

	if !baseShippingCharges.IsZero() {
		shippingCharges := models.AccountTransaction{
			BusinessId:          businessId,
			AccountId:           systemAccounts[models.AccountCodeShippingCharge],
			BranchId:            branchId,
			TransactionDateTime: transactionTime,
			BaseCurrencyId:      baseCurrencyId,
			BaseDebit:           decimal.NewFromInt(0),
			BaseCredit:          baseShippingCharges,
			ForeignCurrencyId:   foreignCurrencyId,
			ForeignDebit:        decimal.NewFromInt(0),
			ForeignCredit:       foreignShippingCharges,
			ExchangeRate:        exchangeRate,
		}
		accountIds = append(accountIds, systemAccounts[models.AccountCodeShippingCharge])
		accTransactions = append(accTransactions, shippingCharges)
	}

	if !baseAdjustmentAmount.IsZero() {
		var otherCharges models.AccountTransaction
		if baseAdjustmentAmount.IsNegative() {
			otherCharges = models.AccountTransaction{
				BusinessId:          businessId,
				AccountId:           systemAccounts[models.AccountCodeOtherCharges],
				BranchId:            branchId,
				TransactionDateTime: transactionTime,
				BaseCurrencyId:      baseCurrencyId,
				BaseCredit:          decimal.NewFromInt(0),
				BaseDebit:           baseAdjustmentAmount.Abs(),
				ForeignCurrencyId:   foreignCurrencyId,
				ForeignCredit:       decimal.NewFromInt(0),
				ForeignDebit:        foreignAdjustmentAmount,
				ExchangeRate:        exchangeRate,
			}
		} else {
			otherCharges = models.AccountTransaction{
				BusinessId:          businessId,
				AccountId:           systemAccounts[models.AccountCodeOtherCharges],
				BranchId:            branchId,
				TransactionDateTime: transactionTime,
				BaseCurrencyId:      baseCurrencyId,
				BaseCredit:          baseAdjustmentAmount.Abs(),
				BaseDebit:           decimal.NewFromInt(0),
				ForeignCurrencyId:   foreignCurrencyId,
				ForeignCredit:       foreignAdjustmentAmount,
				ForeignDebit:        decimal.NewFromInt(0),
				ExchangeRate:        exchangeRate,
			}
		}
		accountIds = append(accountIds, systemAccounts[models.AccountCodeOtherCharges])
		accTransactions = append(accTransactions, otherCharges)
	}

	detailAccounts := make(map[int]decimal.Decimal)
	stockHistories := make([]*models.StockHistory, 0)
	productPurchaseAccounts := make(map[int]decimal.Decimal)
	productInventoryAccounts := make(map[int]decimal.Decimal)
	stockDate, err := utils.ConvertToDate(invoice.InvoiceDate, business.Timezone)
	if err != nil {
		return 0, nil, 0, nil, err
	}

	for _, invoiceDetail := range invoice.Details {
		detailAccountId := invoiceDetail.DetailAccountId
		if detailAccountId == 0 {
			detailAccountId = systemAccounts[models.AccountCodeSales]
		}
		amount, ok := detailAccounts[detailAccountId]
		if !ok {
			amount = decimal.NewFromInt(0)
		}
		if invoice.IsTaxInclusive != nil && *invoice.IsTaxInclusive {
			amount = amount.Add(invoiceDetail.DetailTotalAmount.Add(invoiceDetail.DetailDiscountAmount).Sub(invoiceDetail.DetailTaxAmount))
		} else {
			amount = amount.Add(invoiceDetail.DetailTotalAmount.Add(invoiceDetail.DetailDiscountAmount))
		}
		detailAccounts[detailAccountId] = amount

		if invoiceDetail.ProductId > 0 {
			productDetail, err := GetProductDetail(tx, invoiceDetail.ProductId, invoiceDetail.ProductType)
			if err != nil {
				config.LogError(logger, "InvoiceWorkflow.go", "CreateInvoice", "GetProductDetail", invoiceDetail, err)
				return 0, nil, 0, nil, err
			}

			if productDetail.InventoryAccountId > 0 {
				productPurchaseAmount, ok := productPurchaseAccounts[productDetail.PurchaseAccountId]
				if !ok {
					productPurchaseAmount = decimal.NewFromInt(0)
				}
				if invoice.IsTaxInclusive != nil && *invoice.IsTaxInclusive {
					productPurchaseAmount = productPurchaseAmount.Add(invoiceDetail.DetailTotalAmount.Add(invoiceDetail.DetailDiscountAmount).Sub(invoiceDetail.DetailTaxAmount))
				} else {
					productPurchaseAmount = productPurchaseAmount.Add(invoiceDetail.DetailTotalAmount.Add(invoiceDetail.DetailDiscountAmount))
				}
				productPurchaseAccounts[productDetail.PurchaseAccountId] = productPurchaseAmount

				productInventoryAmount, ok := productInventoryAccounts[productDetail.InventoryAccountId]
				if !ok {
					productInventoryAmount = decimal.NewFromInt(0)
				}
				if invoice.IsTaxInclusive != nil && *invoice.IsTaxInclusive {
					productInventoryAmount = productInventoryAmount.Add(invoiceDetail.DetailTotalAmount.Add(invoiceDetail.DetailDiscountAmount).Sub(invoiceDetail.DetailTaxAmount))
				} else {
					productInventoryAmount = productInventoryAmount.Add(invoiceDetail.DetailTotalAmount.Add(invoiceDetail.DetailDiscountAmount))
				}
				productInventoryAccounts[productDetail.InventoryAccountId] = productInventoryAmount

				stockHistory := models.StockHistory{
					BusinessId:        invoice.BusinessId,
					WarehouseId:       invoice.WarehouseId,
					ProductId:         invoiceDetail.ProductId,
					ProductType:       invoiceDetail.ProductType,
					BatchNumber:       invoiceDetail.BatchNumber,
					StockDate:         stockDate,
					Qty:               invoiceDetail.DetailQty.Neg(),
					Description:       "Invoice #" + invoice.InvoiceNumber,
					ReferenceType:     models.StockReferenceTypeInvoice,
					ReferenceID:       invoice.ID,
					ReferenceDetailID: invoiceDetail.ID,
					IsOutgoing:        utils.NewTrue(),
				}
				err = tx.Exec("UPDATE sales_invoice_details SET cogs = 0 WHERE id = ?",
					stockHistory.ReferenceDetailID).Error
				if err != nil {
					tx.Rollback()
					config.LogError(logger, "InvoiceWorkflow.go", "CreateInvoice", "ResetDetailCogs", stockHistory.ReferenceDetailID, err)
					return 0, nil, 0, nil, err
				}

				err = tx.Create(&stockHistory).Error
				if err != nil {
					tx.Rollback()
					config.LogError(logger, "InvoiceWorkflow.go", "CreateInvoice", "CreateStockHistory", stockHistory, err)
					return 0, nil, 0, nil, err
				}
				stockHistories = append(stockHistories, &stockHistory)
			}
		}
	}

	for accId, accAmount := range detailAccounts {
		accountIds = append(accountIds, accId)
		if baseCurrencyId != foreignCurrencyId {
			accTransactions = append(accTransactions, models.AccountTransaction{
				BusinessId:          businessId,
				AccountId:           accId,
				BranchId:            branchId,
				TransactionDateTime: transactionTime,
				BaseCurrencyId:      baseCurrencyId,
				BaseCredit:          accAmount.Mul(exchangeRate),
				BaseDebit:           decimal.NewFromInt(0),
				ForeignCurrencyId:   foreignCurrencyId,
				ForeignCredit:       accAmount,
				ForeignDebit:        decimal.NewFromInt(0),
				ExchangeRate:        exchangeRate,
			})
		} else {
			accTransactions = append(accTransactions, models.AccountTransaction{
				BusinessId:          businessId,
				AccountId:           accId,
				BranchId:            branchId,
				TransactionDateTime: transactionTime,
				BaseCurrencyId:      baseCurrencyId,
				BaseCredit:          accAmount,
				BaseDebit:           decimal.NewFromInt(0),
			})
		}
	}

	for purchaseAccId := range productPurchaseAccounts {
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

	for inventoryAccId := range productInventoryAccounts {
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
		TransactionNumber:   invoice.InvoiceNumber,
		CustomerId:          invoice.CustomerId,
		ReferenceId:         invoice.ID,
		ReferenceType:       models.AccountReferenceTypeInvoice,
		AccountTransactions: accTransactions,
	}

	err = tx.Create(&accJournal).Error
	if err != nil {
		config.LogError(logger, "InvoiceWorkflow.go", "CreateInvoice", "CreateAccountJournal", accJournal, err)
		tx.Rollback()
		return 0, nil, 0, nil, err
	}

	return accJournal.ID, accountIds, foreignCurrencyId, stockHistories, nil
}

func DeleteInvoice(tx *gorm.DB, logger *logrus.Logger, recordId int, recordType string, businessId string, business models.Business, oldInvoice models.SalesInvoice) (int, []int, int, []*models.StockHistory, error) {

	foreignCurrencyId := oldInvoice.CurrencyId
	accountJournal, _, accountIds, err := GetExistingAccountJournal(tx, oldInvoice.ID, models.AccountReferenceTypeInvoice)
	if err != nil {
		config.LogError(logger, "InvoiceWorkflow.go", "DeleteInvoice", "GetExistingAccountJournal", oldInvoice, err)
		return 0, nil, 0, nil, err
	}

	var stockHistories []*models.StockHistory
	err = tx.Where("reference_id = ? AND reference_type = ? AND is_reversal = 0 AND reversed_by_stock_history_id IS NULL", oldInvoice.ID, models.StockReferenceTypeInvoice).Find(&stockHistories).Error
	if err != nil {
		config.LogError(logger, "InvoiceWorkflow.go", "DeleteInvoice", "FindStockHistories", oldInvoice, err)
		return 0, nil, 0, nil, err
	}
	// Inventory ledger immutability: do not delete stock history; append reversal entries instead.
	stockReversals, err := ReverseStockHistories(tx, stockHistories, ReversalReasonSalesInvoiceVoidUpdate)
	if err != nil {
		config.LogError(logger, "InvoiceWorkflow.go", "DeleteInvoice", "ReverseStockHistories", oldInvoice, err)
		return 0, nil, 0, nil, err
	}

	// stockFragments := make([]*StockFragment, 0)
	// for _, invoiceDetail := range oldInvoice.Details {
	// 	if invoiceDetail.ProductId > 0 {
	// 		productDetail, err := GetProductDetail(tx, invoiceDetail.ProductId, invoiceDetail.ProductType)
	// 		if err != nil {
	// 			config.LogError(logger, "InvoiceWorkflow.go", "DeleteInvoice", "GetProductDetail", invoiceDetail, err)
	// 			return 0, nil, 0, nil, err
	// 		}

	// 		if productDetail.InventoryAccountId > 0 {
	// 			stockFragments = append(stockFragments, &StockFragment{
	// 				WarehouseId:  oldInvoice.WarehouseId,
	// 				ProductId:    invoiceDetail.ProductId,
	// 				ProductType:  invoiceDetail.ProductType,
	// 				BatchNumber:  invoiceDetail.BatchNumber,
	// 				ReceivedDate: oldInvoice.InvoiceDate,
	// 			})
	// 		}
	// 	}
	// }

	// Phase 1: do not delete posted journals; create a reversal journal instead.
	reversalID, err := ReverseAccountJournal(tx, accountJournal, ReversalReasonSalesInvoiceVoidUpdate)
	if err != nil {
		config.LogError(logger, "InvoiceWorkflow.go", "DeleteInvoice", "ReverseAccountJournal", accountJournal, err)
		return 0, nil, 0, nil, err
	}

	return reversalID, accountIds, foreignCurrencyId, stockReversals, nil
}
