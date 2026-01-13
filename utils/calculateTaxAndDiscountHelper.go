package utils

import (
	"context"

	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

type Tax struct {
	ID   int
	Rate decimal.Decimal
}

func CalculateTaxAmount(ctx context.Context, db *gorm.DB, taxID int, isTaxGroup bool, totalAmount decimal.Decimal, isTaxInclusive bool) decimal.Decimal {
	var tax Tax
	if isTaxGroup {
		err := db.WithContext(ctx).Table("tax_groups").First(&tax, taxID).Error
		if err != nil {
			// Handle error, return 0 or log the error
			return decimal.NewFromFloat(0)
		}
	} else {
		err := db.WithContext(ctx).Table("taxes").First(&tax, taxID).Error
		if err != nil {
			// Handle error, return 0 or log the error
			return decimal.NewFromFloat(0)
		}
	}

	taxRate := tax.Rate

	var taxAmount decimal.Decimal
	if isTaxInclusive {
		// Tax-inclusive: (totalAmount / (100 + taxRate)) * taxRate
		taxAmount = totalAmount.DivRound(taxRate.Add(decimal.NewFromFloat(100)), 4).Mul(taxRate)
	} else {
		// Tax-exclusive: (totalAmount / 100) * taxRate
		taxAmount = totalAmount.DivRound(decimal.NewFromFloat(100), 4).Mul(taxRate)
	}

	return taxAmount
}

func CalculateDiscountAmount(subTotal decimal.Decimal, discount decimal.Decimal, discountType string) decimal.Decimal {

	var discountAmount decimal.Decimal

	decimalOneHundred := decimal.NewFromFloat(100)

	if discount.GreaterThan(decimal.NewFromFloat(0.0)) {
		if discountType == "P" {
			discountAmount = subTotal.Mul(discount).DivRound(decimalOneHundred, 4)
		} else {
			discountAmount = discount
		}
	} else {
		discountAmount = decimal.NewFromFloat(0.0)
	}

	return discountAmount
}

// func (item *PurchaseOrderDetail) CalculateItemDiscountAndTax(ctx context.Context, isTaxInclusive bool) {

// 	db := config.GetDB()

// 	// calculate detail subtotal
// 	detailAmount := item.DetailQty.Mul(item.DetailUnitRate)
// 	// calculate discount amount
// 	var discountAmount decimal.Decimal

// 	if item.DetailDiscountType != nil {
// 		discountAmount = utils.CalculateDiscountAmount(detailAmount, item.DetailDiscount, string(*item.DetailDiscountType))
// 	}
// 	item.DetailDiscountAmount = discountAmount

//     // Calculate subtotal amount
//     item.DetailTotalAmount = item.DetailQty.Mul(item.DetailUnitRate).Sub(item.DetailDiscountAmount)

// 	var taxAmount decimal.Decimal
// 	if item.DetailTaxId > 0 {
// 		taxAmount = utils.CalculateTaxAmount(ctx, db, item.DetailTaxId, item.DetailTotalAmount, isTaxInclusive)
// 	} else {
// 		taxAmount = decimal.NewFromFloat(0)
// 	}

// 	item.DetailTaxAmount = taxAmount
// }

// func updateItemDetailTotal(item *PurchaseOrderDetail, isDetailTaxInclusive bool, orderSubtotal decimal.Decimal, totalExclusiveTaxAmount decimal.Decimal, totalDetailDiscountAmount decimal.Decimal, totalDetailTaxAmount decimal.Decimal) (decimal.Decimal, decimal.Decimal, decimal.Decimal, decimal.Decimal) {

// 	// var orderSubtotal, totalExclusiveTaxAmount, totalDetailDiscountAmount, totalDetailTaxAmount decimal.Decimal

// 	orderSubtotal = orderSubtotal.Add(item.DetailTotalAmount)
// 	totalDetailDiscountAmount = totalDetailDiscountAmount.Add(item.DetailDiscountAmount)
// 	totalDetailTaxAmount = totalDetailTaxAmount.Add(item.DetailTaxAmount)
// 	if isDetailTaxInclusive {
// 		totalExclusiveTaxAmount = totalExclusiveTaxAmount.Add(decimal.NewFromFloat(0.0))
// 	} else {
// 		totalExclusiveTaxAmount = totalExclusiveTaxAmount.Add(item.DetailTaxAmount)
// 	}

// 	return orderSubtotal, totalDetailDiscountAmount, totalDetailTaxAmount, totalExclusiveTaxAmount
// }
