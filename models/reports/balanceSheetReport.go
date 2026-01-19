package reports

import (
	"context"
	"database/sql"
	"errors"
	"sort"
	"time"

	"github.com/mmdatafocus/books_backend/config"
	"github.com/mmdatafocus/books_backend/models"
	"github.com/mmdatafocus/books_backend/utils"
	"github.com/shopspring/decimal"
)

// func GetBalanceSheetReport(ctx context.Context, toDate time.Time, reportType string, branchID *int) ([]*models.BalanceSheetResponse, error) {
// 	businessId, ok := utils.GetBusinessIdFromContext(ctx)
// 	if !ok || businessId == "" {
// 		return nil, errors.New("business id is required")
// 	}

// 	db := config.GetDB()

// 	business, err := models.GetBusinessById(ctx, businessId)
// 	if err != nil {
// 		return nil, err
// 	}

// 	fromDate, err := utils.GetFromDateFromFiscalYear(toDate, string(business.FiscalYear))
// 	if err != nil {
// 		return nil, err
// 	}

// 	// Initialize branchID to 0 if it's nil
// 	if branchID == nil {
// 		branchID = new(int)
// 		*branchID = 0
// 	}

// 	var balances []models.BalanceSheet

// 	query := `
// 				WITH LastRows AS (
// 					SELECT
// 						ac.main_type AS main_type,
// 						ac.detail_type AS detail_type,
// 						ac.name AS account_name,
// 						acb.account_id AS account_id,
// 						SUM(acb.base_debit - acb.base_credit) AS amount
// 					FROM
// 						account_transactions AS acb
// 					JOIN
// 						accounts AS ac ON acb.account_id = ac.id
// 					WHERE
// 						acb.business_id = ?
// 						AND acb.base_currency_id = ?
// 						AND acb.transaction_date_time >= ?
// 						AND acb.transaction_date_time <= ?
// 						AND acb.account_id IN (
// 							SELECT id FROM accounts WHERE main_type IN ('Asset', 'Liability', 'Equity')
// 						)
// 				`
// 				if *branchID != 0 {
// 					query += " AND acb.branch_id = ?"
// 				}

// 				query += `
// 					GROUP BY acb.account_id, ac.main_type, ac.detail_type, ac.name
// 				),

// 				RetainedEarnings AS (
// 					SELECT
// 						'Liabilities & Equities' AS main_type,
// 						'Equities' AS main_group_type,
// 						'Retained Earnings' AS account_name,
// 						'Equities' AS sub_type,
// 						0 AS account_id,
// 						retain_amount AS amount
// 					FROM (
// 						SELECT SUM(credit) - SUM(debit) AS retain_amount
// 						FROM account_currency_daily_balances
// 						WHERE
// 							business_id = ?
// 							AND branch_id = ?
// 							AND currency_id = ?
// 							AND transaction_date < ?
// 							AND account_id IN (
// 								SELECT id FROM accounts WHERE main_type IN ('INCOME', 'EXPENSE')
// 							)
// 					) AS retain_query
// 				),
// 				CurrentYearEarnings AS (
// 					SELECT
// 						'Liabilities & Equities' AS main_type,
// 						'Equities' AS main_group_type,
// 						'Current Year Earnings' AS account_name,
// 						'Equities' AS sub_type,
// 						0 AS account_id,
// 						current_year_amount AS amount
// 					FROM (
// 						SELECT SUM(credit) - SUM(debit) AS current_year_amount
// 						FROM account_currency_daily_balances
// 						WHERE
// 							business_id = ?
// 							AND branch_id = ?
// 							AND currency_id = ?
// 							AND transaction_date >= ?
// 							AND transaction_date <= ?
// 							AND account_id IN (
// 								SELECT id FROM accounts WHERE main_type IN ('INCOME', 'EXPENSE')
// 							)
// 					) AS current_year_query
// 				)
// 				SELECT
// 					CASE
// 						WHEN main_type = 'Asset' THEN 'Asset'
// 						WHEN main_type = 'Liability' OR main_type = 'Equity' THEN 'Liabilities & Equities'
// 					END AS main_type,
// 					CASE
// 						WHEN detail_type = 'FixedAsset' THEN 'Fixed Asset'
// 						WHEN main_type = 'Liability' THEN 'Liabilities'
// 						WHEN main_type = 'Equity' THEN 'Equities'
// 						ELSE 'Current Asset'
// 					END AS main_group_type,
// 					CASE
// 						WHEN detail_type = 'FixedAsset' THEN 'Fixed Assets'
// 						WHEN detail_type = 'Cash' THEN 'Cash'
// 						WHEN detail_type = 'Bank' THEN 'Bank'
// 						WHEN detail_type = 'Stock' THEN 'Stock'
// 						WHEN detail_type = 'AccountsReceivable' THEN 'Accounts Receivable'
// 						WHEN detail_type = 'OtherCurrentAsset' THEN 'Other Current Asset'
// 						WHEN detail_type = 'AccountsPayable' OR detail_type = 'OtherCurrentLiability' THEN 'Current Liabilities'
// 						WHEN detail_type = 'OtherLiability' THEN 'Other Liabilities'
// 						ELSE 'Equities'
// 					END AS sub_type,
// 					account_name,
// 					account_id,
// 					CASE
// 						WHEN main_type IN ('Liability', 'Equity') THEN
// 							CASE
// 								WHEN amount < 0 THEN -amount
// 								ELSE -amount
// 							END
// 						ELSE amount
// 					END AS amount
// 				FROM LastRows

// 				UNION ALL

// 				SELECT
// 					main_type,
// 					main_group_type,
// 					sub_type,
// 					account_name,
// 					account_id,
// 					amount
// 				FROM RetainedEarnings

// 				UNION ALL

// 				SELECT
// 					main_type,
// 					main_group_type,
// 					sub_type,
// 					account_name,
// 					account_id,
// 					amount
// 				FROM CurrentYearEarnings

// 				ORDER BY
// 					CASE
// 						WHEN main_type = 'Asset' AND main_group_type = 'Current Asset' AND sub_type = 'Cash' THEN 1
// 						WHEN main_type = 'Asset' AND main_group_type = 'Current Asset' AND sub_type = 'Bank' THEN 2
// 						WHEN main_type = 'Asset' AND main_group_type = 'Current Asset' AND sub_type = 'Accounts Receivable' THEN 3
// 						WHEN main_type = 'Asset' AND main_group_type = 'Current Asset' AND sub_type = 'Other Current Asset' THEN 4
// 						WHEN main_type = 'Asset' AND main_group_type = 'Fixed Asset' THEN 5
// 						WHEN main_type = 'Liabilities & Equities' AND main_group_type = 'Liabilities' AND sub_type = 'Current Liabilities' THEN 6
// 						WHEN main_type = 'Liabilities & Equities' AND main_group_type = 'Liabilities' AND sub_type = 'Other Liabilities' THEN 7
// 						WHEN main_group_type = 'Equities' AND account_id > 0 THEN 8
// 						ELSE 9
// 					END
// 			`
// 			args := []interface{}{businessId, business.BaseCurrencyId, fromDate, toDate}
// 			if *branchID != 0 {
// 				args = append(args, branchID)
// 			}
// 			args = append(args, businessId, branchID, business.BaseCurrencyId, fromDate)
// 			args = append(args, businessId, branchID, business.BaseCurrencyId, fromDate, toDate)

// 			rows, err := db.Raw(query, args...).Rows()

// 			if err != nil {
// 				return nil, err
// 			}
// 			defer rows.Close()

// 	// Iterate over the rows and scan into BalanceSheet structs
// 	for rows.Next() {
// 		var balance models.BalanceSheet
// 		var amount sql.NullString

// 		if err := rows.Scan(
// 			&balance.AccountMainType,
// 			&balance.AccountGroupType,
// 			&balance.AccountSubType,
// 			&balance.AccountName,
// 			&balance.AccountId,
// 			&amount,
// 		); err != nil {
// 			return nil, err
// 		}

// 		// Check if amount is valid before parsing
// 		if amount.Valid {
// 			amountValue, err := decimal.NewFromString(amount.String)
// 			if err != nil {
// 				return nil, err
// 			}
// 			balance.Amount = amountValue
// 		} else {
// 			// Handle NULL values for amount field as needed
// 			balance.Amount = decimal.NewFromFloat(0) // or any other default value
// 		}

// 		balances = append(balances, *&balance)
// 	}
// 	if err := rows.Err(); err != nil {
// 		return nil, err
// 	}

// 	// Group balances by main type, group type, and sub type
// 	groupedBalances := make(map[string]map[string]map[string][]models.BalanceSheet)
// 	for _, balance := range balances {
// 		if _, ok := groupedBalances[balance.AccountMainType]; !ok {
// 			groupedBalances[balance.AccountMainType] = make(map[string]map[string][]models.BalanceSheet)
// 		}
// 		if _, ok := groupedBalances[balance.AccountMainType][balance.AccountGroupType]; !ok {
// 			groupedBalances[balance.AccountMainType][balance.AccountGroupType] = make(map[string][]models.BalanceSheet)
// 		}
// 		groupedBalances[balance.AccountMainType][balance.AccountGroupType][balance.AccountSubType] = append(groupedBalances[balance.AccountMainType][balance.AccountGroupType][balance.AccountSubType], balance)
// 	}

// 	// Construct the response JSON with totals
// 	var balanceSheetResponse []*models.BalanceSheetResponse
// 	for mainType, groupTypes := range groupedBalances {
// 		var mainTypeResponse models.BalanceSheetResponse
// 		mainTypeResponse.MainType = mainType

// 		// Initialize total for the main type
// 		var mainTypeTotal decimal.Decimal

// 		for groupType, subTypes := range groupTypes {
// 			var groupTypeResponse models.MainType
// 			groupTypeResponse.GroupType = groupType

// 			// Initialize total for the group type
// 			var groupTypeTotal decimal.Decimal

// 			for subType, accounts := range subTypes {
// 				var subTypeResponse models.GroupType
// 				subTypeResponse.SubType = subType

// 				// Initialize total for the sub type
// 				var subTypeTotal decimal.Decimal

// 				for _, account := range accounts {
// 					var accountResponse models.SubType
// 					accountResponse.AccountName = account.AccountName
// 					accountResponse.AccountId = account.AccountId
// 					accountResponse.Amount = account.Amount

// 					// Add amount to sub type total
// 					subTypeTotal = subTypeTotal.Add(account.Amount)

// 					subTypeResponse.Accounts = append(subTypeResponse.Accounts, accountResponse)
// 				}

// 				// Add sub type total to group type total
// 				groupTypeTotal = groupTypeTotal.Add(subTypeTotal)

// 				// Append sub type total to sub types
// 				subTypeResponse.Total = subTypeTotal
// 				groupTypeResponse.Accounts = append(groupTypeResponse.Accounts, subTypeResponse)
// 			}

// 			// Add group type total to main type total
// 			mainTypeTotal = mainTypeTotal.Add(groupTypeTotal)

// 			// Append group type total to group types
// 			groupTypeResponse.Total = groupTypeTotal
// 			mainTypeResponse.Accounts = append(mainTypeResponse.Accounts, groupTypeResponse)
// 		}

// 		// Add main type total to main type response
// 		mainTypeResponse.Total = mainTypeTotal

// 		// Append main type response to balance sheet response
// 		balanceSheetResponse = append(balanceSheetResponse, &mainTypeResponse)
// 	}

// 	// Sort BalanceSheetResponse slice by MainType
// 	sort.Slice(balanceSheetResponse, func(i, j int) bool {
// 		return balanceSheetResponse[i].MainType < balanceSheetResponse[j].MainType
// 	})

// 	// Sort MainType slice within BalanceSheetResponse
// 	for _, mainTypeResponse := range balanceSheetResponse {
// 		sort.Slice(mainTypeResponse.Accounts, func(i, j int) bool {
// 			return mainTypeResponse.Accounts[i].GroupType < mainTypeResponse.Accounts[j].GroupType
// 		})
// 	}
// 	return balanceSheetResponse, nil
// }

func GetBalanceSheetReport(ctx context.Context, toDate models.MyDateString, reportType string, branchID *int) ([]*models.BalanceSheetResponse, error) {
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

	// Initialize branchID to 0 if it's nil
	if branchID == nil {
		branchID = new(int)
		*branchID = 0
	}

	var balances []models.BalanceSheet

	rows, err := db.Raw(`
        WITH LastRows AS (
            SELECT
                -- acb.id,
                ac.main_type AS main_type,
                ac.detail_type AS detail_type,
                ac.name AS account_name,
                acb.account_id AS account_id,
                acb.running_balance AS amount,
				ac.parent_account_id AS parent_account_id,
                ROW_NUMBER() OVER (PARTITION BY acb.account_id ORDER BY acb.transaction_date DESC) AS row_num
            FROM
                account_currency_daily_balances AS acb
            JOIN
                accounts AS ac ON acb.account_id = ac.id
            WHERE
                acb.business_id= ?
                AND acb.branch_id = ?
				AND acb.currency_id = ?
                AND acb.transaction_date <= ?
                AND acb.account_id IN (
                    SELECT id FROM accounts WHERE main_type IN ('Asset','Liability','Equity')
                )

        ),

		-- CTE to include parent accounts if missing in the main result
		ParentAccounts AS (
			SELECT DISTINCT
				ac.main_type AS main_type,
				ac.detail_type AS detail_type,
				ac.name AS account_name,
				ac.id AS account_id,
				0 AS amount,
				ac.parent_account_id AS parent_account_id,
				1 AS row_num
			FROM
				accounts ac
			WHERE
				ac.id IN (SELECT DISTINCT parent_account_id FROM LastRows WHERE parent_account_id <> 0)
				AND ac.id NOT IN (SELECT DISTINCT account_id FROM LastRows)
		),

        RetainedEarnings AS (
            SELECT
                -- -1 AS id,
                'Liabilities & Equities' AS main_type,
                'Equities' AS main_group_type,
                'Retained Earnings' AS account_name,
                'Equities' AS sub_type,
                0 AS account_id,
                retain_amount AS amount
            FROM (
                SELECT SUM(credit) - SUM(debit) as retain_amount
                FROM account_currency_daily_balances
                WHERE
                    business_id= ?
                    AND branch_id = ?
					AND currency_id = ?
                    AND transaction_date < ?
                    AND account_id IN (
                        SELECT id FROM accounts WHERE main_type IN ('INCOME', 'EXPENSE')
                    )
            ) AS retain_query
            WHERE retain_amount <> 0
        ),
        CurrentYearEarnings AS (
            SELECT
                -- -1 AS id,
                'Liabilities & Equities' AS main_type,
                'Equities' AS main_group_type,
                'Current Year Earnings' AS account_name,
                'Equities' AS sub_type,
                0 AS account_id,
                current_year_amount AS amount
            FROM (
                SELECT SUM(credit) - SUM(debit) as current_year_amount
                FROM account_currency_daily_balances
                WHERE
                    business_id= ?
                    AND branch_id = ?
					AND currency_id = ?
                    AND transaction_date >= ?
                    AND transaction_date <= ?
                    AND account_id IN (
                        SELECT id FROM accounts WHERE main_type IN ('INCOME', 'EXPENSE')
                    )
            ) AS current_year_query
        )

        SELECT
			CASE
				WHEN lr.main_type = 'Asset' THEN 'Asset'
				WHEN lr.main_type = 'Liability' OR lr.main_type = 'Equity' THEN 'Liabilities & Equities'
			END AS main_type,
			CASE
				WHEN lr.detail_type = 'FixedAsset' THEN 'Fixed Asset'
				WHEN lr.detail_type = 'OtherAsset' THEN 'Other Asset' -- up
				WHEN lr.main_type = 'Liability' THEN 'Liabilities'
				WHEN lr.main_type = 'Equity' THEN 'Equities'
				ELSE 'Current Asset'
			END AS main_group_type,
			CASE
				WHEN lr.detail_type = 'FixedAsset' THEN 'Fixed Asset'
				WHEN lr.detail_type = 'OtherAsset' THEN 'Other Asset' -- up
				WHEN lr.detail_type = 'Cash' THEN 'Cash'
				WHEN lr.detail_type = 'Bank' THEN 'Bank'
				WHEN lr.detail_type = 'Stock' THEN 'Stock'
				WHEN lr.detail_type = 'AccountsReceivable' THEN 'Accounts Receivable'
				WHEN lr.detail_type = 'OtherCurrentAsset' THEN 'Other Current Asset'
				WHEN lr.detail_type = 'AccountsPayable' OR lr.detail_type = 'OtherCurrentLiability' THEN 'Current Liabilities'
				WHEN lr.detail_type = 'LongTermLiability' THEN 'Long Term Liabilities'
				WHEN lr.detail_type = 'OtherLiability' THEN 'Other Liabilities'
				ELSE 'Equities'
			END AS sub_type,
			lr.account_name,
			lr.account_id,
			CASE 
				WHEN lr.main_type IN ('Liability', 'Equity') THEN 
					CASE 
						WHEN lr.amount < 0 THEN -lr.amount
						ELSE -lr.amount
					END
				ELSE lr.amount
			END AS amount,
			parent_ac.name AS parent_account_name
		FROM
			LastRows lr
		LEFT JOIN
			accounts parent_ac ON lr.parent_account_id = parent_ac.id
		WHERE
			lr.row_num = 1

        UNION ALL

		SELECT
			CASE
				WHEN ParentAccounts.main_type = 'Asset' THEN 'Asset'
				WHEN ParentAccounts.main_type = 'Liability' OR ParentAccounts.main_type = 'Equity' THEN 'Liabilities & Equities'
			END AS main_type,
			CASE
				WHEN ParentAccounts.detail_type = 'FixedAsset' THEN 'Fixed Asset'
				WHEN ParentAccounts.detail_type = 'OtherAsset' THEN 'Other Asset' -- up
				WHEN ParentAccounts.main_type = 'Liability' THEN 'Liabilities'
				WHEN ParentAccounts.main_type = 'Equity' THEN 'Equities'
				ELSE 'Current Asset'
			END AS main_group_type,
			CASE
				WHEN ParentAccounts.detail_type = 'FixedAsset' THEN 'Fixed Asset'
				WHEN ParentAccounts.detail_type = 'OtherAsset' THEN 'Other Asset' -- up
				WHEN ParentAccounts.detail_type = 'Cash' THEN 'Cash'
				WHEN ParentAccounts.detail_type = 'Bank' THEN 'Bank'
				WHEN ParentAccounts.detail_type = 'Stock' THEN 'Stock'
				WHEN ParentAccounts.detail_type = 'AccountsReceivable' THEN 'Accounts Receivable'
				WHEN ParentAccounts.detail_type = 'OtherCurrentAsset' THEN 'Other Current Asset'
				WHEN ParentAccounts.detail_type = 'AccountsPayable' OR ParentAccounts.detail_type = 'OtherCurrentLiability' THEN 'Current Liabilities'
				WHEN ParentAccounts.detail_type = 'LongTermLiability' THEN 'Long Term Liabilities'
				WHEN ParentAccounts.detail_type = 'OtherLiability' THEN 'Other Liabilities'
				ELSE 'Equities'
			END AS sub_type,
			account_name,
			account_id,
			amount,
			NULL AS parent_account_name
		FROM
			ParentAccounts -- Include parent accounts

		UNION ALL

        SELECT
            -- id,
            main_type,
            main_group_type,
            sub_type,
            account_name,
            account_id,
            amount,
			NULL AS parent_account_name

        FROM
            RetainedEarnings

        UNION ALL

        SELECT
            -- id,
            main_type,
            main_group_type,
            sub_type,
            account_name,
            account_id,
            amount,
			NULL AS parent_account_name

        FROM
            CurrentYearEarnings

        ORDER BY
            CASE
                WHEN main_type = 'Asset' AND  main_group_type = 'Current Asset' AND sub_type = 'Cash' THEN 1
                WHEN main_type = 'Asset' AND  main_group_type = 'Current Asset' AND sub_type = 'Bank' THEN 2
                WHEN main_type = 'Asset' AND  main_group_type = 'Current Asset' AND sub_type = 'Accounts Receivable' THEN 3
                WHEN main_type = 'Asset' AND  main_group_type = 'Current Asset' AND sub_type = 'Other Current Asset' THEN 4
                WHEN main_type = 'Asset' AND main_group_type = 'Fixed Asset' THEN 5
				WHEN main_type = 'Liabilities & Equities' AND main_group_type = 'Liabilities' AND sub_type = 'Current Liabilities' THEN 6
				WHEN main_type = 'Liabilities & Equities' AND main_group_type = 'Liabilities' AND sub_type = 'LongTermLiability' THEN 7
                WHEN main_type = 'Liabilities & Equities' AND main_group_type = 'Liabilities' AND sub_type = 'Other Liabilities' THEN 8
                WHEN main_group_type = 'Equities'  and account_id > 0 THEN 9
                ELSE 10
            END;

    `, businessId, *branchID, business.BaseCurrencyId, toDate,
		businessId, *branchID, business.BaseCurrencyId, fromDate,
		businessId, *branchID, business.BaseCurrencyId, fromDate, toDate).Rows()

	if err != nil {
		return nil, err
	}
	defer rows.Close()

	// Iterate over the rows and scan into BalanceSheet structs
	for rows.Next() {
		var balance models.BalanceSheet
		var amount sql.NullString
		var parentAccountName sql.NullString

		if err := rows.Scan(
			&balance.AccountMainType,
			&balance.AccountGroupType,
			&balance.AccountSubType,
			&balance.AccountName,
			&balance.AccountId,
			&amount,
			&parentAccountName,
		); err != nil {
			return nil, err
		}

		// Check if amount is valid before parsing
		if amount.Valid {
			amountValue, err := decimal.NewFromString(amount.String)
			if err != nil {
				return nil, err
			}
			balance.Amount = amountValue
		} else {
			// Handle NULL values for amount field as needed
			balance.Amount = decimal.NewFromFloat(0) // or any other default value
		}

		// Add parent account
		if parentAccountName.Valid {
			balance.ParentAccountName = parentAccountName.String
		}

		balances = append(balances, balance)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	groupedBalances := make(map[string]map[string]map[string][]models.BalanceSheet)
	parentChildAccounts := make(map[string][]models.BalanceSheet) // New map for parent-child relationships

	for _, balance := range balances {
		if balance.ParentAccountName == "" {
			// No parent, group normally
			if _, ok := groupedBalances[balance.AccountMainType]; !ok {
				groupedBalances[balance.AccountMainType] = make(map[string]map[string][]models.BalanceSheet)
			}
			if _, ok := groupedBalances[balance.AccountMainType][balance.AccountGroupType]; !ok {
				groupedBalances[balance.AccountMainType][balance.AccountGroupType] = make(map[string][]models.BalanceSheet)
			}
			// Check for same group and subtype
			if balance.AccountGroupType == balance.AccountSubType {
				// Handle collapsing or separating sub_type when same as main_group_type
				groupedBalances[balance.AccountMainType][balance.AccountGroupType][""] = append(groupedBalances[balance.AccountMainType][balance.AccountGroupType][""], balance)
			} else {
				groupedBalances[balance.AccountMainType][balance.AccountGroupType][balance.AccountSubType] = append(groupedBalances[balance.AccountMainType][balance.AccountGroupType][balance.AccountSubType], balance)
			}
			// groupedBalances[balance.AccountMainType][balance.AccountGroupType][balance.AccountSubType] = append(groupedBalances[balance.AccountMainType][balance.AccountGroupType][balance.AccountSubType], balance)
		} else {
			// If the account has a parent, append it to the parent's list of sub-accounts
			parentChildAccounts[balance.ParentAccountName] = append(parentChildAccounts[balance.ParentAccountName], balance)

		}
	}

	// Construct the response JSON with totals
	var balanceSheetResponse []*models.BalanceSheetResponse

	for mainType, groupTypes := range groupedBalances {
		var mainTypeResponse models.BalanceSheetResponse
		mainTypeResponse.MainType = mainType
		var mainTypeTotal decimal.Decimal

		for groupType, subTypes := range groupTypes {
			var groupTypeResponse models.MainType
			groupTypeResponse.GroupType = groupType
			var groupTypeTotal decimal.Decimal

			for subType, accounts := range subTypes {
				var subTypeResponse models.GroupType
				subTypeResponse.SubType = subType
				var subTypeTotal decimal.Decimal

				for _, account := range accounts {
					var accountResponse models.SubType
					accountResponse.AccountName = account.AccountName
					accountResponse.AccountId = account.AccountId
					accountResponse.Amount = account.Amount
					accountResponse.ParentAccountName = account.ParentAccountName
					accountResponse.Total = accountResponse.Total.Add(account.Amount)
					// Add sub-accounts
					if subAccounts, ok := parentChildAccounts[account.AccountName]; ok {
						for _, subAccount := range subAccounts {
							accountResponse.SubAccounts = append(accountResponse.SubAccounts, models.SubAccount{
								AccountName: subAccount.AccountName,
								AccountId:   subAccount.AccountId,
								Amount:      subAccount.Amount,
							})
							accountResponse.Total = accountResponse.Total.Add(subAccount.Amount)
						}
					}

					// subTypeTotal = subTypeTotal.Add(account.Amount)
					subTypeTotal = subTypeTotal.Add(accountResponse.Total)
					subTypeResponse.Accounts = append(subTypeResponse.Accounts, accountResponse)
				}

				groupTypeTotal = groupTypeTotal.Add(subTypeTotal)
				subTypeResponse.Total = subTypeTotal
				groupTypeResponse.Accounts = append(groupTypeResponse.Accounts, subTypeResponse)
			}

			mainTypeTotal = mainTypeTotal.Add(groupTypeTotal)
			groupTypeResponse.Total = groupTypeTotal
			mainTypeResponse.Accounts = append(mainTypeResponse.Accounts, groupTypeResponse)
		}

		mainTypeResponse.Total = mainTypeTotal
		balanceSheetResponse = append(balanceSheetResponse, &mainTypeResponse)
	}

	sort.Slice(balanceSheetResponse, func(i, j int) bool {
		return balanceSheetResponse[i].MainType < balanceSheetResponse[j].MainType
	})

	for _, mainTypeResponse := range balanceSheetResponse {
		sort.Slice(mainTypeResponse.Accounts, func(i, j int) bool {
			return mainTypeResponse.Accounts[i].GroupType < mainTypeResponse.Accounts[j].GroupType
		})
	}

	return balanceSheetResponse, nil
}
