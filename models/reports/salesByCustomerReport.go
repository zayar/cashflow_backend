package reports

import (
	"context"
	"errors"

	"github.com/mmdatafocus/books_backend/config"
	"github.com/mmdatafocus/books_backend/models"
	"github.com/mmdatafocus/books_backend/utils"
	"github.com/shopspring/decimal"
)

type SalesByCustomerResponse struct {
	CustomerID        int             `json:"CustomerId"`
	CustomerName      *string         `json:"CustomerName,omitempty"`
	InvoiceCount      int             `json:"InvoiceCount"`
	TotalSales        decimal.Decimal `json:"TotalSales"`
	TotalSalesWithTax decimal.Decimal `json:"TotalSalesWithTax"`
	TotalDiscount     decimal.Decimal `json:"TotalDiscount"`
}

func GetSalesByCustomerReport(ctx context.Context, branchId *int, fromDate models.MyDateString, toDate models.MyDateString) ([]*SalesByCustomerResponse, error) {

	sqlT := `
SELECT 
    siv.customer_id,
    siv.total_sales,
    siv.total_sales + siv.total_tax AS total_sales_with_tax,
    siv.total_discount,
    siv.invoice_count,
    customers.name AS customer_name
FROM
    (SELECT 
        customer_id,
            SUM(invoice_total_amount * (CASE
                WHEN currency_id = @baseCurrencyId THEN 1
                ELSE exchange_rate
            END)) AS total_sales,
            SUM(invoice_total_tax_amount * (CASE
                WHEN currency_id = @baseCurrencyId THEN 1
                ELSE exchange_rate
            END)) AS total_tax,
            SUM(invoice_total_discount_amount * (CASE
                WHEN currency_id = @baseCurrencyId THEN 1
                ELSE exchange_rate
            END)) AS total_discount,
            COUNT(sales_invoices.id) AS invoice_count
    FROM
        sales_invoices
    WHERE
        business_id = @businessId
            AND invoice_date BETWEEN @fromDate AND @toDate
            AND current_status IN ('Paid' , 'Partial Paid', 'Confirmed')
		{{- if .branchId }} AND branch_id = @branchId {{- end }}
    GROUP BY customer_id) AS siv
        LEFT JOIN
    customers ON customers.id = siv.customer_id;	
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

	if branchId != nil && *branchId != 0 {
		if err := utils.ValidateResourceId[models.Branch](ctx, businessId, branchId); err != nil {
			return nil, errors.New("branch not found")
		}
	}

	// generating sql from template
	sql, err := utils.ExecTemplate(sqlT, map[string]interface{}{
		"branchId": utils.DereferencePtr(branchId),
	})
	if err != nil {
		return nil, err
	}

	var records []*SalesByCustomerResponse
	db := config.GetDB()
	if err := db.WithContext(ctx).Raw(sql, map[string]interface{}{
		"businessId":     businessId,
		"fromDate":       fromDate,
		"toDate":         toDate,
		"branchId":       branchId,
		"baseCurrencyId": business.BaseCurrencyId,
	}).Scan(&records).Error; err != nil {
		return nil, err
	}

	// // export as excel and saves to file
	// if err := exportExcel(records, "salesByCustomer.xlsx"); err != nil {
	// 	return nil, err
	// }

	// var exporters []ExcelExporter
	// for _, r := range records {
	// 	exporters = append(exporters, r)
	// }

	// if err := _exportExcel(exporters, "salesByCustomer.xlsx",
	// 	"Customer Name", "Invoice Count", "Total Sales With Tax", "Total Sales",
	// ); err != nil {
	// 	return nil, err
	// }

	return records, nil
}

func (r SalesByCustomerResponse) GetCellValues() []interface{} {
	return []interface{}{
		utils.DereferencePtr(r.CustomerName, ""),
		r.InvoiceCount,
		r.TotalSalesWithTax,
		r.TotalSales,
	}
}

// func (r SalesByCustomerResponse) InsertCellValue(f *excelize.File, i int) error {
// 	f.SetCellValue("Sheet1", "A"+fmt.Sprint(i), utils.DereferencePtr(r.CustomerName))
// 	f.SetCellValue("Sheet1", "B"+fmt.Sprint(i), r.InvoiceCount)
// 	return nil
// }
