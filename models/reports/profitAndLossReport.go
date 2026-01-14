package reports

import (
	"context"
	"errors"

	"github.com/mmdatafocus/books_backend/config"
	"github.com/mmdatafocus/books_backend/models"
	"github.com/mmdatafocus/books_backend/utils"
	"github.com/shopspring/decimal"
)

func GetProfitAndLossReport(ctx context.Context, fromDate models.MyDateString, toDate models.MyDateString, reportType string, branchID *int) (*models.ProfitAndLossResponse, error) {
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

	rows, err := db.Raw(`
        WITH LastRows AS (
            SELECT 
                ac.main_type AS main_type,
                ac.detail_type AS detail_type,
                ac.name AS account_name,
				ac.system_default_code AS system_code,
                acb.account_id AS account_id,
                SUM(acb.balance) AS amount
            FROM 
                account_currency_daily_balances AS acb
            JOIN
                accounts AS ac ON acb.account_id = ac.id
            WHERE 
                acb.business_id= ? 
                AND acb.branch_id= ?
                AND acb.currency_id = ?
                AND acb.transaction_date >= ? -- Start of date range
                AND acb.transaction_date <= ? -- End of date range
                AND acb.account_id IN (
                    SELECT id FROM accounts WHERE main_type IN ('Income','Expense')
                )
			GROUP BY 
				ac.main_type, ac.detail_type, ac.name, acb.account_id
        )
        SELECT 
            CASE 
                WHEN detail_type = 'Income' THEN 'Operating Income'
                WHEN detail_type = 'Expense' THEN 'Operating Expense'
                WHEN detail_type = 'CostOfGoodsSold' THEN 'Cost Of Goods Sold'
                WHEN detail_type = 'OtherIncome' THEN 'Non Operating Income'
                WHEN detail_type = 'OtherExpense' THEN 'Non Operating Expense'
            END AS group_type,
            main_type,
            detail_type,
            account_name,
			system_code,
            account_id,
            CASE 
                WHEN main_type IN ('Income') THEN 
                    CASE 
                        WHEN amount < 0 THEN -amount 
                        ELSE -amount  
                    END
                ELSE amount
            END AS amount
        FROM 
            LastRows
        ORDER BY 
            CASE 
                WHEN detail_type = 'Income'  THEN 1
                WHEN detail_type = 'CostOfGoodsSold'  THEN 2
                WHEN detail_type = 'Expense'  THEN 3
                WHEN detail_type = 'OtherIncome'  THEN 4
                ELSE 5
            END;

    `, businessId, branchID, business.BaseCurrencyId, fromDate, toDate).Rows()

	if err != nil {
		return nil, err
	}
	defer rows.Close()

	// Map to store results grouped by groupType
	groupedResults := make(map[string]*models.PlAccountGroup)
	// Initialize a slice to maintain the order
	var groupOrder []string

	// Iterate over rows
	for rows.Next() {
		var groupType, mainType, detailType, accountName, systemCode string
		var accountID int
		var amountStr string

		// Scan the values from the row into variables
		err := rows.Scan(&groupType, &mainType, &detailType, &accountName, &systemCode, &accountID, &amountStr)
		if err != nil {
			return nil, err
		}

		// Parse the amount string to decimal.Decimal
		amount, err := decimal.NewFromString(amountStr)
		if err != nil {
			return nil, err
		}

		// if sales discount
		if systemCode == "507" {
			groupType = "Operating Income"
			detailType = "Income"
			mainType = "Income"
			amount = amount.Neg()
		} else if systemCode == "405" { // purchase discount
			groupType = "Operating Expense"
			detailType = "Expense"
			mainType = "Expense"
			amount = amount.Neg()
		}

		// Check if groupType already exists in the map
		if _, ok := groupedResults[groupType]; !ok {
			// If groupType doesn't exist, add it to the map and the order slice
			groupedResults[groupType] = &models.PlAccountGroup{
				GroupType: groupType,
				Total:     decimal.Zero,
				Accounts:  []models.AccountGroupItem{},
			}
			groupOrder = append(groupOrder, groupType)
		}

		// Append the current row data to the respective groupType
		groupedResults[groupType].Accounts = append(groupedResults[groupType].Accounts, models.AccountGroupItem{
			MainType:    mainType,
			DetailType:  detailType,
			AccountName: accountName,
			AccountID:   accountID,
			Amount:      amount,
		})

		// Update the total amount for the group type
		groupedResults[groupType].Total = groupedResults[groupType].Total.Add(amount)
	}

	// Calculate gross profit, operating cost, and net profit/loss
	var grossProfit, operatingCost, operatingProfit, nonOperationIncome, nonOperationExpense, netProfit decimal.Decimal
	for _, groupType := range groupOrder {
		group := groupedResults[groupType]

		switch group.GroupType {
		case "Operating Income":
			grossProfit = grossProfit.Add(group.Total)
		case "Cost Of Goods Sold":
			grossProfit = grossProfit.Sub(group.Total)
		case "Operating Expense":
			operatingCost = operatingCost.Add(group.Total)
		case "Non Operating Income":
			nonOperationIncome = nonOperationIncome.Add(group.Total)
		case "Non Operating Expense":
			nonOperationExpense = nonOperationExpense.Add(group.Total)
		}
	}

	// Calculate net profit as per the formula
	operatingProfit = grossProfit.Sub(operatingCost)
	netProfit = operatingProfit.Add(nonOperationIncome).Sub(nonOperationExpense)

	// Create ProfitAndLossResponse object
	response := &models.ProfitAndLossResponse{
		GrossProfit:     grossProfit,
		OperatingProfit: operatingProfit,
		NetProfit:       netProfit,
		PlAccountGroups: make([]models.PlAccountGroup, 0, len(groupedResults)),
	}

	// Append grouped results to PlAccountGroups in the order of insertion
	for _, groupType := range groupOrder {
		response.PlAccountGroups = append(response.PlAccountGroups, *groupedResults[groupType])
	}

	return response, nil
}
