package reports

import (
	"context"
	"errors"
	"time"

	"github.com/mmdatafocus/books_backend/config"
	"github.com/mmdatafocus/books_backend/models"
	"github.com/mmdatafocus/books_backend/utils"
	"github.com/shopspring/decimal"
)

type ARAgingDetailResponse struct {
	Interval   string           `json:"interval"`
	Details    []*ARAgingDetail `json:"details,omitempty"`
	Amount     decimal.Decimal  `json:"amount"`
	BalanceDue decimal.Decimal  `json:"balanceDue"`
}

type ARAgingDetail struct {
	InvoiceID           int       `json:"invoiceId"`
	InvoiceDate         time.Time `json:"invoiceDate"`
	InvoiceNumber       string    `json:"invoiceNumber"`
	InvoiceStatus       string    `json:"invoiceStatus"`
	DueInterval         string    `json:"due_interval"`
	CustomerID          int       `json:"customerId"`
	CustomerName        *string   `json:"customerName,omitempty"`
	Age                 int       `json:"age"`
	CurrencySymbol      string    `json:"currencySymbol"`
	DecimalPlaces       models.DecimalPlaces
	TotalAmount         decimal.Decimal `json:"totalAmount"`
	RemainingBalance    decimal.Decimal `json:"remainingBalance"`
	TotalAmountFcy      decimal.Decimal `json:"totalAmountFcy"`
	RemainingBalanceFcy decimal.Decimal `json:"remainingBalanceFcy"`
}

// type ARAgingDetailResponse struct {
// 	Interval   DaysInterval
// 	Details    []*ARAgingDetail
// 	Amount     decimal.Decimal
// 	BalanceDue decimal.Decimal
// }

// type ARAgingDetailResponse struct {
// 	Current             []*ARAgingDetail `json:"current,omitempty"`
// 	CurrentAmount       decimal.Decimal  `json:"currentAmount"`
// 	CurrentBalanceDue   decimal.Decimal  `json:"currentBalanceDue"`
// 	Int1to15            []*ARAgingDetail `json:"int1to15,omitempty"`
// 	Int1to15Amount      decimal.Decimal  `json:"int1to15Amount"`
// 	Int1to15BalanceDue  decimal.Decimal  `json:"int1to15BalanceDue"`
// 	Int16to30           []*ARAgingDetail `json:"int16to30,omitempty"`
// 	Int16to30Amount     decimal.Decimal  `json:"int16to30Amount"`
// 	Int16to30BalanceDue decimal.Decimal  `json:"int16to30BalanceDue"`
// 	Int31to45           []*ARAgingDetail `json:"int31to45,omitempty"`
// 	Int31to45Amount     decimal.Decimal  `json:"int31to45Amount"`
// 	Int31to45BalanceDue decimal.Decimal  `json:"int31to45BalanceDue"`
// 	Int45plus           []*ARAgingDetail `json:"int45plus,omitempty"`
// 	Int45plusAmount     decimal.Decimal  `json:"int45plusAmount"`
// 	Int45plusBalanceDue decimal.Decimal  `json:"int45plusBalanceDue"`
// }

func GetARAgingDetailReport(ctx context.Context, currentDate models.MyDateString, branchID *int, warehouseID *int) ([]*ARAgingDetailResponse, error) {

	sqlTemplate := `
WITH InvoiceAging AS (
    SELECT
        id,
        invoice_number,
        current_status,
        exchange_rate,
        customer_id,
        currency_id,
        invoice_date,
		(
			CASE
				WHEN currency_id <> @baseCurrencyId THEN invoice_total_amount
				ELSE 0
			END
		) invoice_total_amount,
		(
			CASE
				WHEN currency_id <> @baseCurrencyId THEN invoice_total_amount
				ELSE 0
			END
		) remaining_balance,
		-- calculating adjusted amounts
		(
			CASE
				WHEN currency_id <> @baseCurrencyId THEN invoice_total_amount * exchange_rate
				ELSE invoice_total_amount
			END
		) AS adjusted_invoice_total_amount,
		(
			CASE
				WHEN currency_id <> @baseCurrencyId THEN remaining_balance * exchange_rate
				ELSE remaining_balance
			END
		) AS adjusted_remaining_balance,
        CASE
            WHEN remaining_balance > 0 THEN DATEDIFF(@currentDate, invoice_due_date)
            ELSE 0
        END AS days_overdue
    FROM
        sales_invoices
    where
		business_id = @businessId
        AND current_status in ('Confirmed', 'Partial Paid')
        AND invoice_date < @currentDate
		{{- if .warehouseId }} AND warehouse_id = @warehouseId {{- end }}
		{{- if .branchId }} AND branch_id = @branchId {{- end }}
)
SELECT
    InvoiceAging.id as invoice_id,
    InvoiceAging.invoice_number,
    InvoiceAging.invoice_date,
    InvoiceAging.customer_id,
    customers.name as customer_name,
    currencies.symbol AS currency_symbol,
	currencies.decimal_places,
    InvoiceAging.invoice_total_amount AS total_amount_fcy,
    InvoiceAging.adjusted_invoice_total_amount AS total_amount,
    InvoiceAging.remaining_balance AS remaining_balance_fcy,
    InvoiceAging.adjusted_remaining_balance AS remaining_balance,
    (
        CASE
            WHEN days_overdue > 0 THEN days_overdue
            ELSE 0
        END
    ) as age,
    (
        CASE
            WHEN days_overdue <= 0 THEN InvoiceAging.current_status
            ELSE "Overdue"
        END
    ) AS invoice_status,
    (
        CASE
            WHEN days_overdue <= 0 THEN "current"
            WHEN days_overdue BETWEEN 1
            AND 15 THEN "int1to15"
            WHEN days_overdue BETWEEN 16
            AND 30 THEN "int16to30"
            WHEN days_overdue BETWEEN 31
            AND 45 THEN "int31to45"
            ELSE "int46plus"
        END
    ) AS due_interval
FROM
    InvoiceAging
    LEFT JOIN currencies on InvoiceAging.currency_id = currencies.id
    LEFT JOIN customers on InvoiceAging.customer_id = customers.id
	ORDER BY due_interval;
`
	var agingDetails []*ARAgingDetail
	businessId, ok := utils.GetBusinessIdFromContext(ctx)
	if !ok || businessId == "" {
		return nil, errors.New("business id is required")
	}
	business, err := models.GetBusiness(ctx)
	if err != nil {
		return nil, errors.New("business id is required")
	}
	if err := currentDate.EndOfDayUTCTime(business.Timezone); err != nil {
		return nil, err
	}

	sql, err := utils.ExecTemplate(sqlTemplate, map[string]interface{}{
		"warehouseId": utils.DereferencePtr(warehouseID, 0),
		"branchId":    utils.DereferencePtr(branchID, 0),
	})
	if err != nil {
		return nil, err
	}

	db := config.GetDB()
	if err := db.WithContext(ctx).Raw(sql, map[string]interface{}{
		"businessId":     businessId,
		"baseCurrencyId": business.BaseCurrencyId,
		"currentDate":    currentDate,
		"warehouseId":    warehouseID,
		"branchId":       branchID,
	}).Scan(&agingDetails).Error; err != nil {
		return nil, err
	}
	if len(agingDetails) <= 0 {
		return nil, nil
	}
	var results []*ARAgingDetailResponse

	currentInterval := agingDetails[0].DueInterval
	currentAmount := decimal.NewFromInt(0)
	currentBalanceDue := decimal.NewFromInt(0)
	currentDetails := make([]*ARAgingDetail, 0)

	for _, invoice := range agingDetails {
		// if reach a new interval
		if invoice.DueInterval != currentInterval {
			// copying the value of previous interval
			responseForPreviousInterval := ARAgingDetailResponse{
				Interval:   currentInterval,
				Amount:     currentAmount,
				BalanceDue: currentBalanceDue,
				Details:    currentDetails,
			}
			results = append(results, &responseForPreviousInterval)

			// reset current values for new interval
			currentInterval = invoice.DueInterval
			currentAmount = decimal.NewFromInt(0)
			currentBalanceDue = decimal.NewFromInt(0)
			currentDetails = nil
		}

		currentAmount = currentAmount.Add(invoice.TotalAmount)
		currentBalanceDue = currentBalanceDue.Add(invoice.RemainingBalance)
		currentDetails = append(currentDetails, invoice)
	}

	responseForLastInterval := ARAgingDetailResponse{
		Interval:   currentInterval,
		Details:    currentDetails,
		Amount:     currentAmount,
		BalanceDue: currentBalanceDue,
	}
	results = append(results, &responseForLastInterval)

	return results, nil
}
