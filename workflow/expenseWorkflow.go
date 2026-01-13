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

func ProcessExpenseWorkflow(tx *gorm.DB, logger *logrus.Logger, msg config.PubSubMessage) error {
	var accountJournalId int
	var accountIds []int
	var foreignCurrencyId int
	business, err := models.GetBusinessById2(tx, msg.BusinessId)
	if err != nil {
		config.LogError(logger, "ExpenseWorkflow.go", "ProcessExpenseWorkflow", "GetBusiness", msg.BusinessId, err)
		return err
	}
	if msg.Action == string(models.PubSubMessageActionCreate) {

		var expense models.Expense
		err := json.Unmarshal([]byte(msg.NewObj), &expense)
		if err != nil {
			config.LogError(logger, "ExpenseWorkflow.go", "ProcessExpenseWorkflow > Create", "Unmarshal msg.NewObj", msg.NewObj, err)
			return err
		}
		accountJournalId, accountIds, foreignCurrencyId, err = CreateExpense(tx, logger, msg.ID, msg.ReferenceType, msg.BusinessId, *business, expense)
		if err != nil {
			config.LogError(logger, "ExpenseWorkflow.go", "ProcessExpenseWorkflow > Create", "CreateExpense", nil, err)
			return err
		}
		err = UpdateBalances(tx, logger, msg.BusinessId, business.BaseCurrencyId, expense.BranchId, accountIds, expense.ExpenseDate, foreignCurrencyId)
		if err != nil {
			config.LogError(logger, "ExpenseWorkflow.go", "ProcessExpenseWorkflow > Create", "UpdateBalances", expense, err)
			return nil
		}
		err = UpdateBankBalances(tx, business.BaseCurrencyId, expense.BranchId, accountIds, expense.ExpenseDate)
		if err != nil {
			config.LogError(logger, "ExpenseWorkflow.go", "ProcessExpenseWorkflow > Create", "UpdateBankBalances", expense, err)
			return err
		}

	} else if msg.Action == string(models.PubSubMessageActionUpdate) {

		var oldForeignCurrencyId int
		var oldAccountIds []int
		var expense models.Expense
		err := json.Unmarshal([]byte(msg.NewObj), &expense)
		if err != nil {
			config.LogError(logger, "ExpenseWorkflow.go", "ProcessExpenseWorkflow > Update", "Unmarshal msg.NewObj", msg.NewObj, err)
			return err
		}
		var oldExpense models.Expense
		err = json.Unmarshal([]byte(msg.OldObj), &oldExpense)
		if err != nil {
			config.LogError(logger, "ExpenseWorkflow.go", "ProcessExpenseWorkflow > Update", "Unmarshal msg.OldObj", msg.OldObj, err)
			return err
		}
		_, oldAccountIds, oldForeignCurrencyId, err = DeleteExpense(tx, logger, msg.ID, msg.ReferenceType, msg.BusinessId, *business, oldExpense)
		if err != nil {
			config.LogError(logger, "ExpenseWorkflow.go", "ProcessExpenseWorkflow > Update", "DeleteExpense", nil, err)
			return err
		}
		accountJournalId, accountIds, foreignCurrencyId, err = CreateExpense(tx, logger, msg.ID, msg.ReferenceType, msg.BusinessId, *business, expense)
		if err != nil {
			config.LogError(logger, "ExpenseWorkflow.go", "ProcessExpenseWorkflow > Update", "CreateExpense", nil, err)
			return err
		}
		if oldExpense.BranchId != expense.BranchId || oldExpense.ExpenseDate != expense.ExpenseDate || oldForeignCurrencyId != foreignCurrencyId {
			err = UpdateBalances(tx, logger, msg.BusinessId, business.BaseCurrencyId, oldExpense.BranchId, oldAccountIds, oldExpense.ExpenseDate, oldForeignCurrencyId)
			if err != nil {
				config.LogError(logger, "ExpenseWorkflow.go", "ProcessExpenseWorkflow > Update", "UpdateBalances Old", oldExpense, err)
				return err
			}
			err = UpdateBankBalances(tx, business.BaseCurrencyId, oldExpense.BranchId, oldAccountIds, oldExpense.ExpenseDate)
			if err != nil {
				config.LogError(logger, "ExpenseWorkflow.go", "ProcessExpenseWorkflow > Update", "UpdateBankBalances Old", oldExpense, err)
				return err
			}
		} else {
			accountIds = utils.MergeIntSlices(accountIds, oldAccountIds)
		}
		err = UpdateBalances(tx, logger, msg.BusinessId, business.BaseCurrencyId, expense.BranchId, accountIds, expense.ExpenseDate, foreignCurrencyId)
		if err != nil {
			config.LogError(logger, "ExpenseWorkflow.go", "ProcessExpenseWorkflow > Update", "UpdateBalances", expense, err)
			return err
		}
		err = UpdateBankBalances(tx, business.BaseCurrencyId, expense.BranchId, accountIds, expense.ExpenseDate)
		if err != nil {
			config.LogError(logger, "ExpenseWorkflow.go", "ProcessExpenseWorkflow > Update", "UpdateBankBalances", expense, err)
			return err
		}
	} else if msg.Action == string(models.PubSubMessageActionDelete) {

		var oldExpense models.Expense
		err = json.Unmarshal([]byte(msg.OldObj), &oldExpense)
		if err != nil {
			config.LogError(logger, "ExpenseWorkflow.go", "ProcessExpenseWorkflow > Delete", "Unmarshal msg.OldObj", msg.OldObj, err)
			return err
		}
		accountJournalId, accountIds, foreignCurrencyId, err = DeleteExpense(tx, logger, msg.ID, msg.ReferenceType, msg.BusinessId, *business, oldExpense)
		if err != nil {
			config.LogError(logger, "ExpenseWorkflow.go", "ProcessExpenseWorkflow > Delete", "DeleteExpense", nil, err)
			return err
		}
		err = UpdateBalances(tx, logger, msg.BusinessId, business.BaseCurrencyId, oldExpense.BranchId, accountIds, oldExpense.ExpenseDate, foreignCurrencyId)
		if err != nil {
			config.LogError(logger, "ExpenseWorkflow.go", "ProcessExpenseWorkflow > Delete", "UpdateBalances", oldExpense, err)
			return err
		}
		err = UpdateBankBalances(tx, business.BaseCurrencyId, oldExpense.BranchId, accountIds, oldExpense.ExpenseDate)
		if err != nil {
			config.LogError(logger, "ExpenseWorkflow.go", "ProcessExpenseWorkflow > Delete", "UpdateBankBalances", oldExpense, err)
			return err
		}
	}
	err = tx.Model(&models.PubSubMessageRecord{}).Where("id=?", msg.ID).Updates(map[string]interface{}{"account_journal_id": accountJournalId, "is_processed": true}).Error
	if err != nil {
		config.LogError(logger, "ExpenseWorkflow.go", "ProcessExpenseWorkflow", "UpdatePubSubMessageRecord", accountJournalId, err)
		return err
	}
	return nil
}

func CreateExpense(tx *gorm.DB, logger *logrus.Logger, recordId int, recordType string, businessId string, business models.Business, expense models.Expense) (int, []int, int, error) {

	accountIds := []int{expense.AssetAccountId, expense.ExpenseAccountId}
	transactionTime := expense.ExpenseDate
	branchId := expense.BranchId
	baseCurrencyId := business.BaseCurrencyId
	foreignCurrencyId := expense.CurrencyId

	baseAssetAmount := expense.TotalAmount
	foreignAssetAmount := decimal.NewFromInt(0)
	baseBankCharges := expense.BankCharges
	foreignBankCharges := decimal.NewFromInt(0)
	baseTaxAmount := expense.TaxAmount
	foreignTaxAmount := decimal.NewFromInt(0)
	baseExpenseAmount := expense.TotalAmount.Sub(expense.TaxAmount).Sub(expense.BankCharges)
	foreignExpenseAmount := decimal.NewFromInt(0)
	exchangeRate := expense.ExchangeRate

	if baseCurrencyId != foreignCurrencyId {
		foreignAssetAmount = baseAssetAmount
		baseAssetAmount = foreignAssetAmount.Mul(exchangeRate)
		foreignBankCharges = baseBankCharges
		baseBankCharges = foreignBankCharges.Mul(exchangeRate)
		foreignTaxAmount = baseTaxAmount
		baseTaxAmount = foreignTaxAmount.Mul(exchangeRate)
		foreignExpenseAmount = baseExpenseAmount
		baseExpenseAmount = foreignExpenseAmount.Mul(exchangeRate)
	}

	accTransactions := make([]models.AccountTransaction, 0)

	// Insert into banking transaction if asset (withdraw) account is cash or bank
	withdrawAccountInfo, err := GetAccount(tx, expense.AssetAccountId)
	if err != nil {
		config.LogError(logger, "ExpenseWorkflow.go", "CreateExpense", "GetWithdrawAccount", expense.AssetAccountId, err)
		return 0, nil, 0, err
	}
	bankingTransactionId := 0
	if withdrawAccountInfo.DetailType == models.AccountDetailTypeCash ||
		withdrawAccountInfo.DetailType == models.AccountDetailTypeBank {

		bankingTransaction := models.BankingTransaction{
			BusinessId:        businessId,
			BranchId:          expense.BranchId,
			FromAccountId:     expense.AssetAccountId,
			FromAccountAmount: expense.TotalAmount,
			ToAccountId:       expense.ExpenseAccountId,
			ToAccountAmount:   expense.TotalAmount.Sub(expense.TaxAmount).Sub(expense.BankCharges),
			CustomerId:        expense.CustomerId,
			SupplierId:        expense.SupplierId,
			TransactionDate:   expense.ExpenseDate,
			TransactionId:     expense.ID,
			TransactionNumber: expense.ExpenseNumber,
			TransactionType:   models.BankingTransactionTypeExpense,
			ExchangeRate:      expense.ExchangeRate,
			CurrencyId:        expense.CurrencyId,
			Amount:            expense.TotalAmount,
			TaxAmount:         expense.TaxAmount,
			BankCharges:       expense.BankCharges,
			ReferenceNumber:   expense.ReferenceNumber,
			Description:       expense.Notes,
		}
		err = tx.Create(&bankingTransaction).Error
		if err != nil {
			config.LogError(logger, "ExpenseWorkflow.go", "CreateExpense", "GetSystemAccounts", businessId, err)
			return 0, nil, 0, err
		}
		bankingTransactionId = bankingTransaction.ID
	}

	assetTransact := models.AccountTransaction{
		BusinessId:           businessId,
		AccountId:            expense.AssetAccountId,
		BranchId:             branchId,
		TransactionDateTime:  transactionTime,
		BaseCurrencyId:       baseCurrencyId,
		BaseDebit:            decimal.NewFromInt(0),
		BaseCredit:           baseAssetAmount,
		ForeignCurrencyId:    foreignCurrencyId,
		ForeignDebit:         decimal.NewFromInt(0),
		ForeignCredit:        foreignAssetAmount,
		ExchangeRate:         exchangeRate,
		BankingTransactionId: bankingTransactionId,
	}
	accTransactions = append(accTransactions, assetTransact)

	expenseTransact := models.AccountTransaction{
		BusinessId:           businessId,
		AccountId:            expense.ExpenseAccountId,
		BranchId:             branchId,
		TransactionDateTime:  transactionTime,
		BaseCurrencyId:       baseCurrencyId,
		BaseDebit:            baseExpenseAmount,
		BaseCredit:           decimal.NewFromInt(0),
		ForeignCurrencyId:    foreignCurrencyId,
		ForeignDebit:         foreignExpenseAmount,
		ForeignCredit:        decimal.NewFromInt(0),
		ExchangeRate:         exchangeRate,
		BankingTransactionId: bankingTransactionId,
	}
	accTransactions = append(accTransactions, expenseTransact)

	if !baseBankCharges.IsZero() {
		systemAccounts, err := models.GetSystemAccounts(businessId)
		if err != nil {
			config.LogError(logger, "ExpenseWorkflow.go", "CreateExpense", "GetSystemAccounts", businessId, err)
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
			ForeignCurrencyId:    foreignCurrencyId,
			ForeignDebit:         foreignBankCharges,
			ForeignCredit:        decimal.NewFromInt(0),
			ExchangeRate:         exchangeRate,
			BankingTransactionId: bankingTransactionId,
		}
		accountIds = append(accountIds, systemAccounts[models.AccountCodeBankFeesAndCharges])
		accTransactions = append(accTransactions, bankCharges)
	}

	if !baseTaxAmount.IsZero() {
		systemAccounts, err := models.GetSystemAccounts(businessId)
		if err != nil {
			config.LogError(logger, "ExpenseWorkflow.go", "CreateExpense", "GetSystemAccounts", businessId, err)
			return 0, nil, 0, err
		}

		taxPayable := models.AccountTransaction{
			BusinessId:           businessId,
			AccountId:            systemAccounts[models.AccountCodeTaxPayable],
			BranchId:             branchId,
			TransactionDateTime:  transactionTime,
			BaseCurrencyId:       baseCurrencyId,
			BaseDebit:            baseTaxAmount,
			BaseCredit:           decimal.NewFromInt(0),
			ForeignCurrencyId:    foreignCurrencyId,
			ForeignDebit:         foreignTaxAmount,
			ForeignCredit:        decimal.NewFromInt(0),
			ExchangeRate:         exchangeRate,
			BankingTransactionId: bankingTransactionId,
		}
		accountIds = append(accountIds, systemAccounts[models.AccountCodeTaxPayable])
		accTransactions = append(accTransactions, taxPayable)
	}

	accJournal := models.AccountJournal{
		BusinessId:          businessId,
		BranchId:            expense.BranchId,
		TransactionDateTime: expense.ExpenseDate,
		TransactionNumber:   strconv.Itoa(expense.ID),
		TransactionDetails:  expense.Notes,
		ReferenceNumber:     expense.ReferenceNumber,
		CustomerId:          expense.CustomerId,
		SupplierId:          expense.SupplierId,
		ReferenceId:         expense.ID,
		ReferenceType:       models.AccountReferenceTypeExpense,
		AccountTransactions: accTransactions,
	}

	err = tx.Create(&accJournal).Error
	if err != nil {
		config.LogError(logger, "ExpenseWorkflow.go", "CreateExpense", "CreateAccountJournal", accJournal, err)
		return 0, nil, 0, err
	}

	return accJournal.ID, accountIds, foreignCurrencyId, nil
}

func DeleteExpense(tx *gorm.DB, logger *logrus.Logger, recordId int, recordType string, businessId string, business models.Business, oldExpense models.Expense) (int, []int, int, error) {

	foreignCurrencyId := oldExpense.CurrencyId
	accountJournal, _, accountIds, err := GetExistingAccountJournal(tx, oldExpense.ID, models.AccountReferenceTypeExpense)
	if err != nil {
		config.LogError(logger, "ExpenseWorkflow.go", "DeleteExpense", "GetExistingAccountJournal", oldExpense, err)
		return 0, nil, 0, err
	}

	// Phase 1: do not delete posted journals; create a reversal journal instead.
	reversalID, err := ReverseAccountJournal(tx, accountJournal, ReversalReasonExpenseVoidUpdate)
	if err != nil {
		config.LogError(logger, "ExpenseWorkflow.go", "DeleteExpense", "ReverseAccountJournal", accountJournal, err)
		return 0, nil, 0, err
	}

	err = tx.Where("transaction_id = ? AND transaction_type = ?", oldExpense.ID, models.BankingTransactionTypeExpense).Delete(&models.BankingTransaction{}).Error
	if err != nil {
		config.LogError(logger, "ExpenseWorkflow.go", "DeleteExpense", "DeleteBankingTransaction", oldExpense, err)
		return 0, nil, 0, err
	}

	return reversalID, accountIds, foreignCurrencyId, nil
}
