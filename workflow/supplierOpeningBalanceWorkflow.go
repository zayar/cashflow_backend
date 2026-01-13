package workflow

import (
	"encoding/json"
	"strconv"

	"bitbucket.org/mmdatafocus/books_backend/config"
	"bitbucket.org/mmdatafocus/books_backend/models"
	"bitbucket.org/mmdatafocus/books_backend/utils"
	"github.com/shopspring/decimal"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

func ProcessSupplierOpeningBalanceWorkflow(tx *gorm.DB, logger *logrus.Logger, msg config.PubSubMessage) error {
	var accountJournalId int
	var accountIds []int
	var foreignCurrencyId int
	business, err := models.GetBusinessById2(tx, msg.BusinessId)
	if err != nil {
		config.LogError(logger, "SupplierOpeningBalanceWorkflow.go", "ProcessSupplierOpeningBalanceWorkflow", "GetBusiness", msg.BusinessId, err)
		return err
	}
	if msg.Action == string(models.PubSubMessageActionCreate) {

		var supplierOpeningBalance models.SupplierOpeningBalance
		err := json.Unmarshal([]byte(msg.NewObj), &supplierOpeningBalance)
		if err != nil {
			config.LogError(logger, "SupplierOpeningBalanceWorkflow.go", "ProcessSupplierOpeningBalanceWorkflow > Create", "Unmarshal msg.NewObj", msg.NewObj, err)
			return err
		}
		accountJournalId, accountIds, foreignCurrencyId, err = CreateSupplierOpeningBalance(tx, logger, msg.ID, msg.ReferenceType, msg.BusinessId, *business, supplierOpeningBalance)
		if err != nil {
			config.LogError(logger, "SupplierOpeningBalanceWorkflow.go", "ProcessSupplierOpeningBalanceWorkflow > Create", "CreateSupplierOpeningBalance", nil, err)
			return err
		}
		err = UpdateBalances(tx, logger, msg.BusinessId, business.BaseCurrencyId, supplierOpeningBalance.OpeningBalanceBranchId, accountIds, business.MigrationDate, foreignCurrencyId)
		if err != nil {
			config.LogError(logger, "SupplierOpeningBalanceWorkflow.go", "ProcessSupplierOpeningBalanceWorkflow > Create", "UpdateBalances", supplierOpeningBalance, err)
			return err
		}

	} else if msg.Action == string(models.PubSubMessageActionUpdate) {

		var oldForeignCurrencyId int
		var oldAccountIds []int
		var supplierOpeningBalance models.SupplierOpeningBalance
		err := json.Unmarshal([]byte(msg.NewObj), &supplierOpeningBalance)
		if err != nil {
			config.LogError(logger, "SupplierOpeningBalanceWorkflow.go", "ProcessSupplierOpeningBalanceWorkflow > Update", "Unmarshal msg.NewObj", msg.NewObj, err)
			return err
		}
		var oldSupplierOpeningBalance models.SupplierOpeningBalance
		err = json.Unmarshal([]byte(msg.OldObj), &oldSupplierOpeningBalance)
		if err != nil {
			config.LogError(logger, "SupplierOpeningBalanceWorkflow.go", "ProcessSupplierOpeningBalanceWorkflow > Update", "Unmarshal msg.OldObj", msg.OldObj, err)
			return err
		}
		_, oldAccountIds, oldForeignCurrencyId, err = DeleteSupplierOpeningBalance(tx, logger, msg.ID, msg.ReferenceType, msg.BusinessId, *business, oldSupplierOpeningBalance)
		if err != nil {
			config.LogError(logger, "SupplierOpeningBalanceWorkflow.go", "ProcessSupplierOpeningBalanceWorkflow > Update", "DeleteSupplierOpeningBalance", nil, err)
			return err
		}
		accountJournalId, accountIds, foreignCurrencyId, err = CreateSupplierOpeningBalance(tx, logger, msg.ID, msg.ReferenceType, msg.BusinessId, *business, supplierOpeningBalance)
		if err != nil {
			config.LogError(logger, "SupplierOpeningBalanceWorkflow.go", "ProcessSupplierOpeningBalanceWorkflow > Update", "CreateSupplierOpeningBalance", nil, err)
			return err
		}
		if oldSupplierOpeningBalance.OpeningBalanceBranchId != supplierOpeningBalance.OpeningBalanceBranchId || oldForeignCurrencyId != foreignCurrencyId {
			err = UpdateBalances(tx, logger, msg.BusinessId, business.BaseCurrencyId, oldSupplierOpeningBalance.OpeningBalanceBranchId, oldAccountIds, business.MigrationDate, oldForeignCurrencyId)
			if err != nil {
				config.LogError(logger, "SupplierOpeningBalanceWorkflow.go", "ProcessSupplierOpeningBalanceWorkflow > Update", "UpdateBalances Old", oldSupplierOpeningBalance, err)
				return err
			}
		} else {
			accountIds = utils.MergeIntSlices(accountIds, oldAccountIds)
		}
		err = UpdateBalances(tx, logger, msg.BusinessId, business.BaseCurrencyId, supplierOpeningBalance.OpeningBalanceBranchId, accountIds, business.MigrationDate, foreignCurrencyId)
		if err != nil {
			config.LogError(logger, "SupplierOpeningBalanceWorkflow.go", "ProcessSupplierOpeningBalanceWorkflow > Update", "UpdateBalances", supplierOpeningBalance, err)
			return err
		}
	} else if msg.Action == string(models.PubSubMessageActionDelete) {

		var oldSupplierOpeningBalance models.SupplierOpeningBalance
		err = json.Unmarshal([]byte(msg.OldObj), &oldSupplierOpeningBalance)
		if err != nil {
			config.LogError(logger, "SupplierOpeningBalanceWorkflow.go", "ProcessSupplierOpeningBalanceWorkflow > Delete", "Unmarshal msg.OldObj", msg.OldObj, err)
			return err
		}
		accountJournalId, accountIds, foreignCurrencyId, err = DeleteSupplierOpeningBalance(tx, logger, msg.ID, msg.ReferenceType, msg.BusinessId, *business, oldSupplierOpeningBalance)
		if err != nil {
			config.LogError(logger, "SupplierOpeningBalanceWorkflow.go", "ProcessSupplierOpeningBalanceWorkflow > Delete", "DeleteSupplierOpeningBalance", nil, err)
			return err
		}
		err = UpdateBalances(tx, logger, msg.BusinessId, business.BaseCurrencyId, oldSupplierOpeningBalance.OpeningBalanceBranchId, accountIds, business.MigrationDate, foreignCurrencyId)
		if err != nil {
			config.LogError(logger, "SupplierOpeningBalanceWorkflow.go", "ProcessSupplierOpeningBalanceWorkflow > Delete", "UpdateBalances", oldSupplierOpeningBalance, err)
			return err
		}
	}

	err = tx.Model(&models.PubSubMessageRecord{}).Where("id=?", msg.ID).Updates(map[string]interface{}{"account_journal_id": accountJournalId, "is_processed": true}).Error
	if err != nil {
		config.LogError(logger, "SupplierOpeningBalanceWorkflow.go", "ProcessSupplierOpeningBalanceWorkflow", "UpdatePubSubMessageRecord", accountJournalId, err)
		return err
	}
	return nil
}

func CreateSupplierOpeningBalance(tx *gorm.DB, logger *logrus.Logger, recordId int, recordType string, businessId string, business models.Business, supplierOpeningBalance models.SupplierOpeningBalance) (int, []int, int, error) {

	systemAccounts, err := models.GetSystemAccounts(businessId)
	if err != nil {
		config.LogError(logger, "SupplierOpeningBalanceWorkflow.go", "CreateSupplierOpeningBalance", "GetSystemAccounts", businessId, err)
		return 0, nil, 0, err
	}

	baseAmount := supplierOpeningBalance.OpeningBalance
	foreignAmount := decimal.NewFromInt(0)
	exchangeRate := supplierOpeningBalance.ExchangeRate
	transactionTime := business.MigrationDate
	branchId := supplierOpeningBalance.OpeningBalanceBranchId
	baseCurrencyId := business.BaseCurrencyId
	foreignCurrencyId := supplierOpeningBalance.CurrencyId

	if baseCurrencyId != foreignCurrencyId {
		baseAmount = supplierOpeningBalance.OpeningBalance.Mul(exchangeRate)
		foreignAmount = supplierOpeningBalance.OpeningBalance
	}

	accountsPayable := models.AccountTransaction{
		BusinessId:          businessId,
		AccountId:           systemAccounts[models.AccountCodeAccountsPayable],
		BranchId:            branchId,
		TransactionDateTime: transactionTime,
		BaseCurrencyId:      baseCurrencyId,
		BaseDebit:           decimal.NewFromInt(0),
		BaseCredit:          baseAmount,
		ForeignCurrencyId:   foreignCurrencyId,
		ForeignDebit:        decimal.NewFromInt(0),
		ForeignCredit:       foreignAmount,
		ExchangeRate:        exchangeRate,
	}

	openingBalanceAdjustments := models.AccountTransaction{
		BusinessId:          businessId,
		AccountId:           systemAccounts[models.AccountCodeOpeningBalanceAdjustments],
		BranchId:            branchId,
		TransactionDateTime: transactionTime,
		BaseCurrencyId:      baseCurrencyId,
		BaseDebit:           baseAmount,
		BaseCredit:          decimal.NewFromInt(0),
		ForeignCurrencyId:   foreignCurrencyId,
		ForeignCredit:       decimal.NewFromInt(0),
		ForeignDebit:        foreignAmount,
		ExchangeRate:        exchangeRate,
	}

	journal := models.AccountJournal{
		BusinessId:          businessId,
		BranchId:            branchId,
		TransactionDateTime: transactionTime,
		TransactionNumber:   strconv.Itoa(supplierOpeningBalance.ID),
		TransactionDetails:  supplierOpeningBalance.SupplierName,
		SupplierId:          supplierOpeningBalance.ID,
		ReferenceId:         supplierOpeningBalance.ID,
		ReferenceType:       models.AccountReferenceTypeSupplierOpeningBalance,
		AccountTransactions: []models.AccountTransaction{accountsPayable, openingBalanceAdjustments},
	}

	accountIds := []int{systemAccounts[models.AccountCodeAccountsPayable], systemAccounts[models.AccountCodeOpeningBalanceAdjustments]}

	err = tx.Create(&journal).Error
	if err != nil {
		config.LogError(logger, "SupplierOpeningBalanceWorkflow.go", "CreateSupplierOpeningBalance", "CreateAccountJournal", journal, err)
		return 0, nil, 0, err
	}
	return journal.ID, accountIds, foreignCurrencyId, nil
}

func DeleteSupplierOpeningBalance(tx *gorm.DB, logger *logrus.Logger, recordId int, recordType string, businessId string, business models.Business, oldSupplierOpeningBalance models.SupplierOpeningBalance) (int, []int, int, error) {

	foreignCurrencyId := oldSupplierOpeningBalance.CurrencyId
	accountJournal, _, accountIds, err := GetExistingAccountJournal(tx, oldSupplierOpeningBalance.ID, models.AccountReferenceTypeSupplierOpeningBalance)
	if err != nil {
		config.LogError(logger, "SupplierOpeningBalanceWorkflow.go", "DeleteSupplierOpeningBalance", "GetExistingAccountJournal", oldSupplierOpeningBalance, err)
		return 0, nil, 0, err
	}

	// Phase 1: do not delete posted journals; create a reversal journal instead.
	reversalID, err := ReverseAccountJournal(tx, accountJournal, ReversalReasonSupplierOpeningBalanceResetVoid)
	if err != nil {
		config.LogError(logger, "SupplierOpeningBalanceWorkflow.go", "DeleteSupplierOpeningBalance", "ReverseAccountJournal", accountJournal, err)
		return 0, nil, 0, err
	}
	return reversalID, accountIds, foreignCurrencyId, nil
}
