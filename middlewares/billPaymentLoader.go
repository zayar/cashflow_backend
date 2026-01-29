package middlewares

import (
	"context"
	"time"

	"github.com/graph-gophers/dataloader/v7"
	"github.com/mmdatafocus/books_backend/models"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

type billPaymentReader struct {
	db *gorm.DB
}

type billPaymentRow struct {
	BillId          int             `gorm:"column:bill_id"`
	PaymentDate     time.Time       `gorm:"column:payment_date"`
	PaymentNumber   string          `gorm:"column:payment_number"`
	ReferenceNumber string          `gorm:"column:reference_number"`
	Amount          decimal.Decimal `gorm:"column:amount"`
	PaymentModeId   int             `gorm:"column:payment_mode_id"`
}

func (r *billPaymentReader) GetBillPayments(ctx context.Context, billIds []int) []*dataloader.Result[[]*models.BillPayment] {
	var rows []billPaymentRow
	err := r.db.WithContext(ctx).
		Table("supplier_payments as payments").
		Joins("JOIN supplier_paid_bills as bills on payments.id = bills.supplier_payment_id").
		Select("payments.payment_date, payments.payment_number, payments.reference_number, bills.paid_amount as amount, payments.payment_mode_id, bills.bill_id").
		Where("bills.bill_id IN ?", billIds).
		Scan(&rows).Error
	if err != nil {
		return handleError[[]*models.BillPayment](len(billIds), err)
	}

	resultMap := make(map[int][]*models.BillPayment, len(billIds))
	for _, row := range rows {
		resultMap[row.BillId] = append(resultMap[row.BillId], &models.BillPayment{
			PaymentDate:     row.PaymentDate,
			PaymentNumber:   row.PaymentNumber,
			ReferenceNumber: row.ReferenceNumber,
			Amount:          row.Amount,
			PaymentModeId:   row.PaymentModeId,
		})
	}

	loaderResults := make([]*dataloader.Result[[]*models.BillPayment], 0, len(billIds))
	for _, id := range billIds {
		loaderResults = append(loaderResults, &dataloader.Result[[]*models.BillPayment]{Data: resultMap[id]})
	}
	return loaderResults
}

func GetBillPayments(ctx context.Context, billId int) ([]*models.BillPayment, error) {
	loaders := For(ctx)
	return loaders.billPaymentLoader.Load(ctx, billId)()
}
