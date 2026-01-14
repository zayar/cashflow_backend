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

func ProcessSupplierAdvanceAppliedWorkflow(tx *gorm.DB, logger *logrus.Logger, msg config.PubSubMessage) error {

	var accountJournalId int
	var accountIds []int
	var foreignCurrencyId int
	business, err := models.GetBusinessById2(tx, msg.BusinessId)
	if err != nil {
		config.LogError(logger, "SupplierAdvanceAppliedWorkflow.go", "ProcessSupplierAdvanceAppliedWorkflow", "GetBusiness", msg.BusinessId, err)
		return err
	}
	if msg.Action == string(models.PubSubMessageActionCreate) {

		var supplierCreditBill models.SupplierCreditBill
		err := json.Unmarshal([]byte(msg.NewObj), &supplierCreditBill)
		if err != nil {
			config.LogError(logger, "SupplierAdvanceAppliedWorkflow.go", "ProcessSupplierAdvanceAppliedWorkflow > Create", "Unmarshal msg.NewObj", msg.NewObj, err)
			return err
		}
		accountJournalId, accountIds, foreignCurrencyId, err = CreateSupplierAdvanceApplied(tx, logger, msg.ID, msg.ReferenceType, msg.BusinessId, *business, supplierCreditBill)
		if err != nil {
			config.LogError(logger, "SupplierAdvanceAppliedWorkflow.go", "ProcessSupplierAdvanceAppliedWorkflow > Create", "CreateSupplierAdvanceApplied", nil, err)
			return err
		}
		err = UpdateBalances(tx, logger, msg.BusinessId, business.BaseCurrencyId, supplierCreditBill.BranchId, accountIds, supplierCreditBill.CreditDate, foreignCurrencyId)
		if err != nil {
			config.LogError(logger, "SupplierAdvanceAppliedWorkflow.go", "ProcessSupplierAdvanceAppliedWorkflow > Create", "UpdateBalances", supplierCreditBill, err)
			return err
		}

	} else if msg.Action == string(models.PubSubMessageActionUpdate) {

		var oldForeignCurrencyId int
		var oldAccountIds []int
		var supplierCreditBill models.SupplierCreditBill
		err := json.Unmarshal([]byte(msg.NewObj), &supplierCreditBill)
		if err != nil {
			config.LogError(logger, "SupplierAdvanceAppliedWorkflow.go", "ProcessSupplierAdvanceAppliedWorkflow > Update", "Unmarshal msg.NewObj", msg.NewObj, err)
			return err
		}
		var oldSupplierCreditBill models.SupplierCreditBill
		err = json.Unmarshal([]byte(msg.OldObj), &oldSupplierCreditBill)
		if err != nil {
			config.LogError(logger, "SupplierAdvanceAppliedWorkflow.go", "ProcessSupplierAdvanceAppliedWorkflow > Update", "Unmarshal msg.OldObj", msg.OldObj, err)
			return err
		}
		_, oldAccountIds, oldForeignCurrencyId, err = DeleteSupplierAdvanceApplied(tx, logger, msg.ID, msg.ReferenceType, msg.BusinessId, *business, oldSupplierCreditBill)
		if err != nil {
			config.LogError(logger, "SupplierAdvanceAppliedWorkflow.go", "ProcessSupplierAdvanceAppliedWorkflow > Update", "DeleteSupplierAdvanceApplied", nil, err)
			return err
		}
		accountJournalId, accountIds, foreignCurrencyId, err = CreateSupplierAdvanceApplied(tx, logger, msg.ID, msg.ReferenceType, msg.BusinessId, *business, supplierCreditBill)
		if err != nil {
			config.LogError(logger, "SupplierAdvanceAppliedWorkflow.go", "ProcessSupplierAdvanceAppliedWorkflow > Update", "CreateSupplierAdvanceApplied", nil, err)
			return err
		}
		if oldSupplierCreditBill.BranchId != supplierCreditBill.BranchId || oldSupplierCreditBill.CreditDate != supplierCreditBill.CreditDate || oldForeignCurrencyId != foreignCurrencyId {
			err = UpdateBalances(tx, logger, msg.BusinessId, business.BaseCurrencyId, oldSupplierCreditBill.BranchId, oldAccountIds, oldSupplierCreditBill.CreditDate, oldForeignCurrencyId)
			if err != nil {
				config.LogError(logger, "SupplierAdvanceAppliedWorkflow.go", "ProcessSupplierAdvanceAppliedWorkflow > Update", "UpdateBalances Old", oldSupplierCreditBill, err)
				return err
			}
		} else {
			accountIds = utils.MergeIntSlices(accountIds, oldAccountIds)
		}
		err = UpdateBalances(tx, logger, msg.BusinessId, business.BaseCurrencyId, supplierCreditBill.BranchId, accountIds, supplierCreditBill.CreditDate, foreignCurrencyId)
		if err != nil {
			config.LogError(logger, "SupplierAdvanceAppliedWorkflow.go", "ProcessSupplierAdvanceAppliedWorkflow > Update", "UpdateBalances", supplierCreditBill, err)
			return err
		}
	} else if msg.Action == string(models.PubSubMessageActionDelete) {

		var oldSupplierCreditBill models.SupplierCreditBill
		err = json.Unmarshal([]byte(msg.OldObj), &oldSupplierCreditBill)
		if err != nil {
			config.LogError(logger, "SupplierAdvanceAppliedWorkflow.go", "ProcessSupplierAdvanceAppliedWorkflow > Delete", "Unmarshal msg.OldObj", msg.OldObj, err)
			return err
		}
		accountJournalId, accountIds, foreignCurrencyId, err = DeleteSupplierAdvanceApplied(tx, logger, msg.ID, msg.ReferenceType, msg.BusinessId, *business, oldSupplierCreditBill)
		if err != nil {
			config.LogError(logger, "SupplierAdvanceAppliedWorkflow.go", "ProcessSupplierAdvanceAppliedWorkflow > Delete", "DeleteSupplierAdvanceApplied", nil, err)
			return err
		}
		err = UpdateBalances(tx, logger, msg.BusinessId, business.BaseCurrencyId, oldSupplierCreditBill.BranchId, accountIds, oldSupplierCreditBill.CreditDate, foreignCurrencyId)
		if err != nil {
			config.LogError(logger, "SupplierAdvanceAppliedWorkflow.go", "ProcessSupplierAdvanceAppliedWorkflow > Delete", "UpdateBalances", oldSupplierCreditBill, err)
			return err
		}
	}
	err = tx.Model(&models.PubSubMessageRecord{}).Where("id=?", msg.ID).Updates(map[string]interface{}{"account_journal_id": accountJournalId, "is_processed": true}).Error
	if err != nil {
		config.LogError(logger, "SupplierAdvanceAppliedWorkflow.go", "ProcessSupplierAdvanceAppliedWorkflow", "UpdatePubSubMessageRecord", accountJournalId, err)
		return err
	}
	return nil
}

func CreateSupplierAdvanceApplied(tx *gorm.DB, logger *logrus.Logger, recordId int, recordType string, businessId string, business models.Business, supplierCreditBill models.SupplierCreditBill) (int, []int, int, error) {

	systemAccounts, err := models.GetSystemAccounts(businessId)
	if err != nil {
		config.LogError(logger, "SupplierAdvanceAppliedWorkflow.go", "CreateSupplierAdvanceApplied", "GetSystemAccounts", businessId, err)
		return 0, nil, 0, err
	}

	transactionTime := supplierCreditBill.CreditDate
	branchId := supplierCreditBill.BranchId
	baseCurrencyId := business.BaseCurrencyId
	foreignCurrencyId := supplierCreditBill.CurrencyId

	baseBillAmount := supplierCreditBill.Amount
	foreignBillAmount := decimal.NewFromInt(0)
	baseAmount := supplierCreditBill.Amount
	foreignAmount := decimal.NewFromInt(0)
	exchangeRate := supplierCreditBill.ExchangeRate
	billExchangeRate := supplierCreditBill.BillExchangeRate

	if baseCurrencyId != foreignCurrencyId {
		foreignAmount = baseAmount
		baseAmount = foreignAmount.Mul(exchangeRate)
	} else if baseCurrencyId == foreignCurrencyId && supplierCreditBill.BillCurrencyId != baseCurrencyId {
		foreignAmount = baseAmount.DivRound(exchangeRate, 4)
	}

	if baseCurrencyId == foreignCurrencyId && supplierCreditBill.BillCurrencyId != baseCurrencyId {
		foreignBillAmount = foreignAmount
		baseBillAmount = foreignBillAmount.Mul(billExchangeRate)
		foreignCurrencyId = supplierCreditBill.BillCurrencyId
	} else if baseCurrencyId != foreignCurrencyId && supplierCreditBill.BillCurrencyId != baseCurrencyId {
		foreignBillAmount = baseBillAmount
		baseBillAmount = foreignBillAmount.Mul(billExchangeRate)
	} else if baseCurrencyId != foreignCurrencyId && supplierCreditBill.BillCurrencyId != foreignCurrencyId {
		foreignBillAmount = baseBillAmount
		baseBillAmount = foreignBillAmount.Mul(billExchangeRate)
	}

	accountIds := make([]int, 0)
	accTransactions := make([]models.AccountTransaction, 0)

	accountsPayable := models.AccountTransaction{
		BusinessId:          businessId,
		AccountId:           systemAccounts[models.AccountCodeAccountsPayable],
		BranchId:            branchId,
		TransactionDateTime: transactionTime,
		BaseCurrencyId:      baseCurrencyId,
		BaseDebit:           baseBillAmount,
		BaseCredit:          decimal.NewFromInt(0),
		ForeignCurrencyId:   foreignCurrencyId,
		ForeignDebit:        foreignBillAmount,
		ForeignCredit:       decimal.NewFromInt(0),
		ExchangeRate:        billExchangeRate,
	}
	accTransactions = append(accTransactions, accountsPayable)
	accountIds = append(accountIds, systemAccounts[models.AccountCodeAccountsPayable])

	advancePayment := models.AccountTransaction{
		BusinessId:          businessId,
		AccountId:           systemAccounts[models.AccountCodeAdvancePayment],
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
	accTransactions = append(accTransactions, advancePayment)
	accountIds = append(accountIds, systemAccounts[models.AccountCodeAdvancePayment])

	if !baseBillAmount.Equals(baseAmount) {
		gainLossAmount := baseAmount.Sub(baseBillAmount)
		if gainLossAmount.IsPositive() {
			exchangeGainLoss := models.AccountTransaction{
				BusinessId:          businessId,
				AccountId:           systemAccounts[models.AccountCodeExchangeGainOrLoss],
				BranchId:            branchId,
				TransactionDateTime: transactionTime,
				BaseCurrencyId:      baseCurrencyId,
				BaseDebit:           gainLossAmount.Abs(),
				BaseCredit:          decimal.NewFromInt(0),
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
				BaseDebit:           decimal.NewFromInt(0),
				BaseCredit:          gainLossAmount.Abs(),
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
		BranchId:            supplierCreditBill.BranchId,
		TransactionDateTime: supplierCreditBill.CreditDate,
		TransactionNumber:   strconv.Itoa(supplierCreditBill.ID),
		TransactionDetails:  "",
		ReferenceNumber:     supplierCreditBill.BillNumber,
		ReferenceId:         supplierCreditBill.ID,
		ReferenceType:       models.AccountReferenceTypeSupplierAdvanceApplied,
		SupplierId:          supplierCreditBill.SupplierId,
		AccountTransactions: accTransactions,
	}

	err = tx.Create(&accJournal).Error
	if err != nil {
		config.LogError(logger, "SupplierAdvanceAppliedWorkflow.go", "CreateSupplierAdvanceApplied", "CreateAccountJournal", accJournal, err)
		return 0, nil, 0, err
	}
	return accJournal.ID, accountIds, foreignCurrencyId, nil
}

func DeleteSupplierAdvanceApplied(tx *gorm.DB, logger *logrus.Logger, recordId int, recordType string, businessId string, business models.Business, oldSupplierCreditBill models.SupplierCreditBill) (int, []int, int, error) {

	foreignCurrencyId := oldSupplierCreditBill.CurrencyId
	accountJournal, _, accountIds, err := GetExistingAccountJournal(tx, oldSupplierCreditBill.ID, models.AccountReferenceTypeSupplierAdvanceApplied)
	if err != nil {
		config.LogError(logger, "SupplierAdvanceAppliedWorkflow.go", "DeleteSupplierAdvanceApplied", "GetExistingAccountJournal", oldSupplierCreditBill, err)
		return 0, nil, 0, err
	}

	// Phase 1: do not delete posted journals; create a reversal journal instead.
	reversalID, err := ReverseAccountJournal(tx, accountJournal, ReversalReasonSupplierAdvanceAppliedVoidUpdate)
	if err != nil {
		config.LogError(logger, "SupplierAdvanceAppliedWorkflow.go", "DeleteSupplierAdvanceApplied", "ReverseAccountJournal", accountJournal, err)
		return 0, nil, 0, err
	}
	return reversalID, accountIds, foreignCurrencyId, nil
}
