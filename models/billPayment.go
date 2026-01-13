package models

import (
	"context"
	"time"

	"bitbucket.org/mmdatafocus/books_backend/config"
	"github.com/shopspring/decimal"
)

type BillPayment struct {
	PaymentDate     time.Time       `json:"paymentDate"`
	PaymentNumber   string          `json:"paymentNumber"`
	ReferenceNumber string          `json:"referenceNumber"`
	Amount          decimal.Decimal `json:"amount"`
	PaymentModeId   int             `json:"payment_mode_id"`
}

// get amount and other infos, from supplier payments
func GetBillPayments(ctx context.Context, billId int) ([]*BillPayment, error) {
	db := config.GetDB()
	var billPayments []*BillPayment
	if err := db.Table("supplier_payments as payments").
		Joins("JOIN supplier_paid_bills as bills on payments.id = bills.supplier_payment_id").
		Select("payments.payment_date, payments.payment_number, payments.reference_number, bills.paid_amount as amount, payments.payment_mode_id").
		Where("bills.bill_id = ?", billId).
		Scan(&billPayments).Error; err != nil {
		return nil, err
	}
	return billPayments, nil
}

// sql := `
// SELECT
// payments.id AS payment_id,
// payments.payment_date AS payment_date,
// payments.payment_number AS payment_number,
// payments.reference_number AS reference_number,
// bills.paid_amount AS amount,
// payments.payment_mode_id AS payment_mode_id,
// bills.bill_id AS bill_id
// FROM
// 	supplier_payments AS payments
// JOIN supplier_paid_bills AS bills
// ON
// 	payments.id = bills.supplier_payment_id
// WHERE
// 	bills.bill_id = ?;`
