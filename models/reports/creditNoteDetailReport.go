package reports

import (
	"context"
	"time"

	"bitbucket.org/mmdatafocus/books_backend/config"
	"bitbucket.org/mmdatafocus/books_backend/models"
	"bitbucket.org/mmdatafocus/books_backend/utils"
	"github.com/shopspring/decimal"
)

type CreditNoteDetailResponse struct {
	CreditNoteDate       time.Time       `json:"creditNoteDate"`
	CreditNoteStatus     string          `json:"creditNoteStatus"`
	CustomerID           int             `json:"customerId"`
	CustomerName         *string         `json:"customerName,omitempty"`
	CurrencyId           int             `json:"currencyId"`
	CreditNoteNumber     string          `json:"creditNoteNumber"`
	CreditNoteAmountFcy  decimal.Decimal `json:"creditNoteAmountFcy"`
	CreditNoteBalanceFcy decimal.Decimal `json:"creditNoteBalanceFcy"`
	CreditNoteAmount     decimal.Decimal `json:"creditNoteAmount"`
	CreditNoteBalance    decimal.Decimal `json:"creditNoteBalance"`
}

func GetCreditNoteDetailsReport(ctx context.Context, fromDate models.MyDateString, toDate models.MyDateString, branchID *int, warehouseID *int) ([]*CreditNoteDetailResponse, error) {

	sqlT := `
SELECT
	cn.credit_note_date,
	cn.current_status credit_note_status,
    cn.credit_note_number,
	(
		CASE
			WHEN cn.currency_id <> @baseCurrencyId THEN cn.credit_note_total_amount
			ELSE 0
		END
	) AS credit_note_amount_fcy,
	(
		CASE
			WHEN cn.currency_id <> @baseCurrencyId THEN cn.remaining_balance
			ELSE 0
		END
	) AS credit_note_balance_fcy,
    (
        CASE
            WHEN cn.currency_id <> @baseCurrencyId THEN cn.credit_note_total_amount * cn.exchange_rate
            ELSE cn.credit_note_total_amount
        END
    ) credit_note_amount,
    (
        CASE
            WHEN cn.currency_id <> @baseCurrencyId THEN cn.remaining_balance * cn.exchange_rate
            ELSE cn.remaining_balance
        END
    ) credit_note_balance,
    cn.currency_id,
    cn.customer_id,
    customers.name customer_name
FROM
    credit_notes cn
    LEFT JOIN customers ON customers.id = cn.customer_id
	WHERE cn.business_id = @businessId
	AND cn.credit_note_date BETWEEN @fromDate AND @toDate
	{{- if .branchId }} AND cn.branch_id = @branchId {{- end }}
	{{- if .warehouseId }} AND cn.warehouse_id = @warehouseId {{- end }}
ORDER BY cn.credit_note_date;
	`
	var results []*CreditNoteDetailResponse

	business, err := models.GetBusiness(ctx)
	if err != nil {
		return nil, err
	}

	if err := fromDate.StartOfDayUTCTime(business.Timezone); err != nil {
		return nil, err
	}
	if err := toDate.EndOfDayUTCTime(business.Timezone); err != nil {
		return nil, err
	}

	sql, err := utils.ExecTemplate(sqlT, map[string]interface{}{
		"branchId":    utils.DereferencePtr(branchID, 0) > 0,
		"warehouseId": utils.DereferencePtr(warehouseID, 0) > 0,
	})
	if err != nil {
		return nil, err
	}
	db := config.GetDB()
	if err := db.WithContext(ctx).Raw(sql, map[string]interface{}{
		"businessId":     business.ID,
		"baseCurrencyId": business.BaseCurrencyId,
		"fromDate":       fromDate,
		"toDate":         toDate,
		"branchId":       branchID,
		"warehouseId":    warehouseID,
	}).Scan(&results).Error; err != nil {
		return nil, err
	}
	return results, nil
}
