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

type APAgingDetail struct {
	BillID              int       `json:"billId"`
	BillDate            time.Time `json:"billDate"`
	BillNumber          string    `json:"billNumber"`
	BillStatus          string    `json:"billStatus"`
	SupplierID          int       `json:"supplierId"`
	SupplierName        *string   `json:"supplierName,omitempty"`
	Age                 int       `json:"age"`
	DueInterval         string    `json:"due_interval"`
	CurrencySymbol      string    `json:"currencySymbol"`
	DecimalPlaces       models.DecimalPlaces
	TotalAmount         decimal.Decimal `json:"totalAmount"`
	RemainingBalance    decimal.Decimal `json:"remainingBalance"`
	TotalAmountFcy      decimal.Decimal `json:"totalAmountFcy"`
	RemainingBalanceFcy decimal.Decimal `json:"remainingBalanceFcy"`
}

type APAgingDetailResponse struct {
	Interval   string           `json:"interval"`
	Details    []*APAgingDetail `json:"details,omitempty"`
	Amount     decimal.Decimal  `json:"amount"`
	BalanceDue decimal.Decimal  `json:"balanceDue"`
}

func GetAPAgingDetailReport(ctx context.Context, currentDate models.MyDateString, branchID *int, warehouseID *int) ([]*APAgingDetailResponse, error) {

	sqlTemplate := `
WITH LatestBillOutbox AS (
    SELECT
        reference_id,
        MAX(id) AS max_id
    FROM
        pub_sub_message_records
    WHERE
        business_id = @businessId
        AND reference_type = 'BL'
    GROUP BY
        reference_id
),
BillAging AS (
    SELECT
        id,
        bill_number,
        current_status,
        exchange_rate,
        supplier_id,
        currency_id,
        bill_date,
		(
			CASE
				WHEN currency_id <> @baseCurrencyId THEN bill_total_amount
				ELSE 0
			END
		) AS bill_total_amount,
		(
			CASE
				WHEN currency_id <> @baseCurrencyId THEN remaining_balance
				ELSE 0
			END
		) AS remaining_balance,
		-- calculating adjusted amount
		(
			CASE
				WHEN currency_id <> @baseCurrencyId THEN bill_total_amount * exchange_rate
				ELSE bill_total_amount
			END
		) AS adjusted_bill_total_amount,
		(
			CASE
				WHEN currency_id <> @baseCurrencyId THEN remaining_balance * exchange_rate
				ELSE remaining_balance
			END
		) AS adjusted_remaining_balance,
        CASE
            WHEN remaining_balance > 0 THEN DATEDIFF(@currentDate, bill_due_date)
            ELSE 0
        END AS days_overdue
    FROM
        bills
        LEFT JOIN LatestBillOutbox lbo ON lbo.reference_id = bills.id
        LEFT JOIN pub_sub_message_records b_outbox ON b_outbox.id = lbo.max_id
    where
        bills.business_id = @businessId
        AND current_status in ('Confirmed', 'Partial Paid')
        AND bill_date < @currentDate
        AND (b_outbox.processing_status IS NULL OR b_outbox.processing_status <> 'DEAD')
		{{- if .warehouseId }} AND bills.warehouse_id = @warehouseId {{- end }}
		{{- if .branchId }} AND bills.branch_id = @branchId {{- end }}
)
SELECT
    BillAging.id as bill_id,
    BillAging.bill_number,
    BillAging.bill_date,
    BillAging.supplier_id,
    suppliers.name as supplier_name,
    currencies.symbol AS currency_symbol,
	currencies.decimal_places,
    BillAging.bill_total_amount AS total_amount_fcy,
    BillAging.adjusted_bill_total_amount AS total_amount,
    BillAging.remaining_balance AS remaining_balance_fcy,
    BillAging.adjusted_remaining_balance AS remaining_balance,
    (
        CASE
            WHEN days_overdue > 0 THEN days_overdue
            ELSE 0
        END
    ) as age,
    (
        CASE
            WHEN days_overdue <= 0 THEN BillAging.current_status
            ELSE "Overdue"
        END
    ) AS bill_status,
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
    BillAging
    LEFT JOIN currencies on BillAging.currency_id = currencies.id
    LEFT JOIN suppliers on BillAging.supplier_id = suppliers.id
order by
    due_interval;
`
	var agingDetails []*APAgingDetail
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
		"branchId":    utils.DereferencePtr(branchID, 0),
		"warehouseId": utils.DereferencePtr(warehouseID, 0),
	})
	if err != nil {
		return nil, err
	}
	db := config.GetDB()
	if err := db.WithContext(ctx).Raw(sql, map[string]interface{}{
		"businessId":     businessId,
		"baseCurrencyId": business.BaseCurrencyId,
		"currentDate":    currentDate,
		"branchId":       branchID,
		"warehouseId":    warehouseID,
	}).Scan(&agingDetails).Error; err != nil {
		return nil, err
	}
	var results []*APAgingDetailResponse

	if len(agingDetails) <= 0 {
		return nil, nil
	}

	currentInterval := agingDetails[0].DueInterval
	currentAmount := decimal.NewFromInt(0)
	currentBalanceDue := decimal.NewFromInt(0)
	currentDetails := make([]*APAgingDetail, 0)

	for _, invoice := range agingDetails {
		// if reach a new interval
		if invoice.DueInterval != currentInterval {
			// copying the value of previous interval
			responseForPreviousInterval := APAgingDetailResponse{
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

	responseForLastInterval := APAgingDetailResponse{
		Interval:   currentInterval,
		Details:    currentDetails,
		Amount:     currentAmount,
		BalanceDue: currentBalanceDue,
	}
	results = append(results, &responseForLastInterval)

	return results, nil
}
