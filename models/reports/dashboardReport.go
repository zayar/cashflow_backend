package reports

import (
	"context"
	"errors"
	"strings"
	"time"

	"bitbucket.org/mmdatafocus/books_backend/config"
	"bitbucket.org/mmdatafocus/books_backend/models"
	"bitbucket.org/mmdatafocus/books_backend/utils"
	_ "github.com/go-sql-driver/mysql"
	"github.com/shopspring/decimal"
)

type PayableReceivableResponse struct {
	TotalReceivable APandAR `json:"totalReceivable"`
	TotalPayable    APandAR `json:"totalPayable"`
}

type APandAR struct {
	CurrencySymbol string          `json:"currency_symbol"`
	Total          decimal.Decimal `json:"total"`
	Current        decimal.Decimal `json:"current"`
	Int1to15       decimal.Decimal `json:"int1to15"`
	Int16to30      decimal.Decimal `json:"int16to30"`
	Int31to45      decimal.Decimal `json:"int31to45"`
	Int46plus      decimal.Decimal `json:"int46plus"`
}

type IncomeExpenseReponse struct {
	TotalIncome          decimal.Decimal        `json:"total_income"`
	TotalExpense         decimal.Decimal        `json:"total_expense"`
	IncomeExpenseDetails []IncomeExpenseDetails `json:"income_expense_detail"`
}

type IncomeExpenseDetails struct {
	Month         string          `json:"month"`
	IncomeAmount  decimal.Decimal `json:"income_amount"`
	ExpenseAmount decimal.Decimal `json:"expense_amount"`
}

type TopExpensesResponse struct {
	AccountName string          `json:"accountName"`
	Amount      decimal.Decimal `json:"amount"`
}

type CashFlowReponse struct {
	TotalOpeningBalance decimal.Decimal   `json:"total_opening_balance"`
	TotalIncomingAmount decimal.Decimal   `json:"total_incoming_amount"`
	TotalOutgoingAmount decimal.Decimal   `json:"total_outgoing_amount"`
	TotalEndingBalance  decimal.Decimal   `json:"total_ending_balance"`
	CashFlowDetails     []CashFlowDetails `json:"cash_flow_details"`
}

type CashFlowDetails struct {
	Month          string          `json:"month"`
	OpeningBalance decimal.Decimal `json:"opening_balance"`
	IncomingAmount decimal.Decimal `json:"incoming_amount"`
	OutgoingAmount decimal.Decimal `json:"outgoing_amount"`
	EndingBalance  decimal.Decimal `json:"ending_balance"`
}

func GetTotalPayableReceivable(ctx context.Context) ([]*PayableReceivableResponse, error) {

	db := config.GetDB()

	businessId, ok := utils.GetBusinessIdFromContext(ctx)
	if !ok || businessId == "" {
		return nil, errors.New("business id is required")
	}

	business, err := models.GetBusinessById(ctx, businessId)
	if err != nil {
		return nil, err
	}

	currency, err := models.GetCurrency(ctx, business.BaseCurrencyId)
	if err != nil {
		return nil, err
	}

	currentDate := time.Now().Format("2006-01-02")

	billStatus := []string{
		string(models.BillStatusConfirmed),
		string(models.BillStatusPartialPaid),
	}

	invoiceStatus := []string{
		string(models.SalesInvoiceStatusConfirmed),
		string(models.SalesInvoiceStatusPartialPaid),
	}

	var payableReceivableResponse PayableReceivableResponse

	payableQuery := `
    WITH Payable AS (
        SELECT
            b.supplier_id,
            b.currency_id,
            b.remaining_balance,
            CASE
                WHEN b.currency_id = ? THEN b.remaining_balance
                ELSE b.remaining_balance * b.exchange_rate
            END AS adjusted_remaining_balance,
            CASE
                WHEN b.remaining_balance > 0 THEN DATEDIFF(?, b.bill_due_date)
                ELSE 0
            END AS days_overdue
        FROM
            bills b
        WHERE
            b.business_id = ?
            AND b.bill_date < ?
            AND b.current_status IN ?
			AND b.remaining_balance > 0
    )
    SELECT
        SUM(adjusted_remaining_balance) as total,
        SUM(
            CASE
                WHEN days_overdue <= 0 THEN adjusted_remaining_balance
                ELSE 0
            END
        ) AS current,
        SUM(
            CASE
                WHEN days_overdue BETWEEN 1 AND 15 THEN adjusted_remaining_balance
                ELSE 0
            END
        ) AS int1to15,
        SUM(
            CASE
                WHEN days_overdue BETWEEN 16 AND 30 THEN adjusted_remaining_balance
                ELSE 0
            END
        ) AS int16to30,
        SUM(
            CASE
                WHEN days_overdue BETWEEN 31 AND 45 THEN adjusted_remaining_balance
                ELSE 0
            END
        ) AS int31to45,
        SUM(
            CASE
                WHEN days_overdue > 46 THEN adjusted_remaining_balance
                ELSE 0
            END
        ) AS int46plus
    FROM
        Payable;`

	receivableQuery := `
    WITH Receivable AS (
        SELECT
            CASE
                WHEN inv.currency_id = ? THEN inv.remaining_balance
                ELSE inv.remaining_balance * inv.exchange_rate
            END AS adjusted_remaining_balance,
            CASE
                WHEN inv.remaining_balance > 0 THEN DATEDIFF(?, inv.invoice_due_date)
                ELSE 0
            END AS days_overdue
        FROM
            sales_invoices inv
        WHERE
            inv.business_id = ?
            AND inv.invoice_date < ?
            AND inv.current_status IN ?
			AND inv.remaining_balance > 0
    )
    SELECT
        SUM(adjusted_remaining_balance) as total,
        SUM(
            CASE
                WHEN days_overdue <= 0 THEN adjusted_remaining_balance
                ELSE 0
            END
        ) AS current,
        SUM(
            CASE
                WHEN days_overdue BETWEEN 1 AND 15 THEN adjusted_remaining_balance
                ELSE 0
            END
        ) AS int1to15,
        SUM(
            CASE
                WHEN days_overdue BETWEEN 16 AND 30 THEN adjusted_remaining_balance
                ELSE 0
            END
        ) AS int16to30,
        SUM(
            CASE
                WHEN days_overdue BETWEEN 31 AND 45 THEN adjusted_remaining_balance
                ELSE 0
            END
        ) AS int31to45,
        SUM(
            CASE
                WHEN days_overdue > 46 THEN adjusted_remaining_balance
                ELSE 0
            END
        ) AS int46plus
    FROM
        Receivable;`

	//  payable query
	if err := db.Raw(payableQuery,
		business.BaseCurrencyId, currentDate, businessId, currentDate, billStatus).
		Scan(&payableReceivableResponse.TotalPayable).Error; err != nil {
		return nil, err
	}

	// receivable query
	if err := db.Raw(receivableQuery,
		business.BaseCurrencyId, currentDate, businessId, currentDate, invoiceStatus).
		Scan(&payableReceivableResponse.TotalReceivable).Error; err != nil {
		return nil, err
	}

	payableReceivableResponse.TotalPayable.CurrencySymbol = currency.Symbol
	payableReceivableResponse.TotalReceivable.CurrencySymbol = currency.Symbol

	return []*PayableReceivableResponse{&payableReceivableResponse}, nil
}

func GetTotalIncomeExpense(ctx context.Context, filterType string) (*IncomeExpenseReponse, error) {
	db := config.GetDB()

	businessId, ok := utils.GetBusinessIdFromContext(ctx)
	if !ok || businessId == "" {
		return nil, errors.New("business id is required")
	}

	business, err := models.GetBusinessById(ctx, businessId)
	if err != nil {
		return nil, err
	}

	fiscalYearStartMonth, err := utils.GetFiscalYearStartMonth(string(business.FiscalYear))
	if err != nil {
		return nil, err
	}

	startDate, endDate, err := utils.GetStartAndEndDateWithBusinessFiscalYear(fiscalYearStartMonth, filterType)
	if err != nil {
		panic(err)
	}

	// fmt.Println("start",startDate)
	// fmt.Println("end",endDate)

	query := `
				WITH RECURSIVE MonthList AS (
					SELECT ? AS month_date
					UNION ALL
					SELECT DATE_ADD(month_date, INTERVAL 1 MONTH)
					FROM MonthList
					WHERE DATE_ADD(month_date, INTERVAL 1 MONTH) <= ?
				),
				MonthlyAgg AS (
					SELECT
						DATE_FORMAT(transaction_date, '%Y-%m') AS month,
						SUM(total_income) AS income_amount,
						SUM(total_expense) AS expense_amount
					FROM daily_summaries
					WHERE
						transaction_date >= ?
						AND transaction_date <= ?
						AND business_id = ?
						AND branch_id = 0
						AND currency_id = ?
					GROUP BY DATE_FORMAT(transaction_date, '%Y-%m')
				)
				SELECT
					DATE_FORMAT(ml.month_date, '%Y-%m') AS month,
					COALESCE(ma.income_amount, 0) AS IncomeAmount,
					COALESCE(ma.expense_amount, 0) AS ExpenseAmount
				FROM
					MonthList ml
				LEFT JOIN
					MonthlyAgg ma ON DATE_FORMAT(ml.month_date, '%Y-%m') = ma.month
				ORDER BY
					ml.month_date;
                `

	rows, err := db.Raw(query,
		startDate, endDate,
		startDate, endDate, businessId, business.BaseCurrencyId).Rows()

	// Rollout safety: if daily_summaries isn't available yet, fall back to the legacy
	// calculation from account_currency_daily_balances so the dashboard keeps working.
	if err != nil && strings.Contains(strings.ToLower(err.Error()), "daily_summaries") &&
		(strings.Contains(strings.ToLower(err.Error()), "doesn't exist") || strings.Contains(strings.ToLower(err.Error()), "does not exist")) {
		legacyQuery := `
				WITH RECURSIVE MonthList AS (
					SELECT ? AS month_date
					UNION ALL
					SELECT DATE_ADD(month_date, INTERVAL 1 MONTH)
					FROM MonthList
					WHERE DATE_ADD(month_date, INTERVAL 1 MONTH) <= ?
				),
				MonthlyAgg AS (
					SELECT
						DATE_FORMAT(acb.transaction_date, '%Y-%m') AS month,
						COALESCE(SUM(CASE WHEN a.main_type = 'Income' THEN -acb.balance ELSE 0 END), 0) AS income_amount,
						COALESCE(SUM(CASE WHEN a.main_type = 'Expense' THEN  acb.balance ELSE 0 END), 0) AS expense_amount
					FROM account_currency_daily_balances acb
					JOIN accounts a ON a.id = acb.account_id
					WHERE
						acb.transaction_date >= ?
						AND acb.transaction_date <= ?
						AND acb.business_id = ?
						AND acb.branch_id = 0
						AND acb.currency_id = ?
						AND a.main_type IN ('Income', 'Expense')
					GROUP BY DATE_FORMAT(acb.transaction_date, '%Y-%m')
				)
				SELECT
					DATE_FORMAT(ml.month_date, '%Y-%m') AS month,
					COALESCE(ma.income_amount, 0) AS IncomeAmount,
					COALESCE(ma.expense_amount, 0) AS ExpenseAmount
				FROM
					MonthList ml
				LEFT JOIN
					MonthlyAgg ma ON DATE_FORMAT(ml.month_date, '%Y-%m') = ma.month
				ORDER BY
					ml.month_date;
                `
		rows, err = db.Raw(legacyQuery,
			startDate, endDate,
			startDate, endDate, businessId, business.BaseCurrencyId).Rows()
	}

	if err != nil {
		return nil, err
	}
	defer rows.Close()

	response := &IncomeExpenseReponse{
		TotalIncome:          decimal.NewFromInt(0),
		TotalExpense:         decimal.NewFromInt(0),
		IncomeExpenseDetails: []IncomeExpenseDetails{},
	}

	for rows.Next() {
		var monthStr string
		var incomeAmount, expenseAmount decimal.Decimal

		err := rows.Scan(&monthStr, &incomeAmount, &expenseAmount)
		if err != nil {
			return nil, err
		}

		// Parse month string to time.Time
		month, err := time.Parse("2006-01", monthStr)
		if err != nil {
			return nil, err
		}

		formattedMonth := month.Format("2006-Jan")

		detail := IncomeExpenseDetails{
			Month:         formattedMonth,
			IncomeAmount:  incomeAmount,
			ExpenseAmount: expenseAmount,
		}
		response.IncomeExpenseDetails = append(response.IncomeExpenseDetails, detail)
		response.TotalIncome = response.TotalIncome.Add(incomeAmount)
		response.TotalExpense = response.TotalExpense.Add(expenseAmount)

	}

	return response, nil
}

func GetTopExpense(ctx context.Context, filterType string) ([]*TopExpensesResponse, error) {
	db := config.GetDB()

	businessId, ok := utils.GetBusinessIdFromContext(ctx)
	if !ok || businessId == "" {
		return nil, errors.New("business id is required")
	}

	business, err := models.GetBusinessById(ctx, businessId)
	if err != nil {
		return nil, err
	}

	expense := string(models.AccountMainTypeExpense)

	fiscalYearStartMonth, err := utils.GetFiscalYearStartMonth(string(business.FiscalYear))
	if err != nil {
		return nil, err
	}

	startDate, endDate, err := utils.GetStartAndEndDateWithBusinessFiscalYear(fiscalYearStartMonth, filterType)
	if err != nil {
		return nil, err
	}

	// fmt.Println("start", startDate)
	// fmt.Println("end", endDate)

	var topExpenses []*TopExpensesResponse

	query := `
			WITH TopExpenses AS (
                SELECT 
                    ac.name AS account_name,
                    acb.account_id AS account_id,
                    acb.running_balance AS amount,
                    ROW_NUMBER() OVER (PARTITION BY acb.account_id ORDER BY acb.transaction_date DESC) AS row_num
                FROM 
                    account_currency_daily_balances AS acb
                JOIN
                    accounts AS ac ON acb.account_id = ac.id
                WHERE 
                    acb.transaction_date >= ?
                    AND acb.transaction_date <= ?
                    AND acb.business_id = ?
                    AND acb.branch_id = 0
                    AND acb.currency_id = ?
                    AND acb.account_id IN (
                        SELECT id FROM accounts WHERE main_type = ?
                    )
            )
            SELECT 
                account_name,
                amount
            FROM 
                TopExpenses
            WHERE 
                row_num = 1
            ORDER BY 
                amount DESC`

	if err := db.Raw(query,
		startDate, endDate, businessId, business.BaseCurrencyId, expense).
		Scan(&topExpenses).Error; err != nil {
		return nil, err
	}

	return topExpenses, nil
}

func GetTotalCashFlow(ctx context.Context, filterType string) (*CashFlowReponse, error) {
	db := config.GetDB()

	businessId, ok := utils.GetBusinessIdFromContext(ctx)
	if !ok || businessId == "" {
		return nil, errors.New("business id is required")
	}

	business, err := models.GetBusinessById(ctx, businessId)
	if err != nil {
		return nil, err
	}

	accountTypes := []string{
		string(models.AccountDetailTypeCash),
		string(models.AccountDetailTypeBank),
	}

	fiscalYearStartMonth, err := utils.GetFiscalYearStartMonth(string(business.FiscalYear))
	if err != nil {
		return nil, err
	}

	startDate, endDate, err := utils.GetStartAndEndDateWithBusinessFiscalYear(fiscalYearStartMonth, filterType)
	if err != nil {
		panic(err)
	}

	// fmt.Println("start",startDate)
	// fmt.Println("end",endDate)

	query := `
				WITH RECURSIVE MonthList AS (
					SELECT DATE(?) AS month_date

					UNION ALL

					SELECT DATE_ADD(month_date, INTERVAL 1 MONTH)

					FROM MonthList

					WHERE month_date < ?
				),
				BeginningCash AS (
					SELECT 
						ml.month_date AS month,
						COALESCE(SUM(acb.debit) - SUM(acb.credit), 0) AS beginning_balance

					FROM MonthList ml

					LEFT JOIN account_currency_daily_balances AS acb
						ON acb.transaction_date < ml.month_date
						AND acb.business_id = ?
						AND acb.branch_id = 0
						AND acb.currency_id = ?
						AND acb.account_id IN (
							SELECT id FROM accounts WHERE detail_type IN ?
						)
					GROUP BY ml.month_date
				),
				BankCash AS (
					SELECT 
						DATE_FORMAT(transaction_date, '%Y-%m') AS month,
						ac.detail_type AS detail_type,
						ac.main_type AS main_type,
						ac.id AS account_id,
						balance
					FROM account_currency_daily_balances AS acb
					JOIN accounts AS ac ON acb.account_id = ac.id
					WHERE 
						acb.transaction_date >= ?
						AND acb.transaction_date <= ?
						AND acb.business_id = ?
						AND acb.branch_id = 0
						AND acb.currency_id = ?
						AND acb.account_id IN (
							SELECT id FROM accounts WHERE detail_type IN ?
						)
				)
				SELECT
					COALESCE(bc.beginning_balance, 0) AS beginning_cash_balance,
					DATE_FORMAT(ml.month_date, '%Y-%m') AS month,
					COALESCE(SUM(CASE WHEN bank_cash.balance > 0 THEN bank_cash.balance ELSE 0 END), 0) AS incoming_amount,
					COALESCE(SUM(CASE WHEN bank_cash.balance < 0 THEN -bank_cash.balance ELSE 0 END), 0) AS outgoing_amount,
					(COALESCE(bc.beginning_balance, 0) + 
					COALESCE(SUM(CASE WHEN bank_cash.balance > 0 THEN bank_cash.balance ELSE 0 END), 0) - 
					COALESCE(SUM(CASE WHEN bank_cash.balance < 0 THEN -bank_cash.balance ELSE 0 END), 0)
					) AS ending_cash_balance
				FROM
					MonthList ml
				LEFT JOIN
					BankCash bank_cash ON DATE_FORMAT(ml.month_date, '%Y-%m') = bank_cash.month
				LEFT JOIN
					BeginningCash bc ON DATE_FORMAT(ml.month_date, '%Y-%m') = DATE_FORMAT(bc.month, '%Y-%m')
				GROUP BY
					ml.month_date, bc.beginning_balance
				ORDER BY
					ml.month_date;
                `

	rows, err := db.Raw(query,
		startDate, endDate, businessId, business.BaseCurrencyId, accountTypes,
		startDate, endDate, businessId, business.BaseCurrencyId, accountTypes,
	).Rows()

	if err != nil {
		return nil, err
	}
	defer rows.Close()

	response := &CashFlowReponse{
		TotalOpeningBalance: decimal.NewFromInt(0),
		TotalIncomingAmount: decimal.NewFromInt(0),
		TotalOutgoingAmount: decimal.NewFromInt(0),
		TotalEndingBalance:  decimal.NewFromInt(0),
		CashFlowDetails:     []CashFlowDetails{},
	}

	isFirstRow := true

	for rows.Next() {
		var monthStr string
		var beginningBalance, incomingAmount, outgoingAmount, endingBalance decimal.Decimal

		err := rows.Scan(&beginningBalance, &monthStr, &incomingAmount, &outgoingAmount, &endingBalance)
		if err != nil {
			return nil, err
		}

		// Set TotalOpeningBalance from the first row's opening balance
		if isFirstRow {
			response.TotalOpeningBalance = beginningBalance
			isFirstRow = false
		}

		// Parse month string to time.Time
		month, err := time.Parse("2006-01", monthStr)
		if err != nil {
			return nil, err
		}

		formattedMonth := month.Format("2006-Jan")

		detail := CashFlowDetails{
			Month:          formattedMonth,
			OpeningBalance: beginningBalance,
			IncomingAmount: incomingAmount,
			OutgoingAmount: outgoingAmount,
			EndingBalance:  endingBalance,
		}
		response.CashFlowDetails = append(response.CashFlowDetails, detail)
		response.TotalIncomingAmount = response.TotalIncomingAmount.Add(incomingAmount)
		response.TotalOutgoingAmount = response.TotalOutgoingAmount.Add(outgoingAmount)

		response.TotalEndingBalance = endingBalance
	}

	return response, nil
}
