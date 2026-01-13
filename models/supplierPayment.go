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

type SupplierPayment struct {
	ID                int                `gorm:"primary_key" json:"id"`
	BusinessId        string             `gorm:"index;not null" json:"business_id" binding:"required"`
	SupplierId        int                `gorm:"index;not null" json:"supplier_id" binding:"required"`
	BranchId          int                `gorm:"index;not null" json:"branch_id"`
	CurrencyId        int                `gorm:"not null" json:"currency_id" binding:"required"`
	ExchangeRate      decimal.Decimal    `gorm:"type:decimal(20,4);default:0" json:"exchange_rate"`
	Amount            decimal.Decimal    `gorm:"type:decimal(20,4);default:0" json:"amount" binding:"required"`
	BankCharges       decimal.Decimal    `gorm:"type:decimal(20,4);default:0" json:"bank_charges"`
	PaymentDate       time.Time          `gorm:"not null" json:"payment_date" binding:"required"`
	PaymentNumber     string             `gorm:"size:255;not null" json:"payment_number" binding:"required"`
	SequenceNo        decimal.Decimal    `gorm:"type:decimal(15);not null" json:"sequence_no"`
	PaymentModeId     int                `gorm:"default:null" json:"payment_mode_id"`
	WithdrawAccountId int                `gorm:"default:null" json:"withdraw_account_id"`
	ReferenceNumber   string             `gorm:"size:255;default:null" json:"reference_number"`
	Notes             string             `gorm:"type:text;default:null" json:"notes"`
	Documents         []*Document        `gorm:"polymorphic:Reference" json:"documents"`
	PaidBills         []SupplierPaidBill `json:"paid_bills" validate:"required,dive,required"`
	CreatedAt         time.Time          `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt         time.Time          `gorm:"autoUpdateTime" json:"updated_at"`
}

type NewSupplierPayment struct {
	BusinessId        string          `json:"business_id" binding:"required"`
	SupplierId        int             `json:"supplier_id" binding:"required"`
	BranchId          int             `json:"branch_id" binding:"required"`
	CurrencyId        int             `json:"currency_id" binding:"required"`
	ExchangeRate      decimal.Decimal `json:"exchange_rate"`
	Amount            decimal.Decimal `json:"amount" binding:"required"`
	BankCharges       decimal.Decimal `json:"bank_charges"`
	PaymentDate       time.Time       `json:"payment_date" binding:"required"`
	PaymentModeId     int             `json:"payment_mode"`
	WithdrawAccountId int             `json:"withdraw_account_id"`
	ReferenceNumber   string          `json:"reference_number"`
	Notes             string          `json:"notes"`
	PaidBills         []NewPaidBill   `json:"paid_bills"`
	Documents         []*NewDocument  `json:"documents"`
}

type SupplierPaidBill struct {
	ID                int             `gorm:"primary_key" json:"id"`
	SupplierPaymentId int             `json:"supplier_payment_id" binding:"required"`
	BillId            int             `json:"bill_id" binding:"required"`
	PaidAmount        decimal.Decimal `gorm:"type:decimal(20,4);default:0" json:"paid_amount" binding:"required"`
	// WithholdingTax    decimal.Decimal `gorm:"type:decimal(20,4);default:0" json:"withholding_tax" binding:"required"`
	CreatedAt time.Time `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt time.Time `gorm:"autoUpdateTime" json:"updated_at"`
}

type NewPaidBill struct {
	PaidBillId int             `json:"paid_bill_id"`
	BillId     int             `json:"bill_id" binding:"required"`
	PaidAmount decimal.Decimal `json:"paid_amount" binding:"required"`
	// WithholdingTax decimal.Decimal `json:"withholding_tax" binding:"required"`
}

type SupplierPaymentsConnection struct {
	Edges    []*SupplierPaymentsEdge `json:"edges"`
	PageInfo *PageInfo               `json:"pageInfo"`
}

type SupplierPaymentsEdge Edge[SupplierPayment]

func (obj SupplierPayment) GetId() int {
	return obj.ID
}

func (input NewSupplierPayment) validate(ctx context.Context, businessId string) error {

	// exists supplier
	if err := utils.ValidateResourceId[Supplier](ctx, businessId, input.SupplierId); err != nil {
		return errors.New("supplier not found")
	}
	// exists branch
	if err := utils.ValidateResourceId[Branch](ctx, businessId, input.BranchId); err != nil {
		return errors.New("branch not found")
	}
	// exists Currency
	if err := utils.ValidateResourceId[Currency](ctx, businessId, input.CurrencyId); err != nil {
		return errors.New("currency not found")
	}
	// validate PaymentDate
	if err := validateTransactionLock(ctx, input.PaymentDate, businessId, PurchaseTransactionLock); err != nil {
		return err
	}

	return nil
}

func (sp SupplierPayment) CheckTransactionLock(ctx context.Context) error {
	return validateTransactionLock(ctx, sp.PaymentDate, sp.BusinessId, PurchaseTransactionLock)
}

// implements methods for pagination

// node
// returns decoded curosr string
func (sp SupplierPayment) GetCursor() string {
	return sp.CreatedAt.String()
}

// GetID method for SupplierPayment reference Data
func (sp *SupplierPayment) GetID() int {
	return sp.ID
}

// GetID method for SupplierPaidBill reference Data
func (spb *SupplierPaidBill) GetID() int {
	return spb.ID
}

func CreateSupplierPayment(ctx context.Context, input *NewSupplierPayment) (*SupplierPayment, error) {

	db := config.GetDB()

	businessId, ok := utils.GetBusinessIdFromContext(ctx)
	if !ok || businessId == "" {
		return nil, errors.New("business id is required")
	}

	business, err := GetBusinessById(ctx, businessId)
	if err != nil {
		return nil, err
	}

	if err := input.validate(ctx, businessId); err != nil {
		return nil, err
	}

	tx := db.Begin()
	// construct paidBills
	paidBills, err := mapPaidBillsInput(tx, ctx, businessId, input)
	if err != nil {
		tx.Rollback()
		return nil, err
	}

	// construct documents
	documents, err := mapNewDocuments(input.Documents, "supplier_payments", 0)
	if err != nil {
		tx.Rollback()
		return nil, err
	}

	// validate input currency_id
	if input.CurrencyId != business.BaseCurrencyId {
		account, err := GetAccount(ctx, input.WithdrawAccountId)
		if err != nil {
			tx.Rollback()
			return nil, err
		}
		if account.CurrencyId != input.CurrencyId && account.CurrencyId != business.BaseCurrencyId {
			tx.Rollback()
			return nil, errors.New("multiple foreign currencies not allowed")
		}
	}

	supplierPayment := SupplierPayment{
		BusinessId:        businessId,
		SupplierId:        input.SupplierId,
		BranchId:          input.BranchId,
		CurrencyId:        input.CurrencyId,
		ExchangeRate:      input.ExchangeRate,
		Amount:            input.Amount,
		BankCharges:       input.BankCharges,
		PaymentDate:       input.PaymentDate,
		PaymentModeId:     input.PaymentModeId,
		WithdrawAccountId: input.WithdrawAccountId,
		ReferenceNumber:   input.ReferenceNumber,
		Notes:             input.Notes,
		Documents:         documents,
		PaidBills:         paidBills,
	}

	seqNo, err := utils.GetSequence[SupplierPayment](ctx, businessId)
	if err != nil {
		tx.Rollback()
		return nil, err
	}
	prefix, err := getTransactionPrefix(ctx, input.BranchId, "Supplier Payment")
	if err != nil {
		tx.Rollback()
		return nil, err
	}
	supplierPayment.SequenceNo = decimal.NewFromInt(seqNo)
	supplierPayment.PaymentNumber = prefix + fmt.Sprint(seqNo)

	err = tx.WithContext(ctx).Create(&supplierPayment).Error
	if err != nil {
		tx.Rollback()
		return nil, err
	}

	supplierPayment.Documents = nil
	err = PublishToAccounting(ctx, tx, businessId, supplierPayment.PaymentDate, supplierPayment.ID, AccountReferenceTypeSupplierPayment, supplierPayment, nil, PubSubMessageActionCreate)
	if err != nil {
		tx.Rollback()
		return nil, err
	}

	if err = tx.Commit().Error; err != nil {
		return nil, err
	}

	supplierPayment.Documents = documents
	return &supplierPayment, nil
}

func UpdateSupplierPayment(ctx context.Context, paymentID int, updatedSupplierPayment *NewSupplierPayment) (*SupplierPayment, error) {

	businessId, ok := utils.GetBusinessIdFromContext(ctx)
	if !ok || businessId == "" {
		return nil, errors.New("business id is required")
	}

	business, err := GetBusinessById(ctx, businessId)
	if err != nil {
		return nil, err
	}

	// Fetch the existing purchase order
	oldSupplierPayment, err := utils.FetchModelForChange[SupplierPayment](ctx, businessId, paymentID)
	if err != nil {
		return nil, err
	}

	// copy oldSupplierPayment instead of fetching again
	var existingSupplierPayment = *oldSupplierPayment

	db := config.GetDB()
	tx := db.Begin()
	// construct paidBills
	paidBills, err := mapPaidBillsInput(tx, ctx, businessId, updatedSupplierPayment)
	if err != nil {
		tx.Rollback()
		return nil, err
	}

	// validate updatedSupplierPayment currency_id
	if updatedSupplierPayment.CurrencyId != business.BaseCurrencyId {
		account, err := GetAccount(ctx, updatedSupplierPayment.WithdrawAccountId)
		if err != nil {
			tx.Rollback()
			return nil, err
		}
		if account.CurrencyId != updatedSupplierPayment.CurrencyId && account.CurrencyId != business.BaseCurrencyId {
			tx.Rollback()
			return nil, errors.New("multiple foreign currencies not allowed")
		}
	}

	// Update the fields of the existing purchase order with the provided updated details
	existingSupplierPayment.SupplierId = updatedSupplierPayment.SupplierId
	existingSupplierPayment.BranchId = updatedSupplierPayment.BranchId
	existingSupplierPayment.CurrencyId = updatedSupplierPayment.CurrencyId
	existingSupplierPayment.ExchangeRate = updatedSupplierPayment.ExchangeRate
	existingSupplierPayment.Amount = updatedSupplierPayment.Amount
	existingSupplierPayment.BankCharges = updatedSupplierPayment.BankCharges
	existingSupplierPayment.PaymentDate = updatedSupplierPayment.PaymentDate
	existingSupplierPayment.PaymentModeId = updatedSupplierPayment.PaymentModeId
	existingSupplierPayment.WithdrawAccountId = updatedSupplierPayment.WithdrawAccountId
	existingSupplierPayment.ReferenceNumber = updatedSupplierPayment.ReferenceNumber
	existingSupplierPayment.Notes = updatedSupplierPayment.Notes
	existingSupplierPayment.PaidBills = paidBills

	// Save the updated purchase order to the database
	if err := tx.WithContext(ctx).Save(&existingSupplierPayment).Error; err != nil {
		tx.Rollback()
		return nil, err
	}

	documents, err := upsertDocuments(ctx, tx, updatedSupplierPayment.Documents, "supplier_payments", paymentID)
	if err != nil {
		tx.Rollback()
		return nil, err
	}

	oldSupplierPayment.Documents = nil
	existingSupplierPayment.Documents = nil
	err = PublishToAccounting(ctx, tx, businessId, existingSupplierPayment.PaymentDate, existingSupplierPayment.ID, AccountReferenceTypeSupplierPayment, existingSupplierPayment, oldSupplierPayment, PubSubMessageActionUpdate)
	if err != nil {
		tx.Rollback()
		return nil, err
	}

	if err = tx.Commit().Error; err != nil {
		return nil, err
	}

	existingSupplierPayment.Documents = documents
	return &existingSupplierPayment, nil
}

func mapPaidBillsInput(tx *gorm.DB, ctx context.Context, businessId string, input *NewSupplierPayment) ([]SupplierPaidBill, error) {

	var supplierPaidBills []SupplierPaidBill
	var totalBillPaidAmount decimal.Decimal

	for _, paidBillInput := range input.PaidBills {

		if paidBillInput.PaidAmount.IsZero() {
			continue
		}

		if paidBillInput.PaidAmount.IsNegative() {
			return nil, errors.New("the paid amount cannot be negative")
		}

		bill, err := utils.FetchModelForChange[Bill](ctx, businessId, paidBillInput.BillId)
		if err != nil {
			return nil, err
		}

		if paidBillInput.PaidBillId == 0 {

			// paidAmount is greater than billTotalAmount
			if paidBillInput.PaidAmount.Cmp(bill.RemainingBalance) == 1 {
				return nil, errors.New("the amount entered is more than the balance for the selected bill number - " + bill.BillNumber)
			} else if paidBillInput.PaidAmount.Cmp(bill.RemainingBalance) == -1 {
				// paidAmount is less than billRemainingAmount
				bill.CurrentStatus = BillStatusPartialPaid
			} else {
				// paidAmount is equal to billRemainingAmount
				bill.CurrentStatus = BillStatusPaid
			}
			// billRemainingAmount = bill.BillTotalAmount.Sub(bill.BillTotalPaidAmount)
			bill.BillTotalPaidAmount = bill.BillTotalPaidAmount.Add(paidBillInput.PaidAmount)
			bill.RemainingBalance = bill.RemainingBalance.Sub(paidBillInput.PaidAmount)

			paidBill := SupplierPaidBill{
				BillId:     paidBillInput.BillId,
				PaidAmount: paidBillInput.PaidAmount,
			}

			supplierPaidBills = append(supplierPaidBills, paidBill)

		} else if paidBillInput.PaidBillId > 0 {
			var existingPaidBill SupplierPaidBill
			if err := tx.WithContext(ctx).First(&existingPaidBill, paidBillInput.PaidBillId).Error; err != nil {
				return nil, err
			}
			// unpay previous amount before paying updated amount
			bill.BillTotalPaidAmount = bill.BillTotalPaidAmount.Sub(existingPaidBill.PaidAmount).Add(paidBillInput.PaidAmount)
			bill.RemainingBalance = bill.RemainingBalance.Add(existingPaidBill.PaidAmount).Sub(paidBillInput.PaidAmount)
			existingPaidBill.PaidAmount = paidBillInput.PaidAmount
			if bill.RemainingBalance.IsNegative() {
				return nil, errors.New("the amount entered is more than the balance for the selected bill number - " + bill.BillNumber)
			} else if bill.RemainingBalance.IsPositive() { // paidAmount is less than invoiceRemainingAmount
				bill.CurrentStatus = BillStatusPartialPaid
			} else {
				// paidAmount is equal to invoiceRemainingAmount
				bill.CurrentStatus = BillStatusPaid
			}

			if err := tx.WithContext(ctx).Save(&existingPaidBill).Error; err != nil {
				return nil, err
			}
			supplierPaidBills = append(supplierPaidBills, existingPaidBill)
		}

		if err := tx.WithContext(ctx).Save(&bill).Error; err != nil {
			return nil, err
		}

		totalBillPaidAmount = totalBillPaidAmount.Add(paidBillInput.PaidAmount)
	}
	// check equal supplier's paid amount and paid amount of each bills
	// if input.Amount != totalBillPaidAmount {
	if input.Amount.Cmp(totalBillPaidAmount) != 0 {
		return nil, errors.New("the amount entered is not equal for each selected bill")
	}
	return supplierPaidBills, nil
}

func DeleteSupplierPayment(ctx context.Context, id int) (*SupplierPayment, error) {
	businessId, ok := utils.GetBusinessIdFromContext(ctx)
	if !ok || businessId == "" {
		return nil, errors.New("business id is required")
	}

	result, err := utils.FetchModelForChange[SupplierPayment](ctx, businessId, id, "PaidBills", "Documents")
	if err != nil {
		return nil, err
	}

	db := config.GetDB()
	tx := db.Begin()
	// Adjust BillTotalPaidAmount for each associated PaidBill
	for _, paidBill := range result.PaidBills {
		bill, err := utils.FetchModelForChange[Bill](ctx, businessId, paidBill.BillId)
		if err != nil {
			tx.Rollback()
			return nil, err
		}

		// remainingPaidAmount := bill.BillTotalPaidAmount.Sub(paidBill.PaidAmount)
		billPaymentAmount := bill.BillTotalPaidAmount
		billAdvanceAmount := bill.BillTotalAdvanceUsedAmount
		billCreditAmount := bill.BillTotalCreditUsedAmount
		remainingPaidAmount := billPaymentAmount.Add(billAdvanceAmount).Add(billCreditAmount).Sub(paidBill.PaidAmount)
		
		if remainingPaidAmount.IsNegative() {
			tx.Rollback()
			return nil, errors.New("resulting BillTotalPaidAmount cannot be negative")
		}
		if remainingPaidAmount.GreaterThan(decimal.Zero) {
			bill.CurrentStatus = BillStatusPartialPaid
		} else {
			bill.CurrentStatus = BillStatusConfirmed
		}
		bill.BillTotalPaidAmount = billPaymentAmount.Sub(paidBill.PaidAmount)
		bill.RemainingBalance = bill.RemainingBalance.Add(paidBill.PaidAmount)

		// Update the bill
		if err := tx.WithContext(ctx).Save(&bill).Error; err != nil {
			tx.Rollback()
			return nil, err
		}
	}

	err = tx.WithContext(ctx).Model(&result).Association("PaidBills").Unscoped().Clear()
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

	result.Documents = nil
	err = PublishToAccounting(ctx, tx, businessId, result.PaymentDate, result.ID, AccountReferenceTypeSupplierPayment, nil, result, PubSubMessageActionDelete)
	if err != nil {
		tx.Rollback()
		return nil, err
	}

	if err = tx.Commit().Error; err != nil {
		return nil, err
	}

	return result, nil

}

func GetSupplierPayment(ctx context.Context, id int) (*SupplierPayment, error) {
	db := config.GetDB()

	var result SupplierPayment
	err := db.WithContext(ctx).First(&result, id).Error
	if err != nil {
		return nil, err
	}

	return &result, nil
}

func GetSupplierPaidBill(ctx context.Context, id int) (*SupplierPaidBill, error) {
	db := config.GetDB()

	var result SupplierPaidBill
	err := db.WithContext(ctx).First(&result, id).Error
	if err != nil {
		return nil, err
	}

	return &result, nil
}

func GetSupplierPayments(ctx context.Context, paymentNumber *string) ([]*SupplierPayment, error) {
	db := config.GetDB()
	var results []*SupplierPayment

	businessId, ok := utils.GetBusinessIdFromContext(ctx)
	if !ok || businessId == "" {
		return nil, errors.New("business id is required")
	}

	dbCtx := db.WithContext(ctx).Where("business_id = ?", businessId)
	if paymentNumber != nil && len(*paymentNumber) > 0 {
		dbCtx = dbCtx.Where("payment_number LIKE ?", "%"+*paymentNumber+"%")
	}
	err := dbCtx.
		Find(&results).Error
	if err != nil {
		return nil, err
	}
	return results, nil
}

func PaginateSupplierPayment(ctx context.Context,
	limit *int, after *string,
	paymentNumber *string,
	referenceNumber *string,
	branchID *int,
	supplierID *int,
	withdrawAccountId *int,
	startPaymentDate *MyDateString,
	endPaymentDate *MyDateString) (*SupplierPaymentsConnection, error) {

	business, err := GetBusiness(ctx)
	if err != nil {
		return nil, err
	}
	if err := startPaymentDate.StartOfDayUTCTime(business.Timezone); err != nil {
		return nil, err
	}
	if err := endPaymentDate.EndOfDayUTCTime(business.Timezone); err != nil {
		return nil, err
	}

	db := config.GetDB()
	dbCtx := db.WithContext(ctx).Where("business_id = ?", business.ID)
	if paymentNumber != nil && *paymentNumber != "" {
		dbCtx.Where("payment_number LIKE ?", "%"+*paymentNumber+"%")
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
	if withdrawAccountId != nil && *withdrawAccountId > 0 {
		dbCtx.Where("withdraw_account_id = ?", *withdrawAccountId)
	}
	if startPaymentDate != nil && endPaymentDate != nil {
		dbCtx.Where("payment_date BETWEEN ? AND ?", startPaymentDate, endPaymentDate)
	}

	edges, pageInfo, err := FetchPageCompositeCursor[SupplierPayment](dbCtx, *limit, after, "created_at", "<")
	if err != nil {
		return nil, err
	}
	var supplierPaymentsConnection SupplierPaymentsConnection
	supplierPaymentsConnection.PageInfo = pageInfo
	for _, edge := range edges {
		supplierPaymentEdge := SupplierPaymentsEdge(edge)
		supplierPaymentsConnection.Edges = append(supplierPaymentsConnection.Edges, &supplierPaymentEdge)
	}

	return &supplierPaymentsConnection, err
}
