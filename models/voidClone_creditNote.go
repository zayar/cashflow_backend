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

// VoidAndCloneCreditNote voids an existing confirmed credit note and creates a new Draft credit note cloned from it.
func VoidAndCloneCreditNote(ctx context.Context, businessId string, creditNoteId int) (*CreditNote, error) {
	if businessId == "" {
		return nil, errors.New("business id is required")
	}
	if creditNoteId <= 0 {
		return nil, errors.New("credit_note_id is required")
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

	var oldCN CreditNote
	if err := tx.WithContext(ctx).
		Preload("Details").
		Where("business_id = ? AND id = ?", businessId, creditNoteId).
		First(&oldCN).Error; err != nil {
		tx.Rollback()
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, utils.ErrorRecordNotFound
		}
		return nil, err
	}

	if oldCN.isApplied() {
		tx.Rollback()
		return nil, errors.New("cannot void+clone an applied/refund credit note")
	}
	if oldCN.CurrentStatus != CreditNoteStatusConfirmed {
		tx.Rollback()
		return nil, errors.New("only confirmed credit notes can be void+cloned")
	}

	newDetails := make([]CreditNoteDetail, 0, len(oldCN.Details))
	for _, d := range oldCN.Details {
		d.ID = 0
		d.CreditNoteId = 0
		newDetails = append(newDetails, d)
	}

	seqNo, err := utils.GetSequence[CreditNote](ctx, businessId)
	if err != nil {
		tx.Rollback()
		return nil, err
	}
	prefix, err := getTransactionPrefix(ctx, oldCN.BranchId, "Credit Note")
	if err != nil {
		tx.Rollback()
		return nil, err
	}

	newCN := CreditNote{
		BusinessId:               oldCN.BusinessId,
		CustomerId:               oldCN.CustomerId,
		BranchId:                 oldCN.BranchId,
		ReferenceNumber:          oldCN.ReferenceNumber,
		CreditNoteDate:           oldCN.CreditNoteDate,
		SalesPersonId:            oldCN.SalesPersonId,
		CreditNoteSubject:        oldCN.CreditNoteSubject,
		Notes:                    oldCN.Notes,
		TermsAndConditions:       oldCN.TermsAndConditions,
		WarehouseId:              oldCN.WarehouseId,
		CurrencyId:               oldCN.CurrencyId,
		ExchangeRate:             oldCN.ExchangeRate,
		CreditNoteDiscount:       oldCN.CreditNoteDiscount,
		CreditNoteDiscountType:   oldCN.CreditNoteDiscountType,
		CreditNoteDiscountAmount: oldCN.CreditNoteDiscountAmount,
		ShippingCharges:          oldCN.ShippingCharges,
		AdjustmentAmount:         oldCN.AdjustmentAmount,
		IsTaxInclusive:           oldCN.IsTaxInclusive,
		CreditNoteTaxId:          oldCN.CreditNoteTaxId,
		CreditNoteTaxType:        oldCN.CreditNoteTaxType,
		CreditNoteTaxAmount:      oldCN.CreditNoteTaxAmount,

		SequenceNo:       decimal.NewFromInt(seqNo),
		CreditNoteNumber: prefix + fmt.Sprint(seqNo),
		CurrentStatus:    CreditNoteStatusDraft,
		Details:          newDetails,

		CreditNoteSubtotal:            oldCN.CreditNoteSubtotal,
		CreditNoteTotalDiscountAmount: oldCN.CreditNoteTotalDiscountAmount,
		CreditNoteTotalTaxAmount:      oldCN.CreditNoteTotalTaxAmount,
		CreditNoteTotalAmount:         oldCN.CreditNoteTotalAmount,
		CreditNoteTotalUsedAmount:     decimal.NewFromInt(0),
		RemainingBalance:              oldCN.CreditNoteTotalAmount,
	}

	if err := tx.WithContext(ctx).Create(&newCN).Error; err != nil {
		tx.Rollback()
		return nil, err
	}

	// Void old credit note.
	if err := tx.WithContext(ctx).Model(&CreditNote{}).
		Where("business_id = ? AND id = ?", businessId, oldCN.ID).
		Update("current_status", CreditNoteStatusVoid).Error; err != nil {
		tx.Rollback()
		return nil, err
	}

	// Apply inventory reversal for Confirmed -> Void.
	oldForTransition := oldCN
	oldForTransition.CurrentStatus = CreditNoteStatusVoid
	if config.UseStockCommandsFor("CREDIT_NOTE") {
		if err := ApplyCreditNoteStockForStatusTransition(tx.WithContext(ctx), &oldForTransition, CreditNoteStatusConfirmed); err != nil {
			tx.Rollback()
			return nil, err
		}
	} else {
		if err := oldForTransition.AfterUpdateCurrentStatus(tx.WithContext(ctx), string(CreditNoteStatusConfirmed)); err != nil {
			tx.Rollback()
			return nil, err
		}
	}

	// Publish delete/void posting for old credit note (outbox record only).
	if err := PublishToAccounting(ctx, tx, businessId, oldCN.CreditNoteDate, oldCN.ID, AccountReferenceTypeCreditNote, nil, oldCN, PubSubMessageActionDelete); err != nil {
		tx.Rollback()
		return nil, err
	}

	if err := tx.Commit().Error; err != nil {
		return nil, err
	}
	return &newCN, nil
}

