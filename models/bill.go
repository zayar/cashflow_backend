package models

import (
	"context"
	"errors"
	"fmt"
	"time"

	"bitbucket.org/mmdatafocus/books_backend/config"
	"bitbucket.org/mmdatafocus/books_backend/utils"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

type Bill struct {
	ID                         int             `gorm:"primary_key" json:"id"`
	BusinessId                 string          `gorm:"index;not null" json:"business_id" binding:"required"`
	SupplierId                 int             `gorm:"index;not null" json:"supplier_id" binding:"required"`
	BranchId                   int             `gorm:"index;not null" json:"branch_id"`
	PurchaseOrderId            int             `gorm:"index;default:null" json:"purchase_order_id"`
	PurchaseOrderNumber        string          `gorm:"size:255" json:"purchase_order_number"`
	BillNumber                 string          `gorm:"size:255;not null" json:"bill_number" binding:"required"`
	SequenceNo                 decimal.Decimal `gorm:"type:decimal(15);not null" json:"sequence_no"`
	ReferenceNumber            string          `gorm:"size:255;default:null" json:"reference_number"`
	BillDate                   time.Time       `gorm:"not null" json:"bill_date" binding:"required"`
	BillPaymentTerms           PaymentTerms    `gorm:"type:enum('Net15', 'Net30', 'Net45', 'Net60', 'DueMonthEnd', 'DueNextMonthEnd', 'DueOnReceipt', 'Custom');not null" json:"bill_payment_terms" binding:"required"`
	BillPaymentTermsCustomDays int             `gorm:"default:0" json:"bill_payment_terms_custom_days"`
	BillDueDate                *time.Time      `gorm:"default:null" json:"bill_due_date"`
	BillSubject                string          `gorm:"size:255;default:null" json:"bill_subject"`
	Notes                      string          `gorm:"type:text;default:null" json:"notes"`
	CurrencyId                 int             `gorm:"not null" json:"currency_id" binding:"required"`
	ExchangeRate               decimal.Decimal `gorm:"type:decimal(20,4);default:0" json:"exchange_rate"`
	BillDiscount               decimal.Decimal `gorm:"type:decimal(20,4);default:0" json:"bill_discount"`
	BillDiscountType           *DiscountType   `gorm:"type:enum('P', 'A');default:null" json:"bill_discount_type"`
	BillDiscountAmount         decimal.Decimal `gorm:"type:decimal(20,4);default:0" json:"bill_discount_amount"`
	AdjustmentAmount           decimal.Decimal `gorm:"type:decimal(20,4);default:0" json:"adjustment_amount"`
	IsTaxInclusive             *bool           `gorm:"not null;default:false" json:"is_tax_inclusive"`
	BillTaxId                  int             `gorm:"default:null" json:"bill_tax_id"`
	BillTaxType                *TaxType        `gorm:"type:enum('I', 'G');default:null" json:"bill_tax_type"`
	BillTaxAmount              decimal.Decimal `gorm:"type:decimal(20,4);default:0" json:"bill_tax_amount"`
	CurrentStatus              BillStatus      `gorm:"type:enum('Draft', 'Confirmed','Void', 'Partial Paid', 'Paid');default:Draft" json:"current_status" binding:"required"`
	Documents                  []*Document     `gorm:"polymorphic:Reference" json:"documents"`
	WarehouseId                int             `gorm:"not null" json:"warehouse_id"`
	BillSubtotal               decimal.Decimal `gorm:"type:decimal(20,4);default:0" json:"bill_subtotal"`
	BillTotalDiscountAmount    decimal.Decimal `gorm:"type:decimal(20,4);default:0" json:"bill_total_discount_amount"`
	BillTotalTaxAmount         decimal.Decimal `gorm:"type:decimal(20,4);default:0" json:"bill_total_tax_amount"`
	BillTotalAmount            decimal.Decimal `gorm:"type:decimal(20,4);default:0" json:"bill_total_amount"`
	BillTotalPaidAmount        decimal.Decimal `gorm:"type:decimal(20,4);default:0" json:"bill_total_paid_amount"`
	BillTotalCreditUsedAmount  decimal.Decimal `gorm:"type:decimal(20,4);default:0" json:"bill_total_credit_used_amount"`
	BillTotalAdvanceUsedAmount decimal.Decimal `gorm:"type:decimal(20,4);default:0" json:"bill_total_advance_used_amount"`
	RemainingBalance           decimal.Decimal `gorm:"type:decimal(20,4);default:0" json:"remaining_balance"`
	Details                    []BillDetail    `json:"bill_details" validate:"required,dive,required"`
	CreatedAt                  time.Time       `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt                  time.Time       `gorm:"autoUpdateTime" json:"updated_at"`
}

type NewBill struct {
	SupplierId          int    `json:"supplier_id" binding:"required"`
	BranchId            int    `json:"branch_id" binding:"required"`
	PurchaseOrderNumber string `json:"purchase_order_number"`
	// BillNumber       string          `json:"bill_number" binding:"required"`
	ReferenceNumber            string          `json:"reference_number"`
	BillDate                   time.Time       `json:"bill_date" binding:"required"`
	BillPaymentTerms           PaymentTerms    `json:"bill_payment_terms" binding:"required"`
	BillPaymentTermsCustomDays int             `json:"bill_payment_terms_custom_days"`
	BillSubject                string          `json:"bill_subject"`
	Notes                      string          `json:"notes"`
	CurrencyId                 int             `json:"currency_id" binding:"required"`
	ExchangeRate               decimal.Decimal `json:"exchange_rate"`
	BillDiscount               decimal.Decimal `json:"bill_discount"`
	BillDiscountType           *DiscountType   `json:"bill_discount_type"`
	AdjustmentAmount           decimal.Decimal `json:"adjustment_amount"`
	IsTaxInclusive             *bool           `json:"is_tax_inclusive" binding:"required"`
	BillTaxId                  int             `json:"bill_tax_id"`
	BillTaxType                *TaxType        `json:"bill_tax_type"`
	CurrentStatus              BillStatus      `json:"current_status" binding:"required"`
	Documents                  []*NewDocument  `json:"documents"`
	WarehouseId                int             `json:"warehouse_id" binding:"required"`
	Details                    []NewBillDetail `json:"details"`
}

type BillDetail struct {
	ID                   int             `gorm:"primary_key" json:"id"`
	BillId               int             `gorm:"index;not null" json:"bill_id" binding:"required"`
	ProductId            int             `gorm:"index" json:"product_id"`
	ProductType          ProductType     `gorm:"type:enum('S','G','C','V','I');default:S" json:"product_type"`
	BatchNumber          string          `gorm:"size:100" json:"batch_number"`
	Name                 string          `gorm:"size:100" json:"name" binding:"required"`
	Description          string          `gorm:"size:255;default:null" json:"description"`
	CustomerId           int             `gorm:"default:null" json:"customer_id"`
	DetailAccountId      int             `gorm:"default:null" json:"detail_account_id"`
	DetailQty            decimal.Decimal `gorm:"type:decimal(20,4);default:0" json:"detail_qty" binding:"required"`
	DetailUnitRate       decimal.Decimal `gorm:"type:decimal(20,4);default:0" json:"detail_unit_rate" binding:"required"`
	DetailTaxId          int             `gorm:"default:null" json:"detail_tax_id"`
	DetailTaxType        *TaxType        `gorm:"type:enum('I', 'G');default:null" json:"detail_tax_type"`
	DetailDiscount       decimal.Decimal `gorm:"default:0" json:"detail_discount"`
	DetailDiscountType   *DiscountType   `gorm:"type:enum('P', 'A');default:null" json:"detail_discount_type"`
	DetailDiscountAmount decimal.Decimal `gorm:"type:decimal(20,4);default:0" json:"detail_discount_amount"`
	DetailTaxAmount      decimal.Decimal `gorm:"type:decimal(20,4);default:0" json:"detail_tax_amount"`
	DetailTotalAmount    decimal.Decimal `gorm:"type:decimal(20,4);default:0" json:"detail_total_amount"`
	PurchaseOrderItemId  int             `gorm:"index" json:"purchase_order_item_id"`
	CreatedAt            time.Time       `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt            time.Time       `gorm:"autoUpdateTime" json:"updated_at"`
}

type NewBillDetail struct {
	DetailId            int             `json:"detail_id"`
	ProductId           int             `json:"product_id"`
	ProductType         ProductType     `json:"product_type"`
	BatchNumber         string          `json:"batch_number"`
	Name                string          `json:"name" binding:"required"`
	Description         string          `json:"description"`
	CustomerId          int             `json:"customer_id"`
	DetailAccountId     int             `json:"detail_account_id"`
	DetailQty           decimal.Decimal `json:"detail_qty" binding:"required"`
	DetailUnitRate      decimal.Decimal `json:"detail_unit_rate" binding:"required"`
	DetailTaxId         int             `json:"detail_tax_id"`
	DetailTaxType       *TaxType        `json:"detail_tax_type"`
	DetailDiscount      decimal.Decimal `json:"detail_discount"`
	DetailDiscountType  *DiscountType   `json:"detail_discount_type"`
	IsDeletedItem       *bool           `json:"is_deleted_item"`
	PurchaseOrderItemId int             `json:"purchase_order_item_id"`
}

type BillsConnection struct {
	Edges            []*BillsEdge     `json:"edges"`
	PageInfo         *PageInfo        `json:"pageInfo"`
	BillTotalSummary BillTotalSummary `json:"billTotalSummary"`
}

type BillTotalSummary struct {
	TotalOutstandingPayable decimal.Decimal `json:"total_outstanding_payable"`
	DueToday                decimal.Decimal `json:"due_today"`
	DueWithin30Days         decimal.Decimal `json:"due_within_30_days"`
	TotalOverdue            decimal.Decimal `json:"total_overdue"`
}

type BillsEdge Edge[Bill]

func (b Bill) CheckTransactionLock(ctx context.Context) error {

	// don't check date for supplier opening balance
	if b.BillNumber == "Supplier Opening Balance" {
		return nil
	}

	if err := validateTransactionLock(ctx, b.BillDate, b.BusinessId, PurchaseTransactionLock); err != nil {
		return err
	}

	// check for inventory value adjustment
	for _, detail := range b.Details {
		if err := ValidateValueAdjustment(ctx, b.BusinessId, b.BillDate, detail.ProductType, detail.ProductId, &detail.BatchNumber); err != nil {
			return fmt.Errorf(err.Error(), detail.Name)
		}
	}
	return nil
}

// returns decoded curosr string
func (b Bill) GetCursor() string {
	return b.CreatedAt.String()
}

// func (input *NewBillDetail) validate(ctx context.Context, businessId string, id int) error {
// 	if id > 0 {
// 		if err := utils.ValidateResourceId[BillDetail](ctx, businessId, id); err != nil {
// 			return err
// 		}
// 	}
// 	// validate product
// 	if err := validateProductId(ctx, businessId, input.ProductId, input.ProductType); err != nil {
// 		return err
// 	}
// 	// // validate batch number
// 	// if err := utils.ValidateUnique[BillDetail](ctx, businessId, "batch_number", input.BatchNumber, id); err != nil {
// 	// 	return err
// 	// }
// 	// validate tax
// 	if input.DetailTaxId > 0 {
// 		if err := validateTaxExists(ctx, businessId, input.DetailTaxId, *input.DetailTaxType); err != nil {
// 			return err
// 		}
// 	}

// 	return nil
// }

// GetID method for Bill reference Data
func (b *Bill) GetID() int {
	return b.ID
}

func (bill *Bill) GetFieldValues(tx *gorm.DB) (*utils.DetailFieldValues, error) {
	return utils.FetchDetailFieldValues(tx, &BillDetail{}, "bill_id", bill.ID)
}

func (item *BillDetail) CalculateItemDiscountAndTax(ctx context.Context, isTaxInclusive bool) {

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

func updateBillDetailTotal(item *BillDetail, isTaxInclusive bool, orderSubtotal decimal.Decimal, totalExclusiveTaxAmount decimal.Decimal, totalDetailDiscountAmount decimal.Decimal, totalDetailTaxAmount decimal.Decimal) (decimal.Decimal, decimal.Decimal, decimal.Decimal, decimal.Decimal) {

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

func (input NewBill) validate(ctx context.Context, businessId string, _ int) error {

	// exists supplier
	if err := utils.ValidateResourceId[Supplier](ctx, businessId, input.SupplierId); err != nil {
		return errors.New("supplier not found")
	}
	// exists branch
	if err := utils.ValidateResourceId[Branch](ctx, businessId, input.BranchId); err != nil {
		return errors.New("branch not found")
	}
	// exists wareshouse
	if input.WarehouseId > 0 {
		// exists warehouse
		if err := utils.ValidateResourceId[Warehouse](ctx, businessId, input.WarehouseId); err != nil {
			return errors.New("warehouse not found")
		}
	}
	// exists tax
	if input.BillTaxType != nil {
		if err := validateTaxExists(ctx, businessId, input.BillTaxId, *input.BillTaxType); err != nil {
			return errors.New("tax not found")
		}
	}

	// validate billDate
	if err := validateTransactionLock(ctx, input.BillDate, businessId, PurchaseTransactionLock); err != nil {
		return err
	}
	// validate each product for inventory adjustment date
	for _, inputDetail := range input.Details {
		if err := ValidateValueAdjustment(ctx, businessId, input.BillDate, inputDetail.ProductType, inputDetail.ProductId, &inputDetail.BatchNumber); err != nil {
			return err
		}
	}

	return nil
}

func (bill Bill) ValidateStockQty(ctx context.Context, businessId string) error {
	for _, billItem := range bill.Details {
		if billItem.ProductId > 0 {
			product, err := GetProductOrVariant(ctx, string(billItem.ProductType), billItem.ProductId)
			if err != nil {
				return err
			}
			if product.GetInventoryAccountID() > 0 {
				db := config.GetDB()
				currentQty, err := GetProductStock(db, ctx, businessId, bill.WarehouseId, billItem.BatchNumber, billItem.ProductType, billItem.ProductId)
				if err != nil {
					return err
				}

				if currentQty.Sub(billItem.DetailQty).LessThan(decimal.NewFromFloat(0)) {
					return errors.New("stock qty cannot accept negative")
				}
			}
		}
	}
	return nil
}

func CreateBill(ctx context.Context, input *NewBill) (*Bill, error) {

	businessId, ok := utils.GetBusinessIdFromContext(ctx)
	if !ok || businessId == "" {
		return nil, errors.New("business id is required")
	}

	// IMPORTANT (correctness): if callers request "Confirmed" on create, we still create as Draft
	// and then transition Draft -> Confirmed inside the same DB transaction.
	// This ensures stock movements happen through the same status-transition path everywhere.
	requestedStatus := input.CurrentStatus

	if err := input.validate(ctx, businessId, 0); err != nil {
		return nil, err
	}

	db := config.GetDB()
	var po PurchaseOrder
	purchaseOrderId := 0
	if len(input.PurchaseOrderNumber) > 0 {
		// exists purchase order
		err := db.Where("business_id = ? AND order_number = ?", businessId, input.PurchaseOrderNumber).First(&po).Error
		if err != nil {
			return nil, errors.New("purchase order not found")
		}
		purchaseOrderId = po.ID
	}

	// construct Images
	documents, err := mapNewDocuments(input.Documents, "bills", 0)
	if err != nil {
		return nil, err
	}

	tx := db.Begin()
	// IMPORTANT: always rollback on early-return or panic to avoid leaking DB locks
	// (leaked transactions are a common cause of MySQL 1205 lock wait timeouts).
	defer func() {
		if r := recover(); r != nil {
			_ = tx.Rollback().Error
			panic(r)
		}
	}()
	defer func() { _ = tx.Rollback().Error }()

	var billItems []BillDetail
	var billSubtotal,
		billTotalAmount,
		totalExclusiveTaxAmount,
		totalDetailDiscountAmount,
		totalDetailTaxAmount decimal.Decimal

	for _, item := range input.Details {

		billItem := BillDetail{
			ProductId:           item.ProductId,
			ProductType:         item.ProductType,
			BatchNumber:         item.BatchNumber,
			Name:                item.Name,
			Description:         item.Description,
			DetailAccountId:     item.DetailAccountId,
			CustomerId:          item.CustomerId,
			DetailQty:           item.DetailQty,
			DetailUnitRate:      item.DetailUnitRate,
			DetailTaxId:         item.DetailTaxId,
			DetailTaxType:       item.DetailTaxType,
			DetailDiscount:      item.DetailDiscount,
			DetailDiscountType:  item.DetailDiscountType,
			PurchaseOrderItemId: item.PurchaseOrderItemId,
		}

		// Calculate tax and total amounts for the item
		billItem.CalculateItemDiscountAndTax(ctx, *input.IsTaxInclusive)

		billSubtotal = billSubtotal.Add(billItem.DetailTotalAmount)
		totalDetailDiscountAmount = totalDetailDiscountAmount.Add(billItem.DetailDiscountAmount)
		totalDetailTaxAmount = totalDetailTaxAmount.Add(billItem.DetailTaxAmount)

		if input.IsTaxInclusive != nil && *input.IsTaxInclusive {
			totalExclusiveTaxAmount = totalExclusiveTaxAmount.Add(decimal.NewFromFloat(0.0))
		} else {
			totalExclusiveTaxAmount = totalExclusiveTaxAmount.Add(billItem.DetailTaxAmount)
		}

		// Add the item to the PurchaseOrder
		billItems = append(billItems, billItem)

		// update billed qty in po detail
		if purchaseOrderId > 0 && requestedStatus == BillStatusConfirmed {
			if err := UpdatePoDetailBilledQty(tx, ctx, purchaseOrderId, billItem, "create", decimal.NewFromFloat(0), 0); err != nil {
				tx.Rollback()
				return nil, err
			}

		}

	}

	// calculate bill discount
	var billDiscountAmount decimal.Decimal

	if input.BillDiscountType != nil {
		billDiscountAmount = utils.CalculateDiscountAmount(billSubtotal, input.BillDiscount, string(*input.BillDiscountType))
	}
	// calculate order tax amount (always exclusive)
	var billTaxAmount decimal.Decimal
	if input.BillTaxId > 0 {
		if *input.BillTaxType == TaxTypeGroup {
			billTaxAmount = utils.CalculateTaxAmount(ctx, db, input.BillTaxId, true, billSubtotal, false)
		} else {
			billTaxAmount = utils.CalculateTaxAmount(ctx, db, input.BillTaxId, false, billSubtotal, false)
		}
	} else {
		billTaxAmount = decimal.NewFromFloat(0)
	}

	// Sum (order discount + total detail discount)
	totalBillDiscountAmount := billDiscountAmount.Add(totalDetailDiscountAmount)
	// Sum (order tax amount + total detail tax amount)
	totalBillTaxAmount := billTaxAmount.Add(totalDetailTaxAmount)

	billTotalAmount = billSubtotal.Add(billTaxAmount).Add(totalExclusiveTaxAmount).Add(input.AdjustmentAmount).Sub(billDiscountAmount)
	// store Bill
	bill := Bill{
		BusinessId:          businessId,
		SupplierId:          input.SupplierId,
		BranchId:            input.BranchId,
		PurchaseOrderId:     purchaseOrderId,
		PurchaseOrderNumber: input.PurchaseOrderNumber,
		// BillNumber:              input.BillNumber,
		ReferenceNumber:            input.ReferenceNumber,
		BillDate:                   input.BillDate,
		BillDueDate:                calculateDueDate(input.BillDate, input.BillPaymentTerms, input.BillPaymentTermsCustomDays),
		BillPaymentTerms:           input.BillPaymentTerms,
		BillPaymentTermsCustomDays: input.BillPaymentTermsCustomDays,
		BillSubject:                input.BillSubject,
		Notes:                      input.Notes,
		CurrencyId:                 input.CurrencyId,
		ExchangeRate:               input.ExchangeRate,
		BillDiscount:               input.BillDiscount,
		BillDiscountType:           input.BillDiscountType,
		BillDiscountAmount:         billDiscountAmount,
		AdjustmentAmount:           input.AdjustmentAmount,
		BillTaxId:                  input.BillTaxId,
		BillTaxType:                input.BillTaxType,
		BillTaxAmount:              billTaxAmount,
		CurrentStatus:              BillStatusDraft,
		Documents:                  documents,
		WarehouseId:                input.WarehouseId,
		Details:                    billItems,
		BillTotalDiscountAmount:    totalBillDiscountAmount,
		BillTotalTaxAmount:         totalBillTaxAmount,
		BillSubtotal:               billSubtotal,
		BillTotalAmount:            billTotalAmount,
		RemainingBalance:           billTotalAmount,
		IsTaxInclusive:             input.IsTaxInclusive,
	}

	seqNo, err := utils.GetSequence[Bill](ctx, businessId)
	if err != nil {
		tx.Rollback()
		return nil, err
	}
	prefix, err := getTransactionPrefix(ctx, input.BranchId, "Bill")
	if err != nil {
		tx.Rollback()
		return nil, err
	}
	bill.SequenceNo = decimal.NewFromInt(seqNo)
	bill.BillNumber = prefix + fmt.Sprint(seqNo)

	err = tx.WithContext(ctx).Create(&bill).Error
	if err != nil {
		tx.Rollback()
		return nil, err
	}

	// Reload with Details so stock side-effects can access them.
	if err := tx.WithContext(ctx).Preload("Details").First(&bill, bill.ID).Error; err != nil {
		tx.Rollback()
		return nil, err
	}

	// If requested "Confirmed", apply the status transition deterministically (Draft -> Confirmed).
	if requestedStatus == BillStatusConfirmed {
		if err := tx.WithContext(ctx).Model(&bill).Update("CurrentStatus", BillStatusConfirmed).Error; err != nil {
			tx.Rollback()
			return nil, err
		}
		bill.CurrentStatus = BillStatusConfirmed

		// Preserve existing side-effect: close PO if required.
		if bill.PurchaseOrderId > 0 {
			if err := ClosePoStatus(tx.WithContext(ctx), &bill); err != nil {
				tx.Rollback()
				return nil, err
			}
		}

		// Apply inventory side-effects deterministically (prefer explicit command handler).
		if config.UseStockCommandsFor("BILL") {
			if err := ApplyBillStockForStatusTransition(tx.WithContext(ctx), &bill, BillStatusDraft); err != nil {
				tx.Rollback()
				return nil, err
			}
		} else {
			if err := bill.AfterUpdateCurrentStatus(tx.WithContext(ctx), string(BillStatusDraft)); err != nil {
				tx.Rollback()
				return nil, err
			}
		}

		// Write outbox record (publishing happens after commit via dispatcher).
		if err := PublishToAccounting(ctx, tx, businessId, bill.BillDate, bill.ID, AccountReferenceTypeBill, bill, nil, PubSubMessageActionCreate); err != nil {
			tx.Rollback()
			return nil, err
		}
	}

	if err := tx.Commit().Error; err != nil {
		return nil, err
	}

	return &bill, nil
}

func UpdateBill(ctx context.Context, billID int, updatedBill *NewBill) (*Bill, error) {
	db := config.GetDB()

	businessId, ok := utils.GetBusinessIdFromContext(ctx)
	if !ok || businessId == "" {
		return nil, errors.New("business id is required")
	}

	if err := updatedBill.validate(ctx, businessId, billID); err != nil {
		return nil, err
	}

	// Fetch the existing bill
	oldBill, err := utils.FetchModelForChange[Bill](ctx, businessId, billID, "Details")
	if err != nil {
		return nil, err
	}
	// don't allow updating (partial) paid bill
	if oldBill.CurrentStatus == BillStatusPartialPaid || oldBill.CurrentStatus == BillStatusPaid {
		return nil, errors.New("cannot update paid/partial paid bill")
	}
	// Fintech integrity guardrail (behind flag): inventory-affecting docs are immutable after confirm.
	if config.StrictInventoryDocImmutability() && oldBill.CurrentStatus == BillStatusConfirmed {
		return nil, errors.New("cannot edit a confirmed bill; void and recreate to preserve inventory/valuation integrity")
	}

	// copy oldBill instead of fetching from db again
	existingBill := *oldBill

	existingDetails := append([]BillDetail(nil), existingBill.Details...)

	oldStatus := existingBill.CurrentStatus
	// validate current status
	switch updatedBill.CurrentStatus {
	case BillStatusDraft:
		if oldStatus != BillStatusDraft {
			return nil, errors.New("invalid status")
		}
	case BillStatusConfirmed:
		break
	default:
		return nil, errors.New("invalid status")
	}

	var oldBillDate *time.Time
	// nil if invoice dates are the same, old invoice date if not
	if !existingBill.BillDate.Equal(updatedBill.BillDate) {
		v := existingBill.BillDate
		oldBillDate = &v
	}

	var oldWarehouseId *int
	// nil if warehouse ids are the same, old warehouse id if not
	if existingBill.WarehouseId != updatedBill.WarehouseId {
		v := existingBill.WarehouseId
		oldWarehouseId = &v
	}

	var po PurchaseOrder
	if existingBill.PurchaseOrderId > 0 {
		// exists purchase order
		err := db.Preload("Details").Where("business_id = ? AND id = ?", businessId, existingBill.PurchaseOrderId).First(&po).Error
		if err != nil {
			return nil, err
		}
	}

	// Update the fields of the existing bill with the provided updated details
	existingBill.SupplierId = updatedBill.SupplierId
	existingBill.BranchId = updatedBill.BranchId
	existingBill.ReferenceNumber = updatedBill.ReferenceNumber
	existingBill.BillDate = updatedBill.BillDate
	existingBill.BillDueDate = calculateDueDate(updatedBill.BillDate, updatedBill.BillPaymentTerms, updatedBill.BillPaymentTermsCustomDays)
	existingBill.BillPaymentTerms = updatedBill.BillPaymentTerms
	existingBill.BillPaymentTermsCustomDays = updatedBill.BillPaymentTermsCustomDays
	existingBill.BillSubject = updatedBill.BillSubject
	existingBill.Notes = updatedBill.Notes
	existingBill.CurrencyId = updatedBill.CurrencyId
	existingBill.CurrentStatus = updatedBill.CurrentStatus
	existingBill.ExchangeRate = updatedBill.ExchangeRate
	existingBill.BillDiscount = updatedBill.BillDiscount
	existingBill.BillDiscountType = updatedBill.BillDiscountType
	existingBill.AdjustmentAmount = updatedBill.AdjustmentAmount
	existingBill.BillTaxId = updatedBill.BillTaxId
	existingBill.BillTaxType = updatedBill.BillTaxType
	existingBill.WarehouseId = updatedBill.WarehouseId
	existingBill.IsTaxInclusive = updatedBill.IsTaxInclusive

	var orderSubtotal,
		orderTotalAmount,
		totalExclusiveTaxAmount,
		totalDetailDiscountAmount,
		totalDetailTaxAmount decimal.Decimal

	// Iterate through the updated items

	tx := db.Begin()
	for _, updatedItem := range updatedBill.Details {

		var existingItem *BillDetail
		// var existingItemIndex int

		// // Check if the item already exists in the bill
		// for i, item := range existingBill.Details {
		// 	if item.ID == updatedItem.DetailId {
		// 		existingItem = &item
		// 		existingItemIndex = i
		// 		break
		// 	}
		// }

		// Check if the item already exists in the bill
		for _, item := range existingDetails {
			if item.ID == updatedItem.DetailId {
				existingItem = &item
				break
			}
		}
		// If the item doesn't exist, add it to the bill, along with stock
		if existingItem == nil {
			newItem := BillDetail{
				ProductId:           updatedItem.ProductId,
				ProductType:         updatedItem.ProductType,
				BatchNumber:         updatedItem.BatchNumber,
				Name:                updatedItem.Name,
				Description:         updatedItem.Description,
				DetailAccountId:     updatedItem.DetailAccountId,
				DetailQty:           updatedItem.DetailQty,
				DetailUnitRate:      updatedItem.DetailUnitRate,
				DetailTaxId:         updatedItem.DetailTaxId,
				DetailTaxType:       updatedItem.DetailTaxType,
				DetailDiscount:      updatedItem.DetailDiscount,
				DetailDiscountType:  updatedItem.DetailDiscountType,
				PurchaseOrderItemId: updatedItem.PurchaseOrderItemId,
			}

			// Calculate tax and total amounts for the item
			newItem.CalculateItemDiscountAndTax(ctx, *updatedBill.IsTaxInclusive)
			orderSubtotal, totalDetailDiscountAmount, totalDetailTaxAmount, totalExclusiveTaxAmount = updateBillDetailTotal(&newItem, *updatedBill.IsTaxInclusive, orderSubtotal, totalExclusiveTaxAmount, totalDetailDiscountAmount, totalDetailTaxAmount)
			existingBill.Details = append(existingBill.Details, newItem)

			if oldStatus == BillStatusConfirmed {

				inventoryAccId := 0
				if newItem.ProductId > 0 {
					product, err := GetProductOrVariant(ctx, string(newItem.ProductType), newItem.ProductId)

					if err != nil {
						tx.Rollback()
						return nil, err
					}

					if product.GetInventoryAccountID() > 0 {
						if err := UpdateStockSummaryReceivedQty(tx, businessId, existingBill.WarehouseId, newItem.ProductId, string(newItem.ProductType), newItem.BatchNumber, newItem.DetailQty, existingBill.BillDate); err != nil {
							tx.Rollback()
							return nil, err
						}
						inventoryAccId = product.GetInventoryAccountID()
					}
				}

				if existingBill.PurchaseOrderId > 0 {
					// update billed qty in po detail
					// var poDetail PurchaseOrderDetail
					// var err error
					// if newItem.PurchaseOrderItemId > 0 {
					// 	err = tx.WithContext(ctx).Where("id = ?", newItem.PurchaseOrderItemId).First(&poDetail).Error
					// } else {
					// 	err = tx.Where("purchase_order_id = ? AND product_id = ? AND product_type = ? AND batch_number = ?",
					// 		existingBill.PurchaseOrderId, newItem.ProductId, newItem.ProductType, newItem.BatchNumber).First(&poDetail).Error

					// }
					// if err == gorm.ErrRecordNotFound {
					// 	// Skip update if record does not exist
					// 	continue
					// } else if err != nil {
					// 	tx.Rollback()
					// 	return nil, err
					// }

					if err := UpdatePoDetailBilledQty(tx, ctx, existingBill.PurchaseOrderId, newItem, "create", decimal.NewFromFloat(0), inventoryAccId); err != nil {
						tx.Rollback()
						return nil, err
					}

					_, err = ChangePoCurrentStatus(tx.WithContext(ctx), ctx, businessId, existingBill.PurchaseOrderId)
					if err != nil {
						tx.Rollback()
						return nil, err
					}
				}
			}
		} else {

			var oldDetailQty *decimal.Decimal
			// nil if detail qty does not change
			if !existingItem.DetailQty.Equal(updatedItem.DetailQty) {
				v := existingItem.DetailQty
				oldDetailQty = &v
			}

			var oldBatchNumber *string
			// nil if batch number does not change
			if existingItem.BatchNumber != updatedItem.BatchNumber {
				v := existingItem.BatchNumber
				oldBatchNumber = &v
			}

			// if the item is to be DELETED
			if updatedItem.IsDeletedItem != nil && *updatedItem.IsDeletedItem {
				// fmt.Println("Is deleted Item")
				// if err := db.WithContext(ctx).Delete(&existingItem).Error; err != nil {
				// 	return nil, err
				// }
				// Find the index of the item to delete
				for i, item := range existingBill.Details {
					if item.ID == updatedItem.DetailId {
						// Delete the item from the database
						if err := tx.WithContext(ctx).Delete(&existingBill.Details[i]).Error; err != nil {
							tx.Rollback()
							return nil, err
						}
						// Remove the item from the slice
						existingBill.Details = append(existingBill.Details[:i], existingBill.Details[i+1:]...)

						if oldStatus == BillStatusConfirmed {
							inventoryAccId := 0
							if item.ProductId > 0 {
								product, err := GetProductOrVariant(ctx, string(item.ProductType), item.ProductId)
								if err != nil {
									tx.Rollback()
									return nil, err
								}

								if product.GetInventoryAccountID() > 0 {
									//? add cases for partialPaid or don't allow it to update
									if existingBill.CurrentStatus == BillStatusConfirmed {
										if err := UpdateStockSummaryReceivedQty(tx, businessId,
											utils.DereferencePtr(oldWarehouseId, existingBill.WarehouseId),
											item.ProductId,
											string(item.ProductType),
											utils.DereferencePtr(oldBatchNumber, existingItem.BatchNumber),
											utils.DereferencePtr(oldDetailQty, item.DetailQty).Neg(),
											utils.DereferencePtr(oldBillDate, existingBill.BillDate)); err != nil {
											tx.Rollback()
											return nil, err
										}
									}
									inventoryAccId = product.GetInventoryAccountID()
								}
							}

							if existingBill.PurchaseOrderId > 0 {
								if err := UpdatePoDetailBilledQty(tx, ctx, existingBill.PurchaseOrderId, item, "delete", decimal.NewFromFloat(0), inventoryAccId); err != nil {
									tx.Rollback()
									return nil, err
								}

								_, err = ChangePoCurrentStatus(tx.WithContext(ctx), ctx, businessId, existingBill.PurchaseOrderId)
								if err != nil {
									tx.Rollback()
									return nil, err
								}
							}
						}
					}
				}
			} else {
				var oldQty decimal.Decimal = existingItem.DetailQty
				// UPDATE existing item details

				// don't let update product
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
				existingItem.CalculateItemDiscountAndTax(ctx, *updatedBill.IsTaxInclusive)
				orderSubtotal, totalDetailDiscountAmount, totalDetailTaxAmount, totalExclusiveTaxAmount = updateBillDetailTotal(existingItem, *updatedBill.IsTaxInclusive, orderSubtotal, totalExclusiveTaxAmount, totalDetailDiscountAmount, totalDetailTaxAmount)
				// existingBill.Details = append(existingBill.Details, *existingItem)
				inventoryAccId := 0
				if existingItem.ProductId > 0 {
					product, err := GetProductOrVariant(ctx, string(existingItem.ProductType), existingItem.ProductId)
					if err != nil {
						tx.Rollback()
						return nil, err
					}
					// update stockSummary received qty
					if product.GetInventoryAccountID() > 0 {
						inventoryAccId = product.GetInventoryAccountID()
						// newly confirm
						if existingBill.CurrentStatus == BillStatusConfirmed && oldStatus == BillStatusDraft {
							if err := UpdateStockSummaryReceivedQty(tx,
								businessId,
								existingBill.WarehouseId,
								existingItem.ProductId,
								string(existingItem.ProductType),
								existingItem.BatchNumber,
								existingItem.DetailQty,
								existingBill.BillDate); err != nil {
								tx.Rollback()
								return nil, err
							}
						} else if existingBill.CurrentStatus == BillStatusConfirmed && oldStatus == BillStatusConfirmed {
							// updateStockSummary

							// lock business
							err := utils.BusinessLock(ctx, businessId, "stockLock", "modelHoodsStockSummary.go", "SaleBillDetailBeforeUpdate")
							if err != nil {
								tx.Rollback()
								return nil, err
							}

							// if belong to a different stock_summary
							if oldBillDate != nil || oldWarehouseId != nil || oldBatchNumber != nil {

								// remove old stock old date old warehouse
								if err := UpdateStockSummaryReceivedQty(tx, businessId,
									utils.DereferencePtr(oldWarehouseId, existingBill.WarehouseId),
									existingItem.ProductId,
									string(existingItem.ProductType),
									utils.DereferencePtr(oldBatchNumber, existingItem.BatchNumber),
									utils.DereferencePtr(oldDetailQty, existingItem.DetailQty).Neg(),
									utils.DereferencePtr(oldBillDate, existingBill.BillDate),
								); err != nil {
									tx.Rollback()
									return nil, err
								}
								// add new stock
								if err := UpdateStockSummaryReceivedQty(tx, businessId,
									existingBill.WarehouseId,
									existingItem.ProductId,
									string(existingItem.ProductType),
									existingItem.BatchNumber,
									existingItem.DetailQty,
									existingBill.BillDate); err != nil {
									tx.Rollback()
									return nil, err
								}

							} else if oldDetailQty != nil {
								// same warehouse, same date, same batch number but qty differs

								addedQty := existingItem.DetailQty.Sub(*oldDetailQty)
								// if addedQty.IsPositive() {
								// 	if err := ValidateProductStock(ctx, businessId, existingBill.WarehouseId, existingItem.BatchNumber, existingItem.ProductType, existingItem.ProductId, addedQty); err != nil {
								// 		tx.Rollback()
								// 		return nil, err
								// 	}
								// }
								// if err != nil {
								// 	tx.Rollback()
								// 	return nil, err
								// }
								if err := UpdateStockSummaryReceivedQty(tx, businessId,
									existingBill.WarehouseId,
									existingItem.ProductId,
									string(existingItem.ProductType),
									existingItem.BatchNumber,
									addedQty,
									existingBill.BillDate); err != nil {
									tx.Rollback()
									return nil, err
								}
							}
						}
					}
				}

				if err := tx.WithContext(ctx).Save(&existingItem).Error; err != nil {
					tx.Rollback()
					return nil, err
				}

				if existingBill.CurrentStatus == BillStatusConfirmed && existingBill.PurchaseOrderId > 0 {

					if !existingItem.DetailQty.Equal(oldQty) || (oldStatus == BillStatusDraft && existingBill.CurrentStatus == BillStatusConfirmed) {

						if !existingItem.DetailQty.Equal(oldQty) && oldStatus == BillStatusConfirmed {

							if err := UpdatePoDetailBilledQty(tx, ctx, existingBill.PurchaseOrderId, *existingItem, "update", oldQty, inventoryAccId); err != nil {
								tx.Rollback()
								return nil, err
							}
						}

						if oldStatus == BillStatusDraft && existingBill.CurrentStatus == BillStatusConfirmed {

							if err := UpdatePoDetailBilledQty(tx, ctx, existingBill.PurchaseOrderId, *existingItem, "create", decimal.NewFromFloat(0), inventoryAccId); err != nil {
								tx.Rollback()
								return nil, err
							}
						}

						_, err = ChangePoCurrentStatus(tx.WithContext(ctx), ctx, businessId, existingBill.PurchaseOrderId)
						if err != nil {
							tx.Rollback()
							return nil, err
						}
					}
				}

			}
		}
	}

	// calculate order discount
	var orderDiscountAmount decimal.Decimal

	if updatedBill.BillDiscountType != nil {
		orderDiscountAmount = utils.CalculateDiscountAmount(orderSubtotal, updatedBill.BillDiscount, string(*updatedBill.BillDiscountType))
	}

	existingBill.BillDiscountAmount = orderDiscountAmount

	// orderSubtotal = orderSubtotal.Sub(orderDiscountAmount)
	existingBill.BillSubtotal = orderSubtotal

	// calculate order tax amount (always exclusive)
	var orderTaxAmount decimal.Decimal
	if updatedBill.BillTaxId > 0 {
		if *updatedBill.BillTaxType == TaxTypeGroup {
			orderTaxAmount = utils.CalculateTaxAmount(ctx, db, updatedBill.BillTaxId, true, orderSubtotal, false)
		} else {
			orderTaxAmount = utils.CalculateTaxAmount(ctx, db, updatedBill.BillTaxId, false, orderSubtotal, false)
		}
	} else {
		orderTaxAmount = decimal.NewFromFloat(0)
	}

	existingBill.BillTaxAmount = orderTaxAmount

	// Sum (order discount + total detail discount)
	totalOrderDiscountAmount := orderDiscountAmount.Add(totalDetailDiscountAmount)
	existingBill.BillTotalDiscountAmount = totalOrderDiscountAmount

	// Sum (order tax amount + total detail tax amount)
	totalOrderTaxAmount := orderTaxAmount.Add(totalDetailTaxAmount)
	existingBill.BillTotalTaxAmount = totalOrderTaxAmount
	// Sum Grand total amount (subtotal+ exclusive tax + adj amount)
	orderTotalAmount = orderSubtotal.Add(orderTaxAmount).Add(totalExclusiveTaxAmount).Add(updatedBill.AdjustmentAmount).Sub(orderDiscountAmount)

	existingBill.BillTotalAmount = orderTotalAmount
	existingBill.RemainingBalance = orderTotalAmount

	// Save the updated bill to the database
	if err := tx.WithContext(ctx).
		// Omit("Documents").
		Save(&existingBill).Error; err != nil {
		tx.Rollback()
		return nil, err
	}

	// Refresh the existingBill to get the latest details
	if err := tx.WithContext(ctx).Preload("Details").First(&existingBill, billID).Error; err != nil {
		tx.Rollback()
		return nil, err
	}

	// if err := existingBill.AfterUpdateCurrentStatus(tx.WithContext(ctx), string(oldStatus)); err != nil {
	// 	tx.Rollback()
	// 	return nil, err
	// }

	if oldStatus == BillStatusDraft && existingBill.CurrentStatus == BillStatusConfirmed {
		err := PublishToAccounting(ctx, tx, businessId, existingBill.BillDate, existingBill.ID, AccountReferenceTypeBill, existingBill, nil, PubSubMessageActionCreate)
		if err != nil {
			tx.Rollback()
			return nil, err
		}
	} else if oldStatus == BillStatusConfirmed && existingBill.CurrentStatus == BillStatusConfirmed {
		err := PublishToAccounting(ctx, tx, businessId, existingBill.BillDate, existingBill.ID, AccountReferenceTypeBill, existingBill, oldBill, PubSubMessageActionUpdate)
		if err != nil {
			tx.Rollback()
			return nil, err
		}
	}

	documents, err := upsertDocuments(ctx, tx, updatedBill.Documents, "bills", billID)
	if err != nil {
		tx.Rollback()
		return nil, err
	}
	existingBill.Documents = documents

	if err := tx.Commit().Error; err != nil {
		return nil, err
	}

	return &existingBill, nil
}

func DeleteBill(ctx context.Context, id int) (*Bill, error) {
	businessId, ok := utils.GetBusinessIdFromContext(ctx)
	if !ok || businessId == "" {
		return nil, errors.New("business id is required")
	}

	db := config.GetDB()
	result, err := utils.FetchModelForChange[Bill](ctx, businessId, id, "Details", "Documents")
	if err != nil {
		return nil, err
	}

	if result.CurrentStatus == BillStatusConfirmed {
		err := result.ValidateStockQty(ctx, businessId)
		if err != nil {
			return nil, err
		}
	}

	tx := db.Begin()

	for _, billItem := range result.Details {
		if result.CurrentStatus == BillStatusConfirmed {
			inventoryAccId := 0
			if billItem.ProductId > 0 {
				product, err := GetProductOrVariant(ctx, string(billItem.ProductType), billItem.ProductId)
				if err != nil {
					tx.Rollback()
					return nil, err
				}
				if product.GetInventoryAccountID() > 0 {
					// reduced received qty from stock summary if bill is confirmed
					if err := UpdateStockSummaryReceivedQty(tx, result.BusinessId, result.WarehouseId, billItem.ProductId, string(billItem.ProductType), billItem.BatchNumber, billItem.DetailQty.Neg(), result.BillDate); err != nil {
						tx.Rollback()
						return nil, err
					}

				}
				inventoryAccId = product.GetInventoryAccountID()
			}
			if result.PurchaseOrderId > 0 {
				if err := UpdatePoDetailBilledQty(tx, ctx, result.PurchaseOrderId, billItem, "delete", decimal.NewFromFloat(0), inventoryAccId); err != nil {
					tx.Rollback()
					return nil, err
				}

				_, err = ChangePoCurrentStatus(tx.WithContext(ctx), ctx, businessId, result.PurchaseOrderId)
				if err != nil {
					tx.Rollback()
					return nil, err
				}
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

	if err := deleteDocuments(ctx, tx, result.Documents); err != nil {
		tx.Rollback()
		return nil, err
	}

	if result.CurrentStatus == BillStatusConfirmed {
		err = PublishToAccounting(ctx, tx, businessId, result.BillDate, result.ID, AccountReferenceTypeBill, nil, result, PubSubMessageActionDelete)
		if err != nil {
			tx.Rollback()
			return nil, err
		}
	}

	if err := tx.Commit().Error; err != nil {
		return nil, err
	}

	return result, nil
}

func UpdateStatusBill(ctx context.Context, id int, status string) (*Bill, error) {
	businessId, ok := utils.GetBusinessIdFromContext(ctx)
	if !ok || businessId == "" {
		return nil, errors.New("business id is required")
	}
	// db action
	db := config.GetDB()
	tx := db.Begin()

	bill, err := utils.FetchModelForChange[Bill](ctx, businessId, id, "Details")
	if err != nil {
		return nil, err
	}

	if bill.CurrentStatus == BillStatusConfirmed && status == string(BillStatusVoid) {
		err := bill.ValidateStockQty(ctx, businessId)
		if err != nil {
			return nil, err
		}
	}

	oldStatus := bill.CurrentStatus

	if err := tx.WithContext(ctx).Model(&bill).UpdateColumn("CurrentStatus", status).Error; err != nil {
		tx.Rollback()
		return nil, err
	}

	// Apply inventory side-effects deterministically (prefer explicit command handler).
	if config.UseStockCommandsFor("BILL") {
		bill.CurrentStatus = BillStatus(status)
		if err := ApplyBillStockForStatusTransition(tx.WithContext(ctx), bill, oldStatus); err != nil {
			tx.Rollback()
			return nil, err
		}
	} else {
		if err := bill.AfterUpdateCurrentStatus(tx.WithContext(ctx), string(oldStatus)); err != nil {
			tx.Rollback()
			return nil, err
		}
	}

	if bill.PurchaseOrderId > 0 {

		for _, billItem := range bill.Details {
			inventoryAccId := 0
			if billItem.ProductId > 0 {
				product, err := GetProductOrVariant(ctx, string(billItem.ProductType), billItem.ProductId)
				if err != nil {
					tx.Rollback()
					return nil, err
				}
				inventoryAccId = product.GetInventoryAccountID()
			}
			if status == string(BillStatusVoid) || status == string(BillStatusConfirmed) {
				if status == string(BillStatusVoid) {
					if err := UpdatePoDetailBilledQty(tx, ctx, bill.PurchaseOrderId, billItem, "delete", decimal.NewFromFloat(0), inventoryAccId); err != nil {
						tx.Rollback()
						return nil, err
					}
				}
				if status == string(BillStatusConfirmed) {
					if err := UpdatePoDetailBilledQty(tx, ctx, bill.PurchaseOrderId, billItem, "create", decimal.NewFromFloat(0), inventoryAccId); err != nil {
						tx.Rollback()
						return nil, err
					}
				}

				_, err = ChangePoCurrentStatus(tx.WithContext(ctx), ctx, businessId, bill.PurchaseOrderId)
				if err != nil {
					tx.Rollback()
					return nil, err
				}
			}

		}
	}

	if oldStatus == BillStatusDraft && status == string(BillStatusConfirmed) {
		err := PublishToAccounting(ctx, tx, businessId, bill.BillDate, bill.ID, AccountReferenceTypeBill, bill, nil, PubSubMessageActionCreate)
		if err != nil {
			tx.Rollback()
			return nil, err
		}
	} else if oldStatus == BillStatusConfirmed && status == string(BillStatusVoid) {
		err = PublishToAccounting(ctx, tx, businessId, bill.BillDate, bill.ID, AccountReferenceTypeBill, nil, bill, PubSubMessageActionDelete)
		if err != nil {
			tx.Rollback()
			return nil, err
		}
	}

	// log history of status change
	if err := createHistory(tx.WithContext(ctx), "Update", bill.ID, "bills", nil, nil, "Updated current status to "+status); err != nil {
		tx.Rollback()
		return nil, err
	}

	// Commit the transaction
	if err := tx.Commit().Error; err != nil {
		return nil, err
	}

	return bill, nil
}

func GetBill(ctx context.Context, id int) (*Bill, error) {
	businessId, ok := utils.GetBusinessIdFromContext(ctx)
	if !ok || businessId == "" {
		return nil, errors.New("business id is required")
	}

	return utils.FetchModel[Bill](ctx, businessId, id)
}

func GetBills(ctx context.Context, billNumber *string) ([]*Bill, error) {
	db := config.GetDB()
	var results []*Bill

	businessId, ok := utils.GetBusinessIdFromContext(ctx)
	if !ok || businessId == "" {
		return nil, errors.New("business id is required")
	}

	dbCtx := db.WithContext(ctx).Where("business_id = ?", businessId)
	if billNumber != nil && len(*billNumber) > 0 {
		dbCtx = dbCtx.Where("bill_number LIKE ?", "%"+*billNumber+"%")
	}
	err := dbCtx.
		Find(&results).Error
	if err != nil {
		return nil, err
	}
	return results, nil
}

func GetBillIdsBySupplierID(ctx context.Context, supplierId int, billStatus string) ([]int, error) {

	db := config.GetDB()
	var billIds []int

	businessId, ok := utils.GetBusinessIdFromContext(ctx)
	if !ok || businessId == "" {
		return nil, errors.New("business id is required")
	}

	dbCtx := db.WithContext(ctx).Model(&Bill{}).Where("business_id = ?", businessId).Where("supplier_id = ?", supplierId)

	var result *gorm.DB // Declare result variable

	if billStatus == "Paid" {
		result = dbCtx.Where("current_status = ?", BillStatusPaid).Pluck("id", &billIds)
	} else if billStatus == "UnPaid" {
		result = dbCtx.Where("current_status = ? OR current_status = ?", BillStatusPartialPaid, BillStatusConfirmed).Pluck("id", &billIds)
	} else {
		result = dbCtx.Pluck("id", &billIds)
	}

	if result.Error != nil {
		return nil, result.Error
	}

	return billIds, nil
}

func PaginateBill(ctx context.Context,
	limit *int, after *string,
	billNumber *string,
	referenceNumber *string,
	branchID *int,
	warehouseID *int,
	supplierID *int,
	currentStatus *BillStatus,
	startBillDate *MyDateString,
	endBillDate *MyDateString,
	startBillDueDate *MyDateString,
	endBillDueDate *MyDateString) (*BillsConnection, error) {

	business, err := GetBusiness(ctx)
	if err != nil {
		return nil, errors.New("business id is required")
	}

	if err := startBillDate.StartOfDayUTCTime(business.Timezone); err != nil {
		return nil, err
	}
	if err := endBillDate.EndOfDayUTCTime(business.Timezone); err != nil {
		return nil, err
	}
	if err := startBillDueDate.StartOfDayUTCTime(business.Timezone); err != nil {
		return nil, err
	}
	if err := endBillDueDate.EndOfDayUTCTime(business.Timezone); err != nil {
		return nil, err
	}

	db := config.GetDB()
	dbCtx := db.WithContext(ctx).Where("business_id = ?", business.ID)

	if billNumber != nil && *billNumber != "" {
		dbCtx.Where("bill_number LIKE ?", "%"+*billNumber+"%")
	}
	if referenceNumber != nil && *referenceNumber != "" {
		dbCtx.Where("reference_number LIKE ?", "%"+*referenceNumber+"%")
	}
	if branchID != nil && *branchID > 0 {
		dbCtx.Where("branch_id = ?", *branchID)
	}
	if supplierID != nil && *supplierID > 0 {
		dbCtx.Where("supplier_id = ?", *supplierID)
	}
	if warehouseID != nil && *warehouseID > 0 {
		dbCtx.Where("warehouse_id = ?", *warehouseID)
	}
	if currentStatus != nil {
		dbCtx.Where("current_status = ?", *currentStatus)
	}
	if startBillDate != nil && endBillDate != nil {
		dbCtx.Where("bill_date BETWEEN ? AND ?", startBillDate, endBillDate)
	}
	if startBillDueDate != nil && endBillDueDate != nil {
		dbCtx.Where("bill_due_date BETWEEN ? AND ?", startBillDueDate, endBillDueDate)
	}

	edges, pageInfo, err := FetchPageCompositeCursor[Bill](dbCtx, *limit, after, "created_at", "<")
	if err != nil {
		return nil, err
	}
	var billsConnection BillsConnection
	billsConnection.PageInfo = pageInfo
	for _, edge := range edges {
		billsEdge := BillsEdge(edge)
		billsConnection.Edges = append(billsConnection.Edges, &billsEdge)
	}

	return &billsConnection, err
}

func GetBillTotalSummary(ctx context.Context) (*BillTotalSummary, error) {
	var totalSummary BillTotalSummary
	businessId, ok := utils.GetBusinessIdFromContext(ctx)
	if !ok || businessId == "" {
		return nil, errors.New("business id is required")
	}
	business, err := GetBusiness(ctx)
	if err != nil {
		return nil, errors.New("business id is required")
	}
	timezone := "Asia/Yangon"
	if business.Timezone != "" {
		timezone = business.Timezone
	}

	db := config.GetDB()
	dbCtx := db.WithContext(ctx).Where("business_id = ?", businessId)

	err = dbCtx.Table("bills").
		Select("SUM((bill_total_amount - bill_total_paid_amount) * CASE WHEN exchange_rate = 0 THEN 1 ELSE exchange_rate END) AS TotalOutstandingPayable, "+
			"SUM(CASE WHEN DATE(CONVERT_TZ(bill_due_date, 'UTC', ?)) = DATE(CONVERT_TZ(UTC_TIMESTAMP(), 'UTC', ?)) THEN (bill_total_amount - bill_total_paid_amount) * CASE WHEN exchange_rate = 0 THEN 1 ELSE exchange_rate END ELSE 0 END) AS DueToday, "+
			"SUM(CASE WHEN DATE(CONVERT_TZ(bill_due_date, 'UTC', ?)) BETWEEN DATE(CONVERT_TZ(UTC_TIMESTAMP(), 'UTC', ?)) AND DATE_ADD(DATE(CONVERT_TZ(UTC_TIMESTAMP(), 'UTC', ?)), INTERVAL 30 DAY) THEN (bill_total_amount - bill_total_paid_amount) * CASE WHEN exchange_rate = 0 THEN 1 ELSE exchange_rate END ELSE 0 END) AS DueWithin30Days, "+
			"SUM(CASE WHEN DATE(CONVERT_TZ(bill_due_date, 'UTC', ?)) < DATE(CONVERT_TZ(UTC_TIMESTAMP(), 'UTC', ?)) THEN (bill_total_amount - bill_total_paid_amount) * CASE WHEN exchange_rate = 0 THEN 1 ELSE exchange_rate END ELSE 0 END) AS TotalOverdue",
			timezone, timezone, timezone, timezone, timezone, timezone, timezone).
		Where("current_status IN ('Confirmed', 'Partial Paid')").
		Scan(&totalSummary).Error

	if err != nil {
		return nil, err
	}
	return &totalSummary, nil
}

func GetBalanceDueAmount(ctx context.Context, bill *Bill) (*decimal.Decimal, error) {
	balanceDue := bill.BillTotalAmount.Sub(bill.BillTotalPaidAmount)
	return &balanceDue, nil
}
