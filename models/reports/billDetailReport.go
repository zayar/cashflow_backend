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

type BillDetailResponse struct {
	BillID              int             `json:"billId"`
	BillNumber          string          `json:"billNumber"`
	OrderNumber         string          `json:"orderNumber"`
	BillStatus          string          `json:"billStatus"`
	BillDate            time.Time       `json:"billDate"`
	BillDueDate         time.Time       `json:"billDueDate"`
	TotalAmount         decimal.Decimal `json:"totalAmount"`
	TotalAmountFcy      decimal.Decimal `json:"totalAmountFcy"`
	RemainingBalance    decimal.Decimal `json:"remainingBalance"`
	RemainingBalanceFcy decimal.Decimal `json:"remainingBalanceFcy"`
	CurrencyID          int             `json:"currencyId"`
	CurrencySymbol      string          `json:"currencySymbol"`
	DecimalPlaces       models.DecimalPlaces
	SupplierID          int    `json:"supplierId"`
	SupplierName        string `json:"supplierName"`
}

func GetBillDetailReport(ctx context.Context, fromDate models.MyDateString, toDate models.MyDateString, branchID *int, warehouseID *int) ([]*BillDetailResponse, error) {

	sqlTemplate := `
SELECT
    bill.id AS bill_id,
    bill.bill_number,
    bill.purchase_order_number AS order_number,
    bill.bill_date,
    bill.bill_due_date,
    (
        CASE
			WHEN remaining_balance > 0 AND DATEDIFF(UTC_TIMESTAMP(), bill_due_date) <= 0 THEN 'Overdue'
			ELSE current_status
		END
	) bill_status,
	(
		CASE
			WHEN bill.currency_id <> @baseCurrencyId THEN bill.bill_total_amount
			ELSE 0
		END
	) AS total_amount_fcy,
	(
		CASE
			WHEN bill.currency_id <> @baseCurrencyId THEN bill.bill_total_amount * bill.exchange_rate
			ELSE bill.bill_total_amount
		END
	) total_amount,
	(
			CASE
				WHEN bill.currency_id <> @baseCurrencyId THEN bill.remaining_balance
				ELSE 0
			END
	) AS remaining_balance_fcy,
	(
		CASE
			WHEN bill.currency_id <> @baseCurrencyId THEN bill.remaining_balance * bill.exchange_rate
			ELSE bill.remaining_balance
		END
	) remaining_balance,
	bill.currency_id,
	currencies.symbol AS currency_symbol,
	currencies.decimal_places,
	bill.supplier_id,
	suppliers.name AS supplier_name
FROM
	bills bill
LEFT JOIN currencies ON currencies.id = bill.currency_id
LEFT JOIN suppliers ON suppliers.id = bill.supplier_id
	WHERE bill.business_id = @businessId
	AND bill.bill_date BETWEEN @fromDate AND @toDate
	{{- if .warehouseId }} AND bill.warehouse_id = @warehouseId {{- end }}
	{{- if .branchId }} AND bill.branch_id = @branchId {{- end }}
`
	var results []*BillDetailResponse

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
		"fromDate":       fromDate,
		"toDate":         toDate,
		"branchId":       branchID,
		"warehouseId":    warehouseID,
	}).Scan(&results).Error; err != nil {
		return nil, err
	}
	return results, nil
}
