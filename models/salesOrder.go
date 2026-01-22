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

type SalesOrder struct {
	ID                          int                `gorm:"primary_key" json:"id"`
	BusinessId                  string             `gorm:"index;not null" json:"business_id" binding:"required"`
	CustomerId                  int                `gorm:"index;not null" json:"customer_id" binding:"required"`
	BranchId                    int                `gorm:"index;not null" json:"branch_id" binding:"required"`
	OrderNumber                 string             `gorm:"size:255;not null" json:"order_number" binding:"required"`
	SequenceNo                  decimal.Decimal    `gorm:"type:decimal(15);not null" json:"sequence_no"`
	ReferenceNumber             string             `gorm:"size:255" json:"reference_number"`
	OrderDate                   time.Time          `gorm:"not null" json:"order_date" binding:"required"`
	ExpectedShipmentDate        *time.Time         `json:"expected_shipment_date"`
	OrderPaymentTerms           PaymentTerms       `gorm:"type:enum('Net15', 'Net30', 'Net45', 'Net60', 'DueMonthEnd', 'DueNextMonthEnd', 'DueOnReceipt', 'Custom');not null" json:"order_payment_terms" binding:"required"`
	OrderPaymentTermsCustomDays int                `gorm:"default:0" json:"order_payment_terms_custom_days"`
	DeliveryMethodId            int                `json:"delivery_method_id"`
	SalesPersonId               int                `json:"sales_person_id"`
	Notes                       string             `gorm:"type:text" json:"notes"`
	TermsAndConditions          string             `gorm:"type:text" json:"terms_and_conditions"`
	CurrencyId                  int                `gorm:"not null" json:"currency_id" binding:"required"`
	ExchangeRate                decimal.Decimal    `gorm:"type:decimal(20,4);default:0" json:"exchange_rate"`
	OrderDiscount               decimal.Decimal    `gorm:"type:decimal(20,4);default:0" json:"order_discount"`
	OrderDiscountType           *DiscountType      `gorm:"type:enum('P', 'A');default:null" json:"order_discount_type"`
	OrderDiscountAmount         decimal.Decimal    `gorm:"type:decimal(20,4);default:0" json:"order_discount_amount"`
	ShippingCharges             decimal.Decimal    `gorm:"type:decimal(20,4);default:0" json:"shipping_charges"`
	AdjustmentAmount            decimal.Decimal    `gorm:"type:decimal(20,4);default:0" json:"adjustment_amount"`
	IsTaxInclusive              *bool              `gorm:"not null;default:false" json:"is_tax_inclusive"`
	OrderTaxId                  int                `json:"order_tax_id"`
	OrderTaxType                *TaxType           `gorm:"type:enum('I', 'G');default:null" json:"order_tax_type"`
	OrderTaxAmount              decimal.Decimal    `gorm:"type:decimal(20,4);default:0" json:"order_tax_amount"`
	CurrentStatus               SalesOrderStatus   `gorm:"type:enum('Draft', 'Confirmed','Partially Invoiced', 'Closed', 'Cancelled');not null" json:"current_status" binding:"required"`
	Documents                   []*Document        `gorm:"polymorphic:Reference" json:"documents"`
	OrderSubtotal               decimal.Decimal    `gorm:"type:decimal(20,4);default:0" json:"order_subtotal"`
	OrderTotalDiscountAmount    decimal.Decimal    `gorm:"type:decimal(20,4);default:0" json:"order_total_discount_amount"`
	OrderTotalTaxAmount         decimal.Decimal    `gorm:"type:decimal(20,4);default:0" json:"order_total_tax_amount"`
	OrderTotalAmount            decimal.Decimal    `gorm:"type:decimal(20,4);default:0" json:"order_total_amount"`
	WarehouseId                 int                `gorm:"not null" json:"warehouse_id"`
	CreatedAt                   time.Time          `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt                   time.Time          `gorm:"autoUpdateTime" json:"updated_at"`
	Details                     []SalesOrderDetail `gorm:"foreignKey:SalesOrderId" json:"details"`
}

type NewSalesOrder struct {
	BusinessId                  string                `json:"business_id" binding:"required"`
	CustomerId                  int                   `json:"customer_id" binding:"required"`
	BranchId                    int                   `json:"branch_id" binding:"required"`
	ReferenceNumber             string                `json:"reference_number"`
	OrderDate                   time.Time             `json:"order_date" binding:"required"`
	ExpectedShipmentDate        *time.Time            `json:"expected_shipment_date"`
	OrderPaymentTerms           PaymentTerms          `json:"order_payment_terms" binding:"required"`
	OrderPaymentTermsCustomDays int                   `json:"order_payment_terms_custom_days"`
	DeliveryMethodId            int                   `json:"delivery_method_id"`
	SalesPersonId               int                   `json:"sales_person_id"`
	Notes                       string                `json:"notes"`
	TermsAndConditions          string                `json:"terms_and_conditions"`
	CurrencyId                  int                   `json:"currency_id" binding:"required"`
	ExchangeRate                decimal.Decimal       `json:"exchange_rate"`
	OrderDiscount               decimal.Decimal       `json:"order_discount"`
	OrderDiscountType           *DiscountType         `json:"order_discount_type"`
	ShippingCharges             decimal.Decimal       `json:"shipping_charges"`
	IsTaxInclusive              *bool                 `json:"is_tax_inclusive" binding:"required"`
	AdjustmentAmount            decimal.Decimal       `json:"adjustment_amount"`
	OrderTaxId                  int                   `json:"order_tax_id"`
	OrderTaxType                *TaxType              `json:"order_tax_type"`
	CurrentStatus               SalesOrderStatus      `json:"current_status" binding:"required"`
	WarehouseId                 int                   `json:"warehouse_id"`
	Documents                   []*NewDocument        `json:"documents"`
	Details                     []NewSalesOrderDetail `json:"details"`
}

type SalesOrderDetail struct {
	ID                   int             `gorm:"primary_key" json:"id"`
	SalesOrderId         int             `gorm:"index;not null" json:"sales_order_id" binding:"required"`
	ProductId            int             `json:"product_id"`
	ProductType          ProductType     `gorm:"type:enum('S','G','C','V','I');default:S" json:"product_type"`
	BatchNumber          string          `gorm:"size:100" json:"batch_number"`
	Name                 string          `gorm:"size:100" json:"name" binding:"required"`
	Description          string          `gorm:"size:255" json:"description"`
	DetailQty            decimal.Decimal `gorm:"type:decimal(20,4);default:0" json:"detail_qty" binding:"required"`
	DetailInvoicedQty    decimal.Decimal `gorm:"type:decimal(20,4);default:0" json:"detail_invoiced_qty"`
	DetailUnitRate       decimal.Decimal `gorm:"type:decimal(20,4);default:0" json:"detail_unit_rate" binding:"required"`
	DetailDiscount       decimal.Decimal `gorm:"type:decimal(20,4);default:0" json:"detail_discount"`
	DetailDiscountType   *DiscountType   `gorm:"type:enum('P', 'A');default:null" json:"detail_discount_type"`
	DetailTaxId          int             `json:"detail_tax_id"`
	DetailTaxType        *TaxType        `gorm:"type:enum('I', 'G');default:null" json:"detail_tax_type"`
	DetailAccountId      int             `gorm:"default:null" json:"detail_account_id"`
	DetailDiscountAmount decimal.Decimal `gorm:"type:decimal(20,4);default:0" json:"detail_discount_amount"`
	DetailTaxAmount      decimal.Decimal `gorm:"type:decimal(20,4);default:0" json:"detail_tax_amount"`
	DetailTotalAmount    decimal.Decimal `gorm:"type:decimal(20,4);default:0" json:"detail_total_amount"`
}

type NewSalesOrderDetail struct {
	DetailId           int             `json:"detail_id"`
	ProductId          int             `json:"product_id"`
	ProductType        ProductType     `json:"product_type"`
	BatchNumber        string          `json:"batch_number"`
	Name               string          `json:"name" binding:"required"`
	Description        string          `json:"description"`
	DetailQty          decimal.Decimal `json:"detail_qty" binding:"required"`
	DetailUnitRate     decimal.Decimal `json:"detail_unit_rate" binding:"required"`
	DetailDiscount     decimal.Decimal `json:"detail_discount"`
	DetailDiscountType *DiscountType   `json:"detail_discount_type"`
	DetailTaxId        int             `json:"detail_tax_id"`
	DetailTaxType      *TaxType        `json:"detail_tax_type"`
	DetailAccountId    int             `json:"detail_account_id"`
	IsDeletedItem      *bool           `json:"is_deleted_item"`
}

type SalesOrdersConnection struct {
	Edges    []*SalesOrdersEdge `json:"edges"`
	PageInfo *PageInfo          `json:"pageInfo"`
}

type SalesOrdersEdge Edge[SalesOrder]

func (so SalesOrder) InvoiceId(ctx context.Context) (int, error) {
	db := config.GetDB()
	var id int
	err := db.WithContext(ctx).Model(&SalesInvoice{}).Where("sales_order_id = ?", so.ID).Select("id").Scan(&id).Error
	if err == gorm.ErrRecordNotFound {
		return 0, nil
	}
	return id, err
}

func (so SalesOrder) GetCursor() string {
	return so.CreatedAt.String()
}

func (obj SalesOrder) GetId() int {
	return obj.ID
}
func (s *SalesOrder) GetFieldValues(tx *gorm.DB) (*utils.DetailFieldValues, error) {
	return utils.FetchDetailFieldValues(tx, &SalesOrderDetail{}, "sales_order_id", s.ID)
}

// validate transaction lock
func (so SalesOrder) CheckTransactionLock(ctx context.Context) error {
	if err := validateTransactionLock(ctx, so.OrderDate, so.BusinessId, SalesTransactionLock); err != nil {
		return err
	}
	// check for inventory value adjustment
	for _, detail := range so.Details {
		if err := ValidateValueAdjustment(ctx, so.BusinessId, so.OrderDate, detail.ProductType, detail.ProductId, &detail.BatchNumber); err != nil {
			return fmt.Errorf(err.Error(), detail.Name)
		}
	}
	return nil
}

func (input NewSalesOrder) validate(ctx context.Context, businessId string, _ int) error {

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
	// exists DeliveryMethod
	if input.DeliveryMethodId > 0 {
		// exists SalesPerson
		if err := utils.ValidateResourceId[DeliveryMethod](ctx, businessId, input.DeliveryMethodId); err != nil {
			return errors.New("deliveryMethod not found")
		}
	}

	// validate order date
	if err := validateTransactionLock(ctx, input.OrderDate, businessId, SalesTransactionLock); err != nil {
		return err
	}
	for _, detail := range input.Details {
		if err := ValidateValueAdjustment(ctx, businessId, input.OrderDate, detail.ProductType, detail.ProductId, &detail.BatchNumber); err != nil {
			return err
		}
	}

	return nil
}

func (item *SalesOrderDetail) CalculateSaleItemDiscountAndTax(ctx context.Context, isTaxInclusive bool) {

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

func updateSaleItemDetailTotal(item *SalesOrderDetail, isTaxInclusive bool, orderSubtotal decimal.Decimal, totalExclusiveTaxAmount decimal.Decimal, totalDetailDiscountAmount decimal.Decimal, totalDetailTaxAmount decimal.Decimal) (decimal.Decimal, decimal.Decimal, decimal.Decimal, decimal.Decimal) {

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

func CreateSalesOrder(ctx context.Context, input *NewSalesOrder) (*SalesOrder, error) {
	db := config.GetDB()

	businessId, ok := utils.GetBusinessIdFromContext(ctx)
	if !ok || businessId == "" {
		return nil, errors.New("business id is required")
	}

	// IMPORTANT (correctness): if callers request "Confirmed" on create, we still create as Draft
	// and then transition Draft -> Confirmed inside the same DB transaction.
	requestedStatus := input.CurrentStatus

	// validate SalesOrder
	if err := input.validate(ctx, businessId, 0); err != nil {
		return nil, err
	}

	// construct Images
	documents, err := mapNewDocuments(input.Documents, "sales_orders", 0)
	if err != nil {
		return nil, err
	}

	var saleOrderItems []SalesOrderDetail
	var orderSubtotal,
		orderTotalAmount,
		totalExclusiveTaxAmount,
		totalDetailDiscountAmount,
		totalDetailTaxAmount decimal.Decimal

	for _, item := range input.Details {
		saleOrderItem := SalesOrderDetail{
			ProductId:          item.ProductId,
			ProductType:        item.ProductType,
			BatchNumber:        item.BatchNumber,
			Name:               item.Name,
			Description:        item.Description,
			DetailQty:          item.DetailQty,
			DetailUnitRate:     item.DetailUnitRate,
			DetailTaxId:        item.DetailTaxId,
			DetailTaxType:      item.DetailTaxType,
			DetailDiscount:     item.DetailDiscount,
			DetailDiscountType: item.DetailDiscountType,
			DetailAccountId:    item.DetailAccountId,
		}

		// Calculate tax and total amounts for the item
		saleOrderItem.CalculateSaleItemDiscountAndTax(ctx, *input.IsTaxInclusive)

		orderSubtotal = orderSubtotal.Add(saleOrderItem.DetailTotalAmount)
		totalDetailDiscountAmount = totalDetailDiscountAmount.Add(saleOrderItem.DetailDiscountAmount)
		totalDetailTaxAmount = totalDetailTaxAmount.Add(saleOrderItem.DetailTaxAmount)

		if input.IsTaxInclusive != nil && *input.IsTaxInclusive {
			totalExclusiveTaxAmount = totalExclusiveTaxAmount.Add(decimal.NewFromFloat(0.0))
		} else {
			totalExclusiveTaxAmount = totalExclusiveTaxAmount.Add(saleOrderItem.DetailTaxAmount)
		}

		// Add the item to the SalesOrder
		saleOrderItems = append(saleOrderItems, saleOrderItem)

	}

	// calculate order discount
	var orderDiscountAmount decimal.Decimal

	if input.OrderDiscountType != nil {
		orderDiscountAmount = utils.CalculateDiscountAmount(orderSubtotal, input.OrderDiscount, string(*input.OrderDiscountType))
	}

	// orderSubtotal = orderSubtotal.Sub(orderDiscountAmount)

	// calculate order tax amount (always exclusive)
	var orderTaxAmount decimal.Decimal
	if input.OrderTaxId > 0 && input.OrderTaxType != nil {
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

	orderTotalAmount = orderSubtotal.Add(orderTaxAmount).Add(totalExclusiveTaxAmount).Add(input.AdjustmentAmount).Add(input.ShippingCharges).Sub(orderDiscountAmount)

	// store saleOrder
	saleOrder := SalesOrder{
		BusinessId:                  businessId,
		CustomerId:                  input.CustomerId,
		BranchId:                    input.BranchId,
		ReferenceNumber:             input.ReferenceNumber,
		OrderDate:                   input.OrderDate,
		ExpectedShipmentDate:        input.ExpectedShipmentDate,
		OrderPaymentTerms:           input.OrderPaymentTerms,
		OrderPaymentTermsCustomDays: input.OrderPaymentTermsCustomDays,
		DeliveryMethodId:            input.DeliveryMethodId,
		SalesPersonId:               input.SalesPersonId,
		Notes:                       input.Notes,
		TermsAndConditions:          input.TermsAndConditions,
		CurrencyId:                  input.CurrencyId,
		ExchangeRate:                input.ExchangeRate,
		OrderDiscount:               input.OrderDiscount,
		OrderDiscountType:           input.OrderDiscountType,
		OrderDiscountAmount:         orderDiscountAmount,
		ShippingCharges:             input.ShippingCharges,
		AdjustmentAmount:            input.AdjustmentAmount,
		IsTaxInclusive:              input.IsTaxInclusive,
		OrderTaxId:                  input.OrderTaxId,
		OrderTaxType:                input.OrderTaxType,
		OrderTaxAmount:              orderTaxAmount,
		CurrentStatus:               SalesOrderStatusDraft,
		Documents:                   documents,
		Details:                     saleOrderItems,
		OrderTotalDiscountAmount:    totalOrderDiscountAmount,
		OrderTotalTaxAmount:         totalOrderTaxAmount,
		OrderSubtotal:               orderSubtotal,
		OrderTotalAmount:            orderTotalAmount,
		WarehouseId:                 input.WarehouseId,
	}

	tx := db.Begin()
	seqNo, err := utils.GetSequence[SalesOrder](ctx, businessId)
	if err != nil {
		tx.Rollback()
		return nil, err
	}
	prefix, err := getTransactionPrefix(ctx, input.BranchId, "Sales Order")
	if err != nil {
		tx.Rollback()
		return nil, err
	}
	saleOrder.SequenceNo = decimal.NewFromInt(seqNo)
	saleOrder.OrderNumber = prefix + fmt.Sprint(seqNo)

	err = tx.WithContext(ctx).Create(&saleOrder).Error
	if err != nil {
		tx.Rollback()
		return nil, err
	}

	// If requested "Confirmed", apply the status transition deterministically (Draft -> Confirmed).
	if requestedStatus == SalesOrderStatusConfirmed {
		if err := tx.WithContext(ctx).Model(&saleOrder).Update("CurrentStatus", SalesOrderStatusConfirmed).Error; err != nil {
			tx.Rollback()
			return nil, err
		}
		saleOrder.CurrentStatus = SalesOrderStatusConfirmed

		// Apply committed stock side-effects deterministically (prefer explicit command handler).
		if config.UseStockCommandsFor("SALES_ORDER") {
			if err := ApplySalesOrderStockForStatusTransition(tx.WithContext(ctx), &saleOrder, SalesOrderStatusDraft); err != nil {
				tx.Rollback()
				return nil, err
			}
		} else {
			if err := saleOrder.AfterUpdateCurrentStatus(tx.WithContext(ctx), string(SalesOrderStatusDraft)); err != nil {
				tx.Rollback()
				return nil, err
			}
		}
	}

	// Commit the transaction
	if err := tx.Commit().Error; err != nil {
		return nil, err
	}

	return &saleOrder, nil
}

func UpdateSalesOrder(ctx context.Context, saleOrderId int, updatedOrder *NewSalesOrder) (*SalesOrder, error) {

	businessId, ok := utils.GetBusinessIdFromContext(ctx)
	if !ok || businessId == "" {
		return nil, errors.New("business id is required")
	}

	// validate SalesOrder
	if err := updatedOrder.validate(ctx, businessId, saleOrderId); err != nil {
		return nil, err
	}
	existingOrder, err := utils.FetchModelForChange[SalesOrder](ctx, businessId, saleOrderId, "Details")
	if err != nil {
		return nil, err
	}
	// Fintech integrity guardrail (behind flag): inventory-affecting docs are immutable after confirm.
	if config.StrictInventoryDocImmutability() && existingOrder.CurrentStatus == SalesOrderStatusConfirmed {
		return nil, errors.New("cannot edit a confirmed sales order; cancel and recreate to preserve inventory commitments integrity")
	}

	db := config.GetDB()
	tx := db.Begin()

	oldStatus := existingOrder.CurrentStatus

	// Update the fields of the existing purchase order with the provided updated details
	existingOrder.CustomerId = updatedOrder.CustomerId
	existingOrder.BranchId = updatedOrder.BranchId
	existingOrder.ReferenceNumber = updatedOrder.ReferenceNumber
	existingOrder.OrderDate = updatedOrder.OrderDate
	existingOrder.ExpectedShipmentDate = updatedOrder.ExpectedShipmentDate
	existingOrder.OrderPaymentTerms = updatedOrder.OrderPaymentTerms
	existingOrder.OrderPaymentTermsCustomDays = updatedOrder.OrderPaymentTermsCustomDays
	existingOrder.DeliveryMethodId = updatedOrder.DeliveryMethodId
	existingOrder.SalesPersonId = updatedOrder.SalesPersonId
	existingOrder.Notes = updatedOrder.Notes
	existingOrder.TermsAndConditions = updatedOrder.TermsAndConditions
	existingOrder.CurrencyId = updatedOrder.CurrencyId
	existingOrder.ExchangeRate = updatedOrder.ExchangeRate
	existingOrder.OrderDiscount = updatedOrder.OrderDiscount
	existingOrder.OrderDiscountType = updatedOrder.OrderDiscountType
	existingOrder.ShippingCharges = updatedOrder.ShippingCharges
	existingOrder.AdjustmentAmount = updatedOrder.AdjustmentAmount
	existingOrder.IsTaxInclusive = updatedOrder.IsTaxInclusive
	existingOrder.OrderTaxId = updatedOrder.OrderTaxId
	existingOrder.OrderTaxType = updatedOrder.OrderTaxType
	existingOrder.CurrentStatus = updatedOrder.CurrentStatus
	// validate
	existingOrder.WarehouseId = updatedOrder.WarehouseId

	var orderSubtotal,
		orderTotalAmount,
		totalExclusiveTaxAmount,
		totalDetailDiscountAmount,
		totalDetailTaxAmount decimal.Decimal

	// Build a stable lookup for existing details (range over slice copies will break pointers).
	existingByID := make(map[int]*SalesOrderDetail, len(existingOrder.Details))
	for i := range existingOrder.Details {
		d := &existingOrder.Details[i]
		existingByID[d.ID] = d
	}

	// Iterate through the updated items and apply changes in a single transaction.
	for _, updatedItem := range updatedOrder.Details {
		// Delete
		if updatedItem.IsDeletedItem != nil && *updatedItem.IsDeletedItem {
			if updatedItem.DetailId <= 0 {
				continue
			}
			if item, ok := existingByID[updatedItem.DetailId]; ok {
				// reduce order qty from stock summary if SO is confirmed
				if item.ProductId > 0 && existingOrder.CurrentStatus == SalesOrderStatusConfirmed {
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
			}
			if err := tx.WithContext(ctx).
				Where("id = ? AND sales_order_id = ?", updatedItem.DetailId, saleOrderId).
				Delete(&SalesOrderDetail{}).Error; err != nil {
				tx.Rollback()
				return nil, err
			}
			continue
		}

		// Create
		if updatedItem.DetailId <= 0 {
			newItem := SalesOrderDetail{
				SalesOrderId:       saleOrderId, // CRITICAL: ensure parent exists before inserting child
				ProductId:          updatedItem.ProductId,
				ProductType:        updatedItem.ProductType,
				BatchNumber:        updatedItem.BatchNumber,
				Name:               updatedItem.Name,
				Description:        updatedItem.Description,
				DetailQty:          updatedItem.DetailQty,
				DetailUnitRate:     updatedItem.DetailUnitRate,
				DetailTaxId:        updatedItem.DetailTaxId,
				DetailTaxType:      updatedItem.DetailTaxType,
				DetailDiscount:     updatedItem.DetailDiscount,
				DetailDiscountType: updatedItem.DetailDiscountType,
				DetailAccountId:    updatedItem.DetailAccountId,
			}
			newItem.CalculateSaleItemDiscountAndTax(ctx, *updatedOrder.IsTaxInclusive)
			orderSubtotal, totalDetailDiscountAmount, totalDetailTaxAmount, totalExclusiveTaxAmount =
				updateSaleItemDetailTotal(&newItem, *updatedOrder.IsTaxInclusive, orderSubtotal, totalExclusiveTaxAmount, totalDetailDiscountAmount, totalDetailTaxAmount)

			if err := tx.WithContext(ctx).Create(&newItem).Error; err != nil {
				tx.Rollback()
				return nil, err
			}
			continue
		}

		// Update
		item, ok := existingByID[updatedItem.DetailId]
		if !ok {
			// If client sent a DetailId that isn't in the order, fail fast (prevents FK chaos).
			tx.Rollback()
			return nil, errors.New("sales order detail not found")
		}

		item.ProductId = updatedItem.ProductId
		item.ProductType = updatedItem.ProductType
		item.BatchNumber = updatedItem.BatchNumber
		item.Name = updatedItem.Name
		item.Description = updatedItem.Description
		item.DetailQty = updatedItem.DetailQty
		item.DetailUnitRate = updatedItem.DetailUnitRate
		item.DetailTaxId = updatedItem.DetailTaxId
		item.DetailTaxType = updatedItem.DetailTaxType
		item.DetailDiscount = updatedItem.DetailDiscount
		item.DetailDiscountType = updatedItem.DetailDiscountType
		item.DetailAccountId = updatedItem.DetailAccountId

		item.CalculateSaleItemDiscountAndTax(ctx, *updatedOrder.IsTaxInclusive)
		orderSubtotal, totalDetailDiscountAmount, totalDetailTaxAmount, totalExclusiveTaxAmount =
			updateSaleItemDetailTotal(item, *updatedOrder.IsTaxInclusive, orderSubtotal, totalExclusiveTaxAmount, totalDetailDiscountAmount, totalDetailTaxAmount)

		// CRITICAL: Save the actual struct pointer (not **SalesOrderDetail).
		if err := tx.WithContext(ctx).Save(item).Error; err != nil {
			tx.Rollback()
			return nil, err
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
	orderTotalAmount = orderSubtotal.Add(orderTaxAmount).Add(totalExclusiveTaxAmount).Add(updatedOrder.AdjustmentAmount).Add(updatedOrder.ShippingCharges).Sub(orderDiscountAmount)

	existingOrder.OrderTotalAmount = orderTotalAmount

	// Save the updated purchase order to the database
	// CRITICAL: existingOrder is already a pointer; do not pass **SalesOrder to GORM.
	if err := tx.WithContext(ctx).Save(existingOrder).Error; err != nil {
		tx.Rollback()
		return nil, err
	}

	// Refresh the existingOrder to get the latest details
	if err := tx.WithContext(ctx).Preload("Details").First(&existingOrder, saleOrderId).Error; err != nil {
		tx.Rollback()
		return nil, err
	}

	if err := existingOrder.AfterUpdateCurrentStatus(tx.WithContext(ctx), string(oldStatus)); err != nil {
		tx.Rollback()
		return nil, err
	}

	documents, err := upsertDocuments(ctx, tx, updatedOrder.Documents, "sales_orders", saleOrderId)
	if err != nil {
		tx.Rollback()
		return nil, err
	}
	existingOrder.Documents = documents

	// Commit the transaction
	if err := tx.Commit().Error; err != nil {
		return nil, err
	}

	return existingOrder, nil
}

func DeleteSalesOrder(ctx context.Context, id int) (*SalesOrder, error) {
	businessId, ok := utils.GetBusinessIdFromContext(ctx)
	if !ok || businessId == "" {
		return nil, errors.New("business id is required")
	}

	result, err := utils.FetchModelForChange[SalesOrder](ctx, businessId, id, "Documents", "Details")
	if err != nil {
		return nil, utils.ErrorRecordNotFound
	}
	db := config.GetDB()
	tx := db.Begin()
	// reduced received qty from stock summary if sale order is confirmed
	if result.CurrentStatus == SalesOrderStatusConfirmed {
		for _, item := range result.Details {
			if item.ProductId > 0 {
				product, err := GetProductOrVariant(ctx, string(item.ProductType), item.ProductId)
				if err != nil {
					tx.Rollback()
					return nil, err
				}
				if product.GetInventoryAccountID() > 0 {
					if err := UpdateStockSummaryCommittedQty(tx, result.BusinessId, result.WarehouseId, item.ProductId, string(item.ProductType), item.BatchNumber, item.DetailQty.Neg(), result.OrderDate); err != nil {
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
	// Commit the transaction
	if err := tx.Commit().Error; err != nil {
		return nil, err
	}

	return result, nil
}

func UpdateStatusSalesOrder(ctx context.Context, id int, status string) (*SalesOrder, error) {
	businessId, ok := utils.GetBusinessIdFromContext(ctx)
	if !ok || businessId == "" {
		return nil, errors.New("business id is required")
	}

	so, err := utils.FetchModelForChange[SalesOrder](ctx, businessId, id, "Details")
	if err != nil {
		return nil, err
	}

	if so.CurrentStatus == SalesOrderStatusClosed {
		return nil, errors.New("cannot update sale order that is already closed")
	}

	if so.CurrentStatus == SalesOrderStatusPartiallyInvoiced && status == string(SalesOrderStatusCancelled) {
		return nil, errors.New("sale orders that are converted to invoice cannot be cancelled.")
	}

	oldStatus := so.CurrentStatus

	// db action
	db := config.GetDB()
	tx := db.Begin()
	err = tx.WithContext(ctx).Model(&so).Updates(map[string]interface{}{
		"CurrentStatus": status,
	}).Error

	if err != nil {
		tx.Rollback()
		return nil, err
	}

	// Apply committed stock side-effects deterministically (prefer explicit command handler).
	if config.UseStockCommandsFor("SALES_ORDER") {
		so.CurrentStatus = SalesOrderStatus(status)
		if err := ApplySalesOrderStockForStatusTransition(tx.WithContext(ctx), so, oldStatus); err != nil {
			tx.Rollback()
			return nil, err
		}
	} else {
		if err := so.AfterUpdateCurrentStatus(tx.WithContext(ctx), string(oldStatus)); err != nil {
			tx.Rollback()
			return nil, err
		}
	}

	// Commit the transaction
	if err := tx.Commit().Error; err != nil {
		return nil, err
	}

	return so, nil
}

func GetSalesOrder(ctx context.Context, id int) (*SalesOrder, error) {
	db := config.GetDB()

	var result SalesOrder
	err := db.WithContext(ctx).First(&result, id).Error
	if err != nil {
		return nil, err
	}

	return &result, nil
}

func GetSalesOrders(ctx context.Context, customerId *int, notes *string) ([]*SalesOrder, error) {
	db := config.GetDB()
	var results []*SalesOrder

	businessId, ok := utils.GetBusinessIdFromContext(ctx)
	if !ok || businessId == "" {
		return nil, errors.New("business id is required")
	}

	dbCtx := db.WithContext(ctx).Where("business_id = ?", businessId)

	if notes != nil && len(*notes) > 0 {
		dbCtx = dbCtx.Where("notes LIKE ?", "%"+*notes+"%")
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

func PaginateSalesOrder(ctx context.Context, limit *int, after *string,
	orderNumber *string,
	referenceNumber *string,
	branchID *int,
	warehouseID *int,
	customerID *int,
	status *SalesOrderStatus,
	startOrderDate *MyDateString,
	endOrderDate *MyDateString,
	startExpectedShipmentDate *MyDateString,
	endExpectedShipmentDate *MyDateString) (*SalesOrdersConnection, error) {

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
	if err := startExpectedShipmentDate.StartOfDayUTCTime(business.Timezone); err != nil {
		return nil, err
	}
	if err := endExpectedShipmentDate.EndOfDayUTCTime(business.Timezone); err != nil {
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
	if customerID != nil && *customerID > 0 {
		dbCtx.Where("customer_id = ?", *customerID)
	}
	if warehouseID != nil && *warehouseID > 0 {
		dbCtx.Where("warehouse_id = ?", *warehouseID)
	}
	if status != nil {
		dbCtx.Where("current_status = ?", *status)
	}
	if startOrderDate != nil && endOrderDate != nil {
		dbCtx.Where("order_date BETWEEN ? AND ?", startOrderDate, endOrderDate)
	}
	if startExpectedShipmentDate != nil && endExpectedShipmentDate != nil {
		dbCtx.Where("expected_shipment_date BETWEEN ? AND ?", startExpectedShipmentDate, endExpectedShipmentDate)
	}

	edges, pageInfo, err := FetchPageCompositeCursor[SalesOrder](dbCtx, *limit, after, "created_at", "<")
	if err != nil {
		return nil, err
	}
	var salesOrdersConnection SalesOrdersConnection
	salesOrdersConnection.PageInfo = pageInfo
	for _, edge := range edges {
		salesOrdersEdge := SalesOrdersEdge(edge)
		salesOrdersConnection.Edges = append(salesOrdersConnection.Edges, &salesOrdersEdge)
	}

	return &salesOrdersConnection, err
}

func UpdateSaleOrderDetailInvoicedQty(tx *gorm.DB, ctx context.Context, saleOrderId int, invoiceItem SalesInvoiceDetail, action string, oldQty decimal.Decimal, inventoryAccId int) error {
	var saleOrderDetail SalesOrderDetail
	var err error
	if invoiceItem.SalesOrderItemId > 0 {
		err = tx.WithContext(ctx).Where("id = ?", invoiceItem.SalesOrderItemId).First(&saleOrderDetail).Error
	} else {
		err = tx.Where("sales_order_id = ? AND product_id = ? AND product_type = ? AND batch_number = ?",
			saleOrderId, invoiceItem.ProductId, invoiceItem.ProductType, invoiceItem.BatchNumber).First(&saleOrderDetail).Error

	}
	if err == gorm.ErrRecordNotFound {
		// Skip update if record does not exist
		return nil
	} else if err != nil {
		tx.Rollback()
		return err
	}

	var so SalesOrder
	if saleOrderId > 0 {
		// exists sale order
		err := tx.Where("id = ?", saleOrderId).First(&so).Error
		if err != nil {
			return err
		}
	}

	if action == "create" {
		if invoiceItem.DetailQty.GreaterThan(saleOrderDetail.DetailQty.Sub(saleOrderDetail.DetailInvoicedQty)) {
			tx.Rollback()
			return errors.New("bill qty must be equal or less than sale order qty")
		}
		saleOrderDetail.DetailInvoicedQty = saleOrderDetail.DetailInvoicedQty.Add(invoiceItem.DetailQty)

		if inventoryAccId > 0 {
			if err := UpdateStockSummaryCommittedQty(tx, so.BusinessId, so.WarehouseId, invoiceItem.ProductId, string(invoiceItem.ProductType), invoiceItem.BatchNumber, invoiceItem.DetailQty.Neg(), so.OrderDate); err != nil {
				tx.Rollback()
				return err
			}
		}

	} else if action == "update" {

		if invoiceItem.DetailQty.GreaterThan(saleOrderDetail.DetailQty.Sub(saleOrderDetail.DetailInvoicedQty.Sub(oldQty))) {
			tx.Rollback()
			return errors.New("bill qty must be equal or less than sale order qty")
		}
		saleOrderDetail.DetailInvoicedQty = saleOrderDetail.DetailInvoicedQty.Add(invoiceItem.DetailQty.Sub(oldQty))

		if err := UpdateStockSummaryCommittedQty(tx, so.BusinessId, so.WarehouseId, invoiceItem.ProductId, string(invoiceItem.ProductType), invoiceItem.BatchNumber, oldQty.Sub(invoiceItem.DetailQty), so.OrderDate); err != nil {
			tx.Rollback()
			return err
		}

	} else if action == "delete" {
		saleOrderDetail.DetailInvoicedQty = saleOrderDetail.DetailInvoicedQty.Sub(invoiceItem.DetailQty)

		if err := UpdateStockSummaryCommittedQty(tx, so.BusinessId, so.WarehouseId, invoiceItem.ProductId, string(invoiceItem.ProductType), invoiceItem.BatchNumber, invoiceItem.DetailQty, so.OrderDate); err != nil {
			tx.Rollback()
			return err
		}
	}

	if err := tx.WithContext(ctx).Save(&saleOrderDetail).Error; err != nil {
		tx.Rollback()
		return err
	}
	return nil
}

func ChangeSaleOrderCurrentStatus(tx *gorm.DB, ctx context.Context, businessId string, soId int) (*SalesOrder, error) {

	var saleOrder SalesOrder
	err := tx.Preload("Details").Where("business_id = ? AND id = ?", businessId, soId).First(&saleOrder).Error
	if err != nil {
		return nil, err
	}

	isPartialQtyInvoiced := false
	isNonInvoice := false

	for _, orderItem := range saleOrder.Details {
		if orderItem.DetailQty.Sub(orderItem.DetailInvoicedQty).GreaterThan(decimal.NewFromFloat(0)) && !orderItem.DetailInvoicedQty.Equal(decimal.NewFromFloat(0)) {
			isPartialQtyInvoiced = true
			break
		}

		if orderItem.DetailInvoicedQty.Equal(decimal.NewFromFloat(0)) {
			isNonInvoice = true
		}

	}
	var status string

	if isPartialQtyInvoiced {
		status = string(SalesOrderStatusPartiallyInvoiced)
	} else if isNonInvoice {
		status = string(SalesOrderStatusConfirmed)
	} else {
		status = string(SalesOrderStatusClosed)
	}

	err = tx.Model(&saleOrder).Updates(map[string]interface{}{
		"CurrentStatus": status,
	}).Error
	if err != nil {
		tx.Rollback()
		return nil, err
	}

	return &saleOrder, nil
}
