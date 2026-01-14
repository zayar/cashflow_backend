package models

import (
	"context"
	"errors"
	"time"

	"github.com/mmdatafocus/books_backend/config"
	"github.com/mmdatafocus/books_backend/utils"
	"github.com/shopspring/decimal"
)

type SupplierCreditBill struct {
	ID                   int                     `gorm:"primary_key" json:"id"`
	BusinessId           string                  `gorm:"autoIncrement:false;not null" json:"business_id" binding:"required"`
	ReferenceId          int                     `gorm:"autoIncrement:false;not null" json:"reference_id" binding:"required"`
	ReferenceType        SupplierCreditApplyType `gorm:"type:enum('Credit','Advance');not null" json:"reference_type"`
	BranchId             int                     `gorm:"not null" json:"branch_id"`
	SupplierId           int                     `gorm:"not null" json:"supplier_id"`
	BillId               int                     `gorm:"autoIncrement:false;not null" json:"bill_id" binding:"required"`
	CreditDate           time.Time               `gorm:"not null" json:"credit_date"`
	Amount               decimal.Decimal         `gorm:"type:decimal(20,4);default:0" json:"amount"`
	SupplierCreditNumber string                  `gorm:"size:255;default:null" json:"supplier_credit_number"`
	BillNumber           string                  `gorm:"size:255;default:null" json:"bill_number"`
	CurrencyId           int                     `gorm:"not null" json:"currency_id" binding:"required"`
	ExchangeRate         decimal.Decimal         `gorm:"type:decimal(20,4);default:0" json:"exchange_rate"`
	BillCurrencyId       int                     `gorm:"default:0" json:"bill_currency_id"`
	BillExchangeRate     decimal.Decimal         `gorm:"type:decimal(20,4);default:0" json:"bill_exchange_rate"`
	CreatedAt            time.Time               `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt            time.Time               `gorm:"autoUpdateTime" json:"updated_at"`
}

// type NewSupplierCreditApplyToBill struct {
// 	SupplierCreditId int                  `json:"supplier_credit_id" binding:"required"`
// 	ApplyBills       []NewCreditApplyBill `json:"bills"`
// }
// type NewCreditApplyBill struct {
// 	BillId       int             `json:"bill_id" binding:"required"`
// 	BillNumber   string          `json:"bill_number"`
// 	Amount       decimal.Decimal `json:"amount"`
// 	CurrencyId   int             `json:"currency_id"`
// 	ExchangeRate decimal.Decimal `json:"exchange_rate"`
// }

type NewBillApplyToSupplierCredit struct {
	BillId       int                  `json:"bill_id" binding:"required"`
	ApplyCredits []NewBillApplyCredit `json:"credits"`
}

type NewBillApplyCredit struct {
	ReferenceId   int                     `json:"reference_id"`
	ReferenceType SupplierCreditApplyType `json:"reference_type"`
	Amount        decimal.Decimal         `json:"amount"`
	CurrencyId    int                     `json:"currency_id"`
	ExchangeRate  decimal.Decimal         `json:"exchange_rate"`
}

func (scb SupplierCreditBill) CheckTransactionLock(ctx context.Context) error {
	return validateTransactionLock(ctx, scb.CreditDate, scb.BusinessId, PurchaseTransactionLock)
}

// commenting since not used, has multi-currency issues
// // apply one credit to multiple bills
// func CreateSupplierApplyToBill(ctx context.Context, input *NewSupplierCreditApplyToBill) ([]*SupplierCreditBill, error) {

// 	businessId, ok := utils.GetBusinessIdFromContext(ctx)
// 	if !ok || businessId == "" {
// 		return nil, errors.New("business id is required")
// 	}

// 	existingSupplierCredit, err := utils.FetchModelForChange[SupplierCredit](ctx, businessId, input.SupplierCreditId)
// 	if err != nil {
// 		return nil, err
// 	}

// 	if existingSupplierCredit.CurrentStatus != SupplierCreditStatusConfirmed {
// 		return nil, errors.New("supplier credit status must be confirm")
// 	}

// 	var supplierCreditTotalUsedAmount decimal.Decimal
// 	var createdSupplierCreditBills []*SupplierCreditBill

// 	db := config.GetDB()
// 	tx := db.Begin()

// 	for _, item := range input.ApplyBills {
// 		var baseCurrencyAmount decimal.Decimal
// 		var billCreditUseAmount, remainingBalance decimal.Decimal

// 		bill, err := utils.FetchModelForChange[Bill](ctx, businessId, item.BillId)
// 		if err != nil {
// 			return nil, err
// 		}

// 		if bill.CurrentStatus != BillStatusConfirmed && bill.CurrentStatus != BillStatusPartialPaid {
// 			tx.Rollback()
// 			return nil, errors.New("bill status must be confirm or partial paid")
// 		}

// 		if item.CurrencyId != existingSupplierCredit.CurrencyId {
// 			baseCurrencyAmount = item.Amount.Mul(item.ExchangeRate)
// 			if baseCurrencyAmount.GreaterThan(bill.RemainingBalance) {
// 				tx.Rollback()
// 				return nil, errors.New("amount less than or equal bill remaining amount")
// 			}

// 			billCreditUseAmount = bill.BillTotalCreditUsedAmount.Add(baseCurrencyAmount)
// 			remainingBalance = bill.RemainingBalance.Sub(baseCurrencyAmount)
// 		} else {
// 			if item.Amount.GreaterThan(bill.RemainingBalance) {
// 				tx.Rollback()
// 				return nil, errors.New("amount less than or equal bill remaining amount")
// 			}

// 			billCreditUseAmount = bill.BillTotalCreditUsedAmount.Add(item.Amount)
// 			remainingBalance = bill.RemainingBalance.Sub(item.Amount)
// 		}

// 		supplierCreditBill := SupplierCreditBill{
// 			BusinessId:           businessId,
// 			ReferenceId:          existingSupplierCredit.ID,
// 			ReferenceType:        SupplierCreditApplyTypeCredit,
// 			BranchId:             existingSupplierCredit.BranchId,
// 			SupplierId:           existingSupplierCredit.SupplierId,
// 			BillId:               item.BillId,
// 			BillNumber:           item.BillNumber,
// 			SupplierCreditNumber: existingSupplierCredit.SupplierCreditNumber,
// 			CreditDate:           existingSupplierCredit.SupplierCreditDate,
// 			Amount:               item.Amount,
// 			CurrencyId:           item.CurrencyId,
// 			ExchangeRate:         item.ExchangeRate,
// 		}

// 		err = tx.WithContext(ctx).Create(&supplierCreditBill).Error
// 		if err != nil {
// 			tx.Rollback()
// 			return nil, err
// 		}

// 		if remainingBalance.IsZero() {
// 			bill.CurrentStatus = BillStatusPaid
// 		} else {
// 			bill.CurrentStatus = BillStatusPartialPaid
// 		}
// 		bill.RemainingBalance = remainingBalance
// 		bill.BillTotalCreditUsedAmount = billCreditUseAmount
// 		// Save the updated bill's credit used amount
// 		if err := tx.WithContext(ctx).Save(&bill).Error; err != nil {
// 			tx.Rollback()
// 			return nil, err
// 		}

// 		supplierCreditTotalUsedAmount = supplierCreditTotalUsedAmount.Add(item.Amount)
// 		// Append the created SupplierCreditBill to the slice
// 		createdSupplierCreditBills = append(createdSupplierCreditBills, &supplierCreditBill)
// 	}

// 	totalUseAmount := existingSupplierCredit.SupplierCreditTotalUsedAmount.Add(supplierCreditTotalUsedAmount)

// 	remainingBalance := existingSupplierCredit.RemainingBalance.Sub(supplierCreditTotalUsedAmount)

// 	// if totalUseAmount.GreaterThan(existingSupplierCredit.SupplierCreditTotalAmount) {
// 	if supplierCreditTotalUsedAmount.GreaterThan(existingSupplierCredit.RemainingBalance) {
// 		tx.Rollback()
// 		return nil, errors.New("total amount must be less than or equal remaining balance of supplier credit")
// 	}

// 	if remainingBalance.IsZero() {
// 		existingSupplierCredit.CurrentStatus = SupplierCreditStatusClosed
// 	}
// 	existingSupplierCredit.SupplierCreditTotalUsedAmount = totalUseAmount
// 	existingSupplierCredit.RemainingBalance = remainingBalance
// 	// Save the updated bill's credit used amount
// 	if err := tx.WithContext(ctx).Save(&existingSupplierCredit).Error; err != nil {
// 		tx.Rollback()
// 		return nil, err
// 	}
// 	if err := tx.Commit().Error; err != nil {
// 		return nil, err
// 	}

// 	return createdSupplierCreditBills, nil
// }

// apply multiple credits to one bill
func CreateSupplierApplyCredit(ctx context.Context, input *NewBillApplyToSupplierCredit) ([]*SupplierCreditBill, error) {
	db := config.GetDB()

	business, err := GetBusiness(ctx)
	if err != nil {
		return nil, err
	}
	businessId := business.ID.String()

	bill, err := utils.FetchModelForChange[Bill](ctx, businessId, input.BillId)
	if err != nil {
		return nil, err
	}

	if bill.CurrentStatus != BillStatusConfirmed && bill.CurrentStatus != BillStatusPartialPaid {
		return nil, errors.New("bill status must be confirm or partial paid")
	}

	var billTotalCredit decimal.Decimal
	var billTotalAdvance decimal.Decimal
	var createdSupplierCreditBills []*SupplierCreditBill

	tx := db.Begin()

	for _, item := range input.ApplyCredits {

		if item.Amount.IsZero() {
			continue
		}
		var referenceId int
		var creditNumber string
		var date time.Time
		var currencyId int
		exchangeRate := item.ExchangeRate
		billCurrencyId := bill.CurrencyId
		billExchangeRate := bill.ExchangeRate

		// var baseCurrencyAmount decimal.Decimal
		// var creditTotalUsedAmount, remainingBalance, advanceTotalUsedAmount decimal.Decimal
		var adjustedAmount decimal.Decimal

		if item.ReferenceType == SupplierCreditApplyTypeCredit {
			supplierCredit, err := utils.FetchModelForChange[SupplierCredit](ctx, businessId, item.ReferenceId)
			if err != nil {
				tx.Rollback()
				return nil, err
			}

			if supplierCredit.BranchId != bill.BranchId {
				tx.Rollback()
				return nil, errors.New("cannot apply credit belonging to other branch")
			}

			// if supplierCredit.CurrentStatus != SupplierCreditStatusConfirmed {
			// 	tx.Rollback()
			// 	return nil, errors.New("supplierCredit status must be confirm")
			// }
			if err := supplierCredit.useAmount(item.Amount); err != nil {
				tx.Rollback()
				return nil, err
			}
			// Save the updated supplier credit's used amount
			if err := tx.WithContext(ctx).Save(&supplierCredit).Error; err != nil {
				tx.Rollback()
				return nil, err
			}

			// if item.CurrencyId != bill.CurrencyId {
			// 	baseCurrencyAmount = item.Amount.Mul(item.ExchangeRate)
			// 	if baseCurrencyAmount.GreaterThan(supplierCredit.RemainingBalance) {
			// 		tx.Rollback()
			// 		return nil, errors.New("amount less than or equal remaining credit")
			// 	}

			// 	creditTotalUsedAmount = supplierCredit.SupplierCreditTotalUsedAmount.Add(baseCurrencyAmount)
			// 	remainingBalance = supplierCredit.RemainingBalance.Sub(baseCurrencyAmount)

			// } else {
			// 	if item.Amount.GreaterThan(supplierCredit.RemainingBalance) {
			// 		tx.Rollback()
			// 		return nil, errors.New("amount less than or equal remaining credit amount")
			// 	}

			// 	creditTotalUsedAmount = supplierCredit.SupplierCreditTotalUsedAmount.Add(item.Amount)
			// 	remainingBalance = supplierCredit.RemainingBalance.Sub(item.Amount)

			// }

			referenceId = supplierCredit.ID
			creditNumber = supplierCredit.SupplierCreditNumber
			date = supplierCredit.SupplierCreditDate
			currencyId = supplierCredit.CurrencyId

			adjustedAmount = item.Amount
			if supplierCredit.CurrencyId != bill.CurrencyId {
				adjustedAmount = business.AdjustCurrency(item.CurrencyId, item.Amount, item.ExchangeRate)
				if bill.CurrencyId == business.BaseCurrencyId {
					exchangeRate = supplierCredit.ExchangeRate
					billExchangeRate = item.ExchangeRate
				}
			} else {
				exchangeRate = supplierCredit.ExchangeRate
			}
			billTotalCredit = billTotalCredit.Add(adjustedAmount)
		} else {
			supplierAdvance, err := utils.FetchModelForChange[SupplierCreditAdvance](ctx, businessId, item.ReferenceId)
			if err != nil {
				tx.Rollback()
				return nil, err
			}

			if err := supplierAdvance.useAmount(item.Amount); err != nil {
				tx.Rollback()
				return nil, err
			}

			// Save the updated supplier advance's used amount
			if err := tx.WithContext(ctx).Save(&supplierAdvance).Error; err != nil {
				tx.Rollback()
				return nil, err
			}

			referenceId = supplierAdvance.ID
			creditNumber = ""
			date = supplierAdvance.CreatedAt
			currencyId = supplierAdvance.CurrencyId

			adjustedAmount = item.Amount
			if supplierAdvance.CurrencyId != bill.CurrencyId {
				adjustedAmount = business.AdjustCurrency(item.CurrencyId, item.Amount, item.ExchangeRate)
				// adjustedAmount = item.Amount.Mul(item.ExchangeRate)
				if bill.CurrencyId == business.BaseCurrencyId {
					billExchangeRate = item.ExchangeRate
					exchangeRate = supplierAdvance.ExchangeRate
				}
			} else {
				exchangeRate = supplierAdvance.ExchangeRate
			}
			billTotalAdvance = billTotalAdvance.Add(adjustedAmount)

			// validating CreatedAt, which has been used for creating new SupplierCreditBill's CreditDate
			if err := validateTransactionLock(ctx, supplierAdvance.CreatedAt, businessId, PurchaseTransactionLock); err != nil {
				tx.Rollback()
				return nil, err
			}

		}

		supplierCreditBill := SupplierCreditBill{
			BusinessId:           businessId,
			ReferenceId:          referenceId,
			ReferenceType:        item.ReferenceType,
			BranchId:             bill.BranchId,
			SupplierId:           bill.SupplierId,
			BillId:               bill.ID,
			BillNumber:           bill.BillNumber,
			SupplierCreditNumber: creditNumber,
			CreditDate:           date,
			Amount:               item.Amount,
			CurrencyId:           currencyId,
			ExchangeRate:         exchangeRate,
			BillCurrencyId:       billCurrencyId,
			BillExchangeRate:     billExchangeRate,
		}

		err := tx.WithContext(ctx).Create(&supplierCreditBill).Error
		if err != nil {
			tx.Rollback()
			return nil, err
		}
		// Append the created SupplierCreditBill to the slice
		createdSupplierCreditBills = append(createdSupplierCreditBills, &supplierCreditBill)
	}

	var totalUseAmount decimal.Decimal

	totalCreditAmount := bill.BillTotalCreditUsedAmount.Add(billTotalCredit)
	totalAdvanceAmount := bill.BillTotalAdvanceUsedAmount.Add(billTotalAdvance)
	totalUseAmount = billTotalCredit.Add(billTotalAdvance)
	totalRemainingAmount := bill.RemainingBalance.Sub(totalUseAmount)

	if totalUseAmount.GreaterThan(bill.RemainingBalance) {
		tx.Rollback()
		return nil, errors.New("total amount less than or equal bill total amount")
	}
	if totalRemainingAmount.IsZero() {
		bill.CurrentStatus = BillStatusPaid
	} else {
		bill.CurrentStatus = BillStatusPartialPaid
	}

	bill.BillTotalCreditUsedAmount = totalCreditAmount
	bill.BillTotalAdvanceUsedAmount = totalAdvanceAmount
	bill.RemainingBalance = totalRemainingAmount
	// Save the updated bill's credit used amount
	if err := tx.WithContext(ctx).Save(&bill).Error; err != nil {
		tx.Rollback()
		return nil, err
	}

	if len(createdSupplierCreditBills) > 0 {
		for _, supplierCreditBill := range createdSupplierCreditBills {
			if supplierCreditBill.ReferenceType == SupplierCreditApplyTypeAdvance {
				err := PublishToAccounting(ctx, tx, businessId, supplierCreditBill.CreditDate, supplierCreditBill.ID, AccountReferenceTypeSupplierAdvanceApplied, supplierCreditBill, nil, PubSubMessageActionCreate)
				if err != nil {
					tx.Rollback()
					return nil, err
				}
			} else if supplierCreditBill.ReferenceType == SupplierCreditApplyTypeCredit &&
				!supplierCreditBill.ExchangeRate.Equals(supplierCreditBill.BillExchangeRate) {
				err := PublishToAccounting(ctx, tx, businessId, supplierCreditBill.CreditDate, supplierCreditBill.ID, AccountReferenceTypeSupplierCreditApplied, supplierCreditBill, nil, PubSubMessageActionCreate)
				if err != nil {
					tx.Rollback()
					return nil, err
				}
			}
		}
	}

	if err := tx.Commit().Error; err != nil {
		return nil, err
	}

	return createdSupplierCreditBills, nil
}

func DeleteSupplierCreditBill(ctx context.Context, id int) (*SupplierCreditBill, error) {
	business, err := GetBusiness(ctx)
	if err != nil {
		return nil, err
	}
	businessId := business.ID.String()

	db := config.GetDB()

	result, err := utils.FetchModelForChange[SupplierCreditBill](ctx, businessId, id)
	if err != nil {
		return nil, err
	}

	bill, err := utils.FetchModelForChange[Bill](ctx, businessId, result.BillId)
	if err != nil {
		return nil, err
	}

	tx := db.Begin()
	billAppliedAmount := result.Amount
	// if result.CurrencyId != bill.CurrencyId {
	// 	billAppliedAmount = business.AdjustCurrency(result.CurrencyId, result.Amount, result.ExchangeRate)
	// }
	if result.CurrencyId == business.BaseCurrencyId && result.BillCurrencyId != business.BaseCurrencyId {
		billAppliedAmount = result.Amount.DivRound(result.ExchangeRate, 4)
	} else if result.CurrencyId != business.BaseCurrencyId && result.BillCurrencyId == business.BaseCurrencyId {
		billAppliedAmount = result.Amount.Mul(result.BillExchangeRate)
	}

	if result.ReferenceType == SupplierCreditApplyTypeCredit {
		// update amount and status of supplier credit
		supplierCredit, err := utils.FetchModelForChange[SupplierCredit](ctx, businessId, result.ReferenceId)
		if err != nil {
			tx.Rollback()
			return nil, err
		}

		if err := supplierCredit.unUseAmount(result.Amount); err != nil {
			tx.Rollback()
			return nil, err
		}
		// Save the updated supplierCredit's credit used amount
		if err := tx.WithContext(ctx).Save(&supplierCredit).Error; err != nil {
			tx.Rollback()
			return nil, err
		}

		bill.BillTotalCreditUsedAmount = bill.BillTotalCreditUsedAmount.Sub(billAppliedAmount)

	} else {
		// update amount and status of supplier advance
		advance, err := utils.FetchModelForChange[SupplierCreditAdvance](ctx, businessId, result.ReferenceId)
		if err != nil {
			tx.Rollback()
			return nil, err
		}

		if err := advance.unUseAmount(result.Amount); err != nil {
			tx.Rollback()
			return nil, err
		}
		// Save the updated advance used amount
		if err := tx.WithContext(ctx).Save(&advance).Error; err != nil {
			tx.Rollback()
			return nil, err
		}

		bill.BillTotalAdvanceUsedAmount = bill.BillTotalAdvanceUsedAmount.Sub(billAppliedAmount)
	}

	// update amount and status of bill
	bill.RemainingBalance = bill.RemainingBalance.Add(billAppliedAmount)
	if bill.BillTotalAmount.GreaterThan(bill.RemainingBalance) {
		bill.CurrentStatus = BillStatusPartialPaid
	} else {
		bill.CurrentStatus = BillStatusConfirmed
	}
	// Save the updated bill's credit used amount
	if err := tx.WithContext(ctx).Save(&bill).Error; err != nil {
		tx.Rollback()
		return nil, err
	}

	err = tx.WithContext(ctx).Delete(&result).Error
	if err != nil {
		tx.Rollback()
		return nil, err
	}

	if result.ReferenceType == SupplierCreditApplyTypeAdvance {
		err = PublishToAccounting(ctx, tx, businessId, result.CreditDate, result.ID, AccountReferenceTypeSupplierAdvanceApplied, nil, result, PubSubMessageActionDelete)
		if err != nil {
			tx.Rollback()
			return nil, err
		}
	} else if result.ReferenceType == SupplierCreditApplyTypeCredit &&
		!result.ExchangeRate.Equals(result.BillExchangeRate) {
		err = PublishToAccounting(ctx, tx, businessId, result.CreditDate, result.ID, AccountReferenceTypeSupplierCreditApplied, nil, result, PubSubMessageActionDelete)
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
