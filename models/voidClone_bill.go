package models

import (
	"context"
	"errors"
	"fmt"

	"bitbucket.org/mmdatafocus/books_backend/config"
	"bitbucket.org/mmdatafocus/books_backend/utils"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

// VoidAndCloneBill voids an existing confirmed bill and creates a new Draft bill cloned from it.
// The clone is created as Draft and does NOT publish posting until it is confirmed later.
func VoidAndCloneBill(ctx context.Context, businessId string, billId int) (*Bill, error) {
	if businessId == "" {
		return nil, errors.New("business id is required")
	}
	if billId <= 0 {
		return nil, errors.New("bill id is required")
	}
	if ctx == nil {
		return nil, errors.New("context is required")
	}

	db := config.GetDB()
	if db == nil {
		return nil, errors.New("db is nil")
	}
	ctx = utils.SetBusinessIdInContext(ctx, businessId)

	tx := db.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
			panic(r)
		}
	}()

	var oldBill Bill
	if err := tx.WithContext(ctx).
		Preload("Details").
		Where("business_id = ? AND id = ?", businessId, billId).
		First(&oldBill).Error; err != nil {
		tx.Rollback()
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, utils.ErrorRecordNotFound
		}
		return nil, err
	}

	if oldBill.CurrentStatus == BillStatusPartialPaid || oldBill.CurrentStatus == BillStatusPaid {
		tx.Rollback()
		return nil, errors.New("cannot void+clone a paid/partial paid bill")
	}
	if oldBill.CurrentStatus != BillStatusConfirmed {
		tx.Rollback()
		return nil, errors.New("only confirmed bills can be void+cloned")
	}

	// Ensure voiding won't make stock negative.
	if err := oldBill.ValidateStockQty(ctx, businessId); err != nil {
		tx.Rollback()
		return nil, err
	}

	// Clone details.
	newDetails := make([]BillDetail, 0, len(oldBill.Details))
	for _, d := range oldBill.Details {
		d.ID = 0
		d.BillId = 0
		newDetails = append(newDetails, d)
	}

	seqNo, err := utils.GetSequence[Bill](ctx, businessId)
	if err != nil {
		tx.Rollback()
		return nil, err
	}
	prefix, err := getTransactionPrefix(ctx, oldBill.BranchId, "Bill")
	if err != nil {
		tx.Rollback()
		return nil, err
	}

	newBill := Bill{
		BusinessId:          oldBill.BusinessId,
		SupplierId:          oldBill.SupplierId,
		BranchId:            oldBill.BranchId,
		PurchaseOrderId:     oldBill.PurchaseOrderId,
		PurchaseOrderNumber: oldBill.PurchaseOrderNumber,
		ReferenceNumber:     oldBill.ReferenceNumber,
		BillDate:            oldBill.BillDate,
		BillDueDate:         oldBill.BillDueDate,
		BillPaymentTerms:    oldBill.BillPaymentTerms,
		BillPaymentTermsCustomDays: oldBill.BillPaymentTermsCustomDays,
		BillSubject:         oldBill.BillSubject,
		Notes:               oldBill.Notes,
		CurrencyId:          oldBill.CurrencyId,
		ExchangeRate:        oldBill.ExchangeRate,
		WarehouseId:         oldBill.WarehouseId,
		BillDiscount:        oldBill.BillDiscount,
		BillDiscountType:    oldBill.BillDiscountType,
		BillDiscountAmount:  oldBill.BillDiscountAmount,
		AdjustmentAmount:    oldBill.AdjustmentAmount,
		BillTaxId:           oldBill.BillTaxId,
		BillTaxType:         oldBill.BillTaxType,
		BillTaxAmount:       oldBill.BillTaxAmount,
		IsTaxInclusive:      oldBill.IsTaxInclusive,

		SequenceNo:     decimal.NewFromInt(seqNo),
		BillNumber:     prefix + fmt.Sprint(seqNo),
		CurrentStatus:  BillStatusDraft,
		Details:        newDetails,
		BillSubtotal:   oldBill.BillSubtotal,
		BillTotalDiscountAmount: oldBill.BillTotalDiscountAmount,
		BillTotalTaxAmount:      oldBill.BillTotalTaxAmount,
		BillTotalAmount:         oldBill.BillTotalAmount,
		BillTotalPaidAmount:     decimal.NewFromInt(0),
		RemainingBalance:        oldBill.BillTotalAmount,
	}

	if err := tx.WithContext(ctx).Create(&newBill).Error; err != nil {
		tx.Rollback()
		return nil, err
	}

	// Void old bill.
	if err := tx.WithContext(ctx).Model(&Bill{}).
		Where("business_id = ? AND id = ?", businessId, oldBill.ID).
		Update("current_status", BillStatusVoid).Error; err != nil {
		tx.Rollback()
		return nil, err
	}

	// Reverse PO billed qty if linked.
	if oldBill.PurchaseOrderId > 0 {
		for _, item := range oldBill.Details {
			invAccId := 0
			if item.ProductId > 0 {
				p, err := GetProductOrVariant(ctx, string(item.ProductType), item.ProductId)
				if err != nil {
					tx.Rollback()
					return nil, err
				}
				invAccId = p.GetInventoryAccountID()
			}
			if err := UpdatePoDetailBilledQty(tx, ctx, oldBill.PurchaseOrderId, item, "delete", decimal.NewFromInt(0), invAccId); err != nil {
				tx.Rollback()
				return nil, err
			}
		}
		if _, err := ChangePoCurrentStatus(tx.WithContext(ctx), ctx, businessId, oldBill.PurchaseOrderId); err != nil {
			tx.Rollback()
			return nil, err
		}
	}

	// Apply inventory reversal for Confirmed -> Void.
	oldForTransition := oldBill
	oldForTransition.CurrentStatus = BillStatusVoid
	if config.UseStockCommandsFor("BILL") {
		if err := ApplyBillStockForStatusTransition(tx.WithContext(ctx), &oldForTransition, BillStatusConfirmed); err != nil {
			tx.Rollback()
			return nil, err
		}
	} else {
		if err := oldForTransition.AfterUpdateCurrentStatus(tx.WithContext(ctx), string(BillStatusConfirmed)); err != nil {
			tx.Rollback()
			return nil, err
		}
	}

	// Publish delete/void posting for old bill (outbox record only).
	if err := PublishToAccounting(ctx, tx, businessId, oldBill.BillDate, oldBill.ID, AccountReferenceTypeBill, nil, oldBill, PubSubMessageActionDelete); err != nil {
		tx.Rollback()
		return nil, err
	}

	if err := tx.Commit().Error; err != nil {
		return nil, err
	}
	return &newBill, nil
}

