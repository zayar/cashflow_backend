package models

import (
	"context"
	"time"

	"github.com/mmdatafocus/books_backend/config"
	"github.com/shopspring/decimal"
)

type InvoicePayment struct {
	PaymentDate     time.Time       `json:"paymentDate"`
	PaymentNumber   string          `json:"paymentNumber"`
	ReferenceNumber string          `json:"referenceNumber"`
	Amount          decimal.Decimal `json:"amount"`
	PaymentModeId   int             `json:"payment_mode_id"`
}

// get amount and other infos, from customer payments
func GetInvoicePayments(ctx context.Context, invoiceId int) ([]*InvoicePayment, error) {
	db := config.GetDB()
	var invoicePayments []*InvoicePayment
	if err := db.Table("customer_payments as payments").
		Joins("JOIN paid_invoices as invoices on payments.id = invoices.customer_payment_id").
		Select("payments.payment_date, payments.payment_number, payments.reference_number, invoices.paid_amount as amount, payments.payment_mode_id").
		Where("invoices.invoice_id = ?", invoiceId).
		Scan(&invoicePayments).Error; err != nil {
		return nil, err
	}
	return invoicePayments, nil
}
