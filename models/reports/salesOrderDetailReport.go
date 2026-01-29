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

type SalesOrderDetailResponse struct {
	OrderID              int             `json:"orderId"`
	OrderNumber          string          `json:"orderNumber"`
	OrderStatus          string          `json:"orderStatus"`
	OrderDate            time.Time       `json:"orderDate"`
	ExpectedShipmentDate time.Time       `json:"expectedShipmentDate"`
	OrderAmount          decimal.Decimal `json:"orderAmount"`
	CurrencyID           int             `json:"currencyId"`
	CurrencySymbol       string          `json:"currencySymbol"`
	DecimalPlaces        models.DecimalPlaces
	OrderAmountFcy       decimal.Decimal `json:"orderAmountFcy"`
	CustomerID           int             `json:"customerId"`
	CustomerName         string          `json:"customerName"`
}

func GetSalesOrderDetailReport(ctx context.Context, fromDate models.MyDateString, toDate models.MyDateString, branchID *int, warehouseID *int) ([]*SalesOrderDetailResponse, error) {

	sqlTemplate := `
SELECT
    so.id as order_id,
    so.order_number,
    so.current_status AS order_status,
    so.order_date,
    so.expected_shipment_date,
    (
		CASE
			WHEN so.currency_id <> @baseCurrencyId THEN so.order_total_amount
			ELSE 0
		END
	) order_amount_fcy,
    (
        CASE
            WHEN so.currency_id <> @baseCurrencyId THEN so.order_total_amount * so.exchange_rate
            ELSE so.order_total_amount
        END
    ) order_amount,
    so.currency_id,
    currencies.symbol AS currency_symbol,
	currencies.decimal_places,
    so.customer_id,
    customers.name AS customer_name
FROM
    sales_orders so
    LEFT JOIN currencies ON currencies.id = so.currency_id
    LEFT JOIN customers ON customers.id = so.customer_id
WHERE
    so.business_id = @businessId
	AND so.order_date BETWEEN @fromDate AND @toDate
    AND so.current_status NOT IN ('Draft', 'Void')
	{{- if .branchId }} AND so.branch_id = @branchId {{- end }}
	{{- if .warehouseId }} AND so.warehouse_id = @warehouseId {{- end }}
`

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
	var results []*SalesOrderDetailResponse
	sql, err := utils.ExecTemplate(sqlTemplate, map[string]interface{}{
		"branchId":    utils.DereferencePtr(branchID, 0),
		"warehouseId": utils.DereferencePtr(warehouseID, 0),
	})
	if err != nil {
		return nil, err
	}

	if err := db.WithContext(ctx).Raw(sql, map[string]interface{}{
		"businessId":     businessId,
		"baseCurrencyId": business.BaseCurrencyId,
		"branchId":       branchID,
		"warehouseId":    warehouseID,
		"fromDate":       fromDate,
		"toDate":         toDate,
	}).Scan(&results).Error; err != nil {
		return nil, err
	}
	return results, nil
}
