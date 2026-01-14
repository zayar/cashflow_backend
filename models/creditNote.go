package models

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/mmdatafocus/books_backend/config"
	"github.com/mmdatafocus/books_backend/utils"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

type CreditNote struct {
	ID                            int                `gorm:"primary_key" json:"id"`
	BusinessId                    string             `gorm:"index;not null" json:"business_id" binding:"required"`
	CustomerId                    int                `gorm:"index;not null" json:"customer_id" binding:"required"`
	BranchId                      int                `gorm:"index;not null" json:"branch_id" binding:"required"`
	CreditNoteNumber              string             `gorm:"size:255;not null" json:"credit_note_number" binding:"required"`
	SequenceNo                    decimal.Decimal    `gorm:"type:decimal(15);not null" json:"sequence_no"`
	ReferenceNumber               string             `gorm:"size:255;default:null" json:"reference_number"`
	CreditNoteDate                time.Time          `gorm:"not null" json:"credit_note_date" binding:"required"`
	SalesPersonId                 int                `gorm:"default:null" json:"sales_person_id"`
	CreditNoteSubject             string             `gorm:"size:255;default:null" json:"credit_note_subject"`
	Notes                         string             `gorm:"type:text;default:null" json:"notes"`
	TermsAndConditions            string             `gorm:"type:text;default:null" json:"terms_and_conditions"`
	WarehouseId                   int                `gorm:"not null" json:"warehouse_id" binding:"required"`
	CurrencyId                    int                `gorm:"not null" json:"currency_id" binding:"required"`
	ExchangeRate                  decimal.Decimal    `gorm:"type:decimal(20,4);default:0" json:"exchange_rate"`
	CreditNoteDiscount            decimal.Decimal    `gorm:"type:decimal(20,4);default:0" json:"credit_note_discount"`
	CreditNoteDiscountType        *DiscountType      `gorm:"type:enum('P', 'A');default:null" json:"credit_note_discount_type"`
	CreditNoteDiscountAmount      decimal.Decimal    `gorm:"type:decimal(20,4);default:0" json:"credit_note_discount_amount"`
	ShippingCharges               decimal.Decimal    `gorm:"type:decimal(20,4);default:0" json:"shipping_charges"`
	AdjustmentAmount              decimal.Decimal    `gorm:"type:decimal(20,4);default:0" json:"adjustment_amount"`
	IsTaxInclusive                *bool              `gorm:"default:false" json:"is_tax_inclusive" binding:"required"`
	CreditNoteTaxId               int                `gorm:"default:Null" json:"credit_note_tax_id"`
	CreditNoteTaxType             *TaxType           `gorm:"type:enum('I', 'G');default:null" json:"credit_note_tax_type"`
	CreditNoteTaxAmount           decimal.Decimal    `gorm:"type:decimal(20,4);default:0" json:"credit_note_tax_amount"`
	CurrentStatus                 CreditNoteStatus   `gorm:"type:enum('Draft','Confirmed','Void','Closed');not null" json:"current_status" binding:"required"`
	Documents                     []*Document        `gorm:"polymorphic:Reference" json:"documents"`
	CreditNoteSubtotal            decimal.Decimal    `gorm:"type:decimal(20,4);default:0" json:"credit_note_subtotal"`
	CreditNoteTotalDiscountAmount decimal.Decimal    `gorm:"type:decimal(20,4);default:0" json:"credit_note_total_discount_amount"`
	CreditNoteTotalTaxAmount      decimal.Decimal    `gorm:"type:decimal(20,4);default:0" json:"credit_note_total_tax_amount"`
	CreditNoteTotalAmount         decimal.Decimal    `gorm:"type:decimal(20,4);default:0" json:"credit_note_total_amount"`
	CreditNoteTotalUsedAmount     decimal.Decimal    `gorm:"type:decimal(20,4);default:0" json:"credit_note_total_used_amount"`
	CreditNoteTotalRefundAmount   decimal.Decimal    `gorm:"type:decimal(20,4);default:0" json:"credit_note_total_refund_amount"`
	RemainingBalance              decimal.Decimal    `gorm:"type:decimal(20,4);default:0" json:"remaining_balance"`
	Details                       []CreditNoteDetail `gorm:"foreignKey:CreditNoteId" json:"details"`
	CreatedAt                     time.Time          `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt                     time.Time          `gorm:"autoUpdateTime" json:"updated_at"`
}

type NewCreditNote struct {
	CustomerId             int                   `json:"customer_id" binding:"required"`
	BranchId               int                   `json:"branch_id"`
	ReferenceNumber        string                `json:"reference_number"`
	CreditNoteDate         time.Time             `json:"credit_note_date" binding:"required"`
	SalesPersonId          int                   `json:"sales_person_id"`
	CreditNoteSubject      string                `json:"credit_note_subject"`
	Notes                  string                `json:"notes"`
	TermsAndConditions     string                `json:"terms_and_conditions"`
	CurrencyId             int                   `json:"currency_id" binding:"required"`
	ExchangeRate           decimal.Decimal       `json:"exchange_rate"`
	WarehouseId            int                   `json:"warehouse_id" binding:"required"`
	CreditNoteDiscount     decimal.Decimal       `json:"credit_note_discount"`
	CreditNoteDiscountType *DiscountType         `json:"credit_note_discount_type"`
	ShippingCharges        decimal.Decimal       `json:"shipping_charges"`
	AdjustmentAmount       decimal.Decimal       `json:"adjustment_amount"`
	IsTaxInclusive         *bool                 `json:"is_tax_inclusive" binding:"required"`
	CreditNoteTaxId        int                   `json:"credit_note_tax_id"`
	CreditNoteTaxType      *TaxType              `json:"credit_note_tax_type"`
	CurrentStatus          CreditNoteStatus      `json:"current_status" binding:"required"`
	Documents              []*NewDocument        `json:"documents"`
	Details                []NewCreditNoteDetail `json:"details"`
}

type CreditNoteDetail struct {
	ID                   int             `gorm:"primary_key" json:"id"`
	CreditNoteId         int             `gorm:"index;not null" json:"credit_note_id" binding:"required"`
	ProductId            int             `gorm:"index" json:"product_id"`
	ProductType          ProductType     `gorm:"type:enum('S','G','C','V','I');default:S" json:"product_type"`
	BatchNumber          string          `gorm:"size:100" json:"batch_number"`
	Name                 string          `gorm:"size:100" json:"name" binding:"required"`
	Description          string          `gorm:"size:255;default:null" json:"description"`
	DetailAccountId      int             `gorm:"default:null" json:"detail_account_id"`
	DetailQty            decimal.Decimal `gorm:"type:decimal(20,4);default:0" json:"detail_qty" binding:"required"`
	DetailUnitRate       decimal.Decimal `gorm:"type:decimal(20,4);default:0" json:"detail_unit_rate" binding:"required"`
	DetailTaxId          int             `gorm:"default:null" json:"detail_tax_id"`
	DetailTaxType        *TaxType        `gorm:"type:enum('I', 'G');default:null" json:"detail_tax_type"`
	DetailDiscount       decimal.Decimal `json:"detail_discount"`
	DetailDiscountType   *DiscountType   `gorm:"type:enum('P', 'A');default:null" json:"detail_discount_type"`
	DetailDiscountAmount decimal.Decimal `gorm:"type:decimal(20,4);default:0" json:"detail_discount_amount"`
	DetailTaxAmount      decimal.Decimal `gorm:"type:decimal(20,4);default:0" json:"detail_tax_amount"`
	DetailTotalAmount    decimal.Decimal `gorm:"type:decimal(20,4);default:0" json:"detail_total_amount"`
	Cogs                 decimal.Decimal `gorm:"type:decimal(20,4);default:0" json:"cogs"`
	CreatedAt            time.Time       `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt            time.Time       `gorm:"autoUpdateTime" json:"updated_at"`
}

type NewCreditNoteDetail struct {
	DetailId           int             `json:"detail_id"`
	ProductId          int             `json:"product_id"`
	ProductType        ProductType     `json:"product_type"`
	BatchNumber        string          `json:"batch_number"`
	Name               string          `json:"name" binding:"required"`
	Description        string          `json:"description"`
	DetailAccountId    int             `json:"detail_account_id"`
	DetailQty          decimal.Decimal `json:"detail_qty" binding:"required"`
	DetailUnitRate     decimal.Decimal `json:"detail_unit_rate" binding:"required"`
	DetailTaxId        int             `json:"detail_tax_id"`
	DetailTaxType      *TaxType        `json:"detail_tax_type"`
	IsTaxInclusive     *bool           `json:"is_tax_inclusive"`
	DetailDiscount     decimal.Decimal `json:"detail_discount"`
	DetailDiscountType *DiscountType   `json:"detail_discount_type"`
	IsDeletedItem      *bool           `json:"is_deleted_item"`
}

type CustomerCreditAdvance struct {
	ID               int                   `gorm:"primary_key" json:"id"`
	BusinessId       string                `gorm:"index;not null" json:"business_id" binding:"required"`
	Date             time.Time             `gorm:"not null" json:"date"`
	BranchId         int                   `gorm:"index" json:"branch_id"`
	CustomerId       int                   `gorm:"index" json:"customer_id"`
	Amount           decimal.Decimal       `gorm:"type:decimal(20,4);default:0" json:"amount"`
	UsedAmount       decimal.Decimal       `gorm:"type:decimal(20,4);default:0" json:"used_amount"`
	CurrencyId       int                   `gorm:"index;not null" json:"currency_id"`
	ExchangeRate     decimal.Decimal       `gorm:"type:decimal(20,4);default:0" json:"exchange_rate"`
	CurrentStatus    CustomerAdvanceStatus `gorm:"type:enum('Draft','Confirmed','Closed');not null" json:"current_status" binding:"required"`
	RefundAmount     decimal.Decimal       `gorm:"type:decimal(20,4);default:0" json:"credit_note_total_refund_amount"`
	RemainingBalance decimal.Decimal       `gorm:"type:decimal(20,4);default:0" json:"remaining_balance"`
	CreatedAt        time.Time             `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt        time.Time             `gorm:"autoUpdateTime" json:"updated_at"`
}

type CreditNotesConnection struct {
	Edges    []*CreditNotesEdge `json:"edges"`
	PageInfo *PageInfo          `json:"pageInfo"`
}

type CreditNotesEdge Edge[CreditNote]

func (obj CreditNote) GetId() int {
	return obj.ID
}

// returns decoded curosr string
func (cn CreditNote) GetCursor() string {
	return cn.CreatedAt.String()
}

func (c *CreditNote) GetRemainingBalance() decimal.Decimal {
	return c.RemainingBalance
}

func (c *CreditNote) AddRefundAmount(amount decimal.Decimal) error {
	if amount.GreaterThan(c.RemainingBalance) {
		return errors.New("amount must be less than or equal to remaining balance of credit note")
	}
	c.CreditNoteTotalRefundAmount = c.CreditNoteTotalRefundAmount.Add(amount)
	c.RemainingBalance = c.RemainingBalance.Sub(amount)
	return nil
}

func (c *CreditNote) UpdateStatus() error {
	if c.RemainingBalance.IsZero() {
		c.CurrentStatus = CreditNoteStatusClosed
	} else {
		c.CurrentStatus = CreditNoteStatusConfirmed
	}
	return nil
}

func (sc CreditNote) GetDueDate() time.Time {
	return sc.CreditNoteDate
}

func (c *CustomerCreditAdvance) GetId() int {
	return c.ID
}

func (c *CustomerCreditAdvance) GetRemainingBalance() decimal.Decimal {
	return c.RemainingBalance
}

func (c *CustomerCreditAdvance) AddRefundAmount(amount decimal.Decimal) error {
	if amount.GreaterThan(c.RemainingBalance) {
		return errors.New("amount must be less than or equal to remaining balance of customer advance")
	}
	c.RefundAmount = c.RefundAmount.Add(amount)
	c.RemainingBalance = c.RemainingBalance.Sub(amount)
	return nil
}

func (c *CustomerCreditAdvance) UpdateStatus() error {
	if c.RemainingBalance.IsZero() {
		c.CurrentStatus = CustomerAdvanceStatusClosed
	} else {
		c.CurrentStatus = CustomerAdvanceStatusConfirmed
	}
	return nil
}

func (c *CustomerCreditAdvance) useAmount(amount decimal.Decimal) error {

	if c.CurrentStatus != CustomerAdvanceStatusConfirmed {
		return errors.New("customer advance status must be confirm")
	}

	if c.RemainingBalance.LessThan(amount) {
		return errors.New("advacne remaining balance less than applied amount")
	}
	c.UsedAmount = c.UsedAmount.Add(amount)
	c.RemainingBalance = c.RemainingBalance.Sub(amount)
	if c.RemainingBalance.IsZero() {
		c.CurrentStatus = CustomerAdvanceStatusClosed
	}
	return nil
}

func (c *CustomerCreditAdvance) unUseAmount(amount decimal.Decimal) error {

	if c.CurrentStatus == CustomerAdvanceStatusClosed {
		c.CurrentStatus = CustomerAdvanceStatusConfirmed
	}

	c.UsedAmount = c.UsedAmount.Sub(amount)
	c.RemainingBalance = c.RemainingBalance.Add(amount)
	return nil
}

func (c CustomerCreditAdvance) isUsed() bool {
	return c.UsedAmount.GreaterThan(decimal.Zero)
}

func (cn *CreditNote) useAmount(amount decimal.Decimal) error {
	// use creditNote amount
	if cn.CurrentStatus != CreditNoteStatusConfirmed {
		return errors.New("credit note status must be confirm")
	}

	if cn.RemainingBalance.LessThan(amount) {
		return errors.New("credit remaining balance less than applied amount")
	}

	cn.CreditNoteTotalUsedAmount = cn.CreditNoteTotalUsedAmount.Add(amount)
	cn.RemainingBalance = cn.RemainingBalance.Sub(amount)
	if cn.RemainingBalance.IsZero() {
		cn.CurrentStatus = CreditNoteStatusClosed
	}
	return nil
}

func (cn *CreditNote) unUseAmount(amount decimal.Decimal) error {

	if cn.CurrentStatus == CreditNoteStatusClosed {
		cn.CurrentStatus = CreditNoteStatusConfirmed
	}

	cn.CreditNoteTotalUsedAmount = cn.CreditNoteTotalUsedAmount.Sub(amount)
	cn.RemainingBalance = cn.RemainingBalance.Add(amount)
	return nil
}

func (c *CustomerCreditAdvance) GetDueDate() time.Time {
	return c.Date
}

func (s *CreditNote) GetFieldValues(tx *gorm.DB) (*utils.DetailFieldValues, error) {
	return utils.FetchDetailFieldValues(tx, &CreditNoteDetail{}, "credit_note_id", s.ID)
}

// if credit note has been either applied to bill or refunded
func (cn CreditNote) isApplied() bool {
	return cn.CreditNoteTotalUsedAmount.GreaterThan(decimal.Zero) || cn.CreditNoteTotalRefundAmount.GreaterThan(decimal.Zero)
}

func (cn CreditNote) CheckTransactionLock(ctx context.Context) error {
	if err := validateTransactionLock(ctx, cn.CreditNoteDate, cn.BusinessId, SalesTransactionLock); err != nil {
		return err
	}
	// check for inventory value adjustment
	for _, detail := range cn.Details {
		if err := ValidateValueAdjustment(ctx, cn.BusinessId, cn.CreditNoteDate, detail.ProductType, detail.ProductId, &detail.BatchNumber); err != nil {
			return fmt.Errorf(err.Error(), detail.Name)
		}
	}
	return nil
}

func (ca CustomerCreditAdvance) CheckTransactionLock(ctx context.Context) error {
	return validateTransactionLock(ctx, ca.Date, ca.BusinessId, SalesTransactionLock)
}

func (input NewCreditNote) validate(ctx context.Context, businessId string, _ int) error {

	// exists customer
	if err := utils.ValidateResourceId[Customer](ctx, businessId, input.CustomerId); err != nil {
		return errors.New("customer not found")
	}
	// exists branch
	if err := utils.ValidateResourceId[Branch](ctx, businessId, input.BranchId); err != nil {
		return errors.New("branch not found")
	}
	// exists Currency
	if err := utils.ValidateResourceId[Currency](ctx, businessId, input.CurrencyId); err != nil {
		return errors.New("currency not found")
	}
	// exists wareshouse
	if input.WarehouseId > 0 {
		// exists warehouse
		if err := utils.ValidateResourceId[Warehouse](ctx, businessId, input.WarehouseId); err != nil {
			return errors.New("warehouse not found")
		}
	}
	// exists SalePerson
	if input.SalesPersonId > 0 {
		// exists SalesPerson
		if err := utils.ValidateResourceId[SalesPerson](ctx, businessId, input.SalesPersonId); err != nil {
			return errors.New("salesPerson not found")
		}
	}
	// validate creditNoteDate
	if err := validateTransactionLock(ctx, input.CreditNoteDate, businessId, SalesTransactionLock); err != nil {
		return err
	}
	for _, detail := range input.Details {
		if err := ValidateValueAdjustment(ctx, businessId, input.CreditNoteDate, detail.ProductType, detail.ProductId, &detail.BatchNumber); err != nil {
			return err
		}
	}

	return nil
}

func (item *CreditNoteDetail) CalculateSaleItemDiscountAndTax(ctx context.Context, isTaxInclusive bool) {

	db := config.GetDB()

	// calculate detail subtotal
	detailAmount := item.DetailQty.Mul(item.DetailUnitRate)
	// calculate discount amount
	var discountAmount decimal.Decimal

	if item.DetailDiscountType != nil {
		discountAmount = utils.CalculateDiscountAmount(detailAmount, item.DetailDiscount, string(*item.DetailDiscountType))
	}
	item.DetailDiscountAmount = discountAmount

	// Calculate subtotal amount
	item.DetailTotalAmount = item.DetailQty.Mul(item.DetailUnitRate).Sub(item.DetailDiscountAmount)

	var taxAmount decimal.Decimal
	if item.DetailTaxId > 0 {
		if *item.DetailTaxType == TaxTypeGroup {
			taxAmount = utils.CalculateTaxAmount(ctx, db, item.DetailTaxId, true, item.DetailTotalAmount, isTaxInclusive)
		} else {
			taxAmount = utils.CalculateTaxAmount(ctx, db, item.DetailTaxId, false, item.DetailTotalAmount, isTaxInclusive)
		}
	} else {
		taxAmount = decimal.NewFromFloat(0)
	}

	item.DetailTaxAmount = taxAmount
}

func updateCreditNoteItemDetailTotal(item *CreditNoteDetail, isTaxInclusive bool, orderSubtotal decimal.Decimal, totalExclusiveTaxAmount decimal.Decimal, totalDetailDiscountAmount decimal.Decimal, totalDetailTaxAmount decimal.Decimal) (decimal.Decimal, decimal.Decimal, decimal.Decimal, decimal.Decimal) {

	// var orderSubtotal, totalExclusiveTaxAmount, totalDetailDiscountAmount, totalDetailTaxAmount decimal.Decimal

	orderSubtotal = orderSubtotal.Add(item.DetailTotalAmount)
	totalDetailDiscountAmount = totalDetailDiscountAmount.Add(item.DetailDiscountAmount)
	totalDetailTaxAmount = totalDetailTaxAmount.Add(item.DetailTaxAmount)
	if isTaxInclusive {
		totalExclusiveTaxAmount = totalExclusiveTaxAmount.Add(decimal.NewFromFloat(0.0))
	} else {
		totalExclusiveTaxAmount = totalExclusiveTaxAmount.Add(item.DetailTaxAmount)
	}

	return orderSubtotal, totalDetailDiscountAmount, totalDetailTaxAmount, totalExclusiveTaxAmount
}

func CreateCreditNote(ctx context.Context, input *NewCreditNote) (*CreditNote, error) {
	db := config.GetDB()

	businessId, ok := utils.GetBusinessIdFromContext(ctx)
	if !ok || businessId == "" {
		return nil, errors.New("business id is required")
	}

	// IMPORTANT (correctness): if callers request "Confirmed" on create, we still create as Draft
	// and then transition Draft -> Confirmed inside the same DB transaction.
	requestedStatus := input.CurrentStatus

	// validate Credit Note
	if err := input.validate(ctx, businessId, 0); err != nil {
		return nil, err
	}

	// construct Images
	documents, err := mapNewDocuments(input.Documents, "credit_notes", 0)
	if err != nil {
		return nil, err
	}

	var creditNoteItems []CreditNoteDetail
	var creditNoteSubtotal,
		creditNoteTotalAmount,
		totalExclusiveTaxAmount,
		totalDetailDiscountAmount,
		totalDetailTaxAmount decimal.Decimal

	for _, item := range input.Details {
		creditNoteItem := CreditNoteDetail{
			ProductId:          item.ProductId,
			ProductType:        item.ProductType,
			BatchNumber:        item.BatchNumber,
			Name:               item.Name,
			Description:        item.Description,
			DetailAccountId:    item.DetailAccountId,
			DetailQty:          item.DetailQty,
			DetailUnitRate:     item.DetailUnitRate,
			DetailTaxId:        item.DetailTaxId,
			DetailTaxType:      item.DetailTaxType,
			DetailDiscount:     item.DetailDiscount,
			DetailDiscountType: item.DetailDiscountType,
		}

		// Calculate tax and total amounts for the item
		creditNoteItem.CalculateSaleItemDiscountAndTax(ctx, *input.IsTaxInclusive)

		creditNoteSubtotal = creditNoteSubtotal.Add(creditNoteItem.DetailTotalAmount)
		totalDetailDiscountAmount = totalDetailDiscountAmount.Add(creditNoteItem.DetailDiscountAmount)
		totalDetailTaxAmount = totalDetailTaxAmount.Add(creditNoteItem.DetailTaxAmount)

		if input.IsTaxInclusive != nil && *input.IsTaxInclusive {
			totalExclusiveTaxAmount = totalExclusiveTaxAmount.Add(decimal.NewFromFloat(0.0))
		} else {
			totalExclusiveTaxAmount = totalExclusiveTaxAmount.Add(creditNoteItem.DetailTaxAmount)
		}

		// Add the item to the PurchaseOrder
		creditNoteItems = append(creditNoteItems, creditNoteItem)

	}

	// calculate order discount
	var creditNoteDiscountAmount decimal.Decimal

	if input.CreditNoteDiscountType != nil {
		creditNoteDiscountAmount = utils.CalculateDiscountAmount(creditNoteSubtotal, input.CreditNoteDiscount, string(*input.CreditNoteDiscountType))
	}

	//	creditNoteSubtotal = creditNoteSubtotal.Sub(creditNoteDiscountAmount)

	// calculate order tax amount (always exclusive)
	var creditNoteTaxAmount decimal.Decimal
	if input.CreditNoteTaxId > 0 {
		if *input.CreditNoteTaxType == TaxTypeGroup {
			creditNoteTaxAmount = utils.CalculateTaxAmount(ctx, db, input.CreditNoteTaxId, true, creditNoteSubtotal, false)
		} else {
			creditNoteTaxAmount = utils.CalculateTaxAmount(ctx, db, input.CreditNoteTaxId, false, creditNoteSubtotal, false)
		}
	} else {
		creditNoteTaxAmount = decimal.NewFromFloat(0)
	}

	// Sum (order discount + total detail discount)
	totalCreditNoteDiscountAmount := creditNoteDiscountAmount.Add(totalDetailDiscountAmount)
	// Sum (order tax amount + total detail tax amount)
	totalCreditNoteTaxAmount := creditNoteTaxAmount.Add(totalDetailTaxAmount)

	creditNoteTotalAmount = creditNoteSubtotal.Add(creditNoteTaxAmount).Add(totalExclusiveTaxAmount).Add(input.AdjustmentAmount).Sub(creditNoteDiscountAmount)

	// store CreditNote
	creditNote := CreditNote{
		BusinessId:                    businessId,
		CustomerId:                    input.CustomerId,
		BranchId:                      input.BranchId,
		ReferenceNumber:               input.ReferenceNumber,
		CreditNoteDate:                input.CreditNoteDate,
		CreditNoteSubject:             input.CreditNoteSubject,
		WarehouseId:                   input.WarehouseId,
		SalesPersonId:                 input.SalesPersonId,
		Notes:                         input.Notes,
		TermsAndConditions:            input.TermsAndConditions,
		CurrencyId:                    input.CurrencyId,
		ExchangeRate:                  input.ExchangeRate,
		CreditNoteDiscount:            input.CreditNoteDiscount,
		CreditNoteDiscountType:        input.CreditNoteDiscountType,
		CreditNoteDiscountAmount:      creditNoteDiscountAmount,
		ShippingCharges:               input.ShippingCharges,
		AdjustmentAmount:              input.AdjustmentAmount,
		IsTaxInclusive:                input.IsTaxInclusive,
		CreditNoteTaxId:               input.CreditNoteTaxId,
		CreditNoteTaxType:             input.CreditNoteTaxType,
		CreditNoteTaxAmount:           creditNoteTaxAmount,
		CurrentStatus:                 CreditNoteStatusDraft,
		Documents:                     documents,
		Details:                       creditNoteItems,
		CreditNoteTotalDiscountAmount: totalCreditNoteDiscountAmount,
		CreditNoteTotalTaxAmount:      totalCreditNoteTaxAmount,
		CreditNoteSubtotal:            creditNoteSubtotal,
		CreditNoteTotalAmount:         creditNoteTotalAmount,
		RemainingBalance:              creditNoteTotalAmount,
	}

	tx := db.Begin()

	seqNo, err := utils.GetSequence[CreditNote](ctx, businessId)
	if err != nil {
		tx.Rollback()
		return nil, err
	}
	prefix, err := getTransactionPrefix(ctx, input.BranchId, "Credit Note")
	if err != nil {
		tx.Rollback()
		return nil, err
	}
	creditNote.SequenceNo = decimal.NewFromInt(seqNo)
	creditNote.CreditNoteNumber = prefix + fmt.Sprint(seqNo)

	err = tx.WithContext(ctx).Create(&creditNote).Error
	if err != nil {
		tx.Rollback()
		return nil, err
	}

	// If requested "Confirmed", apply the status transition deterministically (Draft -> Confirmed).
	if requestedStatus == CreditNoteStatusConfirmed {
		if err := tx.WithContext(ctx).Model(&creditNote).Update("CurrentStatus", CreditNoteStatusConfirmed).Error; err != nil {
			tx.Rollback()
			return nil, err
		}
		creditNote.CurrentStatus = CreditNoteStatusConfirmed

		// Apply inventory side-effects deterministically (prefer explicit command handler).
		if config.UseStockCommandsFor("CREDIT_NOTE") {
			if err := ApplyCreditNoteStockForStatusTransition(tx.WithContext(ctx), &creditNote, CreditNoteStatusDraft); err != nil {
				tx.Rollback()
				return nil, err
			}
		} else {
			if err := creditNote.AfterUpdateCurrentStatus(tx.WithContext(ctx), string(CreditNoteStatusDraft)); err != nil {
				tx.Rollback()
				return nil, err
			}
		}

		// Write outbox record (publishing happens after commit via dispatcher).
		if err := PublishToAccounting(ctx, tx, businessId, creditNote.CreditNoteDate, creditNote.ID, AccountReferenceTypeCreditNote, creditNote, nil, PubSubMessageActionCreate); err != nil {
			tx.Rollback()
			return nil, err
		}
	}

	if err := tx.Commit().Error; err != nil {
		return nil, err
	}

	return &creditNote, nil
}

func UpdateCreditNote(ctx context.Context, creditNoteId int, updatedCreditNote *NewCreditNote) (*CreditNote, error) {
	db := config.GetDB()
	businessId, ok := utils.GetBusinessIdFromContext(ctx)
	if !ok || businessId == "" {
		return nil, errors.New("business id is required")
	}

	// validate CreditNote
	if err := updatedCreditNote.validate(ctx, businessId, creditNoteId); err != nil {
		return nil, err
	}

	oldCreditNote, err := utils.FetchModelForChange[CreditNote](ctx, businessId, creditNoteId, "Details")
	if err != nil {
		return nil, err
	}
	// disallow update if applied to bill
	if oldCreditNote.isApplied() {
		return nil, errors.New("cannot update applied/refund credit note")
	}
	// Fintech integrity guardrail (behind flag): inventory-affecting docs are immutable after confirm.
	if config.StrictInventoryDocImmutability() && oldCreditNote.CurrentStatus == CreditNoteStatusConfirmed {
		return nil, errors.New("cannot edit a confirmed credit note; void and recreate to preserve inventory/valuation integrity")
	}
	// deep copy of oldCreditNote.Details
	existingDetails := append([]CreditNoteDetail(nil), oldCreditNote.Details...)
	var existingCreditNote CreditNote = *oldCreditNote

	oldStatus := existingCreditNote.CurrentStatus
	oldCreditNoteDate := utils.NilOrElse(updatedCreditNote.CreditNoteDate.Equal(existingCreditNote.CreditNoteDate), existingCreditNote.CreditNoteDate)
	oldWarehouseId := utils.NilOrElse(updatedCreditNote.WarehouseId == existingCreditNote.WarehouseId, existingCreditNote.WarehouseId)

	// Update the fields of the existing purchase order with the provided updated details
	existingCreditNote.CustomerId = updatedCreditNote.CustomerId
	existingCreditNote.BranchId = updatedCreditNote.BranchId
	existingCreditNote.ReferenceNumber = updatedCreditNote.ReferenceNumber
	existingCreditNote.CreditNoteDate = updatedCreditNote.CreditNoteDate
	existingCreditNote.CreditNoteSubject = updatedCreditNote.CreditNoteSubject
	existingCreditNote.WarehouseId = updatedCreditNote.WarehouseId
	existingCreditNote.SalesPersonId = updatedCreditNote.SalesPersonId
	existingCreditNote.Notes = updatedCreditNote.Notes
	existingCreditNote.TermsAndConditions = updatedCreditNote.TermsAndConditions
	existingCreditNote.CurrencyId = updatedCreditNote.CurrencyId
	existingCreditNote.ExchangeRate = updatedCreditNote.ExchangeRate
	existingCreditNote.CreditNoteDiscount = updatedCreditNote.CreditNoteDiscount
	existingCreditNote.CreditNoteDiscountType = updatedCreditNote.CreditNoteDiscountType
	existingCreditNote.ShippingCharges = updatedCreditNote.ShippingCharges
	existingCreditNote.AdjustmentAmount = updatedCreditNote.AdjustmentAmount
	existingCreditNote.IsTaxInclusive = updatedCreditNote.IsTaxInclusive
	existingCreditNote.CreditNoteTaxId = updatedCreditNote.CreditNoteTaxId
	existingCreditNote.CreditNoteTaxType = updatedCreditNote.CreditNoteTaxType
	existingCreditNote.CurrentStatus = updatedCreditNote.CurrentStatus

	var creditNoteSubtotal,
		creditNoteTotalAmount,
		totalExclusiveTaxAmount,
		totalDetailDiscountAmount,
		totalDetailTaxAmount decimal.Decimal

	tx := db.Begin()
	// Iterate through the updated items

	for _, updatedItem := range updatedCreditNote.Details {
		var existingItem *CreditNoteDetail

		// Check if the item already exists in the purchase order
		for _, item := range existingDetails {
			if item.ID == updatedItem.DetailId {
				existingItem = &item
				break
			}
		}

		// nil if product does not exist in DB
		var itemProduct *ProductInterface
		// If the item doesn't exist, add it to the purchase order
		if existingItem == nil {
			fmt.Println("is not existing- ")
			newItem := CreditNoteDetail{
				ProductId:          updatedItem.ProductId,
				ProductType:        updatedItem.ProductType,
				DetailAccountId:    updatedItem.DetailAccountId,
				BatchNumber:        updatedItem.BatchNumber,
				Name:               updatedItem.Name,
				Description:        updatedItem.Description,
				DetailQty:          updatedItem.DetailQty,
				DetailUnitRate:     updatedItem.DetailUnitRate,
				DetailTaxId:        updatedItem.DetailTaxId,
				DetailTaxType:      updatedItem.DetailTaxType,
				DetailDiscount:     updatedItem.DetailDiscount,
				DetailDiscountType: updatedItem.DetailDiscountType,
			}

			if updatedItem.ProductId > 0 {
				product, err := GetProductOrVariant(ctx, string(updatedItem.ProductType), updatedItem.ProductId)
				if err != nil {
					tx.Rollback()
					return nil, err
				}
				itemProduct = &product
			}
			// Calculate tax and total amounts for the item
			newItem.CalculateSaleItemDiscountAndTax(ctx, *updatedCreditNote.IsTaxInclusive)
			creditNoteSubtotal, totalDetailDiscountAmount, totalDetailTaxAmount, totalExclusiveTaxAmount = updateCreditNoteItemDetailTotal(&newItem, *updatedCreditNote.IsTaxInclusive, creditNoteSubtotal, totalExclusiveTaxAmount, totalDetailDiscountAmount, totalDetailTaxAmount)

			// append newly created detail to the slice
			existingCreditNote.Details = append(existingCreditNote.Details, newItem)

			if itemProduct != nil && (*itemProduct).GetInventoryAccountID() > 0 && existingCreditNote.CurrentStatus == CreditNoteStatusConfirmed {
				if err := UpdateStockSummaryReceivedQty(tx, businessId,
					existingCreditNote.WarehouseId,
					updatedItem.ProductId,
					string(updatedItem.ProductType),
					updatedItem.BatchNumber,
					updatedItem.DetailQty,
					existingCreditNote.CreditNoteDate); err != nil {
					tx.Rollback()
					return nil, err
				}
			}

		} else {

			if existingItem.ProductId > 0 {
				product, err := GetProductOrVariant(ctx, string(existingItem.ProductType), existingItem.ProductId)
				if err != nil {
					tx.Rollback()
					return nil, err
				}
				itemProduct = &product
			}

			if updatedItem.IsDeletedItem != nil && *updatedItem.IsDeletedItem {
				// Find the index of the item to delete
				var existingItemIndex int
				for i, detail := range existingCreditNote.Details {
					if updatedItem.DetailId == detail.ID {
						existingItemIndex = i
						break
					}
				}
				// Delete the item from the database
				if err := tx.WithContext(ctx).Delete(&existingCreditNote.Details[existingItemIndex]).Error; err != nil {
					tx.Rollback()
					return nil, err
				}

				// Remove the item from the slice
				existingCreditNote.Details = append(existingCreditNote.Details[:existingItemIndex], existingCreditNote.Details[existingItemIndex+1:]...)

				if err := ValidateProductStock(tx, ctx, businessId,
					existingCreditNote.WarehouseId, existingItem.BatchNumber, existingItem.ProductType, existingItem.ProductId, existingItem.DetailQty); err != nil {
					tx.Rollback()
					return nil, err
				}
				if itemProduct != nil && (*itemProduct).GetInventoryAccountID() > 0 && existingCreditNote.CurrentStatus == CreditNoteStatusConfirmed {
					if err := UpdateStockSummaryReceivedQty(tx, existingCreditNote.BusinessId, existingCreditNote.WarehouseId, existingItem.ProductId, string(existingItem.ProductType), existingItem.BatchNumber, existingItem.DetailQty.Neg(), existingCreditNote.CreditNoteDate); err != nil {
						tx.Rollback()
						return nil, err
					}
				}
			} else {
				// Update existing item details

				// nil if values don't change, old value otherwise
				oldDetailQty := utils.NilOrElse(existingItem.DetailQty.Equal(updatedItem.DetailQty), existingItem.DetailQty)
				oldBatchNumber := utils.NilOrElse(existingItem.BatchNumber == updatedItem.BatchNumber, existingItem.BatchNumber)

				// don't let product change
				// existingItem.ProductId = updatedItem.ProductId
				// existingItem.ProductType = updatedItem.ProductType
				existingItem.BatchNumber = updatedItem.BatchNumber
				existingItem.Name = updatedItem.Name
				existingItem.Description = updatedItem.Description
				existingItem.DetailAccountId = updatedItem.DetailAccountId
				existingItem.DetailQty = updatedItem.DetailQty
				existingItem.DetailUnitRate = updatedItem.DetailUnitRate
				existingItem.DetailTaxId = updatedItem.DetailTaxId
				existingItem.DetailTaxType = updatedItem.DetailTaxType
				existingItem.DetailDiscount = updatedItem.DetailDiscount
				existingItem.DetailDiscountType = updatedItem.DetailDiscountType

				// Calculate tax and total amounts for the item
				existingItem.CalculateSaleItemDiscountAndTax(ctx, *updatedCreditNote.IsTaxInclusive)
				creditNoteSubtotal, totalDetailDiscountAmount, totalDetailTaxAmount, totalExclusiveTaxAmount = updateCreditNoteItemDetailTotal(existingItem, *updatedCreditNote.IsTaxInclusive, creditNoteSubtotal, totalExclusiveTaxAmount, totalDetailDiscountAmount, totalDetailTaxAmount)
				// existingCreditNote.Details = append(existingCreditNote.Details, *existingItem)

				if err := tx.WithContext(ctx).Save(&existingItem).Error; err != nil {
					tx.Rollback()
					return nil, err
				}

				if itemProduct != nil && (*itemProduct).GetInventoryAccountID() > 0 && updatedCreditNote.CurrentStatus == CreditNoteStatusConfirmed {
					if oldStatus == CreditNoteStatusDraft {
						// newly confirmed
						// no need to check for old values to subtract since new
						if err := UpdateStockSummaryReceivedQty(tx, businessId,
							existingCreditNote.WarehouseId,
							existingItem.ProductId,
							string(existingItem.ProductType),
							existingItem.BatchNumber,
							existingItem.DetailQty,
							existingCreditNote.CreditNoteDate); err != nil {
							tx.Rollback()
							return nil, err
						}

					} else if oldStatus == CreditNoteStatusConfirmed {
						if oldCreditNoteDate != nil || oldWarehouseId != nil || oldBatchNumber != nil {
							// if belongs to different stock_summary_daily_balance record

							// subtract old values from old record
							if err := UpdateStockSummaryReceivedQty(tx, businessId,
								utils.DereferencePtr(oldWarehouseId, existingCreditNote.WarehouseId),
								existingItem.ProductId,
								string(existingItem.ProductType),
								utils.DereferencePtr(oldBatchNumber, existingItem.BatchNumber),
								utils.DereferencePtr(oldDetailQty, existingItem.DetailQty).Neg(),
								utils.DereferencePtr(oldCreditNoteDate, existingCreditNote.CreditNoteDate)); err != nil {
								tx.Rollback()
								return nil, err
							}
							// create new record with new values
							if err := UpdateStockSummaryReceivedQty(tx, businessId,
								existingCreditNote.WarehouseId,
								existingItem.ProductId,
								string(existingItem.ProductType),
								existingItem.BatchNumber,
								existingItem.DetailQty,
								existingCreditNote.CreditNoteDate); err != nil {
								tx.Rollback()
								return nil, err
							}

							// post validation in case new qty is less than old qty, stock_qty becomes negative

							if oldWarehouseId != nil || oldBatchNumber != nil {
								// validate previous record if moved to a new stock_summary
								if _, err := GetProductStock(tx, ctx, businessId,
									utils.DereferencePtr(oldWarehouseId, existingCreditNote.WarehouseId),
									utils.DereferencePtr(oldBatchNumber, existingItem.BatchNumber),
									existingItem.ProductType, existingItem.ProductId); err != nil {
									tx.Rollback()
									return nil, err
								}
							}
							if _, err := GetProductStock(tx, ctx, businessId,
								existingCreditNote.WarehouseId, existingItem.BatchNumber, existingItem.ProductType, existingItem.ProductId); err != nil {
								tx.Rollback()
								return nil, err
							}
						} else if oldDetailQty != nil {
							addedQty := existingItem.DetailQty.Sub(*oldDetailQty)
							if addedQty.IsNegative() {
								// validate if some qty is removed
								if err := ValidateProductStock(tx, ctx, businessId,
									existingCreditNote.WarehouseId,
									existingItem.BatchNumber,
									existingItem.ProductType,
									existingItem.ProductId,
									addedQty.Abs()); err != nil {
									tx.Rollback()
									return nil, err
								}
							}
							if err := UpdateStockSummaryReceivedQty(tx, businessId,
								existingCreditNote.WarehouseId,
								existingItem.ProductId,
								string(existingItem.ProductType),
								existingItem.BatchNumber,
								addedQty,
								existingCreditNote.CreditNoteDate); err != nil {
								tx.Rollback()
								return nil, err
							}
						}
					}
				}
			}
		}
	}

	// calculate order discount
	var creditNoteDiscountAmount decimal.Decimal

	if updatedCreditNote.CreditNoteDiscountType != nil {
		creditNoteDiscountAmount = utils.CalculateDiscountAmount(creditNoteSubtotal, updatedCreditNote.CreditNoteDiscount, string(*updatedCreditNote.CreditNoteDiscountType))
	}

	existingCreditNote.CreditNoteDiscountAmount = creditNoteDiscountAmount

	// creditNoteSubtotal = creditNoteSubtotal.Sub(creditNoteDiscountAmount)
	existingCreditNote.CreditNoteSubtotal = creditNoteSubtotal

	// calculate order tax amount (always exclusive)
	var creditNoteTaxAmount decimal.Decimal
	if updatedCreditNote.CreditNoteTaxId > 0 {
		if *updatedCreditNote.CreditNoteTaxType == TaxTypeGroup {
			creditNoteTaxAmount = utils.CalculateTaxAmount(ctx, db, updatedCreditNote.CreditNoteTaxId, true, creditNoteSubtotal, false)
		} else {
			creditNoteTaxAmount = utils.CalculateTaxAmount(ctx, db, updatedCreditNote.CreditNoteTaxId, false, creditNoteSubtotal, false)
		}
	} else {
		creditNoteTaxAmount = decimal.NewFromFloat(0)
	}

	existingCreditNote.CreditNoteTaxAmount = creditNoteTaxAmount

	// Sum (order discount + total detail discount)
	totalCreditNoteDiscountAmount := creditNoteDiscountAmount.Add(totalDetailDiscountAmount)
	existingCreditNote.CreditNoteTotalDiscountAmount = totalCreditNoteDiscountAmount

	// Sum (order tax amount + total detail tax amount)
	totalCreditNoteTaxAmount := creditNoteTaxAmount.Add(totalDetailTaxAmount)
	existingCreditNote.CreditNoteTotalTaxAmount = totalCreditNoteTaxAmount
	// Sum Grand total amount (subtotal+ exclusive tax + adj amount)
	creditNoteTotalAmount = creditNoteSubtotal.Add(creditNoteTaxAmount).Add(totalExclusiveTaxAmount).Add(updatedCreditNote.AdjustmentAmount).Sub(creditNoteDiscountAmount)

	existingCreditNote.CreditNoteTotalAmount = creditNoteTotalAmount
	existingCreditNote.RemainingBalance = creditNoteTotalAmount

	// Save the updated purchase order to the database
	if err := tx.WithContext(ctx).Save(&existingCreditNote).Error; err != nil {
		tx.Rollback()
		return nil, err
	}

	// Refresh the existingInvoice to get the latest details
	if err := tx.WithContext(ctx).Preload("Details").First(&existingCreditNote, creditNoteId).Error; err != nil {
		tx.Rollback()
		return nil, err
	}

	if oldStatus == CreditNoteStatusDraft && existingCreditNote.CurrentStatus == CreditNoteStatusConfirmed {
		err := PublishToAccounting(ctx, tx, businessId, existingCreditNote.CreditNoteDate, existingCreditNote.ID, AccountReferenceTypeCreditNote, existingCreditNote, nil, PubSubMessageActionCreate)
		if err != nil {
			tx.Rollback()
			return nil, err
		}
	} else if oldStatus == CreditNoteStatusConfirmed && existingCreditNote.CurrentStatus == CreditNoteStatusConfirmed {
		err := PublishToAccounting(ctx, tx, businessId, existingCreditNote.CreditNoteDate, existingCreditNote.ID, AccountReferenceTypeCreditNote, existingCreditNote, oldCreditNote, PubSubMessageActionUpdate)
		if err != nil {
			tx.Rollback()
			return nil, err
		}
	}

	documents, err := upsertDocuments(ctx, tx, updatedCreditNote.Documents, "credit_notes", creditNoteId)
	if err != nil {
		tx.Rollback()
		return nil, err
	}
	existingCreditNote.Documents = documents

	if err := tx.Commit().Error; err != nil {
		return nil, err
	}

	return &existingCreditNote, nil
}

func DeleteCreditNote(ctx context.Context, id int) (*CreditNote, error) {
	businessId, ok := utils.GetBusinessIdFromContext(ctx)
	if !ok || businessId == "" {
		return nil, errors.New("business id is required")
	}

	db := config.GetDB()
	tx := db.Begin()
	oldCreditNote, err := utils.FetchModelForChange[CreditNote](ctx, businessId, id, "Details", "Documents")
	if err != nil {
		return nil, err
	}
	// disallow delete if credit note has been applied or refunded
	if oldCreditNote.isApplied() {
		return nil, errors.New("cannot delete applied/refunded credit note")
	}
	var result CreditNote = *oldCreditNote

	// reduced received qty from stock summary if sale order is confirmed
	if result.CurrentStatus == CreditNoteStatusConfirmed {
		for _, item := range result.Details {
			if item.ProductId > 0 {
				product, err := GetProductOrVariant(ctx, string(item.ProductType), item.ProductId)
				if err != nil {
					tx.Rollback()
					return nil, err
				}
				if product.GetInventoryAccountID() > 0 {
					if err := ValidateProductStock(tx, ctx, businessId, result.WarehouseId, item.BatchNumber, item.ProductType, item.ProductId, item.DetailQty); err != nil {
						tx.Rollback()
						return nil, err
					}
					if err := UpdateStockSummaryReceivedQty(tx, result.BusinessId, result.WarehouseId, item.ProductId, string(item.ProductType), item.BatchNumber, item.DetailQty.Neg(), result.CreditNoteDate); err != nil {
						tx.Rollback()
						return nil, err
					}
				}
			}
		}

		var creditInvoices []CustomerCreditInvoice

		err := tx.WithContext(ctx).Where("business_id = ?", businessId).
			Where("reference_id = ?", id).
			Where("reference_type = ?", CustomerCreditApplyTypeCredit).
			Find(&creditInvoices).Error
		if err != nil {
			tx.Rollback()
			return nil, err
		}

		// Adjust InvoiceTotalCreditAmount for each associated PaidInvoice
		for _, creditInvoice := range creditInvoices {
			invoice, err := utils.FetchModelForChange[SalesInvoice](ctx, businessId, creditInvoice.InvoiceId)
			if err != nil {
				tx.Rollback()
				return nil, err
			}

			invoicePaymentAmount := invoice.InvoiceTotalPaidAmount
			invoiceAdvanceAmount := invoice.InvoiceTotalAdvanceUsedAmount
			invoiceCreditAmount := invoice.InvoiceTotalCreditUsedAmount
			remainingPaidAmount := invoicePaymentAmount.Add(invoiceAdvanceAmount).Add(invoiceCreditAmount).Sub(creditInvoice.Amount)

			if remainingPaidAmount.IsNegative() {
				tx.Rollback()
				return nil, errors.New("resulting BillTotalPaidAmount cannot be negative")
			}
			if remainingPaidAmount.GreaterThan(decimal.Zero) {
				invoice.CurrentStatus = SalesInvoiceStatusPartialPaid
			} else {
				invoice.CurrentStatus = SalesInvoiceStatusConfirmed
			}
			invoice.InvoiceTotalCreditUsedAmount = invoiceCreditAmount.Sub(creditInvoice.Amount)
			invoice.RemainingBalance = invoice.RemainingBalance.Add(creditInvoice.Amount)

			// Update the invoice
			if err := tx.WithContext(ctx).Save(&invoice).Error; err != nil {
				tx.Rollback()
				return nil, err
			}

			err = tx.WithContext(ctx).Delete(&creditInvoice).Error
			if err != nil {
				tx.Rollback()
				return nil, err
			}
		}
	}

	err = tx.WithContext(ctx).Model(&result).Association("Details").Unscoped().Clear()
	if err != nil {
		tx.Rollback()
		return nil, err
	}

	err = tx.WithContext(ctx).Delete(&result).Error
	if err != nil {
		tx.Rollback()
		return nil, err
	}

	if result.CurrentStatus == CreditNoteStatusConfirmed {
		err = PublishToAccounting(ctx, tx, businessId, oldCreditNote.CreditNoteDate, oldCreditNote.ID, AccountReferenceTypeCreditNote, nil, oldCreditNote, PubSubMessageActionDelete)
		if err != nil {
			tx.Rollback()
			return nil, err
		}
	}

	if err := deleteDocuments(ctx, tx, oldCreditNote.Documents); err != nil {
		tx.Rollback()
		return nil, err
	}

	if err := tx.Commit().Error; err != nil {
		return nil, err
	}

	return oldCreditNote, nil

}

func UpdateStatusCreditNote(ctx context.Context, id int, status string) (*CreditNote, error) {
	businessId, ok := utils.GetBusinessIdFromContext(ctx)
	if !ok || businessId == "" {
		return nil, errors.New("business id is required")
	}
	// db action
	db := config.GetDB()
	tx := db.Begin()

	creditNote, err := utils.FetchModelForChange[CreditNote](ctx, businessId, id, "Details")
	if err != nil {
		return nil, err
	}

	oldStatus := creditNote.CurrentStatus

	err = tx.WithContext(ctx).Model(&creditNote).Updates(map[string]interface{}{
		"CurrentStatus": status,
	}).Error

	if err != nil {
		tx.Rollback()
		return nil, err
	}

	// Apply inventory side-effects deterministically (prefer explicit command handler).
	if config.UseStockCommandsFor("CREDIT_NOTE") {
		creditNote.CurrentStatus = CreditNoteStatus(status)
		if err := ApplyCreditNoteStockForStatusTransition(tx.WithContext(ctx), creditNote, oldStatus); err != nil {
			tx.Rollback()
			return nil, err
		}
	} else {
		if err := creditNote.AfterUpdateCurrentStatus(tx.WithContext(ctx), string(oldStatus)); err != nil {
			tx.Rollback()
			return nil, err
		}
	}

	if oldStatus == CreditNoteStatusDraft && status == string(CreditNoteStatusConfirmed) {
		err := PublishToAccounting(ctx, tx, businessId, creditNote.CreditNoteDate, creditNote.ID, AccountReferenceTypeCreditNote, creditNote, nil, PubSubMessageActionCreate)
		if err != nil {
			tx.Rollback()
			return nil, err
		}
	} else if oldStatus == CreditNoteStatusConfirmed && status == string(CreditNoteStatusVoid) {
		err = PublishToAccounting(ctx, tx, businessId, creditNote.CreditNoteDate, creditNote.ID, AccountReferenceTypeCreditNote, nil, creditNote, PubSubMessageActionDelete)
		if err != nil {
			tx.Rollback()
			return nil, err
		}
	}

	// Commit the transaction
	if err := tx.Commit().Error; err != nil {
		return nil, err
	}

	return creditNote, nil
}

func GetCreditNote(ctx context.Context, id int) (*CreditNote, error) {
	db := config.GetDB()

	var result CreditNote
	err := db.WithContext(ctx).First(&result, id).Error
	if err != nil {
		return nil, err
	}

	return &result, nil
}

func GetConfirmedCreditNote(tx *gorm.DB, ctx context.Context, id int, businessId string) (*CreditNote, error) {
	var result CreditNote
	err := tx.WithContext(ctx).
		Where("business_id = ?", businessId).
		Not("current_status IN (?)", []string{string(CreditNoteStatusDraft), string(CreditNoteStatusVoid)}).
		// Where("current_status = ?", CreditNoteStatusConfirmed).
		First(&result, id).Error
	if err != nil {
		tx.Rollback()
		return nil, errors.New("reference id (for credit note) not found")
	}
	return &result, nil
}

func GetConfirmedCustomerAdvance(tx *gorm.DB, ctx context.Context, id int, businessId string) (*CustomerCreditAdvance, error) {
	var result CustomerCreditAdvance
	err := tx.WithContext(ctx).
		Where("business_id = ?", businessId).
		Not("current_status = ?", CustomerAdvanceStatusDraft).
		First(&result, id).Error
	if err != nil {
		tx.Rollback()
		return nil, errors.New("reference id (for customer advance) not found")
	}
	return &result, nil
}

func GetCreditNotes(ctx context.Context, customerId *int, creditNoteNumber *string) ([]*CreditNote, error) {
	db := config.GetDB()
	var results []*CreditNote

	businessId, ok := utils.GetBusinessIdFromContext(ctx)
	if !ok || businessId == "" {
		return nil, errors.New("business id is required")
	}

	dbCtx := db.WithContext(ctx).Where("business_id = ?", businessId)

	if creditNoteNumber != nil && len(*creditNoteNumber) > 0 {
		dbCtx = dbCtx.Where("credit_note_number LIKE ?", "%"+*creditNoteNumber+"%")
	}

	if customerId != nil && *customerId != 0 && *customerId > 0 {
		dbCtx = dbCtx.Where("customer_id =?", customerId)
	}

	err := dbCtx.
		Find(&results).Error
	if err != nil {
		return nil, err
	}
	return results, nil
}

func PaginateCreditNote(ctx context.Context, limit *int, after *string,
	creditNoteNumber *string,
	referenceNumber *string,
	branchID *int,
	warehouseID *int,
	customerID *int,
	status *CreditNoteStatus,
	startCreditNoteDate *MyDateString,
	endCreditNoteDate *MyDateString) (*CreditNotesConnection, error) {

	businessId, ok := utils.GetBusinessIdFromContext(ctx)
	if !ok || businessId == "" {
		return nil, errors.New("business id is required")
	}

	business, err := GetBusiness(ctx)
	if err != nil {
		return nil, errors.New("business id is required")
	}
	if err := startCreditNoteDate.StartOfDayUTCTime(business.Timezone); err != nil {
		return nil, err
	}
	if err := endCreditNoteDate.EndOfDayUTCTime(business.Timezone); err != nil {
		return nil, err
	}

	db := config.GetDB()
	dbCtx := db.WithContext(ctx).Where("business_id = ?", businessId)

	if creditNoteNumber != nil && *creditNoteNumber != "" {
		dbCtx.Where("credit_note_number LIKE ?", "%"+*creditNoteNumber+"%")
	}
	if referenceNumber != nil && *referenceNumber != "" {
		dbCtx.Where("reference_number LIKE ?", "%"+*referenceNumber+"%")
	}
	if branchID != nil && *branchID > 0 {
		dbCtx.Where("branch_id = ?", *branchID)
	}
	if warehouseID != nil && *warehouseID > 0 {
		dbCtx.Where("warehouse_id = ?", *warehouseID)
	}
	if customerID != nil && *customerID > 0 {
		dbCtx.Where("customer_id = ?", *customerID)
	}
	if status != nil {
		dbCtx.Where("current_status = ?", *status)
	}
	if startCreditNoteDate != nil && endCreditNoteDate != nil {
		dbCtx.Where("credit_note_date BETWEEN ? AND ?", startCreditNoteDate, endCreditNoteDate)
	}

	edges, pageInfo, err := FetchPageCompositeCursor[CreditNote](dbCtx, *limit, after, "created_at", "<")
	if err != nil {
		return nil, err
	}
	var creditNotesConnection CreditNotesConnection
	creditNotesConnection.PageInfo = pageInfo
	for _, edge := range edges {
		creditNotesEdge := CreditNotesEdge(edge)
		creditNotesConnection.Edges = append(creditNotesConnection.Edges, &creditNotesEdge)
	}

	return &creditNotesConnection, err
}

func GetUnusedCustomerCredits(ctx context.Context, branchId int, customerId int) ([]*CreditNote, error) {
	// db := config.GetDB()
	var results []*CreditNote

	businessId, ok := utils.GetBusinessIdFromContext(ctx)
	if !ok || businessId == "" {
		return nil, errors.New("business id is required")
	}

	db := config.GetDB()
	dbCtx := db.WithContext(ctx).Where("business_id = ?", businessId)

	if branchId > 0 {
		dbCtx.Where("branch_id = ?", branchId)
	}
	err := dbCtx.WithContext(ctx).
		Where("customer_id = ?", customerId).
		Where("current_status = ?", CreditNoteStatusConfirmed).
		Find(&results).Error

	if err != nil {
		return nil, err
	}
	return results, nil
}

func GetUnusedCustomerCreditAdvances(ctx context.Context, branchId int, customerId int) ([]*CustomerCreditAdvance, error) {
	var results []*CustomerCreditAdvance
	db := config.GetDB()
	dbCtx := db.WithContext(ctx).
		Where("customer_id = ?", customerId)
	if branchId > 0 {
		dbCtx.Where("branch_id = ?", branchId)
	}
	err := dbCtx.Where("current_status = ? AND remaining_balance > 0", CustomerAdvanceStatusConfirmed).
		Find(&results).Error

	if err != nil {
		return nil, err
	}
	return results, nil
}
