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

func ProcessSupplierPaymentWorkflow(tx *gorm.DB, logger *logrus.Logger, msg config.PubSubMessage) error {

	var accountJournalId int
	var accountIds []int
	var foreignCurrencyId int
	business, err := models.GetBusinessById2(tx, msg.BusinessId)
	if err != nil {
		config.LogError(logger, "SupplierPaymentWorkflow.go", "ProcessSupplierPaymentWorkflow", "GetBusiness", msg.BusinessId, err)
		return err
	}
	if msg.Action == string(models.PubSubMessageActionCreate) {

		var supplierPayment models.SupplierPayment
		err := json.Unmarshal([]byte(msg.NewObj), &supplierPayment)
		if err != nil {
			config.LogError(logger, "SupplierPaymentWorkflow.go", "ProcessSupplierPaymentWorkflow > Create", "Unmarshal msg.NewObj", msg.NewObj, err)
			return err
		}
		accountJournalId, accountIds, foreignCurrencyId, err = CreateSupplierPayment(tx, logger, msg.ID, msg.ReferenceType, msg.BusinessId, *business, supplierPayment)
		if err != nil {
			config.LogError(logger, "SupplierPaymentWorkflow.go", "ProcessSupplierPaymentWorkflow > Create", "CreateSupplierPayment", nil, err)
			return err
		}
		err = UpdateBalances(tx, logger, msg.BusinessId, business.BaseCurrencyId, supplierPayment.BranchId, accountIds, supplierPayment.PaymentDate, foreignCurrencyId)
		if err != nil {
			config.LogError(logger, "SupplierPaymentWorkflow.go", "ProcessSupplierPaymentWorkflow > Create", "UpdateBalances", supplierPayment, err)
			return err
		}
		err = UpdateBankBalances(tx, business.BaseCurrencyId, supplierPayment.BranchId, accountIds, supplierPayment.PaymentDate)
		if err != nil {
			config.LogError(logger, "SupplierPaymentWorkflow.go", "ProcessSupplierPaymentWorkflow > Create", "UpdateBankBalances", supplierPayment, err)
			return err
		}

	} else if msg.Action == string(models.PubSubMessageActionUpdate) {

		var oldForeignCurrencyId int
		var oldAccountIds []int
		var supplierPayment models.SupplierPayment
		err := json.Unmarshal([]byte(msg.NewObj), &supplierPayment)
		if err != nil {
			config.LogError(logger, "SupplierPaymentWorkflow.go", "ProcessSupplierPaymentWorkflow > Update", "Unmarshal msg.NewObj", msg.NewObj, err)
			return err
		}
		var oldSupplierPayment models.SupplierPayment
		err = json.Unmarshal([]byte(msg.OldObj), &oldSupplierPayment)
		if err != nil {
			config.LogError(logger, "SupplierPaymentWorkflow.go", "ProcessSupplierPaymentWorkflow > Update", "Unmarshal msg.OldObj", msg.OldObj, err)
			return err
		}
		_, oldAccountIds, oldForeignCurrencyId, err = DeleteSupplierPayment(tx, logger, msg.ID, msg.ReferenceType, msg.BusinessId, *business, oldSupplierPayment)
		if err != nil {
			config.LogError(logger, "SupplierPaymentWorkflow.go", "ProcessSupplierPaymentWorkflow > Update", "DeleteSupplierPayment", nil, err)
			return err
		}
		accountJournalId, accountIds, foreignCurrencyId, err = CreateSupplierPayment(tx, logger, msg.ID, msg.ReferenceType, msg.BusinessId, *business, supplierPayment)
		if err != nil {
			config.LogError(logger, "SupplierPaymentWorkflow.go", "ProcessSupplierPaymentWorkflow > Update", "CreateSupplierPayment", nil, err)
			return err
		}
		if oldSupplierPayment.BranchId != supplierPayment.BranchId || oldSupplierPayment.PaymentDate != supplierPayment.PaymentDate || oldForeignCurrencyId != foreignCurrencyId {
			err = UpdateBalances(tx, logger, msg.BusinessId, business.BaseCurrencyId, oldSupplierPayment.BranchId, oldAccountIds, oldSupplierPayment.PaymentDate, oldForeignCurrencyId)
			if err != nil {
				config.LogError(logger, "SupplierPaymentWorkflow.go", "ProcessSupplierPaymentWorkflow > Update", "UpdateBalances Old", oldSupplierPayment, err)
				return err
			}
			err = UpdateBankBalances(tx, business.BaseCurrencyId, oldSupplierPayment.BranchId, oldAccountIds, oldSupplierPayment.PaymentDate)
			if err != nil {
				config.LogError(logger, "SupplierPaymentWorkflow.go", "ProcessSupplierPaymentWorkflow > Update", "UpdateBankBalances Old", oldSupplierPayment, err)
				return err
			}
		} else {
			accountIds = utils.MergeIntSlices(accountIds, oldAccountIds)
		}
		err = UpdateBalances(tx, logger, msg.BusinessId, business.BaseCurrencyId, supplierPayment.BranchId, accountIds, supplierPayment.PaymentDate, foreignCurrencyId)
		if err != nil {
			config.LogError(logger, "SupplierPaymentWorkflow.go", "ProcessSupplierPaymentWorkflow > Update", "UpdateBalances", supplierPayment, err)
			return err
		}
		err = UpdateBankBalances(tx, business.BaseCurrencyId, supplierPayment.BranchId, accountIds, supplierPayment.PaymentDate)
		if err != nil {
			config.LogError(logger, "SupplierPaymentWorkflow.go", "ProcessSupplierPaymentWorkflow > Update", "UpdateBankBalances", supplierPayment, err)
			return err
		}
	} else if msg.Action == string(models.PubSubMessageActionDelete) {

		var oldSupplierPayment models.SupplierPayment
		err = json.Unmarshal([]byte(msg.OldObj), &oldSupplierPayment)
		if err != nil {
			config.LogError(logger, "SupplierPaymentWorkflow.go", "ProcessSupplierPaymentWorkflow > Delete", "Unmarshal msg.OldObj", msg.OldObj, err)
			return err
		}
		accountJournalId, accountIds, foreignCurrencyId, err = DeleteSupplierPayment(tx, logger, msg.ID, msg.ReferenceType, msg.BusinessId, *business, oldSupplierPayment)
		if err != nil {
			config.LogError(logger, "SupplierPaymentWorkflow.go", "ProcessSupplierPaymentWorkflow > Delete", "DeleteSupplierPayment", nil, err)
			return err
		}
		err = UpdateBalances(tx, logger, msg.BusinessId, business.BaseCurrencyId, oldSupplierPayment.BranchId, accountIds, oldSupplierPayment.PaymentDate, foreignCurrencyId)
		if err != nil {
			config.LogError(logger, "SupplierPaymentWorkflow.go", "ProcessSupplierPaymentWorkflow > Delete", "UpdateBalances", oldSupplierPayment, err)
			return err
		}
		err = UpdateBankBalances(tx, business.BaseCurrencyId, oldSupplierPayment.BranchId, accountIds, oldSupplierPayment.PaymentDate)
		if err != nil {
			config.LogError(logger, "SupplierPaymentWorkflow.go", "ProcessSupplierPaymentWorkflow > Delete", "UpdateBankBalances", oldSupplierPayment, err)
			return err
		}
	}
	err = tx.Model(&models.PubSubMessageRecord{}).Where("id=?", msg.ID).Updates(map[string]interface{}{"account_journal_id": accountJournalId, "is_processed": true}).Error
	if err != nil {
		return err
	}
	return nil
}

func CreateSupplierPayment(tx *gorm.DB, logger *logrus.Logger, recordId int, recordType string, businessId string, business models.Business, supplierPayment models.SupplierPayment) (int, []int, int, error) {

	systemAccounts, err := models.GetSystemAccounts(businessId)
	if err != nil {
		config.LogError(logger, "SupplierPaymentWorkflow.go", "CreateSupplierPayment", "GetSystemAccounts", businessId, err)
		return 0, nil, 0, err
	}

	transactionTime := supplierPayment.PaymentDate
	branchId := supplierPayment.BranchId
	baseCurrencyId := business.BaseCurrencyId
	foreignCurrencyId := supplierPayment.CurrencyId
	paymentCurrencyId := supplierPayment.CurrencyId

	billAmount := decimal.NewFromInt(0)
	baseBillAmount := decimal.NewFromInt(0)
	baseAmount := decimal.NewFromInt(0)
	foreignAmount := decimal.NewFromInt(0)
	baseBankCharges := supplierPayment.BankCharges
	foreignBankCharges := decimal.NewFromInt(0)
	baseTotalAmount := decimal.NewFromInt(0)
	foreignTotalAmount := decimal.NewFromInt(0)
	exchangeRate := supplierPayment.ExchangeRate

	withdrawAccount, err := GetAccount(tx, supplierPayment.WithdrawAccountId)
	if err != nil {
		config.LogError(logger, "SupplierPaymentWorkflow.go", "CreateSupplierPayment", "GetWithdrawAccount", supplierPayment.WithdrawAccountId, err)
		return 0, nil, 0, err
	}

	if baseCurrencyId != paymentCurrencyId {
		var bill models.Bill
		for _, paidBill := range supplierPayment.PaidBills {
			err = tx.First(&bill, paidBill.BillId).Error
			if err != nil {
				config.LogError(logger, "SupplierPaymentWorkflow.go", "CreateSupplierPayment", "GetPaidBill", paidBill.BillId, err)
				return 0, nil, 0, err
			}
			billAmount = billAmount.Add(paidBill.PaidAmount)
			baseBillAmount = baseBillAmount.Add(paidBill.PaidAmount.Mul(bill.ExchangeRate))
			baseAmount = baseAmount.Add(paidBill.PaidAmount.Mul(exchangeRate))
			foreignAmount = foreignAmount.Add(paidBill.PaidAmount)
		}
		foreignCurrencyId = paymentCurrencyId
	} else {
		for _, paidBill := range supplierPayment.PaidBills {
			billAmount = billAmount.Add(paidBill.PaidAmount)
			baseBillAmount = baseBillAmount.Add(paidBill.PaidAmount)
			baseAmount = baseAmount.Add(paidBill.PaidAmount)
		}
	}

	withdrawAccountCurrencyId := 0
	if withdrawAccount.CurrencyId == 0 || withdrawAccount.CurrencyId == baseCurrencyId { // base-currency account
		if baseCurrencyId != paymentCurrencyId {
			foreignTotalAmount = billAmount.Add(supplierPayment.BankCharges)
			baseTotalAmount = foreignTotalAmount.Mul(exchangeRate)
			foreignBankCharges = supplierPayment.BankCharges
			baseBankCharges = foreignBankCharges.Mul(exchangeRate)
			withdrawAccountCurrencyId = paymentCurrencyId
			foreignCurrencyId = paymentCurrencyId
		} else {
			baseTotalAmount = billAmount.Add(supplierPayment.BankCharges)
			baseBankCharges = supplierPayment.BankCharges
			withdrawAccountCurrencyId = baseCurrencyId
		}
	} else { // foreign currency account
		if baseCurrencyId != paymentCurrencyId {
			foreignTotalAmount = billAmount.Add(supplierPayment.BankCharges)
			baseTotalAmount = foreignTotalAmount.Mul(exchangeRate)
			foreignBankCharges = supplierPayment.BankCharges
			baseBankCharges = foreignBankCharges.Mul(exchangeRate)
			withdrawAccountCurrencyId = paymentCurrencyId
			foreignCurrencyId = paymentCurrencyId
		} else {
			if exchangeRate.IsZero() {
				baseTotalAmount = billAmount.Add(supplierPayment.BankCharges)
				foreignTotalAmount = decimal.NewFromInt(0)
				baseBankCharges = supplierPayment.BankCharges
				foreignBankCharges = decimal.NewFromInt(0)
				withdrawAccountCurrencyId = withdrawAccount.CurrencyId
				foreignCurrencyId = withdrawAccount.CurrencyId
			} else {
				baseTotalAmount = billAmount.Add(supplierPayment.BankCharges)
				foreignTotalAmount = baseTotalAmount.DivRound(exchangeRate, 4)
				baseBankCharges = supplierPayment.BankCharges
				foreignBankCharges = baseBankCharges.DivRound(exchangeRate, 4)
				withdrawAccountCurrencyId = withdrawAccount.CurrencyId
				foreignCurrencyId = withdrawAccount.CurrencyId
			}
		}
	}

	// Insert into banking transaction if withdraw account is cash or bank
	var withdrawAccountInfo models.Account
	err = tx.First(&withdrawAccountInfo, supplierPayment.WithdrawAccountId).Error
	if err != nil {
		config.LogError(logger, "SupplierPaymentWorkflow.go", "CreateSupplierPayment", "GetDepositAccount", supplierPayment.WithdrawAccountId, err)
		return 0, nil, 0, err
	}
	bankingTransactionId := 0
	if withdrawAccountInfo.DetailType == models.AccountDetailTypeCash ||
		withdrawAccountInfo.DetailType == models.AccountDetailTypeBank {

		bankingTransaction := models.BankingTransaction{
			BusinessId:        businessId,
			BranchId:          supplierPayment.BranchId,
			FromAccountId:     supplierPayment.WithdrawAccountId,
			FromAccountAmount: billAmount.Add(supplierPayment.BankCharges),
			ToAccountId:       systemAccounts[models.AccountCodeAccountsPayable],
			ToAccountAmount:   billAmount,
			SupplierId:        supplierPayment.SupplierId,
			TransactionDate:   supplierPayment.PaymentDate,
			TransactionId:     supplierPayment.ID,
			TransactionNumber: supplierPayment.PaymentNumber,
			TransactionType:   models.BankingTransactionTypeSupplierPayment,
			ExchangeRate:      supplierPayment.ExchangeRate,
			CurrencyId:        supplierPayment.CurrencyId,
			Amount:            billAmount,
			BankCharges:       supplierPayment.BankCharges,
			ReferenceNumber:   supplierPayment.ReferenceNumber,
			Description:       supplierPayment.Notes,
		}
		err = tx.Create(&bankingTransaction).Error
		if err != nil {
			config.LogError(logger, "SupplierPaymentWorkflow.go", "CreateSupplierPayment", "CreateBankingTransaction", bankingTransaction, err)
			return 0, nil, 0, err
		}
		bankingTransactionId = bankingTransaction.ID
	}

	accountIds := make([]int, 0)
	accTransactions := make([]models.AccountTransaction, 0)

	accountsPayable := models.AccountTransaction{
		BusinessId:           businessId,
		AccountId:            systemAccounts[models.AccountCodeAccountsPayable],
		BranchId:             branchId,
		TransactionDateTime:  transactionTime,
		BaseCurrencyId:       baseCurrencyId,
		BaseDebit:            baseBillAmount,
		BaseCredit:           decimal.NewFromInt(0),
		ForeignCurrencyId:    paymentCurrencyId,
		ForeignDebit:         foreignAmount,
		ForeignCredit:        decimal.NewFromInt(0),
		ExchangeRate:         exchangeRate,
		BankingTransactionId: bankingTransactionId,
	}
	accTransactions = append(accTransactions, accountsPayable)
	accountIds = append(accountIds, systemAccounts[models.AccountCodeAccountsPayable])

	withdrawAccountTransact := models.AccountTransaction{
		BusinessId:           businessId,
		AccountId:            supplierPayment.WithdrawAccountId,
		BranchId:             branchId,
		TransactionDateTime:  transactionTime,
		BaseCurrencyId:       baseCurrencyId,
		BaseDebit:            decimal.NewFromInt(0),
		BaseCredit:           baseTotalAmount,
		ForeignCurrencyId:    withdrawAccountCurrencyId,
		ForeignDebit:         decimal.NewFromInt(0),
		ForeignCredit:        foreignTotalAmount,
		ExchangeRate:         exchangeRate,
		BankingTransactionId: bankingTransactionId,
	}
	accTransactions = append(accTransactions, withdrawAccountTransact)
	accountIds = append(accountIds, supplierPayment.WithdrawAccountId)

	if !baseBillAmount.Equals(baseAmount) {
		gainLossAmount := baseAmount.Sub(baseBillAmount)
		if gainLossAmount.IsPositive() {
			exchangeGainLoss := models.AccountTransaction{
				BusinessId:           businessId,
				AccountId:            systemAccounts[models.AccountCodeExchangeGainOrLoss],
				BranchId:             branchId,
				TransactionDateTime:  transactionTime,
				BaseCurrencyId:       baseCurrencyId,
				BaseDebit:            gainLossAmount.Abs(),
				BaseCredit:           decimal.NewFromInt(0),
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
				BaseDebit:            decimal.NewFromInt(0),
				BaseCredit:           gainLossAmount.Abs(),
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
			ForeignCurrencyId:    withdrawAccountCurrencyId,
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
		BranchId:            supplierPayment.BranchId,
		TransactionDateTime: supplierPayment.PaymentDate,
		TransactionNumber:   strconv.Itoa(supplierPayment.ID),
		TransactionDetails:  supplierPayment.Notes,
		SupplierId:          supplierPayment.SupplierId,
		ReferenceNumber:     supplierPayment.ReferenceNumber,
		ReferenceId:         supplierPayment.ID,
		ReferenceType:       models.AccountReferenceTypeSupplierPayment,
		AccountTransactions: accTransactions,
	}

	err = tx.Create(&accJournal).Error
	if err != nil {
		config.LogError(logger, "SupplierPaymentWorkflow.go", "CreateSupplierPayment", "CreateAccountJournal", accJournal, err)
		return 0, nil, 0, err
	}

	return accJournal.ID, accountIds, foreignCurrencyId, nil
}

func DeleteSupplierPayment(tx *gorm.DB, logger *logrus.Logger, recordId int, recordType string, businessId string, business models.Business, oldSupplierPayment models.SupplierPayment) (int, []int, int, error) {

	foreignCurrencyId := oldSupplierPayment.CurrencyId
	accountJournal, _, accountIds, err := GetExistingAccountJournal(tx, oldSupplierPayment.ID, models.AccountReferenceTypeSupplierPayment)
	if err != nil {
		config.LogError(logger, "SupplierPaymentWorkflow.go", "DeleteSupplierPayment", "GetExistingAccountJournal", oldSupplierPayment, err)
		return 0, nil, 0, err
	}

	// Phase 1: do not delete posted journals; create a reversal journal instead.
	reversalID, err := ReverseAccountJournal(tx, accountJournal, ReversalReasonSupplierPaymentVoidUpdate)
	if err != nil {
		config.LogError(logger, "SupplierPaymentWorkflow.go", "DeleteSupplierPayment", "ReverseAccountJournal", accountJournal, err)
		return 0, nil, 0, err
	}

	err = tx.Where("transaction_id = ? AND transaction_type = ?", oldSupplierPayment.ID, models.BankingTransactionTypeSupplierPayment).Delete(&models.BankingTransaction{}).Error
	if err != nil {
		config.LogError(logger, "SupplierPaymentWorkflow.go", "DeleteSupplierPayment", "DeleteBankingTransaction", oldSupplierPayment, err)
		return 0, nil, 0, err
	}

	return reversalID, accountIds, foreignCurrencyId, nil
}
