package workflow

import (
	"encoding/json"
	"strconv"

	"github.com/mmdatafocus/books_backend/config"
	"github.com/mmdatafocus/books_backend/models"
	"github.com/mmdatafocus/books_backend/utils"
	"github.com/shopspring/decimal"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

func ProcessSupplierCreditAppliedWorkflow(tx *gorm.DB, logger *logrus.Logger, msg config.PubSubMessage) error {

	var accountJournalId int
	var accountIds []int
	var foreignCurrencyId int
	business, err := models.GetBusinessById2(tx, msg.BusinessId)
	if err != nil {
		config.LogError(logger, "SupplierCreditAppliedWorkflow.go", "ProcessSupplierCreditAppliedWorkflow", "GetBusiness", msg.BusinessId, err)
		return err
	}
	if msg.Action == string(models.PubSubMessageActionCreate) {

		var supplierCreditBill models.SupplierCreditBill
		err := json.Unmarshal([]byte(msg.NewObj), &supplierCreditBill)
		if err != nil {
			config.LogError(logger, "SupplierCreditAppliedWorkflow.go", "ProcessSupplierCreditAppliedWorkflow > Create", "Unmarshal msg.NewObj", msg.NewObj, err)
			return err
		}
		accountJournalId, accountIds, foreignCurrencyId, err = CreateSupplierCreditApplied(tx, logger, msg.ID, msg.ReferenceType, msg.BusinessId, *business, supplierCreditBill)
		if err != nil {
			config.LogError(logger, "SupplierCreditAppliedWorkflow.go", "ProcessSupplierCreditAppliedWorkflow > Create", "CreateSupplierCreditApplied", nil, err)
			return err
		}
		err = UpdateBalances(tx, logger, msg.BusinessId, business.BaseCurrencyId, supplierCreditBill.BranchId, accountIds, supplierCreditBill.CreditDate, foreignCurrencyId)
		if err != nil {
			config.LogError(logger, "SupplierCreditAppliedWorkflow.go", "ProcessSupplierCreditAppliedWorkflow > Create", "UpdateBalances", supplierCreditBill, err)
			return err
		}

	} else if msg.Action == string(models.PubSubMessageActionUpdate) {

		var oldForeignCurrencyId int
		var oldAccountIds []int
		var supplierCreditBill models.SupplierCreditBill
		err := json.Unmarshal([]byte(msg.NewObj), &supplierCreditBill)
		if err != nil {
			config.LogError(logger, "SupplierCreditAppliedWorkflow.go", "ProcessSupplierCreditAppliedWorkflow > Update", "Unmarshal msg.NewObj", msg.NewObj, err)
			return err
		}
		var oldSupplierCreditBill models.SupplierCreditBill
		err = json.Unmarshal([]byte(msg.OldObj), &oldSupplierCreditBill)
		if err != nil {
			config.LogError(logger, "SupplierCreditAppliedWorkflow.go", "ProcessSupplierCreditAppliedWorkflow > Update", "Unmarshal msg.OldObj", msg.OldObj, err)
			return err
		}
		_, oldAccountIds, oldForeignCurrencyId, err = DeleteSupplierCreditApplied(tx, logger, msg.ID, msg.ReferenceType, msg.BusinessId, *business, oldSupplierCreditBill)
		if err != nil {
			config.LogError(logger, "SupplierCreditAppliedWorkflow.go", "ProcessSupplierCreditAppliedWorkflow > Update", "DeleteSupplierCreditApplied", nil, err)
			return err
		}
		accountJournalId, accountIds, foreignCurrencyId, err = CreateSupplierCreditApplied(tx, logger, msg.ID, msg.ReferenceType, msg.BusinessId, *business, supplierCreditBill)
		if err != nil {
			config.LogError(logger, "SupplierCreditAppliedWorkflow.go", "ProcessSupplierCreditAppliedWorkflow > Update", "CreateSupplierCreditApplied", nil, err)
			return err
		}
		if oldSupplierCreditBill.BranchId != supplierCreditBill.BranchId || oldSupplierCreditBill.CreditDate != supplierCreditBill.CreditDate || oldForeignCurrencyId != foreignCurrencyId {
			err = UpdateBalances(tx, logger, msg.BusinessId, business.BaseCurrencyId, oldSupplierCreditBill.BranchId, oldAccountIds, oldSupplierCreditBill.CreditDate, oldForeignCurrencyId)
			if err != nil {
				config.LogError(logger, "SupplierCreditAppliedWorkflow.go", "ProcessSupplierCreditAppliedWorkflow > Update", "UpdateBalances Old", oldSupplierCreditBill, err)
				return err
			}
		} else {
			accountIds = utils.MergeIntSlices(accountIds, oldAccountIds)
		}
		err = UpdateBalances(tx, logger, msg.BusinessId, business.BaseCurrencyId, supplierCreditBill.BranchId, accountIds, supplierCreditBill.CreditDate, foreignCurrencyId)
		if err != nil {
			config.LogError(logger, "SupplierCreditAppliedWorkflow.go", "ProcessSupplierCreditAppliedWorkflow > Update", "UpdateBalances", supplierCreditBill, err)
			return err
		}
	} else if msg.Action == string(models.PubSubMessageActionDelete) {

		var oldSupplierCreditBill models.SupplierCreditBill
		err = json.Unmarshal([]byte(msg.OldObj), &oldSupplierCreditBill)
		if err != nil {
			config.LogError(logger, "SupplierCreditAppliedWorkflow.go", "ProcessSupplierCreditAppliedWorkflow > Delete", "Unmarshal msg.OldObj", msg.OldObj, err)
			return err
		}
		accountJournalId, accountIds, foreignCurrencyId, err = DeleteSupplierCreditApplied(tx, logger, msg.ID, msg.ReferenceType, msg.BusinessId, *business, oldSupplierCreditBill)
		if err != nil {
			config.LogError(logger, "SupplierCreditAppliedWorkflow.go", "ProcessSupplierCreditAppliedWorkflow > Delete", "DeleteSupplierCreditApplied", nil, err)
			return err
		}
		err = UpdateBalances(tx, logger, msg.BusinessId, business.BaseCurrencyId, oldSupplierCreditBill.BranchId, accountIds, oldSupplierCreditBill.CreditDate, foreignCurrencyId)
		if err != nil {
			config.LogError(logger, "SupplierCreditAppliedWorkflow.go", "ProcessSupplierCreditAppliedWorkflow > Delete", "UpdateBalances", oldSupplierCreditBill, err)
			return err
		}
	}
	err = tx.Model(&models.PubSubMessageRecord{}).Where("id=?", msg.ID).Updates(map[string]interface{}{"account_journal_id": accountJournalId, "is_processed": true}).Error
	if err != nil {
		config.LogError(logger, "SupplierCreditAppliedWorkflow.go", "ProcessSupplierCreditAppliedWorkflow", "UpdatePubSubMessageRecord", accountJournalId, err)
		return err
	}
	return nil
}

func CreateSupplierCreditApplied(tx *gorm.DB, logger *logrus.Logger, recordId int, recordType string, businessId string, business models.Business, supplierCreditBill models.SupplierCreditBill) (int, []int, int, error) {

	systemAccounts, err := models.GetSystemAccounts(businessId)
	if err != nil {
		config.LogError(logger, "SupplierCreditAppliedWorkflow.go", "CreateSupplierCreditApplied", "GetSystemAccounts", businessId, err)
		return 0, nil, 0, err
	}

	transactionTime := supplierCreditBill.CreditDate
	branchId := supplierCreditBill.BranchId
	baseCurrencyId := business.BaseCurrencyId
	foreignCurrencyId := supplierCreditBill.CurrencyId
	exchangeRate := supplierCreditBill.ExchangeRate
	baseBillAmount := supplierCreditBill.Amount.Mul(supplierCreditBill.BillExchangeRate)
	baseAmount := supplierCreditBill.Amount.Mul(supplierCreditBill.ExchangeRate)

	accountIds := make([]int, 0)
	accTransactions := make([]models.AccountTransaction, 0)

	if baseBillAmount.GreaterThanOrEqual(baseAmount) {
		lossAmount := baseBillAmount.Sub(baseAmount)
		accountsReceivable := models.AccountTransaction{
			BusinessId:          businessId,
			AccountId:           systemAccounts[models.AccountCodeAccountsPayable],
			BranchId:            branchId,
			TransactionDateTime: transactionTime,
			BaseCurrencyId:      baseCurrencyId,
			BaseDebit:           lossAmount,
			BaseCredit:          decimal.NewFromInt(0),
			ForeignCurrencyId:   foreignCurrencyId,
			ForeignDebit:        decimal.NewFromInt(0),
			ForeignCredit:       decimal.NewFromInt(0),
			ExchangeRate:        exchangeRate,
		}
		accTransactions = append(accTransactions, accountsReceivable)
		accountIds = append(accountIds, systemAccounts[models.AccountCodeAccountsPayable])

		exchangeGainLoss := models.AccountTransaction{
			BusinessId:          businessId,
			AccountId:           systemAccounts[models.AccountCodeExchangeGainOrLoss],
			BranchId:            branchId,
			TransactionDateTime: transactionTime,
			BaseCurrencyId:      baseCurrencyId,
			BaseCredit:          lossAmount,
			BaseDebit:           decimal.NewFromInt(0),
			ForeignCurrencyId:   foreignCurrencyId,
			ForeignDebit:        decimal.NewFromInt(0),
			ForeignCredit:       decimal.NewFromInt(0),
			ExchangeRate:        exchangeRate,
			RealisedAmount:      baseAmount,
		}
		accountIds = append(accountIds, systemAccounts[models.AccountCodeExchangeGainOrLoss])
		accTransactions = append(accTransactions, exchangeGainLoss)
	} else {
		gainAmount := baseAmount.Sub(baseBillAmount)
		accountsReceivable := models.AccountTransaction{
			BusinessId:          businessId,
			AccountId:           systemAccounts[models.AccountCodeAccountsPayable],
			BranchId:            branchId,
			TransactionDateTime: transactionTime,
			BaseCurrencyId:      baseCurrencyId,
			BaseDebit:           decimal.NewFromInt(0),
			BaseCredit:          gainAmount,
			ForeignCurrencyId:   foreignCurrencyId,
			ForeignDebit:        decimal.NewFromInt(0),
			ForeignCredit:       decimal.NewFromInt(0),
			ExchangeRate:        exchangeRate,
		}
		accTransactions = append(accTransactions, accountsReceivable)
		accountIds = append(accountIds, systemAccounts[models.AccountCodeAccountsPayable])

		exchangeGainLoss := models.AccountTransaction{
			BusinessId:          businessId,
			AccountId:           systemAccounts[models.AccountCodeExchangeGainOrLoss],
			BranchId:            branchId,
			TransactionDateTime: transactionTime,
			BaseCurrencyId:      baseCurrencyId,
			BaseCredit:          decimal.NewFromInt(0),
			BaseDebit:           gainAmount,
			ForeignCurrencyId:   foreignCurrencyId,
			ForeignDebit:        decimal.NewFromInt(0),
			ForeignCredit:       decimal.NewFromInt(0),
			ExchangeRate:        exchangeRate,
			RealisedAmount:      baseAmount,
		}
		accountIds = append(accountIds, systemAccounts[models.AccountCodeExchangeGainOrLoss])
		accTransactions = append(accTransactions, exchangeGainLoss)
	}

	accJournal := models.AccountJournal{
		BusinessId:          businessId,
		BranchId:            supplierCreditBill.BranchId,
		TransactionDateTime: supplierCreditBill.CreditDate,
		TransactionNumber:   strconv.Itoa(supplierCreditBill.ID),
		TransactionDetails:  "",
		ReferenceNumber:     supplierCreditBill.BillNumber,
		ReferenceId:         supplierCreditBill.ID,
		ReferenceType:       models.AccountReferenceTypeSupplierCreditApplied,
		SupplierId:          supplierCreditBill.SupplierId,
		AccountTransactions: accTransactions,
	}

	err = tx.Create(&accJournal).Error
	if err != nil {
		config.LogError(logger, "SupplierCreditAppliedWorkflow.go", "CreateSupplierCreditApplied", "CreateAccountJournal", accJournal, err)
		return 0, nil, 0, err
	}
	return accJournal.ID, accountIds, foreignCurrencyId, nil
}

func DeleteSupplierCreditApplied(tx *gorm.DB, logger *logrus.Logger, recordId int, recordType string, businessId string, business models.Business, oldSupplierCreditBill models.SupplierCreditBill) (int, []int, int, error) {

	foreignCurrencyId := oldSupplierCreditBill.CurrencyId
	accountJournal, _, accountIds, err := GetExistingAccountJournal(tx, oldSupplierCreditBill.ID, models.AccountReferenceTypeSupplierCreditApplied)
	if err != nil {
		config.LogError(logger, "SupplierCreditAppliedWorkflow.go", "DeleteSupplierCreditApplied", "GetExistingAccountJournal", oldSupplierCreditBill, err)
		return 0, nil, 0, err
	}

	// Phase 1: do not delete posted journals; create a reversal journal instead.
	reversalID, err := ReverseAccountJournal(tx, accountJournal, ReversalReasonSupplierCreditAppliedVoidUpdate)
	if err != nil {
		config.LogError(logger, "SupplierCreditAppliedWorkflow.go", "DeleteSupplierCreditApplied", "ReverseAccountJournal", accountJournal, err)
		return 0, nil, 0, err
	}
	return reversalID, accountIds, foreignCurrencyId, nil
}
