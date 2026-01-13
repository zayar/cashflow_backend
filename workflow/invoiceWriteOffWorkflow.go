package workflow

import (
	"encoding/json"
	"strconv"

	"bitbucket.org/mmdatafocus/books_backend/config"
	"bitbucket.org/mmdatafocus/books_backend/models"
	"github.com/shopspring/decimal"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

func ProcessInvoiceWriteOffWorkflow(tx *gorm.DB, logger *logrus.Logger, msg config.PubSubMessage) error {

	var accountJournalId int
	var accountIds []int
	var foreignCurrencyId int
	business, err := models.GetBusinessById2(tx, msg.BusinessId)
	if err != nil {
		config.LogError(logger, "InvoiceWriteOffWorkflow.go", "ProcessInvoiceWriteOffWorkflow", "GetBusiness", msg.BusinessId, err)
		return err
	}
	if msg.Action == string(models.PubSubMessageActionCreate) {

		var invoice models.SalesInvoice
		err := json.Unmarshal([]byte(msg.NewObj), &invoice)
		if err != nil {
			config.LogError(logger, "InvoiceWriteOffWorkflow.go", "ProcessInvoiceWriteOffWorkflow > Create", "Unmarshal msg.NewObj", msg.NewObj, err)
			return err
		}
		accountJournalId, accountIds, foreignCurrencyId, err = CreateInvoiceWriteOff(tx, logger, msg.ID, msg.ReferenceType, msg.BusinessId, *business, invoice)
		if err != nil {
			config.LogError(logger, "InvoiceWriteOffWorkflow.go", "ProcessInvoiceWriteOffWorkflow > Create", "CreateInvoiceWriteOff", nil, err)
			return err
		}
		err = UpdateBalances(tx, logger, msg.BusinessId, business.BaseCurrencyId, invoice.BranchId, accountIds, *invoice.WriteOffDate, foreignCurrencyId)
		if err != nil {
			config.LogError(logger, "InvoiceWriteOffWorkflow.go", "ProcessInvoiceWriteOffWorkflow > Create", "UpdateBalances", invoice, err)
			return err
		}
	} else if msg.Action == string(models.PubSubMessageActionDelete) {

		var oldInvoice models.SalesInvoice
		err = json.Unmarshal([]byte(msg.OldObj), &oldInvoice)
		if err != nil {
			config.LogError(logger, "InvoiceWriteOffWorkflow.go", "ProcessInvoiceWriteOffWorkflow > Delete", "Unmarshal msg.OldObj", msg.OldObj, err)
			return err
		}
		accountJournalId, accountIds, foreignCurrencyId, err = DeleteInvoiceWriteOff(tx, logger, msg.ID, msg.ReferenceType, msg.BusinessId, *business, oldInvoice)
		if err != nil {
			config.LogError(logger, "InvoiceWriteOffWorkflow.go", "ProcessInvoiceWriteOffWorkflow > Delete", "DeleteInvoiceWriteOff", nil, err)
			return err
		}
		err = UpdateBalances(tx, logger, msg.BusinessId, business.BaseCurrencyId, oldInvoice.BranchId, accountIds, msg.TransactionDateTime, foreignCurrencyId)
		if err != nil {
			config.LogError(logger, "InvoiceWriteOffWorkflow.go", "ProcessInvoiceWriteOffWorkflow > Delete", "UpdateBalances", oldInvoice, err)
			return err
		}
	}
	err = tx.Model(&models.PubSubMessageRecord{}).Where("id=?", msg.ID).Updates(map[string]interface{}{"account_journal_id": accountJournalId, "is_processed": true}).Error
	if err != nil {
		config.LogError(logger, "InvoiceWriteOffWorkflow.go", "ProcessInvoiceWriteOffWorkflow", "UpdatePubSubMessageRecord", accountJournalId, err)
		return err
	}
	return nil
}

func CreateInvoiceWriteOff(tx *gorm.DB, logger *logrus.Logger, recordId int, recordType string, businessId string, business models.Business, invoice models.SalesInvoice) (int, []int, int, error) {

	systemAccounts, err := models.GetSystemAccounts(businessId)
	if err != nil {
		config.LogError(logger, "InvoiceWriteOffWorkflow.go", "CreateInvoiceWriteOff", "GetSystemAccounts", businessId, err)
		return 0, nil, 0, err
	}

	transactionTime := *invoice.WriteOffDate
	branchId := invoice.BranchId
	baseCurrencyId := business.BaseCurrencyId
	foreignCurrencyId := invoice.CurrencyId

	baseAmount := invoice.InvoiceTotalWriteOffAmount
	foreignAmount := decimal.NewFromInt(0)
	exchangeRate := invoice.ExchangeRate

	if baseCurrencyId != foreignCurrencyId {
		foreignAmount = baseAmount
		baseAmount = foreignAmount.Mul(exchangeRate)
	}

	accountIds := make([]int, 0)
	accTransactions := make([]models.AccountTransaction, 0)

	accountsReceivable := models.AccountTransaction{
		BusinessId:          businessId,
		AccountId:           systemAccounts[models.AccountCodeAccountsReceivable],
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
	accTransactions = append(accTransactions, accountsReceivable)
	accountIds = append(accountIds, systemAccounts[models.AccountCodeAccountsReceivable])

	badDebt := models.AccountTransaction{
		BusinessId:          businessId,
		AccountId:           systemAccounts[models.AccountCodeBadDebt],
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
	accTransactions = append(accTransactions, badDebt)
	accountIds = append(accountIds, systemAccounts[models.AccountCodeBadDebt])

	accJournal := models.AccountJournal{
		BusinessId:          businessId,
		BranchId:            invoice.BranchId,
		TransactionDateTime: transactionTime,
		TransactionNumber:   strconv.Itoa(invoice.ID),
		TransactionDetails:  "",
		ReferenceNumber:     invoice.InvoiceNumber,
		ReferenceId:         invoice.ID,
		ReferenceType:       models.AccountReferenceTypeInvoiceWriteOff,
		CustomerId:          invoice.CustomerId,
		AccountTransactions: accTransactions,
	}

	err = tx.Create(&accJournal).Error
	if err != nil {
		config.LogError(logger, "InvoiceWriteOffWorkflow.go", "CreateInvoiceWriteOff", "CreateAccountJournal", accJournal, err)
		return 0, nil, 0, err
	}
	return accJournal.ID, accountIds, foreignCurrencyId, nil
}

func DeleteInvoiceWriteOff(tx *gorm.DB, logger *logrus.Logger, recordId int, recordType string, businessId string, business models.Business, oldInvoice models.SalesInvoice) (int, []int, int, error) {

	foreignCurrencyId := oldInvoice.CurrencyId
	accountJournal, _, accountIds, err := GetExistingAccountJournal(tx, oldInvoice.ID, models.AccountReferenceTypeInvoiceWriteOff)
	if err != nil {
		config.LogError(logger, "InvoiceWriteOffWorkflow.go", "DeleteInvoiceWriteOff", "GetExistingAccountJournal", oldInvoice, err)
		return 0, nil, 0, err
	}

	// Phase 1: do not delete posted journals; create a reversal journal instead.
	reversalID, err := ReverseAccountJournal(tx, accountJournal, ReversalReasonInvoiceWriteOffVoidUpdate)
	if err != nil {
		config.LogError(logger, "InvoiceWriteOffWorkflow.go", "DeleteInvoiceWriteOff", "ReverseAccountJournal", accountJournal, err)
		return 0, nil, 0, err
	}
	return reversalID, accountIds, foreignCurrencyId, nil
}
