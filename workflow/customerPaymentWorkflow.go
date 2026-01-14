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

func ProcessCustomerPaymentWorkflow(tx *gorm.DB, logger *logrus.Logger, msg config.PubSubMessage) error {

	var accountJournalId int
	var accountIds []int
	var foreignCurrencyId int
	business, err := models.GetBusinessById2(tx, msg.BusinessId)
	if err != nil {
		config.LogError(logger, "CustomerPaymentWorkflow.go", "ProcessCustomerPaymentWorkflow", "GetBusiness", msg.BusinessId, err)
		return err
	}
	if msg.Action == string(models.PubSubMessageActionCreate) {

		var customerPayment models.CustomerPayment
		err := json.Unmarshal([]byte(msg.NewObj), &customerPayment)
		if err != nil {
			config.LogError(logger, "CustomerPaymentWorkflow.go", "ProcessCustomerPaymentWorkflow > Create", "Unmarshal msg.NewObj", msg.NewObj, err)
			return err
		}
		accountJournalId, accountIds, foreignCurrencyId, err = CreateCustomerPayment(tx, logger, msg.ID, msg.ReferenceType, msg.BusinessId, *business, customerPayment)
		if err != nil {
			config.LogError(logger, "CustomerPaymentWorkflow.go", "ProcessCustomerPaymentWorkflow > Create", "CreateCustomerPayment", nil, err)
			return err
		}
		err = UpdateBalances(tx, logger, msg.BusinessId, business.BaseCurrencyId, customerPayment.BranchId, accountIds, customerPayment.PaymentDate, foreignCurrencyId)
		if err != nil {
			config.LogError(logger, "CustomerPaymentWorkflow.go", "ProcessCustomerPaymentWorkflow > Create", "UpdateBalances", customerPayment, err)
			return err
		}
		err = UpdateBankBalances(tx, business.BaseCurrencyId, customerPayment.BranchId, accountIds, customerPayment.PaymentDate)
		if err != nil {
			config.LogError(logger, "CustomerPaymentWorkflow.go", "ProcessCustomerPaymentWorkflow > Create", "UpdateBankBalances", customerPayment, err)
			return err
		}

	} else if msg.Action == string(models.PubSubMessageActionUpdate) {

		var oldForeignCurrencyId int
		var oldAccountIds []int
		var customerPayment models.CustomerPayment
		err := json.Unmarshal([]byte(msg.NewObj), &customerPayment)
		if err != nil {
			config.LogError(logger, "CustomerPaymentWorkflow.go", "ProcessCustomerPaymentWorkflow > Update", "Unmarshal msg.NewObj", msg.NewObj, err)
			return err
		}
		var oldCustomerPayment models.CustomerPayment
		err = json.Unmarshal([]byte(msg.OldObj), &oldCustomerPayment)
		if err != nil {
			config.LogError(logger, "CustomerPaymentWorkflow.go", "ProcessCustomerPaymentWorkflow > Update", "Unmarshal msg.OldObj", msg.OldObj, err)
			return err
		}
		_, oldAccountIds, oldForeignCurrencyId, err = DeleteCustomerPayment(tx, logger, msg.ID, msg.ReferenceType, msg.BusinessId, *business, oldCustomerPayment)
		if err != nil {
			config.LogError(logger, "CustomerPaymentWorkflow.go", "ProcessCustomerPaymentWorkflow > Update", "DeleteCustomerPayment", nil, err)
			return err
		}
		accountJournalId, accountIds, foreignCurrencyId, err = CreateCustomerPayment(tx, logger, msg.ID, msg.ReferenceType, msg.BusinessId, *business, customerPayment)
		if err != nil {
			config.LogError(logger, "CustomerPaymentWorkflow.go", "ProcessCustomerPaymentWorkflow > Update", "CreateCustomerPayment", nil, err)
			return err
		}
		if oldCustomerPayment.BranchId != customerPayment.BranchId || oldCustomerPayment.PaymentDate != customerPayment.PaymentDate || oldForeignCurrencyId != foreignCurrencyId {
			err = UpdateBalances(tx, logger, msg.BusinessId, business.BaseCurrencyId, oldCustomerPayment.BranchId, oldAccountIds, oldCustomerPayment.PaymentDate, oldForeignCurrencyId)
			if err != nil {
				config.LogError(logger, "CustomerPaymentWorkflow.go", "ProcessCustomerPaymentWorkflow > Update", "UpdateBalances Old", oldCustomerPayment, err)
				return err
			}
			err = UpdateBankBalances(tx, business.BaseCurrencyId, oldCustomerPayment.BranchId, oldAccountIds, oldCustomerPayment.PaymentDate)
			if err != nil {
				config.LogError(logger, "CustomerPaymentWorkflow.go", "ProcessCustomerPaymentWorkflow > Update", "UpdateBankBalances Old", oldCustomerPayment, err)
				return err
			}
		} else {
			accountIds = utils.MergeIntSlices(accountIds, oldAccountIds)
		}
		err = UpdateBalances(tx, logger, msg.BusinessId, business.BaseCurrencyId, customerPayment.BranchId, accountIds, customerPayment.PaymentDate, foreignCurrencyId)
		if err != nil {
			config.LogError(logger, "CustomerPaymentWorkflow.go", "ProcessCustomerPaymentWorkflow > Update", "UpdateBalances", customerPayment, err)
			return err
		}
		err = UpdateBankBalances(tx, business.BaseCurrencyId, customerPayment.BranchId, oldAccountIds, customerPayment.PaymentDate)
		if err != nil {
			config.LogError(logger, "CustomerPaymentWorkflow.go", "ProcessCustomerPaymentWorkflow > Update", "UpdateBankBalances", customerPayment, err)
			return err
		}
	} else if msg.Action == string(models.PubSubMessageActionDelete) {

		var oldCustomerPayment models.CustomerPayment
		err = json.Unmarshal([]byte(msg.OldObj), &oldCustomerPayment)
		if err != nil {
			config.LogError(logger, "CustomerPaymentWorkflow.go", "ProcessCustomerPaymentWorkflow > Delete", "Unmarshal msg.OldObj", msg.OldObj, err)
			return err
		}
		accountJournalId, accountIds, foreignCurrencyId, err = DeleteCustomerPayment(tx, logger, msg.ID, msg.ReferenceType, msg.BusinessId, *business, oldCustomerPayment)
		if err != nil {
			config.LogError(logger, "CustomerPaymentWorkflow.go", "ProcessCustomerPaymentWorkflow > Delete", "DeleteCustomerPayment", nil, err)
			return err
		}
		err = UpdateBalances(tx, logger, msg.BusinessId, business.BaseCurrencyId, oldCustomerPayment.BranchId, accountIds, oldCustomerPayment.PaymentDate, foreignCurrencyId)
		if err != nil {
			config.LogError(logger, "CustomerPaymentWorkflow.go", "ProcessCustomerPaymentWorkflow > Delete", "UpdateBalances", oldCustomerPayment, err)
			return err
		}
		err = UpdateBankBalances(tx, business.BaseCurrencyId, oldCustomerPayment.BranchId, accountIds, oldCustomerPayment.PaymentDate)
		if err != nil {
			config.LogError(logger, "CustomerPaymentWorkflow.go", "ProcessCustomerPaymentWorkflow > Delete", "UpdateBankBalances", oldCustomerPayment, err)
			return err
		}
	}
	err = tx.Model(&models.PubSubMessageRecord{}).Where("id=?", msg.ID).Updates(map[string]interface{}{"account_journal_id": accountJournalId, "is_processed": true}).Error
	if err != nil {
		config.LogError(logger, "CustomerPaymentWorkflow.go", "ProcessCustomerPaymentWorkflow", "UpdatePubSubMessageRecord", accountJournalId, err)
		return err
	}
	return nil
}

func CreateCustomerPayment(tx *gorm.DB, logger *logrus.Logger, recordId int, recordType string, businessId string, business models.Business, customerPayment models.CustomerPayment) (int, []int, int, error) {

	systemAccounts, err := models.GetSystemAccounts(businessId)
	if err != nil {
		config.LogError(logger, "CustomerPaymentWorkflow.go", "CreateCustomerPayment", "GetSystemAccounts", businessId, err)
		return 0, nil, 0, err
	}

	transactionTime := customerPayment.PaymentDate
	branchId := customerPayment.BranchId
	baseCurrencyId := business.BaseCurrencyId
	foreignCurrencyId := customerPayment.CurrencyId
	paymentCurrencyId := customerPayment.CurrencyId

	invoiceAmount := decimal.NewFromInt(0)
	baseInvoiceAmount := decimal.NewFromInt(0)
	baseAmount := decimal.NewFromInt(0)
	foreignAmount := decimal.NewFromInt(0)
	baseBankCharges := customerPayment.BankCharges
	foreignBankCharges := decimal.NewFromInt(0)
	baseTotalAmount := decimal.NewFromInt(0)
	foreignTotalAmount := decimal.NewFromInt(0)
	exchangeRate := customerPayment.ExchangeRate

	depositAccount, err := GetAccount(tx, customerPayment.DepositAccountId)
	if err != nil {
		config.LogError(logger, "CustomerPaymentWorkflow.go", "CreateCustomerPayment", "GetDepositAccount", customerPayment.DepositAccountId, err)
		return 0, nil, 0, err
	}

	if baseCurrencyId != foreignCurrencyId {
		for _, paidInvoice := range customerPayment.PaidInvoices {
			var invoice models.SalesInvoice
			err = tx.First(&invoice, paidInvoice.InvoiceId).Error
			if err != nil {
				config.LogError(logger, "CustomerPaymentWorkflow.go", "CreateCustomerPayment", "GetPaidInvoice", paidInvoice.InvoiceId, err)
				return 0, nil, 0, err
			}
			invoiceAmount = invoiceAmount.Add(paidInvoice.PaidAmount)
			baseInvoiceAmount = baseInvoiceAmount.Add(paidInvoice.PaidAmount.Mul(invoice.ExchangeRate))
			baseAmount = baseAmount.Add(paidInvoice.PaidAmount.Mul(exchangeRate))
			foreignAmount = foreignAmount.Add(paidInvoice.PaidAmount)
		}
		foreignCurrencyId = paymentCurrencyId
	} else {
		for _, paidInvoice := range customerPayment.PaidInvoices {
			invoiceAmount = invoiceAmount.Add(paidInvoice.PaidAmount)
			baseInvoiceAmount = baseInvoiceAmount.Add(paidInvoice.PaidAmount)
			baseAmount = baseAmount.Add(paidInvoice.PaidAmount)
		}
	}

	depositAccountCurrencyId := 0
	if depositAccount.CurrencyId == 0 || depositAccount.CurrencyId == baseCurrencyId { // base-currency account
		if baseCurrencyId != paymentCurrencyId {
			// foreignTotalAmount = invoiceAmount.Add(customerPayment.BankCharges)
			foreignTotalAmount = invoiceAmount.Sub(customerPayment.BankCharges)
			baseTotalAmount = foreignTotalAmount.Mul(exchangeRate)
			foreignBankCharges = customerPayment.BankCharges
			baseBankCharges = foreignBankCharges.Mul(exchangeRate)
			depositAccountCurrencyId = paymentCurrencyId
			foreignCurrencyId = paymentCurrencyId
		} else {
			// baseTotalAmount = invoiceAmount.Add(customerPayment.BankCharges)
			baseTotalAmount = invoiceAmount.Sub(customerPayment.BankCharges)
			baseBankCharges = customerPayment.BankCharges
			depositAccountCurrencyId = baseCurrencyId
		}
	} else { // foreign currency account
		if baseCurrencyId != paymentCurrencyId {
			// foreignTotalAmount = invoiceAmount.Add(customerPayment.BankCharges)
			foreignTotalAmount = invoiceAmount.Sub(customerPayment.BankCharges)
			baseTotalAmount = foreignTotalAmount.Mul(exchangeRate)
			foreignBankCharges = customerPayment.BankCharges
			baseBankCharges = foreignBankCharges.Mul(exchangeRate)
			depositAccountCurrencyId = paymentCurrencyId
			foreignCurrencyId = paymentCurrencyId
		} else {
			if exchangeRate.IsZero() {
				// baseTotalAmount = invoiceAmount.Add(customerPayment.BankCharges)
				baseTotalAmount = invoiceAmount.Sub(customerPayment.BankCharges)
				foreignTotalAmount = decimal.NewFromInt(0)
				baseBankCharges = customerPayment.BankCharges
				foreignBankCharges = decimal.NewFromInt(0)
				depositAccountCurrencyId = depositAccount.CurrencyId
				foreignCurrencyId = depositAccount.CurrencyId
			} else {
				// baseTotalAmount = invoiceAmount.Add(customerPayment.BankCharges)
				baseTotalAmount = invoiceAmount.Sub(customerPayment.BankCharges)
				foreignTotalAmount = baseTotalAmount.DivRound(exchangeRate, 4)
				baseBankCharges = customerPayment.BankCharges
				foreignBankCharges = baseBankCharges.DivRound(exchangeRate, 4)
				depositAccountCurrencyId = depositAccount.CurrencyId
				foreignCurrencyId = depositAccount.CurrencyId
			}
		}
	}

	accountIds := make([]int, 0)
	accTransactions := make([]models.AccountTransaction, 0)

	// Insert into banking transaction if deposit account is cash or bank
	depositAccountInfo, err := GetAccount(tx, customerPayment.DepositAccountId)
	if err != nil {
		config.LogError(logger, "CustomerPaymentWorkflow.go", "CreateCustomerPayment", "GetDepositAccount", customerPayment.DepositAccountId, err)
		return 0, nil, 0, err
	}
	bankingTransactionId := 0
	if depositAccountInfo.DetailType == models.AccountDetailTypeCash ||
		depositAccountInfo.DetailType == models.AccountDetailTypeBank {

		bankingTransaction := models.BankingTransaction{
			BusinessId:        businessId,
			BranchId:          customerPayment.BranchId,
			FromAccountId:     systemAccounts[models.AccountCodeAccountsReceivable],
			FromAccountAmount: invoiceAmount,
			ToAccountId:       customerPayment.DepositAccountId,
			ToAccountAmount:   invoiceAmount.Sub(customerPayment.BankCharges),
			CustomerId:        customerPayment.CustomerId,
			TransactionDate:   customerPayment.PaymentDate,
			TransactionId:     customerPayment.ID,
			TransactionNumber: customerPayment.PaymentNumber,
			TransactionType:   models.BankingTransactionTypeCustomerPayment,
			ExchangeRate:      customerPayment.ExchangeRate,
			CurrencyId:        customerPayment.CurrencyId,
			Amount:            invoiceAmount,
			BankCharges:       customerPayment.BankCharges,
			ReferenceNumber:   customerPayment.ReferenceNumber,
			Description:       customerPayment.Notes,
		}
		err = tx.Create(&bankingTransaction).Error
		if err != nil {
			config.LogError(logger, "CustomerPaymentWorkflow.go", "CreateCustomerPayment", "CreateBankingTransaction", bankingTransaction, err)
			return 0, nil, 0, err
		}
		bankingTransactionId = bankingTransaction.ID
	}

	accountsReceivable := models.AccountTransaction{
		BusinessId:           businessId,
		AccountId:            systemAccounts[models.AccountCodeAccountsReceivable],
		BranchId:             branchId,
		TransactionDateTime:  transactionTime,
		BaseCurrencyId:       baseCurrencyId,
		BaseCredit:           baseInvoiceAmount,
		BaseDebit:            decimal.NewFromInt(0),
		ForeignCurrencyId:    paymentCurrencyId,
		ForeignCredit:        foreignAmount,
		ForeignDebit:         decimal.NewFromInt(0),
		ExchangeRate:         exchangeRate,
		BankingTransactionId: bankingTransactionId,
	}
	accTransactions = append(accTransactions, accountsReceivable)
	accountIds = append(accountIds, systemAccounts[models.AccountCodeAccountsReceivable])

	depositAccountTransact := models.AccountTransaction{
		BusinessId:           businessId,
		AccountId:            customerPayment.DepositAccountId,
		BranchId:             branchId,
		TransactionDateTime:  transactionTime,
		BaseCurrencyId:       baseCurrencyId,
		BaseCredit:           decimal.NewFromInt(0),
		BaseDebit:            baseTotalAmount,
		ForeignCurrencyId:    depositAccountCurrencyId,
		ForeignCredit:        decimal.NewFromInt(0),
		ForeignDebit:         foreignTotalAmount,
		ExchangeRate:         exchangeRate,
		BankingTransactionId: bankingTransactionId,
	}
	accTransactions = append(accTransactions, depositAccountTransact)
	accountIds = append(accountIds, customerPayment.DepositAccountId)

	if !baseInvoiceAmount.Equals(baseAmount) {
		gainLossAmount := baseAmount.Sub(baseInvoiceAmount)
		if gainLossAmount.IsPositive() {
			exchangeGainLoss := models.AccountTransaction{
				BusinessId:           businessId,
				AccountId:            systemAccounts[models.AccountCodeExchangeGainOrLoss],
				BranchId:             branchId,
				TransactionDateTime:  transactionTime,
				BaseCurrencyId:       baseCurrencyId,
				BaseCredit:           gainLossAmount.Abs(),
				BaseDebit:            decimal.NewFromInt(0),
				ForeignCurrencyId:    paymentCurrencyId,
				ForeignDebit:         decimal.NewFromInt(0),
				ForeignCredit:        decimal.NewFromInt(0),
				ExchangeRate:         exchangeRate,
				RealisedAmount:       baseAmount,
				BankingTransactionId: bankingTransactionId,
			}
			accountIds = append(accountIds, systemAccounts[models.AccountCodeExchangeGainOrLoss])
			accTransactions = append(accTransactions, exchangeGainLoss)
		} else {
			exchangeGainLoss := models.AccountTransaction{
				BusinessId:           businessId,
				AccountId:            systemAccounts[models.AccountCodeExchangeGainOrLoss],
				BranchId:             branchId,
				TransactionDateTime:  transactionTime,
				BaseCurrencyId:       baseCurrencyId,
				BaseCredit:           decimal.NewFromInt(0),
				BaseDebit:            gainLossAmount.Abs(),
				ForeignCurrencyId:    paymentCurrencyId,
				ForeignDebit:         decimal.NewFromInt(0),
				ForeignCredit:        decimal.NewFromInt(0),
				ExchangeRate:         exchangeRate,
				RealisedAmount:       baseAmount,
				BankingTransactionId: bankingTransactionId,
			}
			accountIds = append(accountIds, systemAccounts[models.AccountCodeExchangeGainOrLoss])
			accTransactions = append(accTransactions, exchangeGainLoss)
		}
	}

	if !baseBankCharges.IsZero() {
		bankCharges := models.AccountTransaction{
			BusinessId:           businessId,
			AccountId:            systemAccounts[models.AccountCodeBankFeesAndCharges],
			BranchId:             branchId,
			TransactionDateTime:  transactionTime,
			BaseCurrencyId:       baseCurrencyId,
			BaseDebit:            baseBankCharges,
			BaseCredit:           decimal.NewFromInt(0),
			ForeignCurrencyId:    depositAccountCurrencyId,
			ForeignDebit:         foreignBankCharges,
			ForeignCredit:        decimal.NewFromInt(0),
			ExchangeRate:         exchangeRate,
			BankingTransactionId: bankingTransactionId,
		}
		accountIds = append(accountIds, systemAccounts[models.AccountCodeBankFeesAndCharges])
		accTransactions = append(accTransactions, bankCharges)
	}

	accJournal := models.AccountJournal{
		BusinessId:          businessId,
		BranchId:            customerPayment.BranchId,
		TransactionDateTime: customerPayment.PaymentDate,
		TransactionNumber:   strconv.Itoa(customerPayment.ID),
		CustomerId:          customerPayment.CustomerId,
		TransactionDetails:  customerPayment.Notes,
		ReferenceNumber:     customerPayment.ReferenceNumber,
		ReferenceId:         customerPayment.ID,
		ReferenceType:       models.AccountReferenceTypeCustomerPayment,
		AccountTransactions: accTransactions,
	}

	err = tx.Create(&accJournal).Error
	if err != nil {
		config.LogError(logger, "CustomerPaymentWorkflow.go", "CreateCustomerPayment", "CreateAccountJournal", accJournal, err)
		return 0, nil, 0, err
	}

	return accJournal.ID, accountIds, foreignCurrencyId, nil
}

func DeleteCustomerPayment(tx *gorm.DB, logger *logrus.Logger, recordId int, recordType string, businessId string, business models.Business, oldCustomerPayment models.CustomerPayment) (int, []int, int, error) {

	foreignCurrencyId := oldCustomerPayment.CurrencyId
	accountJournal, _, accountIds, err := GetExistingAccountJournal(tx, oldCustomerPayment.ID, models.AccountReferenceTypeCustomerPayment)
	if err != nil {
		config.LogError(logger, "CustomerPaymentWorkflow.go", "DeleteCustomerPayment", "GetExistingAccountJournal", oldCustomerPayment, err)
		return 0, nil, 0, err
	}

	// Phase 1: do not delete posted journals; create a reversal journal instead.
	reversalID, err := ReverseAccountJournal(tx, accountJournal, ReversalReasonCustomerPaymentVoidUpdate)
	if err != nil {
		config.LogError(logger, "CustomerPaymentWorkflow.go", "DeleteCustomerPayment", "ReverseAccountJournal", accountJournal, err)
		return 0, nil, 0, err
	}

	err = tx.Where("transaction_id = ? AND transaction_type = ?", oldCustomerPayment.ID, models.BankingTransactionTypeCustomerPayment).Delete(&models.BankingTransaction{}).Error
	if err != nil {
		config.LogError(logger, "CustomerPaymentWorkflow.go", "DeleteCustomerPayment", "DeleteBankingTransaction", oldCustomerPayment, err)
		return 0, nil, 0, err
	}

	return reversalID, accountIds, foreignCurrencyId, nil
}
