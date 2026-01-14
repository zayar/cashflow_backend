package reports

import (
	"context"
	"errors"

	"github.com/mmdatafocus/books_backend/config"
	"github.com/mmdatafocus/books_backend/models"
	"github.com/mmdatafocus/books_backend/utils"
	"github.com/shopspring/decimal"
)

type MovementOfEquityResponse struct {
	OpeningBalance decimal.Decimal
	NetChange      decimal.Decimal
	ClosingBalance decimal.Decimal
	AccountGroups  []CashAccountGroup
}

func GetMovementOfEquityReport(ctx context.Context, fromDate models.MyDateString, toDate models.MyDateString, reportType string, branchID *int) ([]*MovementOfEquityResponse, error) {
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
        WITH OpeningBalance AS (
			SELECT 
				'' AS group_name,
				'Opening Balance'  AS account_name ,
				'' AS account_code,
				0 AS account_id,
				COALESCE(SUM(acb.credit) - SUM(acb.debit), 0) AS amount
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
					SELECT id FROM accounts WHERE main_type IN  ('EQUITY')
				)
		),
		CurrentYearEarnings AS (
			SELECT 
				ac.detail_type AS detail_type,
				acb.running_balance AS amount,
				ROW_NUMBER() OVER (PARTITION BY acb.account_id ORDER BY acb.transaction_date DESC) AS row_num
			FROM 
				account_currency_daily_balances AS acb
			JOIN
				accounts AS ac ON acb.account_id = ac.id
			WHERE 
				acb.business_id=? 
				AND acb.branch_id = ?
				AND acb.currency_id = ?
				AND acb.transaction_date >= ?
				AND acb.transaction_date <= ?
				AND acb.account_id IN (
					SELECT id FROM accounts WHERE main_type IN ('Income','Expense')
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
					SELECT id FROM accounts WHERE main_type IN  ('EQUITY')
				)
		),
		MainQuery AS (
			SELECT 
				'Changes in Equity' AS group_name,
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
					SELECT id FROM accounts WHERE main_type IN  ('EQUITY')
				)
			GROUP BY 
				ac.detail_type, ac.main_type, ac.name, ac.id
		)

		SELECT * FROM OpeningBalance

		UNION ALL

		SELECT 
			'Changes in Equity' AS group_name,
			'Current Year Earnings' AS account_name,
			'' AS account_code,
			0 AS account_id,
            COALESCE(
				SUM(CASE 
					WHEN detail_type IN ('Income', 'OtherIncome') THEN -amount
					ELSE -amount
				END) , 
			0)
			AS amount
		FROM 
			CurrentYearEarnings
		WHERE 
			row_num = 1

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
						FROM CurrentYearEarnings
						WHERE row_num = 1
					) AS net_income)  ,
              0)
                AS amount

		UNION ALL

		SELECT * FROM MainQuery

		UNION ALL

		SELECT 
			'' AS group_name,
			'Closing Balance' AS account_name,
			'' AS account_code,
			0 AS account_id,
            COALESCE(
				(SELECT amount FROM OpeningBalance) + 
				(SELECT amount FROM NetChange) +
				(SELECT SUM(net_profit) FROM (
						SELECT 
							CASE 
								WHEN detail_type IN ('Income', 'OtherIncome') THEN -amount
								ELSE -amount
							END AS net_profit
						FROM CurrentYearEarnings
						WHERE row_num = 1
					) AS net_income)
             ,0)
			AS amount

		ORDER BY 
			CASE 
				WHEN account_name = 'Opening Balance'  THEN 1
				WHEN group_name = 'Changes in Equity'  THEN 2
                WHEN account_name = 'Net Changes in Equity'  THEN 3
				ELSE 4
			END,
			account_name ASC;
    `

	rows, err := db.Raw(query,
		businessID, *branchID, business.BaseCurrencyId, fromDate,
		businessID, *branchID, business.BaseCurrencyId, fromDate, toDate,
		businessID, *branchID, business.BaseCurrencyId, fromDate, toDate,
		businessID, *branchID, business.BaseCurrencyId, fromDate, toDate,
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
		case "Opening Balance":
			beginningCash = amount
		case "Net Change":
			netChange = amount
		case "Closing Balance":
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

	response := &MovementOfEquityResponse{
		OpeningBalance: beginningCash,
		NetChange:      netChange,
		ClosingBalance: endingCash,
		AccountGroups:  cashAccountGroups,
	}

	return []*MovementOfEquityResponse{response}, nil
}
