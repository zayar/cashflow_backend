package workflow

import (
	"encoding/json"
	"fmt"
	"slices"
	"strconv"
	"time"

	"bitbucket.org/mmdatafocus/books_backend/config"
	"bitbucket.org/mmdatafocus/books_backend/models"
	"bitbucket.org/mmdatafocus/books_backend/utils"
	"github.com/shopspring/decimal"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

func ProcessOpeningBalanceWorkflow(tx *gorm.DB, logger *logrus.Logger, msg config.PubSubMessage) error {
	var accountJournalId int
	var accountIds []int
	var foreignCurrencyId int
	business, err := models.GetBusinessById2(tx, msg.BusinessId)
	if err != nil {
		config.LogError(logger, "OpeningBalanceWorkflow.go", "ProcessOpeningBalanceWorkflow", "GetBusiness", msg.BusinessId, err)
		return err
	}
	if msg.Action == string(models.PubSubMessageActionCreate) {

		var openingBalance models.OpeningBalance
		err := json.Unmarshal([]byte(msg.NewObj), &openingBalance)
		if err != nil {
			config.LogError(logger, "OpeningBalanceWorkflow.go", "ProcessOpeningBalanceWorkflow > Create", "Unmarshal msg.NewObj", msg.NewObj, err)
			return err
		}
		var oldAccountIds []int
		var oldTransactionDateTime *time.Time
		_, oldAccountIds, oldTransactionDateTime, err = DeleteOpeningBalance(tx, logger, msg.ID, msg.ReferenceType, msg.BusinessId, *business, openingBalance)

		if err != nil {
			config.LogError(logger, "OpeningBalanceWorkflow.go", "ProcessOpeningBalanceWorkflow > Create", "DeleteOpeningBalance", nil, err)
			return err
		}
		accountJournalId, accountIds, foreignCurrencyId, err = CreateOpeningBalance(tx, logger, msg.ID, msg.ReferenceType, msg.BusinessId, *business, openingBalance)
		if err != nil {
			config.LogError(logger, "OpeningBalanceWorkflow.go", "ProcessOpeningBalanceWorkflow > Create", "CreateOpeningBalance", nil, err)
			return err
		}
		if len(oldAccountIds) > 0 {
			accountIds = utils.MergeIntSlices(accountIds, oldAccountIds)
		}
		// Determine the oldest date among openingBalance.MigrationDate, business.MigrationDate, and oldTransactionDateTime
		oldestDate := utils.FindOldestDate(&openingBalance.MigrationDate, &business.MigrationDate, oldTransactionDateTime)
		if len(accountIds) > 0 {
			err = UpdateBalances(tx, logger, msg.BusinessId, business.BaseCurrencyId, openingBalance.BranchId, accountIds, *oldestDate, foreignCurrencyId)
			if err != nil {
				config.LogError(logger, "OpeningBalanceWorkflow.go", "OpeningBalanceWorkflow > Create", "UpdateBalances", openingBalance, err)
				return err
			}
			err = UpdateBankBalances(tx, business.BaseCurrencyId, openingBalance.BranchId, accountIds, *oldestDate)
			if err != nil {
				config.LogError(logger, "OpeningBalanceWorkflow.go", "OpeningBalanceWorkflow > Create", "UpdateBankBalances", openingBalance, err)
				return err
			}
		}
	}

	err = tx.Model(&models.PubSubMessageRecord{}).Where("id=?", msg.ID).Updates(map[string]interface{}{"account_journal_id": accountJournalId, "is_processed": true}).Error
	if err != nil {
		config.LogError(logger, "OpeningBalanceWorkflow.go", "ProcessOpeningBalanceWorkflow", "UpdatePubSubMessageRecord", accountJournalId, err)
		return err
	}
	return nil
}

func CreateOpeningBalance(tx *gorm.DB, logger *logrus.Logger, recordId int, recordType string, businessId string, business models.Business, openingBalance models.OpeningBalance) (int, []int, int, error) {

	systemAccounts, err := models.GetSystemAccounts(businessId)
	if err != nil {
		config.LogError(logger, "OpeningBalanceWorkflow.go", "CreateOpeningBalance", "GetSystemAccounts", businessId, err)
		return 0, nil, 0, err
	}

	transactionTime := openingBalance.MigrationDate
	branchId := openingBalance.BranchId
	baseCurrencyId := business.BaseCurrencyId
	foreignCurrencyId := business.BaseCurrencyId

	accountIds := make([]int, 0)
	accTransactions := make([]models.AccountTransaction, 0)
	totalDebit := decimal.NewFromInt(0)
	totalCredit := decimal.NewFromInt(0)

	for _, detail := range openingBalance.Details {
		totalDebit = totalDebit.Add(detail.Debit)
		totalCredit = totalCredit.Add(detail.Credit)
		accountIds = append(accountIds, detail.AccountId)

		acc, err := GetAccount(tx, detail.AccountId)
		if err != nil {
			config.LogError(logger, "OpeningBalanceWorkflow.go", "CreateOpeningBalance", "GetAccount", detail, err)
			return 0, nil, 0, err
		}
		bankingTransactionId := 0
		if acc.DetailType == models.AccountDetailTypeCash ||
			acc.DetailType == models.AccountDetailTypeBank {

			bankingTransaction := models.BankingTransaction{
				BusinessId:        businessId,
				BranchId:          openingBalance.BranchId,
				TransactionDate:   business.MigrationDate,
				TransactionId:     openingBalance.ID,
				TransactionNumber: fmt.Sprintf("%d", openingBalance.ID),
				TransactionType:   models.BankingTransactionTypeOpeningBalance,
				ExchangeRate:      decimal.NewFromInt(0),
				Description:       "Opening Balance",
			}
			if !detail.Debit.IsZero() {
				bankingTransaction.Amount = detail.Debit
				bankingTransaction.ToAccountAmount = detail.Debit
				bankingTransaction.CurrencyId = business.BaseCurrencyId
				bankingTransaction.ToAccountId = acc.ID
			} else {
				bankingTransaction.Amount = detail.Credit
				bankingTransaction.FromAccountAmount = detail.Credit
				bankingTransaction.CurrencyId = business.BaseCurrencyId
				bankingTransaction.FromAccountId = acc.ID
			}
			err = tx.Create(&bankingTransaction).Error
			if err != nil {
				config.LogError(logger, "OpeningBalanceWorkflow.go", "CreateOpeningBalance", "CreateBankingTransaction", bankingTransaction, err)
				return 0, nil, 0, err
			}
			bankingTransactionId = bankingTransaction.ID
		}

		accTransactions = append(accTransactions, models.AccountTransaction{
			BusinessId:           businessId,
			AccountId:            detail.AccountId,
			BranchId:             branchId,
			TransactionDateTime:  transactionTime,
			BaseCurrencyId:       baseCurrencyId,
			BaseDebit:            detail.Debit,
			BaseCredit:           detail.Credit,
			ForeignCurrencyId:    foreignCurrencyId,
			ForeignDebit:         decimal.NewFromInt(0),
			ForeignCredit:        decimal.NewFromInt(0),
			ExchangeRate:         decimal.NewFromInt(0),
			BankingTransactionId: bankingTransactionId,
		})
	}

	openingBalanceAdjustments := models.AccountTransaction{
		BusinessId:          businessId,
		AccountId:           systemAccounts[models.AccountCodeOpeningBalanceAdjustments],
		BranchId:            branchId,
		TransactionDateTime: transactionTime,
		BaseCurrencyId:      baseCurrencyId,
		ForeignCurrencyId:   foreignCurrencyId,
		ForeignCredit:       decimal.NewFromInt(0),
		ForeignDebit:        decimal.NewFromInt(0),
		ExchangeRate:        decimal.NewFromInt(0),
	}

	if totalDebit.LessThan(totalCredit) {
		openingBalanceAdjustments.BaseDebit = totalCredit.Sub(totalDebit)
		accountIds = append(accountIds, systemAccounts[models.AccountCodeOpeningBalanceAdjustments])
		accTransactions = append(accTransactions, openingBalanceAdjustments)
	} else if totalCredit.LessThan(totalDebit) {
		openingBalanceAdjustments.BaseCredit = totalDebit.Sub(totalCredit)
		accountIds = append(accountIds, systemAccounts[models.AccountCodeOpeningBalanceAdjustments])
		accTransactions = append(accTransactions, openingBalanceAdjustments)
	}

	journal := models.AccountJournal{
		BusinessId:          businessId,
		BranchId:            branchId,
		TransactionDateTime: transactionTime,
		TransactionNumber:   strconv.Itoa(openingBalance.ID),
		TransactionDetails:  "Opening Balance",
		ReferenceId:         openingBalance.ID,
		ReferenceType:       models.AccountReferenceTypeOpeningBalance,
		AccountTransactions: accTransactions,
	}

	err = tx.Create(&journal).Error
	if err != nil {
		config.LogError(logger, "OpeningBalanceWorkflow.go", "CreateOpeningBalance", "CreateAccountJournal", journal, err)
		return 0, nil, 0, err
	}
	return journal.ID, accountIds, foreignCurrencyId, nil
}

func DeleteOpeningBalance(tx *gorm.DB, logger *logrus.Logger, recordId int, recordType string, businessId string, business models.Business, oldOpeningBalance models.OpeningBalance) (int, []int, *time.Time, error) {

	var accountJournal models.AccountJournal
	accountIds := make([]int, 0)

	var count int64
	err := tx.Model(&accountJournal).Where("business_id = ? AND branch_id = ? AND reference_type = ?", businessId, oldOpeningBalance.BranchId, models.AccountReferenceTypeOpeningBalance).Count(&count).Error
	if err != nil {
		config.LogError(logger, "OpeningBalanceWorkflow.go", "DeleteOpeningBalance", "GetExistingAccountJournal", oldOpeningBalance, err)
		return 0, nil, nil, err
	}
	if count <= 0 {
		return 0, nil, nil, err
	}

	err = tx.Preload("AccountTransactions").Where("business_id = ? AND branch_id = ? AND reference_type = ?", businessId, oldOpeningBalance.BranchId, models.AccountReferenceTypeOpeningBalance).First(&accountJournal).Error
	if err != nil {
		config.LogError(logger, "OpeningBalanceWorkflow.go", "DeleteOpeningBalance", "GetExistingAccountJournal", oldOpeningBalance, err)
		return 0, nil, nil, err
	}
	for _, transaction := range accountJournal.AccountTransactions {
		if !slices.Contains(accountIds, transaction.AccountId) {
			accountIds = append(accountIds, transaction.AccountId)
		}
	}
	// Phase 1: do not delete posted journals; create a reversal journal instead.
	reversalID, err := ReverseAccountJournal(tx, &accountJournal, ReversalReasonOpeningBalanceResetVoid)
	if err != nil {
		config.LogError(logger, "OpeningBalanceWorkflow.go", "DeleteOpeningBalance", "ReverseAccountJournal", accountJournal, err)
		return 0, nil, nil, err
	}

	err = tx.Where("transaction_type = ?", models.BankingTransactionTypeOpeningBalance).Delete(&models.BankingTransaction{}).Error
	if err != nil {
		config.LogError(logger, "OpeningBalanceWorkflow.go", "DeleteOpeningBalance", "DeleteBankingTransaction", accountJournal, err)
		return 0, nil, nil, err
	}
	return reversalID, accountIds, &accountJournal.TransactionDateTime, nil
}
