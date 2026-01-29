package reports

import (
	"context"
	"errors"

	"github.com/mmdatafocus/books_backend/config"
	"github.com/mmdatafocus/books_backend/models"
	"github.com/mmdatafocus/books_backend/utils"
	"github.com/shopspring/decimal"
)

type APAgingSummaryResponse struct {
	SupplierName   string `json:"supplierName"`
	SupplierID     int    `json:"supplierId"`
	CurrencySymbol string `json:"currencySymbol"`
	DecimalPlaces  models.DecimalPlaces
	Total          decimal.Decimal `json:"total"`
	TotalFcy       decimal.Decimal `json:"totalFcy"`
	Current        decimal.Decimal `json:"current"`
	Int1to15       decimal.Decimal `json:"int1to15"`
	Int16to30      decimal.Decimal `json:"int16to30"`
	Int31to45      decimal.Decimal `json:"int31to45"`
	Int46plus      decimal.Decimal `json:"int46plus"`
	BillCount      int             `json:"billCount"`
}

func GetAPAgingSummaryReport(ctx context.Context, currentDate models.MyDateString, branchID *int, warehouseID *int) ([]*APAgingSummaryResponse, error) {

	var results []*APAgingSummaryResponse
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
        b.supplier_id,
        b.currency_id,
        (
			CASE
				WHEN currency_id <> @baseCurrencyId THEN b.remaining_balance
                ELSE 0
			END
		) AS remaining_balance,
        (
            CASE
                WHEN b.currency_id = @baseCurrencyId THEN b.remaining_balance
                ELSE b.remaining_balance * b.exchange_rate
            END
        ) AS adjusted_remaining_balance,
        CASE
            WHEN b.remaining_balance > 0 THEN DATEDIFF(@currentDate, b.bill_due_date)
            ELSE 0
        END AS days_overdue
    FROM
        bills b
        LEFT JOIN LatestBillOutbox lbo ON lbo.reference_id = b.id
        LEFT JOIN pub_sub_message_records b_outbox ON b_outbox.id = lbo.max_id
    WHERE
        b.business_id = @businessId
        AND bill_date < @currentDate
        AND b.current_status IN ('Confirmed', 'Partial Paid')
        AND (b_outbox.processing_status IS NULL OR b_outbox.processing_status <> 'DEAD')
        {{- if .branchId }} AND branch_id = @branchId {{- end}}
        {{- if .warehouseId }} AND warehouse_id = @warehouseId {{- end}}
)
SELECT
    supplier_id,
    suppliers.name as supplier_name,
    BillAging.currency_id,
    currencies.symbol as currency_symbol,
	currencies.decimal_places,
    COUNT(*) as bill_count,
    SUM(remaining_balance) as total_fcy,
    SUM(adjusted_remaining_balance) as total,
    SUM(
        CASE
            WHEN days_overdue <= 0 THEN adjusted_remaining_balance
            ELSE 0
        END
    ) AS current,
    SUM(
        CASE
            WHEN days_overdue BETWEEN 1
            AND 15 THEN adjusted_remaining_balance
            ELSE 0
        END
    ) AS int1to15,
    SUM(
        CASE
            WHEN days_overdue BETWEEN 16
            AND 30 THEN adjusted_remaining_balance
            ELSE 0
        END
    ) AS int16to30,
    SUM(
        CASE
            WHEN days_overdue BETWEEN 31
            AND 45 THEN adjusted_remaining_balance
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
    BillAging
    LEFT JOIN suppliers ON suppliers.id = BillAging.supplier_id
    LEFT JOIN currencies ON currencies.id = BillAging.currency_id
GROUP BY
    supplier_id,
    currency_id
ORDER BY
    supplier_id,
    currency_id;
`
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

	db := config.GetDB()
	sql, err := utils.ExecTemplate(sqlTemplate, map[string]interface{}{
		"branchId":    utils.DereferencePtr(branchID, 0),
		"warehouseId": utils.DereferencePtr(warehouseID, 0),
	})
	if err != nil {
		return nil, err
	}

	if err := db.WithContext(ctx).Raw(sql, map[string]interface{}{
		"currentDate":    currentDate,
		"businessId":     businessId,
		"baseCurrencyId": business.BaseCurrencyId,
		"branchId":       branchID,
		"warehouseId":    warehouseID,
	}).Scan(&results).Error; err != nil {
		return nil, err
	}

	return results, nil
}
