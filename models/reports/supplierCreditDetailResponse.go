package reports

import (
	"context"
	"time"

	"github.com/mmdatafocus/books_backend/config"
	"github.com/mmdatafocus/books_backend/models"
	"github.com/mmdatafocus/books_backend/utils"
	"github.com/shopspring/decimal"
)

type SupplierCreditDetailResponse struct {
	SupplierCreditDate       time.Time       `json:"supplierCreditDate"`
	SupplierCreditStatus     string          `json:"supplierCreditStatus"`
	SupplierID               int             `json:"supplierId"`
	SupplierName             *string         `json:"supplierName,omitempty"`
	SupplierCreditNumber     string          `json:"supplierCreditNumber"`
	SupplierCreditAmountFcy  decimal.Decimal `json:"supplierCreditAmountFcy"`
	SupplierCreditBalanceFcy decimal.Decimal `json:"supplierCreditBalanceFcy"`
	SupplierCreditAmount     decimal.Decimal `json:"supplierCreditAmount"`
	SupplierCreditBalance    decimal.Decimal `json:"supplierCreditBalance"`
	CurrencyId               int             `json:"currencyId"`
}

func GetSupplierCreditDetailsReport(ctx context.Context, fromDate models.MyDateString, toDate models.MyDateString, branchID *int, warehouseID *int) ([]*SupplierCreditDetailResponse, error) {

	sqlT := `
SELECT
sc.supplier_credit_date,
sc.current_status supplier_credit_status,
    sc.supplier_credit_number,
    (
		CASE
			WHEN sc.currency_id <> @baseCurrencyId THEN sc.supplier_credit_total_amount
			ELSE 0
		END
	) supplier_credit_amount_fcy,
    (
		CASE
			WHEN sc.currency_id <> @baseCurrencyId THEN sc.remaining_balance
			ELSE 0
		END
	) supplier_credit_balance_fcy,
    (
        CASE
            WHEN sc.currency_id <> @baseCurrencyId THEN sc.supplier_credit_total_amount * sc.exchange_rate
            ELSE sc.supplier_credit_total_amount
        END
    ) supplier_credit_amount,
    (
        CASE
            WHEN sc.currency_id <> @baseCurrencyId THEN sc.remaining_balance * sc.exchange_rate
            ELSE sc.remaining_balance
        END
    ) supplier_credit_balance,
    sc.currency_id,
    sc.supplier_id,
    suppliers.name supplier_name
FROM
    supplier_credits sc
LEFT JOIN suppliers ON suppliers.id = sc.supplier_id
	WHERE sc.business_id = @businessId
	AND sc.supplier_credit_date BETWEEN @fromDate AND @toDate
	{{- if .branchId }} AND sc.branch_id = @branchId {{- end }}
	{{- if .warehouseId }} AND sc.warehouse_id = @warehouseId {{- end }}
ORDER BY sc.supplier_credit_date;
	`
	var results []*SupplierCreditDetailResponse

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
