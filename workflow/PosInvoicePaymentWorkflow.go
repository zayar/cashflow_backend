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

func ProcessPosInvoicePaymentWorkflow(tx *gorm.DB, logger *logrus.Logger, msg config.PubSubMessage) error {

	var accountJournalId int
	var accountIds []int
	var foreignCurrencyId int
	var stockHistories []*models.StockHistory
	business, err := models.GetBusinessById2(tx, msg.BusinessId)
	if err != nil {
		config.LogError(logger, "PosInvoicePaymentWorkflow.go", "ProcessPosInvoicePaymentWorkflow", "GetBusiness", msg.BusinessId, err)
		return err
	}
	if msg.Action == string(models.PubSubMessageActionCreate) {

		var invoice models.SalesInvoice
		err := json.Unmarshal([]byte(msg.NewObj), &invoice)
		if err != nil {
			config.LogError(logger, "PosInvoicePaymentWorkflow.go", "ProcessPosInvoicePaymentWorkflow > Create", "Unmarshal msg.NewObj", msg.NewObj, err)
			return err
		}
		accountJournalId, accountIds, foreignCurrencyId, stockHistories, err = CreateInvoice(tx, logger, msg.ID, msg.ReferenceType, msg.BusinessId, *business, invoice)
		if err != nil {
			config.LogError(logger, "PosInvoicePaymentWorkflow.go", "ProcessPosInvoicePaymentWorkflow > Create", "CreateInvoice", nil, err)
			return err
		}
		valuationAccountIds, err := ProcessOutgoingStocks(tx, logger, stockHistories)
		if err != nil {
			config.LogError(logger, "PosInvoicePaymentWorkflow.go", "ProcessPosInvoicePaymentWorkflow > Create", "ProcessInventoryValuation", stockHistories, err)
			return err
		}

		if len(valuationAccountIds) > 0 {
			for _, accId := range valuationAccountIds {
				if !slices.Contains(accountIds, accId) {
					accountIds = append(accountIds, accId)
				}
			}
		}
		err = UpdateBalances(tx, logger, msg.BusinessId, business.BaseCurrencyId, invoice.BranchId, accountIds, invoice.InvoiceDate, foreignCurrencyId)
		if err != nil {
			config.LogError(logger, "PosInvoicePaymentWorkflow.go", "ProcessPosInvoicePaymentWorkflow > Create", "UpdateBalances", invoice, err)
			return err
		}
	}

	err = tx.Model(&models.PubSubMessageRecord{}).Where("id=?", msg.ID).Updates(map[string]interface{}{"account_journal_id": accountJournalId, "is_processed": true}).Error
	if err != nil {
		config.LogError(logger, "PosInvoicePaymentWorkflow.go", "ProcessPosInvoicePaymentWorkflow", "UpdatePubSubMessageRecord", accountJournalId, err)
		return err
	}
	return nil
}

func CreatePosInvoicePayment(tx *gorm.DB, logger *logrus.Logger, recordId int, recordType string, businessId string, business models.Business, invoice models.PosCheckoutInvoicePayment) (int, []int, int, []*models.StockHistory, error) {

	systemAccounts, err := models.GetSystemAccounts(businessId)
	if err != nil {
		config.LogError(logger, "PosInvoicePaymentWorkflow.go", "CreatePosInvoicePayment", "GetSystemAccounts", businessId, err)
		return 0, nil, 0, nil, err
	}

	paymentCurrencyId := invoice.CurrencyId

	exchangeRate := invoice.ExchangeRate
	transactionTime := invoice.InvoiceDate
	baseCurrencyId := business.BaseCurrencyId
	foreignCurrencyId := invoice.CurrencyId
	branchId := invoice.BranchId
	baseDiscountAmount := invoice.InvoiceTotalDiscountAmount
	baseTaxAmount := invoice.InvoiceTotalTaxAmount
	baseAdjustmentAmount := invoice.AdjustmentAmount
	baseShippingCharges := invoice.ShippingCharges
	foreignDiscountAmount := decimal.NewFromInt(0)
	foreignTaxAmount := decimal.NewFromInt(0)
	foreignAdjustmentAmount := decimal.NewFromInt(0)
	foreignShippingCharges := decimal.NewFromInt(0)
	// for customer payment
	invoiceAmount := decimal.NewFromInt(0)
	baseBankCharges := invoice.BankCharges
	foreignBankCharges := decimal.NewFromInt(0)
	baseTotalAmount := decimal.NewFromInt(0)
	foreignTotalAmount := decimal.NewFromInt(0)

	if baseCurrencyId != foreignCurrencyId {
		foreignDiscountAmount = baseDiscountAmount
		baseDiscountAmount = foreignDiscountAmount.Mul(exchangeRate)
		foreignTaxAmount = baseTaxAmount
		baseTaxAmount = foreignTaxAmount.Mul(exchangeRate)
		foreignAdjustmentAmount = baseAdjustmentAmount
		foreignShippingCharges = baseShippingCharges
		baseAdjustmentAmount = foreignAdjustmentAmount.Mul(exchangeRate)
		baseShippingCharges = foreignShippingCharges.Mul(exchangeRate)
	}

	// for customer payment
	depositAccount, err := GetAccount(tx, invoice.DepositAccountId)
	if err != nil {
		config.LogError(logger, "PosInvoicePaymentWorkflow.go", "CreatePosInvoicePayment", "GetDepositAccount", invoice.DepositAccountId, err)
		return 0, nil, 0, nil, err
	}

	invoiceAmount = invoiceAmount.Add(invoice.InvoiceTotalAmount)
	if baseCurrencyId != foreignCurrencyId {
		foreignCurrencyId = paymentCurrencyId
	}

	depositAccountCurrencyId := 0
	if depositAccount.CurrencyId == 0 || depositAccount.CurrencyId == baseCurrencyId { // base-currency account
		if baseCurrencyId != paymentCurrencyId {
			// foreignTotalAmount = invoiceAmount.Add(customerPayment.BankCharges)
			foreignTotalAmount = invoiceAmount.Sub(invoice.BankCharges)
			baseTotalAmount = foreignTotalAmount.Mul(exchangeRate)
			foreignBankCharges = invoice.BankCharges
			baseBankCharges = foreignBankCharges.Mul(exchangeRate)
			depositAccountCurrencyId = paymentCurrencyId
			foreignCurrencyId = paymentCurrencyId
		} else {
			// baseTotalAmount = invoiceAmount.Add(invoice.BankCharges)
			baseTotalAmount = invoiceAmount.Sub(invoice.BankCharges)
			baseBankCharges = invoice.BankCharges
			depositAccountCurrencyId = baseCurrencyId
		}
	} else { // foreign currency account
		if baseCurrencyId != paymentCurrencyId {
			// foreignTotalAmount = invoiceAmount.Add(invoice.BankCharges)
			foreignTotalAmount = invoiceAmount.Sub(invoice.BankCharges)
			baseTotalAmount = foreignTotalAmount.Mul(exchangeRate)
			foreignBankCharges = invoice.BankCharges
			baseBankCharges = foreignBankCharges.Mul(exchangeRate)
			depositAccountCurrencyId = paymentCurrencyId
			foreignCurrencyId = paymentCurrencyId
		} else {
			if exchangeRate.IsZero() {
				// baseTotalAmount = invoiceAmount.Add(invoice.BankCharges)
				baseTotalAmount = invoiceAmount.Sub(invoice.BankCharges)
				foreignTotalAmount = decimal.NewFromInt(0)
				baseBankCharges = invoice.BankCharges
				foreignBankCharges = decimal.NewFromInt(0)
				depositAccountCurrencyId = depositAccount.CurrencyId
				foreignCurrencyId = depositAccount.CurrencyId
			} else {
				// baseTotalAmount = invoiceAmount.Add(invoice.BankCharges)
				baseTotalAmount = invoiceAmount.Sub(invoice.BankCharges)
				foreignTotalAmount = baseTotalAmount.DivRound(exchangeRate, 4)
				baseBankCharges = invoice.BankCharges
				foreignBankCharges = baseBankCharges.DivRound(exchangeRate, 4)
				depositAccountCurrencyId = depositAccount.CurrencyId
				foreignCurrencyId = depositAccount.CurrencyId
			}
		}
	}

	accountIds := make([]int, 0)
	accTransactions := make([]models.AccountTransaction, 0)

	if !baseDiscountAmount.IsZero() {
		discount := models.AccountTransaction{
			BusinessId:          businessId,
			AccountId:           systemAccounts[models.AccountCodeDiscount],
			BranchId:            branchId,
			TransactionDateTime: transactionTime,
			BaseCurrencyId:      baseCurrencyId,
			BaseCredit:          decimal.NewFromInt(0),
			BaseDebit:           baseDiscountAmount,
			ForeignCurrencyId:   foreignCurrencyId,
			ForeignCredit:       decimal.NewFromInt(0),
			ForeignDebit:        foreignDiscountAmount,
			ExchangeRate:        exchangeRate,
		}
		accountIds = append(accountIds, systemAccounts[models.AccountCodeDiscount])
		accTransactions = append(accTransactions, discount)
	}

	if !baseTaxAmount.IsZero() {
		taxPayable := models.AccountTransaction{
			BusinessId:          businessId,
			AccountId:           systemAccounts[models.AccountCodeTaxPayable],
			BranchId:            branchId,
			TransactionDateTime: transactionTime,
			BaseCurrencyId:      baseCurrencyId,
			BaseCredit:          baseTaxAmount,
			BaseDebit:           decimal.NewFromInt(0),
			ForeignCurrencyId:   foreignCurrencyId,
			ForeignCredit:       foreignTaxAmount,
			ForeignDebit:        decimal.NewFromInt(0),
			ExchangeRate:        exchangeRate,
		}
		accountIds = append(accountIds, systemAccounts[models.AccountCodeTaxPayable])
		accTransactions = append(accTransactions, taxPayable)
	}

	if !baseShippingCharges.IsZero() {
		shippingCharges := models.AccountTransaction{
			BusinessId:          businessId,
			AccountId:           systemAccounts[models.AccountCodeShippingCharge],
			BranchId:            branchId,
			TransactionDateTime: transactionTime,
			BaseCurrencyId:      baseCurrencyId,
			BaseDebit:           decimal.NewFromInt(0),
			BaseCredit:          baseShippingCharges,
			ForeignCurrencyId:   foreignCurrencyId,
			ForeignDebit:        decimal.NewFromInt(0),
			ForeignCredit:       foreignShippingCharges,
			ExchangeRate:        exchangeRate,
		}
		accountIds = append(accountIds, systemAccounts[models.AccountCodeShippingCharge])
		accTransactions = append(accTransactions, shippingCharges)
	}

	if !baseAdjustmentAmount.IsZero() {
		var otherCharges models.AccountTransaction
		if baseAdjustmentAmount.IsNegative() {
			otherCharges = models.AccountTransaction{
				BusinessId:          businessId,
				AccountId:           systemAccounts[models.AccountCodeOtherCharges],
				BranchId:            branchId,
				TransactionDateTime: transactionTime,
				BaseCurrencyId:      baseCurrencyId,
				BaseCredit:          decimal.NewFromInt(0),
				BaseDebit:           baseAdjustmentAmount.Abs(),
				ForeignCurrencyId:   foreignCurrencyId,
				ForeignCredit:       decimal.NewFromInt(0),
				ForeignDebit:        foreignAdjustmentAmount,
				ExchangeRate:        exchangeRate,
			}
		} else {
			otherCharges = models.AccountTransaction{
				BusinessId:          businessId,
				AccountId:           systemAccounts[models.AccountCodeOtherCharges],
				BranchId:            branchId,
				TransactionDateTime: transactionTime,
				BaseCurrencyId:      baseCurrencyId,
				BaseCredit:          baseAdjustmentAmount.Abs(),
				BaseDebit:           decimal.NewFromInt(0),
				ForeignCurrencyId:   foreignCurrencyId,
				ForeignCredit:       foreignAdjustmentAmount,
				ForeignDebit:        decimal.NewFromInt(0),
				ExchangeRate:        exchangeRate,
			}
		}
		accountIds = append(accountIds, systemAccounts[models.AccountCodeOtherCharges])
		accTransactions = append(accTransactions, otherCharges)
	}

	detailAccounts := make(map[int]decimal.Decimal)
	stockHistories := make([]*models.StockHistory, 0)
	productPurchaseAccounts := make(map[int]decimal.Decimal)
	productInventoryAccounts := make(map[int]decimal.Decimal)

	var saleInvoice models.SalesInvoice

	err = tx.First(&saleInvoice, invoice.SalesInvoiceId).Error
	if err != nil {
		config.LogError(logger, "PosInvoicePaymentWorkflow.go", "CreatePosInvoicePayment", "GetSaleInvoice", invoice.SalesInvoiceId, err)
		return 0, nil, 0, nil, err
	}

	stockDate, err := utils.ConvertToDate(invoice.InvoiceDate, business.Timezone)
	if err != nil {
		return 0, nil, 0, nil, err
	}

	for _, invoiceDetail := range saleInvoice.Details {
		detailAccountId := invoiceDetail.DetailAccountId
		if detailAccountId == 0 {
			detailAccountId = systemAccounts[models.AccountCodeSales]
		}
		amount, ok := detailAccounts[detailAccountId]
		if !ok {
			amount = decimal.NewFromInt(0)
		}
		if invoice.IsTaxInclusive != nil && *invoice.IsTaxInclusive {
			amount = amount.Add(invoiceDetail.DetailTotalAmount.Add(invoiceDetail.DetailDiscountAmount).Sub(invoiceDetail.DetailTaxAmount))
		} else {
			amount = amount.Add(invoiceDetail.DetailTotalAmount.Add(invoiceDetail.DetailDiscountAmount))
		}
		detailAccounts[detailAccountId] = amount

		if invoiceDetail.ProductId > 0 {
			productDetail, err := GetProductDetail(tx, invoiceDetail.ProductId, invoiceDetail.ProductType)
			if err != nil {
				config.LogError(logger, "PosInvoicePaymentWorkflow.go", "CreatePosInvoicePayment", "GetProductDetail", invoiceDetail, err)
				return 0, nil, 0, nil, err
			}

			if productDetail.InventoryAccountId > 0 {
				productPurchaseAmount, ok := productPurchaseAccounts[productDetail.PurchaseAccountId]
				if !ok {
					productPurchaseAmount = decimal.NewFromInt(0)
				}
				if invoice.IsTaxInclusive != nil && *invoice.IsTaxInclusive {
					productPurchaseAmount = productPurchaseAmount.Add(invoiceDetail.DetailTotalAmount.Add(invoiceDetail.DetailDiscountAmount).Sub(invoiceDetail.DetailTaxAmount))
				} else {
					productPurchaseAmount = productPurchaseAmount.Add(invoiceDetail.DetailTotalAmount.Add(invoiceDetail.DetailDiscountAmount))
				}
				productPurchaseAccounts[productDetail.PurchaseAccountId] = productPurchaseAmount

				productInventoryAmount, ok := productInventoryAccounts[productDetail.InventoryAccountId]
				if !ok {
					productInventoryAmount = decimal.NewFromInt(0)
				}
				if invoice.IsTaxInclusive != nil && *invoice.IsTaxInclusive {
					productInventoryAmount = productInventoryAmount.Add(invoiceDetail.DetailTotalAmount.Add(invoiceDetail.DetailDiscountAmount).Sub(invoiceDetail.DetailTaxAmount))
				} else {
					productInventoryAmount = productInventoryAmount.Add(invoiceDetail.DetailTotalAmount.Add(invoiceDetail.DetailDiscountAmount))
				}
				productInventoryAccounts[productDetail.InventoryAccountId] = productInventoryAmount

				stockHistory := models.StockHistory{
					BusinessId:        invoice.BusinessId,
					WarehouseId:       invoice.WarehouseId,
					ProductId:         invoiceDetail.ProductId,
					ProductType:       invoiceDetail.ProductType,
					BatchNumber:       invoiceDetail.BatchNumber,
					StockDate:         stockDate,
					Qty:               invoiceDetail.DetailQty.Neg(),
					Description:       "Invoice #" + invoice.InvoiceNumber,
					ReferenceType:     models.StockReferenceTypeInvoice,
					ReferenceID:       invoice.ID,
					ReferenceDetailID: invoiceDetail.ID,
					IsOutgoing:        utils.NewTrue(),
				}
				err = tx.Exec("UPDATE sales_invoice_details SET cogs = 0 WHERE id = ?",
					stockHistory.ReferenceDetailID).Error
				if err != nil {
					tx.Rollback()
					config.LogError(logger, "PosInvoicePaymentWorkflow.go", "CreatePosInvoicePayment", "ResetDetailCogs", stockHistory.ReferenceDetailID, err)
					return 0, nil, 0, nil, err
				}

				err = tx.Create(&stockHistory).Error
				if err != nil {
					tx.Rollback()
					config.LogError(logger, "PosInvoicePaymentWorkflow.go", "CreatePosInvoicePayment", "CreateStockHistory", stockHistory, err)
					return 0, nil, 0, nil, err
				}
				stockHistories = append(stockHistories, &stockHistory)
			}
		}
	}

	for accId, accAmount := range detailAccounts {
		accountIds = append(accountIds, accId)
		if baseCurrencyId != foreignCurrencyId {
			accTransactions = append(accTransactions, models.AccountTransaction{
				BusinessId:          businessId,
				AccountId:           accId,
				BranchId:            branchId,
				TransactionDateTime: transactionTime,
				BaseCurrencyId:      baseCurrencyId,
				BaseCredit:          accAmount.Mul(exchangeRate),
				BaseDebit:           decimal.NewFromInt(0),
				ForeignCurrencyId:   foreignCurrencyId,
				ForeignCredit:       accAmount,
				ForeignDebit:        decimal.NewFromInt(0),
				ExchangeRate:        exchangeRate,
			})
		} else {
			accTransactions = append(accTransactions, models.AccountTransaction{
				BusinessId:          businessId,
				AccountId:           accId,
				BranchId:            branchId,
				TransactionDateTime: transactionTime,
				BaseCurrencyId:      baseCurrencyId,
				BaseCredit:          accAmount,
				BaseDebit:           decimal.NewFromInt(0),
			})
		}
	}

	for purchaseAccId := range productPurchaseAccounts {
		if !slices.Contains(accountIds, purchaseAccId) {
			accountIds = append(accountIds, purchaseAccId)
		}
		if baseCurrencyId != foreignCurrencyId {
			accTransactions = append(accTransactions, models.AccountTransaction{
				BusinessId:          businessId,
				AccountId:           purchaseAccId,
				BranchId:            branchId,
				TransactionDateTime: transactionTime,
				BaseCurrencyId:      baseCurrencyId,
				BaseCredit:          decimal.NewFromInt(0),
				// BaseDebit:            purchaseAmount.Mul(exchangeRate),
				BaseDebit:            decimal.NewFromInt(0),
				IsInventoryValuation: utils.NewTrue(),
			})
		} else {
			accTransactions = append(accTransactions, models.AccountTransaction{
				BusinessId:          businessId,
				AccountId:           purchaseAccId,
				BranchId:            branchId,
				TransactionDateTime: transactionTime,
				BaseCurrencyId:      baseCurrencyId,
				BaseCredit:          decimal.NewFromInt(0),
				// BaseDebit:            purchaseAmount,
				BaseDebit:            decimal.NewFromInt(0),
				IsInventoryValuation: utils.NewTrue(),
			})
		}
	}

	for inventoryAccId := range productInventoryAccounts {
		if !slices.Contains(accountIds, inventoryAccId) {
			accountIds = append(accountIds, inventoryAccId)
		}
		if baseCurrencyId != foreignCurrencyId {
			accTransactions = append(accTransactions, models.AccountTransaction{
				BusinessId:          businessId,
				AccountId:           inventoryAccId,
				BranchId:            branchId,
				TransactionDateTime: transactionTime,
				BaseCurrencyId:      baseCurrencyId,
				BaseDebit:           decimal.NewFromInt(0),
				// BaseCredit:           inventoryAmount.Mul(exchangeRate),
				BaseCredit:           decimal.NewFromInt(0),
				IsInventoryValuation: utils.NewTrue(),
			})
		} else {
			accTransactions = append(accTransactions, models.AccountTransaction{
				BusinessId:          businessId,
				AccountId:           inventoryAccId,
				BranchId:            branchId,
				TransactionDateTime: transactionTime,
				BaseCurrencyId:      baseCurrencyId,
				BaseDebit:           decimal.NewFromInt(0),
				// BaseCredit:           inventoryAmount,
				BaseCredit:           decimal.NewFromInt(0),
				IsInventoryValuation: utils.NewTrue(),
			})
		}
	}

	// for Customer payment\
	// Insert into banking transaction if deposit account is cash or bank
	depositAccountInfo, err := GetAccount(tx, invoice.DepositAccountId)
	if err != nil {
		config.LogError(logger, "PosInvoicePaymentWorkflow.go", "CreatePosInvoicePayment", "GetDepositAccount", invoice.DepositAccountId, err)
		return 0, nil, 0, nil, err
	}
	bankingTransactionId := 0
	if depositAccountInfo.DetailType == models.AccountDetailTypeCash ||
		depositAccountInfo.DetailType == models.AccountDetailTypeBank {

		bankingTransaction := models.BankingTransaction{
			BusinessId:        businessId,
			BranchId:          invoice.BranchId,
			FromAccountId:     systemAccounts[models.AccountCodeAccountsReceivable],
			FromAccountAmount: invoiceAmount,
			ToAccountId:       invoice.DepositAccountId,
			ToAccountAmount:   invoiceAmount.Sub(invoice.BankCharges),
			CustomerId:        invoice.CustomerId,
			TransactionDate:   invoice.InvoiceDate,
			TransactionId:     invoice.CustomerPaymentId,
			TransactionNumber: invoice.PaymentNumber,
			TransactionType:   models.BankingTransactionTypeCustomerPayment,
			ExchangeRate:      invoice.ExchangeRate,
			CurrencyId:        invoice.CurrencyId,
			Amount:            invoiceAmount,
			BankCharges:       invoice.BankCharges,
			ReferenceNumber:   invoice.ReferenceNumber,
			Description:       invoice.Notes,
		}
		err = tx.Create(&bankingTransaction).Error
		if err != nil {
			config.LogError(logger, "PosInvoicePaymentWorkflow.go", "CreatePosInvoicePayment", "CreateBankingTransaction", bankingTransaction, err)
			return 0, nil, 0, nil, err
		}
		bankingTransactionId = bankingTransaction.ID
	}

	depositAccountTransact := models.AccountTransaction{
		BusinessId:           businessId,
		AccountId:            invoice.DepositAccountId,
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
	accountIds = append(accountIds, invoice.DepositAccountId)

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
		BranchId:            branchId,
		TransactionDateTime: transactionTime,
		TransactionNumber:   invoice.InvoiceNumber,
		TransactionDetails:  invoice.Notes,
		ReferenceNumber:     invoice.ReferenceNumber,
		CustomerId:          invoice.CustomerId,
		ReferenceId:         invoice.ID,
		ReferenceType:       models.AccountReferenceTypePosInvoicePayment,
		AccountTransactions: accTransactions,
	}

	err = tx.Create(&accJournal).Error
	if err != nil {
		config.LogError(logger, "PosInvoicePaymentWorkflow.go", "CreatePosInvoicePayment", "CreateAccountJournal", accJournal, err)
		tx.Rollback()
		return 0, nil, 0, nil, err
	}

	return accJournal.ID, accountIds, foreignCurrencyId, stockHistories, nil
}
