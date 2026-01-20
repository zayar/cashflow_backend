package reports

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/mmdatafocus/books_backend/config"
	"github.com/mmdatafocus/books_backend/models"
	"github.com/mmdatafocus/books_backend/utils"
	"github.com/shopspring/decimal"
)

type DetailedGeneralLedgerReportConnection struct {
	Edges    []*DetailedGeneralLedgerReportEdge `json:"edges"`
	PageInfo *models.PageInfo                   `json:"pageInfo"`
}

type DetailedGeneralLedgerReportEdge struct {
	Cursor string                        `json:"cursor"`
	Node   *models.DetailedGeneralLedger `json:"node"`
}

func PaginateDetailedGeneralLedgerReport(ctx context.Context, limit *int, after *string, fromDate models.MyDateString, toDate models.MyDateString, reportType string, branchID *int) (*DetailedGeneralLedgerReportConnection, error) {

	businessId, ok := utils.GetBusinessIdFromContext(ctx)
	if !ok || businessId == "" {
		return nil, errors.New("business id is required")
	}
	business, err := models.GetBusiness(ctx)
	if err != nil {
		return nil, errors.New("business id is required")
	}
	if err := fromDate.StartOfDayUTCTime(business.Timezone); err != nil {
		return nil, err
	}
	if err := toDate.EndOfDayUTCTime(business.Timezone); err != nil {
		return nil, err
	}

	db := config.GetDB()

	// db = db.Session(&gorm.Session{
	// 	Logger: config.WriteGormLog(), // Apply the custom logger
	// })
	// Initialize branchID to 0 if it's nil
	if branchID == nil {
		branchID = new(int)
		*branchID = 0
	}

	var results []*models.DetailLedgerTransaction
	// query := db.Raw(`
	//     SELECT
	//         account_transactions.id,
	//         accounts.name AS account_name,
	//         account_transactions.base_currency_id,
	//         currencies.name AS currency_name,
	//         currencies.symbol AS currency_symbol,
	//         account_transactions.transaction_date_time,
	//         account_transactions.account_id,
	//         account_transactions.description,
	//         account_transactions.base_debit AS debit,
	//         account_transactions.base_credit AS credit,
	//         account_transactions.foreign_debit,
	//         account_transactions.foreign_credit,
	//         account_transactions.exchange_rate,
	//         account_journals.reference_id,
	//         account_journals.reference_type AS transaction_type,
	//         account_journals.transaction_number AS transaction_number,
	//         account_journals.transaction_details AS transaction_details,
	//         account_journals.reference_number AS reference_number,
	//         customers.name AS customer_name,
	//         suppliers.name AS supplier_name,
	// 		f_currency.name AS foreign_currency_name,
	//         f_currency.symbol AS foreign_currency_symbol
	//     FROM
	//         account_transactions
	//     JOIN
	//         accounts ON account_transactions.account_id = accounts.id
	//     JOIN
	//         currencies ON account_transactions.base_currency_id = currencies.id
	//     JOIN
	//         account_journals ON account_transactions.journal_id = account_journals.id
	// 	LEFT JOIN
	//         currencies AS f_currency ON account_transactions.foreign_currency_id = f_currency.id
	//     LEFT JOIN
	//         customers ON account_journals.customer_id = customers.id
	//     LEFT JOIN
	//         suppliers ON account_journals.supplier_id = suppliers.id
	//     WHERE
	//         account_transactions.business_id = ?
	//         AND account_transactions.transaction_date_time BETWEEN ? AND ?
	//     ORDER BY
	//         accounts.name, account_transactions.transaction_date_time ASC
	// `, businessId, fromDate, toDate)

	// if branchID != nil && *branchID > 0 {
	// 	fmt.Println("more than zero")
	// 	query.Where("branch_id = ?", branchID)
	// }

	sqlT := `
	 SELECT 
            account_transactions.id,
            accounts.name AS account_name,
            account_transactions.base_currency_id,
            currencies.name AS currency_name,
            currencies.symbol AS currency_symbol,
            account_transactions.transaction_date_time,
            account_transactions.account_id,
            account_transactions.description,
            account_transactions.base_debit AS debit,
            account_transactions.base_credit AS credit,
            account_transactions.foreign_debit,
            account_transactions.foreign_credit,
            account_transactions.exchange_rate,
            account_journals.reference_id,
            account_journals.reference_type AS transaction_type,
            account_journals.transaction_number AS transaction_number,
            account_journals.transaction_details AS transaction_details,
            account_journals.reference_number AS reference_number,
            customers.name AS customer_name,
            suppliers.name AS supplier_name,
			f_currency.name AS foreign_currency_name,
            f_currency.symbol AS foreign_currency_symbol
        FROM 
            account_transactions
        JOIN 
            accounts ON account_transactions.account_id = accounts.id
        JOIN
            currencies ON account_transactions.base_currency_id = currencies.id
        JOIN 
            account_journals ON account_transactions.journal_id = account_journals.id
		LEFT JOIN
            currencies AS f_currency ON account_transactions.foreign_currency_id = f_currency.id
        LEFT JOIN 
            customers ON account_journals.customer_id = customers.id
        LEFT JOIN 
            suppliers ON account_journals.supplier_id = suppliers.id
        WHERE 
            account_transactions.business_id = @businessId
            AND account_journals.is_reversal = 0
            AND account_journals.reversed_by_journal_id IS NULL
            AND account_transactions.transaction_date_time BETWEEN @fromDate AND @toDate
			 {{- if not .AllBranch }}
                AND account_transactions.branch_id = @branchId
            {{- end }}
        ORDER BY 
            accounts.name, account_transactions.transaction_date_time ASC
	`

	sql, err := utils.ExecTemplate(sqlT, map[string]interface{}{
		"AllBranch": *branchID == 0,
	})
	if err != nil {
		return nil, err
	}
	if err := db.WithContext(ctx).Raw(sql, map[string]interface{}{
		"businessId": businessId,
		"fromDate":   fromDate,
		"toDate":     toDate,
		"branchId":   branchID,
	}).Scan(&results).Error; err != nil {
		return nil, err
	}

	accountNameTransactionMap := make(map[string][]*models.DetailLedgerTransaction)
	for _, transaction := range results {
		accountName := transaction.AccountName
		accountNameTransactionMap[accountName] = append(accountNameTransactionMap[accountName], transaction)
	}

	// Collect unique account names
	var uniqueAccountNames []string
	for accountName := range accountNameTransactionMap {
		uniqueAccountNames = append(uniqueAccountNames, accountName)
	}

	// Sort the unique account names
	sort.Strings(uniqueAccountNames)

	// Construct final result
	// Construct final result
	var accountTransactionResults []*DetailedGeneralLedgerReportEdge
	for _, accountName := range uniqueAccountNames {
		transactions := accountNameTransactionMap[accountName]

		// Sort transactions by transactionDateTime
		sort.Slice(transactions, func(i, j int) bool {
			return transactions[i].TransactionDateTime.Before(transactions[j].TransactionDateTime)
		})
		account := &models.DetailedGeneralLedger{
			AccountId:          transactions[0].AccountId,
			AccountName:        accountName,
			CurrencyId:         transactions[0].BaseCurrencyId,
			CurrencyName:       transactions[0].CurrencyName,
			CurrencySymbol:     transactions[0].CurrencySymbol,
			OpeningBalanceDate: time.Time(fromDate),
			ClosingBalanceDate: time.Time(toDate),
			Transactions:       make([]*models.DetailLedgerTransaction, len(transactions)),
		}

		// Populate Transactions slice
		for i, t := range transactions {
			detailTransaction := &models.DetailLedgerTransaction{
				AccountId:             t.AccountId,
				TransactionDateTime:   t.TransactionDateTime,
				AccountName:           t.AccountName,
				Description:           t.Description,
				Debit:                 t.Debit,
				Credit:                t.Credit,
				ForeignDebit:          t.ForeignDebit,
				ForeignCredit:         t.ForeignCredit,
				ForeignCurrencyName:   t.ForeignCurrencyName,
				ForeignCurrencySymbol: t.ForeignCurrencySymbol,
				ExchangeRate:          t.ExchangeRate,
				// BaseClosingBalance:  t.BaseClosingBalance,
				TransactionType:    t.TransactionType,
				TransactionNumber:  t.TransactionNumber,
				TransactionDetails: t.TransactionDetails,
				ReferenceNumber:    t.ReferenceNumber,
				CustomerName:       t.CustomerName,
				SupplierName:       t.SupplierName,
			}
			account.Transactions[i] = detailTransaction
		}

		var dailyBalances []*models.AccountCurrencyDailyBalance
		query := db.Raw(`
            (SELECT * FROM account_currency_daily_balances
            WHERE business_id = ? AND account_id = ? AND branch_id = ? AND currency_id = ? AND transaction_date BETWEEN ? AND ?
            ORDER BY transaction_date ASC LIMIT 1)
            UNION ALL
            (SELECT * FROM account_currency_daily_balances
            WHERE business_id = ? AND account_id = ? AND branch_id = ? AND currency_id = ? AND transaction_date BETWEEN ? AND ?
            ORDER BY transaction_date DESC LIMIT 1)
        `, businessId, transactions[0].AccountId, branchID, transactions[0].BaseCurrencyId, fromDate, toDate,
			businessId, transactions[0].AccountId, branchID, transactions[0].BaseCurrencyId, fromDate, toDate)

		err := query.Scan(&dailyBalances).Error

		if err != nil {
			return nil, err
		}

		if len(dailyBalances) > 0 {
			account.OpeningBalance = dailyBalances[0].RunningBalance.Sub(dailyBalances[0].Balance)
			account.ClosingBalance = dailyBalances[len(dailyBalances)-1].RunningBalance
		} else {
			account.OpeningBalance = decimal.NewFromInt(0)
			account.ClosingBalance = decimal.NewFromInt(0)
		}
		cursor := fmt.Sprintf("%s|%s", transactions[0].TransactionDateTime.String(), accountName)
		// Create the edge for the current account
		edge := &DetailedGeneralLedgerReportEdge{
			Cursor: models.EncodeCursor(cursor), // Use the first transaction's datetime as cursor
			Node:   account,
		}
		accountTransactionResults = append(accountTransactionResults, edge)
	}
	// Paginate the results
	paginatedResults := paginateResults(accountTransactionResults, limit, after)

	// Construct pagination information
	pageInfo := models.PageInfo{
		StartCursor: "",
		EndCursor:   "",
		HasNextPage: utils.NewFalse(),
	}
	if len(paginatedResults) > 0 {
		pageInfo.StartCursor = paginatedResults[0].Cursor
		pageInfo.EndCursor = paginatedResults[len(paginatedResults)-1].Cursor
		hasNextPage := len(results) > len(paginatedResults)
		pageInfo.HasNextPage = &hasNextPage
	}

	connection := DetailedGeneralLedgerReportConnection{
		Edges:    paginatedResults,
		PageInfo: &pageInfo,
	}

	return &connection, nil
}

// paginateResults is a helper function to paginate the results based on limit and after cursor
func paginateResults(results []*DetailedGeneralLedgerReportEdge, limit *int, after *string) []*DetailedGeneralLedgerReportEdge {
	if limit == nil || *limit <= 0 {
		return results
	}

	if after == nil || *after == "" {
		return results[:min(*limit, len(results))]
	}

	// Find the index of the after cursor
	var startIndex int
	for i, edge := range results {
		if edge.Cursor == *after {
			startIndex = i + 1
			break
		}
	}

	// If after cursor is not found or is the last element, return empty slice
	if startIndex == 0 || startIndex >= len(results) {
		return []*DetailedGeneralLedgerReportEdge{}
	}

	// Paginate from startIndex to limit
	endIndex := min(startIndex+*limit, len(results))
	return results[startIndex:endIndex]
}

// func GetDetailedGeneralLedgerReport(ctx context.Context, fromDate time.Time, toDate time.Time, reportType string, branchID *int) ([]*models.DetailedGeneralLedger, error) {
// 	businessId, ok := utils.GetBusinessIdFromContext(ctx)
// 	if !ok || businessId == "" {
// 		return nil, errors.New("business id is required")
// 	}

// 	db := config.GetDB()

// 	var results []*models.DetailLedgerTransaction
// 	query := db.Raw(`
//         SELECT
//             account_transactions.id,
//             accounts.name AS account_name,
//             account_transactions.transaction_date_time,
//             account_transactions.account_id,
//             account_transactions.description,
//             account_transactions.base_debit AS debit,
//             account_transactions.base_credit AS credit,
//             account_transactions.base_closing_balance,
//             account_journals.reference_id,
//             account_journals.reference_type AS transaction_type,
//             account_journals.transaction_number AS transaction_number,
//             account_journals.transaction_details AS transaction_details,
//             account_journals.reference_number AS reference_number,
//             customers.name AS customer_name,
//             suppliers.name AS supplier_name
//         FROM
//             account_transactions
//         JOIN
//             accounts ON account_transactions.account_id = accounts.id
//         JOIN
//             account_journals ON account_transactions.journal_id = account_journals.id
//         LEFT JOIN
//             customers ON account_journals.customer_id = customers.id
//         LEFT JOIN
//             suppliers ON account_journals.supplier_id = suppliers.id
//         WHERE
//             account_transactions.business_id = ?
//             AND account_transactions.transaction_date_time BETWEEN ? AND ?
//         ORDER BY
//             accounts.name, account_transactions.transaction_date_time
//     `, businessId, fromDate, toDate)

// 	if branchID != nil && *branchID > 0 {
// 		query.Where("branch_id = ?", branchID)
// 	}

// 	err := query.Scan(&results).Error

// 	if err != nil {
// 		return nil, err
// 	}

// 	// Construct a map to hold transactions indexed by account name
// 	accountNameTransactionMap := make(map[string][]*models.DetailLedgerTransaction)
// 	for _, transaction := range results {
// 		accountName := transaction.AccountName
// 		accountNameTransactionMap[accountName] = append(accountNameTransactionMap[accountName], transaction)
// 	}

// 	// Collect unique account names
// 	var uniqueAccountNames []string
// 	for accountName := range accountNameTransactionMap {
// 		uniqueAccountNames = append(uniqueAccountNames, accountName)
// 	}

// 	// Sort the unique account names
// 	sort.Strings(uniqueAccountNames)

// 	// Construct final result
// 	var accountTransactionResults []*models.DetailedGeneralLedger
// 	for _, accountName := range uniqueAccountNames {
// 		transactions := accountNameTransactionMap[accountName]

// 		// Sort transactions by transactionDateTime
// 		sort.Slice(transactions, func(i, j int) bool {
// 			return transactions[i].TransactionDateTime.Before(transactions[j].TransactionDateTime)
// 		})

// 		account := &models.DetailedGeneralLedger{
// 			AccountId:          transactions[0].AccountId, // Assuming all transactions for the same account have the same ID
// 			AccountName:        accountName,
// 			OpeningBalanceDate: fromDate,
// 			ClosingBalanceDate: toDate,
// 			Transactions:       make([]*models.DetailLedgerTransaction, len(transactions)),
// 		}

// 		// Populate Transactions slice
// 		for i, t := range transactions {
// 			detailTransaction := &models.DetailLedgerTransaction{
// 				AccountId:           t.AccountId,
// 				TransactionDateTime: t.TransactionDateTime,
// 				AccountName:         t.AccountName,
// 				Description:         t.Description,
// 				Debit:               t.Debit,
// 				Credit:              t.Credit,
// 				BaseClosingBalance:  t.BaseClosingBalance,
// 				TransactionType:     t.TransactionType,
// 				TransactionNumber:   t.TransactionNumber,
// 				TransactionDetails:  t.TransactionDetails,
// 				ReferenceNumber:     t.ReferenceNumber,
// 				CustomerName:        t.CustomerName,
// 				SupplierName:        t.SupplierName,
// 			}
// 			account.Transactions[i] = detailTransaction
// 		}

// 		// Calculate opening balance based on the first transaction type
// 		firstTransaction := transactions[0]
// 		if firstTransaction.Description == "debit" {
// 			account.OpeningBalance = firstTransaction.BaseClosingBalance.Sub(firstTransaction.Debit).Abs()
// 		} else if firstTransaction.Description == "credit" {
// 			account.OpeningBalance = firstTransaction.BaseClosingBalance.Add(firstTransaction.Credit).Abs()
// 		}

// 		// Calculate closing balance based on the last transaction type
// 		lastTransaction := transactions[len(transactions)-1]
// 		account.ClosingBalance = lastTransaction.BaseClosingBalance.Abs()

// 		accountTransactionResults = append(accountTransactionResults, account)
// 	}

// 	return accountTransactionResults, nil
// }

func GetAllDetailedGeneralLedgerReport(ctx context.Context, fromDate models.MyDateString, toDate models.MyDateString, reportType string, branchID *int) ([]*models.DetailedGeneralLedger, error) {

	businessId, ok := utils.GetBusinessIdFromContext(ctx)
	if !ok || businessId == "" {
		return nil, errors.New("business id is required")
	}
	business, err := models.GetBusiness(ctx)
	if err != nil {
		return nil, errors.New("business id is required")
	}

	if err := fromDate.StartOfDayUTCTime(business.Timezone); err != nil {
		return nil, err
	}
	if err := toDate.EndOfDayUTCTime(business.Timezone); err != nil {
		return nil, err
	}

	db := config.GetDB()

	// Initialize branchID to 0 if it's nil
	if branchID == nil {
		branchID = new(int)
		*branchID = 0
	}

	var detailedLedgerTransactions []*models.DetailLedgerTransaction
	query := db.Raw(`
        SELECT 
            account_transactions.id,
            accounts.name AS account_name,
            account_transactions.base_currency_id,
            currencies.name AS currency_name,
            currencies.symbol AS currency_symbol,
            account_transactions.transaction_date_time,
            account_transactions.account_id,
            account_transactions.description,
            account_transactions.base_debit AS debit,
            account_transactions.base_credit AS credit,
            account_transactions.foreign_debit,
            account_transactions.foreign_credit,
            account_transactions.exchange_rate,
            account_journals.reference_id,
            account_journals.reference_type AS transaction_type,
            account_journals.transaction_number AS transaction_number,
            account_journals.transaction_details AS transaction_details,
            account_journals.reference_number AS reference_number,
            customers.name AS customer_name,
			suppliers.name AS supplier_name,
			f_currency.name AS foreign_currency_name,
            f_currency.symbol AS foreign_currency_symbol
        FROM 
            account_transactions
        JOIN 
            accounts ON account_transactions.account_id = accounts.id
        JOIN
            currencies ON account_transactions.base_currency_id = currencies.id
		JOIN
            currencies AS f_currency ON account_transactions.foreign_currency_id = f_currency.id
        JOIN 
            account_journals ON account_transactions.journal_id = account_journals.id
        LEFT JOIN 
            customers ON account_journals.customer_id = customers.id
        LEFT JOIN 
            suppliers ON account_journals.supplier_id = suppliers.id
        WHERE 
            account_transactions.business_id = ? 
            AND account_journals.is_reversal = 0
            AND account_journals.reversed_by_journal_id IS NULL
            AND account_transactions.transaction_date_time BETWEEN ? AND ?
        ORDER BY 
            accounts.name, account_transactions.transaction_date_time ASC
    `, businessId, fromDate, toDate)

	if branchID != nil && *branchID > 0 {
		query.Where("branch_id = ?", branchID)
	}

	err = query.Scan(&detailedLedgerTransactions).Error

	if err != nil {
		return nil, err
	}

	accountNameTransactionMap := make(map[string][]*models.DetailLedgerTransaction)
	for _, transaction := range detailedLedgerTransactions {
		accountName := transaction.AccountName
		accountNameTransactionMap[accountName] = append(accountNameTransactionMap[accountName], transaction)
	}

	// Collect unique account names
	var uniqueAccountNames []string
	for accountName := range accountNameTransactionMap {
		uniqueAccountNames = append(uniqueAccountNames, accountName)
	}

	// Sort the unique account names
	sort.Strings(uniqueAccountNames)

	// Construct final result
	var results []*models.DetailedGeneralLedger
	for _, accountName := range uniqueAccountNames {
		transactions := accountNameTransactionMap[accountName]

		// Sort transactions by transactionDateTime
		sort.Slice(transactions, func(i, j int) bool {
			return transactions[i].TransactionDateTime.Before(transactions[j].TransactionDateTime)
		})
		account := &models.DetailedGeneralLedger{
			AccountId:          transactions[0].AccountId,
			AccountName:        accountName,
			CurrencyId:         transactions[0].BaseCurrencyId,
			CurrencyName:       transactions[0].CurrencyName,
			CurrencySymbol:     transactions[0].CurrencySymbol,
			OpeningBalanceDate: time.Time(fromDate),
			ClosingBalanceDate: time.Time(toDate),
			Transactions:       make([]*models.DetailLedgerTransaction, len(transactions)),
		}

		// Populate Transactions slice
		for i, t := range transactions {
			detailTransaction := &models.DetailLedgerTransaction{
				AccountId:           t.AccountId,
				TransactionDateTime: t.TransactionDateTime,
				AccountName:         t.AccountName,
				Description:         t.Description,
				Debit:               t.Debit,
				Credit:              t.Credit,
				ForeignDebit:        t.ForeignDebit,
				ForeignCredit:       t.ForeignCredit,
				ExchangeRate:        t.ExchangeRate,
				// BaseClosingBalance:  t.BaseClosingBalance,
				ForeignCurrencyName:   t.ForeignCurrencyName,
				ForeignCurrencySymbol: t.ForeignCurrencySymbol,
				TransactionType:       t.TransactionType,
				TransactionNumber:     t.TransactionNumber,
				TransactionDetails:    t.TransactionDetails,
				ReferenceNumber:       t.ReferenceNumber,
				CustomerName:          t.CustomerName,
				SupplierName:          t.SupplierName,
			}
			account.Transactions[i] = detailTransaction
		}

		var dailyBalances []*models.AccountCurrencyDailyBalance
		query := db.Raw(`
            (SELECT * FROM account_currency_daily_balances
            WHERE business_id = ? AND account_id = ? AND branch_id = ? AND currency_id = ? AND transaction_date BETWEEN ? AND ?
            ORDER BY transaction_date ASC LIMIT 1)
            UNION ALL
            (SELECT * FROM account_currency_daily_balances
            WHERE business_id = ? AND account_id = ? AND branch_id = ? AND currency_id = ? AND transaction_date BETWEEN ? AND ?
            ORDER BY transaction_date DESC LIMIT 1)
        `, businessId, transactions[0].AccountId, branchID, transactions[0].BaseCurrencyId, fromDate, toDate,
			businessId, transactions[0].AccountId, branchID, transactions[0].BaseCurrencyId, fromDate, toDate)

		err := query.Scan(&dailyBalances).Error

		if err != nil {
			return nil, err
		}

		if len(dailyBalances) > 0 {
			account.OpeningBalance = dailyBalances[0].RunningBalance.Sub(dailyBalances[0].Balance)
			account.ClosingBalance = dailyBalances[len(dailyBalances)-1].RunningBalance
		} else {
			account.OpeningBalance = decimal.NewFromInt(0)
			account.ClosingBalance = decimal.NewFromInt(0)
		}

		results = append(results, account)
	}

	return results, nil
}
