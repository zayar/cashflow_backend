package reports

import (
	"context"

	"github.com/mmdatafocus/books_backend/config"
	"github.com/mmdatafocus/books_backend/models"
	"github.com/mmdatafocus/books_backend/utils"
	"github.com/shopspring/decimal"
)

type SupplierBalance struct {
	SupplierID         int             `json:"supplierId"`
	SupplierName       string          `json:"supplierName"`
	BillBalance        decimal.Decimal `json:"billBalance"`
	SupplierCredit     decimal.Decimal `json:"supplierCredit"`
	SupplierAdvance    decimal.Decimal `json:"supplierAdvance"`
	PaidAmount         decimal.Decimal `json:"paidAmount"`
	ClosingBalance     decimal.Decimal `json:"closingBalance"`
	BillBalanceFcy     decimal.Decimal `json:"billBalanceFcy"`
	SupplierCreditFcy  decimal.Decimal `json:"supplierCreditFcy"`
	SupplierAdvanceFcy decimal.Decimal `json:"supplierAdvanceFcy"`
	PaidAmountFcy      decimal.Decimal `json:"paidAmountFcy"`
	ClosingBalanceFcy  decimal.Decimal `json:"closingBalanceFcy"`
	CurrencySymbol     string
	DecimalPlaces      models.DecimalPlaces
}

func GetSupplierBalanceReport(ctx context.Context, toDate *models.MyDateString, branchId *int) ([]*SupplierBalance, error) {
	sql := `
WITH AvailableAdvance AS (
    select
        SUM(sca.remaining_balance * (case when currency_id = @baseCurrencyId then 1 else exchange_rate end)) as adjusted_amount,
        -- SUM(sca.remaining_balance) as amount,
        (case when currency_id = @baseCurrencyId then 0 else SUM(sca.remaining_balance) end) amount,
        sca.supplier_id,
        sca.currency_id
    from
        supplier_credit_advances sca
    where
        business_id = @businessId
        {{- if .branchId }} AND branch_id = @branchId {{- end }}
        AND NOT sca.current_status IN ('Draft')  AND sca.date <= @toDate
    group by
        sca.supplier_id,
        sca.currency_id
),
AvailableCredit AS (
    SELECT
        SUM(sc.remaining_balance * (case when currency_id = @baseCurrencyId then 1 else exchange_rate end)) as adjusted_amount,
        -- SUM(sc.remaining_balance) as amount,
        (case when currency_id = @baseCurrencyId then 0 else SUM(sc.remaining_balance) end) amount,
        sc.supplier_id,
        sc.currency_id
    from
        supplier_credits sc
    where
        business_id = @businessId
        {{- if .branchId }} AND branch_id = @branchId {{- end }}
        AND NOT sc.current_status IN ('Draft')  AND sc.supplier_credit_date <= @toDate
    group by
        sc.supplier_id,
        sc.currency_id
),
Bill AS (
    SELECT
        supplier_id,
        currency_id,
        -- SUM(bill_total_amount) total_bill_amount,
        (case when currency_id = @baseCurrencyId then 0 else SUM(bill_total_amount) end) total_bill_amount,
        SUM(bill_total_amount * (case when currency_id = @baseCurrencyId then 1 else exchange_rate end)) adjusted_total_bill_amount,
        -- SUM(b.bill_total_amount - b.remaining_balance) total_paid_amount,
        (case when currency_id = @baseCurrencyId then 0 else SUM(bill_total_amount - remaining_balance) end) total_paid_amount,
        SUM(
            (b.bill_total_amount - b.remaining_balance) * (case when currency_id = @baseCurrencyId then 1 else exchange_rate end) 
        ) adjusted_total_paid_amount
    from
        bills b
    where
        business_id = @businessId
        {{- if .branchId }} AND branch_id = @branchId {{- end }}
        AND NOT b.current_status IN ('Draft')  AND b.bill_date <= @toDate
    group by
        b.supplier_id,
        b.currency_id
)
SELECT
    Bill.supplier_id AS supplier_id,
    Bill.currency_id AS currency_id,
    suppliers.name as supplier_name,
    COALESCE(Bill.total_bill_amount, 0) AS bill_balance_fcy,
    COALESCE(Bill.adjusted_total_bill_amount, 0) AS bill_balance,
    COALESCE(AvailableCredit.amount, 0) AS supplier_credit_fcy,
    COALESCE(AvailableCredit.adjusted_amount, 0) AS supplier_credit,
    COALESCE(AvailableAdvance.amount, 0) AS supplier_advance_fcy,
    COALESCE(AvailableAdvance.adjusted_amount, 0) AS supplier_advance,
    COALESCE(Bill.total_paid_amount, 0) AS paid_amount_fcy,
    COALESCE(Bill.adjusted_total_paid_amount, 0) AS paid_amount,
    (
        COALESCE(Bill.total_bill_amount, 0) - COALESCE(AvailableCredit.amount, 0) - COALESCE(AvailableAdvance.amount, 0) - COALESCE(Bill.total_paid_amount, 0)
    ) AS closing_balance_fcy,
    (
        COALESCE(Bill.adjusted_total_bill_amount, 0) - COALESCE(AvailableCredit.adjusted_amount, 0) - COALESCE(AvailableAdvance.adjusted_amount, 0) - COALESCE(Bill.adjusted_total_paid_amount, 0)
    ) AS closing_balance,
    currencies.symbol currency_symbol,
    currencies.decimal_places
from
    Bill
LEFT JOIN AvailableCredit ON AvailableCredit.supplier_id = Bill.supplier_id
    AND AvailableCredit.currency_id = Bill.currency_id
LEFT JOIN AvailableAdvance ON AvailableAdvance.supplier_id = Bill.supplier_id
    AND AvailableAdvance.currency_id = Bill.currency_id
LEFT JOIN suppliers ON suppliers.id = Bill.supplier_id
LEFT JOIN currencies ON currencies.id = Bill.currency_id

UNION ALL

SELECT
    AvailableCredit.supplier_id AS supplier_id,
    AvailableCredit.currency_id AS currency_id,
    suppliers.name as supplier_name,
    COALESCE(Bill.total_bill_amount, 0) AS bill_balance_fcy,
    COALESCE(Bill.adjusted_total_bill_amount, 0) AS bill_balance,
    COALESCE(AvailableCredit.amount, 0) AS supplier_credit_fcy,
    COALESCE(AvailableCredit.adjusted_amount, 0) AS supplier_credit,
    COALESCE(AvailableAdvance.amount, 0) AS supplier_advance_fcy,
    COALESCE(AvailableAdvance.adjusted_amount, 0) AS supplier_advance,
    COALESCE(Bill.total_paid_amount, 0) AS paid_amount_fcy,
    COALESCE(Bill.adjusted_total_paid_amount, 0) AS paid_amount,
    (
        COALESCE(Bill.total_bill_amount, 0) - COALESCE(AvailableCredit.amount, 0) - COALESCE(AvailableAdvance.amount, 0) - COALESCE(Bill.total_paid_amount, 0)
    ) AS closing_balance_fcy,
    (
        COALESCE(Bill.adjusted_total_bill_amount, 0) - COALESCE(AvailableCredit.adjusted_amount, 0) - COALESCE(AvailableAdvance.adjusted_amount, 0) - COALESCE(Bill.adjusted_total_paid_amount, 0)
    ) AS closing_balance,
    currencies.symbol currency_symbol,
    currencies.decimal_places
from
    AvailableCredit
LEFT JOIN Bill ON AvailableCredit.supplier_id = Bill.supplier_id
    AND AvailableCredit.currency_id = Bill.currency_id
LEFT JOIN AvailableAdvance ON AvailableAdvance.supplier_id = AvailableCredit.supplier_id
    AND AvailableAdvance.currency_id = AvailableCredit.currency_id
LEFT JOIN suppliers ON suppliers.id = AvailableCredit.supplier_id
LEFT JOIN currencies ON currencies.id = AvailableCredit.currency_id
WHERE Bill.supplier_id IS NULL 

UNION ALL

SELECT
    AvailableAdvance.supplier_id AS supplier_id,
    AvailableAdvance.currency_id AS currency_id,
    suppliers.name as supplier_name,
    COALESCE(Bill.total_bill_amount, 0) AS bill_balance_fcy,
    COALESCE(Bill.adjusted_total_bill_amount, 0) AS bill_balance,
    COALESCE(AvailableCredit.amount, 0) AS supplier_credit_fcy,
    COALESCE(AvailableCredit.adjusted_amount, 0) AS supplier_credit,
    COALESCE(AvailableAdvance.amount, 0) AS supplier_advance_fcy,
    COALESCE(AvailableAdvance.adjusted_amount, 0) AS supplier_advance,
    COALESCE(Bill.total_paid_amount, 0) AS paid_amount_fcy,
    COALESCE(Bill.adjusted_total_paid_amount, 0) AS paid_amount,
    (
        COALESCE(Bill.total_bill_amount, 0) - COALESCE(AvailableCredit.amount, 0) - COALESCE(AvailableAdvance.amount, 0) - COALESCE(Bill.total_paid_amount, 0)
    ) AS closing_balance_fcy,
    (
        COALESCE(Bill.adjusted_total_bill_amount, 0) - COALESCE(AvailableCredit.adjusted_amount, 0) - COALESCE(AvailableAdvance.adjusted_amount, 0) - COALESCE(Bill.adjusted_total_paid_amount, 0)
    ) AS closing_balance,
    currencies.symbol currency_symbol,
    currencies.decimal_places
from
    AvailableAdvance
LEFT JOIN Bill ON AvailableAdvance.supplier_id = Bill.supplier_id
    AND AvailableAdvance.currency_id = Bill.currency_id
LEFT JOIN AvailableCredit ON AvailableAdvance.supplier_id = AvailableCredit.supplier_id
    AND AvailableAdvance.currency_id = AvailableCredit.currency_id
LEFT JOIN suppliers ON suppliers.id = AvailableAdvance.supplier_id
LEFT JOIN currencies ON currencies.id = AvailableAdvance.currency_id
WHERE Bill.supplier_id IS NULL AND AvailableCredit.supplier_id IS NULL

    ORDER by supplier_name, currency_id
`

	business, err := models.GetBusiness(ctx)
	if err != nil {
		return nil, err
	}
	date := toDate.SetDefaultNowIfNil()
	if err := date.EndOfDayUTCTime(business.Timezone); err != nil {
		return nil, err
	}

	db := config.GetDB()
	sql, err = utils.ExecTemplate(sql, map[string]interface{}{
		"branchId": utils.DereferencePtr(branchId, 0),
	})
	if err != nil {
		return nil, err
	}
	var results []*SupplierBalance
	if err := db.WithContext(ctx).Raw(sql, map[string]interface{}{
		"baseCurrencyId": business.BaseCurrencyId,
		"businessId":     business.ID,
		"branchId":       branchId,
		"toDate":         date,
	}).Scan(&results).Error; err != nil {
		return nil, err
	}
	// // calculate closing balance
	// for _, result := range results {
	// 	result.ClosingBalance = result.BillBalance.Sub(result.SupplierCredit).
	// 		Sub(result.SupplierAdvance).Sub(result.PaidAmount)
	// }
	return results, nil
}

func GetSupplierBalanceSummaryReport(ctx context.Context, toDate *models.MyDateString, branchId *int) ([]*SupplierBalance, error) {

	sqlT := `
WITH AccTransactionSummary AS
(
SELECT
    aj.supplier_id AS supplier_id,
    at.foreign_currency_id AS currency_id,
    SUM(
        CASE
            WHEN reference_type = 'BL' OR reference_type = 'SOB' THEN base_credit
            ELSE 0
        END
    ) AS total_bill_bcy,
    SUM(
        CASE
            WHEN (reference_type = 'BL' OR reference_type = 'SOB')
            AND foreign_currency_id != 0
            AND foreign_currency_id != @baseCurrencyId THEN foreign_credit
            ELSE 0
        END
    ) AS total_bill_fcy,
    SUM(
        CASE
            WHEN reference_type = 'SC' THEN base_debit
            WHEN reference_type = 'SCR' THEN base_credit * -1
            ELSE 0
        END
    ) AS total_credit_bcy,
    SUM(
        CASE
            WHEN foreign_currency_id != 0
            AND foreign_currency_id != @baseCurrencyId THEN
                CASE
                    WHEN reference_type = 'SC' THEN foreign_debit
                    WHEN reference_type = 'SCR' THEN foreign_credit * -1
                    ELSE 0
                END
            ELSE 0
        END
    ) AS total_credit_fcy,
    SUM(
        CASE
            WHEN reference_type = 'SP' OR reference_type = 'SAA' THEN base_debit
            ELSE 0
        END
    ) AS total_paid_bcy,
    SUM(
        CASE
            WHEN (reference_type = 'SP' OR reference_type = 'SAA')
            AND foreign_currency_id != 0
            AND foreign_currency_id != @baseCurrencyId THEN foreign_debit
            ELSE 0
        END
    ) AS total_paid_fcy
FROM
    account_journals AS aj
    JOIN account_transactions AS at on aj.id = at.journal_id
WHERE
    -- tenant isolation + ignore reversal chains (count only active journals)
    aj.business_id = @businessId
    AND at.business_id = @businessId
    AND aj.is_reversal = 0
    AND aj.reversed_by_journal_id IS NULL
    at.account_id = @payableAccId
    {{- if .toDate }} AND aj.transaction_date_time <= @transactionDate {{- end }}
    {{- if .branchId }} AND aj.branch_id = @branchId {{- end }}
GROUP by
    aj.supplier_id,
    at.foreign_currency_id
)
SELECT
    ATS.supplier_id,
    suppliers.name supplier_name,
    currencies.symbol currency_symbol,
    currencies.decimal_places,
    ATS.total_bill_bcy bill_balance,
    ATS.total_bill_fcy bill_balance_fcy,
    ATS.total_credit_bcy supplier_credit,
    ATS.total_credit_fcy supplier_credit_fcy,
    ATS.total_paid_bcy paid_amount,
    ATS.total_paid_fcy paid_amount_fcy,
    (ATS.total_bill_bcy - ATS.total_credit_bcy - ATS.total_paid_bcy) closing_balance,
    (ATS.total_bill_fcy - ATS.total_credit_fcy - ATS.total_paid_fcy) closing_balance_fcy


    FROM AccTransactionSummary ATS
    LEFT JOIN suppliers ON suppliers.id = ATS.supplier_id
    LEFT JOIN currencies ON currencies.id = ATS.currency_id
ORDER BY suppliers.name, ATS.currency_id
`
	business, err := models.GetBusiness(ctx)
	if err != nil {
		return nil, err
	}

	transactionDate := toDate.SetDefaultNowIfNil()

	if err := transactionDate.EndOfDayUTCTime(business.Timezone); err != nil {
		return nil, err
	}
	var results []*SupplierBalance
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
		"payableAccId":    accs[models.AccountCodeAccountsPayable],
	}).Scan(&results).Error; err != nil {
		return nil, err
	}
	return results, nil
}
