package reports

import (
	"context"
	"errors"

	"bitbucket.org/mmdatafocus/books_backend/config"
	"bitbucket.org/mmdatafocus/books_backend/models"
	"bitbucket.org/mmdatafocus/books_backend/utils"
	"github.com/shopspring/decimal"
)

type CashFlowResponse struct {
	BeginCashBalance  decimal.Decimal
	NetChange         decimal.Decimal
	EndCashBalance    decimal.Decimal
	CashAccountGroups []CashAccountGroup
}
type CashAccountGroup struct {
	GroupName string
	Total     decimal.Decimal
	Accounts  []GroupItem
}
type GroupItem struct {
	AccountName string
	AccountCode string
	AccountID   int
	Amount      decimal.Decimal
}

func GetCashFlowReport(ctx context.Context, fromDate models.MyDateString, toDate models.MyDateString, reportType string, branchID *int) ([]*CashFlowResponse, error) {
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

	accountCashTypes := []string{
		string(models.AccountDetailTypeCash),
		string(models.AccountDetailTypeBank),
	}

	accountIncomeExpenseTypes := []string{
		string(models.AccountMainTypeIncome),
		string(models.AccountMainTypeExpense),
	}

	accountMainTypes := []string{
		string(models.AccountMainTypeAsset),
		string(models.AccountMainTypeLiability),
		string(models.AccountMainTypeEquity),
	}

	query := `
        WITH BeginningCash AS (
			SELECT 
				'' AS group_name,
				'Beginning Cash Balance'  AS account_name ,
				'' AS account_code,
				0 AS account_id,
				COALESCE(SUM(acb.debit) - SUM(acb.credit), 0) AS amount
			FROM 
				account_currency_daily_balances AS acb
			JOIN
				accounts AS ac ON acb.account_id = ac.id
			WHERE 
				acb.business_id = ? 
				AND acb.branch_id = ?
				AND acb.currency_id = ?
				AND acb.transaction_date < ?
				AND acb.account_id IN (
					SELECT id FROM accounts WHERE detail_type IN  ?
				)
		),
		NetIncome AS (
			SELECT 
				ac.detail_type AS detail_type,
				acb.balance AS amount
				-- acb.running_balance AS amount,
				-- ROW_NUMBER() OVER (PARTITION BY acb.account_id ORDER BY acb.transaction_date DESC) AS row_num
			FROM 
				account_currency_daily_balances AS acb
			JOIN
				accounts AS ac ON acb.account_id = ac.id
			WHERE 
				acb.business_id=? 
				AND acb.branch_id = ?
				AND acb.currency_id = ?
				AND acb.transaction_date BETWEEN ? AND ?
				AND acb.account_id IN (
					SELECT id FROM accounts WHERE main_type IN ?
				)
		),
		NetChange AS (
			SELECT 
				SUM(acb.credit) - SUM(acb.debit)  AS amount
			FROM 
				account_currency_daily_balances AS acb
			JOIN
				accounts AS ac ON acb.account_id = ac.id
			WHERE 
				acb.business_id = ? 
				AND acb.branch_id = ?
				AND acb.currency_id = ?
				AND acb.transaction_date BETWEEN ? AND ?
				AND acb.account_id IN (
					SELECT id FROM accounts
						WHERE main_type IN ? -- ('ASSET','LIABILITY','EQUITY')  
						AND  NOT detail_type IN ? -- ('Bank','Cash')
				)
		),
		MainQuery AS (
			SELECT 
				CASE
					WHEN ac.detail_type = 'FixedAsset' THEN 'Investing Activities'
					WHEN ac.detail_type = 'Equity' THEN 'Financing Activities'
					ELSE 'Operating Activities'
				END AS group_name,
				ac.name AS account_name,
				ac.code AS account_code,
				ac.id AS account_id,
				SUM(acb.credit) - SUM(acb.debit) AS amount
			FROM 
				account_currency_daily_balances AS acb
			JOIN
				accounts AS ac ON acb.account_id = ac.id
			WHERE 
				acb.business_id = ? 
				AND acb.branch_id = ?
				AND acb.currency_id = ?
				AND acb.transaction_date BETWEEN ? AND ?
				AND acb.account_id IN (
					SELECT id FROM accounts
						WHERE main_type IN ? -- ('ASSET','LIABILITY','EQUITY')  
						AND  NOT detail_type IN ? -- ('Bank','Cash')
				)
			GROUP BY 
				ac.detail_type, ac.main_type, ac.name, ac.id
		)

		SELECT * FROM BeginningCash

		UNION ALL

		SELECT 
			'Operating Activities' AS group_name,
			'Net Income' AS account_name,
			'' AS account_code,
			0 AS account_id,
            COALESCE(
				SUM(CASE 
					WHEN detail_type IN ('Income', 'OtherIncome') THEN -amount
					ELSE -amount
				END) , 
			0)
			AS net_profit
		FROM 
			NetIncome
		-- WHERE 
			-- row_num = 1

		UNION ALL

		SELECT
			'' AS group_name,
				'Net Change' AS account_name,
				'' AS account_code,
				0 AS account_id,
                COALESCE(
					(SELECT amount FROM NetChange) + 
					(SELECT SUM(net_profit) FROM (
						SELECT 
							CASE 
								WHEN detail_type IN ('Income', 'OtherIncome') THEN -amount
								ELSE -amount
							END AS net_profit
						FROM NetIncome
						-- WHERE row_num = 1
					) AS net_income)  ,
                0)
                AS amount

		UNION ALL

		SELECT * FROM MainQuery

		UNION ALL

		SELECT 
			'' AS group_name,
			'Ending Cash Balance' AS account_name,
			'' AS account_code,
			0 AS account_id,
            COALESCE(
				(SELECT amount FROM BeginningCash) + 
				(SELECT amount FROM NetChange) +
				(SELECT SUM(net_profit) FROM (
						SELECT 
							CASE 
								WHEN detail_type IN ('Income', 'OtherIncome') THEN -amount
								ELSE -amount
							END AS net_profit
						FROM NetIncome
						-- WHERE row_num = 1
					) AS net_income)
             ,0)
			AS amount

		ORDER BY 
			CASE 
				WHEN account_name = 'Beginning Cash Balance'  THEN 1
				WHEN group_name = 'Operating Activities'  THEN 2
				WHEN group_name = 'Investing Activities' THEN 3
				WHEN group_name = 'Financing Activities' THEN 4
				WHEN account_name = 'Net Change' THEN 5
				ELSE 6
			END,
			account_name ASC;
    `

	rows, err := db.Raw(query,
		businessID, *branchID, business.BaseCurrencyId, fromDate, accountCashTypes,
		businessID, *branchID, business.BaseCurrencyId, fromDate, toDate, accountIncomeExpenseTypes,
		businessID, *branchID, business.BaseCurrencyId, fromDate, toDate, accountMainTypes, accountCashTypes,
		businessID, *branchID, business.BaseCurrencyId, fromDate, toDate, accountMainTypes, accountCashTypes,
	).Rows()

	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var beginningCash, netChange, endingCash decimal.Decimal
	var cashAccountGroups []CashAccountGroup

	for rows.Next() {
		var groupName, accountName, accountCode string
		var accountID int
		var amount decimal.Decimal

		err := rows.Scan(&groupName, &accountName, &accountCode, &accountID, &amount)
		if err != nil {
			return nil, err
		}

		switch accountName {
		case "Beginning Cash Balance":
			beginningCash = amount
		case "Net Change":
			netChange = amount
		case "Ending Cash Balance":
			endingCash = amount
		default:
			// Group the accounts by their respective groups
			var groupIndex int
			var found bool
			for i, group := range cashAccountGroups {
				if group.GroupName == groupName {
					groupIndex = i
					found = true
					break
				}
			}
			if !found {
				cashAccountGroups = append(cashAccountGroups, CashAccountGroup{GroupName: groupName})
				groupIndex = len(cashAccountGroups) - 1
			}
			// Add the account to the appropriate group
			groupItem := GroupItem{
				AccountName: accountName,
				AccountCode: accountCode,
				AccountID:   accountID,
				Amount:      amount,
			}
			cashAccountGroups[groupIndex].Accounts = append(cashAccountGroups[groupIndex].Accounts, groupItem)
		}
	}

	// Calculate total for each group
	for i := range cashAccountGroups {
		var total decimal.Decimal
		for _, account := range cashAccountGroups[i].Accounts {
			total = total.Add(account.Amount)
		}
		cashAccountGroups[i].Total = total
	}

	cashFlowResponse := &CashFlowResponse{
		BeginCashBalance:  beginningCash,
		NetChange:         netChange,
		EndCashBalance:    endingCash,
		CashAccountGroups: cashAccountGroups,
	}

	return []*CashFlowResponse{cashFlowResponse}, nil
}
