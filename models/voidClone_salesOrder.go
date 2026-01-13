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

// CancelAndCloneSalesOrder cancels an existing confirmed sales order and creates a new Draft sales order cloned from it.
func CancelAndCloneSalesOrder(ctx context.Context, businessId string, salesOrderId int) (*SalesOrder, error) {
	if businessId == "" {
		return nil, errors.New("business id is required")
	}
	if salesOrderId <= 0 {
		return nil, errors.New("sales_order_id is required")
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

	var oldSO SalesOrder
	if err := tx.WithContext(ctx).
		Preload("Details").
		Where("business_id = ? AND id = ?", businessId, salesOrderId).
		First(&oldSO).Error; err != nil {
		tx.Rollback()
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, utils.ErrorRecordNotFound
		}
		return nil, err
	}

	if oldSO.CurrentStatus != SalesOrderStatusConfirmed {
		tx.Rollback()
		return nil, errors.New("only confirmed sales orders can be cancel+cloned")
	}

	newDetails := make([]SalesOrderDetail, 0, len(oldSO.Details))
	for _, d := range oldSO.Details {
		d.ID = 0
		d.SalesOrderId = 0
		d.DetailInvoicedQty = decimal.NewFromInt(0)
		newDetails = append(newDetails, d)
	}

	seqNo, err := utils.GetSequence[SalesOrder](ctx, businessId)
	if err != nil {
		tx.Rollback()
		return nil, err
	}
	prefix, err := getTransactionPrefix(ctx, oldSO.BranchId, "Sales Order")
	if err != nil {
		tx.Rollback()
		return nil, err
	}

	newSO := SalesOrder{
		BusinessId:                  oldSO.BusinessId,
		CustomerId:                  oldSO.CustomerId,
		BranchId:                    oldSO.BranchId,
		ReferenceNumber:             oldSO.ReferenceNumber,
		OrderDate:                   oldSO.OrderDate,
		ExpectedShipmentDate:        oldSO.ExpectedShipmentDate,
		OrderPaymentTerms:           oldSO.OrderPaymentTerms,
		OrderPaymentTermsCustomDays: oldSO.OrderPaymentTermsCustomDays,
		DeliveryMethodId:            oldSO.DeliveryMethodId,
		SalesPersonId:               oldSO.SalesPersonId,
		Notes:                       oldSO.Notes,
		TermsAndConditions:          oldSO.TermsAndConditions,
		CurrencyId:                  oldSO.CurrencyId,
		ExchangeRate:                oldSO.ExchangeRate,
		OrderDiscount:               oldSO.OrderDiscount,
		OrderDiscountType:           oldSO.OrderDiscountType,
		OrderDiscountAmount:         oldSO.OrderDiscountAmount,
		ShippingCharges:             oldSO.ShippingCharges,
		AdjustmentAmount:            oldSO.AdjustmentAmount,
		IsTaxInclusive:              oldSO.IsTaxInclusive,
		OrderTaxId:                  oldSO.OrderTaxId,
		OrderTaxType:                oldSO.OrderTaxType,
		OrderTaxAmount:              oldSO.OrderTaxAmount,
		WarehouseId:                 oldSO.WarehouseId,

		SequenceNo:     decimal.NewFromInt(seqNo),
		OrderNumber:    prefix + fmt.Sprint(seqNo),
		CurrentStatus:  SalesOrderStatusDraft,
		Details:        newDetails,
		OrderSubtotal:  oldSO.OrderSubtotal,
		OrderTotalDiscountAmount: oldSO.OrderTotalDiscountAmount,
		OrderTotalTaxAmount:      oldSO.OrderTotalTaxAmount,
		OrderTotalAmount:         oldSO.OrderTotalAmount,
	}

	if err := tx.WithContext(ctx).Create(&newSO).Error; err != nil {
		tx.Rollback()
		return nil, err
	}

	// Cancel old sales order.
	if err := tx.WithContext(ctx).Model(&SalesOrder{}).
		Where("business_id = ? AND id = ?", businessId, oldSO.ID).
		Update("current_status", SalesOrderStatusCancelled).Error; err != nil {
		tx.Rollback()
		return nil, err
	}

	// Reverse committed stock for Confirmed -> Cancelled.
	oldForTransition := oldSO
	oldForTransition.CurrentStatus = SalesOrderStatusCancelled
	if config.UseStockCommandsFor("SALES_ORDER") {
		if err := ApplySalesOrderStockForStatusTransition(tx.WithContext(ctx), &oldForTransition, SalesOrderStatusConfirmed); err != nil {
			tx.Rollback()
			return nil, err
		}
	} else {
		if err := oldForTransition.AfterUpdateCurrentStatus(tx.WithContext(ctx), string(SalesOrderStatusConfirmed)); err != nil {
			tx.Rollback()
			return nil, err
		}
	}

	if err := tx.Commit().Error; err != nil {
		return nil, err
	}
	return &newSO, nil
}

