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

func ProcessCreditNoteWorkflow(tx *gorm.DB, logger *logrus.Logger, msg config.PubSubMessage) error {

	var accountJournalId int
	var accountIds []int
	var foreignCurrencyId int
	var stockHistories []*models.StockHistory
	business, err := models.GetBusinessById2(tx, msg.BusinessId)
	if err != nil {
		config.LogError(logger, "CreditNoteWorkflow.go", "ProcessCreditNoteWorkflow", "GetBusiness", msg.BusinessId, err)
		return err
	}
	if msg.Action == string(models.PubSubMessageActionCreate) {

		var creditNote models.CreditNote
		err := json.Unmarshal([]byte(msg.NewObj), &creditNote)
		if err != nil {
			config.LogError(logger, "CreditNoteWorkflow.go", "ProcessCreditNoteWorkflow > Create", "Unmarshal msg.NewObj", msg.NewObj, err)
			return err
		}
		accountJournalId, accountIds, foreignCurrencyId, stockHistories, err = CreateCreditNote(tx, logger, msg.ID, msg.ReferenceType, msg.BusinessId, *business, creditNote)
		if err != nil {
			config.LogError(logger, "CreditNoteWorkflow.go", "ProcessCreditNoteWorkflow > Create", "CreateCreditNote", nil, err)
			return err
		}
		valuationAccountIds, err := ProcessIncomingStocks(tx, logger, stockHistories)
		if err != nil {
			config.LogError(logger, "CreditNoteWorkflow.go", "ProcessCreditNoteWorkflow > Create", "ProcessIncomingStocks", stockHistories, err)
			return err
		}

		if len(valuationAccountIds) > 0 {
			for _, accId := range valuationAccountIds {
				if !slices.Contains(accountIds, accId) {
					accountIds = append(accountIds, accId)
				}
			}
		}
		err = UpdateBalances(tx, logger, msg.BusinessId, business.BaseCurrencyId, creditNote.BranchId, accountIds, creditNote.CreditNoteDate, foreignCurrencyId)
		if err != nil {
			config.LogError(logger, "CreditNoteWorkflow.go", "ProcessCreditNoteWorkflow > Create", "UpdateBalances", nil, err)
			return err
		}

	} else if msg.Action == string(models.PubSubMessageActionUpdate) {

		var oldForeignCurrencyId int
		var oldAccountIds []int
		var creditNote models.CreditNote
		var oldStockHistories []*models.StockHistory
		err := json.Unmarshal([]byte(msg.NewObj), &creditNote)
		if err != nil {
			config.LogError(logger, "CreditNoteWorkflow.go", "ProcessCreditNoteWorkflow > Update", "Unmarshal NewObj", msg.NewObj, err)
			return err
		}
		var oldCreditNote models.CreditNote
		err = json.Unmarshal([]byte(msg.OldObj), &oldCreditNote)
		if err != nil {
			config.LogError(logger, "CreditNoteWorkflow.go", "ProcessCreditNoteWorkflow > Update", "Unmarshal OldObj", msg.OldObj, err)
			return err
		}
		_, oldAccountIds, oldForeignCurrencyId, oldStockHistories, err = DeleteCreditNote(tx, logger, msg.ID, msg.ReferenceType, msg.BusinessId, *business, oldCreditNote)
		if err != nil {
			config.LogError(logger, "CreditNoteWorkflow.go", "ProcessCreditNoteWorkflow > Update", "DeleteCreditNote", nil, err)
			return err
		}
		accountJournalId, accountIds, foreignCurrencyId, stockHistories, err = CreateCreditNote(tx, logger, msg.ID, msg.ReferenceType, msg.BusinessId, *business, creditNote)
		if err != nil {
			config.LogError(logger, "CreditNoteWorkflow.go", "ProcessCreditNoteWorkflow > Update", "CreateCreditNote", nil, err)
			return err
		}
		mergedStockHistories := mergeStockHistories(stockHistories, oldStockHistories)
		valuationAccountIds, err := ProcessStockHistories(tx, logger, mergedStockHistories)
		if err != nil {
			config.LogError(logger, "CreditNoteWorkflow.go", "ProcessCreditNoteWorkflow > Update", "ProcessIncomingStocks", mergedStockHistories, err)
			return err
		}

		if len(valuationAccountIds) > 0 {
			for _, accId := range valuationAccountIds {
				if !slices.Contains(accountIds, accId) {
					accountIds = append(accountIds, accId)
				}
			}
		}
		if oldCreditNote.BranchId != creditNote.BranchId || oldCreditNote.CreditNoteDate != creditNote.CreditNoteDate || oldForeignCurrencyId != foreignCurrencyId {
			err = UpdateBalances(tx, logger, msg.BusinessId, business.BaseCurrencyId, oldCreditNote.BranchId, oldAccountIds, oldCreditNote.CreditNoteDate, oldForeignCurrencyId)
			if err != nil {
				config.LogError(logger, "CreditNoteWorkflow.go", "ProcessCreditNoteWorkflow > Update", "UpdateBalances Old", oldCreditNote, err)
				return err
			}
		} else {
			accountIds = utils.MergeIntSlices(accountIds, oldAccountIds)
		}
		err = UpdateBalances(tx, logger, msg.BusinessId, business.BaseCurrencyId, creditNote.BranchId, accountIds, creditNote.CreditNoteDate, foreignCurrencyId)
		if err != nil {
			config.LogError(logger, "CreditNoteWorkflow.go", "ProcessCreditNoteWorkflow > Update", "UpdateBalances", creditNote, err)
			return err
		}
	} else if msg.Action == string(models.PubSubMessageActionDelete) {

		var oldCreditNote models.CreditNote
		err = json.Unmarshal([]byte(msg.OldObj), &oldCreditNote)
		if err != nil {
			config.LogError(logger, "CreditNoteWorkflow.go", "ProcessCreditNoteWorkflow > Delete", "Unmarshal OldObj", msg.OldObj, err)
			return err
		}
		accountJournalId, accountIds, foreignCurrencyId, stockHistories, err = DeleteCreditNote(tx, logger, msg.ID, msg.ReferenceType, msg.BusinessId, *business, oldCreditNote)
		if err != nil {
			config.LogError(logger, "CreditNoteWorkflow.go", "ProcessCreditNoteWorkflow > Delete", "DeleteCreditNote", nil, err)
			return err
		}
		valuationAccountIds, err := ProcessStockHistories(tx, logger, stockHistories)
		if err != nil {
			config.LogError(logger, "CreditNoteWorkflow.go", "ProcessCreditNoteWorkflow > Delete", "ProcessIncomingStocks", stockHistories, err)
			return err
		}

		if len(valuationAccountIds) > 0 {
			for _, accId := range valuationAccountIds {
				if !slices.Contains(accountIds, accId) {
					accountIds = append(accountIds, accId)
				}
			}
		}
		err = UpdateBalances(tx, logger, msg.BusinessId, business.BaseCurrencyId, oldCreditNote.BranchId, accountIds, oldCreditNote.CreditNoteDate, foreignCurrencyId)
		if err != nil {
			config.LogError(logger, "CreditNoteWorkflow.go", "ProcessCreditNoteWorkflow > Delete", "UpdateBalances", oldCreditNote, err)
			return err
		}
	}
	err = tx.Model(&models.PubSubMessageRecord{}).Where("id=?", msg.ID).Updates(map[string]interface{}{"account_journal_id": accountJournalId, "is_processed": true}).Error
	if err != nil {
		config.LogError(logger, "CreditNoteWorkflow.go", "ProcessCreditNoteWorkflow", "UpdatePubSubMessageRecord", accountJournalId, err)
		return err
	}
	return nil
}

func CreateCreditNote(tx *gorm.DB, logger *logrus.Logger, recordId int, recordType string, businessId string, business models.Business, creditNote models.CreditNote) (int, []int, int, []*models.StockHistory, error) {

	systemAccounts, err := models.GetSystemAccounts(businessId)
	if err != nil {
		config.LogError(logger, "CreditNoteWorkflow.go", "CreateCreditNote", "GetSystemAccounts", businessId, err)
		return 0, nil, 0, nil, err
	}

	exchangeRate := creditNote.ExchangeRate
	transactionTime := creditNote.CreditNoteDate
	baseCurrencyId := business.BaseCurrencyId
	foreignCurrencyId := creditNote.CurrencyId
	branchId := creditNote.BranchId
	baseDiscountAmount := creditNote.CreditNoteTotalDiscountAmount
	baseTaxAmount := creditNote.CreditNoteTotalTaxAmount
	baseReceivableAmount := creditNote.CreditNoteTotalAmount
	baseAdjustmentAmount := creditNote.AdjustmentAmount
	foreignDiscountAmount := decimal.NewFromInt(0)
	foreignTaxAmount := decimal.NewFromInt(0)
	foreignReceivableAmount := decimal.NewFromInt(0)
	foreignAdjustmentAmount := decimal.NewFromInt(0)

	if baseCurrencyId != foreignCurrencyId {
		foreignDiscountAmount = baseDiscountAmount
		baseDiscountAmount = foreignDiscountAmount.Mul(exchangeRate)
		foreignTaxAmount = baseTaxAmount
		baseTaxAmount = foreignTaxAmount.Mul(exchangeRate)
		foreignReceivableAmount = baseReceivableAmount
		baseReceivableAmount = foreignReceivableAmount.Mul(exchangeRate)
		baseAdjustmentAmount = foreignAdjustmentAmount.Mul(exchangeRate)
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
		BaseCredit:          baseReceivableAmount,
		BaseDebit:           decimal.NewFromInt(0),
		ForeignCurrencyId:   foreignCurrencyId,
		ForeignCredit:       foreignReceivableAmount,
		ForeignDebit:        decimal.NewFromInt(0),
		ExchangeRate:        exchangeRate,
	}
	accTransactions = append(accTransactions, accountsReceivable)

	if !baseDiscountAmount.IsZero() {
		salesDiscount := models.AccountTransaction{
			BusinessId:          businessId,
			AccountId:           systemAccounts[models.AccountCodeDiscount],
			BranchId:            branchId,
			TransactionDateTime: transactionTime,
			BaseCurrencyId:      baseCurrencyId,
			BaseCredit:          baseDiscountAmount,
			BaseDebit:           decimal.NewFromInt(0),
			ForeignCurrencyId:   foreignCurrencyId,
			ForeignCredit:       foreignDiscountAmount,
			ForeignDebit:        decimal.NewFromInt(0),
			ExchangeRate:        exchangeRate,
		}
		accountIds = append(accountIds, systemAccounts[models.AccountCodeDiscount])
		accTransactions = append(accTransactions, salesDiscount)
	}

	if !baseTaxAmount.IsZero() {
		taxPayable := models.AccountTransaction{
			BusinessId:          businessId,
			AccountId:           systemAccounts[models.AccountCodeTaxPayable],
			BranchId:            branchId,
			TransactionDateTime: transactionTime,
			BaseCurrencyId:      baseCurrencyId,
			BaseCredit:          decimal.NewFromInt(0),
			BaseDebit:           baseTaxAmount,
			ForeignCurrencyId:   foreignCurrencyId,
			ForeignCredit:       decimal.NewFromInt(0),
			ForeignDebit:        foreignTaxAmount,
			ExchangeRate:        exchangeRate,
		}
		accountIds = append(accountIds, systemAccounts[models.AccountCodeTaxPayable])
		accTransactions = append(accTransactions, taxPayable)
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
				BaseCredit:          baseAdjustmentAmount,
				BaseDebit:           decimal.NewFromInt(0),
				ForeignCurrencyId:   foreignCurrencyId,
				ForeignCredit:       foreignAdjustmentAmount,
				ForeignDebit:        decimal.NewFromInt(0),
				ExchangeRate:        exchangeRate,
			}
		} else {
			otherCharges = models.AccountTransaction{
				BusinessId:          businessId,
				AccountId:           systemAccounts[models.AccountCodeOtherCharges],
				BranchId:            branchId,
				TransactionDateTime: transactionTime,
				BaseCurrencyId:      baseCurrencyId,
				BaseCredit:          decimal.NewFromInt(0),
				BaseDebit:           baseAdjustmentAmount,
				ForeignCurrencyId:   foreignCurrencyId,
				ForeignCredit:       decimal.NewFromInt(0),
				ForeignDebit:        foreignAdjustmentAmount,
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
	stockDate, err := utils.ConvertToDate(creditNote.CreditNoteDate, business.Timezone)
	if err != nil {
		return 0, nil, 0, nil, err
	}

	for _, creditNoteDetail := range creditNote.Details {
		detailAccountId := creditNoteDetail.DetailAccountId
		if detailAccountId == 0 {
			detailAccountId = systemAccounts[models.AccountCodeSales]
		}
		amount, ok := detailAccounts[detailAccountId]
		if !ok {
			amount = decimal.NewFromInt(0)
		}
		if creditNote.IsTaxInclusive != nil && *creditNote.IsTaxInclusive {
			amount = amount.Add(creditNoteDetail.DetailTotalAmount.Add(creditNoteDetail.DetailDiscountAmount).Sub(creditNoteDetail.DetailTaxAmount))
		} else {
			amount = amount.Add(creditNoteDetail.DetailTotalAmount.Add(creditNoteDetail.DetailDiscountAmount))
		}
		detailAccounts[detailAccountId] = amount

		if creditNoteDetail.ProductId > 0 {
			// Non-tracked items (no inventory account) should still generate a journal entry,
			// but must NOT generate stock histories / inventory valuation lines.
			// Otherwise we risk creating AccountTransactions with account_id=0 and failing journal creation.
			if !CheckIfStockNeedsInventoryTracking(tx, creditNoteDetail.ProductId, creditNoteDetail.ProductType) {
				continue
			}

			productDetail, err := GetProductDetail(tx, creditNoteDetail.ProductId, creditNoteDetail.ProductType)
			if err != nil {
				config.LogError(logger, "CreditNoteWorkflow.go", "CreateCreditNote", "GetProductDetail", creditNoteDetail, err)
				return 0, nil, 0, nil, err
			}

			// Only track valuation accounts when configured on the product.
			if productDetail.PurchaseAccountId > 0 {
				productPurchaseAmount, ok := productPurchaseAccounts[productDetail.PurchaseAccountId]
				if !ok {
					productPurchaseAmount = decimal.NewFromInt(0)
				}
				if creditNote.IsTaxInclusive != nil && *creditNote.IsTaxInclusive {
					productPurchaseAmount = productPurchaseAmount.Add(creditNoteDetail.DetailTotalAmount.Add(creditNoteDetail.DetailDiscountAmount).Sub(creditNoteDetail.DetailTaxAmount))
				} else {
					productPurchaseAmount = productPurchaseAmount.Add(creditNoteDetail.DetailTotalAmount.Add(creditNoteDetail.DetailDiscountAmount))
				}
				productPurchaseAccounts[productDetail.PurchaseAccountId] = productPurchaseAmount
			}
			if productDetail.InventoryAccountId > 0 {
				productInventoryAmount, ok := productInventoryAccounts[productDetail.InventoryAccountId]
				if !ok {
					productInventoryAmount = decimal.NewFromInt(0)
				}
				if creditNote.IsTaxInclusive != nil && *creditNote.IsTaxInclusive {
					productInventoryAmount = productInventoryAmount.Add(creditNoteDetail.DetailTotalAmount.Add(creditNoteDetail.DetailDiscountAmount).Sub(creditNoteDetail.DetailTaxAmount))
				} else {
					productInventoryAmount = productInventoryAmount.Add(creditNoteDetail.DetailTotalAmount.Add(creditNoteDetail.DetailDiscountAmount))
				}
				productInventoryAccounts[productDetail.InventoryAccountId] = productInventoryAmount
			}

			stockHistory := models.StockHistory{
				BusinessId:        creditNote.BusinessId,
				WarehouseId:       creditNote.WarehouseId,
				ProductId:         creditNoteDetail.ProductId,
				ProductType:       creditNoteDetail.ProductType,
				BatchNumber:       creditNoteDetail.BatchNumber,
				StockDate:         stockDate,
				Qty:               creditNoteDetail.DetailQty,
				Description:       "CreditNote #" + creditNote.CreditNoteNumber,
				ReferenceType:     models.StockReferenceTypeCreditNote,
				ReferenceID:       creditNote.ID,
				ReferenceDetailID: creditNoteDetail.ID,
				IsOutgoing:        utils.NewFalse(),
			}
			if baseCurrencyId != foreignCurrencyId {
				stockHistory.BaseUnitValue = creditNoteDetail.DetailUnitRate.Mul(exchangeRate)
			} else {
				stockHistory.BaseUnitValue = creditNoteDetail.DetailUnitRate
			}
			err = tx.Exec("UPDATE credit_note_details SET cogs = 0 WHERE id = ?",
				stockHistory.ReferenceDetailID).Error
			if err != nil {
				tx.Rollback()
				config.LogError(logger, "CreditNoteWorkflow.go", "CreateCreditNote", "ResetDetailCogs", stockHistory.ReferenceDetailID, err)
				return 0, nil, 0, nil, err
			}
			err = tx.Create(&stockHistory).Error
			if err != nil {
				tx.Rollback()
				config.LogError(logger, "CreditNoteWorkflow.go", "CreateCreditNote", "CreateStockHistory", stockHistory, err)
				return 0, nil, 0, nil, err
			}
			stockHistories = append(stockHistories, &stockHistory)
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
				BaseCredit:          decimal.NewFromInt(0),
				BaseDebit:           accAmount.Mul(exchangeRate),
				ForeignCurrencyId:   foreignCurrencyId,
				ForeignCredit:       decimal.NewFromInt(0),
				ForeignDebit:        accAmount,
				ExchangeRate:        exchangeRate,
			})
		} else {
			accTransactions = append(accTransactions, models.AccountTransaction{
				BusinessId:          businessId,
				AccountId:           accId,
				BranchId:            branchId,
				TransactionDateTime: transactionTime,
				BaseCurrencyId:      baseCurrencyId,
				BaseCredit:          decimal.NewFromInt(0),
				BaseDebit:           accAmount,
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
		TransactionNumber:   creditNote.CreditNoteNumber,
		CustomerId:          creditNote.CustomerId,
		ReferenceId:         creditNote.ID,
		ReferenceType:       models.AccountReferenceTypeCreditNote,
		AccountTransactions: accTransactions,
	}

	err = tx.Create(&accJournal).Error
	if err != nil {
		tx.Rollback()
		config.LogError(logger, "CreditNoteWorkflow.go", "CreateCreditNote", "CreateAccountJournal", accJournal, err)
		return 0, nil, 0, nil, err
	}

	return accJournal.ID, accountIds, foreignCurrencyId, stockHistories, nil

}

func DeleteCreditNote(tx *gorm.DB, logger *logrus.Logger, recordId int, recordType string, businessId string, business models.Business, oldCreditNote models.CreditNote) (int, []int, int, []*models.StockHistory, error) {

	foreignCurrencyId := oldCreditNote.CurrencyId
	accountJournal, _, accountIds, err := GetExistingAccountJournal(tx, oldCreditNote.ID, models.AccountReferenceTypeCreditNote)
	if err != nil {
		config.LogError(logger, "CreditNoteWorkflow.go", "DeleteCreditNote", "GetExistingAccountJournal", oldCreditNote, err)
		return 0, nil, 0, nil, err
	}

	var stockHistories []*models.StockHistory
	err = tx.Where("reference_id = ? AND reference_type = ? AND is_reversal = 0 AND reversed_by_stock_history_id IS NULL", oldCreditNote.ID, models.StockReferenceTypeCreditNote).Find(&stockHistories).Error
	if err != nil {
		config.LogError(logger, "CreditNoteWorkflow.go", "DeleteCreditNote", "FindStockHistories", oldCreditNote, err)
		return 0, nil, 0, nil, err
	}
	// Inventory ledger immutability: do not delete stock history; append reversal entries instead.
	stockReversals, err := ReverseStockHistories(tx, stockHistories, ReversalReasonCreditNoteVoidUpdate)
	if err != nil {
		config.LogError(logger, "CreditNoteWorkflow.go", "DeleteCreditNote", "ReverseStockHistories", oldCreditNote, err)
		return 0, nil, 0, nil, err
	}

	// Phase 1: do not delete posted journals; create a reversal journal instead.
	reversalID, err := ReverseAccountJournal(tx, accountJournal, ReversalReasonCreditNoteVoidUpdate)
	if err != nil {
		config.LogError(logger, "CreditNoteWorkflow.go", "DeleteCreditNote", "ReverseAccountJournal", accountJournal, err)
		return 0, nil, 0, nil, err
	}
	return reversalID, accountIds, foreignCurrencyId, stockReversals, nil
}
