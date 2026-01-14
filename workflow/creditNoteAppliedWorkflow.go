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

func ProcessCreditNoteAppliedWorkflow(tx *gorm.DB, logger *logrus.Logger, msg config.PubSubMessage) error {

	var accountJournalId int
	var accountIds []int
	var foreignCurrencyId int
	business, err := models.GetBusinessById2(tx, msg.BusinessId)
	if err != nil {
		config.LogError(logger, "CreditNoteAppliedWorkflow.go", "ProcessCreditNoteAppliedWorkflow", "GetBusiness", msg.BusinessId, err)
		return err
	}
	if msg.Action == string(models.PubSubMessageActionCreate) {

		var customerCreditInvoice models.CustomerCreditInvoice
		err := json.Unmarshal([]byte(msg.NewObj), &customerCreditInvoice)
		if err != nil {
			config.LogError(logger, "CreditNoteAppliedWorkflow.go", "ProcessCreditNoteAppliedWorkflow > Create", "Unmarshal msg.NewObj", msg.NewObj, err)
			return err
		}
		accountJournalId, accountIds, foreignCurrencyId, err = CreateCreditNoteApplied(tx, logger, msg.ID, msg.ReferenceType, msg.BusinessId, *business, customerCreditInvoice)
		if err != nil {
			config.LogError(logger, "CreditNoteAppliedWorkflow.go", "ProcessCreditNoteAppliedWorkflow > Create", "CreateCreditNoteApplied", nil, err)
			return err
		}
		err = UpdateBalances(tx, logger, msg.BusinessId, business.BaseCurrencyId, customerCreditInvoice.BranchId, accountIds, customerCreditInvoice.CreditDate, foreignCurrencyId)
		if err != nil {
			config.LogError(logger, "CreditNoteAppliedWorkflow.go", "ProcessCreditNoteAppliedWorkflow > Create", "UpdateBalances", customerCreditInvoice, err)
			return err
		}

	} else if msg.Action == string(models.PubSubMessageActionUpdate) {

		var oldForeignCurrencyId int
		var oldAccountIds []int
		var customerCreditInvoice models.CustomerCreditInvoice
		err := json.Unmarshal([]byte(msg.NewObj), &customerCreditInvoice)
		if err != nil {
			config.LogError(logger, "CreditNoteAppliedWorkflow.go", "ProcessCreditNoteAppliedWorkflow > Update", "Unmarshal msg.NewObj", msg.NewObj, err)
			return err
		}
		var oldCustomerCreditInvoice models.CustomerCreditInvoice
		err = json.Unmarshal([]byte(msg.OldObj), &oldCustomerCreditInvoice)
		if err != nil {
			config.LogError(logger, "CreditNoteAppliedWorkflow.go", "ProcessCreditNoteAppliedWorkflow > Update", "Unmarshal msg.OldObj", msg.OldObj, err)
			return err
		}
		_, oldAccountIds, oldForeignCurrencyId, err = DeleteCreditNoteApplied(tx, logger, msg.ID, msg.ReferenceType, msg.BusinessId, *business, oldCustomerCreditInvoice)
		if err != nil {
			config.LogError(logger, "CreditNoteAppliedWorkflow.go", "ProcessCreditNoteAppliedWorkflow > Update", "DeleteCreditNoteApplied", nil, err)
			return err
		}
		accountJournalId, accountIds, foreignCurrencyId, err = CreateCreditNoteApplied(tx, logger, msg.ID, msg.ReferenceType, msg.BusinessId, *business, customerCreditInvoice)
		if err != nil {
			config.LogError(logger, "CreditNoteAppliedWorkflow.go", "ProcessCreditNoteAppliedWorkflow > Update", "CreateCreditNoteApplied", nil, err)
			return err
		}
		if oldCustomerCreditInvoice.BranchId != customerCreditInvoice.BranchId || oldCustomerCreditInvoice.CreditDate != customerCreditInvoice.CreditDate || oldForeignCurrencyId != foreignCurrencyId {
			err = UpdateBalances(tx, logger, msg.BusinessId, business.BaseCurrencyId, oldCustomerCreditInvoice.BranchId, oldAccountIds, oldCustomerCreditInvoice.CreditDate, oldForeignCurrencyId)
			if err != nil {
				config.LogError(logger, "CreditNoteAppliedWorkflow.go", "ProcessCreditNoteAppliedWorkflow > Update", "UpdateBalances Old", oldCustomerCreditInvoice, err)
				return err
			}
		} else {
			accountIds = utils.MergeIntSlices(accountIds, oldAccountIds)
		}
		err = UpdateBalances(tx, logger, msg.BusinessId, business.BaseCurrencyId, customerCreditInvoice.BranchId, accountIds, customerCreditInvoice.CreditDate, foreignCurrencyId)
		if err != nil {
			config.LogError(logger, "CreditNoteAppliedWorkflow.go", "ProcessCreditNoteAppliedWorkflow > Update", "UpdateBalances", customerCreditInvoice, err)
			return err
		}
	} else if msg.Action == string(models.PubSubMessageActionDelete) {

		var oldCustomerCreditInvoice models.CustomerCreditInvoice
		err = json.Unmarshal([]byte(msg.OldObj), &oldCustomerCreditInvoice)
		if err != nil {
			config.LogError(logger, "CreditNoteAppliedWorkflow.go", "ProcessCreditNoteAppliedWorkflow > Delete", "Unmarshal msg.OldObj", msg.OldObj, err)
			return err
		}
		accountJournalId, accountIds, foreignCurrencyId, err = DeleteCreditNoteApplied(tx, logger, msg.ID, msg.ReferenceType, msg.BusinessId, *business, oldCustomerCreditInvoice)
		if err != nil {
			config.LogError(logger, "CreditNoteAppliedWorkflow.go", "ProcessCreditNoteAppliedWorkflow > Delete", "DeleteCreditNoteApplied", nil, err)
			return err
		}
		err = UpdateBalances(tx, logger, msg.BusinessId, business.BaseCurrencyId, oldCustomerCreditInvoice.BranchId, accountIds, oldCustomerCreditInvoice.CreditDate, foreignCurrencyId)
		if err != nil {
			config.LogError(logger, "CreditNoteAppliedWorkflow.go", "ProcessCreditNoteAppliedWorkflow > Delete", "UpdateBalances", oldCustomerCreditInvoice, err)
			return err
		}
	}
	err = tx.Model(&models.PubSubMessageRecord{}).Where("id=?", msg.ID).Updates(map[string]interface{}{"account_journal_id": accountJournalId, "is_processed": true}).Error
	if err != nil {
		config.LogError(logger, "CreditNoteAppliedWorkflow.go", "ProcessCreditNoteAppliedWorkflow", "UpdatePubSubMessageRecord", accountJournalId, err)
		return err
	}
	return nil
}

func CreateCreditNoteApplied(tx *gorm.DB, logger *logrus.Logger, recordId int, recordType string, businessId string, business models.Business, customerCreditInvoice models.CustomerCreditInvoice) (int, []int, int, error) {

	systemAccounts, err := models.GetSystemAccounts(businessId)
	if err != nil {
		config.LogError(logger, "CreditNoteAppliedWorkflow.go", "CreateCreditNoteApplied", "GetSystemAccounts", businessId, err)
		return 0, nil, 0, err
	}

	transactionTime := customerCreditInvoice.CreditDate
	branchId := customerCreditInvoice.BranchId
	baseCurrencyId := business.BaseCurrencyId
	foreignCurrencyId := customerCreditInvoice.CurrencyId
	exchangeRate := customerCreditInvoice.ExchangeRate
	baseInvoiceAmount := customerCreditInvoice.Amount.Mul(customerCreditInvoice.InvoiceExchangeRate)
	baseAmount := customerCreditInvoice.Amount.Mul(customerCreditInvoice.ExchangeRate)

	accountIds := make([]int, 0)
	accTransactions := make([]models.AccountTransaction, 0)

	if baseInvoiceAmount.GreaterThanOrEqual(baseAmount) {
		gainAmount := baseInvoiceAmount.Sub(baseAmount)
		accountsReceivable := models.AccountTransaction{
			BusinessId:          businessId,
			AccountId:           systemAccounts[models.AccountCodeAccountsReceivable],
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
		accountIds = append(accountIds, systemAccounts[models.AccountCodeAccountsReceivable])

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
	} else {
		lossAmount := baseAmount.Sub(baseInvoiceAmount)
		accountsReceivable := models.AccountTransaction{
			BusinessId:          businessId,
			AccountId:           systemAccounts[models.AccountCodeAccountsReceivable],
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
		accountIds = append(accountIds, systemAccounts[models.AccountCodeAccountsReceivable])

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
	}

	accJournal := models.AccountJournal{
		BusinessId:          businessId,
		BranchId:            customerCreditInvoice.BranchId,
		TransactionDateTime: customerCreditInvoice.CreditDate,
		TransactionNumber:   strconv.Itoa(customerCreditInvoice.ID),
		TransactionDetails:  "",
		ReferenceNumber:     customerCreditInvoice.InvoiceNumber,
		ReferenceId:         customerCreditInvoice.ID,
		ReferenceType:       models.AccountReferenceTypeCreditNoteApplied,
		CustomerId:          customerCreditInvoice.CustomerId,
		AccountTransactions: accTransactions,
	}

	err = tx.Create(&accJournal).Error
	if err != nil {
		config.LogError(logger, "CreditNoteAppliedWorkflow.go", "CreateCreditNoteApplied", "CreateAccountJournal", accJournal, err)
		return 0, nil, 0, err
	}
	return accJournal.ID, accountIds, foreignCurrencyId, nil
}

func DeleteCreditNoteApplied(tx *gorm.DB, logger *logrus.Logger, recordId int, recordType string, businessId string, business models.Business, oldCustomerCreditInvoice models.CustomerCreditInvoice) (int, []int, int, error) {

	foreignCurrencyId := oldCustomerCreditInvoice.CurrencyId
	accountJournal, _, accountIds, err := GetExistingAccountJournal(tx, oldCustomerCreditInvoice.ID, models.AccountReferenceTypeCreditNoteApplied)
	if err != nil {
		config.LogError(logger, "CreditNoteAppliedWorkflow.go", "DeleteCreditNoteApplied", "GetExistingAccountJournal", oldCustomerCreditInvoice, err)
		return 0, nil, 0, err
	}

	// Phase 1: do not delete posted journals; create a reversal journal instead.
	reversalID, err := ReverseAccountJournal(tx, accountJournal, ReversalReasonCreditNoteAppliedVoidUpdate)
	if err != nil {
		config.LogError(logger, "CreditNoteAppliedWorkflow.go", "DeleteCreditNoteApplied", "ReverseAccountJournal", accountJournal, err)
		return 0, nil, 0, err
	}
	return reversalID, accountIds, foreignCurrencyId, nil
}
