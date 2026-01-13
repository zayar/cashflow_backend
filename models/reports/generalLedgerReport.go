package reports

import (
	"context"
	"errors"

	"bitbucket.org/mmdatafocus/books_backend/config"
	"bitbucket.org/mmdatafocus/books_backend/models"
	"bitbucket.org/mmdatafocus/books_backend/utils"
)

func GetGeneralLedgerReport(ctx context.Context, fromDate models.MyDateString, toDate models.MyDateString, reportType string, branchID *int) ([]*models.AccountSummary, error) {
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

	query := `
        SELECT 
            acc.id as account_id,
            acc.main_type as account_main_type,
            acc.name as account_name,
            acc.code as account_code,
            SUM(acb.debit) AS debit,
            SUM(acb.credit) AS credit,
            (CASE WHEN acc.main_type IN ('Asset', 'Expense') 
            THEN SUM(acb.debit) - SUM(acb.credit)
            ELSE SUM(acb.credit) - SUM(acb.debit) END) AS balance,
            -- ABS((acb.running_balance) - (acb.balance)) AS opening_balance,
            (
				SELECT 
					CASE WHEN acc.main_type IN ('Asset', 'Expense') THEN running_balance ELSE -running_balance END
				FROM 
					account_currency_daily_balances AS acb_first
				WHERE 
					acb_first.account_id = acc.id
					AND acb_first.branch_id = ?
					AND acb_first.currency_id = ?
					AND acb_first.transaction_date BETWEEN ? AND ?
				ORDER BY 
					transaction_date ASC
				LIMIT 1
			) - 
			(
				SELECT 
					CASE WHEN acc.main_type IN ('Asset', 'Expense') THEN balance ELSE -balance END 
				FROM 
					account_currency_daily_balances AS acb_first
				WHERE 
					acb_first.account_id = acc.id
					AND acb_first.branch_id = ?
					AND acb_first.currency_id = ?
					AND acb_first.transaction_date BETWEEN ? AND ?
				ORDER BY 
					transaction_date ASC 
				LIMIT 1
			) AS opening_balance,
            (
                SELECT 
					CASE WHEN acc.main_type IN ('Asset', 'Expense') THEN running_balance ELSE -running_balance END
                FROM 
					account_currency_daily_balances AS acb_inner
                WHERE 
					acb_inner.account_id = acc.id
                	AND acb_inner.branch_id = ?
					AND acb_inner.currency_id = ?
                	AND acb_inner.transaction_date BETWEEN ? AND ?
                ORDER BY 
					transaction_date DESC 
                LIMIT 1
            ) AS closing_balance
        FROM 
            accounts AS acc
        LEFT JOIN
            account_currency_daily_balances AS acb ON acb.account_id = acc.id
            AND acb.branch_id = ?
			AND acb.currency_id = ?
            AND acb.transaction_date BETWEEN ? AND ?
        WHERE 
            acc.business_id = ?
        GROUP BY
			acc.id,
			acc.main_type,
			acc.name,
			acc.code
		ORDER BY 
			acc.name ASC
	`

	rows, err := db.Raw(query,
		*branchID, business.BaseCurrencyId, fromDate, toDate,
		*branchID, business.BaseCurrencyId, fromDate, toDate,
		*branchID, business.BaseCurrencyId, fromDate, toDate,
		*branchID, business.BaseCurrencyId, fromDate, toDate,
		businessId,
	).Rows()

	if err != nil {
		return nil, err //errors.Wrap(err, "failed to execute SQL query")
	}
	defer rows.Close()

	var accountSummaries []*models.AccountSummary

	// Iterate over query results
	for rows.Next() {
		var summary models.AccountSummary
		if err := db.ScanRows(rows, &summary); err != nil {
			return nil, err //errors.Wrap(err, "failed to scan row")
		}
		accountSummaries = append(accountSummaries, &summary)
	}

	if err := rows.Err(); err != nil {
		return nil, err //errors.Wrap(err, "error while iterating over rows")
	}

	return accountSummaries, nil
}

// func GetGeneralLedgerReport(ctx context.Context, fromDate time.Time, toDate time.Time, reportType string, branchID *int) ([]*models.AccountSummary, error) {

// 	businessId, ok := utils.GetBusinessIdFromContext(ctx)
// 	if !ok || businessId == "" {
// 		return nil, errors.New("business id is required")
// 	}
// 	db := config.GetDB()

// 	if branchID == nil  {
// 		 branchID = new(int)
//    		 *branchID = 0
// 	}

// 	dbCtx := db.WithContext(ctx).
// 				Preload("AccountCurrencyDailyBalances", "branch_id = ? AND transaction_date BETWEEN ? AND ?", branchID, fromDate, toDate).
// 				Where("business_id = ?", businessId).
// 				Order("name")

// 	// if reportType != nil && *reportType != "" {
// 	// 	dbCtx.Where("reportType LIKE ?", "%"+*reportType+"%")
// 	// }

// 	// db query
// 	var results []*models.Account
// 	err := dbCtx.Find(&results).Error
// 	if err != nil {
// 		return nil, err
// 	}

// 	var accountSummaries []*models.AccountSummary

// 	for i := range results {
// 		var debit, credit, closingBalance, openingBalance decimal.Decimal
// 		if len(results[i].AccountCurrencyDailyBalances ) > 0 {
// 			lastTransaction := results[i].AccountCurrencyDailyBalances [len(results[i].AccountCurrencyDailyBalances )-1]
// 			firstTransaction := results[i].AccountCurrencyDailyBalances [0]
// 			closingBalance = lastTransaction.RunningBalance
// 			openingBalance = firstTransaction.RunningBalance.Sub(firstTransaction.Balance)
// 		}else{
// 			closingBalance = decimal.NewFromFloat(0)
// 			openingBalance = decimal.NewFromFloat(0)
// 		}
// 		// Iterate over account transactions and sum debit and credit

// 		for _, transaction := range results[i].AccountCurrencyDailyBalances {
// 			debit = debit.Add(transaction.Debit)
// 			credit = credit.Add(transaction.Credit)
// 		}
// 		summary := models.AccountSummary{
// 			AccountId: 			results[i].ID,
// 			AccountName: 		results[i].Name,
// 			AccountMainType: 	string(results[i].MainType),
// 			Code: 		   		results[i].Code,
// 			Debit:       		debit,
// 			Credit:      		credit,
// 			Balance:     		debit.Sub(credit),
// 			ClosingBalance:     closingBalance.Abs(),
// 			OpeningBalance:     openingBalance.Abs(),
// 		}

// 		accountSummaries = append(accountSummaries, &summary)
// 	}

// 	return accountSummaries, nil
// }
