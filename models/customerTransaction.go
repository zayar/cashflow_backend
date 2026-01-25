package models

import (
	"context"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/mmdatafocus/books_backend/config"
	"github.com/mmdatafocus/books_backend/utils"
	"github.com/shopspring/decimal"
)

type CustomerTransaction struct {
	ID             string          `json:"id"`
	SourceType     string          `json:"source_type"` // "Invoice", "Credit Note", "Payment"
	SourceID       int             `json:"source_id"`
	Date           time.Time       `json:"date"`
	DocumentNumber string          `json:"document_number"`
	Status         string          `json:"status"`
	Description    string          `json:"description"`
	Amount         decimal.Decimal `json:"amount"`
	Balance        decimal.Decimal `json:"balance"`
	CurrencyID     int             `json:"currency_id"`
	ExchangeRate   decimal.Decimal `json:"exchange_rate"`
}

type CustomerTransactionsResponse struct {
	Transactions []CustomerTransaction `json:"transactions"`
	TotalCount   int64                 `json:"total_count"`
}

func GetCustomerTransactions(ctx context.Context, customerId int, fromDate, toDate *time.Time, docTypes []string, status string, search string, page, limit int) (*CustomerTransactionsResponse, error) {
	db := config.GetDB()
	businessId, ok := utils.GetBusinessIdFromContext(ctx)
	if !ok || businessId == "" {
		return nil, context.Canceled
	}

	var transactions []CustomerTransaction

	// 1. Fetch Sales Invoices
	if len(docTypes) == 0 || contains(docTypes, "Invoice") {
		var invoices []SalesInvoice
		query := db.WithContext(ctx).Where("business_id = ? AND customer_id = ?", businessId, customerId)
		if fromDate != nil {
			query = query.Where("invoice_date >= ?", fromDate)
		}
		if toDate != nil {
			query = query.Where("invoice_date <= ?", toDate)
		}
		if status != "" {
			query = query.Where("current_status = ?", status)
		}
		if search != "" {
			query = query.Where("(invoice_number LIKE ? OR reference_number LIKE ?)", "%"+search+"%", "%"+search+"%")
		}
		// Exclude "Customer Opening Balance" if desired, or keep it. It acts like an invoice.
		// query = query.Where("invoice_number != ?", "Customer Opening Balance") 

		if err := query.Find(&invoices).Error; err != nil {
			return nil, err
		}

		for _, inv := range invoices {
			transactions = append(transactions, CustomerTransaction{
				ID:             "IV:" + strconv.Itoa(inv.ID),
				SourceType:     "Invoice",
				SourceID:       inv.ID,
				Date:           inv.InvoiceDate,
				DocumentNumber: inv.InvoiceNumber,
				Status:         string(inv.CurrentStatus),
				Description:    inv.InvoiceSubject, // or Notes
				Amount:         inv.InvoiceTotalAmount,
				Balance:        inv.RemainingBalance,
				CurrencyID:     inv.CurrencyId,
				ExchangeRate:   inv.ExchangeRate,
			})
		}
	}

	// 2. Fetch Credit Notes
	if len(docTypes) == 0 || contains(docTypes, "Credit Note") {
		var creditNotes []CreditNote
		query := db.WithContext(ctx).Where("business_id = ? AND customer_id = ?", businessId, customerId)
		if fromDate != nil {
			query = query.Where("credit_note_date >= ?", fromDate)
		}
		if toDate != nil {
			query = query.Where("credit_note_date <= ?", toDate)
		}
		if status != "" {
			query = query.Where("current_status = ?", status)
		}
		if search != "" {
			query = query.Where("(credit_note_number LIKE ? OR reference_number LIKE ?)", "%"+search+"%", "%"+search+"%")
		}

		if err := query.Find(&creditNotes).Error; err != nil {
			return nil, err
		}

		for _, cn := range creditNotes {
			transactions = append(transactions, CustomerTransaction{
				ID:             "CN:" + strconv.Itoa(cn.ID),
				SourceType:     "Credit Note",
				SourceID:       cn.ID,
				Date:           cn.CreditNoteDate,
				DocumentNumber: cn.CreditNoteNumber,
				Status:         string(cn.CurrentStatus),
				Description:    cn.CreditNoteSubject,
				Amount:         cn.CreditNoteTotalAmount, // Usually negative in context of balance, but here we show absolute amount
				Balance:        cn.RemainingBalance,
				CurrencyID:     cn.CurrencyId,
				ExchangeRate:   cn.ExchangeRate,
			})
		}
	}

	// 3. Fetch Customer Payments
	if len(docTypes) == 0 || contains(docTypes, "Payment") {
		var payments []CustomerPayment
		query := db.WithContext(ctx).Where("business_id = ? AND customer_id = ?", businessId, customerId)
		if fromDate != nil {
			query = query.Where("payment_date >= ?", fromDate)
		}
		if toDate != nil {
			query = query.Where("payment_date <= ?", toDate)
		}
		// Status for payments? Usually they are just "Paid" or "Confirmed" if they exist.
		// CustomerPayment doesn't seem to have a Status field in the struct I read earlier?
		// Wait, let me check CustomerPayment struct again.
        // It DOES NOT have a CurrentStatus field in the struct definition I read (lines 15-35 of customerPayment.go).
        // It has CreatedAt/UpdatedAt.
        // So we assume they are valid if they exist.
		if search != "" {
			query = query.Where("(payment_number LIKE ? OR reference_number LIKE ?)", "%"+search+"%", "%"+search+"%")
		}

		if err := query.Find(&payments).Error; err != nil {
			return nil, err
		}

		for _, pay := range payments {
			transactions = append(transactions, CustomerTransaction{
				ID:             "CP:" + strconv.Itoa(pay.ID),
				SourceType:     "Payment",
				SourceID:       pay.ID,
				Date:           pay.PaymentDate,
				DocumentNumber: pay.PaymentNumber,
				Status:         "Paid", // Implicit
				Description:    pay.Notes,
				Amount:         pay.Amount,
				Balance:        decimal.Zero, // Payments don't have balance usually, unless unused amount?
				// Logic for unused amount would require checking used amount which isn't directly on CustomerPayment struct I saw.
				// Actually, CustomerPayment has PaidInvoices. The difference might be unused (Advance).
				// But let's keep it simple: Balance = 0.
				CurrencyID:     pay.CurrencyId,
				ExchangeRate:   pay.ExchangeRate,
			})
		}
	}

	// Sort by Date Descending
	sort.Slice(transactions, func(i, j int) bool {
		return transactions[i].Date.After(transactions[j].Date)
	})

	// Pagination
	totalCount := int64(len(transactions))
	start := (page - 1) * limit
	if start < 0 {
		start = 0
	}
	end := start + limit
	if end > int(totalCount) {
		end = int(totalCount)
	}
	
	paginated := []CustomerTransaction{}
	if start < int(totalCount) {
		paginated = transactions[start:end]
	}

	return &CustomerTransactionsResponse{
		Transactions: paginated,
		TotalCount:   totalCount,
	}, nil
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if strings.EqualFold(s, item) {
			return true
		}
	}
	return false
}
