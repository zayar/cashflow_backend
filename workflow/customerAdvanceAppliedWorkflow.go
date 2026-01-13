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

func ProcessCustomerAdvanceAppliedWorkflow(tx *gorm.DB, logger *logrus.Logger, msg config.PubSubMessage) error {

	var accountJournalId int
	var accountIds []int
	var foreignCurrencyId int
	business, err := models.GetBusinessById2(tx, msg.BusinessId)
	if err != nil {
		config.LogError(logger, "CustomerAdvanceAppliedWorkflow.go", "ProcessCustomerAdvanceAppliedWorkflow", "GetBusiness", msg.BusinessId, err)
		return err
	}
	if msg.Action == string(models.PubSubMessageActionCreate) {

		var customerCreditInvoice models.CustomerCreditInvoice
		err := json.Unmarshal([]byte(msg.NewObj), &customerCreditInvoice)
		if err != nil {
			config.LogError(logger, "CustomerAdvanceAppliedWorkflow.go", "ProcessCustomerAdvanceAppliedWorkflow > Create", "Unmarshal msg.NewObj", msg.NewObj, err)
			return err
		}
		accountJournalId, accountIds, foreignCurrencyId, err = CreateCustomerAdvanceApplied(tx, logger, msg.ID, msg.ReferenceType, msg.BusinessId, *business, customerCreditInvoice)
		if err != nil {
			config.LogError(logger, "CustomerAdvanceAppliedWorkflow.go", "ProcessCustomerAdvanceAppliedWorkflow > Create", "CreateCustomerAdvanceApplied", nil, err)
			return err
		}
		err = UpdateBalances(tx, logger, msg.BusinessId, business.BaseCurrencyId, customerCreditInvoice.BranchId, accountIds, customerCreditInvoice.CreditDate, foreignCurrencyId)
		if err != nil {
			config.LogError(logger, "CustomerAdvanceAppliedWorkflow.go", "ProcessCustomerAdvanceAppliedWorkflow > Create", "UpdateBalances", customerCreditInvoice, err)
			return err
		}

	} else if msg.Action == string(models.PubSubMessageActionUpdate) {

		var oldForeignCurrencyId int
		var oldAccountIds []int
		var customerCreditInvoice models.CustomerCreditInvoice
		err := json.Unmarshal([]byte(msg.NewObj), &customerCreditInvoice)
		if err != nil {
			config.LogError(logger, "CustomerAdvanceAppliedWorkflow.go", "ProcessCustomerAdvanceAppliedWorkflow > Update", "Unmarshal msg.NewObj", msg.NewObj, err)
			return err
		}
		var oldCustomerCreditInvoice models.CustomerCreditInvoice
		err = json.Unmarshal([]byte(msg.OldObj), &oldCustomerCreditInvoice)
		if err != nil {
			config.LogError(logger, "CustomerAdvanceAppliedWorkflow.go", "ProcessCustomerAdvanceAppliedWorkflow > Update", "Unmarshal msg.OldObj", msg.OldObj, err)
			return err
		}
		_, oldAccountIds, oldForeignCurrencyId, err = DeleteCustomerAdvanceApplied(tx, logger, msg.ID, msg.ReferenceType, msg.BusinessId, *business, oldCustomerCreditInvoice)
		if err != nil {
			config.LogError(logger, "CustomerAdvanceAppliedWorkflow.go", "ProcessCustomerAdvanceAppliedWorkflow > Update", "DeleteCustomerAdvanceApplied", nil, err)
			return err
		}
		accountJournalId, accountIds, foreignCurrencyId, err = CreateCustomerAdvanceApplied(tx, logger, msg.ID, msg.ReferenceType, msg.BusinessId, *business, customerCreditInvoice)
		if err != nil {
			config.LogError(logger, "CustomerAdvanceAppliedWorkflow.go", "ProcessCustomerAdvanceAppliedWorkflow > Update", "CreateCustomerAdvanceApplied", nil, err)
			return err
		}
		if oldCustomerCreditInvoice.BranchId != customerCreditInvoice.BranchId || oldCustomerCreditInvoice.CreditDate != customerCreditInvoice.CreditDate || oldForeignCurrencyId != foreignCurrencyId {
			err = UpdateBalances(tx, logger, msg.BusinessId, business.BaseCurrencyId, oldCustomerCreditInvoice.BranchId, oldAccountIds, oldCustomerCreditInvoice.CreditDate, oldForeignCurrencyId)
			if err != nil {
				config.LogError(logger, "CustomerAdvanceAppliedWorkflow.go", "ProcessCustomerAdvanceAppliedWorkflow > Update", "UpdateBalances Old", oldCustomerCreditInvoice, err)
				return err
			}
		} else {
			accountIds = utils.MergeIntSlices(accountIds, oldAccountIds)
		}
		err = UpdateBalances(tx, logger, msg.BusinessId, business.BaseCurrencyId, customerCreditInvoice.BranchId, accountIds, customerCreditInvoice.CreditDate, foreignCurrencyId)
		if err != nil {
			config.LogError(logger, "CustomerAdvanceAppliedWorkflow.go", "ProcessCustomerAdvanceAppliedWorkflow > Update", "UpdateBalances", customerCreditInvoice, err)
			return err
		}
	} else if msg.Action == string(models.PubSubMessageActionDelete) {

		var oldCustomerCreditInvoice models.CustomerCreditInvoice
		err = json.Unmarshal([]byte(msg.OldObj), &oldCustomerCreditInvoice)
		if err != nil {
			config.LogError(logger, "CustomerAdvanceAppliedWorkflow.go", "ProcessCustomerAdvanceAppliedWorkflow > Delete", "Unmarshal msg.OldObj", msg.OldObj, err)
			return err
		}
		accountJournalId, accountIds, foreignCurrencyId, err = DeleteCustomerAdvanceApplied(tx, logger, msg.ID, msg.ReferenceType, msg.BusinessId, *business, oldCustomerCreditInvoice)
		if err != nil {
			config.LogError(logger, "CustomerAdvanceAppliedWorkflow.go", "ProcessCustomerAdvanceAppliedWorkflow > Delete", "DeleteCustomerAdvanceApplied", nil, err)
			return err
		}
		err = UpdateBalances(tx, logger, msg.BusinessId, business.BaseCurrencyId, oldCustomerCreditInvoice.BranchId, accountIds, oldCustomerCreditInvoice.CreditDate, foreignCurrencyId)
		if err != nil {
			config.LogError(logger, "CustomerAdvanceAppliedWorkflow.go", "ProcessCustomerAdvanceAppliedWorkflow > Delete", "UpdateBalances", oldCustomerCreditInvoice, err)
			return err
		}
	}
	err = tx.Model(&models.PubSubMessageRecord{}).Where("id=?", msg.ID).Updates(map[string]interface{}{"account_journal_id": accountJournalId, "is_processed": true}).Error
	if err != nil {
		config.LogError(logger, "CustomerAdvanceAppliedWorkflow.go", "ProcessCustomerAdvanceAppliedWorkflow", "UpdatePubSubMessageRecord", accountJournalId, err)
		return err
	}
	return nil
}

func CreateCustomerAdvanceApplied(tx *gorm.DB, logger *logrus.Logger, recordId int, recordType string, businessId string, business models.Business, customerCreditInvoice models.CustomerCreditInvoice) (int, []int, int, error) {

	systemAccounts, err := models.GetSystemAccounts(businessId)
	if err != nil {
		config.LogError(logger, "CustomerAdvanceAppliedWorkflow.go", "CreateCustomerAdvanceApplied", "GetSystemAccounts", businessId, err)
		return 0, nil, 0, err
	}

	transactionTime := customerCreditInvoice.CreditDate
	branchId := customerCreditInvoice.BranchId
	baseCurrencyId := business.BaseCurrencyId
	foreignCurrencyId := customerCreditInvoice.CurrencyId

	baseInvoiceAmount := customerCreditInvoice.Amount
	foreignInvoiceAmount := decimal.NewFromInt(0)
	baseAmount := customerCreditInvoice.Amount
	foreignAmount := decimal.NewFromInt(0)
	exchangeRate := customerCreditInvoice.ExchangeRate
	invoiceExchangeRate := customerCreditInvoice.InvoiceExchangeRate

	if baseCurrencyId != foreignCurrencyId {
		foreignAmount = baseAmount
		baseAmount = foreignAmount.Mul(exchangeRate)
	} else if baseCurrencyId == foreignCurrencyId && customerCreditInvoice.InvoiceCurrencyId != baseCurrencyId {
		foreignAmount = baseAmount.DivRound(exchangeRate, 4)
	}

	if baseCurrencyId == foreignCurrencyId && customerCreditInvoice.InvoiceCurrencyId != baseCurrencyId {
		foreignInvoiceAmount = foreignAmount
		baseInvoiceAmount = foreignInvoiceAmount.Mul(invoiceExchangeRate)
		foreignCurrencyId = customerCreditInvoice.InvoiceCurrencyId
	} else if baseCurrencyId != foreignCurrencyId && customerCreditInvoice.InvoiceCurrencyId != baseCurrencyId {
		foreignInvoiceAmount = baseInvoiceAmount
		baseInvoiceAmount = foreignInvoiceAmount.Mul(invoiceExchangeRate)
	} else if baseCurrencyId != foreignCurrencyId && customerCreditInvoice.InvoiceCurrencyId != foreignCurrencyId {
		foreignInvoiceAmount = baseInvoiceAmount
		baseInvoiceAmount = foreignInvoiceAmount.Mul(invoiceExchangeRate)
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
		BaseCredit:          baseInvoiceAmount,
		ForeignCurrencyId:   foreignCurrencyId,
		ForeignDebit:        decimal.NewFromInt(0),
		ForeignCredit:       foreignInvoiceAmount,
		ExchangeRate:        invoiceExchangeRate,
	}
	accTransactions = append(accTransactions, accountsReceivable)
	accountIds = append(accountIds, systemAccounts[models.AccountCodeAccountsReceivable])

	unearnedRevenue := models.AccountTransaction{
		BusinessId:          businessId,
		AccountId:           systemAccounts[models.AccountCodeUnearnedRevenue],
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
	accTransactions = append(accTransactions, unearnedRevenue)
	accountIds = append(accountIds, systemAccounts[models.AccountCodeUnearnedRevenue])

	if !baseInvoiceAmount.Equals(baseAmount) {
		gainLossAmount := baseAmount.Sub(baseInvoiceAmount)
		if gainLossAmount.IsPositive() {
			exchangeGainLoss := models.AccountTransaction{
				BusinessId:          businessId,
				AccountId:           systemAccounts[models.AccountCodeExchangeGainOrLoss],
				BranchId:            branchId,
				TransactionDateTime: transactionTime,
				BaseCurrencyId:      baseCurrencyId,
				BaseCredit:          gainLossAmount.Abs(),
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
			exchangeGainLoss := models.AccountTransaction{
				BusinessId:          businessId,
				AccountId:           systemAccounts[models.AccountCodeExchangeGainOrLoss],
				BranchId:            branchId,
				TransactionDateTime: transactionTime,
				BaseCurrencyId:      baseCurrencyId,
				BaseCredit:          decimal.NewFromInt(0),
				BaseDebit:           gainLossAmount.Abs(),
				ForeignCurrencyId:   foreignCurrencyId,
				ForeignDebit:        decimal.NewFromInt(0),
				ForeignCredit:       decimal.NewFromInt(0),
				ExchangeRate:        exchangeRate,
				RealisedAmount:      baseAmount,
			}
			accountIds = append(accountIds, systemAccounts[models.AccountCodeExchangeGainOrLoss])
			accTransactions = append(accTransactions, exchangeGainLoss)
		}
	}

	accJournal := models.AccountJournal{
		BusinessId:          businessId,
		BranchId:            customerCreditInvoice.BranchId,
		TransactionDateTime: customerCreditInvoice.CreditDate,
		TransactionNumber:   strconv.Itoa(customerCreditInvoice.ID),
		TransactionDetails:  "",
		ReferenceNumber:     customerCreditInvoice.InvoiceNumber,
		ReferenceId:         customerCreditInvoice.ID,
		ReferenceType:       models.AccountReferenceTypeCustomerAdvanceApplied,
		CustomerId:          customerCreditInvoice.CustomerId,
		AccountTransactions: accTransactions,
	}

	err = tx.Create(&accJournal).Error
	if err != nil {
		config.LogError(logger, "CustomerAdvanceAppliedWorkflow.go", "CreateCustomerAdvanceApplied", "CreateAccountJournal", accJournal, err)
		return 0, nil, 0, err
	}
	return accJournal.ID, accountIds, foreignCurrencyId, nil
}

func DeleteCustomerAdvanceApplied(tx *gorm.DB, logger *logrus.Logger, recordId int, recordType string, businessId string, business models.Business, oldCustomerCreditInvoice models.CustomerCreditInvoice) (int, []int, int, error) {

	foreignCurrencyId := oldCustomerCreditInvoice.CurrencyId
	accountJournal, _, accountIds, err := GetExistingAccountJournal(tx, oldCustomerCreditInvoice.ID, models.AccountReferenceTypeCustomerAdvanceApplied)
	if err != nil {
		config.LogError(logger, "CustomerAdvanceAppliedWorkflow.go", "DeleteCustomerAdvanceApplied", "GetExistingAccountJournal", oldCustomerCreditInvoice, err)
		return 0, nil, 0, err
	}

	// Phase 1: do not delete posted journals; create a reversal journal instead.
	reversalID, err := ReverseAccountJournal(tx, accountJournal, ReversalReasonCustomerAdvanceAppliedVoidUpdate)
	if err != nil {
		config.LogError(logger, "CustomerAdvanceAppliedWorkflow.go", "DeleteCustomerAdvanceApplied", "ReverseAccountJournal", accountJournal, err)
		return 0, nil, 0, err
	}
	return reversalID, accountIds, foreignCurrencyId, nil
}
