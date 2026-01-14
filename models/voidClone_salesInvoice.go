package models

import (
	"context"
	"errors"
	"fmt"

	"github.com/mmdatafocus/books_backend/config"
	"github.com/mmdatafocus/books_backend/utils"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

// VoidAndCloneSalesInvoice voids an existing confirmed invoice and creates a new Draft invoice cloned from it.
//
// This is the recommended "void + recreate" workflow for fintech integrity once inventory-affecting docs are immutable.
// The clone is created as Draft (so users can edit safely) and does NOT publish posting until it is confirmed later.
//
// Returns the newly created draft invoice.
func VoidAndCloneSalesInvoice(ctx context.Context, businessId string, invoiceId int) (*SalesInvoice, error) {
	if businessId == "" {
		return nil, errors.New("business id is required")
	}
	if invoiceId <= 0 {
		return nil, errors.New("invoice id is required")
	}
	if ctx == nil {
		return nil, errors.New("context is required")
	}

	db := config.GetDB()
	if db == nil {
		return nil, errors.New("db is nil")
	}

	// Ensure downstream helpers (prefix/sequence) have businessId in context.
	ctx = utils.SetBusinessIdInContext(ctx, businessId)

	tx := db.Begin()
	defer func() {
		// Safety: if caller forgets to handle errors, ensure tx is not left open.
		if r := recover(); r != nil {
			tx.Rollback()
			panic(r)
		}
	}()

	var oldInv SalesInvoice
	if err := tx.WithContext(ctx).
		Preload("Details").
		Where("business_id = ? AND id = ?", businessId, invoiceId).
		First(&oldInv).Error; err != nil {
		tx.Rollback()
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, utils.ErrorRecordNotFound
		}
		return nil, err
	}

	// Guardrails: do not auto-void paid invoices.
	if oldInv.CurrentStatus == SalesInvoiceStatusPartialPaid || oldInv.CurrentStatus == SalesInvoiceStatusPaid {
		tx.Rollback()
		return nil, errors.New("cannot void+clone a paid/partial paid invoice; refund/credit workflow required")
	}
	if oldInv.CurrentStatus != SalesInvoiceStatusConfirmed {
		tx.Rollback()
		return nil, errors.New("only confirmed invoices can be void+cloned")
	}

	// Clone details (reset IDs so GORM inserts new rows).
	newDetails := make([]SalesInvoiceDetail, 0, len(oldInv.Details))
	for _, d := range oldInv.Details {
		d.ID = 0
		d.SalesInvoiceId = 0
		d.StockId = 0
		// keep Cogs=0; COGS will be determined when confirmed + posted
		d.Cogs = decimal.NewFromInt(0)
		newDetails = append(newDetails, d)
	}

	seqNo, err := utils.GetSequence[SalesInvoice](ctx, businessId)
	if err != nil {
		tx.Rollback()
		return nil, err
	}
	prefix, err := getTransactionPrefix(ctx, oldInv.BranchId, "Invoice")
	if err != nil {
		tx.Rollback()
		return nil, err
	}

	newInv := SalesInvoice{
		BusinessId:                    oldInv.BusinessId,
		CustomerId:                    oldInv.CustomerId,
		BranchId:                      oldInv.BranchId,
		SalesOrderId:                  oldInv.SalesOrderId,
		OrderNumber:                   oldInv.OrderNumber,
		ReferenceNumber:               oldInv.ReferenceNumber,
		InvoiceDate:                   oldInv.InvoiceDate,
		InvoicePaymentTerms:           oldInv.InvoicePaymentTerms,
		InvoicePaymentTermsCustomDays: oldInv.InvoicePaymentTermsCustomDays,
		InvoiceDueDate:                oldInv.InvoiceDueDate,
		SalesPersonId:                 oldInv.SalesPersonId,
		InvoiceSubject:                oldInv.InvoiceSubject,
		Notes:                         oldInv.Notes,
		TermsAndConditions:            oldInv.TermsAndConditions,
		CurrencyId:                    oldInv.CurrencyId,
		ExchangeRate:                  oldInv.ExchangeRate,
		WarehouseId:                   oldInv.WarehouseId,
		InvoiceDiscount:               oldInv.InvoiceDiscount,
		InvoiceDiscountType:           oldInv.InvoiceDiscountType,
		InvoiceDiscountAmount:         oldInv.InvoiceDiscountAmount,
		ShippingCharges:               oldInv.ShippingCharges,
		AdjustmentAmount:              oldInv.AdjustmentAmount,
		IsTaxInclusive:                oldInv.IsTaxInclusive,
		InvoiceTaxId:                  oldInv.InvoiceTaxId,
		InvoiceTaxType:                oldInv.InvoiceTaxType,
		InvoiceTaxAmount:              oldInv.InvoiceTaxAmount,

		SequenceNo:      decimal.NewFromInt(seqNo),
		InvoiceNumber:   prefix + fmt.Sprint(seqNo),
		CurrentStatus:   SalesInvoiceStatusDraft,
		Details:         newDetails,
		InvoiceSubtotal: oldInv.InvoiceSubtotal,

		InvoiceTotalDiscountAmount: oldInv.InvoiceTotalDiscountAmount,
		InvoiceTotalTaxAmount:      oldInv.InvoiceTotalTaxAmount,
		InvoiceTotalAmount:         oldInv.InvoiceTotalAmount,
		// New draft clone starts unpaid.
		InvoiceTotalPaidAmount:        decimal.NewFromInt(0),
		InvoiceTotalCreditUsedAmount:  decimal.NewFromInt(0),
		InvoiceTotalAdvanceUsedAmount: decimal.NewFromInt(0),
		InvoiceTotalWriteOffAmount:    decimal.NewFromInt(0),
		RemainingBalance:              oldInv.InvoiceTotalAmount,
	}

	if err := tx.WithContext(ctx).Create(&newInv).Error; err != nil {
		tx.Rollback()
		return nil, err
	}

	// Void old invoice inside same transaction, reversing stock immediately (using command handlers).
	if err := tx.WithContext(ctx).Model(&SalesInvoice{}).
		Where("business_id = ? AND id = ?", businessId, oldInv.ID).
		Update("current_status", SalesInvoiceStatusVoid).Error; err != nil {
		tx.Rollback()
		return nil, err
	}

	// Apply inventory reversal for Confirmed -> Void.
	oldForTransition := oldInv
	oldForTransition.CurrentStatus = SalesInvoiceStatusVoid
	if config.UseStockCommandsFor("SALES_INVOICE") {
		if err := ApplySalesInvoiceStockForStatusTransition(tx.WithContext(ctx), &oldForTransition, SalesInvoiceStatusConfirmed); err != nil {
			tx.Rollback()
			return nil, err
		}
	} else {
		if err := oldForTransition.AfterUpdateCurrentStatus(tx.WithContext(ctx), string(SalesInvoiceStatusConfirmed)); err != nil {
			tx.Rollback()
			return nil, err
		}
	}

	// Publish delete/void posting for old invoice (outbox record only; publish happens after commit).
	if err := PublishToAccounting(ctx, tx, businessId, oldInv.InvoiceDate, oldInv.ID, AccountReferenceTypeInvoice, nil, oldInv, PubSubMessageActionDelete); err != nil {
		tx.Rollback()
		return nil, err
	}

	if err := tx.Commit().Error; err != nil {
		return nil, err
	}
	return &newInv, nil
}
