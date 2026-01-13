package reports

import (
	"context"
	"errors"
	"sort"

	"bitbucket.org/mmdatafocus/books_backend/config"
	"bitbucket.org/mmdatafocus/books_backend/models"
	"bitbucket.org/mmdatafocus/books_backend/utils"
)

func GetAccountTypeSummaryReport(ctx context.Context, fromDate models.MyDateString, toDate models.MyDateString, reportType string, branchID *int) ([]*models.AccountSummaryGroup, error) {
	businessID, ok := utils.GetBusinessIdFromContext(ctx)
	if !ok || businessID == "" {
		return nil, errors.New("business ID is required")
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
            acc.main_type AS account_main_type,
            acc.id AS account_id,
            acc.name AS account_name,
            acc.code,
            SUM(acb.debit) AS debit,
            SUM(acb.credit) AS credit,
            ABS(SUM(acb.debit) - SUM(acb.credit)) AS balance,
            ABS((
                SELECT 
                    running_balance 
                FROM 
                    account_currency_daily_balances AS acb_first
                WHERE 
                    acb_first.account_id = acc.id
                    AND acb_first.branch_id = ?
                    AND acb_first.currency_id = ?
                    AND acb_first.transaction_date BETWEEN ? AND ?
                LIMIT 1
            ) - 
            (
                SELECT 
                    balance 
                FROM 
                    account_currency_daily_balances AS acb_first
                WHERE 
                    acb_first.account_id = acc.id
                    AND acb_first.branch_id = ?
                    AND acb_first.currency_id = ?
                    AND acb_first.transaction_date BETWEEN ? AND ?
                LIMIT 1
            )) AS opening_balance,
            (
                SELECT 
                    ABS(running_balance)
                FROM 
                    account_currency_daily_balances AS acb_inner
                WHERE 
                    acb_inner.account_id = acc.id
                    AND acb_inner.branch_id = ?
                    AND acb_inner.currency_id = ?
                    AND acb_inner.transaction_date BETWEEN ? AND ?
                ORDER BY 
                    id DESC 
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
            acc.main_type,
            acc.id,
            acc.name,
            acc.code
        ORDER BY 
            acc.main_type, acc.name ASC;

    `

	var accountGroupSummaries []*models.AccountSummaryGroup

	rows, err := db.Raw(query,
		*branchID, business.BaseCurrencyId, fromDate, toDate,
		*branchID, business.BaseCurrencyId, fromDate, toDate,
		*branchID, business.BaseCurrencyId, fromDate, toDate,
		*branchID, business.BaseCurrencyId, fromDate, toDate,
		businessID,
	).Rows()

	// rows, err := db.Raw(query,
	//     *branchID, business.BaseCurrencyId, fromStr, toStr,
	//     *branchID, business.BaseCurrencyId, fromStr, toStr,
	//     *branchID, business.BaseCurrencyId, fromStr, toStr,
	//     *branchID, business.BaseCurrencyId, fromStr, toStr,
	//     businessID,
	// ).Rows()

	if err != nil {
		return nil, err
	}
	defer rows.Close()

	// Initialize slice to hold account summaries
	var summaries []*models.AccountSummary

	// Iterate over query results
	for rows.Next() {
		var summary models.AccountSummary
		if err := db.ScanRows(rows, &summary); err != nil {
			return nil, err
		}
		summaries = append(summaries, &summary)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	// Sort summaries by main type and name
	sort.Slice(summaries, func(i, j int) bool {
		if summaries[i].AccountMainType == summaries[j].AccountMainType {
			return summaries[i].AccountName < summaries[j].AccountName
		}
		return summaries[i].AccountMainType < summaries[j].AccountMainType
	})

	// Group summaries by main type
	for _, summary := range summaries {
		if len(accountGroupSummaries) == 0 || accountGroupSummaries[len(accountGroupSummaries)-1].AccountMainType != summary.AccountMainType {
			accountGroupSummaries = append(accountGroupSummaries, &models.AccountSummaryGroup{
				AccountMainType:  summary.AccountMainType,
				AccountSummaries: []models.AccountSummary{*summary},
			})
		} else {
			accountGroupSummaries[len(accountGroupSummaries)-1].AccountSummaries = append(accountGroupSummaries[len(accountGroupSummaries)-1].AccountSummaries, *summary)
		}
	}

	return accountGroupSummaries, nil
}

// func GetAccountTypeSummaryReport(ctx context.Context, fromDate time.Time, toDate time.Time, reportType string, branchID *int) ([]*models.AccountSummaryGroup, error) {
//     businessID, ok := utils.GetBusinessIdFromContext(ctx)
//     if !ok || businessID == "" {
//         return nil, errors.New("business ID is required")
//     }

//     db := config.GetDB()

//     // Initialize branchID to 0 if it's nil
//     if branchID == nil {
//         branchID = new(int)
//         *branchID = 0
//     }

//     query := `
//         SELECT
//             acc.main_type AS account_main_type,
//             acc.id AS account_id,
//             acc.name AS account_name,
//             acc.code,
//             SUM(acb.debit) AS debit,
//             SUM(acb.credit) AS credit,
//             ABS(SUM(acb.debit) - SUM(acb.credit)) AS balance,
//             ABS((
//                 SELECT
//                     running_balance
//                 FROM
//                     account_currency_daily_balances AS acb_first
//                 WHERE
//                     acb_first.account_id = acc.id
//                     AND acb_first.branch_id = ?
//                     AND acb_first.transaction_date BETWEEN ? AND ?
//                 LIMIT 1
//             ) -
//             (
//                 SELECT
//                     balance
//                 FROM
//                     account_currency_daily_balances AS acb_first
//                 WHERE
//                     acb_first.account_id = acc.id
//                     AND acb_first.branch_id = ?
//                     AND acb_first.transaction_date BETWEEN ? AND ?
//                 LIMIT 1
//             )) AS opening_balance,
//             (
//                 SELECT
//                     ABS(running_balance)
//                 FROM
//                     account_currency_daily_balances AS acb_inner
//                 WHERE
//                     acb_inner.account_id = acc.id
//                     AND acb_inner.branch_id = ?
//                     AND acb_inner.transaction_date BETWEEN ? AND ?
//                 ORDER BY
//                     id DESC
//                 LIMIT 1
//             ) AS closing_balance
//         FROM
//             accounts AS acc
//         LEFT JOIN
//             account_currency_daily_balances AS acb ON acb.account_id = acc.id
//             AND acb.branch_id = ?
//             AND acb.transaction_date BETWEEN ? AND ?
//         WHERE
//             acc.business_id = ?
//         GROUP BY
//             acc.main_type,
//             acc.id,
//             acc.name,
//             acc.code
//         ORDER BY
//             acc.main_type, acc.name ASC;

//     `

//     rows, err := db.Raw(query,
//         *branchID, fromDate, toDate,
//         *branchID, fromDate, toDate,
//         *branchID, fromDate, toDate,
//         *branchID, fromDate, toDate,
//         businessID,
//     ).Rows()

//     if err != nil {
//         return nil, err
//     }
//     defer rows.Close()

//     var accountGroupSummaries []*models.AccountSummaryGroup

//     // Initialize map to hold account summaries grouped by main type
//     accountGroups := make(map[string][]models.AccountSummary)

//     // Iterate over query results
//     for rows.Next() {
//         var summary models.AccountSummary
//         if err := db.ScanRows(rows, &summary); err != nil {
//             return nil, err
//         }
//         accountGroups[summary.AccountMainType] = append(accountGroups[summary.AccountMainType], summary)
//     }

//     // Convert map to array of AccountSummaryGroup
//     for mainType, summaries := range accountGroups {
//         accountGroupSummaries = append(accountGroupSummaries, &models.AccountSummaryGroup{
//             AccountMainType: mainType,
//             AccountSummaries: summaries,
//         })
//     }

//     if err := rows.Err(); err != nil {
//         return nil, err
//     }

//     return accountGroupSummaries, nil
// }
