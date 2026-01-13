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

// VoidAndCloneSupplierCredit voids an existing confirmed supplier credit and creates a new Draft supplier credit cloned from it.
func VoidAndCloneSupplierCredit(ctx context.Context, businessId string, supplierCreditId int) (*SupplierCredit, error) {
	if businessId == "" {
		return nil, errors.New("business id is required")
	}
	if supplierCreditId <= 0 {
		return nil, errors.New("supplier_credit_id is required")
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

	var oldSC SupplierCredit
	if err := tx.WithContext(ctx).
		Preload("Details").
		Where("business_id = ? AND id = ?", businessId, supplierCreditId).
		First(&oldSC).Error; err != nil {
		tx.Rollback()
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, utils.ErrorRecordNotFound
		}
		return nil, err
	}

	if oldSC.isApplied() {
		tx.Rollback()
		return nil, errors.New("cannot void+clone an applied/refunded supplier credit")
	}
	if oldSC.CurrentStatus != SupplierCreditStatusConfirmed {
		tx.Rollback()
		return nil, errors.New("only confirmed supplier credits can be void+cloned")
	}

	// Clone details.
	newDetails := make([]SupplierCreditDetail, 0, len(oldSC.Details))
	for _, d := range oldSC.Details {
		d.ID = 0
		d.SupplierCreditId = 0
		d.Cogs = decimal.NewFromInt(0)
		newDetails = append(newDetails, d)
	}

	seqNo, err := utils.GetSequence[SupplierCredit](ctx, businessId)
	if err != nil {
		tx.Rollback()
		return nil, err
	}
	prefix, err := getTransactionPrefix(ctx, oldSC.BranchId, "Supplier Credit")
	if err != nil {
		tx.Rollback()
		return nil, err
	}

	newSC := SupplierCredit{
		BusinessId:                   oldSC.BusinessId,
		SupplierId:                   oldSC.SupplierId,
		BranchId:                     oldSC.BranchId,
		ReferenceNumber:              oldSC.ReferenceNumber,
		SupplierCreditDate:           oldSC.SupplierCreditDate,
		SupplierCreditSubject:        oldSC.SupplierCreditSubject,
		Notes:                        oldSC.Notes,
		CurrencyId:                   oldSC.CurrencyId,
		ExchangeRate:                 oldSC.ExchangeRate,
		WarehouseId:                  oldSC.WarehouseId,
		SupplierCreditDiscount:       oldSC.SupplierCreditDiscount,
		SupplierCreditDiscountType:   oldSC.SupplierCreditDiscountType,
		SupplierCreditDiscountAmount: oldSC.SupplierCreditDiscountAmount,
		AdjustmentAmount:             oldSC.AdjustmentAmount,
		IsTaxInclusive:               oldSC.IsTaxInclusive,
		SupplierCreditTaxId:          oldSC.SupplierCreditTaxId,
		SupplierCreditTaxType:        oldSC.SupplierCreditTaxType,
		SupplierCreditTaxAmount:      oldSC.SupplierCreditTaxAmount,

		SequenceNo:           decimal.NewFromInt(seqNo),
		SupplierCreditNumber: prefix + fmt.Sprint(seqNo),
		CurrentStatus:        SupplierCreditStatusDraft,
		Details:              newDetails,

		SupplierCreditSubtotal:            oldSC.SupplierCreditSubtotal,
		SupplierCreditTotalDiscountAmount: oldSC.SupplierCreditTotalDiscountAmount,
		SupplierCreditTotalTaxAmount:      oldSC.SupplierCreditTotalTaxAmount,
		SupplierCreditTotalAmount:         oldSC.SupplierCreditTotalAmount,
		SupplierCreditTotalUsedAmount:     decimal.NewFromInt(0),
		RemainingBalance:                  oldSC.SupplierCreditTotalAmount,
	}

	if err := tx.WithContext(ctx).Create(&newSC).Error; err != nil {
		tx.Rollback()
		return nil, err
	}

	// Void old supplier credit.
	if err := tx.WithContext(ctx).Model(&SupplierCredit{}).
		Where("business_id = ? AND id = ?", businessId, oldSC.ID).
		Update("current_status", SupplierCreditStatusVoid).Error; err != nil {
		tx.Rollback()
		return nil, err
	}

	// Apply inventory reversal for Confirmed -> Void.
	oldForTransition := oldSC
	oldForTransition.CurrentStatus = SupplierCreditStatusVoid
	if config.UseStockCommandsFor("SUPPLIER_CREDIT") {
		if err := ApplySupplierCreditStockForStatusTransition(tx.WithContext(ctx), &oldForTransition, SupplierCreditStatusConfirmed); err != nil {
			tx.Rollback()
			return nil, err
		}
	} else {
		if err := oldForTransition.AfterUpdateCurrentStatus(tx.WithContext(ctx), string(SupplierCreditStatusConfirmed)); err != nil {
			tx.Rollback()
			return nil, err
		}
	}

	// Publish delete/void posting for old supplier credit (outbox record only).
	if err := PublishToAccounting(ctx, tx, businessId, oldSC.SupplierCreditDate, oldSC.ID, AccountReferenceTypeSupplierCredit, nil, oldSC, PubSubMessageActionDelete); err != nil {
		tx.Rollback()
		return nil, err
	}

	if err := tx.Commit().Error; err != nil {
		return nil, err
	}
	return &newSC, nil
}

