package reports

import (
	"context"

	"github.com/mmdatafocus/books_backend/config"
	"github.com/mmdatafocus/books_backend/models"
	"github.com/mmdatafocus/books_backend/utils"
	"github.com/shopspring/decimal"
)

type CustomerBalance struct {
	CustomerID         int             `json:"customerId"`
	CustomerName       string          `json:"customerName"`
	InvoiceBalance     decimal.Decimal `json:"invoiceBalance"`
	CustomerCredit     decimal.Decimal `json:"customerCredit"`
	CustomerAdvance    decimal.Decimal `json:"customerAdvance"`
	ReceivedAmount     decimal.Decimal `json:"receivedAmount"`
	ClosingBalance     decimal.Decimal `json:"closingBalance"`
	InvoiceBalanceFcy  decimal.Decimal `json:"invoiceBalanceFcy"`
	CustomerCreditFcy  decimal.Decimal `json:"customerCreditFcy"`
	CustomerAdvanceFcy decimal.Decimal `json:"customerAdvanceFcy"`
	ReceivedAmountFcy  decimal.Decimal `json:"receivedAmountFcy"`
	ClosingBalanceFcy  decimal.Decimal `json:"closingBalanceFcy"`
	CurrencySymbol     string
	DecimalPlaces      models.DecimalPlaces
}

func GetCustomerBalanceReport(ctx context.Context, toDate *models.MyDateString, branchId *int) ([]*CustomerBalance, error) {
	sql := `
WITH AvailableAdvance AS (
    select
    (case when currency_id = @baseCurrencyId then 0 else SUM(cca.remaining_balance) end) amount,
    SUM(cca.remaining_balance * (case when currency_id = @baseCurrencyId then 1 else exchange_rate end)) adjusted_amount,
        cca.customer_id,
        cca.currency_id
    from
        customer_credit_advances cca
    where
        business_id = @businessId
        {{- if .branchId }} AND branch_id = @branchId {{- end }}
        AND NOT cca.current_status IN ('Draft')  AND cca.date <= @toDate
    group by
        cca.customer_id,
        cca.currency_id
),
AvailableCredit AS (
    SELECT
        (case when currency_id = @baseCurrencyId then 0 else SUM(cn.remaining_balance) end) amount,
        SUM(cn.remaining_balance * (case when currency_id = @baseCurrencyId then 1 else exchange_rate end)) adjusted_amount,
        cn.customer_id,
        cn.currency_id
    from
        credit_notes cn
    where
        business_id = @businessId
        {{- if .branchId }} AND branch_id = @branchId {{- end }}
        AND NOT cn.current_status IN ('Draft') AND credit_note_date <= @toDate
    group by
        cn.customer_id,
        cn.currency_id
),
SalesInvoice AS (
    SELECT
        customer_id,
        (case when currency_id = @baseCurrencyId then 0 else SUM(iv.invoice_total_amount) end) total_invoice_amount,
        SUM(iv.invoice_total_amount * (case when currency_id = @baseCurrencyId then 1 else exchange_rate end)) adjusted_total_invoice_amount,
        (case when currency_id = @baseCurrencyId then 0 else SUM(iv.invoice_total_amount - iv.remaining_balance) end) total_paid_amount,
        SUM((iv.invoice_total_amount - iv.remaining_balance) * (case when currency_id = @baseCurrencyId then 1 else exchange_rate end)) adjusted_total_paid_amount,
        currency_id
    from
        sales_invoices iv
    where
        business_id = @businessId
        {{- if .branchId }} AND branch_id = @branchId {{- end }}
        AND NOT iv.current_status IN ('Draft')  AND iv.invoice_date <= @toDate
    group by
        iv.customer_id,
        iv.currency_id
)
SELECT
    SalesInvoice.customer_id AS customer_id,
    SalesInvoice.currency_id AS currency_id,
	customers.name as customer_name,
    COALESCE(SalesInvoice.total_invoice_amount, 0) AS invoice_balance_fcy,
    COALESCE(AvailableCredit.amount, 0) AS customer_credit_fcy,
    COALESCE(AvailableAdvance.amount, 0) AS customer_advance_fcy,
    COALESCE(SalesInvoice.total_paid_amount, 0) AS received_amount_fcy,
    COALESCE(SalesInvoice.adjusted_total_invoice_amount, 0) AS invoice_balance,
    COALESCE(AvailableCredit.adjusted_amount, 0) AS customer_credit,
    COALESCE(AvailableAdvance.adjusted_amount, 0) AS customer_advance,
    COALESCE(SalesInvoice.adjusted_total_paid_amount, 0) AS received_amount,
    (
        COALESCE(SalesInvoice.total_invoice_amount, 0) - COALESCE(AvailableCredit.amount, 0) - COALESCE(AvailableAdvance.amount, 0)
        - COALESCE(SalesInvoice.total_paid_amount, 0)
    ) closing_balance_fcy,
    (
       COALESCE(SalesInvoice.adjusted_total_invoice_amount, 0) - COALESCE(AvailableCredit.adjusted_amount, 0) - COALESCE(AvailableAdvance.adjusted_amount, 0)
        - COALESCE(SalesInvoice.adjusted_total_paid_amount, 0)
    ) closing_balance,
    currencies.symbol currency_symbol,
    currencies.decimal_places
from
    SalesInvoice
LEFT JOIN AvailableCredit ON AvailableCredit.customer_id = SalesInvoice.customer_id
        AND AvailableCredit.currency_id = SalesInvoice.currency_id
LEFT JOIN AvailableAdvance ON AvailableAdvance.customer_id = SalesInvoice.customer_id
        AND AvailableAdvance.currency_id = SalesInvoice.currency_id
LEFT JOIN customers ON customers.id = SalesInvoice.customer_id
LEFT JOIN currencies ON currencies.id = SalesInvoice.currency_id

UNION ALL

SELECT
    AvailableCredit.customer_id AS customer_id,
    AvailableCredit.currency_id AS currency_id,
	customers.name as customer_name,
    COALESCE(SalesInvoice.total_invoice_amount, 0) AS invoice_balance_fcy,
    COALESCE(AvailableCredit.amount, 0) AS customer_credit_fcy,
    COALESCE(AvailableAdvance.amount, 0) AS customer_advance_fcy,
    COALESCE(SalesInvoice.total_paid_amount, 0) AS received_amount_fcy,
    COALESCE(SalesInvoice.adjusted_total_invoice_amount, 0) AS invoice_balance,
    COALESCE(AvailableCredit.adjusted_amount, 0) AS customer_credit,
    COALESCE(AvailableAdvance.adjusted_amount, 0) AS customer_advance,
    COALESCE(SalesInvoice.adjusted_total_paid_amount, 0) AS received_amount,
    (
        COALESCE(SalesInvoice.total_invoice_amount, 0) - COALESCE(AvailableCredit.amount, 0) - COALESCE(AvailableAdvance.amount, 0)
        - COALESCE(SalesInvoice.total_paid_amount, 0)
    ) closing_balance_fcy,
    (
       COALESCE(SalesInvoice.adjusted_total_invoice_amount, 0) - COALESCE(AvailableCredit.adjusted_amount, 0) - COALESCE(AvailableAdvance.adjusted_amount, 0)
        - COALESCE(SalesInvoice.adjusted_total_paid_amount, 0)
    ) closing_balance,
    currencies.symbol currency_symbol,
    currencies.decimal_places
from
    AvailableCredit
LEFT JOIN SalesInvoice ON AvailableCredit.customer_id = SalesInvoice.customer_id
        AND AvailableCredit.currency_id = SalesInvoice.currency_id
LEFT JOIN AvailableAdvance ON AvailableAdvance.customer_id = AvailableCredit.customer_id
        AND AvailableAdvance.currency_id = AvailableCredit.currency_id
LEFT JOIN customers ON customers.id = AvailableCredit.customer_id
LEFT JOIN currencies ON currencies.id = AvailableCredit.currency_id
WHERE SalesInvoice.customer_id IS NULL

UNION ALL

SELECT
    AvailableAdvance.customer_id AS customer_id,
    AvailableAdvance.currency_id AS currency_id,
	customers.name as customer_name,
    COALESCE(SalesInvoice.total_invoice_amount, 0) AS invoice_balance_fcy,
    COALESCE(AvailableCredit.amount, 0) AS customer_credit_fcy,
    COALESCE(AvailableAdvance.amount, 0) AS customer_advance_fcy,
    COALESCE(SalesInvoice.total_paid_amount, 0) AS received_amount_fcy,
    COALESCE(SalesInvoice.adjusted_total_invoice_amount, 0) AS invoice_balance,
    COALESCE(AvailableCredit.adjusted_amount, 0) AS customer_credit,
    COALESCE(AvailableAdvance.adjusted_amount, 0) AS customer_advance,
    COALESCE(SalesInvoice.adjusted_total_paid_amount, 0) AS received_amount,
    (
        COALESCE(SalesInvoice.total_invoice_amount, 0) - COALESCE(AvailableCredit.amount, 0) - COALESCE(AvailableAdvance.amount, 0)
        - COALESCE(SalesInvoice.total_paid_amount, 0)
    ) closing_balance_fcy,
    (
       COALESCE(SalesInvoice.adjusted_total_invoice_amount, 0) - COALESCE(AvailableCredit.adjusted_amount, 0) - COALESCE(AvailableAdvance.adjusted_amount, 0)
        - COALESCE(SalesInvoice.adjusted_total_paid_amount, 0)
    ) closing_balance,
    currencies.symbol currency_symbol,
    currencies.decimal_places
from
    AvailableAdvance
LEFT JOIN SalesInvoice ON AvailableAdvance.customer_id = SalesInvoice.customer_id
        AND AvailableAdvance.currency_id = SalesInvoice.currency_id
LEFT JOIN AvailableCredit ON AvailableAdvance.customer_id = AvailableCredit.customer_id
        AND AvailableAdvance.currency_id = AvailableCredit.currency_id
LEFT JOIN customers ON customers.id = AvailableAdvance.customer_id
LEFT JOIN currencies ON currencies.id = AvailableAdvance.currency_id
WHERE SalesInvoice.customer_id IS NULL AND AvailableCredit.customer_id IS NULL
ORDER by customer_name, currency_id
	`
	business, err := models.GetBusiness(ctx)
	if err != nil {
		return nil, err
	}

	date := toDate.SetDefaultNowIfNil()

	if err := date.EndOfDayUTCTime(business.Timezone); err != nil {
		return nil, err
	}
	var results []*CustomerBalance
	db := config.GetDB()
	sql, err = utils.ExecTemplate(sql, map[string]interface{}{
		"branchId": utils.DereferencePtr(branchId, 0),
	})
	if err != nil {
		return nil, err
	}
	if err := db.WithContext(ctx).Raw(sql, map[string]interface{}{
		"businessId":     business.ID,
		"baseCurrencyId": business.BaseCurrencyId,
		"toDate":         date,
		"branchId":       branchId,
	}).Scan(&results).Error; err != nil {
		return nil, err
	}
	// // calculate closing balance
	// for _, result := range results {
	// 	result.ClosingBalance = result.InvoiceBalance.Sub(result.CustomerCredit).
	// 		Sub(result.CustomerAdvance).Sub(result.ReceivedAmount)
	// }
	return results, nil
}

func GetCustomerBalanceSummaryReport(ctx context.Context, toDate *models.MyDateString, branchId *int) ([]*CustomerBalance, error) {

	sqlT := `
WITH AccTransactionSummary AS
(
    SELECT
        aj.customer_id AS customer_id,
        CASE
            WHEN at.foreign_currency_id != 0
            AND at.foreign_currency_id != @baseCurrencyId THEN at.foreign_currency_id
            ELSE @baseCurrencyId
        END AS currency_id,
        SUM(
            CASE
                WHEN reference_type = 'IV' OR reference_type = 'COB' THEN base_debit
                ELSE 0
            END
        ) AS total_invoice_bcy,
        SUM(
            CASE
                WHEN (reference_type = 'IV' OR reference_type = 'COB')
                AND foreign_currency_id != 0
                AND foreign_currency_id != @baseCurrencyId THEN foreign_debit
                ELSE 0
            END
        ) AS total_invoice_fcy,
        SUM(
            CASE
                WHEN reference_type = 'CP'
                OR reference_type = 'IWO' 
                OR reference_type = 'CAA' 
                THEN base_credit
                ELSE 0
            END
        ) AS total_received_bcy,
        SUM(
            CASE
                WHEN (
                    reference_type = 'CP'
                    OR reference_type = 'IWO'
                    OR reference_type = 'CAA'
                )
                AND foreign_currency_id != 0
                AND foreign_currency_id != @baseCurrencyId THEN foreign_credit
                ELSE 0
            END
        ) AS total_received_fcy
    FROM
        account_journals AS aj
        JOIN account_transactions AS at ON aj.id = at.journal_id
    WHERE
        -- tenant isolation + ignore reversal chains (count only active journals)
        aj.business_id = @businessId
        AND at.business_id = @businessId
        AND aj.is_reversal = 0
        AND aj.reversed_by_journal_id IS NULL
        AND at.account_id = @receivableAccId
        {{- if .toDate }} AND aj.transaction_date_time <= @transactionDate {{- end }}
        {{- if .branchId }} AND aj.branch_id = @branchId {{- end }}
    GROUP BY
        aj.customer_id,
        currency_id
),
AvailableCredit AS (
    SELECT
        (case when currency_id = @baseCurrencyId then 0 else SUM(cn.remaining_balance) end) amount,
        SUM(cn.remaining_balance * (case when currency_id = @baseCurrencyId then 1 else exchange_rate end)) adjusted_amount,
        cn.customer_id,
        cn.currency_id
    from
        credit_notes cn
    where
        cn.business_id = @businessId
        {{- if .branchId }} AND cn.branch_id = @branchId {{- end }}
        AND NOT cn.current_status IN ('Draft')
        {{- if .toDate }} AND cn.credit_note_date <= @transactionDate {{- end }}
    group by
        cn.customer_id,
        cn.currency_id
)
SELECT
    ATS.customer_id,
    customers.name customer_name,
    currencies.symbol currency_symbol,
    currencies.decimal_places,
    ATS.total_invoice_bcy invoice_balance,
    ATS.total_invoice_fcy invoice_balance_fcy,
    COALESCE(AC.adjusted_amount, 0) customer_credit,
    COALESCE(AC.amount, 0) customer_credit_fcy,
    ATS.total_received_bcy received_amount,
    ATS.total_received_fcy received_amount_fcy,
    (ATS.total_invoice_bcy - COALESCE(AC.adjusted_amount, 0) - ATS.total_received_bcy) closing_balance,
    (ATS.total_invoice_fcy - COALESCE(AC.amount, 0) - ATS.total_received_fcy) closing_balance_fcy


    FROM AccTransactionSummary ATS
    LEFT JOIN AvailableCredit AC ON AC.customer_id = ATS.customer_id AND AC.currency_id = ATS.currency_id
    LEFT JOIN customers ON customers.id = ATS.customer_id
    LEFT JOIN currencies ON currencies.id = ATS.currency_id

UNION ALL

SELECT
    AC.customer_id,
    customers.name customer_name,
    currencies.symbol currency_symbol,
    currencies.decimal_places,
    COALESCE(ATS.total_invoice_bcy, 0) invoice_balance,
    COALESCE(ATS.total_invoice_fcy, 0) invoice_balance_fcy,
    COALESCE(AC.adjusted_amount, 0) customer_credit,
    COALESCE(AC.amount, 0) customer_credit_fcy,
    COALESCE(ATS.total_received_bcy, 0) received_amount,
    COALESCE(ATS.total_received_fcy, 0) received_amount_fcy,
    (COALESCE(ATS.total_invoice_bcy, 0) - COALESCE(AC.adjusted_amount, 0) - COALESCE(ATS.total_received_bcy, 0)) closing_balance,
    (COALESCE(ATS.total_invoice_fcy, 0) - COALESCE(AC.amount, 0) - COALESCE(ATS.total_received_fcy, 0)) closing_balance_fcy
FROM
    AvailableCredit AC
    LEFT JOIN AccTransactionSummary ATS ON ATS.customer_id = AC.customer_id AND ATS.currency_id = AC.currency_id
    LEFT JOIN customers ON customers.id = AC.customer_id
    LEFT JOIN currencies ON currencies.id = AC.currency_id
WHERE ATS.customer_id IS NULL
ORDER BY customer_name, currency_symbol
`
	business, err := models.GetBusiness(ctx)
	if err != nil {
		return nil, err
	}

	transactionDate := toDate.SetDefaultNowIfNil()

	if err := transactionDate.EndOfDayUTCTime(business.Timezone); err != nil {
		return nil, err
	}
	var results []*CustomerBalance
	db := config.GetDB()
	sql, err := utils.ExecTemplate(sqlT, map[string]interface{}{
		"toDate":   toDate != nil,
		"branchId": utils.DereferencePtr(branchId, 0) > 0,
	})
	if err != nil {
		return nil, err
	}

	accs, err := models.GetSystemAccounts(business.ID.String())
	if err != nil {
		return nil, err
	}
	if err := db.WithContext(ctx).Raw(sql, map[string]interface{}{
		"businessId":      business.ID,
		"baseCurrencyId":  business.BaseCurrencyId,
		"transactionDate": transactionDate,
		"branchId":        branchId,
		"receivableAccId": accs[models.AccountCodeAccountsReceivable],
	}).Scan(&results).Error; err != nil {
		return nil, err
	}
	return results, nil
}

// func GetCustomerBalanceSummaryReport(ctx context.Context, toDate *models.MyDateString, branchId *int) ([]*CustomerBalance, error) {

// 	sqlT := `
// WITH AccTransactionSummary AS (
//     SELECT
//         aj.customer_id AS customer_id,
//         at.base_currency_id AS base_currency_id,
//         CASE
//             WHEN at.foreign_currency_id != at.base_currency_id THEN at.foreign_currency_id
//             ELSE NULL
//         END AS foreign_currency_id,
//         SUM(
//             CASE
//                 WHEN reference_type = 'IV' OR reference_type = 'COB' THEN base_debit
//                 ELSE 0
//             END
//         ) OVER (PARTITION BY aj.customer_id) AS total_invoice_bcy,
//         SUM(
//             CASE
//                 WHEN (reference_type = 'IV' OR reference_type = 'COB')
//                 AND at.foreign_currency_id != 0
//                 AND at.foreign_currency_id != at.base_currency_id THEN foreign_debit
//                 ELSE 0
//             END
//         ) OVER (PARTITION BY aj.customer_id) AS total_invoice_fcy,
//         SUM(
//             CASE
//                 WHEN reference_type = 'CN' THEN base_credit
//                 ELSE 0
//             END
//         ) OVER (PARTITION BY aj.customer_id) AS total_credit_bcy,
//         SUM(
//             CASE
//                 WHEN reference_type = 'CN'
//                 AND at.foreign_currency_id != 0
//                 AND at.foreign_currency_id != at.base_currency_id THEN foreign_credit
//                 ELSE 0
//             END
//         ) OVER (PARTITION BY aj.customer_id) AS total_credit_fcy,
//         SUM(
//             CASE
//                 WHEN reference_type IN ('CP', 'IWO', 'CAA') THEN base_credit
//                 ELSE 0
//             END
//         ) OVER (PARTITION BY aj.customer_id) AS total_received_bcy,
//         SUM(
//             CASE
//                 WHEN reference_type IN ('CP', 'IWO', 'CAA')
//                 AND at.foreign_currency_id != 0
//                 AND at.foreign_currency_id != at.base_currency_id THEN foreign_credit
//                 ELSE 0
//             END
//         ) OVER (PARTITION BY aj.customer_id) AS total_received_fcy,
//         ROW_NUMBER() OVER (PARTITION BY aj.customer_id ORDER BY aj.customer_id) AS rn
//     FROM
//         account_journals AS aj
//         JOIN account_transactions AS at ON aj.id = at.journal_id
//     WHERE
//         at.account_id = @receivableAccId
//         {{- if .toDate }} AND aj.transaction_date_time <= @transactionDate {{- end }}
//         {{- if .branchId }} AND aj.branch_id = @branchId {{- end }}
// )
// SELECT
//     ATS.customer_id,
//     customers.name AS customer_name,
//     MAX(base_currency.symbol) AS base_currency_symbol,
//     MAX(base_currency.decimal_places) AS base_currency_decimal_places,
//     MAX(foreign_currency.symbol) AS foreign_currency_symbol,
//     MAX(foreign_currency.decimal_places) AS foreign_currency_decimal_places,
//     MAX(ATS.total_invoice_bcy) AS invoice_balance,
//     MAX(ATS.total_invoice_fcy) AS invoice_balance_fcy,
//     MAX(ATS.total_credit_bcy) AS customer_credit,
//     MAX(ATS.total_credit_fcy) AS customer_credit_fcy,
//     MAX(ATS.total_received_bcy) AS received_amount,
//     MAX(ATS.total_received_fcy) AS received_amount_fcy,
//     MAX(ATS.total_invoice_bcy - ATS.total_credit_bcy - ATS.total_received_bcy) AS closing_balance,
//     MAX(ATS.total_invoice_fcy - ATS.total_credit_fcy - ATS.total_received_fcy) AS closing_balance_fcy
// FROM
//     AccTransactionSummary ATS
//     LEFT JOIN customers ON customers.id = ATS.customer_id
//     LEFT JOIN currencies AS base_currency ON base_currency.id = ATS.base_currency_id
//     LEFT JOIN currencies AS foreign_currency ON foreign_currency.id = ATS.foreign_currency_id
// GROUP BY
//     ATS.customer_id,
//     customers.name
// ORDER BY
//     customers.name;
// `
// 	business, err := models.GetBusiness(ctx)
// 	if err != nil {
// 		return nil, err
// 	}

// 	transactionDate := toDate.SetDefaultNowIfNil()

// 	if err := transactionDate.EndOfDayUTCTime(business.Timezone); err != nil {
// 		return nil, err
// 	}
// 	var results []*CustomerBalance
// 	db := config.GetDB()
// 	sql, err := utils.ExecTemplate(sqlT, map[string]interface{}{
// 		"toDate":   toDate != nil,
// 		"branchId": utils.DereferencePtr(branchId, 0) > 0,
// 	})
// 	if err != nil {
// 		return nil, err
// 	}

// 	accs, err := models.GetSystemAccounts(business.ID.String())
// 	if err != nil {
// 		return nil, err
// 	}
// 	if err := db.WithContext(ctx).Raw(sql, map[string]interface{}{
// 		// "baseCurrencyId":  business.BaseCurrencyId,
// 		"transactionDate": transactionDate,
// 		"branchId":        branchId,
// 		"receivableAccId": accs[models.AccountCodeAccountsReceivable],
// 	}).Scan(&results).Error; err != nil {
// 		return nil, err
// 	}
// 	return results, nil
// }
