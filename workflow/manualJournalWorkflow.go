package workflow

import (
	"encoding/json"
	"slices"

	"github.com/mmdatafocus/books_backend/config"
	"github.com/mmdatafocus/books_backend/models"
	"github.com/mmdatafocus/books_backend/utils"
	"github.com/shopspring/decimal"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

func ProcessManualJournalWorkflow(tx *gorm.DB, logger *logrus.Logger, msg config.PubSubMessage) error {
	var accountJournalId int
	var accountIds []int
	var foreignCurrencyId int
	business, err := models.GetBusinessById2(tx, msg.BusinessId)
	if err != nil {
		config.LogError(logger, "ManualJournalWorkflow.go", "ProcessManualJournalWorkflow", "GetBusiness", msg.BusinessId, err)
		return err
	}
	if msg.Action == string(models.PubSubMessageActionCreate) {

		var journal models.Journal
		err := json.Unmarshal([]byte(msg.NewObj), &journal)
		if err != nil {
			config.LogError(logger, "ManualJournalWorkflow.go", "ProcessManualJournalWorkflow > Create", "Unmarshal msg.NewObj", msg.NewObj, err)
			return err
		}
		accountJournalId, accountIds, foreignCurrencyId, err = CreateManualJournal(tx, logger, msg.ID, msg.ReferenceType, msg.BusinessId, *business, journal)
		if err != nil {
			config.LogError(logger, "ManualJournalWorkflow.go", "ProcessManualJournalWorkflow > Create", "CreateManualJournal", nil, err)
			return err
		}
		err = UpdateBalances(tx, logger, msg.BusinessId, business.BaseCurrencyId, journal.BranchId, accountIds, journal.JournalDate, foreignCurrencyId)
		if err != nil {
			config.LogError(logger, "ManualJournalWorkflow.go", "ProcessManualJournalWorkflow > Create", "UpdateBalances", journal, err)
			return err
		}
		err = UpdateBankBalances(tx, business.BaseCurrencyId, journal.BranchId, accountIds, journal.JournalDate)
		if err != nil {
			config.LogError(logger, "ManualJournalWorkflow.go", "ProcessManualJournalWorkflow > Create", "UpdateBankBalances", journal, err)
			return err
		}

	} else if msg.Action == string(models.PubSubMessageActionUpdate) {

		var oldForeignCurrencyId int
		var oldAccountIds []int
		var journal models.Journal
		err := json.Unmarshal([]byte(msg.NewObj), &journal)
		if err != nil {
			config.LogError(logger, "ManualJournalWorkflow.go", "ProcessManualJournalWorkflow > Update", "Unmarshal msg.NewObj", msg.NewObj, err)
			return err
		}
		var oldJournal models.Journal
		err = json.Unmarshal([]byte(msg.OldObj), &oldJournal)
		if err != nil {
			config.LogError(logger, "ManualJournalWorkflow.go", "ProcessManualJournalWorkflow > Update", "Unmarshal msg.OldObj", msg.OldObj, err)
			return err
		}
		_, oldAccountIds, oldForeignCurrencyId, err = DeleteManualJournal(tx, logger, msg.ID, msg.ReferenceType, msg.BusinessId, *business, oldJournal)
		if err != nil {
			config.LogError(logger, "ManualJournalWorkflow.go", "ProcessManualJournalWorkflow > Update", "DeleteManualJournal", nil, err)
			return err
		}
		accountJournalId, accountIds, foreignCurrencyId, err = CreateManualJournal(tx, logger, msg.ID, msg.ReferenceType, msg.BusinessId, *business, journal)
		if err != nil {
			config.LogError(logger, "ManualJournalWorkflow.go", "ProcessManualJournalWorkflow > Update", "CreateManualJournal", nil, err)
			return err
		}
		if oldJournal.BranchId != journal.BranchId || oldJournal.JournalDate != journal.JournalDate || oldForeignCurrencyId != foreignCurrencyId {
			err = UpdateBalances(tx, logger, msg.BusinessId, business.BaseCurrencyId, oldJournal.BranchId, oldAccountIds, oldJournal.JournalDate, oldForeignCurrencyId)
			if err != nil {
				config.LogError(logger, "ManualJournalWorkflow.go", "ProcessManualJournalWorkflow > Update", "UpdateBalances Old", oldJournal, err)
				return err
			}
			err = UpdateBankBalances(tx, business.BaseCurrencyId, oldJournal.BranchId, oldAccountIds, oldJournal.JournalDate)
			if err != nil {
				config.LogError(logger, "ManualJournalWorkflow.go", "ProcessManualJournalWorkflow > Update", "UpdateBankBalances Old", oldJournal, err)
				return err
			}
		} else {
			accountIds = utils.MergeIntSlices(accountIds, oldAccountIds)
		}
		err = UpdateBalances(tx, logger, msg.BusinessId, business.BaseCurrencyId, journal.BranchId, accountIds, journal.JournalDate, foreignCurrencyId)
		if err != nil {
			config.LogError(logger, "ManualJournalWorkflow.go", "ProcessManualJournalWorkflow > Update", "UpdateBalances", journal, err)
			return err
		}
		err = UpdateBankBalances(tx, business.BaseCurrencyId, journal.BranchId, accountIds, journal.JournalDate)
		if err != nil {
			config.LogError(logger, "ManualJournalWorkflow.go", "ProcessManualJournalWorkflow > Update", "UpdateBankBalances", journal, err)
			return err
		}

	} else if msg.Action == string(models.PubSubMessageActionDelete) {

		var oldJournal models.Journal
		err = json.Unmarshal([]byte(msg.OldObj), &oldJournal)
		if err != nil {
			config.LogError(logger, "ManualJournalWorkflow.go", "ProcessManualJournalWorkflow > Delete", "Unmarshal msg.OldObj", msg.OldObj, err)
			return err
		}
		accountJournalId, accountIds, foreignCurrencyId, err = DeleteManualJournal(tx, logger, msg.ID, msg.ReferenceType, msg.BusinessId, *business, oldJournal)
		if err != nil {
			config.LogError(logger, "ManualJournalWorkflow.go", "ProcessManualJournalWorkflow > Delete", "DeleteManualJournal", nil, err)
			return err
		}
		err = UpdateBalances(tx, logger, msg.BusinessId, business.BaseCurrencyId, oldJournal.BranchId, accountIds, oldJournal.JournalDate, foreignCurrencyId)
		if err != nil {
			config.LogError(logger, "ManualJournalWorkflow.go", "ProcessManualJournalWorkflow > Delete", "UpdateBalances", oldJournal, err)
			return err
		}
		err = UpdateBankBalances(tx, business.BaseCurrencyId, oldJournal.BranchId, accountIds, oldJournal.JournalDate)
		if err != nil {
			config.LogError(logger, "ManualJournalWorkflow.go", "ProcessManualJournalWorkflow > Delete", "UpdateBankBalances", oldJournal, err)
			return err
		}
	}
	err = tx.Model(&models.PubSubMessageRecord{}).Where("id=?", msg.ID).Updates(map[string]interface{}{"account_journal_id": accountJournalId, "is_processed": true}).Error
	if err != nil {
		config.LogError(logger, "ManualJournalWorkflow.go", "ProcessManualJournalWorkflow", "UpdatePubSubMessageRecord", accountJournalId, err)
		return err
	}
	return nil
}

func CreateManualJournal(tx *gorm.DB, logger *logrus.Logger, recordId int, recordType string, businessId string, business models.Business, journal models.Journal) (int, []int, int, error) {

	exchangeRate := journal.ExchangeRate
	transactionTime := journal.JournalDate
	baseCurrencyId := business.BaseCurrencyId
	foreignCurrencyId := journal.CurrencyId
	journalCurrencyId := journal.CurrencyId
	branchId := journal.BranchId
	accountIds := make([]int, 0)
	accCurrencyId := 0
	baseDebit := decimal.NewFromInt(0)
	baseCredit := decimal.NewFromInt(0)
	foreignDebit := decimal.NewFromInt(0)
	foreignCredit := decimal.NewFromInt(0)

	accTransactions := make([]models.AccountTransaction, len(journal.Transactions))
	for i, transact := range journal.Transactions {
		if !slices.Contains(accountIds, transact.ID) {
			accountIds = append(accountIds, transact.AccountId)
		}
		acc, err := GetAccount(tx, transact.AccountId)
		if err != nil {
			config.LogError(logger, "ManualJournalWorkflow.go", "CreateManualJournal", "GetAccount", transact, err)
			return 0, nil, 0, err
		}
		accCurrencyId = acc.CurrencyId
		baseDebit = transact.Debit
		baseCredit = transact.Credit
		if accCurrencyId == 0 || accCurrencyId == baseCurrencyId { // base-currency account
			if baseCurrencyId != journalCurrencyId {
				foreignDebit = baseDebit
				baseDebit = foreignDebit.Mul(exchangeRate)
				foreignCredit = baseCredit
				baseCredit = foreignCredit.Mul(exchangeRate)
				accCurrencyId = journalCurrencyId
				foreignCurrencyId = journalCurrencyId
			} else {
				accCurrencyId = baseCurrencyId
			}
		} else { // foreign currency account
			if baseCurrencyId != journalCurrencyId {
				foreignDebit = baseDebit
				baseDebit = foreignDebit.Mul(exchangeRate)
				foreignCredit = baseCredit
				baseCredit = foreignCredit.Mul(exchangeRate)
				accCurrencyId = journalCurrencyId
				foreignCurrencyId = journalCurrencyId
			} else {
				if !exchangeRate.IsZero() {
					foreignDebit = baseDebit.DivRound(exchangeRate, 4)
					foreignCredit = baseCredit.DivRound(exchangeRate, 4)
				}
				foreignCurrencyId = accCurrencyId
			}
		}

		bankingTransactionId := 0
		if acc.DetailType == models.AccountDetailTypeCash ||
			acc.DetailType == models.AccountDetailTypeBank {

			bankingTransaction := models.BankingTransaction{
				BusinessId:        businessId,
				BranchId:          journal.BranchId,
				SupplierId:        journal.SupplierId,
				CustomerId:        journal.CustomerId,
				TransactionDate:   journal.JournalDate,
				TransactionId:     journal.ID,
				TransactionNumber: journal.JournalNumber,
				TransactionType:   models.BankingTransactionTypeManualJournal,
				ExchangeRate:      exchangeRate,
				ReferenceNumber:   journal.ReferenceNumber,
				Description:       journal.JournalNotes,
			}
			if !baseDebit.IsZero() {
				bankingTransaction.Amount = transact.Debit
				bankingTransaction.ToAccountAmount = transact.Debit
				bankingTransaction.CurrencyId = journal.CurrencyId
				bankingTransaction.ToAccountId = acc.ID
			} else {
				bankingTransaction.Amount = transact.Credit
				bankingTransaction.FromAccountAmount = transact.Credit
				bankingTransaction.CurrencyId = journal.CurrencyId
				bankingTransaction.FromAccountId = acc.ID
			}
			err = tx.Create(&bankingTransaction).Error
			if err != nil {
				config.LogError(logger, "ManualJournalWorkflow.go", "CreateManualJournal", "CreateBankingTransaction", bankingTransaction, err)
				return 0, nil, 0, err
			}
			bankingTransactionId = bankingTransaction.ID
		}

		accTransactions[i] = models.AccountTransaction{
			BusinessId:           businessId,
			AccountId:            transact.AccountId,
			BranchId:             branchId,
			TransactionDateTime:  transactionTime,
			Description:          transact.Description,
			BaseCurrencyId:       baseCurrencyId,
			BaseDebit:            baseDebit,
			BaseCredit:           baseCredit,
			ForeignCurrencyId:    accCurrencyId,
			ForeignDebit:         foreignDebit,
			ForeignCredit:        foreignCredit,
			ExchangeRate:         exchangeRate,
			BankingTransactionId: bankingTransactionId,
		}
	}

	accJournal := models.AccountJournal{
		BusinessId:          businessId,
		BranchId:            journal.BranchId,
		TransactionDateTime: journal.JournalDate,
		TransactionNumber:   journal.JournalNumber,
		TransactionDetails:  journal.JournalNotes,
		ReferenceNumber:     journal.ReferenceNumber,
		CustomerId:          journal.CustomerId,
		SupplierId:          journal.SupplierId,
		ReferenceId:         journal.ID,
		ReferenceType:       models.AccountReferenceTypeJournal,
		AccountTransactions: accTransactions,
	}

	err := tx.Create(&accJournal).Error
	if err != nil {
		config.LogError(logger, "ManualJournalWorkflow.go", "CreateManualJournal", "CreateAccountJournal", accJournal, err)
		return 0, nil, 0, err
	}

	return accJournal.ID, accountIds, foreignCurrencyId, nil
}

func DeleteManualJournal(tx *gorm.DB, logger *logrus.Logger, recordId int, recordType string, businessId string, business models.Business, oldJournal models.Journal) (int, []int, int, error) {

	foreignCurrencyId := oldJournal.CurrencyId
	accountJournal, _, accountIds, err := GetExistingAccountJournal(tx, oldJournal.ID, models.AccountReferenceTypeJournal)
	if err != nil {
		config.LogError(logger, "ManualJournalWorkflow.go", "DeleteManualJournal", "GetExistingAccountJournal", oldJournal, err)
		return 0, nil, 0, err
	}

	// Phase 1: do not delete posted journals; create a reversal journal instead.
	reversalID, err := ReverseAccountJournal(tx, accountJournal, ReversalReasonManualJournalVoidUpdate)
	if err != nil {
		config.LogError(logger, "ManualJournalWorkflow.go", "DeleteManualJournal", "ReverseAccountJournal", accountJournal, err)
		return 0, nil, 0, err
	}

	err = tx.Where("transaction_id = ? AND transaction_type = ?", oldJournal.ID, models.BankingTransactionTypeManualJournal).Delete(&models.BankingTransaction{}).Error
	if err != nil {
		config.LogError(logger, "ManualJournalWorkflow.go", "DeleteManualJournal", "DeleteBankingTransaction", accountJournal, err)
		return 0, nil, 0, err
	}

	return reversalID, accountIds, foreignCurrencyId, nil
}
