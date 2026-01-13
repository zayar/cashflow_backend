package reports

import (
	"context"
	"errors"

	"bitbucket.org/mmdatafocus/books_backend/config"
	"bitbucket.org/mmdatafocus/books_backend/models"
	"bitbucket.org/mmdatafocus/books_backend/utils"
	"github.com/shopspring/decimal"
)

type SalesBySalesPersonResponse struct {
	SalesPersonID            int             `json:"SalesPersonId"`
	SalesPersonName          *string         `json:"SalesPersonName,omitempty"`
	InvoiceCount             int             `json:"InvoiceCount"`
	TotalInvoiceSales        decimal.Decimal `json:"TotalInvoiceSales"`
	TotalInvoiceSalesWithTax decimal.Decimal `json:"TotalInvoiceSalesWithTax"`
	TotalInvoiceDiscount     decimal.Decimal `json:"TotalInvoiceDiscount"`
	// TotalInvoiceTaxDiscount    decimal.Decimal
	CreditNoteCount             int             `json:"CreditNoteCount"`
	TotalCreditNoteSales        decimal.Decimal `json:"TotalCreditNoteSales"`
	TotalCreditNoteSalesWithTax decimal.Decimal `json:"TotalCreditNoteSalesWithTax"`
	TotalCreditNoteDiscount     decimal.Decimal `json:"TotalCreditNoteDiscount"`
	// TotalCreditNoteTaxDiscount decimal.Decimal `json:"TotalCreditNoteSalesWithTax"`
	TotalSales        decimal.Decimal `json:"TotalSales"`
	TotalSalesWithTax decimal.Decimal `json:"TotalSalesWithTax"`
}

// do more checking for reports not filtering businessId
func GetSalesBySalesPersonReport(ctx context.Context, branchId *int, fromDate models.MyDateString, toDate models.MyDateString) ([]*SalesBySalesPersonResponse, error) {
	var records []*SalesBySalesPersonResponse
	sqlTemplate := `
WITH SalesInvoices AS (
    SELECT
        sales_person_id AS spid,
        COUNT(id) AS invoiceCount,
        SUM(invoice_total_amount * (CASE WHEN currency_id = @baseCurrencyId THEN 1 ELSE exchange_rate END)) AS totalInvoiceSalesWithTax,
        SUM(invoice_tax_amount * (CASE WHEN currency_id = @baseCurrencyId THEN 1 ELSE exchange_rate END)) AS totalInvoiceTax,
        SUM(invoice_discount_amount * (CASE WHEN currency_id = @baseCurrencyId THEN 1 ELSE exchange_rate END)) AS totalInvoiceDiscount
    FROM
        sales_invoices
    WHERE
        invoice_date BETWEEN @fromDate AND @toDate
        AND current_status IN ('Paid', 'Partial Paid', 'Confirmed')
       {{- if .branchId }} AND branch_id = @branchId {{- end }}
    GROUP BY
        sales_person_id
),
CreditNotes as (
    SELECT
        sales_person_id AS spid,
        COUNT(id) AS creditNoteCount,
        SUM(credit_note_total_amount * (CASE WHEN currency_id = @baseCurrencyId THEN 1 ELSE exchange_rate END)) AS totalCreditNoteSalesWithTax,
        SUM(credit_note_tax_amount * (CASE WHEN currency_id = @baseCurrencyId THEN 1 ELSE exchange_rate END)) AS totalCreditNoteTax,
        SUM(credit_note_discount_amount * (CASE WHEN currency_id = @baseCurrencyId THEN 1 ELSE exchange_rate END)) AS totalCreditNoteDiscount
    FROM
        credit_notes
    WHERE
        credit_note_date BETWEEN @fromDate AND @toDate
        AND current_status = 'Closed'
       {{- if .branchId }} AND branch_id = @branchId {{- end }}
    GROUP BY
        sales_person_id
)
SELECT
    SalesInvoices.invoiceCount as invoice_count,
    SalesInvoices.totalInvoiceSalesWithTax as total_invoice_sales_with_tax,
    SalesInvoices.totalInvoiceSalesWithTax - SalesInvoices.totalInvoiceTax as total_invoice_sales,
    SalesInvoices.totalInvoiceDiscount as total_invoice_discount,
    CreditNotes.creditNoteCount as credit_note_count,
    CreditNotes.totalCreditNoteSalesWithTax as total_credit_note_sales_with_tax,
    CreditNotes.totalCreditNoteSalesWithTax - CreditNotes.totalCreditNoteTax as total_credit_note_sales,
    CreditNotes.totalCreditNoteDiscount as total_credit_note_discount,
    sales_people.id as sales_person_id,
    sales_people.name as sales_person_name,
	(SalesInvoices.totalInvoiceSalesWithTax - SalesInvoices.totalInvoiceTax) - (CreditNotes.totalCreditNoteSalesWithTax - CreditNotes.totalCreditNoteTax) total_sales,
	(SalesInvoices.totalInvoiceSalesWithTax - CreditNotes.totalCreditNoteSalesWithTax) total_sales_with_tax
FROM
    sales_people
    LEFT JOIN SalesInvoices ON SalesInvoices.spid = sales_people.id
    LEFT JOIN CreditNotes ON CreditNotes.spid = sales_people.id
	WHERE sales_people.business_id = @businessId
`

	business, err := models.GetBusiness(ctx)
	if err != nil {
		return nil, errors.New("business id is required")
	}
	businessId := business.ID.String()
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
	sql, err := utils.ExecTemplate(sqlTemplate, map[string]interface{}{
		"branchId": utils.DereferencePtr(branchId),
	})
	if err != nil {
		return nil, err
	}

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

	return records, nil
}
