package reports

import (
	"context"
	"errors"
	"time"

	"bitbucket.org/mmdatafocus/books_backend/config"
	"bitbucket.org/mmdatafocus/books_backend/models"
	"bitbucket.org/mmdatafocus/books_backend/utils"
)

func GetTrialBalanceReport(ctx context.Context, toDate models.MyDateString, reportType string, branchID *int) ([]*models.TrialBalance, error) {

	businessId, ok := utils.GetBusinessIdFromContext(ctx)
	if !ok || businessId == "" {
		return nil, errors.New("business id is required")
	}
	business, err := models.GetBusiness(ctx)
	if err != nil {
		return nil, errors.New("business id is required")
	}
	if err := toDate.EndOfDayUTCTime(business.Timezone); err != nil {
		return nil, err
	}

	db := config.GetDB()

	fromDateTime, err := utils.GetFromDateFromFiscalYear(time.Time(toDate), string(business.FiscalYear))
	if err != nil {
		return nil, err
	}
	fromDate := models.MyDateString(fromDateTime)
	if err := fromDate.StartOfDayUTCTime(business.Timezone); err != nil {
		return nil, err
	}

	dbCtx := db.WithContext(ctx).Where("business_id = ?", businessId)

	if branchID == nil {
		branchID = new(int)
		*branchID = 0
	}

	var balances []*models.TrialBalance

	rows, err := dbCtx.Raw(`
					WITH LastRows AS (
						SELECT 
							-- acb.id as id,
							ac.main_type AS main_type,
							ac.name AS account_name,
							ac.code AS account_code,
							acb.account_id AS account_id,
							acb.running_balance,
							CASE
								WHEN acb.running_balance >= 0 THEN acb.running_balance
								ELSE 0
							END AS debit,
							CASE
								WHEN acb.running_balance < 0 THEN ABS(acb.running_balance)
								ELSE 0
							END AS credit,
							ROW_NUMBER() OVER (PARTITION BY acb.account_id ORDER BY acb.transaction_date DESC) AS row_num
						FROM 
							account_currency_daily_balances AS acb
						JOIN
							accounts AS ac ON acb.account_id = ac.id
						WHERE 
							acb.business_id = ? 
							AND acb.branch_id = ?
							AND acb.currency_id = ?
							AND acb.transaction_date <= ?
							AND acb.transaction_date >= ?
							AND acb.account_id IN (
								SELECT id FROM accounts WHERE main_type IN ('ASSET','LIABILITY','EQUITY')
							)
					),
					IncomeExpense AS (
						SELECT 
							acb.account_id AS account_id,
							CASE
								WHEN SUM(acb.debit) - SUM(acb.credit) >= 0 THEN SUM(acb.debit) - SUM(acb.credit)
								ELSE 0
							END AS debit,
							CASE
								WHEN SUM(acb.debit) - SUM(acb.credit) < 0 THEN ABS(SUM(acb.debit) - SUM(acb.credit))
								ELSE 0
							END AS credit
						FROM 
							account_currency_daily_balances AS acb
						JOIN
							accounts AS ac ON acb.account_id = ac.id
						WHERE 
							acb.business_id = ?
							AND acb.branch_id = ?
							AND acb.currency_id = ?
							AND acb.transaction_date <= ?
							AND acb.transaction_date >= ?
							AND acb.account_id IN (
								SELECT id FROM accounts WHERE main_type IN ('INCOME','EXPENSE')
							)
						GROUP BY 
							acb.account_id
					),
					RetainedEarnings AS (
						SELECT 
							0 AS account_id,
							CASE
								WHEN retain_amount >= 0 THEN retain_amount
								ELSE 0
							END AS debit,
							CASE
								WHEN retain_amount < 0 THEN ABS(retain_amount)
								ELSE 0
							END AS credit
						FROM (
							SELECT SUM(debit) - SUM(credit) as retain_amount
							FROM account_currency_daily_balances 
							WHERE 
								business_id= ?
								AND branch_id= ?
								AND currency_id= ?
								AND transaction_date <= ?
								AND account_id IN (
									SELECT id FROM accounts WHERE main_type IN ('INCOME', 'EXPENSE')
								)
						) AS retain_query
					)
					SELECT 
						-- id,
						main_type,
						account_name,
						account_code,
						account_id,
						debit,
						credit
					FROM 
						LastRows
					WHERE 
						row_num = 1
						
					UNION ALL

					SELECT 
						-- 0 AS id,
						'Equity' AS main_type,
						'Retained Earnings' AS account_name,
						'' AS account_code,
						account_id,
						debit,
						credit
					FROM 
						RetainedEarnings

					UNION ALL

					SELECT 
						-- id,
						ac.main_type AS main_type,
						ac.name AS account_name,
						ac.code AS account_code,
						ie.account_id,
						ie.debit,
						ie.credit
					FROM 
						IncomeExpense AS ie
					JOIN
						accounts AS ac ON ie.account_id = ac.id
					ORDER BY 
						CASE 
							WHEN main_type = 'Asset'  THEN 1
							WHEN main_type = 'Liability' THEN 2
							WHEN main_type = 'Equity' THEN 3
							WHEN main_type = 'Income' THEN 4
							ELSE 5
						END,
    					account_name ASC;

			`,
		businessId, branchID, business.BaseCurrencyId, toDate, fromDate,
		businessId, branchID, business.BaseCurrencyId, toDate, fromDate,
		businessId, branchID, business.BaseCurrencyId, fromDate).Rows()

	if err != nil {
		return nil, err
	}
	defer rows.Close()

	// Iterate over the rows and scan into TrialBalance structs
	for rows.Next() {
		var balance models.TrialBalance
		if err := rows.Scan(
			&balance.AccountMainType,
			&balance.AccountName,
			&balance.AccountCode,
			&balance.AccountId,
			&balance.Debit,
			&balance.Credit,
		); err != nil {
			return nil, err
		}
		balances = append(balances, &balance)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return balances, nil

}

// func GetTrialBalanceReport(ctx context.Context, toDate time.Time, reportType string, branchID *int) ([]*TrialBalance, error) {
//     businessId, ok := utils.GetBusinessIdFromContext(ctx)
//     if !ok || businessId == "" {
//         return nil, errors.New("business id is required")
//     }

//     db := config.GetDB()

//     dbCtx := db.WithContext(ctx).Where("business_id = ?", businessId)

//     if branchID != nil && *branchID > 0 {
//         dbCtx = dbCtx.Where("branch_id = ?", branchID)
//     }

//     dbCtx = dbCtx.Preload("Account").
//         Where("transaction_date <= ?", toDate).
//         Order("account_id ASC")

//     // db query
//     var results []*AccountCurrencyDailyBalance
//     err := dbCtx.Find(&results).Error
//     if err != nil {
//         return nil, err
//     }

//     // Group transactions by accountMainType
//     accountTransactionMap := make(map[string]*TrialBalance)
//     for _, transaction := range results {
//         // Check if accountMainType already exists in the map
//         mainType := string(transaction.Account.MainType)
//         if _, ok := accountTransactionMap[mainType]; !ok {
//             // If accountMainType doesn't exist, create a new entry
//             accountTransactionMap[mainType] = &TrialBalance{
//                 AccountMainType:    mainType,
//                 AccountSummaries:   []*AccountSummary{},
//             }
//         }

//         // Find existing summary for the account
//         var existingSummary *AccountSummary
//         for _, summary := range accountTransactionMap[mainType].AccountSummaries {
//             if summary.AccountId == transaction.AccountId {
//                 existingSummary = summary
//                 break
//             }
//         }

// 		 // Calculate debit and credit
//         var debit, credit decimal.Decimal
//         if mainType == string(AccountMainTypeAsset) ||
//            mainType == string(AccountMainTypeLiability) ||
//            mainType == string(AccountMainTypeEquity) {
//             if transaction.RunningBalance.Sign() == -1 {
//                 credit = transaction.RunningBalance.Abs()
//             } else {
//                 debit = transaction.RunningBalance.Abs()
//             }
//         }else{
// 			if existingSummary == nil {
// 				debit = transaction.Debit
// 				credit = transaction.Credit
// 			}else{
// 				debit = existingSummary.Debit.Add(transaction.Debit)
// 				credit = existingSummary.Credit.Add(transaction.Credit)
// 			}
// 			balance := debit.Sub(credit)
// 			 if balance.Sign() == -1 {
//                 credit = balance.Abs()
// 				debit = decimal.NewFromFloat(0)
//             } else {
//                 debit = balance
// 				credit = decimal.NewFromFloat(0)
//             }

// 		}
//         // If summary doesn't exist, create a new one
//         if existingSummary == nil {
//             summary := &AccountSummary{
//                 AccountId:    		transaction.AccountId,
//                 AccountName:  		transaction.Account.Name,
//                 AccountMainType:	mainType,
//                 Debit:        		debit,
//                 Credit:       		credit,
//             }
//             // Add the summary to the corresponding accountMainType
//             accountTransactionMap[mainType].AccountSummaries = append(accountTransactionMap[mainType].AccountSummaries, summary)
//         } else {
//             // Update existing summary
// 			existingSummary.Debit = debit
//             existingSummary.Credit = credit
//             // existingSummary.Debit = existingSummary.Debit.Add(debit)
//             // existingSummary.Credit = existingSummary.Credit.Add(credit)
//         }
//     }

//    // Convert map to slice of TrialBalance
//     var trialBalanceReport []*TrialBalance
//     for _, trialBalance := range accountTransactionMap {
//         trialBalanceReport = append(trialBalanceReport, trialBalance)
//     }

//     return trialBalanceReport, nil
// }
