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

func ProcessCustomerOpeningBalanceWorkflow(tx *gorm.DB, logger *logrus.Logger, msg config.PubSubMessage) error {
	var accountJournalId int
	var accountIds []int
	var foreignCurrencyId int
	business, err := models.GetBusinessById2(tx, msg.BusinessId)
	if err != nil {
		config.LogError(logger, "CustomerOpeningBalanceWorkflow.go", "ProcessCustomerOpeningBalanceWorkflow", "GetBusiness", msg.BusinessId, err)
		return err
	}
	if msg.Action == string(models.PubSubMessageActionCreate) {

		var customerOpeningBalance models.CustomerOpeningBalance
		err := json.Unmarshal([]byte(msg.NewObj), &customerOpeningBalance)
		if err != nil {
			config.LogError(logger, "CustomerOpeningBalanceWorkflow.go", "ProcessCustomerOpeningBalanceWorkflow > Create", "Unmarshal msg.NewObj", msg.NewObj, err)
			return err
		}
		accountJournalId, accountIds, foreignCurrencyId, err = CreateCustomerOpeningBalance(tx, logger, msg.ID, msg.ReferenceType, msg.BusinessId, *business, customerOpeningBalance)
		if err != nil {
			config.LogError(logger, "CustomerOpeningBalanceWorkflow.go", "ProcessCustomerOpeningBalanceWorkflow > Create", "CreateCustomerOpeningBalance", nil, err)
			return err
		}
		err = UpdateBalances(tx, logger, msg.BusinessId, business.BaseCurrencyId, customerOpeningBalance.OpeningBalanceBranchId, accountIds, business.MigrationDate, foreignCurrencyId)
		if err != nil {
			config.LogError(logger, "CustomerOpeningBalanceWorkflow.go", "ProcessCustomerOpeningBalanceWorkflow > Create", "UpdateBalances", customerOpeningBalance, err)
			return err
		}

	} else if msg.Action == string(models.PubSubMessageActionUpdate) {

		var oldForeignCurrencyId int
		var oldAccountIds []int
		var customerOpeningBalance models.CustomerOpeningBalance
		err := json.Unmarshal([]byte(msg.NewObj), &customerOpeningBalance)
		if err != nil {
			config.LogError(logger, "CustomerOpeningBalanceWorkflow.go", "ProcessCustomerOpeningBalanceWorkflow > Update", "Unmarshal msg.NewObj", msg.NewObj, err)
			return err
		}
		var oldCustomerOpeningBalance models.CustomerOpeningBalance
		err = json.Unmarshal([]byte(msg.OldObj), &oldCustomerOpeningBalance)
		if err != nil {
			config.LogError(logger, "CustomerOpeningBalanceWorkflow.go", "ProcessCustomerOpeningBalanceWorkflow > Update", "Unmarshal msg.OldObj", msg.OldObj, err)
			return err
		}
		_, oldAccountIds, oldForeignCurrencyId, err = DeleteCustomerOpeningBalance(tx, logger, msg.ID, msg.ReferenceType, msg.BusinessId, *business, oldCustomerOpeningBalance)
		if err != nil {
			config.LogError(logger, "CustomerOpeningBalanceWorkflow.go", "ProcessCustomerOpeningBalanceWorkflow > Update", "DeleteCustomerOpeningBalance", nil, err)
			return err
		}
		accountJournalId, accountIds, foreignCurrencyId, err = CreateCustomerOpeningBalance(tx, logger, msg.ID, msg.ReferenceType, msg.BusinessId, *business, customerOpeningBalance)
		if err != nil {
			config.LogError(logger, "CustomerOpeningBalanceWorkflow.go", "ProcessCustomerOpeningBalanceWorkflow > Update", "CreateCustomerOpeningBalance", nil, err)
			return err
		}
		if oldCustomerOpeningBalance.OpeningBalanceBranchId != customerOpeningBalance.OpeningBalanceBranchId || oldForeignCurrencyId != foreignCurrencyId {
			err = UpdateBalances(tx, logger, msg.BusinessId, business.BaseCurrencyId, oldCustomerOpeningBalance.OpeningBalanceBranchId, oldAccountIds, business.MigrationDate, oldForeignCurrencyId)
			if err != nil {
				config.LogError(logger, "CustomerOpeningBalanceWorkflow.go", "ProcessCustomerOpeningBalanceWorkflow > Update", "UpdateBalances Old", oldCustomerOpeningBalance, err)
				return err
			}
		} else {
			accountIds = utils.MergeIntSlices(accountIds, oldAccountIds)
		}
		err = UpdateBalances(tx, logger, msg.BusinessId, business.BaseCurrencyId, customerOpeningBalance.OpeningBalanceBranchId, accountIds, business.MigrationDate, foreignCurrencyId)
		if err != nil {
			config.LogError(logger, "CustomerOpeningBalanceWorkflow.go", "ProcessCustomerOpeningBalanceWorkflow > Update", "UpdateBalances", customerOpeningBalance, err)
			return err
		}
	} else if msg.Action == string(models.PubSubMessageActionDelete) {

		var oldCustomerOpeningBalance models.CustomerOpeningBalance
		err = json.Unmarshal([]byte(msg.OldObj), &oldCustomerOpeningBalance)
		if err != nil {
			config.LogError(logger, "CustomerOpeningBalanceWorkflow.go", "ProcessCustomerOpeningBalanceWorkflow > Delete", "Unmarshal msg.OldObj", msg.OldObj, err)
			return err
		}
		accountJournalId, accountIds, foreignCurrencyId, err = DeleteCustomerOpeningBalance(tx, logger, msg.ID, msg.ReferenceType, msg.BusinessId, *business, oldCustomerOpeningBalance)
		if err != nil {
			config.LogError(logger, "CustomerOpeningBalanceWorkflow.go", "ProcessCustomerOpeningBalanceWorkflow > Delete", "DeleteCustomerOpeningBalance", nil, err)
			return err
		}
		err = UpdateBalances(tx, logger, msg.BusinessId, business.BaseCurrencyId, oldCustomerOpeningBalance.OpeningBalanceBranchId, accountIds, business.MigrationDate, foreignCurrencyId)
		if err != nil {
			config.LogError(logger, "CustomerOpeningBalanceWorkflow.go", "ProcessCustomerOpeningBalanceWorkflow > Delete", "UpdateBalances", oldCustomerOpeningBalance, err)
			return err
		}
	}
	err = tx.Model(&models.PubSubMessageRecord{}).Where("id=?", msg.ID).Updates(map[string]interface{}{"account_journal_id": accountJournalId, "is_processed": true}).Error
	if err != nil {
		config.LogError(logger, "CustomerOpeningBalanceWorkflow.go", "ProcessCustomerOpeningBalanceWorkflow", "UpdatePubSubMessageRecord", accountJournalId, err)
		return err
	}
	return nil
}

func CreateCustomerOpeningBalance(tx *gorm.DB, logger *logrus.Logger, recordId int, recordType string, businessId string, business models.Business, customerOpeningBalance models.CustomerOpeningBalance) (int, []int, int, error) {

	systemAccounts, err := models.GetSystemAccounts(businessId)
	if err != nil {
		config.LogError(logger, "CustomerOpeningBalanceWorkflow.go", "CreateCustomerOpeningBalance", "GetSystemAccounts", businessId, err)
		return 0, nil, 0, err
	}

	baseAmount := customerOpeningBalance.OpeningBalance
	foreignAmount := decimal.NewFromInt(0)
	exchangeRate := customerOpeningBalance.ExchangeRate
	transactionTime := business.MigrationDate
	branchId := customerOpeningBalance.OpeningBalanceBranchId
	baseCurrencyId := business.BaseCurrencyId
	foreignCurrencyId := customerOpeningBalance.CurrencyId

	if baseCurrencyId != foreignCurrencyId {
		baseAmount = customerOpeningBalance.OpeningBalance.Mul(exchangeRate)
		foreignAmount = customerOpeningBalance.OpeningBalance
	}

	accountsReceivable := models.AccountTransaction{
		BusinessId:          businessId,
		AccountId:           systemAccounts[models.AccountCodeAccountsReceivable],
		BranchId:            branchId,
		TransactionDateTime: transactionTime,
		BaseCurrencyId:      baseCurrencyId,
		BaseDebit:           baseAmount,
		BaseCredit:          decimal.NewFromInt(0),
		ForeignCurrencyId:   foreignCurrencyId,
		ForeignDebit:        foreignAmount,
		ForeignCredit:       decimal.NewFromInt(0),
		ExchangeRate:        exchangeRate,
	}

	openingBalanceAdjustments := models.AccountTransaction{
		BusinessId:          businessId,
		AccountId:           systemAccounts[models.AccountCodeOpeningBalanceAdjustments],
		BranchId:            branchId,
		TransactionDateTime: transactionTime,
		BaseCurrencyId:      baseCurrencyId,
		BaseDebit:           decimal.NewFromInt(0),
		BaseCredit:          baseAmount,
		ForeignCurrencyId:   foreignCurrencyId,
		ForeignCredit:       foreignAmount,
		ForeignDebit:        decimal.NewFromInt(0),
		ExchangeRate:        exchangeRate,
	}

	journal := models.AccountJournal{
		BusinessId:          businessId,
		BranchId:            branchId,
		TransactionDateTime: transactionTime,
		TransactionNumber:   strconv.Itoa(customerOpeningBalance.ID),
		TransactionDetails:  customerOpeningBalance.CustomerName,
		CustomerId:          customerOpeningBalance.ID,
		ReferenceId:         customerOpeningBalance.ID,
		ReferenceType:       models.AccountReferenceTypeCustomerOpeningBalance,
		AccountTransactions: []models.AccountTransaction{accountsReceivable, openingBalanceAdjustments},
	}

	accountIds := []int{systemAccounts[models.AccountCodeAccountsReceivable], systemAccounts[models.AccountCodeOpeningBalanceAdjustments]}

	err = tx.Create(&journal).Error
	if err != nil {
		config.LogError(logger, "CustomerOpeningBalanceWorkflow.go", "CreateCustomerOpeningBalance", "CreateAccountJournal", journal, err)
		return 0, nil, 0, err
	}
	return journal.ID, accountIds, foreignCurrencyId, nil
}

func DeleteCustomerOpeningBalance(tx *gorm.DB, logger *logrus.Logger, recordId int, recordType string, businessId string, business models.Business, oldCustomerOpeningBalance models.CustomerOpeningBalance) (int, []int, int, error) {

	foreignCurrencyId := oldCustomerOpeningBalance.CurrencyId
	accountJournal, _, accountIds, err := GetExistingAccountJournal(tx, oldCustomerOpeningBalance.ID, models.AccountReferenceTypeCustomerOpeningBalance)
	if err != nil {
		config.LogError(logger, "CustomerOpeningBalanceWorkflow.go", "DeleteCustomerOpeningBalance", "GetExistingAccountJournal", oldCustomerOpeningBalance, err)
		return 0, nil, 0, err
	}

	// Phase 1: do not delete posted journals; create a reversal journal instead.
	reversalID, err := ReverseAccountJournal(tx, accountJournal, ReversalReasonCustomerOpeningBalanceResetVoid)
	if err != nil {
		config.LogError(logger, "CustomerOpeningBalanceWorkflow.go", "DeleteCustomerOpeningBalance", "ReverseAccountJournal", accountJournal, err)
		return 0, nil, 0, err
	}
	return reversalID, accountIds, foreignCurrencyId, nil
}
