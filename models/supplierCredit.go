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

type SupplierCredit struct {
	ID                                int                    `gorm:"primary_key" json:"id"`
	BusinessId                        string                 `gorm:"index;not null" json:"business_id" binding:"required"`
	SupplierId                        int                    `gorm:"index;not null" json:"supplier_id" binding:"required"`
	BranchId                          int                    `gorm:"index;not null" json:"branch_id"`
	SupplierCreditNumber              string                 `gorm:"size:255;not null" json:"supplier_credit_number" binding:"required"`
	SequenceNo                        decimal.Decimal        `gorm:"type:decimal(15);not null" json:"sequence_no"`
	ReferenceNumber                   string                 `gorm:"size:255;default:null" json:"reference_number"`
	SupplierCreditDate                time.Time              `gorm:"not null" json:"supplier_credit_date" binding:"required"`
	SupplierCreditSubject             string                 `gorm:"size:255;default:null" json:"supplier_credit_subject"`
	Notes                             string                 `gorm:"type:text;default:null" json:"notes"`
	CurrencyId                        int                    `gorm:"not null" json:"currency_id" binding:"required"`
	ExchangeRate                      decimal.Decimal        `gorm:"type:decimal(20,4);default:0" json:"exchange_rate"`
	WarehouseId                       int                    `gorm:"not null" json:"warehouse_id" binding:"required"`
	SupplierCreditDiscount            decimal.Decimal        `gorm:"type:decimal(20,4);default:0" json:"supplier_credit_discount"`
	SupplierCreditDiscountType        *DiscountType          `gorm:"type:enum('P', 'A');default:null" json:"supplier_credit_discount_type"`
	SupplierCreditDiscountAmount      decimal.Decimal        `gorm:"type:decimal(20,4);default:0" json:"supplier_credit_discount_amount"`
	AdjustmentAmount                  decimal.Decimal        `gorm:"type:decimal(20,4);default:0" json:"adjustment_amount"`
	IsTaxInclusive                    *bool                  `gorm:"not null;default:false" json:"is_tax_inclusive"`
	SupplierCreditTaxId               int                    `gorm:"default:null" json:"order_tax_id"`
	SupplierCreditTaxType             *TaxType               `gorm:"type:enum('I', 'G');default:null" json:"supplier_credit_tax_type"`
	SupplierCreditTaxAmount           decimal.Decimal        `gorm:"type:decimal(20,4);default:0" json:"supplier_credit_tax_amount"`
	CurrentStatus                     SupplierCreditStatus   `gorm:"type:enum('Draft','Confirmed','Void','Closed');not null" json:"current_status" binding:"required"`
	Documents                         []*Document            `gorm:"polymorphic:Reference" json:"documents"`
	Details                           []SupplierCreditDetail `json:"supplier_credit_details" validate:"required,dive,required"`
	SupplierCreditSubtotal            decimal.Decimal        `gorm:"type:decimal(20,4);default:0" json:"supplier_credit_subtotal"`
	SupplierCreditTotalDiscountAmount decimal.Decimal        `gorm:"type:decimal(20,4);default:0" json:"supplier_credit_total_discount_amount"`
	SupplierCreditTotalTaxAmount      decimal.Decimal        `gorm:"type:decimal(20,4);default:0" json:"supplier_credit_total_tax_amount"`
	SupplierCreditTotalAmount         decimal.Decimal        `gorm:"type:decimal(20,4);default:0" json:"supplier_credit_total_amount"`
	SupplierCreditTotalUsedAmount     decimal.Decimal        `gorm:"type:decimal(20,4);default:0" json:"supplier_credit_total_used_amount"`
	SupplierCreditTotalRefundAmount   decimal.Decimal        `gorm:"type:decimal(20,4);default:0" json:"supplier_credit_total_refund_amount"`
	RemainingBalance                  decimal.Decimal        `gorm:"type:decimal(20,4);default:0" json:"remaining_balance"`
	CreatedAt                         time.Time              `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt                         time.Time              `gorm:"autoUpdateTime" json:"updated_at"`
}

type NewSupplierCredit struct {
	SupplierId  int `json:"supplier_id" binding:"required"`
	BranchId    int `json:"branch_id" binding:"required"`
	WarehouseId int `json:"warehouse_id" binding:"required"`
	// SupplierCreditNumber       string                    `json:"supplier_credit_number" binding:"required"`
	ReferenceNumber            string                    `json:"reference_number"`
	SupplierCreditDate         time.Time                 `json:"supplier_credit_date" binding:"required"`
	SupplierCreditSubject      string                    `json:"supplier_credit_subject"`
	Notes                      string                    `json:"notes"`
	CurrencyId                 int                       `json:"currency_id" binding:"required"`
	ExchangeRate               decimal.Decimal           `json:"exchange_rate"`
	SupplierCreditDiscount     decimal.Decimal           `json:"supplier_credit_discount"`
	SupplierCreditDiscountType *DiscountType             `json:"supplier_credit_discount_type"`
	AdjustmentAmount           decimal.Decimal           `json:"adjustment_amount"`
	IsTaxInclusive             *bool                     `json:"is_tax_inclusive" binding:"required"`
	SupplierCreditTaxId        int                       `json:"supplier_credit_tax_id"`
	SupplierCreditTaxType      *TaxType                  `json:"supplier_credit_tax_type"`
	CurrentStatus              SupplierCreditStatus      `json:"current_status" binding:"required"`
	Documents                  []*NewDocument            `json:"documents"`
	Details                    []NewSupplierCreditDetail `json:"details"`
}

type SupplierCreditDetail struct {
	ID                   int             `gorm:"primary_key" json:"id"`
	SupplierCreditId     int             `gorm:"index;not null" json:"supplier_credit_id" binding:"required"`
	ProductId            int             `gorm:"index" json:"product_id"`
	ProductType          ProductType     `gorm:"type:enum('S','G','C','V','I');default:S" json:"product_type"`
	BatchNumber          string          `gorm:"size:100" json:"batch_number"`
	Name                 string          `gorm:"size:100" json:"name" binding:"required"`
	Description          string          `gorm:"size:255;default:null" json:"description"`
	DetailQty            decimal.Decimal `gorm:"type:decimal(20,4);default:0" json:"detail_qty" binding:"required"`
	DetailUnitRate       decimal.Decimal `gorm:"type:decimal(20,4);default:0" json:"detail_unit_rate" binding:"required"`
	DetailTaxId          int             `gorm:"default:null" json:"detail_tax_id"`
	DetailTaxType        *TaxType        `gorm:"type:enum('I', 'G');default:null" json:"detail_tax_type"`
	DetailAccountId      int             `gorm:"default:null" json:"detail_account_id"`
	DetailDiscount       decimal.Decimal `gorm:"default:0" json:"detail_discount"`
	DetailDiscountType   *DiscountType   `gorm:"type:enum('P', 'A');;default:null" json:"detail_discount_type"`
	DetailDiscountAmount decimal.Decimal `gorm:"type:decimal(20,4);default:0" json:"detail_discount_amount"`
	DetailTaxAmount      decimal.Decimal `gorm:"type:decimal(20,4);default:0" json:"detail_tax_amount"`
	DetailTotalAmount    decimal.Decimal `gorm:"type:decimal(20,4);default:0" json:"detail_total_amount"`
	Cogs                 decimal.Decimal `gorm:"type:decimal(20,4);default:0" json:"cogs"`
	CreatedAt            time.Time       `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt            time.Time       `gorm:"autoUpdateTime" json:"updated_at"`
}

type NewSupplierCreditDetail struct {
	DetailId           int             `json:"detail_id"`
	ProductId          int             `json:"product_id"`
	ProductType        ProductType     `json:"product_type"`
	BatchNumber        string          `json:"batch_number"`
	Name               string          `json:"name" binding:"required"`
	Description        string          `json:"description"`
	DetailQty          decimal.Decimal `json:"detail_qty" binding:"required"`
	DetailUnitRate     decimal.Decimal `json:"detail_unit_rate" binding:"required"`
	DetailTaxId        int             `json:"detail_tax_id"`
	DetailTaxType      *TaxType        `json:"detail_tax_type"`
	DetailDiscount     decimal.Decimal `json:"detail_discount"`
	DetailDiscountType *DiscountType   `json:"detail_discount_type"`
	DetailAccountId    int             `json:"detail_account_id"`
	IsDeletedItem      *bool           `json:"is_deleted_item"`
}

type SupplierCreditsEdge Edge[SupplierCredit]

type SupplierCreditsConnection struct {
	Edges    []*SupplierCreditsEdge `json:"edges"`
	PageInfo *PageInfo              `json:"pageInfo"`
}

type SupplierCreditAdvance struct {
	ID               int                   `gorm:"primary_key" json:"id"`
	BusinessId       string                `gorm:"index;not null" json:"business_id" binding:"required"`
	Date             time.Time             `gorm:"not null" json:"date"`
	BranchId         int                   `gorm:"index" json:"branch_id"`
	SupplierId       int                   `gorm:"index" json:"supplier_id"`
	Amount           decimal.Decimal       `gorm:"type:decimal(20,4);default:0" json:"amount"`
	UsedAmount       decimal.Decimal       `gorm:"type:decimal(20,4);default:0" json:"used_amount"`
	CurrencyId       int                   `gorm:"index;not null" json:"currency_id"`
	ExchangeRate     decimal.Decimal       `gorm:"type:decimal(20,4);default:0" json:"exchange_rate"`
	CurrentStatus    SupplierAdvanceStatus `gorm:"type:enum('Draft','Confirmed','Closed');not null" json:"current_status" binding:"required"`
	RefundAmount     decimal.Decimal       `gorm:"type:decimal(20,4);default:0" json:"credit_note_total_refund_amount"`
	RemainingBalance decimal.Decimal       `gorm:"type:decimal(20,4);default:0" json:"remaining_balance"`
	CreatedAt        time.Time             `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt        time.Time             `gorm:"autoUpdateTime" json:"updated_at"`
}

func (sca SupplierCreditAdvance) CheckTransactionLock(ctx context.Context) error {
	return validateTransactionLock(ctx, sca.Date, sca.BusinessId, PurchaseTransactionLock)
}

func (sc SupplierCredit) CheckTransactionLock(ctx context.Context) error {
	if err := validateTransactionLock(ctx, sc.SupplierCreditDate, sc.BusinessId, PurchaseTransactionLock); err != nil {
		return err
	}
	// check for inventory value adjustment
	for _, detail := range sc.Details {
		if err := ValidateValueAdjustment(ctx, sc.BusinessId, sc.SupplierCreditDate, detail.ProductType, detail.ProductId, &detail.BatchNumber); err != nil {
			return fmt.Errorf(err.Error(), detail.Name)
		}
	}
	return nil
}

// returns decoded curosr string
func (sc SupplierCredit) GetCursor() string {
	return sc.CreatedAt.String()
}

func (sc SupplierCredit) GetId() int {
	return sc.ID
}

func (s *SupplierCredit) GetRemainingBalance() decimal.Decimal {
	return s.RemainingBalance
}

func (s *SupplierCredit) AddRefundAmount(amount decimal.Decimal) error {
	if amount.GreaterThan(s.RemainingBalance) {
		return errors.New("amount must be less than or equal to remaining balance of supplier credit")
	}
	s.SupplierCreditTotalRefundAmount = s.SupplierCreditTotalRefundAmount.Add(amount)
	s.RemainingBalance = s.RemainingBalance.Sub(amount)
	return nil
}

func (s SupplierCredit) isApplied() bool {
	return s.SupplierCreditTotalUsedAmount.GreaterThan(decimal.Zero) || s.SupplierCreditTotalRefundAmount.GreaterThan(decimal.Zero)
}

func (a SupplierCreditAdvance) isUsed() bool {
	//? check refund amount?
	return a.UsedAmount.GreaterThan(decimal.Zero)
}

func (c *SupplierCreditAdvance) useAmount(amount decimal.Decimal) error {

	if c.CurrentStatus != SupplierAdvanceStatusConfirmed {
		return errors.New("supplier advance status must be confirm")
	}

	if c.RemainingBalance.LessThan(amount) {
		return errors.New("advacne remaining balance less than applied amount")
	}
	c.UsedAmount = c.UsedAmount.Add(amount)
	c.RemainingBalance = c.RemainingBalance.Sub(amount)
	if c.RemainingBalance.IsZero() {
		c.CurrentStatus = SupplierAdvanceStatusClosed
	}
	return nil
}

func (c *SupplierCreditAdvance) unUseAmount(amount decimal.Decimal) error {

	if c.CurrentStatus == SupplierAdvanceStatusClosed {
		c.CurrentStatus = SupplierAdvanceStatusConfirmed
	}

	c.UsedAmount = c.UsedAmount.Sub(amount)
	c.RemainingBalance = c.RemainingBalance.Add(amount)
	return nil
}

func (sc *SupplierCredit) useAmount(amount decimal.Decimal) error {
	// use creditNote amount
	if sc.CurrentStatus != SupplierCreditStatusConfirmed {
		return errors.New("supplier credit status must be confirm")
	}

	if sc.RemainingBalance.LessThan(amount) {
		return errors.New("credit remaining balance less than applied amount")
	}

	sc.SupplierCreditTotalUsedAmount = sc.SupplierCreditTotalUsedAmount.Add(amount)
	sc.RemainingBalance = sc.RemainingBalance.Sub(amount)
	if sc.RemainingBalance.IsZero() {
		sc.CurrentStatus = SupplierCreditStatusClosed
	}
	return nil
}

func (sc *SupplierCredit) unUseAmount(amount decimal.Decimal) error {

	if sc.CurrentStatus == SupplierCreditStatusClosed {
		sc.CurrentStatus = SupplierCreditStatusConfirmed
	}

	sc.SupplierCreditTotalUsedAmount = sc.SupplierCreditTotalUsedAmount.Sub(amount)
	sc.RemainingBalance = sc.RemainingBalance.Add(amount)
	return nil
}

func (s *SupplierCredit) UpdateStatus() error {
	if s.RemainingBalance.IsZero() {
		s.CurrentStatus = SupplierCreditStatusClosed
	} else {
		s.CurrentStatus = SupplierCreditStatusConfirmed
	}
	return nil
}

func (sc SupplierCredit) GetDueDate() time.Time {
	return sc.SupplierCreditDate
}

func (s *SupplierCreditAdvance) GetId() int {
	return s.ID
}

func (s *SupplierCreditAdvance) GetRemainingBalance() decimal.Decimal {
	return s.RemainingBalance
}

func (s *SupplierCreditAdvance) AddRefundAmount(amount decimal.Decimal) error {
	if amount.GreaterThan(s.RemainingBalance) {
		return errors.New("amount must be less than or equal to remaining balance of supplier advance")
	}
	s.RefundAmount = s.RefundAmount.Add(amount)
	s.RemainingBalance = s.RemainingBalance.Sub(amount)
	return nil
}

func (s *SupplierCreditAdvance) UpdateStatus() error {
	if s.RemainingBalance.IsZero() {
		s.CurrentStatus = SupplierAdvanceStatusClosed
	}
	return nil
}

func (sc SupplierCreditAdvance) GetDueDate() time.Time {
	return sc.Date
}

func (sc *SupplierCredit) GetFieldValues(tx *gorm.DB) (*utils.DetailFieldValues, error) {
	return utils.FetchDetailFieldValues(tx, &SupplierCreditDetail{}, "supplier_credit_id", sc.ID)
}

func (item *SupplierCreditDetail) CalculateItemDiscountAndTax(ctx context.Context, isTaxInclusive bool) {

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
	// if isTaxInclusive {
	// 	item.Cogs = item.DetailTotalAmount.Add(item.DetailDiscountAmount).Sub(item.DetailTaxAmount)
	// } else {
	// 	item.Cogs = item.DetailTotalAmount.Add(item.DetailDiscountAmount)
	// }
	item.Cogs = decimal.NewFromInt(0)
}

func updateSupplierCreditDetailTotal(item *SupplierCreditDetail, isTaxInclusive bool, orderSubtotal decimal.Decimal, totalExclusiveTaxAmount decimal.Decimal, totalDetailDiscountAmount decimal.Decimal, totalDetailTaxAmount decimal.Decimal) (decimal.Decimal, decimal.Decimal, decimal.Decimal, decimal.Decimal) {

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

func (input NewSupplierCredit) validate(ctx context.Context, businessId string, _ int) error {
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
	if input.SupplierCreditTaxType != nil {
		if err := validateTaxExists(ctx, businessId, input.SupplierCreditTaxId, *input.SupplierCreditTaxType); err != nil {
			return err
		}
	}
	// validate supplierCreditDate
	if err := validateTransactionLock(ctx, input.SupplierCreditDate, businessId, PurchaseTransactionLock); err != nil {
		return err
	}
	for _, detail := range input.Details {
		if err := ValidateValueAdjustment(ctx, businessId, input.SupplierCreditDate, detail.ProductType, detail.ProductId, &detail.BatchNumber); err != nil {
			return err
		}
	}

	return nil
}

func CreateSupplierCredit(ctx context.Context, input *NewSupplierCredit) (*SupplierCredit, error) {
	businessId, ok := utils.GetBusinessIdFromContext(ctx)
	if !ok || businessId == "" {
		return nil, errors.New("business id is required")
	}
	// IMPORTANT (correctness): if callers request "Confirmed" on create, we still create as Draft
	// and then transition Draft -> Confirmed inside the same DB transaction.
	requestedStatus := input.CurrentStatus
	// validate
	if err := input.validate(ctx, businessId, 0); err != nil {
		return nil, err
	}

	// construct Images
	documents, err := mapNewDocuments(input.Documents, "supplier_credits", 0)
	if err != nil {
		return nil, err
	}

	var supplierCreditItems []SupplierCreditDetail
	var supplierCreditSubtotal,
		supplierCreditTotalAmount,
		totalExclusiveTaxAmount,
		totalDetailDiscountAmount,
		totalDetailTaxAmount decimal.Decimal

	db := config.GetDB()
	for _, item := range input.Details {
		supplierCreditItem := SupplierCreditDetail{
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

		if err := ValidateProductStock(db, ctx, businessId, input.WarehouseId, item.BatchNumber, item.ProductType, item.ProductId, item.DetailQty); err != nil {
			return nil, err
		}
		// Calculate tax and total amounts for the item
		supplierCreditItem.CalculateItemDiscountAndTax(ctx, *input.IsTaxInclusive)

		supplierCreditSubtotal = supplierCreditSubtotal.Add(supplierCreditItem.DetailTotalAmount)
		totalDetailDiscountAmount = totalDetailDiscountAmount.Add(supplierCreditItem.DetailDiscountAmount)
		totalDetailTaxAmount = totalDetailTaxAmount.Add(supplierCreditItem.DetailTaxAmount)

		if input.IsTaxInclusive != nil && *input.IsTaxInclusive {
			totalExclusiveTaxAmount = totalExclusiveTaxAmount.Add(decimal.NewFromFloat(0.0))
		} else {
			totalExclusiveTaxAmount = totalExclusiveTaxAmount.Add(supplierCreditItem.DetailTaxAmount)
		}

		supplierCreditItem.Cogs = decimal.NewFromInt(0)
		// if item.ProductId > 0 {
		// 	productDetail, err := GetProductDetail(tx, item.ProductId, item.ProductType)
		// 	if err != nil {
		// 		return nil, errors.New("product not found")
		// 	}
		// 	supplierCreditItem.Cogs = productDetail.PurchasePrice
		// }

		// Add the item to the SupplierCreditItems
		supplierCreditItems = append(supplierCreditItems, supplierCreditItem)

	}

	// calculate credit discount
	var supplierCreditDiscountAmount decimal.Decimal

	if input.SupplierCreditDiscountType != nil {
		supplierCreditDiscountAmount = utils.CalculateDiscountAmount(supplierCreditSubtotal, input.SupplierCreditDiscount, string(*input.SupplierCreditDiscountType))
	}

	// supplierCreditSubtotal = supplierCreditSubtotal.Sub(supplierCreditDiscountAmount)

	tx := db.Begin()

	// calculate order tax amount (always exclusive)
	var supplierCreditTaxAmount decimal.Decimal
	if input.SupplierCreditTaxId > 0 {
		if *input.SupplierCreditTaxType == TaxTypeGroup {
			supplierCreditTaxAmount = utils.CalculateTaxAmount(ctx, db, input.SupplierCreditTaxId, true, supplierCreditSubtotal, false)
		} else {
			supplierCreditTaxAmount = utils.CalculateTaxAmount(ctx, db, input.SupplierCreditTaxId, false, supplierCreditSubtotal, false)
		}
	} else {
		supplierCreditTaxAmount = decimal.NewFromFloat(0)
	}

	// Sum (order discount + total detail discount)
	totalSupplierCreditDiscountAmount := supplierCreditDiscountAmount.Add(totalDetailDiscountAmount)
	// Sum (order tax amount + total detail tax amount)
	totalSupplierCreditTaxAmount := supplierCreditTaxAmount.Add(totalDetailTaxAmount)

	supplierCreditTotalAmount = supplierCreditSubtotal.Add(supplierCreditTaxAmount).Add(totalExclusiveTaxAmount).Add(input.AdjustmentAmount).Sub(supplierCreditDiscountAmount)

	// store Bill
	supplierCredit := SupplierCredit{
		BusinessId: businessId,
		SupplierId: input.SupplierId,
		BranchId:   input.BranchId,
		// SupplierCreditNumber:              input.SupplierCreditNumber,
		ReferenceNumber:                   input.ReferenceNumber,
		SupplierCreditDate:                input.SupplierCreditDate,
		SupplierCreditSubject:             input.SupplierCreditSubject,
		Notes:                             input.Notes,
		CurrencyId:                        input.CurrencyId,
		ExchangeRate:                      input.ExchangeRate,
		WarehouseId:                       input.WarehouseId,
		SupplierCreditDiscount:            input.SupplierCreditDiscount,
		SupplierCreditDiscountType:        input.SupplierCreditDiscountType,
		SupplierCreditDiscountAmount:      supplierCreditDiscountAmount,
		AdjustmentAmount:                  input.AdjustmentAmount,
		IsTaxInclusive:                    input.IsTaxInclusive,
		SupplierCreditTaxId:               input.SupplierCreditTaxId,
		SupplierCreditTaxType:             input.SupplierCreditTaxType,
		SupplierCreditTaxAmount:           supplierCreditTaxAmount,
		CurrentStatus:                     SupplierCreditStatusDraft,
		Documents:                         documents,
		Details:                           supplierCreditItems,
		SupplierCreditTotalDiscountAmount: totalSupplierCreditDiscountAmount,
		SupplierCreditTotalTaxAmount:      totalSupplierCreditTaxAmount,
		SupplierCreditSubtotal:            supplierCreditSubtotal,
		SupplierCreditTotalAmount:         supplierCreditTotalAmount,
		RemainingBalance:                  supplierCreditTotalAmount,
	}

	seqNo, err := utils.GetSequence[SupplierCredit](ctx, businessId)
	if err != nil {
		tx.Rollback()
		return nil, err
	}
	prefix, err := getTransactionPrefix(ctx, input.BranchId, "Supplier Credit")
	if err != nil {
		tx.Rollback()
		return nil, err
	}
	supplierCredit.SequenceNo = decimal.NewFromInt(seqNo)
	supplierCredit.SupplierCreditNumber = prefix + fmt.Sprint(seqNo)

	err = tx.WithContext(ctx).Create(&supplierCredit).Error
	if err != nil {
		tx.Rollback()
		return nil, err
	}

	// If requested "Confirmed", apply the status transition deterministically (Draft -> Confirmed).
	if requestedStatus == SupplierCreditStatusConfirmed {
		if err := tx.WithContext(ctx).Model(&supplierCredit).Update("CurrentStatus", SupplierCreditStatusConfirmed).Error; err != nil {
			tx.Rollback()
			return nil, err
		}
		supplierCredit.CurrentStatus = SupplierCreditStatusConfirmed

		// Apply inventory side-effects deterministically (prefer explicit command handler).
		if config.UseStockCommandsFor("SUPPLIER_CREDIT") {
			if err := ApplySupplierCreditStockForStatusTransition(tx.WithContext(ctx), &supplierCredit, SupplierCreditStatusDraft); err != nil {
				tx.Rollback()
				return nil, err
			}
		} else {
			if err := supplierCredit.AfterUpdateCurrentStatus(tx.WithContext(ctx), string(SupplierCreditStatusDraft)); err != nil {
				tx.Rollback()
				return nil, err
			}
		}

		// Write outbox record (publishing happens after commit via dispatcher).
		if err := PublishToAccounting(ctx, tx, businessId, supplierCredit.SupplierCreditDate, supplierCredit.ID, AccountReferenceTypeSupplierCredit, supplierCredit, nil, PubSubMessageActionCreate); err != nil {
			tx.Rollback()
			return nil, err
		}
	}

	if err := tx.Commit().Error; err != nil {
		return nil, err
	}

	return &supplierCredit, nil
}

func UpdateSupplierCredit(ctx context.Context, supplierCreditID int, updatedSupplierCredit *NewSupplierCredit) (*SupplierCredit, error) {
	db := config.GetDB()

	businessId, ok := utils.GetBusinessIdFromContext(ctx)
	if !ok || businessId == "" {
		return nil, errors.New("business id is required")
	}

	if err := updatedSupplierCredit.validate(ctx, businessId, supplierCreditID); err != nil {
		return nil, err
	}

	// Fetch the existing purchase order
	oldSupplierCredit, err := utils.FetchModelForChange[SupplierCredit](ctx, businessId, supplierCreditID, "Details")
	if err != nil {
		return nil, err
	}

	if oldSupplierCredit.isApplied() {
		return nil, errors.New("cannot update applied/refunded supplier credit")
	}
	// Fintech integrity guardrail (behind flag): inventory-affecting docs are immutable after confirm.
	if config.StrictInventoryDocImmutability() && oldSupplierCredit.CurrentStatus == SupplierCreditStatusConfirmed {
		return nil, errors.New("cannot edit a confirmed supplier credit; void and recreate to preserve inventory/valuation integrity")
	}

	var existingSupplierCredit = *oldSupplierCredit
	oldStatus := existingSupplierCredit.CurrentStatus

	oldCreditDate := utils.NilOrElse(existingSupplierCredit.SupplierCreditDate.Equal(updatedSupplierCredit.SupplierCreditDate), existingSupplierCredit.SupplierCreditDate)
	oldWarehouseId := utils.NilOrElse(existingSupplierCredit.WarehouseId == updatedSupplierCredit.WarehouseId, existingSupplierCredit.WarehouseId)

	// Update the fields of the existing purchase order with the provided updated details
	existingSupplierCredit.SupplierId = updatedSupplierCredit.SupplierId
	existingSupplierCredit.BranchId = updatedSupplierCredit.BranchId
	// existingSupplierCredit.SupplierCreditNumber = updatedSupplierCredit.SupplierCreditNumber
	existingSupplierCredit.ReferenceNumber = updatedSupplierCredit.ReferenceNumber
	existingSupplierCredit.SupplierCreditDate = updatedSupplierCredit.SupplierCreditDate
	existingSupplierCredit.SupplierCreditSubject = updatedSupplierCredit.SupplierCreditSubject
	existingSupplierCredit.Notes = updatedSupplierCredit.Notes
	existingSupplierCredit.CurrencyId = updatedSupplierCredit.CurrencyId
	existingSupplierCredit.ExchangeRate = updatedSupplierCredit.ExchangeRate
	existingSupplierCredit.SupplierCreditDiscount = updatedSupplierCredit.SupplierCreditDiscount
	existingSupplierCredit.SupplierCreditDiscountType = updatedSupplierCredit.SupplierCreditDiscountType
	existingSupplierCredit.AdjustmentAmount = updatedSupplierCredit.AdjustmentAmount
	existingSupplierCredit.IsTaxInclusive = updatedSupplierCredit.IsTaxInclusive
	existingSupplierCredit.SupplierCreditTaxId = updatedSupplierCredit.SupplierCreditTaxId
	existingSupplierCredit.SupplierCreditTaxType = updatedSupplierCredit.SupplierCreditTaxType
	existingSupplierCredit.WarehouseId = updatedSupplierCredit.WarehouseId
	existingSupplierCredit.CurrentStatus = updatedSupplierCredit.CurrentStatus

	var orderSubtotal,
		orderTotalAmount,
		totalExclusiveTaxAmount,
		totalDetailDiscountAmount,
		totalDetailTaxAmount decimal.Decimal

	// Iterate through the updated items
	tx := db.Begin()
	for _, updatedItem := range updatedSupplierCredit.Details {
		var existingItem *SupplierCreditDetail
		if updatedItem.DetailId > 0 {
			// Find the live slice element by ID (safe even if prior iterations deleted items).
			for i := range existingSupplierCredit.Details {
				if existingSupplierCredit.Details[i].ID == updatedItem.DetailId {
					existingItem = &existingSupplierCredit.Details[i]
					break
				}
			}
		}

		// nil if product does not exist in DB
		var itemProduct *ProductInterface
		// If the item doesn't exist, add it to the purchase order
		if existingItem == nil {
			newItem := SupplierCreditDetail{
				SupplierCreditId:   existingSupplierCredit.ID,
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

			if updatedItem.ProductId > 0 {
				product, err := GetProductOrVariant(ctx, string(updatedItem.ProductType), updatedItem.ProductId)
				if err != nil {
					tx.Rollback()
					return nil, err
				}
				itemProduct = &product
			}

			// Calculate tax and total amounts for the item
			newItem.CalculateItemDiscountAndTax(ctx, *updatedSupplierCredit.IsTaxInclusive)
			orderSubtotal, totalDetailDiscountAmount, totalDetailTaxAmount, totalExclusiveTaxAmount = updateSupplierCreditDetailTotal(&newItem, *updatedSupplierCredit.IsTaxInclusive, orderSubtotal, totalExclusiveTaxAmount, totalDetailDiscountAmount, totalDetailTaxAmount)
			newItem.Cogs = decimal.NewFromInt(0)

			// Persist detail row deterministically (avoid duplicate detail rows on edit).
			if err := tx.WithContext(ctx).Create(&newItem).Error; err != nil {
				tx.Rollback()
				return nil, err
			}
			existingSupplierCredit.Details = append(existingSupplierCredit.Details, newItem)

			if itemProduct != nil && (*itemProduct).GetInventoryAccountID() > 0 && existingSupplierCredit.CurrentStatus == SupplierCreditStatusConfirmed {
				// no need to check for old values to subtract since new
				if err := ValidateProductStock(tx, ctx, businessId, existingSupplierCredit.WarehouseId, updatedItem.BatchNumber, updatedItem.ProductType, updatedItem.ProductId, updatedItem.DetailQty); err != nil {
					tx.Rollback()
					return nil, err
				}
				if err := UpdateStockSummaryReceivedQty(tx, businessId,
					existingSupplierCredit.WarehouseId,
					updatedItem.ProductId,
					string(updatedItem.ProductType),
					updatedItem.BatchNumber,
					updatedItem.DetailQty.Neg(),
					existingSupplierCredit.SupplierCreditDate); err != nil {
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
				var deletedItemIndex int
				for i, detail := range existingSupplierCredit.Details {
					if detail.ID == updatedItem.DetailId {
						deletedItemIndex = i
						break
					}
				}
				// Delete the item from the database
				if err := tx.WithContext(ctx).Delete(&existingSupplierCredit.Details[deletedItemIndex]).Error; err != nil {
					tx.Rollback()
					return nil, err
				}

				// Remove the item from the slice
				existingSupplierCredit.Details = append(existingSupplierCredit.Details[:deletedItemIndex], existingSupplierCredit.Details[deletedItemIndex+1:]...)

				if existingItem.ProductId > 0 {
					product, err := GetProductOrVariant(ctx, string(existingItem.ProductType), existingItem.ProductId)
					if err != nil {
						tx.Rollback()
						return nil, err
					}
					if product.GetInventoryAccountID() > 0 && existingSupplierCredit.CurrentStatus == SupplierCreditStatusConfirmed {

						if err := UpdateStockSummaryReceivedQty(tx, businessId,
							utils.DereferencePtr(oldWarehouseId, updatedSupplierCredit.WarehouseId),
							existingItem.ProductId,
							string(existingItem.ProductType),
							existingItem.BatchNumber,
							existingItem.DetailQty,
							utils.DereferencePtr(oldCreditDate, existingSupplierCredit.SupplierCreditDate)); err != nil {
							tx.Rollback()
							return nil, err
						}
					}
				}

			} else {
				// Update existing item details

				// nil if values don't change, old value otherwise
				oldDetailQty := utils.NilOrElse(existingItem.DetailQty.Equal(updatedItem.DetailQty), existingItem.DetailQty)
				oldBatchNumber := utils.NilOrElse(existingItem.BatchNumber == updatedItem.BatchNumber, existingItem.BatchNumber)

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
				existingItem.CalculateItemDiscountAndTax(ctx, *updatedSupplierCredit.IsTaxInclusive)
				orderSubtotal, totalDetailDiscountAmount, totalDetailTaxAmount, totalExclusiveTaxAmount = updateSupplierCreditDetailTotal(existingItem, *updatedSupplierCredit.IsTaxInclusive, orderSubtotal, totalExclusiveTaxAmount, totalDetailDiscountAmount, totalDetailTaxAmount)
				// existingSupplierCredit.Details = append(existingSupplierCredit.Details, *existingItem)

				if err := tx.WithContext(ctx).Save(existingItem).Error; err != nil {
					tx.Rollback()
					return nil, err
				}

				if itemProduct != nil && (*itemProduct).GetInventoryAccountID() > 0 && existingSupplierCredit.CurrentStatus == SupplierCreditStatusConfirmed {
					if oldStatus == SupplierCreditStatusDraft {
						// newly confirm
						// check for stock availability
						if err := ValidateProductStock(tx, ctx, businessId, existingSupplierCredit.WarehouseId, existingItem.BatchNumber, existingItem.ProductType, existingItem.ProductId, existingItem.DetailQty); err != nil {
							tx.Rollback()
							return nil, err
						}
						// no need to check for old values to subtract since new
						if err := UpdateStockSummaryReceivedQty(tx, businessId,
							existingSupplierCredit.WarehouseId,
							existingItem.ProductId,
							string(existingItem.ProductType),
							existingItem.BatchNumber,
							existingItem.DetailQty.Neg(),
							existingSupplierCredit.SupplierCreditDate); err != nil {
							tx.Rollback()
							return nil, err
						}
					} else if oldStatus == SupplierCreditStatusConfirmed {
						// updating existing item
						// if belongs to different stock_summary_daily_balances record
						if oldCreditDate != nil || oldWarehouseId != nil || oldBatchNumber != nil {
							//? lock business

							// subtract old values from old record
							if err := UpdateStockSummaryReceivedQty(tx, businessId,
								utils.DereferencePtr(oldWarehouseId, existingSupplierCredit.WarehouseId),
								existingItem.ProductId,
								string(existingItem.ProductType),
								utils.DereferencePtr(oldBatchNumber, existingItem.BatchNumber),
								utils.DereferencePtr(oldDetailQty, existingItem.DetailQty),
								utils.DereferencePtr(oldCreditDate, existingSupplierCredit.SupplierCreditDate)); err != nil {
								tx.Rollback()
								return nil, err
							}
							// validate stock
							if err := ValidateProductStock(tx, ctx, businessId,
								existingSupplierCredit.WarehouseId,
								existingItem.BatchNumber,
								existingItem.ProductType,
								existingItem.ProductId,
								existingItem.DetailQty); err != nil {
								tx.Rollback()
								return nil, err
							}
							// add to new stock_summary_daily_balances
							if err := UpdateStockSummaryReceivedQty(tx, businessId,
								existingSupplierCredit.WarehouseId,
								existingItem.ProductId,
								string(existingItem.ProductType),
								existingItem.BatchNumber,
								existingItem.DetailQty.Neg(),
								existingSupplierCredit.SupplierCreditDate,
							); err != nil {
								tx.Rollback()
								return nil, err
							}
						} else if oldDetailQty != nil {
							// belongs to same stock_summary_daily_balance and quantity is updated
							addedQty := existingItem.DetailQty.Sub(*oldDetailQty)
							if err := ValidateProductStock(tx, ctx, businessId, existingSupplierCredit.WarehouseId, existingItem.BatchNumber, existingItem.ProductType, existingItem.ProductId, addedQty); err != nil {
								tx.Rollback()
								return nil, err
							}
							if err := UpdateStockSummaryReceivedQty(tx, businessId,
								existingSupplierCredit.WarehouseId,
								existingItem.ProductId,
								string(existingItem.ProductType),
								existingItem.BatchNumber,
								addedQty.Neg(),
								existingSupplierCredit.SupplierCreditDate); err != nil {
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
	var orderDiscountAmount decimal.Decimal

	if updatedSupplierCredit.SupplierCreditDiscountType != nil {
		orderDiscountAmount = utils.CalculateDiscountAmount(orderSubtotal, updatedSupplierCredit.SupplierCreditDiscount, string(*updatedSupplierCredit.SupplierCreditDiscountType))
	}

	existingSupplierCredit.SupplierCreditDiscountAmount = orderDiscountAmount

	// orderSubtotal = orderSubtotal.Sub(orderDiscountAmount)
	existingSupplierCredit.SupplierCreditSubtotal = orderSubtotal

	// calculate order tax amount (always exclusive)
	var orderTaxAmount decimal.Decimal
	if updatedSupplierCredit.SupplierCreditTaxId > 0 {
		if *updatedSupplierCredit.SupplierCreditTaxType == TaxTypeGroup {
			orderTaxAmount = utils.CalculateTaxAmount(ctx, db, updatedSupplierCredit.SupplierCreditTaxId, true, orderSubtotal, false)
		} else {
			orderTaxAmount = utils.CalculateTaxAmount(ctx, db, updatedSupplierCredit.SupplierCreditTaxId, false, orderSubtotal, false)
		}
	} else {
		orderTaxAmount = decimal.NewFromFloat(0)
	}

	existingSupplierCredit.SupplierCreditTaxAmount = orderTaxAmount

	// Sum (order discount + total detail discount)
	totalOrderDiscountAmount := orderDiscountAmount.Add(totalDetailDiscountAmount)
	existingSupplierCredit.SupplierCreditTotalDiscountAmount = totalOrderDiscountAmount

	// Sum (order tax amount + total detail tax amount)
	totalOrderTaxAmount := orderTaxAmount.Add(totalDetailTaxAmount)
	existingSupplierCredit.SupplierCreditTotalTaxAmount = totalOrderTaxAmount
	// Sum Grand total amount (subtotal+ exclusive tax + adj amount)
	orderTotalAmount = orderSubtotal.Add(orderTaxAmount).Add(totalExclusiveTaxAmount).Add(updatedSupplierCredit.AdjustmentAmount).Sub(orderDiscountAmount)

	existingSupplierCredit.SupplierCreditTotalAmount = orderTotalAmount
	existingSupplierCredit.RemainingBalance = orderTotalAmount

	// Save the updated supplier credit to the database
	if err := tx.WithContext(ctx).Save(&existingSupplierCredit).Error; err != nil {
		tx.Rollback()
		return nil, err
	}
	// Refresh the existingBill to get the latest details
	if err := tx.WithContext(ctx).Preload("Details").First(&existingSupplierCredit, supplierCreditID).Error; err != nil {
		tx.Rollback()
		return nil, err
	}

	if oldStatus == SupplierCreditStatusDraft && existingSupplierCredit.CurrentStatus == SupplierCreditStatusConfirmed {
		err := PublishToAccounting(ctx, tx, businessId, existingSupplierCredit.SupplierCreditDate, existingSupplierCredit.ID, AccountReferenceTypeSupplierCredit, existingSupplierCredit, nil, PubSubMessageActionCreate)
		if err != nil {
			tx.Rollback()
			return nil, err
		}
	} else if oldStatus == SupplierCreditStatusConfirmed && existingSupplierCredit.CurrentStatus == SupplierCreditStatusConfirmed {
		err := PublishToAccounting(ctx, tx, businessId, existingSupplierCredit.SupplierCreditDate, existingSupplierCredit.ID, AccountReferenceTypeSupplierCredit, existingSupplierCredit, oldSupplierCredit, PubSubMessageActionUpdate)
		if err != nil {
			tx.Rollback()
			return nil, err
		}
	}

	documents, err := upsertDocuments(ctx, tx, updatedSupplierCredit.Documents, "supplier_credits", supplierCreditID)
	if err != nil {
		tx.Rollback()
		return nil, err
	}
	existingSupplierCredit.Documents = documents

	if err := tx.Commit().Error; err != nil {
		return nil, err
	}

	return &existingSupplierCredit, nil
}

func DeleteSupplierCredit(ctx context.Context, id int) (*SupplierCredit, error) {
	businessId, ok := utils.GetBusinessIdFromContext(ctx)
	if !ok || businessId == "" {
		return nil, errors.New("business id is required")
	}

	db := config.GetDB()
	oldSupplierCredit, err := utils.FetchModelForChange[SupplierCredit](ctx, businessId, id, "Details", "Documents")
	if err != nil {
		return nil, err
	}

	if oldSupplierCredit.isApplied() {
		return nil, errors.New("cannot delete applied/refunded supplier credit")
	}

	result := *oldSupplierCredit

	tx := db.Begin()

	// increase received qty from stock summary if supplier credit is confirmed
	if oldSupplierCredit.CurrentStatus == SupplierCreditStatusConfirmed {

		for _, item := range oldSupplierCredit.Details {
			if item.ProductId > 0 {
				product, err := GetProductOrVariant(ctx, string(item.ProductType), item.ProductId)
				if err != nil {
					tx.Rollback()
					return nil, err
				}
				if product.GetInventoryAccountID() > 0 {
					if err := UpdateStockSummaryReceivedQty(tx, oldSupplierCredit.BusinessId, oldSupplierCredit.WarehouseId, item.ProductId, string(item.ProductType), item.BatchNumber, item.DetailQty, oldSupplierCredit.SupplierCreditDate); err != nil {
						tx.Rollback()
						return nil, err
					}
				}
			}

		}

		var creditBills []SupplierCreditBill

		err := tx.WithContext(ctx).Where("business_id = ?", businessId).
			Where("reference_id = ?", id).
			Where("reference_type = ?", SupplierCreditApplyTypeCredit).
			Find(&creditBills).Error
		if err != nil {
			tx.Rollback()
			return nil, err
		}

		// Adjust BillTotalCreditAmount for each associated PaidBill
		for _, creditBill := range creditBills {
			bill, err := utils.FetchModelForChange[Bill](ctx, businessId, creditBill.BillId)
			if err != nil {
				tx.Rollback()
				return nil, err
			}

			// remainingPaidAmount := bill.BillTotalCreditAmount.Sub(creditBill.PaidAmount)
			billPaymentAmount := bill.BillTotalPaidAmount
			billAdvanceAmount := bill.BillTotalAdvanceUsedAmount
			billCreditAmount := bill.BillTotalCreditUsedAmount
			remainingPaidAmount := billPaymentAmount.Add(billAdvanceAmount).Add(billCreditAmount).Sub(creditBill.Amount)

			if remainingPaidAmount.IsNegative() {
				tx.Rollback()
				return nil, errors.New("resulting BillTotalPaidAmount cannot be negative")
			}
			if remainingPaidAmount.GreaterThan(decimal.Zero) {
				bill.CurrentStatus = BillStatusPartialPaid
			} else {
				bill.CurrentStatus = BillStatusConfirmed
			}
			bill.BillTotalCreditUsedAmount = billCreditAmount.Sub(creditBill.Amount)
			bill.RemainingBalance = bill.RemainingBalance.Add(creditBill.Amount)

			// Update the bill
			if err := tx.WithContext(ctx).Save(&bill).Error; err != nil {
				tx.Rollback()
				return nil, err
			}

			err = tx.WithContext(ctx).Delete(&creditBill).Error
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

	if result.CurrentStatus == SupplierCreditStatusConfirmed {
		err = PublishToAccounting(ctx, tx, businessId, oldSupplierCredit.SupplierCreditDate, oldSupplierCredit.ID, AccountReferenceTypeSupplierCredit, nil, oldSupplierCredit, PubSubMessageActionDelete)
		if err != nil {
			tx.Rollback()
			return nil, err
		}
	}

	if err := deleteDocuments(ctx, tx, oldSupplierCredit.Documents); err != nil {
		tx.Rollback()
		return nil, err
	}

	if err := tx.Commit().Error; err != nil {
		return nil, err
	}

	return oldSupplierCredit, nil
}

func UpdateStatusSupplierCredit(ctx context.Context, id int, status string) (*SupplierCredit, error) {
	businessId, ok := utils.GetBusinessIdFromContext(ctx)
	if !ok || businessId == "" {
		return nil, errors.New("business id is required")
	}
	supplierCredit, err := utils.FetchModelForChange[SupplierCredit](ctx, businessId, id, "Details")
	if err != nil {
		return nil, err
	}
	oldStatus := supplierCredit.CurrentStatus

	// db action
	db := config.GetDB()
	tx := db.Begin()

	err = tx.WithContext(ctx).Model(&supplierCredit).Updates(map[string]interface{}{
		"CurrentStatus": status,
	}).Error

	if err != nil {
		tx.Rollback()
		return nil, err
	}

	// Apply inventory side-effects deterministically (prefer explicit command handler).
	if config.UseStockCommandsFor("SUPPLIER_CREDIT") {
		supplierCredit.CurrentStatus = SupplierCreditStatus(status)
		if err := ApplySupplierCreditStockForStatusTransition(tx.WithContext(ctx), supplierCredit, oldStatus); err != nil {
			tx.Rollback()
			return nil, err
		}
	} else {
		if err := supplierCredit.AfterUpdateCurrentStatus(tx.WithContext(ctx), string(oldStatus)); err != nil {
			tx.Rollback()
			return nil, err
		}
	}

	if oldStatus == SupplierCreditStatusDraft && status == string(SupplierCreditStatusConfirmed) {
		err := PublishToAccounting(ctx, tx, businessId, supplierCredit.SupplierCreditDate, supplierCredit.ID, AccountReferenceTypeSupplierCredit, supplierCredit, nil, PubSubMessageActionCreate)
		if err != nil {
			tx.Rollback()
			return nil, err
		}
	} else if oldStatus == SupplierCreditStatusConfirmed && status == string(SupplierCreditStatusVoid) {
		err = PublishToAccounting(ctx, tx, businessId, supplierCredit.SupplierCreditDate, supplierCredit.ID, AccountReferenceTypeSupplierCredit, nil, supplierCredit, PubSubMessageActionDelete)
		if err != nil {
			tx.Rollback()
			return nil, err
		}
	}

	// Commit the transaction
	if err := tx.Commit().Error; err != nil {
		return nil, err
	}

	return supplierCredit, nil
}

func GetSupplierCredit(ctx context.Context, id int) (*SupplierCredit, error) {
	db := config.GetDB()

	var result SupplierCredit
	err := db.WithContext(ctx).First(&result, id).Error
	if err != nil {
		return nil, err
	}

	return &result, nil
}

func GetConfirmedSupplierCredit(tx *gorm.DB, ctx context.Context, id int, businessId string) (*SupplierCredit, error) {
	var result SupplierCredit
	err := tx.WithContext(ctx).
		Where("business_id = ?", businessId).
		Not("current_status IN (?)", []string{string(SupplierCreditStatusDraft), string(SupplierCreditStatusVoid)}).
		First(&result, id).Error
	if err != nil {
		tx.Rollback()
		return nil, errors.New("reference id (for supplier credit) not found")
	}
	return &result, nil
}

func GetConfirmedSupplierAdvance(tx *gorm.DB, ctx context.Context, id int, businessId string) (*SupplierCreditAdvance, error) {
	var result SupplierCreditAdvance
	err := tx.WithContext(ctx).
		Where("business_id = ?", businessId).
		Where("current_status = ?", SupplierAdvanceStatusConfirmed).
		First(&result, id).Error
	if err != nil {
		tx.Rollback()
		return nil, errors.New("reference id (for supplier advance) not found")
	}
	return &result, nil
}

func GetSupplierCredits(ctx context.Context, creditNumber *string) ([]*SupplierCredit, error) {
	db := config.GetDB()
	var results []*SupplierCredit

	businessId, ok := utils.GetBusinessIdFromContext(ctx)
	if !ok || businessId == "" {
		return nil, errors.New("business id is required")
	}

	dbCtx := db.WithContext(ctx).Where("business_id = ?", businessId)
	if creditNumber != nil && len(*creditNumber) > 0 {
		dbCtx = dbCtx.Where("credit_number LIKE ?", "%"+*creditNumber+"%")
	}
	err := dbCtx.
		Find(&results).Error
	if err != nil {
		return nil, err
	}
	return results, nil
}

func GetUnusedSupplierCredits(ctx context.Context, branchId int, supplierId int) ([]*SupplierCredit, error) {
	var results []*SupplierCredit

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
		Where("supplier_id = ?", supplierId).
		Where("current_status = ?", SupplierCreditStatusConfirmed).
		Find(&results).Error

	if err != nil {
		return nil, err
	}
	return results, nil
}

func GetUnusedSupplierAdvances(ctx context.Context, branchId int, supplierId int) ([]*SupplierCreditAdvance, error) {
	var results []*SupplierCreditAdvance

	db := config.GetDB()
	dbCtx := db.WithContext(ctx).Where("supplier_id = ?", supplierId)

	if branchId > 0 {
		dbCtx.Where("branch_id = ?", branchId)
	}
	err := dbCtx.Where("current_status = ? AND remaining_balance > 0", SupplierAdvanceStatusConfirmed).
		Find(&results).Error

	if err != nil {
		return nil, err
	}
	return results, nil
}

func PaginateSupplierCredit(ctx context.Context, limit *int, after *string,
	supplierCreditNumber *string,
	referenceNumber *string,
	branchID *int,
	warehouseID *int,
	supplierID *int,
	currentStatus *SupplierCreditStatus,
	startSupplierCreditDate *MyDateString,
	endSupplierCreditDate *MyDateString,
) (*SupplierCreditsConnection, error) {

	businessId, ok := utils.GetBusinessIdFromContext(ctx)
	if !ok || businessId == "" {
		return nil, errors.New("business id is required")
	}

	business, err := GetBusiness(ctx)
	if err != nil {
		return nil, errors.New("business id is required")
	}
	if err := startSupplierCreditDate.StartOfDayUTCTime(business.Timezone); err != nil {
		return nil, err
	}
	if err := endSupplierCreditDate.EndOfDayUTCTime(business.Timezone); err != nil {
		return nil, err
	}

	db := config.GetDB()
	dbCtx := db.WithContext(ctx).Where("business_id = ?", businessId)

	if supplierCreditNumber != nil && *supplierCreditNumber != "" {
		dbCtx.Where("supplier_credit_number LIKE ?", "%"+*supplierCreditNumber+"%")
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
	if startSupplierCreditDate != nil && endSupplierCreditDate != nil {
		dbCtx.Where("supplier_credit_date BETWEEN ? AND ?", startSupplierCreditDate, endSupplierCreditDate)
	}

	edges, pageInfo, err := FetchPageCompositeCursor[SupplierCredit](dbCtx, *limit, after, "created_at", "<")
	if err != nil {
		return nil, err
	}
	var supplierCreditsConnection SupplierCreditsConnection
	supplierCreditsConnection.PageInfo = pageInfo
	for _, edge := range edges {
		supplierCreditEdge := SupplierCreditsEdge(edge)
		supplierCreditsConnection.Edges = append(supplierCreditsConnection.Edges, &supplierCreditEdge)
	}

	return &supplierCreditsConnection, err
}
