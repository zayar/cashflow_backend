package models

import (
	"context"
	"errors"
	"fmt"
	"time"

	"bitbucket.org/mmdatafocus/books_backend/config"
	"bitbucket.org/mmdatafocus/books_backend/utils"
	"github.com/shopspring/decimal"
)

type RecurringBill struct {
	ID                         int                   `gorm:"primary_key" json:"id"`
	BusinessId                 string                `gorm:"index;not null" json:"business_id" binding:"required"`
	SupplierId                 int                   `gorm:"index;not null" json:"supplier_id" binding:"required"`
	BranchId                   int                   `gorm:"index;not null" json:"branch_id" binding:"required"`
	ProfileName                string                `gorm:"size:100;not null" json:"profile_name" binding:"required"`
	RepeatTimes                int                   `gorm:"not null;default:1" json:"repeat_times" binding:"required"`
	RepeatTerms                RecurringTerms        `gorm:"type:enum('D', 'W', 'M', 'Y')" json:"repeat_terms" binding:"required"`
	StartDate                  time.Time             `gorm:"not null" json:"start_date" binding:"required"`
	EndDate                    *time.Time            `gorm:"default:null" json:"end_date"`
	IsNeverExpired             *bool                 `gorm:"default:false" json:"is_never_expired"`
	BillPaymentTerms           PaymentTerms          `gorm:"type:enum('Net15', 'Net30', 'Net45', 'Net60', 'DueMonthEnd', 'DueNextMonthEnd', 'DueOnReceipt', 'Custom');not null" json:"bill_payment_terms" binding:"required"`
	BillPaymentTermsCustomDays int                   `gorm:"default:0" json:"bill_payment_terms_custom_days"`
	Notes                      string                `gorm:"type:text;default:null" json:"notes"`
	CurrencyId                 int                   `gorm:"not null" json:"currency_id" binding:"required"`
	BillDiscount               decimal.Decimal       `gorm:"type:decimal(20,4);default:0" json:"bill_discount"`
	BillDiscountType           *DiscountType         `gorm:"type:enum('P', 'A');default:null" json:"bill_discount_type"`
	BillDiscountAmount         decimal.Decimal       `gorm:"type:decimal(20,4);default:0" json:"bill_discount_amount"`
	AdjustmentAmount           decimal.Decimal       `gorm:"type:decimal(20,4);default:0" json:"adjustment_amount"`
	IsTaxInclusive             *bool                 `gorm:"not null;default:false" json:"is_tax_inclusive"`
	BillTaxId                  int                   `gorm:"default:null" json:"bill_tax_id"`
	BillTaxType                *TaxType              `gorm:"type:enum('I', 'G');default:null" json:"bill_tax_type"`
	BillTaxAmount              decimal.Decimal       `gorm:"type:decimal(20,4);default:0" json:"bill_tax_amount"`
	BillSubtotal               decimal.Decimal       `gorm:"type:decimal(20,4);default:0" json:"bill_subtotal"`
	BillTotalDiscountAmount    decimal.Decimal       `gorm:"type:decimal(20,4);default:0" json:"bill_total_discount_amount"`
	BillTotalTaxAmount         decimal.Decimal       `gorm:"type:decimal(20,4);default:0" json:"bill_total_tax_amount"`
	BillTotalAmount            decimal.Decimal       `gorm:"type:decimal(20,4);default:0" json:"bill_total_amount"`
	Details                    []RecurringBillDetail `json:"recurring_bill_details" validate:"required,dive,required"`
	CreatedAt                  time.Time             `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt                  time.Time             `gorm:"autoUpdateTime" json:"updated_at"`
}

type NewRecurringBill struct {
	SupplierId                 int                      `json:"supplier_id" binding:"required"`
	BranchId                   int                      `json:"branch_id" binding:"required"`
	ProfileName                string                   `json:"profile_name" binding:"required"`
	RepeatTimes                int                      `json:"repeat_times" binding:"required"`
	RepeatTerms                RecurringTerms           `json:"repeat_terms" binding:"required"`
	StartDate                  time.Time                `json:"start_date" binding:"required"`
	EndDate                    *time.Time               `json:"end_date"`
	IsNeverExpired             *bool                    `json:"is_never_expired"`
	BillPaymentTerms           PaymentTerms             `json:"bill_payment_terms" binding:"required"`
	BillPaymentTermsCustomDays int                      `json:"bill_payment_terms_custom_days"`
	Notes                      string                   `json:"notes"`
	CurrencyId                 int                      `json:"currency_id" binding:"required"`
	BillDiscount               decimal.Decimal          `json:"bill_discount"`
	BillDiscountType           *DiscountType            `json:"bill_discount_type"`
	AdjustmentAmount           decimal.Decimal          `json:"adjustment_amount"`
	IsTaxInclusive             *bool                    `json:"is_tax_inclusive"`
	BillTaxId                  int                      `json:"bill_tax_id"`
	BillTaxType                *TaxType                 `json:"bill_tax_type"`
	Details                    []NewRecurringBillDetail `json:"details"`
}

type RecurringBillDetail struct {
	ID                   int             `gorm:"primary_key" json:"id"`
	RecurringBillId      int             `gorm:"index;not null" json:"recurring_bill_id" binding:"required"`
	ProductId            int             `json:"product_id"`
	ProductType          ProductType     `gorm:"type:enum('S','G','C','V','I');default:S" json:"product_type"`
	Name                 string          `gorm:"size:100" json:"name" binding:"required"`
	Description          string          `gorm:"size:255" json:"description"`
	DetailAccountId      int             `json:"detail_account_id"`
	DetailQty            decimal.Decimal `gorm:"type:decimal(20,4);default:0" json:"detail_qty" binding:"required"`
	DetailUnitRate       decimal.Decimal `gorm:"type:decimal(20,4);default:0" json:"detail_unit_rate" binding:"required"`
	DetailTaxId          int             `json:"detail_tax_id"`
	DetailTaxType        *TaxType        `gorm:"type:enum('I', 'G');default:null" json:"detail_tax_type"`
	DetailDiscount       decimal.Decimal `json:"detail_discount"`
	DetailDiscountType   *DiscountType   `gorm:"type:enum('P', 'A');default:null" json:"detail_discount_type"`
	DetailCustomerId     int             `json:"detail_customer_id"`
	DetailDiscountAmount decimal.Decimal `gorm:"type:decimal(20,4);default:0" json:"detail_discount_amount"`
	DetailTaxAmount      decimal.Decimal `gorm:"type:decimal(20,4);default:0" json:"detail_tax_amount"`
	DetailTotalAmount    decimal.Decimal `gorm:"type:decimal(20,4);default:0" json:"detail_total_amount"`
}

type NewRecurringBillDetail struct {
	DetailId           int             `json:"detail_id"`
	ProductId          int             `json:"product_id"`
	ProductType        ProductType     `json:"product_type"`
	Name               string          `json:"name" binding:"required"`
	Description        string          `json:"description"`
	DetailAccountId    int             `json:"detail_account_id"`
	DetailQty          decimal.Decimal `json:"detail_qty" binding:"required"`
	DetailUnitRate     decimal.Decimal `json:"detail_unit_rate" binding:"required"`
	DetailTaxId        int             `json:"detail_tax_id"`
	DetailTaxType      *TaxType        `json:"detail_tax_type"`
	DetailDiscount     decimal.Decimal `json:"detail_discount"`
	DetailDiscountType *DiscountType   `json:"detail_discount_type"`
	DetailCustomerId   int             `json:"detail_customer_id"`
	IsDeletedItem      *bool           `json:"is_deleted_item"`
}

type RecurringBillsConnection struct {
	Edges    []*RecurringBillsEdge `json:"edges"`
	PageInfo *PageInfo             `json:"pageInfo"`
}

type RecurringBillsEdge Edge[RecurringBill]

func (obj RecurringBill) GetId() int {
	return obj.ID
}

// implements methods for pagination

// node
// returns decoded curosr string
func (rb RecurringBill) GetCursor() string {
	return rb.CreatedAt.String()
}

func (item *RecurringBillDetail) CalculateRecurringItemDiscountAndTax(ctx context.Context, isTaxInclusive bool) {

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

func updateRecurringBillDetailTotal(item *RecurringBillDetail, isTaxInclusive bool, orderSubtotal decimal.Decimal, totalExclusiveTaxAmount decimal.Decimal, totalDetailDiscountAmount decimal.Decimal, totalDetailTaxAmount decimal.Decimal) (decimal.Decimal, decimal.Decimal, decimal.Decimal, decimal.Decimal) {

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

func CreateRecurringBill(ctx context.Context, input *NewRecurringBill) (*RecurringBill, error) {
	db := config.GetDB()

	businessId, ok := utils.GetBusinessIdFromContext(ctx)
	if !ok || businessId == "" {
		return nil, errors.New("business id is required")
	}

	// validate PurchaseOrder
	var count int64
	// exists supplier
	if err := db.WithContext(ctx).Model(&Supplier{}).Where("id = ?", input.SupplierId).Count(&count).Error; err != nil {
		return nil, err
	}
	if count <= 0 {
		return nil, utils.ErrorRecordNotFound
	}

	// exists branch
	if err := db.WithContext(ctx).Model(&Branch{}).Where("id = ?", input.BranchId).Count(&count).Error; err != nil {
		return nil, err
	}
	if count <= 0 {
		return nil, utils.ErrorRecordNotFound
	}

	var billItems []RecurringBillDetail
	var billSubtotal,
		billTotalAmount,
		totalExclusiveTaxAmount,
		totalDetailDiscountAmount,
		totalDetailTaxAmount decimal.Decimal

	for _, item := range input.Details {
		billItem := RecurringBillDetail{
			ProductId:          item.ProductId,
			ProductType:        item.ProductType,
			Name:               item.Name,
			Description:        item.Description,
			DetailAccountId:    item.DetailAccountId,
			DetailCustomerId:   item.DetailCustomerId,
			DetailQty:          item.DetailQty,
			DetailUnitRate:     item.DetailUnitRate,
			DetailTaxId:        item.DetailTaxId,
			DetailTaxType:      item.DetailTaxType,
			DetailDiscount:     item.DetailDiscount,
			DetailDiscountType: item.DetailDiscountType,
		}

		// Calculate tax and total amounts for the item
		billItem.CalculateRecurringItemDiscountAndTax(ctx, *input.IsTaxInclusive)

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

	}

	// calculate bill discount
	var billDiscountAmount decimal.Decimal

	if input.BillDiscountType != nil {
		billDiscountAmount = utils.CalculateDiscountAmount(billSubtotal, input.BillDiscount, string(*input.BillDiscountType))
	}

	// billSubtotal = billSubtotal.Sub(billDiscountAmount)

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

	// store RecurringBill
	bill := RecurringBill{
		BusinessId:                 businessId,
		SupplierId:                 input.SupplierId,
		BranchId:                   input.BranchId,
		ProfileName:                input.ProfileName,
		RepeatTimes:                input.RepeatTimes,
		RepeatTerms:                input.RepeatTerms,
		StartDate:                  input.StartDate,
		EndDate:                    input.EndDate,
		IsNeverExpired:             input.IsNeverExpired,
		BillPaymentTerms:           input.BillPaymentTerms,
		BillPaymentTermsCustomDays: input.BillPaymentTermsCustomDays,
		Notes:                      input.Notes,
		CurrencyId:                 input.CurrencyId,
		BillDiscount:               input.BillDiscount,
		BillDiscountType:           input.BillDiscountType,
		BillDiscountAmount:         billDiscountAmount,
		AdjustmentAmount:           input.AdjustmentAmount,
		IsTaxInclusive:             input.IsTaxInclusive,
		BillTaxId:                  input.BillTaxId,
		BillTaxType:                input.BillTaxType,
		BillTaxAmount:              billTaxAmount,
		Details:                    billItems,
		BillTotalDiscountAmount:    totalBillDiscountAmount,
		BillTotalTaxAmount:         totalBillTaxAmount,
		BillSubtotal:               billSubtotal,
		BillTotalAmount:            billTotalAmount,
	}

	err := db.WithContext(ctx).Create(&bill).Error
	if err != nil {
		return nil, err
	}

	return &bill, nil
}

func UpdateRecurringBill(ctx context.Context, id int, updatedRecurringBill *NewRecurringBill) (*RecurringBill, error) {
	db := config.GetDB()

	// Fetch the existing purchase order
	var existingRecurringBill RecurringBill
	if err := db.WithContext(ctx).Preload("Details").First(&existingRecurringBill, id).Error; err != nil {
		return nil, err
	}

	// Update the fields of the existing purchase order with the provided updated details
	existingRecurringBill.SupplierId = updatedRecurringBill.SupplierId
	existingRecurringBill.BranchId = updatedRecurringBill.BranchId
	existingRecurringBill.ProfileName = updatedRecurringBill.ProfileName
	existingRecurringBill.RepeatTimes = updatedRecurringBill.RepeatTimes
	existingRecurringBill.RepeatTerms = updatedRecurringBill.RepeatTerms
	existingRecurringBill.StartDate = updatedRecurringBill.StartDate
	existingRecurringBill.EndDate = updatedRecurringBill.EndDate
	existingRecurringBill.IsNeverExpired = updatedRecurringBill.IsNeverExpired

	existingRecurringBill.BillPaymentTerms = updatedRecurringBill.BillPaymentTerms
	existingRecurringBill.BillPaymentTermsCustomDays = updatedRecurringBill.BillPaymentTermsCustomDays
	existingRecurringBill.Notes = updatedRecurringBill.Notes
	existingRecurringBill.CurrencyId = updatedRecurringBill.CurrencyId
	existingRecurringBill.BillDiscount = updatedRecurringBill.BillDiscount
	existingRecurringBill.BillDiscountType = updatedRecurringBill.BillDiscountType

	existingRecurringBill.AdjustmentAmount = updatedRecurringBill.AdjustmentAmount
	existingRecurringBill.IsTaxInclusive = updatedRecurringBill.IsTaxInclusive
	existingRecurringBill.BillTaxId = updatedRecurringBill.BillTaxId
	existingRecurringBill.BillTaxType = updatedRecurringBill.BillTaxType

	var orderSubtotal,
		orderTotalAmount,
		totalExclusiveTaxAmount,
		totalDetailDiscountAmount,
		totalDetailTaxAmount decimal.Decimal

	// Iterate through the updated items

	for _, updatedItem := range updatedRecurringBill.Details {
		var existingItem *RecurringBillDetail

		// Check if the item already exists in the purchase order
		for _, item := range existingRecurringBill.Details {
			if item.ID == updatedItem.DetailId {
				existingItem = &item
				break
			}
		}

		// If the item doesn't exist, add it to the purchase order
		if existingItem == nil {
			fmt.Println("is not existing- ")
			newItem := RecurringBillDetail{
				ProductId:          updatedItem.ProductId,
				ProductType:        updatedItem.ProductType,
				Name:               updatedItem.Name,
				Description:        updatedItem.Description,
				DetailAccountId:    updatedItem.DetailAccountId,
				DetailCustomerId:   updatedItem.DetailCustomerId,
				DetailQty:          updatedItem.DetailQty,
				DetailUnitRate:     updatedItem.DetailUnitRate,
				DetailTaxId:        updatedItem.DetailTaxId,
				DetailTaxType:      updatedItem.DetailTaxType,
				DetailDiscount:     updatedItem.DetailDiscount,
				DetailDiscountType: updatedItem.DetailDiscountType,
			}

			// Calculate tax and total amounts for the item
			newItem.CalculateRecurringItemDiscountAndTax(ctx, *updatedRecurringBill.IsTaxInclusive)
			orderSubtotal, totalDetailDiscountAmount, totalDetailTaxAmount, totalExclusiveTaxAmount = updateRecurringBillDetailTotal(&newItem, *updatedRecurringBill.IsTaxInclusive, orderSubtotal, totalExclusiveTaxAmount, totalDetailDiscountAmount, totalDetailTaxAmount)
			existingRecurringBill.Details = append(existingRecurringBill.Details, newItem)

		} else {
			if updatedItem.IsDeletedItem != nil && *updatedItem.IsDeletedItem {
				// fmt.Println("Is deleted Item")
				// if err := db.WithContext(ctx).Delete(&existingItem).Error; err != nil {
				// 	return nil, err
				// }
				// Find the index of the item to delete
				for i, item := range existingRecurringBill.Details {
					if item.ID == updatedItem.DetailId {
						// Delete the item from the database
						if err := db.WithContext(ctx).Delete(&existingRecurringBill.Details[i]).Error; err != nil {
							return nil, err
						}
						// Remove the item from the slice
						existingRecurringBill.Details = append(existingRecurringBill.Details[:i], existingRecurringBill.Details[i+1:]...)
						break // Exit the loop after deleting the item
					}
				}
			} else {
				// Update existing item details
				existingItem.ProductId = updatedItem.ProductId
				existingItem.ProductType = updatedItem.ProductType
				existingItem.Name = updatedItem.Name
				existingItem.Description = updatedItem.Description
				existingItem.DetailAccountId = updatedItem.DetailAccountId
				existingItem.DetailCustomerId = updatedItem.DetailCustomerId
				existingItem.DetailQty = updatedItem.DetailQty
				existingItem.DetailUnitRate = updatedItem.DetailUnitRate
				existingItem.DetailTaxId = updatedItem.DetailTaxId
				existingItem.DetailTaxType = updatedItem.DetailTaxType
				existingItem.DetailDiscount = updatedItem.DetailDiscount
				existingItem.DetailDiscountType = updatedItem.DetailDiscountType

				// Calculate tax and total amounts for the item
				existingItem.CalculateRecurringItemDiscountAndTax(ctx, *updatedRecurringBill.IsTaxInclusive)
				orderSubtotal, totalDetailDiscountAmount, totalDetailTaxAmount, totalExclusiveTaxAmount = updateRecurringBillDetailTotal(existingItem, *updatedRecurringBill.IsTaxInclusive, orderSubtotal, totalExclusiveTaxAmount, totalDetailDiscountAmount, totalDetailTaxAmount)
				// existingRecurringBill.Details = append(existingRecurringBill.Details, *existingItem)

				if err := db.WithContext(ctx).Save(&existingItem).Error; err != nil {
					return nil, err
				}
			}
		}
	}

	// calculate order discount
	var orderDiscountAmount decimal.Decimal

	if updatedRecurringBill.BillDiscountType != nil {
		orderDiscountAmount = utils.CalculateDiscountAmount(orderSubtotal, updatedRecurringBill.BillDiscount, string(*updatedRecurringBill.BillDiscountType))
	}

	existingRecurringBill.BillDiscountAmount = orderDiscountAmount

	// orderSubtotal = orderSubtotal.Sub(orderDiscountAmount)
	existingRecurringBill.BillSubtotal = orderSubtotal

	// calculate order tax amount (always exclusive)
	var orderTaxAmount decimal.Decimal
	if updatedRecurringBill.BillTaxId > 0 {
		if *updatedRecurringBill.BillTaxType == TaxTypeGroup {
			orderTaxAmount = utils.CalculateTaxAmount(ctx, db, updatedRecurringBill.BillTaxId, true, orderSubtotal, false)
		} else {
			orderTaxAmount = utils.CalculateTaxAmount(ctx, db, updatedRecurringBill.BillTaxId, false, orderSubtotal, false)
		}
	} else {
		orderTaxAmount = decimal.NewFromFloat(0)
	}

	existingRecurringBill.BillTaxAmount = orderTaxAmount

	// Sum (order discount + total detail discount)
	totalOrderDiscountAmount := orderDiscountAmount.Add(totalDetailDiscountAmount)
	existingRecurringBill.BillTotalDiscountAmount = totalOrderDiscountAmount

	// Sum (order tax amount + total detail tax amount)
	totalOrderTaxAmount := orderTaxAmount.Add(totalDetailTaxAmount)
	existingRecurringBill.BillTotalTaxAmount = totalOrderTaxAmount
	// Sum Grand total amount (subtotal+ exclusive tax + adj amount)
	orderTotalAmount = orderSubtotal.Add(orderTaxAmount).Add(totalExclusiveTaxAmount).Add(updatedRecurringBill.AdjustmentAmount).Sub(orderDiscountAmount)

	existingRecurringBill.BillTotalAmount = orderTotalAmount

	// Save the updated purchase order to the database
	if err := db.WithContext(ctx).Save(&existingRecurringBill).Error; err != nil {
		return nil, err
	}

	return &existingRecurringBill, nil
}

func DeleteRecurringBill(ctx context.Context, id int) (*RecurringBill, error) {

	businessId, ok := utils.GetBusinessIdFromContext(ctx)
	if !ok || businessId == "" {
		return nil, errors.New("business id is required")
	}

	db := config.GetDB()
	result, err := utils.FetchModel[RecurringBill](ctx, businessId, id, "Details")
	if err != nil {
		return nil, err
	}

	// if err := validateTransactionLock(ctx, result.StartDate, businessId, PurchaseTransactionLock); err != nil {
	// 	return nil, err
	// }
	// if err := validateTransactionLock(ctx, *result.EndDate, businessId, PurchaseTransactionLock); err != nil {
	// 	return nil, err
	// }

	err = db.WithContext(ctx).Model(&result).Association("Details").Unscoped().Clear()
	if err != nil {
		return nil, err
	}

	err = db.WithContext(ctx).Delete(&result).Error
	if err != nil {
		return nil, err
	}

	return result, nil
}

func GetRecurringBill(ctx context.Context, id int) (*RecurringBill, error) {
	db := config.GetDB()

	var result RecurringBill
	err := db.WithContext(ctx).First(&result, id).Error
	if err != nil {
		return nil, err
	}

	return &result, nil
}

func GetRecurringBills(ctx context.Context, profileName *string) ([]*RecurringBill, error) {
	db := config.GetDB()
	var results []*RecurringBill

	businessId, ok := utils.GetBusinessIdFromContext(ctx)
	if !ok || businessId == "" {
		return nil, errors.New("business id is required")
	}

	dbCtx := db.WithContext(ctx).Where("business_id = ?", businessId)
	if profileName != nil && len(*profileName) > 0 {
		dbCtx = dbCtx.Where("bill_number LIKE ?", "%"+*profileName+"%")
	}
	err := dbCtx.
		Find(&results).Error
	if err != nil {
		return nil, err
	}
	return results, nil
}

func PaginateRecurringBill(ctx context.Context, limit *int, after *string,
	name *string) (*RecurringBillsConnection, error) {

	businessId, ok := utils.GetBusinessIdFromContext(ctx)
	if !ok || businessId == "" {
		return nil, errors.New("business id is required")
	}

	db := config.GetDB()
	dbCtx := db.WithContext(ctx).Where("business_id = ?", businessId)
	if name != nil && *name != "" {
		dbCtx.Where("name LIKE ?", "%"+*name+"%")
	}

	edges, pageInfo, err := FetchPageCompositeCursor[RecurringBill](dbCtx, *limit, after, "created_at", "<")
	if err != nil {
		return nil, err
	}
	var recurringBillsConnection RecurringBillsConnection
	recurringBillsConnection.PageInfo = pageInfo
	for _, edge := range edges {
		recurringBillsEdge := RecurringBillsEdge(edge)
		recurringBillsConnection.Edges = append(recurringBillsConnection.Edges, &recurringBillsEdge)
	}

	return &recurringBillsConnection, err
}
