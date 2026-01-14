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

type PurchaseOrderDetailResponse struct {
	OrderID              int             `json:"orderId"`
	OrderNumber          string          `json:"orderNumber"`
	OrderStatus          string          `json:"orderStatus"`
	OrderDate            time.Time       `json:"orderDate"`
	ExpectedDeliveryDate time.Time       `json:"expectedDeliveryDate"`
	OrderAmount          decimal.Decimal `json:"orderAmount"`
	OrderAmountFcy       decimal.Decimal `json:"orderAmountFcy"`
	// CurrencyID           int             `json:"currencyId"`
	CurrencySymbol string `json:"currencySymbol"`
	DecimalPlaces  models.DecimalPlaces
	SupplierID     int    `json:"supplierId"`
	SupplierName   string `json:"supplierName"`
}

func GetPurchaseOrderDetailReport(ctx context.Context, fromDate models.MyDateString, toDate models.MyDateString, branchID *int, warehouseID *int) ([]*PurchaseOrderDetailResponse, error) {

	sqlTemplate := `
SELECT
    po.id as order_id,
    po.order_number,
    po.current_status AS order_status,
    po.order_date,
    po.expected_delivery_date,
    (
		CASE
			WHEN po.currency_id <> @baseCurrencyId THEN po.order_total_amount
			ELSE 0
		END
	) order_amount_fcy,
    (
        CASE
			WHEN po.currency_id <> @baseCurrencyId THEN po.order_total_amount * po.exchange_rate
			ELSE po.order_total_amount
		END
	) order_amount,
	po.currency_id,
	currencies.symbol AS currency_symbol,
	currencies.decimal_places,
	po.supplier_id,
	suppliers.name AS supplier_name
FROM
    purchase_orders po
LEFT JOIN currencies ON currencies.id = po.currency_id
LEFT JOIN suppliers ON suppliers.id = po.supplier_id
	WHERE po.business_id = @businessId
	AND po.order_date BETWEEN @fromDate AND @toDate
	{{- if .branchId }} AND po.branch_id = @branchId {{- end }}
	{{- if .warehouseId }} AND po.warehouse_id = @warehouseId {{- end }}
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
	var results []*PurchaseOrderDetailResponse
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
