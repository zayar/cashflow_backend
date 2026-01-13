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

type PurchaseOrder struct {
	ID                          int             `gorm:"primary_key" json:"id"`
	BusinessId                  string          `gorm:"index;not null" json:"business_id" binding:"required"`
	SupplierId                  int             `gorm:"index;not null" json:"supplier_id" binding:"required"`
	BranchId                    int             `gorm:"index;not null" json:"branch_id"`
	OrderNumber                 string          `gorm:"size:255;not null" json:"order_number" binding:"required"`
	SequenceNo                  decimal.Decimal `gorm:"type:decimal(15);not null" json:"sequence_no"`
	ReferenceNumber             string          `gorm:"size:255;default:null" json:"reference_number"`
	OrderDate                   time.Time       `gorm:"not null" json:"order_date" binding:"required"`
	ExpectedDeliveryDate        *time.Time      `gorm:"default:null" json:"expected_delivery_date"`
	OrderPaymentTerms           PaymentTerms    `gorm:"type:enum('Net15','Net30','Net45','Net60','DueMonthEnd','DueNextMonthEnd','DueOnReceipt','Custom');not null" json:"order_payment_terms" binding:"required"`
	OrderPaymentTermsCustomDays int             `gorm:"default:0" json:"order_payment_terms_custom_days"`
	DeliveryWarehouseId         int             `gorm:"default:null" json:"delivery_warehouse_id"`
	DeliveryCustomerId          int             `gorm:"default:null" json:"delivery_customer_id"`
	DeliveryAddress             string          `gorm:"type:text;default:null" json:"delivery_address"`
	ShipmentPreferenceId        int             `gorm:"default:null" json:"shipment_preference_id"`
	Notes                       string          `gorm:"type:text;default:null" json:"notes"`
	TermsAndConditions          string          `gorm:"type:text;default:null" json:"terms_and_conditions"`
	CurrencyId                  int             `gorm:"not null" json:"currency_id" binding:"required"`
	ExchangeRate                decimal.Decimal `gorm:"type:decimal(20,4);default:0" json:"exchange_rate"`
	OrderDiscount               decimal.Decimal `gorm:"type:decimal(20,4);default:0" json:"order_discount"`
	OrderDiscountType           *DiscountType   `gorm:"type:enum('P','A');default:null;" json:"order_discount_type"`
	OrderDiscountAmount         decimal.Decimal `gorm:"type:decimal(20,4);default:0" json:"order_discount_amount"`
	AdjustmentAmount            decimal.Decimal `gorm:"type:decimal(20,4);default:0" json:"adjustment_amount"`
	OrderTaxId                  int             `gorm:"default:null" json:"order_tax_id"`
	OrderTaxType                *TaxType        `gorm:"type:enum('I','G');default:null;" json:"order_tax_type"`
	OrderTaxAmount              decimal.Decimal `gorm:"type:decimal(20,4);default:0" json:"order_tax_amount"`
	// <always exclusive> ((orderSubTotal - orderDiscountAmount) / 100) * orderTaxRate
	IsTaxInclusive *bool               `gorm:"not null;default:false" json:"is_tax_inclusive"`
	CurrentStatus  PurchaseOrderStatus `gorm:"type:enum('Draft','Confirmed','Partially Billed','Closed','Cancelled');not null" json:"current_status" binding:"required"`
	Documents      []*Document         `gorm:"polymorphic:Reference" json:"documents"`
	WarehouseId    int                 `gorm:"not null" json:"warehouse_id"`
	OrderSubtotal  decimal.Decimal     `gorm:"type:decimal(20,4);default:0" json:"order_subtotal"`
	// sum(detailTotalAmount)
	OrderTotalDiscountAmount decimal.Decimal `gorm:"type:decimal(20,4);default:0" json:"order_total_discount_amount"`
	// orderDiscountAmount + sum(detailDiscountAmount)
	OrderTotalTaxAmount decimal.Decimal `gorm:"type:decimal(20,4);default:0" json:"order_total_tax_amount"`
	// orderTaxAmount + sum(detailTaxAmount)
	OrderTotalAmount decimal.Decimal `gorm:"type:decimal(20,4);default:0" json:"order_total_amount"`
	// <DetailTax-inclusive> order_subtotal - orderDiscountAmount + orderTaxAmount
	// <DetailTax-exclusive> order_subtotal - orderDiscountAmount + orderTotalTaxAmount
	Details   []PurchaseOrderDetail `json:"purchase_order_details" validate:"required,dive,required"`
	CreatedAt time.Time             `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt time.Time             `gorm:"autoUpdateTime" json:"updated_at"`
}

type NewPurchaseOrder struct {
	SupplierId                  int                      `json:"supplier_id" binding:"required"`
	BranchId                    int                      `json:"branch_id" binding:"required"`
	ReferenceNumber             string                   `json:"reference_number"`
	OrderDate                   time.Time                `json:"order_date" binding:"required"`
	ExpectedDeliveryDate        *time.Time               `json:"expected_delivery_date"`
	OrderPaymentTerms           PaymentTerms             `json:"order_payment_terms" binding:"required"`
	OrderPaymentTermsCustomDays int                      `json:"order_payment_terms_custom_days"`
	DeliveryWarehouseId         int                      `json:"delivery_warehouse_id"`
	DeliveryCustomerId          int                      `json:"delivery_customer_id"`
	DeliveryAddress             string                   `json:"delivery_address"`
	ShipmentPreferenceId        int                      `json:"shipment_preference_id"`
	Notes                       string                   `json:"notes"`
	TermsAndConditions          string                   `json:"terms_and_conditions"`
	CurrencyId                  int                      `json:"currency_id"`
	ExchangeRate                decimal.Decimal          `json:"exchange_rate"`
	OrderDiscount               decimal.Decimal          `json:"order_discount"`
	OrderDiscountType           *DiscountType            `json:"order_discount_type"`
	AdjustmentAmount            decimal.Decimal          `json:"adjustment_amount"`
	OrderTaxId                  int                      `json:"order_tax_id"`
	OrderTaxType                *TaxType                 `json:"order_tax_type"`
	CurrentStatus               PurchaseOrderStatus      `json:"current_status" binding:"required"`
	IsTaxInclusive              *bool                    `json:"is_tax_inclusive" binding:"required"`
	Documents                   []*NewDocument           `json:"documents"`
	WarehouseId                 int                      `json:"warehouse_id" binding:"required"`
	Details                     []NewPurchaseOrderDetail `json:"details"`
}

type PurchaseOrderDetail struct {
	ID                   int             `gorm:"primary_key" json:"id"`
	PurchaseOrderId      int             `gorm:"index;not null" json:"purchase_order_id" binding:"required"`
	ProductId            int             `gorm:"index" json:"product_id"`
	ProductType          ProductType     `gorm:"type:enum('S','G','C','V','I');default:S" json:"product_type"`
	BatchNumber          string          `gorm:"size:100" json:"batch_number"`
	Name                 string          `gorm:"size:100" json:"name" binding:"required"`
	Description          string          `gorm:"size:255;default:null" json:"description"`
	DetailAccountId      int             `gorm:"default:null" json:"detail_account_id"`
	DetailQty            decimal.Decimal `gorm:"type:decimal(20,4);default:0" json:"detail_qty" binding:"required"`
	DetailBilledQty      decimal.Decimal `gorm:"type:decimal(20,4);default:0" json:"detail_billed_qty"`
	DetailUnitRate       decimal.Decimal `gorm:"type:decimal(20,4);default:0" json:"detail_unit_rate" binding:"required"`
	DetailTaxId          int             `gorm:"default:null" json:"detail_tax_id"`
	DetailTaxType        *TaxType        `gorm:"type:enum('I','G');default:null;" json:"detail_tax_type"`
	DetailDiscount       decimal.Decimal `gorm:"type:decimal(20,4);default:0" json:"detail_discount"`
	DetailDiscountType   *DiscountType   `gorm:"type:enum('P','A');default:null;" json:"order_discount_type"`
	DetailDiscountAmount decimal.Decimal `gorm:"type:decimal(20,4);default:0" json:"detail_discount_amount"`
	DetailTaxAmount      decimal.Decimal `gorm:"type:decimal(20,4);default:0" json:"detail_tax_amount"`
	DetailTotalAmount    decimal.Decimal `gorm:"type:decimal(20,4);default:0" json:"detail_total_amount"`
}

type NewPurchaseOrderDetail struct {
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
	DetailDiscount     decimal.Decimal `json:"detail_discount"`
	DetailDiscountType *DiscountType   `json:"detail_discount_type"`
	IsDeletedItem      *bool           `json:"is_deleted_item"`
}

type PurchaseOrdersConnection struct {
	Edges    []*PurchaseOrdersEdge `json:"edges"`
	PageInfo *PageInfo             `json:"pageInfo"`
}

type PurchaseOrdersEdge Edge[PurchaseOrder]

func (po PurchaseOrder) CheckTransactionLock(ctx context.Context) error {
	if err := validateTransactionLock(ctx, po.OrderDate, po.BusinessId, PurchaseTransactionLock); err != nil {
		return err
	}
	for _, detail := range po.Details {
		if err := ValidateValueAdjustment(ctx, po.BusinessId, po.OrderDate, detail.ProductType, detail.ProductId, &detail.BatchNumber); err != nil {
			return err
		}
	}
	return nil
}

// returns decoded curosr string
func (po PurchaseOrder) GetCursor() string {
	return po.CreatedAt.String()
}

func (po PurchaseOrder) BillId(ctx context.Context) (int, error) {
	db := config.GetDB()
	var id int
	err := db.WithContext(ctx).Model(&Bill{}).Where("purchase_order_id = ?", po.ID).Select("id").Scan(&id).Error
	if err == gorm.ErrRecordNotFound {
		return 0, nil
	}
	return id, err
}

func (po *PurchaseOrder) GetFieldValues(tx *gorm.DB) (*utils.DetailFieldValues, error) {
	return utils.FetchDetailFieldValues(tx, &PurchaseOrderDetail{}, "purchase_order_id", po.ID)
}

// func (po *PurchaseOrder) GetProductIDs(tx *gorm.DB) ([]int, error) {
// 	var productIDs []int
// 	err := tx.Model(&PurchaseOrderDetail{}).
// 			Where("purchase_order_id = ?", po.ID).
// 			Pluck("product_id", &productIDs).Error
// 	if err != nil {
// 		return nil, err
// 	}

// 	return productIDs, nil
// }

func (item *PurchaseOrderDetail) CalculateItemDiscountAndTax(ctx context.Context, isTaxInclusive bool) {

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
	if item.DetailTaxId > 0 && item.DetailTaxType != nil {
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

func updateItemDetailTotal(item *PurchaseOrderDetail, isDetailTaxInclusive bool, orderSubtotal decimal.Decimal, totalExclusiveTaxAmount decimal.Decimal, totalDetailDiscountAmount decimal.Decimal, totalDetailTaxAmount decimal.Decimal) (decimal.Decimal, decimal.Decimal, decimal.Decimal, decimal.Decimal) {

	// var orderSubtotal, totalExclusiveTaxAmount, totalDetailDiscountAmount, totalDetailTaxAmount decimal.Decimal

	orderSubtotal = orderSubtotal.Add(item.DetailTotalAmount)
	totalDetailDiscountAmount = totalDetailDiscountAmount.Add(item.DetailDiscountAmount)
	totalDetailTaxAmount = totalDetailTaxAmount.Add(item.DetailTaxAmount)
	if isDetailTaxInclusive {
		totalExclusiveTaxAmount = totalExclusiveTaxAmount.Add(decimal.NewFromFloat(0.0))
	} else {
		totalExclusiveTaxAmount = totalExclusiveTaxAmount.Add(item.DetailTaxAmount)
	}

	return orderSubtotal, totalDetailDiscountAmount, totalDetailTaxAmount, totalExclusiveTaxAmount
}

func (input NewPurchaseOrder) validate(ctx context.Context, businessId string, _ int) error {

	// exists supplier
	if err := utils.ValidateResourceId[Supplier](ctx, businessId, input.SupplierId); err != nil {
		return errors.New("supplier not found")
	}
	// exists branch
	if err := utils.ValidateResourceId[Branch](ctx, businessId, input.BranchId); err != nil {
		return errors.New("branch not found")
	}
	// exists warehouse
	if err := utils.ValidateResourceId[Warehouse](ctx, businessId, input.WarehouseId); err != nil {
		return errors.New("warehouse not found")
	}
	// exists tax
	if input.OrderTaxType != nil {
		if err := validateTaxExists(ctx, businessId, input.OrderTaxId, *input.OrderTaxType); err != nil {
			return err
		}
	}
	// validate transaction date
	if err := validateTransactionLock(ctx, input.OrderDate, businessId, PurchaseTransactionLock); err != nil {
		return err
	}
	for _, detail := range input.Details {
		if err := ValidateValueAdjustment(ctx, businessId, input.OrderDate, detail.ProductType, detail.ProductId, &detail.BatchNumber); err != nil {
			return err
		}
	}

	return nil
}

func CreatePurchaseOrder(ctx context.Context, input *NewPurchaseOrder) (*PurchaseOrder, error) {
	db := config.GetDB()

	businessId, ok := utils.GetBusinessIdFromContext(ctx)
	if !ok || businessId == "" {
		return nil, errors.New("business id is required")
	}

	// IMPORTANT (correctness): if callers request "Confirmed" on create, we still create as Draft
	// and then transition Draft -> Confirmed inside the same DB transaction.
	requestedStatus := input.CurrentStatus

	// validate PurchaseOrder
	if err := input.validate(ctx, businessId, 0); err != nil {
		return nil, err
	}

	// construct Images
	documents, err := mapNewDocuments(input.Documents, "purchase_orders", 0)
	if err != nil {
		return nil, err
	}

	var purchaseOrderItems []PurchaseOrderDetail
	var orderSubtotal,
		orderTotalAmount,
		totalExclusiveTaxAmount,
		totalDetailDiscountAmount,
		totalDetailTaxAmount decimal.Decimal

	for _, item := range input.Details {
		purchaseOrderItem := PurchaseOrderDetail{
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
		purchaseOrderItem.CalculateItemDiscountAndTax(ctx, *input.IsTaxInclusive)

		orderSubtotal = orderSubtotal.Add(purchaseOrderItem.DetailTotalAmount)
		totalDetailDiscountAmount = totalDetailDiscountAmount.Add(purchaseOrderItem.DetailDiscountAmount)
		totalDetailTaxAmount = totalDetailTaxAmount.Add(purchaseOrderItem.DetailTaxAmount)

		if input.IsTaxInclusive != nil && *input.IsTaxInclusive {
			totalExclusiveTaxAmount = totalExclusiveTaxAmount.Add(decimal.NewFromFloat(0.0))
		} else {
			totalExclusiveTaxAmount = totalExclusiveTaxAmount.Add(purchaseOrderItem.DetailTaxAmount)
		}

		// Add the item to the PurchaseOrder
		purchaseOrderItems = append(purchaseOrderItems, purchaseOrderItem)

	}

	// calculate order discount
	var orderDiscountAmount decimal.Decimal

	if input.OrderDiscountType != nil {
		orderDiscountAmount = utils.CalculateDiscountAmount(orderSubtotal, input.OrderDiscount, string(*input.OrderDiscountType))
	}

	// orderSubtotal = orderSubtotal.Sub(orderDiscountAmount)

	// calculate order tax amount (always exclusive)
	var orderTaxAmount decimal.Decimal
	if input.OrderTaxType != nil && input.OrderTaxId > 0 {
		if *input.OrderTaxType == TaxTypeGroup {
			orderTaxAmount = utils.CalculateTaxAmount(ctx, db, input.OrderTaxId, true, orderSubtotal, false)
		} else {
			orderTaxAmount = utils.CalculateTaxAmount(ctx, db, input.OrderTaxId, false, orderSubtotal, false)
		}
	} else {
		orderTaxAmount = decimal.NewFromFloat(0)
	}

	// Sum (order discount + total detail discount)
	totalOrderDiscountAmount := orderDiscountAmount.Add(totalDetailDiscountAmount)
	// Sum (order tax amount + total detail tax amount)
	totalOrderTaxAmount := orderTaxAmount.Add(totalDetailTaxAmount)

	orderTotalAmount = orderSubtotal.Add(orderTaxAmount).Add(totalExclusiveTaxAmount).Add(input.AdjustmentAmount).Sub(orderDiscountAmount)

	// store purchaseOrder
	purchaseOrder := PurchaseOrder{
		BusinessId:                  businessId,
		SupplierId:                  input.SupplierId,
		BranchId:                    input.BranchId,
		ReferenceNumber:             input.ReferenceNumber,
		OrderDate:                   input.OrderDate,
		ExpectedDeliveryDate:        input.ExpectedDeliveryDate,
		OrderPaymentTerms:           input.OrderPaymentTerms,
		OrderPaymentTermsCustomDays: input.OrderPaymentTermsCustomDays,
		DeliveryWarehouseId:         input.DeliveryWarehouseId,
		DeliveryCustomerId:          input.DeliveryCustomerId,
		DeliveryAddress:             input.DeliveryAddress,
		ShipmentPreferenceId:        input.ShipmentPreferenceId,
		Notes:                       input.Notes,
		TermsAndConditions:          input.TermsAndConditions,
		CurrencyId:                  input.CurrencyId,
		ExchangeRate:                input.ExchangeRate,
		OrderDiscount:               input.OrderDiscount,
		OrderDiscountType:           input.OrderDiscountType,
		OrderDiscountAmount:         orderDiscountAmount,
		AdjustmentAmount:            input.AdjustmentAmount,
		OrderTaxId:                  input.OrderTaxId,
		OrderTaxType:                input.OrderTaxType,
		OrderTaxAmount:              orderTaxAmount,
		CurrentStatus:               PurchaseOrderStatusDraft,
		Documents:                   documents,
		WarehouseId:                 input.WarehouseId,
		Details:                     purchaseOrderItems,
		OrderTotalDiscountAmount:    totalOrderDiscountAmount,
		OrderTotalTaxAmount:         totalOrderTaxAmount,
		OrderSubtotal:               orderSubtotal,
		OrderTotalAmount:            orderTotalAmount,
		IsTaxInclusive:              input.IsTaxInclusive,
	}

	tx := db.Begin()

	seqNo, err := utils.GetSequence[PurchaseOrder](ctx, businessId)
	if err != nil {
		tx.Rollback()
		return nil, err
	}
	prefix, err := getTransactionPrefix(ctx, input.BranchId, "Purchase Order")
	if err != nil {
		tx.Rollback()
		return nil, err
	}
	purchaseOrder.SequenceNo = decimal.NewFromInt(seqNo)
	purchaseOrder.OrderNumber = prefix + fmt.Sprint(seqNo)

	// IMPORTANT: always rollback on early-return or panic to avoid leaking DB locks
	// (leaked transactions are a common cause of MySQL 1205 lock wait timeouts).
	defer func() {
		if r := recover(); r != nil {
			_ = tx.Rollback().Error
			panic(r)
		}
	}()
	defer func() { _ = tx.Rollback().Error }()

	err = tx.WithContext(ctx).Create(&purchaseOrder).Error
	if err != nil {
		tx.Rollback()
		return nil, err
	}

	// Reload with Details so stock side-effects can access them.
	if err := tx.WithContext(ctx).Preload("Details").First(&purchaseOrder, purchaseOrder.ID).Error; err != nil {
		tx.Rollback()
		return nil, err
	}

	// If requested "Confirmed", apply the status transition deterministically (Draft -> Confirmed).
	if requestedStatus == PurchaseOrderStatusConfirmed {
		if err := tx.WithContext(ctx).Model(&purchaseOrder).Update("CurrentStatus", PurchaseOrderStatusConfirmed).Error; err != nil {
			tx.Rollback()
			return nil, err
		}
		purchaseOrder.CurrentStatus = PurchaseOrderStatusConfirmed

		// Apply inventory side-effects deterministically (prefer explicit command handler).
		if config.UseStockCommandsFor("PURCHASE_ORDER") {
			if err := ApplyPurchaseOrderStockForStatusTransition(tx.WithContext(ctx), &purchaseOrder, PurchaseOrderStatusDraft); err != nil {
				tx.Rollback()
				return nil, err
			}
		} else {
			if err := purchaseOrder.AfterUpdateCurrentStatus(tx.WithContext(ctx), string(PurchaseOrderStatusDraft)); err != nil {
				tx.Rollback()
				return nil, err
			}
		}
	}

	if err := tx.Commit().Error; err != nil {
		return nil, err
	}
	return &purchaseOrder, nil
}

func UpdatePurchaseOrder(ctx context.Context, purchaseOrderID int, updatedOrder *NewPurchaseOrder) (*PurchaseOrder, error) {
	db := config.GetDB()

	businessId, ok := utils.GetBusinessIdFromContext(ctx)
	if !ok || businessId == "" {
		return nil, errors.New("business id is required")
	}

	if err := updatedOrder.validate(ctx, businessId, purchaseOrderID); err != nil {
		return nil, err
	}

	// Fetch the existing purchase order
	existingOrder, err := utils.FetchModelForChange[PurchaseOrder](ctx, businessId, purchaseOrderID, "Details")
	if err != nil {
		return nil, err
	}

	if existingOrder.CurrentStatus == PurchaseOrderStatusPartiallyBilled {
		return nil, errors.New("purchase orders that are converted to bill cannot edit")
	}
	// Fintech integrity guardrail (behind flag): inventory-affecting docs are immutable after confirm.
	if config.StrictInventoryDocImmutability() && existingOrder.CurrentStatus == PurchaseOrderStatusConfirmed {
		return nil, errors.New("cannot edit a confirmed purchase order; cancel and recreate to preserve inventory commitments integrity")
	}

	oldStatus := existingOrder.CurrentStatus

	// Update the fields of the existing purchase order with the provided updated details
	existingOrder.SupplierId = updatedOrder.SupplierId
	existingOrder.BranchId = updatedOrder.BranchId
	existingOrder.ReferenceNumber = updatedOrder.ReferenceNumber
	existingOrder.OrderDate = updatedOrder.OrderDate
	existingOrder.ExpectedDeliveryDate = updatedOrder.ExpectedDeliveryDate
	existingOrder.OrderPaymentTerms = updatedOrder.OrderPaymentTerms
	existingOrder.OrderPaymentTermsCustomDays = updatedOrder.OrderPaymentTermsCustomDays
	existingOrder.DeliveryWarehouseId = updatedOrder.DeliveryWarehouseId
	existingOrder.DeliveryCustomerId = updatedOrder.DeliveryCustomerId
	existingOrder.DeliveryAddress = updatedOrder.DeliveryAddress
	existingOrder.ShipmentPreferenceId = updatedOrder.ShipmentPreferenceId
	existingOrder.Notes = updatedOrder.Notes
	existingOrder.TermsAndConditions = updatedOrder.TermsAndConditions
	existingOrder.CurrencyId = updatedOrder.CurrencyId
	existingOrder.ExchangeRate = updatedOrder.ExchangeRate
	existingOrder.OrderDiscount = updatedOrder.OrderDiscount
	existingOrder.OrderDiscountType = updatedOrder.OrderDiscountType
	existingOrder.AdjustmentAmount = updatedOrder.AdjustmentAmount
	existingOrder.OrderTaxId = updatedOrder.OrderTaxId
	existingOrder.OrderTaxType = updatedOrder.OrderTaxType
	existingOrder.CurrentStatus = updatedOrder.CurrentStatus
	existingOrder.WarehouseId = updatedOrder.WarehouseId
	existingOrder.IsTaxInclusive = updatedOrder.IsTaxInclusive

	tx := db.Begin()

	var orderSubtotal,
		orderTotalAmount,
		totalExclusiveTaxAmount,
		totalDetailDiscountAmount,
		totalDetailTaxAmount decimal.Decimal

	// Iterate through the updated items
	for _, updatedItem := range updatedOrder.Details {
		var existingItem *PurchaseOrderDetail

		// Check if the item already exists in the purchase order
		for _, item := range existingOrder.Details {
			if item.ID == updatedItem.DetailId {
				existingItem = &item
				break
			}
		}

		// If the item doesn't exist, add it to the purchase order
		if existingItem == nil {
			newItem := PurchaseOrderDetail{
				PurchaseOrderId:    purchaseOrderID,
				ProductId:          updatedItem.ProductId,
				ProductType:        updatedItem.ProductType,
				BatchNumber:        updatedItem.BatchNumber,
				Name:               updatedItem.Name,
				Description:        updatedItem.Description,
				DetailAccountId:    updatedItem.DetailAccountId,
				DetailQty:          updatedItem.DetailQty,
				DetailUnitRate:     updatedItem.DetailUnitRate,
				DetailTaxId:        updatedItem.DetailTaxId,
				DetailTaxType:      updatedItem.DetailTaxType,
				DetailDiscount:     updatedItem.DetailDiscount,
				DetailDiscountType: updatedItem.DetailDiscountType,
			}

			// Calculate tax and total amounts for the item
			newItem.CalculateItemDiscountAndTax(ctx, *updatedOrder.IsTaxInclusive)
			orderSubtotal, totalDetailDiscountAmount, totalDetailTaxAmount, totalExclusiveTaxAmount = updateItemDetailTotal(&newItem, *updatedOrder.IsTaxInclusive, orderSubtotal, totalExclusiveTaxAmount, totalDetailDiscountAmount, totalDetailTaxAmount)
			existingOrder.Details = append(existingOrder.Details, newItem)

			if newItem.ProductId > 0 && existingOrder.CurrentStatus == PurchaseOrderStatusConfirmed && updatedOrder.CurrentStatus == PurchaseOrderStatusConfirmed {

				product, err := GetProductOrVariant(ctx, string(newItem.ProductType), newItem.ProductId)
				if err != nil {
					tx.Rollback()
					return nil, err
				}

				if product.GetInventoryAccountID() > 0 {
					if err := UpdateStockSummaryOrderQty(tx, existingOrder.BusinessId, existingOrder.WarehouseId, newItem.ProductId, string(newItem.ProductType), newItem.BatchNumber, newItem.DetailQty, existingOrder.OrderDate); err != nil {
						tx.Rollback()
						return nil, err
					}
				}
			}

		} else {
			if updatedItem.IsDeletedItem != nil && *updatedItem.IsDeletedItem {
				// Find the index of the item to delete
				for i, item := range existingOrder.Details {
					if item.ID == updatedItem.DetailId {
						// reduced order qty from stock summary if po is confirmed
						if item.ProductId > 0 && existingOrder.CurrentStatus == PurchaseOrderStatusConfirmed {

							product, err := GetProductOrVariant(ctx, string(item.ProductType), item.ProductId)
							if err != nil {
								tx.Rollback()
								return nil, err
							}

							if product.GetInventoryAccountID() > 0 {
								if err := UpdateStockSummaryOrderQty(tx, existingOrder.BusinessId, existingOrder.WarehouseId, item.ProductId, string(item.ProductType), item.BatchNumber, item.DetailQty.Neg(), existingOrder.OrderDate); err != nil {
									tx.Rollback()
									return nil, err
								}
							}
						}
						// Delete the item from the database
						if err := tx.WithContext(ctx).Delete(&existingOrder.Details[i]).Error; err != nil {
							tx.Rollback()
							return nil, err
						}
						// Remove the item from the slice
						existingOrder.Details = append(existingOrder.Details[:i], existingOrder.Details[i+1:]...)
						break // Exit the loop after deleting the item
					}
				}
			} else {
				// Update existing item details
				existingItem.ProductId = updatedItem.ProductId
				existingItem.ProductType = updatedItem.ProductType
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
				existingItem.CalculateItemDiscountAndTax(ctx, *updatedOrder.IsTaxInclusive)
				orderSubtotal, totalDetailDiscountAmount, totalDetailTaxAmount, totalExclusiveTaxAmount = updateItemDetailTotal(existingItem, *updatedOrder.IsTaxInclusive, orderSubtotal, totalExclusiveTaxAmount, totalDetailDiscountAmount, totalDetailTaxAmount)

				if err := tx.WithContext(ctx).Save(&existingItem).Error; err != nil {
					tx.Rollback()
					return nil, err
				}
			}
		}
	}

	// calculate order discount
	var orderDiscountAmount decimal.Decimal

	if updatedOrder.OrderDiscountType != nil {
		orderDiscountAmount = utils.CalculateDiscountAmount(orderSubtotal, updatedOrder.OrderDiscount, string(*updatedOrder.OrderDiscountType))
	}

	existingOrder.OrderDiscountAmount = orderDiscountAmount

	// orderSubtotal = orderSubtotal.Sub(orderDiscountAmount)
	existingOrder.OrderSubtotal = orderSubtotal

	// calculate order tax amount (always exclusive)
	var orderTaxAmount decimal.Decimal
	if updatedOrder.OrderTaxId > 0 && updatedOrder.OrderTaxType != nil {
		if *updatedOrder.OrderTaxType == TaxTypeGroup {
			orderTaxAmount = utils.CalculateTaxAmount(ctx, db, updatedOrder.OrderTaxId, true, orderSubtotal, false)
		} else {
			orderTaxAmount = utils.CalculateTaxAmount(ctx, db, updatedOrder.OrderTaxId, false, orderSubtotal, false)
		}
	} else {
		orderTaxAmount = decimal.NewFromFloat(0)
	}

	existingOrder.OrderTaxAmount = orderTaxAmount

	// Sum (order discount + total detail discount)
	totalOrderDiscountAmount := orderDiscountAmount.Add(totalDetailDiscountAmount)
	existingOrder.OrderTotalDiscountAmount = totalOrderDiscountAmount

	// Sum (order tax amount + total detail tax amount)
	totalOrderTaxAmount := orderTaxAmount.Add(totalDetailTaxAmount)
	existingOrder.OrderTotalTaxAmount = totalOrderTaxAmount
	// Sum Grand total amount (subtotal+ exclusive tax + adj amount)
	orderTotalAmount = orderSubtotal.Add(orderTaxAmount).Add(totalExclusiveTaxAmount).Add(updatedOrder.AdjustmentAmount).Sub(orderDiscountAmount)

	existingOrder.OrderTotalAmount = orderTotalAmount

	// Save the updated purchase order to the database
	if err := tx.WithContext(ctx).Save(&existingOrder).Error; err != nil {
		tx.Rollback()
		return nil, err
	}

	// Refresh the existingOrder to get the latest details
	if err := tx.WithContext(ctx).Preload("Details").First(&existingOrder, purchaseOrderID).Error; err != nil {
		tx.Rollback()
		return nil, err
	}

	// Apply inventory side-effects deterministically (prefer explicit command handler).
	if config.UseStockCommandsFor("PURCHASE_ORDER") {
		if err := ApplyPurchaseOrderStockForStatusTransition(tx.WithContext(ctx), existingOrder, oldStatus); err != nil {
			tx.Rollback()
			return nil, err
		}
	} else {
		if err := existingOrder.AfterUpdateCurrentStatus(tx.WithContext(ctx), string(oldStatus)); err != nil {
			tx.Rollback()
			return nil, err
		}
	}

	documents, err := upsertDocuments(ctx, tx, updatedOrder.Documents, "purchase_orders", purchaseOrderID)
	if err != nil {
		tx.Rollback()
		return nil, err
	}
	existingOrder.Documents = documents

	if err := tx.Commit().Error; err != nil {
		return nil, err
	}

	return existingOrder, nil
}

func DeletePurchaseOrder(ctx context.Context, id int) (*PurchaseOrder, error) {
	businessId, ok := utils.GetBusinessIdFromContext(ctx)
	if !ok || businessId == "" {
		return nil, errors.New("business id is required")
	}

	result, err := utils.FetchModelForChange[PurchaseOrder](ctx, businessId, id, "Details", "Documents")
	if err != nil {
		return nil, err
	}

	if result.CurrentStatus == PurchaseOrderStatusClosed {
		return nil, errors.New("cannot delete purchase order that is already closed")
	}

	if result.CurrentStatus == PurchaseOrderStatusPartiallyBilled {
		return nil, errors.New("purchase orders that are converted to bill cannot be deleted")
	}

	db := config.GetDB()
	tx := db.Begin()
	// reduced order qty from stock summary if po is confirmed
	if result.CurrentStatus == PurchaseOrderStatusConfirmed {
		for _, poItem := range result.Details {
			if poItem.ProductId > 0 {
				product, err := GetProductOrVariant(ctx, string(poItem.ProductType), poItem.ProductId)
				if err != nil {
					tx.Rollback()
					return nil, err
				}

				if product.GetInventoryAccountID() > 0 {
					if err := UpdateStockSummaryOrderQty(tx, result.BusinessId, result.WarehouseId, poItem.ProductId, string(poItem.ProductType), poItem.BatchNumber, poItem.DetailQty.Neg(), result.OrderDate); err != nil {
						tx.Rollback()
						return nil, err
					}
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

	if err := tx.Commit().Error; err != nil {
		return nil, err
	}

	return result, nil
}

func GetPurchaseOrder(ctx context.Context, id int) (*PurchaseOrder, error) {
	// fieldNames, err := utils.GetQueryFields(ctx, &PurchaseOrder{})
	// if err != nil {
	// 	return nil, err
	// }
	businessId, ok := utils.GetBusinessIdFromContext(ctx)
	if !ok || businessId == "" {
		return nil, errors.New("business id is required")
	}

	return utils.FetchModel[PurchaseOrder](ctx, businessId, id)
}

func GetPurchaseOrders(ctx context.Context, order_number *string) ([]*PurchaseOrder, error) {
	db := config.GetDB()
	var results []*PurchaseOrder

	businessId, ok := utils.GetBusinessIdFromContext(ctx)
	if !ok || businessId == "" {
		return nil, errors.New("business id is required")
	}

	dbCtx := db.WithContext(ctx).Where("business_id = ?", businessId)
	if order_number != nil && len(*order_number) > 0 {
		dbCtx = dbCtx.Where("order_number LIKE ?", "%"+*order_number+"%")
	}
	err := dbCtx.
		Find(&results).Error
	if err != nil {
		return nil, err
	}
	return results, nil
}

func UpdateStatusPurchaseOrder(ctx context.Context, id int, status string) (*PurchaseOrder, error) {
	businessId, ok := utils.GetBusinessIdFromContext(ctx)
	if !ok || businessId == "" {
		return nil, errors.New("business id is required")
	}

	po, err := utils.FetchModelForChange[PurchaseOrder](ctx, businessId, id, "Details")

	if err != nil {
		return nil, err
	}

	if po.CurrentStatus == PurchaseOrderStatusClosed {
		return nil, errors.New("cannot update purchase order that is already closed")
	}

	if po.CurrentStatus == PurchaseOrderStatusPartiallyBilled && status == string(PurchaseOrderStatusCancelled) {
		return nil, errors.New("purchase orders that are converted to bill cannot be cancelled.")
	}

	oldStatus := po.CurrentStatus

	// err = tx.WithContext(ctx).Model(&po).Updates(map[string]interface{}{
	// 	"CurrentStatus": status,
	// }).Error

	// update CurrentStatus without hook
	// db action
	db := config.GetDB()
	tx := db.Begin()
	if err := tx.WithContext(ctx).Model(&po).UpdateColumn("CurrentStatus", status).Error; err != nil {
		tx.Rollback()
		return nil, err
	}

	// Apply inventory side-effects deterministically (prefer explicit command handler).
	if config.UseStockCommandsFor("PURCHASE_ORDER") {
		po.CurrentStatus = PurchaseOrderStatus(status)
		if err := ApplyPurchaseOrderStockForStatusTransition(tx.WithContext(ctx), po, oldStatus); err != nil {
			tx.Rollback()
			return nil, err
		}
	} else {
		if err := po.AfterUpdateCurrentStatus(tx.WithContext(ctx), string(oldStatus)); err != nil {
			tx.Rollback()
			return nil, err
		}
	}

	if err := createHistory(tx.WithContext(ctx), "Update", id, "purchase_orders", nil, nil, "Updated current status to "+status); err != nil {
		tx.Rollback()
		return nil, err
	}

	// Commit the transaction
	if err := tx.Commit().Error; err != nil {
		return nil, err
	}

	return po, nil
}

func PaginatePurchaseOrder(
	ctx context.Context, limit *int, after *string,

	orderNumber *string,
	referenceNumber *string,

	branchID *int,
	warehouseID *int,
	supplierID *int,
	currentStatus *PurchaseOrderStatus,

	startOrderDate *MyDateString,
	endOrderDate *MyDateString,
	startExpectedDeliveryDate *MyDateString,
	endExpectedDeliveryDate *MyDateString,
) (*PurchaseOrdersConnection, error) {

	businessId, ok := utils.GetBusinessIdFromContext(ctx)
	if !ok || businessId == "" {
		return nil, errors.New("business id is required")
	}

	business, err := GetBusiness(ctx)
	if err != nil {
		return nil, errors.New("business id is required")
	}
	if err := startOrderDate.StartOfDayUTCTime(business.Timezone); err != nil {
		return nil, err
	}
	if err := endOrderDate.EndOfDayUTCTime(business.Timezone); err != nil {
		return nil, err
	}
	if err := startExpectedDeliveryDate.StartOfDayUTCTime(business.Timezone); err != nil {
		return nil, err
	}
	if err := endExpectedDeliveryDate.EndOfDayUTCTime(business.Timezone); err != nil {
		return nil, err
	}

	db := config.GetDB()
	dbCtx := db.WithContext(ctx).Where("business_id = ?", businessId)
	if orderNumber != nil && *orderNumber != "" {
		dbCtx.Where("order_number LIKE ?", "%"+*orderNumber+"%")
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
	if startOrderDate != nil && endOrderDate != nil {
		dbCtx.Where("order_date BETWEEN ? AND ?", startOrderDate, endOrderDate)
	}
	if startExpectedDeliveryDate != nil && endExpectedDeliveryDate != nil {
		dbCtx.Where("expected_delivery_date BETWEEN ? AND ?", startExpectedDeliveryDate, endExpectedDeliveryDate)
	}

	edges, pageInfo, err := FetchPageCompositeCursor[PurchaseOrder](dbCtx, *limit, after, "created_at", "<")
	if err != nil {
		return nil, err
	}
	var purchaseOrdersConnection PurchaseOrdersConnection
	purchaseOrdersConnection.PageInfo = pageInfo
	for _, edge := range edges {
		purchaseOrderEdge := PurchaseOrdersEdge(edge)
		purchaseOrdersConnection.Edges = append(purchaseOrdersConnection.Edges, &purchaseOrderEdge)
	}

	return &purchaseOrdersConnection, err
}

func UpdatePoDetailBilledQty(tx *gorm.DB, ctx context.Context, purchaseOrderId int, billItem BillDetail, action string, oldQty decimal.Decimal, inventoryAccId int) error {
	var poDetail PurchaseOrderDetail
	var err error
	if billItem.PurchaseOrderItemId > 0 {
		err = tx.WithContext(ctx).Where("id = ?", billItem.PurchaseOrderItemId).First(&poDetail).Error
		if err != nil {
			tx.Rollback()
			return errors.New("purchase order item not found")
		}

		var po PurchaseOrder
		if purchaseOrderId > 0 {
			// exists purchase order
			err := tx.Where("id = ?", purchaseOrderId).First(&po).Error
			if err != nil {
				return errors.New("purchase order not found")
			}
		}

		if action == "create" {

			if billItem.DetailQty.GreaterThan(poDetail.DetailQty.Sub(poDetail.DetailBilledQty)) {
				tx.Rollback()
				return errors.New("bill qty must be equal or less than purchase order qty")
			}
			poDetail.DetailBilledQty = poDetail.DetailBilledQty.Add(billItem.DetailQty)

			if inventoryAccId > 0 {
				if err := UpdateStockSummaryOrderQty(tx, po.BusinessId, po.WarehouseId, billItem.ProductId, string(billItem.ProductType), billItem.BatchNumber, billItem.DetailQty.Neg(), po.OrderDate); err != nil {
					tx.Rollback()
					return err
				}
			}

		} else if action == "update" {

			if billItem.DetailQty.GreaterThan(poDetail.DetailQty.Sub(poDetail.DetailBilledQty.Sub(oldQty))) {
				tx.Rollback()
				return errors.New("bill qty must be equal or less than purchase order qty")
			}
			poDetail.DetailBilledQty = poDetail.DetailBilledQty.Add(billItem.DetailQty.Sub(oldQty))

			if inventoryAccId > 0 {
				if err := UpdateStockSummaryOrderQty(tx, po.BusinessId, po.WarehouseId, billItem.ProductId, string(billItem.ProductType), billItem.BatchNumber, oldQty.Sub(billItem.DetailQty), po.OrderDate); err != nil {
					tx.Rollback()
					return err
				}
			}

		} else if action == "delete" {
			poDetail.DetailBilledQty = poDetail.DetailBilledQty.Sub(billItem.DetailQty)

			if inventoryAccId > 0 {
				if err := UpdateStockSummaryOrderQty(tx, po.BusinessId, po.WarehouseId, billItem.ProductId, string(billItem.ProductType), billItem.BatchNumber, billItem.DetailQty, po.OrderDate); err != nil {
					tx.Rollback()
					return err
				}
			}
		}

		if err := tx.WithContext(ctx).Save(&poDetail).Error; err != nil {
			tx.Rollback()
			return err
		}
	}
	return nil
}

func ChangePoCurrentStatus(tx *gorm.DB, ctx context.Context, businessId string, poId int) (*PurchaseOrder, error) {

	var purchaseOrder PurchaseOrder
	err := tx.Preload("Details").Where("business_id = ? AND id = ?", businessId, poId).First(&purchaseOrder).Error
	if err != nil {
		return nil, errors.New("purchase order not found at ChangePoCurrentStatus")
	}

	isPartialQtyBilled := false
	isNonBill := false

	for _, poItem := range purchaseOrder.Details {
		if poItem.DetailQty.Sub(poItem.DetailBilledQty).GreaterThan(decimal.NewFromFloat(0)) && !poItem.DetailBilledQty.Equal(decimal.NewFromFloat(0)) {
			isPartialQtyBilled = true
			break
		}

		if poItem.DetailBilledQty.Equal(decimal.NewFromFloat(0)) {
			isNonBill = true
		}

	}
	var status string

	if isPartialQtyBilled {
		status = string(PurchaseOrderStatusPartiallyBilled)
	} else if isNonBill {
		status = string(PurchaseOrderStatusConfirmed)
	} else {
		status = string(PurchaseOrderStatusClosed)
	}

	err = tx.Model(&purchaseOrder).Updates(map[string]interface{}{
		"CurrentStatus": status,
	}).Error
	if err != nil {
		tx.Rollback()
		return nil, err
	}

	return &purchaseOrder, nil
}
