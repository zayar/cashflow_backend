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

func ProcessBankingTransactionWorkflow(tx *gorm.DB, logger *logrus.Logger, msg config.PubSubMessage) error {

	var accountJournalId int
	var accountIds []int
	var foreignCurrencyId int

	business, err := models.GetBusinessById2(tx, msg.BusinessId)
	if err != nil {
		config.LogError(logger, "BankingTransactionWorkflow.go", "ProcessBankingTransactionWorkflow", "GetBusiness", msg.BusinessId, err)
		return err
	}
	if msg.Action == string(models.PubSubMessageActionCreate) {

		var bankTransaction models.BankingTransaction
		err := json.Unmarshal([]byte(msg.NewObj), &bankTransaction)
		if err != nil {
			config.LogError(logger, "BankingTransactionWorkflow.go", "ProcessBankingTransactionWorkflow > Create", "Unmarshal msg.NewObj", msg.NewObj, err)
			return err
		}
		accountJournalId, accountIds, foreignCurrencyId, err = CreateBankingTransaction(tx, logger, msg.ID, msg.ReferenceType, msg.BusinessId, *business, bankTransaction)
		if err != nil {
			config.LogError(logger, "BankingTransactionWorkflow.go", "ProcessBankingTransactionWorkflow > Create", "CreateBankingTransaction", nil, err)
			return err
		}
		err = UpdateBalances(tx, logger, msg.BusinessId, business.BaseCurrencyId, bankTransaction.BranchId, accountIds, bankTransaction.TransactionDate, foreignCurrencyId)
		if err != nil {
			config.LogError(logger, "BankingTransactionWorkflow.go", "ProcessBankingTransaction > Create", "UpdateBalances", bankTransaction, err)
			return err
		}
		err = UpdateBankBalances(tx, business.BaseCurrencyId, bankTransaction.BranchId, accountIds, bankTransaction.TransactionDate)
		if err != nil {
			config.LogError(logger, "BankingTransactionWorkflow.go", "ProcessBankingTransaction > Create", "UpdateBankBalances", bankTransaction, err)
			return err
		}

	} else if msg.Action == string(models.PubSubMessageActionUpdate) {

		var oldForeignCurrencyId int
		var oldAccountIds []int
		var bankTransaction models.BankingTransaction

		err := json.Unmarshal([]byte(msg.NewObj), &bankTransaction)
		if err != nil {
			config.LogError(logger, "BankingTransactionWorkflow.go", "ProcessBankingTransaction > Update", "Unmarshal msg.NewObj", msg.NewObj, err)
			return err
		}
		var oldBankTransaction models.BankingTransaction
		err = json.Unmarshal([]byte(msg.OldObj), &oldBankTransaction)
		if err != nil {
			config.LogError(logger, "BankingTransactionWorkflow.go", "ProcessBankingTransaction > Update", "Unmarshal msg.OldObj", msg.OldObj, err)
			return err
		}
		_, oldAccountIds, oldForeignCurrencyId, err = DeleteBankingTransaction(tx, logger, msg.ID, msg.ReferenceType, msg.BusinessId, *business, oldBankTransaction, true)
		if err != nil {
			config.LogError(logger, "BankingTransactionWorkflow.go", "ProcessBankingTransaction > Update", "DeleteBankingTransaction", nil, err)
			return err
		}
		accountJournalId, accountIds, foreignCurrencyId, err = CreateBankingTransaction(tx, logger, msg.ID, msg.ReferenceType, msg.BusinessId, *business, bankTransaction)
		if err != nil {
			config.LogError(logger, "BankingTransactionWorkflow.go", "ProcessBankingTransaction > Update", "CreateBankingTransaction", nil, err)
			return err
		}
		if oldBankTransaction.BranchId != bankTransaction.BranchId || oldBankTransaction.TransactionDate != bankTransaction.TransactionDate || oldForeignCurrencyId != foreignCurrencyId {
			err = UpdateBalances(tx, logger, msg.BusinessId, business.BaseCurrencyId, oldBankTransaction.BranchId, oldAccountIds, oldBankTransaction.TransactionDate, oldForeignCurrencyId)
			if err != nil {
				config.LogError(logger, "BankingTransactionWorkflow.go", "ProcessBankingTransaction > Update", "UpdateBalances Old", oldBankTransaction, err)
				return err
			}
			err = UpdateBankBalances(tx, business.BaseCurrencyId, oldBankTransaction.BranchId, oldAccountIds, oldBankTransaction.TransactionDate)
			if err != nil {
				config.LogError(logger, "BankingTransactionWorkflow.go", "ProcessBankingTransaction > Update", "UpdateBankBalances Old", oldBankTransaction, err)
				return err
			}
		} else {
			accountIds = utils.MergeIntSlices(accountIds, oldAccountIds)
		}
		err = UpdateBalances(tx, logger, msg.BusinessId, business.BaseCurrencyId, bankTransaction.BranchId, accountIds, bankTransaction.TransactionDate, foreignCurrencyId)
		if err != nil {
			config.LogError(logger, "BankingTransactionWorkflow.go", "ProcessBankingTransaction > Update", "UpdateBalances", bankTransaction, err)
			return err
		}
		err = UpdateBankBalances(tx, business.BaseCurrencyId, bankTransaction.BranchId, accountIds, bankTransaction.TransactionDate)
		if err != nil {
			config.LogError(logger, "BankingTransactionWorkflow.go", "ProcessBankingTransaction > Update", "UpdateBankBalances", bankTransaction, err)
			return err
		}
	} else if msg.Action == string(models.PubSubMessageActionDelete) {

		var oldBankTransaction models.BankingTransaction
		err = json.Unmarshal([]byte(msg.OldObj), &oldBankTransaction)
		if err != nil {
			return err
		}
		accountJournalId, accountIds, foreignCurrencyId, err = DeleteBankingTransaction(tx, logger, msg.ID, msg.ReferenceType, msg.BusinessId, *business, oldBankTransaction, false)
		if err != nil {
			config.LogError(logger, "BankingTransactionWorkflow.go", "ProcessBankingTransaction > Delete", "DeleteBankingTransaction", nil, err)
			return err
		}
		err = UpdateBalances(tx, logger, msg.BusinessId, business.BaseCurrencyId, oldBankTransaction.BranchId, accountIds, oldBankTransaction.TransactionDate, foreignCurrencyId)
		if err != nil {
			config.LogError(logger, "BankingTransactionWorkflow.go", "ProcessBankingTransaction > Delete", "UpdateBalances", oldBankTransaction, err)
			return err
		}
		err = UpdateBankBalances(tx, business.BaseCurrencyId, oldBankTransaction.BranchId, accountIds, oldBankTransaction.TransactionDate)
		if err != nil {
			config.LogError(logger, "BankingTransactionWorkflow.go", "ProcessBankingTransaction > Delete", "UpdateBankBalances", oldBankTransaction, err)
			return err
		}
	}

	err = tx.Model(&models.PubSubMessageRecord{}).Where("id=?", msg.ID).Updates(map[string]interface{}{"account_journal_id": accountJournalId, "is_processed": true}).Error
	if err != nil {
		config.LogError(logger, "BankingTransactionWorkflow.go", "ProcessBankingTransaction", "UpdatePubSubMessageRecord", accountJournalId, err)
		return err
	}
	return nil
}

func CreateBankingTransaction(tx *gorm.DB, logger *logrus.Logger, recordId int, recordType string, businessId string, business models.Business, bankTransaction models.BankingTransaction) (int, []int, int, error) {

	accountIds := []int{bankTransaction.FromAccountId, bankTransaction.ToAccountId}
	transactionTime := bankTransaction.TransactionDate
	branchId := bankTransaction.BranchId
	baseCurrencyId := business.BaseCurrencyId
	transferCurrencyId := bankTransaction.CurrencyId

	baseBankCharges := decimal.NewFromInt(0)
	foreignBankCharges := decimal.NewFromInt(0)
	baseFromBankingTransactionAmount := decimal.NewFromInt(0)
	foreignFromBankingTransactionAmount := decimal.NewFromInt(0)
	baseToBankingTransactionAmount := decimal.NewFromInt(0)
	foreignToBankingTransactionAmount := decimal.NewFromInt(0)
	exchangeRate := bankTransaction.ExchangeRate
	fromAccountCurrencyId := 0
	toAccountCurrencyId := 0

	fromAccount, err := GetAccount(tx, bankTransaction.FromAccountId)
	if err != nil {
		config.LogError(logger, "BankingTransactionWorkflow.go", "CreateBankingTransaction", "GetFromAccount", bankTransaction, err)
		return 0, nil, 0, err
	}

	toAccount, err := GetAccount(tx, bankTransaction.ToAccountId)
	if err != nil {
		config.LogError(logger, "BankingTransactionWorkflow.go", "CreateBankingTransaction", "GetToAccount", bankTransaction, err)
		return 0, nil, 0, err
	}

	foreignCurrencyId := business.BaseCurrencyId
	if fromAccount.CurrencyId == 0 || fromAccount.CurrencyId == baseCurrencyId { // base-currency account
		if baseCurrencyId != transferCurrencyId {
			if models.AccountReferenceType(recordType) == models.AccountReferenceTypeAccountTransfer ||
				models.AccountReferenceType(recordType) == models.AccountReferenceTypeAccountDeposit {

				foreignFromBankingTransactionAmount = bankTransaction.Amount.Add(bankTransaction.BankCharges)
			} else {
				foreignFromBankingTransactionAmount = bankTransaction.Amount
			}
			baseFromBankingTransactionAmount = foreignFromBankingTransactionAmount.Mul(exchangeRate)
			foreignBankCharges = bankTransaction.BankCharges
			baseBankCharges = foreignBankCharges.Mul(exchangeRate)
			fromAccountCurrencyId = transferCurrencyId
			foreignCurrencyId = transferCurrencyId
		} else {
			if models.AccountReferenceType(recordType) == models.AccountReferenceTypeAccountTransfer ||
				models.AccountReferenceType(recordType) == models.AccountReferenceTypeAccountDeposit {

				baseFromBankingTransactionAmount = bankTransaction.Amount.Add(bankTransaction.BankCharges)
			} else {
				baseFromBankingTransactionAmount = bankTransaction.Amount
			}
			baseBankCharges = bankTransaction.BankCharges
			fromAccountCurrencyId = baseCurrencyId
		}
	} else { // foreign currency account
		if baseCurrencyId != transferCurrencyId {
			if models.AccountReferenceType(recordType) == models.AccountReferenceTypeAccountTransfer ||
				models.AccountReferenceType(recordType) == models.AccountReferenceTypeAccountDeposit {

				foreignFromBankingTransactionAmount = bankTransaction.Amount.Add(bankTransaction.BankCharges)
			} else {
				foreignFromBankingTransactionAmount = bankTransaction.Amount
			}
			baseFromBankingTransactionAmount = foreignFromBankingTransactionAmount.Mul(exchangeRate)
			foreignBankCharges = bankTransaction.BankCharges
			baseBankCharges = foreignBankCharges.Mul(exchangeRate)
			fromAccountCurrencyId = transferCurrencyId
			foreignCurrencyId = transferCurrencyId
		} else {
			if exchangeRate.IsZero() {
				if models.AccountReferenceType(recordType) == models.AccountReferenceTypeAccountTransfer ||
					models.AccountReferenceType(recordType) == models.AccountReferenceTypeAccountDeposit {

					baseFromBankingTransactionAmount = bankTransaction.Amount.Add(bankTransaction.BankCharges)
				} else {
					baseFromBankingTransactionAmount = bankTransaction.Amount
				}
				foreignFromBankingTransactionAmount = decimal.NewFromInt(0)
				baseBankCharges = bankTransaction.BankCharges
				foreignBankCharges = decimal.NewFromInt(0)
				fromAccountCurrencyId = fromAccount.CurrencyId
				foreignCurrencyId = fromAccount.CurrencyId
			} else {
				if models.AccountReferenceType(recordType) == models.AccountReferenceTypeAccountTransfer ||
					models.AccountReferenceType(recordType) == models.AccountReferenceTypeAccountDeposit {

					baseFromBankingTransactionAmount = bankTransaction.Amount.Add(bankTransaction.BankCharges)
				} else {
					baseFromBankingTransactionAmount = bankTransaction.Amount
				}
				foreignFromBankingTransactionAmount = baseFromBankingTransactionAmount.DivRound(exchangeRate, 4)
				baseBankCharges = bankTransaction.BankCharges
				foreignBankCharges = baseBankCharges.DivRound(exchangeRate, 4)
				fromAccountCurrencyId = fromAccount.CurrencyId
				foreignCurrencyId = fromAccount.CurrencyId
			}
		}
	}

	if toAccount.CurrencyId == 0 || toAccount.CurrencyId == baseCurrencyId {
		if baseCurrencyId != transferCurrencyId {
			if models.AccountReferenceType(recordType) == models.AccountReferenceTypeAccountTransfer ||
				models.AccountReferenceType(recordType) == models.AccountReferenceTypeAccountDeposit {

				foreignToBankingTransactionAmount = bankTransaction.Amount
			} else {
				foreignToBankingTransactionAmount = bankTransaction.Amount.Sub(bankTransaction.BankCharges)
			}
			baseToBankingTransactionAmount = foreignToBankingTransactionAmount.Mul(exchangeRate)
			toAccountCurrencyId = transferCurrencyId
			foreignCurrencyId = transferCurrencyId
		} else {
			if models.AccountReferenceType(recordType) == models.AccountReferenceTypeAccountTransfer ||
				models.AccountReferenceType(recordType) == models.AccountReferenceTypeAccountDeposit {

				baseToBankingTransactionAmount = bankTransaction.Amount
			} else {
				baseToBankingTransactionAmount = bankTransaction.Amount.Sub(bankTransaction.BankCharges)
			}
			toAccountCurrencyId = baseCurrencyId
		}
	} else {
		if baseCurrencyId != transferCurrencyId {
			if models.AccountReferenceType(recordType) == models.AccountReferenceTypeAccountTransfer ||
				models.AccountReferenceType(recordType) == models.AccountReferenceTypeAccountDeposit {

				foreignToBankingTransactionAmount = bankTransaction.Amount
			} else {
				foreignToBankingTransactionAmount = bankTransaction.Amount.Sub(bankTransaction.BankCharges)
			}
			baseToBankingTransactionAmount = foreignToBankingTransactionAmount.Mul(exchangeRate)
			toAccountCurrencyId = transferCurrencyId
			foreignCurrencyId = transferCurrencyId
		} else {
			if models.AccountReferenceType(recordType) == models.AccountReferenceTypeAccountTransfer ||
				models.AccountReferenceType(recordType) == models.AccountReferenceTypeAccountDeposit {

				baseToBankingTransactionAmount = bankTransaction.Amount
			} else {
				baseToBankingTransactionAmount = bankTransaction.Amount.Sub(bankTransaction.BankCharges)
			}
			foreignToBankingTransactionAmount = baseToBankingTransactionAmount.DivRound(exchangeRate, 4)
			toAccountCurrencyId = toAccount.CurrencyId
			foreignCurrencyId = toAccount.CurrencyId
		}
	}

	accTransactions := make([]models.AccountTransaction, 0)

	if models.AccountReferenceType(recordType) == models.AccountReferenceTypeSupplierCreditRefund ||
		models.AccountReferenceType(recordType) == models.AccountReferenceTypeSupplierAdvanceRefund {

		systemAccounts, err := models.GetSystemAccounts(businessId)
		if err != nil {
			config.LogError(logger, "CustomerAdvanceAppliedWorkflow.go", "CreateCustomerAdvanceApplied", "GetSystemAccounts", businessId, err)
			return 0, nil, 0, err
		}

		var refund models.Refund
		err = tx.First(&refund, bankTransaction.TransactionId).Error
		if err != nil {
			config.LogError(logger, "BankingTransactionWorkflow.go", "CreateBankingTransaction", "GetRefund", refund, err)
			return 0, nil, 0, err
		}
		if !refund.ExchangeRate.Equal(refund.ReferenceExchangeRate) && !refund.ReferenceExchangeRate.IsZero() {
			amount := foreignFromBankingTransactionAmount.Mul(refund.ExchangeRate)
			refAmount := foreignFromBankingTransactionAmount.Mul(refund.ReferenceExchangeRate)

			if refAmount.GreaterThanOrEqual(amount) {
				gainLossAmount := refAmount.Sub(amount)
				exchangeGainLoss := models.AccountTransaction{
					BusinessId:          businessId,
					AccountId:           systemAccounts[models.AccountCodeExchangeGainOrLoss],
					BranchId:            branchId,
					TransactionDateTime: transactionTime,
					BaseCurrencyId:      baseCurrencyId,
					BaseCredit:          decimal.NewFromInt(0),
					BaseDebit:           gainLossAmount,
					ForeignCurrencyId:   foreignCurrencyId,
					ForeignDebit:        decimal.NewFromInt(0),
					ForeignCredit:       decimal.NewFromInt(0),
					ExchangeRate:        exchangeRate,
					RealisedAmount:      amount,
				}
				accountIds = append(accountIds, systemAccounts[models.AccountCodeExchangeGainOrLoss])
				accTransactions = append(accTransactions, exchangeGainLoss)
			} else {
				gainLossAmount := amount.Sub(refAmount)
				exchangeGainLoss := models.AccountTransaction{
					BusinessId:          businessId,
					AccountId:           systemAccounts[models.AccountCodeExchangeGainOrLoss],
					BranchId:            branchId,
					TransactionDateTime: transactionTime,
					BaseCurrencyId:      baseCurrencyId,
					BaseCredit:          gainLossAmount,
					BaseDebit:           decimal.NewFromInt(0),
					ForeignCurrencyId:   foreignCurrencyId,
					ForeignDebit:        decimal.NewFromInt(0),
					ForeignCredit:       decimal.NewFromInt(0),
					ExchangeRate:        exchangeRate,
					RealisedAmount:      amount,
				}
				accountIds = append(accountIds, systemAccounts[models.AccountCodeExchangeGainOrLoss])
				accTransactions = append(accTransactions, exchangeGainLoss)
			}

			fromAccountTransact := models.AccountTransaction{
				BusinessId:           businessId,
				AccountId:            bankTransaction.FromAccountId,
				BranchId:             branchId,
				TransactionDateTime:  transactionTime,
				BaseCurrencyId:       baseCurrencyId,
				BaseDebit:            decimal.NewFromInt(0),
				BaseCredit:           refAmount,
				ForeignCurrencyId:    fromAccountCurrencyId,
				ForeignDebit:         decimal.NewFromInt(0),
				ForeignCredit:        foreignFromBankingTransactionAmount,
				ExchangeRate:         exchangeRate,
				BankingTransactionId: bankTransaction.ID,
			}
			accTransactions = append(accTransactions, fromAccountTransact)
		} else {
			fromAccountTransact := models.AccountTransaction{
				BusinessId:           businessId,
				AccountId:            bankTransaction.FromAccountId,
				BranchId:             branchId,
				TransactionDateTime:  transactionTime,
				BaseCurrencyId:       baseCurrencyId,
				BaseDebit:            decimal.NewFromInt(0),
				BaseCredit:           baseFromBankingTransactionAmount,
				ForeignCurrencyId:    fromAccountCurrencyId,
				ForeignDebit:         decimal.NewFromInt(0),
				ForeignCredit:        foreignFromBankingTransactionAmount,
				ExchangeRate:         exchangeRate,
				BankingTransactionId: bankTransaction.ID,
			}
			accTransactions = append(accTransactions, fromAccountTransact)
		}
	} else {
		fromAccountTransact := models.AccountTransaction{
			BusinessId:           businessId,
			AccountId:            bankTransaction.FromAccountId,
			BranchId:             branchId,
			TransactionDateTime:  transactionTime,
			BaseCurrencyId:       baseCurrencyId,
			BaseDebit:            decimal.NewFromInt(0),
			BaseCredit:           baseFromBankingTransactionAmount,
			ForeignCurrencyId:    fromAccountCurrencyId,
			ForeignDebit:         decimal.NewFromInt(0),
			ForeignCredit:        foreignFromBankingTransactionAmount,
			ExchangeRate:         exchangeRate,
			BankingTransactionId: bankTransaction.ID,
		}
		accTransactions = append(accTransactions, fromAccountTransact)
	}

	if models.AccountReferenceType(recordType) == models.AccountReferenceTypeCreditNoteRefund ||
		models.AccountReferenceType(recordType) == models.AccountReferenceTypeCustomerAdvanceRefund {

		systemAccounts, err := models.GetSystemAccounts(businessId)
		if err != nil {
			config.LogError(logger, "CustomerAdvanceAppliedWorkflow.go", "CreateCustomerAdvanceApplied", "GetSystemAccounts", businessId, err)
			return 0, nil, 0, err
		}

		var refund models.Refund
		err = tx.First(&refund, bankTransaction.TransactionId).Error
		if err != nil {
			config.LogError(logger, "BankingTransactionWorkflow.go", "CreateBankingTransaction", "GetRefund", refund, err)
			return 0, nil, 0, err
		}
		if !refund.ExchangeRate.Equal(refund.ReferenceExchangeRate) && !refund.ReferenceExchangeRate.IsZero() {
			amount := foreignToBankingTransactionAmount.Mul(refund.ExchangeRate)
			refAmount := foreignToBankingTransactionAmount.Mul(refund.ReferenceExchangeRate)

			if refAmount.GreaterThanOrEqual(amount) {
				gainLossAmount := refAmount.Sub(amount)
				exchangeGainLoss := models.AccountTransaction{
					BusinessId:          businessId,
					AccountId:           systemAccounts[models.AccountCodeExchangeGainOrLoss],
					BranchId:            branchId,
					TransactionDateTime: transactionTime,
					BaseCurrencyId:      baseCurrencyId,
					BaseDebit:           decimal.NewFromInt(0),
					BaseCredit:          gainLossAmount,
					ForeignCurrencyId:   foreignCurrencyId,
					ForeignDebit:        decimal.NewFromInt(0),
					ForeignCredit:       decimal.NewFromInt(0),
					ExchangeRate:        exchangeRate,
					RealisedAmount:      amount,
				}
				accountIds = append(accountIds, systemAccounts[models.AccountCodeExchangeGainOrLoss])
				accTransactions = append(accTransactions, exchangeGainLoss)
			} else {
				gainLossAmount := amount.Sub(refAmount)
				exchangeGainLoss := models.AccountTransaction{
					BusinessId:          businessId,
					AccountId:           systemAccounts[models.AccountCodeExchangeGainOrLoss],
					BranchId:            branchId,
					TransactionDateTime: transactionTime,
					BaseCurrencyId:      baseCurrencyId,
					BaseDebit:           gainLossAmount,
					BaseCredit:          decimal.NewFromInt(0),
					ForeignCurrencyId:   foreignCurrencyId,
					ForeignDebit:        decimal.NewFromInt(0),
					ForeignCredit:       decimal.NewFromInt(0),
					ExchangeRate:        exchangeRate,
					RealisedAmount:      amount,
				}
				accountIds = append(accountIds, systemAccounts[models.AccountCodeExchangeGainOrLoss])
				accTransactions = append(accTransactions, exchangeGainLoss)
			}

			toAccountTransact := models.AccountTransaction{
				BusinessId:           businessId,
				AccountId:            bankTransaction.ToAccountId,
				BranchId:             branchId,
				TransactionDateTime:  transactionTime,
				BaseCurrencyId:       baseCurrencyId,
				BaseDebit:            refAmount,
				BaseCredit:           decimal.NewFromInt(0),
				ForeignCurrencyId:    toAccountCurrencyId,
				ForeignDebit:         foreignToBankingTransactionAmount,
				ForeignCredit:        decimal.NewFromInt(0),
				ExchangeRate:         exchangeRate,
				BankingTransactionId: bankTransaction.ID,
			}
			accTransactions = append(accTransactions, toAccountTransact)
		} else {
			toAccountTransact := models.AccountTransaction{
				BusinessId:           businessId,
				AccountId:            bankTransaction.ToAccountId,
				BranchId:             branchId,
				TransactionDateTime:  transactionTime,
				BaseCurrencyId:       baseCurrencyId,
				BaseDebit:            baseToBankingTransactionAmount,
				BaseCredit:           decimal.NewFromInt(0),
				ForeignCurrencyId:    toAccountCurrencyId,
				ForeignDebit:         foreignToBankingTransactionAmount,
				ForeignCredit:        decimal.NewFromInt(0),
				ExchangeRate:         exchangeRate,
				BankingTransactionId: bankTransaction.ID,
			}
			accTransactions = append(accTransactions, toAccountTransact)
		}
	} else {
		toAccountTransact := models.AccountTransaction{
			BusinessId:           businessId,
			AccountId:            bankTransaction.ToAccountId,
			BranchId:             branchId,
			TransactionDateTime:  transactionTime,
			BaseCurrencyId:       baseCurrencyId,
			BaseDebit:            baseToBankingTransactionAmount,
			BaseCredit:           decimal.NewFromInt(0),
			ForeignCurrencyId:    toAccountCurrencyId,
			ForeignDebit:         foreignToBankingTransactionAmount,
			ForeignCredit:        decimal.NewFromInt(0),
			ExchangeRate:         exchangeRate,
			BankingTransactionId: bankTransaction.ID,
		}
		accTransactions = append(accTransactions, toAccountTransact)
	}

	if !baseBankCharges.IsZero() {
		systemAccounts, err := models.GetSystemAccounts(businessId)
		if err != nil {
			config.LogError(logger, "BankingTransactionWorkflow.go", "CreateBankingTransaction", "GetSystemAccounts", businessId, err)
			return 0, nil, 0, err
		}

		bankCharges := models.AccountTransaction{
			BusinessId:           businessId,
			AccountId:            systemAccounts[models.AccountCodeBankFeesAndCharges],
			BranchId:             branchId,
			TransactionDateTime:  transactionTime,
			BaseCurrencyId:       baseCurrencyId,
			BaseDebit:            baseBankCharges,
			BaseCredit:           decimal.NewFromInt(0),
			ForeignCurrencyId:    fromAccountCurrencyId,
			ForeignDebit:         foreignBankCharges,
			ForeignCredit:        decimal.NewFromInt(0),
			ExchangeRate:         exchangeRate,
			BankingTransactionId: bankTransaction.ID,
		}
		accountIds = append(accountIds, systemAccounts[models.AccountCodeBankFeesAndCharges])
		accTransactions = append(accTransactions, bankCharges)
	}

	accJournal := models.AccountJournal{
		BusinessId:          businessId,
		BranchId:            bankTransaction.BranchId,
		TransactionDateTime: bankTransaction.TransactionDate,
		TransactionNumber:   strconv.Itoa(bankTransaction.ID),
		TransactionDetails:  bankTransaction.Description,
		ReferenceNumber:     bankTransaction.ReferenceNumber,
		ReferenceId:         bankTransaction.ID,
		// ReferenceType:       models.AccountReferenceTypeBankingTransaction,
		ReferenceType:       models.AccountReferenceType(recordType),
		CustomerId:          bankTransaction.CustomerId,
		SupplierId:          bankTransaction.SupplierId,
		AccountTransactions: accTransactions,
	}

	if models.AccountReferenceType(recordType) == models.AccountReferenceTypeAdvanceSupplierPayment && bankTransaction.CreditAdvanceId == 0 {
		var advance models.SupplierCreditAdvance
		if toAccountCurrencyId != baseCurrencyId {
			advance = models.SupplierCreditAdvance{
				BusinessId:       businessId,
				Date:             bankTransaction.TransactionDate,
				BranchId:         bankTransaction.BranchId,
				SupplierId:       bankTransaction.SupplierId,
				Amount:           foreignToBankingTransactionAmount,
				RemainingBalance: foreignToBankingTransactionAmount,
				CurrencyId:       toAccountCurrencyId,
				ExchangeRate:     exchangeRate,
				CurrentStatus:    models.SupplierAdvanceStatusConfirmed,
			}
		} else {
			advance = models.SupplierCreditAdvance{
				BusinessId:       businessId,
				Date:             bankTransaction.TransactionDate,
				BranchId:         bankTransaction.BranchId,
				SupplierId:       bankTransaction.SupplierId,
				Amount:           baseToBankingTransactionAmount,
				RemainingBalance: baseToBankingTransactionAmount,
				CurrencyId:       toAccountCurrencyId,
				ExchangeRate:     exchangeRate,
				CurrentStatus:    models.SupplierAdvanceStatusConfirmed,
			}
		}
		err = tx.Create(&advance).Error
		if err != nil {
			config.LogError(logger, "BankingTransactionWorkflow.go", "CreateBankingTransaction", "CreateSupplierCreditAdvance", advance, err)
			return 0, nil, 0, err
		}
		err = tx.Model(&bankTransaction).Update("credit_advance_id", advance.ID).Error
		if err != nil {
			config.LogError(logger, "BankingTransactionWorkflow.go", "CreateBankingTransaction", "UpdateCreditAdvanceId", advance.ID, err)
			return 0, nil, 0, err
		}
	} else if models.AccountReferenceType(recordType) == models.AccountReferenceTypeAdvanceSupplierPayment && bankTransaction.CreditAdvanceId > 0 {
		var advance models.SupplierCreditAdvance
		err = tx.First(&advance, bankTransaction.CreditAdvanceId).Error
		if err != nil {
			config.LogError(logger, "BankingTransactionWorkflow.go", "CreateBankingTransaction", "GetSupplierCreditAdvance", bankTransaction, err)
			return 0, nil, 0, err
		}

		if toAccountCurrencyId != baseCurrencyId {
			difAmount := foreignToBankingTransactionAmount.Sub(advance.Amount)
			advance.Amount = foreignToBankingTransactionAmount
			advance.RemainingBalance = advance.RemainingBalance.Add(difAmount)
		} else {
			difAmount := baseToBankingTransactionAmount.Sub(advance.Amount)
			advance.Amount = baseToBankingTransactionAmount
			advance.RemainingBalance = advance.RemainingBalance.Add(difAmount)
		}
		if advance.RemainingBalance.IsZero() {
			advance.CurrentStatus = models.SupplierAdvanceStatusClosed
		}
		err = tx.Save(&advance).Error
		if err != nil {
			config.LogError(logger, "BankingTransactionWorkflow.go", "CreateBankingTransaction", "UpdateSupplierCreditAdvance", advance, err)
			return 0, nil, 0, err
		}
	} else if models.AccountReferenceType(recordType) == models.AccountReferenceTypeAdvanceCustomerPayment && bankTransaction.CreditAdvanceId == 0 {
		var advance models.CustomerCreditAdvance
		if toAccountCurrencyId != baseCurrencyId {
			advance = models.CustomerCreditAdvance{
				BusinessId:       businessId,
				Date:             bankTransaction.TransactionDate,
				BranchId:         bankTransaction.BranchId,
				CustomerId:       bankTransaction.CustomerId,
				Amount:           foreignToBankingTransactionAmount,
				RemainingBalance: foreignToBankingTransactionAmount,
				CurrencyId:       toAccountCurrencyId,
				ExchangeRate:     exchangeRate,
				CurrentStatus:    models.CustomerAdvanceStatusConfirmed,
			}
		} else {
			advance = models.CustomerCreditAdvance{
				BusinessId:       businessId,
				Date:             bankTransaction.TransactionDate,
				BranchId:         bankTransaction.BranchId,
				CustomerId:       bankTransaction.CustomerId,
				Amount:           baseToBankingTransactionAmount,
				RemainingBalance: baseToBankingTransactionAmount,
				CurrencyId:       toAccountCurrencyId,
				ExchangeRate:     exchangeRate,
				CurrentStatus:    models.CustomerAdvanceStatusConfirmed,
			}
		}
		err = tx.Create(&advance).Error
		if err != nil {
			config.LogError(logger, "BankingTransactionWorkflow.go", "CreateBankingTransaction", "CreateCustomerCreditAdvance", advance, err)
			return 0, nil, 0, err
		}
		err = tx.Model(&bankTransaction).Update("credit_advance_id", advance.ID).Error
		if err != nil {
			config.LogError(logger, "BankingTransactionWorkflow.go", "CreateBankingTransaction", "UpdateCreditAdvanceId", advance.ID, err)
			return 0, nil, 0, err
		}
	} else if models.AccountReferenceType(recordType) == models.AccountReferenceTypeAdvanceCustomerPayment && bankTransaction.CreditAdvanceId > 0 {
		var advance models.CustomerCreditAdvance
		err = tx.First(&advance, bankTransaction.CreditAdvanceId).Error
		if err != nil {
			return 0, nil, 0, err
		}
		if toAccountCurrencyId != baseCurrencyId {
			difAmount := foreignToBankingTransactionAmount.Sub(advance.Amount)
			advance.Amount = foreignToBankingTransactionAmount
			advance.RemainingBalance = advance.RemainingBalance.Add(difAmount)
		} else {
			difAmount := baseToBankingTransactionAmount.Sub(advance.Amount)
			advance.Amount = baseToBankingTransactionAmount
			advance.RemainingBalance = advance.RemainingBalance.Add(difAmount)
		}
		if advance.RemainingBalance.IsZero() {
			advance.CurrentStatus = models.CustomerAdvanceStatusClosed
		}
		err = tx.Save(&advance).Error
		if err != nil {
			config.LogError(logger, "BankingTransactionWorkflow.go", "CreateBankingTransaction", "UpdateCustomerCreditAdvance", advance, err)
			return 0, nil, 0, err
		}
	}

	err = tx.Create(&accJournal).Error
	if err != nil {
		config.LogError(logger, "BankingTransactionWorkflow.go", "CreateBankingTransaction", "CreateAccountJournal", accJournal, err)
		return 0, nil, 0, err
	}

	// Get latest base closing balance for each account
	// lastBaseTransactions, err := GetLatestBaseTransactionByAccount(tx, baseCurrencyId, branchId, accountIds, transactionTime)
	// if err != nil {
	// 	return nil, err
	// }

	// // Get latest foreign closing balance for each account
	// var lastForeignTransactions []*models.AccountTransaction
	// if baseCurrencyId != foreignCurrencyId {
	// 	lastForeignTransactions, err = GetLatestForeignTransactionByAccount(tx, foreignCurrencyId, branchId, accountIds, transactionTime)
	// 	if err != nil {
	// 		return nil, err
	// 	}
	// }

	// err = UpdateBalances(tx, businessId, baseCurrencyId, branchId, accountIds, transactionTime, lastBaseTransactions, foreignCurrencyId, lastForeignTransactions)
	// err = UpdateBalances(tx, businessId, baseCurrencyId, branchId, accountIds, transactionTime, foreignCurrencyId)
	// if err != nil {
	// 	tx.Rollback()
	// 	return nil, nil, 0, err
	// }

	// record := models.PubSubMessageRecord{
	// 	ID:                  recordId,
	// 	JournalId:           accountJournal.ID,
	// 	BusinessId:          businessId,
	// 	TransactionDateTime: transactionTime,
	// 	ReferenceId:         bankTransaction.ID,
	// 	ReferenceType:       string(accountJournal.ReferenceType),
	// 	ActionType:          "C",
	// }

	return accJournal.ID, accountIds, foreignCurrencyId, nil
}

func DeleteBankingTransaction(tx *gorm.DB, logger *logrus.Logger, recordId int, recordType string, businessId string, business models.Business, oldBankTransaction models.BankingTransaction, isUpdate bool) (int, []int, int, error) {

	foreignCurrencyId := oldBankTransaction.CurrencyId
	accountJournal, _, accountIds, err := GetExistingAccountJournal(tx, oldBankTransaction.ID, models.AccountReferenceType(recordType))
	if err != nil {
		config.LogError(logger, "BankingTransactionWorkflow.go", "DeleteBankingTransaction", "GetExistingAccountJournal", oldBankTransaction, err)
		return 0, nil, 0, err
	}

	// Get latest base closing balance for each account
	// lastBaseTransactions, err := GetLatestBaseTransactionByAccount(tx, baseCurrencyId, branchId, accountIds, transactionTime)
	// if err != nil {
	// 	return nil, err
	// }

	// Get latest foreign closing balance for each account
	// var lastForeignTransactions []*models.AccountTransaction
	// if baseCurrencyId != foreignCurrencyId {
	// 	lastForeignTransactions, err = GetLatestForeignTransactionByAccount(tx, foreignCurrencyId, branchId, accountIds, transactionTime)
	// 	if err != nil {
	// 		return nil, err
	// 	}
	// }

	// Phase 1: do not delete posted journals; create a reversal journal instead.
	reversalID, err := ReverseAccountJournal(tx, accountJournal, ReversalReasonBankingTransactionVoidUpdate)
	if err != nil {
		config.LogError(logger, "BankingTransactionWorkflow.go", "DeleteBankingTransaction", "ReverseAccountJournal", accountJournal, err)
		return 0, nil, 0, err
	}

	if !isUpdate {
		if models.AccountReferenceType(recordType) == models.AccountReferenceTypeAdvanceCustomerPayment && oldBankTransaction.CreditAdvanceId > 0 {
			err = tx.Delete(&models.CustomerCreditAdvance{}, oldBankTransaction.CreditAdvanceId).Error
			if err != nil {
				config.LogError(logger, "BankingTransactionWorkflow.go", "DeleteBankingTransaction", "DeleteCustomerCreditAdvance", oldBankTransaction.CreditAdvanceId, err)
				return 0, nil, 0, err
			}
		} else if models.AccountReferenceType(recordType) == models.AccountReferenceTypeAdvanceSupplierPayment && oldBankTransaction.CreditAdvanceId > 0 {
			err = tx.Delete(&models.SupplierCreditAdvance{}, oldBankTransaction.CreditAdvanceId).Error
			if err != nil {
				config.LogError(logger, "BankingTransactionWorkflow.go", "DeleteBankingTransaction", "DeleteSupplierCreditAdvance", oldBankTransaction.CreditAdvanceId, err)
				return 0, nil, 0, err
			}
		}
	}

	// err = UpdateBalances(tx, businessId, baseCurrencyId, branchId, accountIds, transactionTime, lastBaseTransactions, foreignCurrencyId, lastForeignTransactions)
	// err = UpdateBalances(tx, businessId, baseCurrencyId, branchId, accountIds, transactionTime, foreignCurrencyId)
	// if err != nil {
	// 	tx.Rollback()
	// 	return nil, err
	// }

	// record := models.PubSubMessageRecord{
	// 	ID:                  recordId,
	// 	JournalId:           accountJournal.ID,
	// 	BusinessId:          businessId,
	// 	TransactionDateTime: transactionTime,
	// 	ReferenceId:         oldBankTransaction.ID,
	// 	ReferenceType:       string(accountJournal.ReferenceType),
	// 	ActionType:          "D",
	// }

	return reversalID, accountIds, foreignCurrencyId, nil
}
