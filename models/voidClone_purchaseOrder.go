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

// CancelAndClonePurchaseOrder cancels an existing confirmed purchase order and creates a new Draft purchase order cloned from it.
func CancelAndClonePurchaseOrder(ctx context.Context, businessId string, purchaseOrderId int) (*PurchaseOrder, error) {
	if businessId == "" {
		return nil, errors.New("business id is required")
	}
	if purchaseOrderId <= 0 {
		return nil, errors.New("purchase_order_id is required")
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

	var oldPO PurchaseOrder
	if err := tx.WithContext(ctx).
		Preload("Details").
		Where("business_id = ? AND id = ?", businessId, purchaseOrderId).
		First(&oldPO).Error; err != nil {
		tx.Rollback()
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, utils.ErrorRecordNotFound
		}
		return nil, err
	}

	if oldPO.CurrentStatus != PurchaseOrderStatusConfirmed {
		tx.Rollback()
		return nil, errors.New("only confirmed purchase orders can be cancel+cloned")
	}

	newDetails := make([]PurchaseOrderDetail, 0, len(oldPO.Details))
	for _, d := range oldPO.Details {
		d.ID = 0
		d.PurchaseOrderId = 0
		d.DetailBilledQty = decimal.NewFromInt(0)
		newDetails = append(newDetails, d)
	}

	seqNo, err := utils.GetSequence[PurchaseOrder](ctx, businessId)
	if err != nil {
		tx.Rollback()
		return nil, err
	}
	prefix, err := getTransactionPrefix(ctx, oldPO.BranchId, "Purchase Order")
	if err != nil {
		tx.Rollback()
		return nil, err
	}

	newPO := PurchaseOrder{
		BusinessId:                  oldPO.BusinessId,
		SupplierId:                  oldPO.SupplierId,
		BranchId:                    oldPO.BranchId,
		ReferenceNumber:             oldPO.ReferenceNumber,
		OrderDate:                   oldPO.OrderDate,
		ExpectedDeliveryDate:        oldPO.ExpectedDeliveryDate,
		OrderPaymentTerms:           oldPO.OrderPaymentTerms,
		OrderPaymentTermsCustomDays: oldPO.OrderPaymentTermsCustomDays,
		DeliveryWarehouseId:         oldPO.DeliveryWarehouseId,
		DeliveryCustomerId:          oldPO.DeliveryCustomerId,
		DeliveryAddress:             oldPO.DeliveryAddress,
		ShipmentPreferenceId:        oldPO.ShipmentPreferenceId,
		Notes:                       oldPO.Notes,
		TermsAndConditions:          oldPO.TermsAndConditions,
		CurrencyId:                  oldPO.CurrencyId,
		ExchangeRate:                oldPO.ExchangeRate,
		OrderDiscount:               oldPO.OrderDiscount,
		OrderDiscountType:           oldPO.OrderDiscountType,
		OrderDiscountAmount:         oldPO.OrderDiscountAmount,
		AdjustmentAmount:            oldPO.AdjustmentAmount,
		OrderTaxId:                  oldPO.OrderTaxId,
		OrderTaxType:                oldPO.OrderTaxType,
		OrderTaxAmount:              oldPO.OrderTaxAmount,
		IsTaxInclusive:              oldPO.IsTaxInclusive,
		WarehouseId:                 oldPO.WarehouseId,

		SequenceNo:     decimal.NewFromInt(seqNo),
		OrderNumber:    prefix + fmt.Sprint(seqNo),
		CurrentStatus:  PurchaseOrderStatusDraft,
		Details:        newDetails,
		OrderSubtotal:  oldPO.OrderSubtotal,
		OrderTotalDiscountAmount: oldPO.OrderTotalDiscountAmount,
		OrderTotalTaxAmount:      oldPO.OrderTotalTaxAmount,
		OrderTotalAmount:         oldPO.OrderTotalAmount,
	}

	if err := tx.WithContext(ctx).Create(&newPO).Error; err != nil {
		tx.Rollback()
		return nil, err
	}

	// Cancel old purchase order.
	if err := tx.WithContext(ctx).Model(&PurchaseOrder{}).
		Where("business_id = ? AND id = ?", businessId, oldPO.ID).
		Update("current_status", PurchaseOrderStatusCancelled).Error; err != nil {
		tx.Rollback()
		return nil, err
	}

	// Reverse ordered stock for Confirmed -> Cancelled.
	oldForTransition := oldPO
	oldForTransition.CurrentStatus = PurchaseOrderStatusCancelled
	if config.UseStockCommandsFor("PURCHASE_ORDER") {
		if err := ApplyPurchaseOrderStockForStatusTransition(tx.WithContext(ctx), &oldForTransition, PurchaseOrderStatusConfirmed); err != nil {
			tx.Rollback()
			return nil, err
		}
	} else {
		if err := oldForTransition.AfterUpdateCurrentStatus(tx.WithContext(ctx), string(PurchaseOrderStatusConfirmed)); err != nil {
			tx.Rollback()
			return nil, err
		}
	}

	if err := tx.Commit().Error; err != nil {
		return nil, err
	}
	return &newPO, nil
}

