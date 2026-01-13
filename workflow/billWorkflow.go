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

func ProcessBillWorkflow(tx *gorm.DB, logger *logrus.Logger, msg config.PubSubMessage) error {

	var accountJournalId int
	var accountIds []int
	var foreignCurrencyId int
	var stockHistories []*models.StockHistory
	business, err := models.GetBusinessById2(tx, msg.BusinessId)
	if err != nil {
		config.LogError(logger, "BillWorkflow.go", "ProcessBillWorkflow", "GetBusiness", msg.BusinessId, err)
		return err
	}
	if msg.Action == string(models.PubSubMessageActionCreate) {

		var bill models.Bill
		err := json.Unmarshal([]byte(msg.NewObj), &bill)
		if err != nil {
			config.LogError(logger, "BillWorkflow.go", "ProcessBillWorkflow > Create", "Unmarshal msg.NewObj", msg.NewObj, err)
			return err
		}
		accountJournalId, accountIds, foreignCurrencyId, stockHistories, err = CreateBill(tx, logger, msg.ID, msg.ReferenceType, msg.BusinessId, *business, bill)
		if err != nil {
			config.LogError(logger, "BillWorkflow.go", "ProcessBillWorkflow > Create", "CreateBankingTransaction", nil, err)
			return err
		}
		valuationAccountIds, err := ProcessIncomingStocks(tx, logger, stockHistories)
		if err != nil {
			config.LogError(logger, "BillWorkflow.go", "ProcessBillWorkflow > Create", "ProcessIncomingStocks", stockHistories, err)
			return err
		}

		if len(valuationAccountIds) > 0 {
			for _, accId := range valuationAccountIds {
				if !slices.Contains(accountIds, accId) {
					accountIds = append(accountIds, accId)
				}
			}
		}
		err = UpdateBalances(tx, logger, msg.BusinessId, business.BaseCurrencyId, bill.BranchId, accountIds, bill.BillDate, foreignCurrencyId)
		if err != nil {
			config.LogError(logger, "BillWorkflow.go", "ProcessBillWorkflow > Create", "UpdateBalances", bill, err)
			return err
		}

	} else if msg.Action == string(models.PubSubMessageActionUpdate) {

		var oldForeignCurrencyId int
		var oldAccountIds []int
		var bill models.Bill
		var oldStockHistories []*models.StockHistory
		err := json.Unmarshal([]byte(msg.NewObj), &bill)
		if err != nil {
			config.LogError(logger, "BillWorkflow.go", "ProcessBillWorkflow > Update", "Unmarshal NewObj", msg.NewObj, err)
			return err
		}
		var oldBill models.Bill
		err = json.Unmarshal([]byte(msg.OldObj), &oldBill)
		if err != nil {
			config.LogError(logger, "BillWorkflow.go", "ProcessBillWorkflow > Update", "Unmarshal OldObj", msg.OldObj, err)
			return err
		}
		_, oldAccountIds, oldForeignCurrencyId, oldStockHistories, err = DeleteBill(tx, logger, msg.ID, msg.ReferenceType, msg.BusinessId, *business, oldBill)
		if err != nil {
			config.LogError(logger, "BillWorkflow.go", "ProcessBillWorkflow > Update", "DeleteBill", nil, err)
			return err
		}
		accountJournalId, accountIds, foreignCurrencyId, stockHistories, err = CreateBill(tx, logger, msg.ID, msg.ReferenceType, msg.BusinessId, *business, bill)
		if err != nil {
			config.LogError(logger, "BillWorkflow.go", "ProcessBillWorkflow > Update", "CreateBill", nil, err)
			return err
		}
		mergedStockHistories := mergeStockHistories(stockHistories, oldStockHistories)
		valuationAccountIds, err := ProcessStockHistories(tx, logger, mergedStockHistories)
		if err != nil {
			config.LogError(logger, "BillWorkflow.go", "ProcessBillWorkflow > Update", "ProcessIncomingStocks", ProcessIncomingStocks, err)
			return err
		}

		if len(valuationAccountIds) > 0 {
			for _, accId := range valuationAccountIds {
				if !slices.Contains(accountIds, accId) {
					accountIds = append(accountIds, accId)
				}
			}
		}
		if oldBill.BranchId != bill.BranchId || oldBill.BillDate != bill.BillDate || oldForeignCurrencyId != foreignCurrencyId {
			err = UpdateBalances(tx, logger, msg.BusinessId, business.BaseCurrencyId, oldBill.BranchId, oldAccountIds, oldBill.BillDate, oldForeignCurrencyId)
			if err != nil {
				config.LogError(logger, "BillWorkflow.go", "ProcessBillWorkflow > Update", "UpdateBalances Old", oldBill, err)
				return err
			}
		} else {
			accountIds = utils.MergeIntSlices(accountIds, oldAccountIds)
		}
		err = UpdateBalances(tx, logger, msg.BusinessId, business.BaseCurrencyId, bill.BranchId, accountIds, bill.BillDate, foreignCurrencyId)
		if err != nil {
			config.LogError(logger, "BillWorkflow.go", "ProcessBillWorkflow > Update", "UpdateBalances", bill, err)
			return err
		}
	} else if msg.Action == string(models.PubSubMessageActionDelete) {

		var oldBill models.Bill
		err = json.Unmarshal([]byte(msg.OldObj), &oldBill)
		if err != nil {
			config.LogError(logger, "BillWorkflow.go", "ProcessBillWorkflow > Delete", "Unmarshal OldObj", msg.OldObj, err)
			return err
		}
		accountJournalId, accountIds, foreignCurrencyId, stockHistories, err = DeleteBill(tx, logger, msg.ID, msg.ReferenceType, msg.BusinessId, *business, oldBill)
		if err != nil {
			config.LogError(logger, "BillWorkflow.go", "ProcessBillWorkflow > Delete", "DeleteBill", nil, err)
			return err
		}
		valuationAccountIds, err := ProcessStockHistories(tx, logger, stockHistories)
		if err != nil {
			config.LogError(logger, "BillWorkflow.go", "ProcessBillWorkflow > Delete", "ProcessIncomingStocks", stockHistories, err)
			return err
		}

		if len(valuationAccountIds) > 0 {
			for _, accId := range valuationAccountIds {
				if !slices.Contains(accountIds, accId) {
					accountIds = append(accountIds, accId)
				}
			}
		}
		err = UpdateBalances(tx, logger, msg.BusinessId, business.BaseCurrencyId, oldBill.BranchId, accountIds, oldBill.BillDate, foreignCurrencyId)
		if err != nil {
			config.LogError(logger, "BillWorkflow.go", "ProcessBillWorkflow > Delete", "UpdateBalances", oldBill, err)
			return err
		}
	}
	err = tx.Model(&models.PubSubMessageRecord{}).Where("id=?", msg.ID).Updates(map[string]interface{}{"account_journal_id": accountJournalId, "is_processed": true}).Error
	if err != nil {
		config.LogError(logger, "BillWorkflow.go", "ProcessBillWorkflow", "UpdatePubSubMessageRecord", accountJournalId, err)
		return err
	}
	return nil
}

func CreateBill(tx *gorm.DB, logger *logrus.Logger, recordId int, recordType string, businessId string, business models.Business, bill models.Bill) (int, []int, int, []*models.StockHistory, error) {

	systemAccounts, err := models.GetSystemAccounts(businessId)
	if err != nil {
		config.LogError(logger, "BillWorkflow.go", "CreateBill", "GetSystemAccounts", businessId, err)
		return 0, nil, 0, nil, err
	}

	exchangeRate := bill.ExchangeRate
	transactionTime := bill.BillDate
	baseCurrencyId := business.BaseCurrencyId
	foreignCurrencyId := bill.CurrencyId
	branchId := bill.BranchId
	baseDiscountAmount := bill.BillTotalDiscountAmount
	baseTaxAmount := bill.BillTotalTaxAmount
	basePayableAmount := bill.BillTotalAmount
	baseAdjustmentAmount := bill.AdjustmentAmount
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
		BaseDebit:           decimal.NewFromInt(0),
		BaseCredit:          basePayableAmount,
		ForeignCurrencyId:   foreignCurrencyId,
		ForeignDebit:        decimal.NewFromInt(0),
		ForeignCredit:       foreignPayableAmount,
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
			BaseDebit:           decimal.NewFromInt(0),
			BaseCredit:          baseDiscountAmount,
			ForeignCurrencyId:   foreignCurrencyId,
			ForeignDebit:        decimal.NewFromInt(0),
			ForeignCredit:       foreignDiscountAmount,
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
			BaseDebit:           baseTaxAmount,
			BaseCredit:          decimal.NewFromInt(0),
			ForeignCurrencyId:   foreignCurrencyId,
			ForeignDebit:        foreignTaxAmount,
			ForeignCredit:       decimal.NewFromInt(0),
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
				BaseDebit:           decimal.NewFromInt(0),
				BaseCredit:          baseAdjustmentAmount.Abs(),
				ForeignCurrencyId:   foreignCurrencyId,
				ForeignDebit:        decimal.NewFromInt(0),
				ForeignCredit:       foreignAdjustmentAmount,
				ExchangeRate:        exchangeRate,
			}
		} else {
			otherExpenses = models.AccountTransaction{
				BusinessId:          businessId,
				AccountId:           systemAccounts[models.AccountCodeOtherExpenses],
				BranchId:            branchId,
				TransactionDateTime: transactionTime,
				BaseCurrencyId:      baseCurrencyId,
				BaseDebit:           baseAdjustmentAmount.Abs(),
				BaseCredit:          decimal.NewFromInt(0),
				ForeignCurrencyId:   foreignCurrencyId,
				ForeignDebit:        foreignAdjustmentAmount,
				ForeignCredit:       decimal.NewFromInt(0),
				ExchangeRate:        exchangeRate,
			}
		}
		accountIds = append(accountIds, systemAccounts[models.AccountCodeOtherExpenses])
		accTransactions = append(accTransactions, otherExpenses)
	}

	detailAccounts := make(map[int]decimal.Decimal)
	stockHistories := make([]*models.StockHistory, 0)
	stockDate, err := utils.ConvertToDate(bill.BillDate, business.Timezone)
	if err != nil {
		return 0, nil, 0, nil, err
	}
	for _, billDetail := range bill.Details {
		amount, ok := detailAccounts[billDetail.DetailAccountId]
		if !ok {
			amount = decimal.NewFromInt(0)
		}
		if bill.IsTaxInclusive != nil && *bill.IsTaxInclusive {
			amount = amount.Add(billDetail.DetailTotalAmount.Add(billDetail.DetailDiscountAmount).Sub(billDetail.DetailTaxAmount))
		} else {
			amount = amount.Add(billDetail.DetailTotalAmount.Add(billDetail.DetailDiscountAmount))
		}
		detailAccounts[billDetail.DetailAccountId] = amount

		if billDetail.ProductId > 0 &&
			CheckIfStockNeedsInventoryTracking(tx, billDetail.ProductId, billDetail.ProductType) {
			stockHistory := models.StockHistory{
				BusinessId:        bill.BusinessId,
				WarehouseId:       bill.WarehouseId,
				ProductId:         billDetail.ProductId,
				ProductType:       billDetail.ProductType,
				BatchNumber:       billDetail.BatchNumber,
				StockDate:         stockDate,
				Qty:               billDetail.DetailQty,
				Description:       "Bill #" + bill.BillNumber,
				ReferenceType:     models.StockReferenceTypeBill,
				ReferenceID:       bill.ID,
				ReferenceDetailID: billDetail.ID,
				IsOutgoing:        utils.NewFalse(),
			}
			if baseCurrencyId != foreignCurrencyId {
				stockHistory.BaseUnitValue = billDetail.DetailUnitRate.Mul(exchangeRate)
			} else {
				stockHistory.BaseUnitValue = billDetail.DetailUnitRate
			}
			err = tx.Create(&stockHistory).Error
			if err != nil {
				tx.Rollback()
				config.LogError(logger, "BillWorkflow.go", "CreateBill", "CreateStockHistory", stockHistory, err)
				return 0, nil, 0, nil, err
			}
			stockHistories = append(stockHistories, &stockHistory)
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
				BaseDebit:           accAmount.Mul(exchangeRate),
				BaseCredit:          decimal.NewFromInt(0),
				ForeignCurrencyId:   foreignCurrencyId,
				ForeignDebit:        accAmount,
				ForeignCredit:       decimal.NewFromInt(0),
				ExchangeRate:        exchangeRate,
			})
		} else {
			accTransactions = append(accTransactions, models.AccountTransaction{
				BusinessId:          businessId,
				AccountId:           accId,
				BranchId:            branchId,
				TransactionDateTime: transactionTime,
				BaseCurrencyId:      baseCurrencyId,
				BaseDebit:           accAmount,
				BaseCredit:          decimal.NewFromInt(0),
			})
		}
	}

	accJournal := models.AccountJournal{
		BusinessId:          businessId,
		BranchId:            branchId,
		TransactionDateTime: transactionTime,
		TransactionNumber:   bill.BillNumber,
		SupplierId:          bill.SupplierId,
		ReferenceId:         bill.ID,
		ReferenceType:       models.AccountReferenceTypeBill,
		AccountTransactions: accTransactions,
	}

	err = tx.Create(&accJournal).Error
	if err != nil {
		tx.Rollback()
		config.LogError(logger, "BillWorkflow.go", "CreateBill", "CreateAccountJournal", accJournal, err)
		return 0, nil, 0, nil, err
	}

	return accJournal.ID, accountIds, foreignCurrencyId, stockHistories, nil
}

func DeleteBill(tx *gorm.DB, logger *logrus.Logger, recordId int, recordType string, businessId string, business models.Business, oldBill models.Bill) (int, []int, int, []*models.StockHistory, error) {

	foreignCurrencyId := oldBill.CurrencyId
	accountJournal, _, accountIds, err := GetExistingAccountJournal(tx, oldBill.ID, models.AccountReferenceTypeBill)
	if err != nil {
		config.LogError(logger, "BillWorkflow.go", "DeleteBill", "GetExistingAccountJournal", oldBill, err)
		return 0, nil, 0, nil, err
	}

	var stockHistories []*models.StockHistory
	err = tx.Where("reference_id = ? AND reference_type = ? AND is_reversal = 0 AND reversed_by_stock_history_id IS NULL", oldBill.ID, models.StockReferenceTypeBill).Find(&stockHistories).Error
	if err != nil {
		config.LogError(logger, "BillWorkflow.go", "DeleteBill", "FindStockHistories", oldBill, err)
		return 0, nil, 0, nil, err
	}
	// Inventory ledger immutability: do not delete stock history; append reversal entries instead.
	stockReversals, err := ReverseStockHistories(tx, stockHistories, ReversalReasonBillVoidUpdate)
	if err != nil {
		config.LogError(logger, "BillWorkflow.go", "DeleteBill", "ReverseStockHistories", oldBill, err)
		return 0, nil, 0, nil, err
	}

	// Phase 1: do not delete posted journals; create a reversal journal instead.
	reversalID, err := ReverseAccountJournal(tx, accountJournal, ReversalReasonBillVoidUpdate)
	if err != nil {
		config.LogError(logger, "BillWorkflow.go", "DeleteBill", "ReverseAccountJournal", accountJournal, err)
		return 0, nil, 0, nil, err
	}

	return reversalID, accountIds, foreignCurrencyId, stockReversals, nil
}
