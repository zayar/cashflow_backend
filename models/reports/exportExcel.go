package reports

/*

type ExcelExporter interface {
	// InsertCellValue(f *excelize.File, rowNo int) error
	GetCellValues() []interface{}
}

func getSalesByCustomerReport(ctx context.Context) ([]*SalesByCustomerResponse, error) {

	sql := `
SELECT
    siv.customer_id,
    siv.total_sales,
    siv.total_sales + siv.total_tax as total_sales_with_tax,
    siv.total_discount,
    siv.invoice_count,
    customers.name AS customer_name
FROM
    (
        SELECT
            customer_id,
            SUM(invoice_total_amount) AS total_sales,
            SUM(invoice_total_tax_amount) AS total_tax,
            SUM(invoice_total_discount_amount) AS total_discount,
            COUNT(sales_invoices.id) AS invoice_count
        FROM
            sales_invoices
        WHERE
		current_status IN ('Paid', 'Partial Paid', 'Confirmed')
        GROUP BY
            customer_id
    ) AS siv
    LEFT JOIN customers ON customers.id = siv.customer_id;
`

	var records []*SalesByCustomerResponse
	db := config.GetDB()
	if err := db.WithContext(ctx).Raw(sql).Scan(&records).Error; err != nil {
		return nil, err
	}

	return records, nil
}

func ExportExcel(w http.ResponseWriter, r *http.Request) {

	f := excelize.NewFile()
	_, err := f.NewSheet("Sheet1")
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
	}
	data, err := getSalesByCustomerReport(context.Background())
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
	}

	// Add headers
	f.SetCellValue("Sheet1", "A1", "CustomerName")
	f.SetCellValue("Sheet1", "B1", "InvoiceCount")
	f.SetCellValue("Sheet1", "C1", "Sales")
	f.SetCellValue("Sheet1", "D1", "SalesWithTax")

	// Add data
	for i, d := range data {
		f.SetCellValue("Sheet1", "A"+fmt.Sprint(i+2), utils.DereferencePtr(d.CustomerName, ""))
		f.SetCellValue("Sheet1", "B"+fmt.Sprint(i+2), d.InvoiceCount)
		f.SetCellValue("Sheet1", "C"+fmt.Sprint(i+2), d.TotalSales)
		f.SetCellValue("Sheet1", "D"+fmt.Sprint(i+2), d.TotalSalesWithTax)
	}

	w.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	w.Header().Set("Content-Disposition", "attachment; filename=export.xlsx")
	if err := f.Write(w); err != nil {
		http.Error(w, "Failed to write file", http.StatusInternalServerError)
	}
}

func exportExcel(data []*SalesByCustomerResponse, filename string) error {

	f := excelize.NewFile()
	_, err := f.NewSheet("Sheet1")
	if err != nil {
		return err
	}

	// Add headers
	f.SetCellValue("Sheet1", "A1", "CustomerName")
	f.SetCellValue("Sheet1", "B1", "InvoiceCount")
	f.SetCellValue("Sheet1", "C1", "Sales")
	f.SetCellValue("Sheet1", "D1", "SalesWithTax")

	// Add data
	for i, d := range data {
		f.SetCellValue("Sheet1", "A"+fmt.Sprint(i+2), utils.DereferencePtr(d.CustomerName, ""))
		f.SetCellValue("Sheet1", "B"+fmt.Sprint(i+2), d.InvoiceCount)
		f.SetCellValue("Sheet1", "C"+fmt.Sprint(i+2), d.TotalSales)
		f.SetCellValue("Sheet1", "D"+fmt.Sprint(i+2), d.TotalSalesWithTax)
	}

	if err := f.SaveAs(filename); err != nil {
		return err
	}
	return nil
}

func _exportExcel(data []ExcelExporter, filename string, headings ...string) error {

	f := excelize.NewFile()
	sheetName := "Sheet1"
	_, err := f.NewSheet("Sheet1")
	if err != nil {
		return err
	}

	// Add headers

	col := 'A'
	for _, h := range headings {
		f.SetCellValue(sheetName, string(col)+"1", h)
		col++
	}

	// Add data
	rowNo := 2
	for _, d := range data {
		// d.InsertCellValue(f, rowNo)
		col := 'A'
		for _, value := range d.GetCellValues() {
			f.SetCellValue(sheetName, string(col)+fmt.Sprint(rowNo), value)
			col++
		}
		rowNo++
	}

	if err := f.SaveAs(filename); err != nil {
		return err
	}
	return nil
}

*/
