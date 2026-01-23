package models

import (
	"time"

	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

type StockHistory struct {
	ID                int                `gorm:"primary_key" json:"id"`
	BusinessId        string             `gorm:"index;not null" json:"business_id"`
	WarehouseId       int                `gorm:"index;not null" json:"warehouse_id"`
	ProductId         int                `gorm:"index;not null" json:"product_id"`
	ProductType       ProductType        `gorm:"type:enum('S','G','C','V','I');default:S" json:"product_type"`
	BatchNumber       string             `gorm:"size:100" json:"batch_number"`
	StockDate         time.Time          `gorm:"not null" json:"stock_date"`
	Qty               decimal.Decimal    `gorm:"type:decimal(20,4);default:0" json:"qty"`
	ClosingQty        decimal.Decimal    `gorm:"type:decimal(20,4);default:0" json:"closing_qty"`
	Description       string             `gorm:"index;size:100;not null" json:"description"`
	BaseUnitValue     decimal.Decimal    `gorm:"type:decimal(20,4);default:0" json:"base_unit_value"`
	ClosingAssetValue decimal.Decimal    `gorm:"type:decimal(20,4);default:0" json:"closing_asset_value"`
	ReferenceType     StockReferenceType `gorm:"type:enum('IV','CN','BL','SC','IVAQ','IVAV','TO','POS','PGOS','PCOS')" json:"reference_type"`
	ReferenceID       int                `json:"reference_id"`
	ReferenceDetailID int                `json:"reference_detail_id"`
	IsOutgoing        *bool              `gorm:"not null;default:false" json:"is_outgoing"`
	IsTransferIn      *bool              `gorm:"not null;default:false" json:"is_transfer_in"`
	// Phase 1: inventory ledger immutability & reversals (append-only)
	IsReversal               bool            `gorm:"not null;default:false;index" json:"is_reversal"`
	ReversesStockHistoryId   *int            `gorm:"index" json:"reverses_stock_history_id"`
	ReversedByStockHistoryId *int            `gorm:"index" json:"reversed_by_stock_history_id"`
	ReversalReason           *string         `gorm:"type:text" json:"reversal_reason"`
	ReversedAt               *time.Time      `gorm:"index" json:"reversed_at"`
	CumulativeIncomingQty    decimal.Decimal `gorm:"type:decimal(20,4);default:0" json:"cumulative_incoming_qty"`
	CumulativeOutgoingQty    decimal.Decimal `gorm:"type:decimal(20,4);default:0" json:"cumulative_outgoing_qty"`
	CumulativeSequence       int             `gorm:"index;default:0" json:"cumulative_sequence"`
	CreatedAt                time.Time       `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt                time.Time       `gorm:"autoUpdateTime" json:"updated_at"`
}

// BeforeSave enforces internal invariants for the inventory ledger.
//
// CRITICAL: Many FIFO queries rely on IsOutgoing to classify consumptions.
// In production data, some historical rows (e.g. certain adjustments) may have qty<0 but IsOutgoing=false,
// which makes FIFO think stock exists (or doesn't) incorrectly and can lead to "insufficient FIFO layers".
//
// We ensure:
// - IsOutgoing is never nil
// - IsTransferIn is never nil
// - For non-zero qty, IsOutgoing always matches qty sign (qty < 0 => outgoing).
func (sh *StockHistory) BeforeSave(tx *gorm.DB) error {
	_ = tx // signature required by gorm; tx may be nil in tests
	if sh == nil {
		return nil
	}
	if sh.IsOutgoing == nil {
		b := false
		sh.IsOutgoing = &b
	}
	if sh.IsTransferIn == nil {
		b := false
		sh.IsTransferIn = &b
	}
	if sh.Qty.IsZero() {
		return nil
	}
	b := sh.Qty.IsNegative()
	sh.IsOutgoing = &b
	return nil
}

// type Stock struct {
// 	ID            int                `gorm:"primary_key" json:"id"`
// 	BusinessId    string             `gorm:"index;not null" json:"business_id"`
// 	WarehouseId   int                `gorm:"index;not null" json:"warehouse_id"`
// 	Description   string             `gorm:"index;size:100;not null" json:"description"`
// 	ReferenceType StockReferenceType `gorm:"type:enum('IV','CN','BL','SC','IVAQ','IVAV','OB','TO','POS','PGOS','PCOS')" json:"reference_type"`
// 	ReferenceID   int                `json:"reference_id"`
// 	ProductId     int                `gorm:"index;not null" json:"product_id"`
// 	ProductType   ProductType        `gorm:"type:enum('S','G','C','V','I');default:S" json:"product_type"`
// 	BatchNumber   string             `gorm:"size:100" json:"batch_number"`
// 	ReceivedDate  time.Time          `gorm:"not null" json:"received_date"`
// 	Qty           decimal.Decimal    `gorm:"type:decimal(20,4);default:0" json:"qty"`
// 	// CurrentQty       decimal.Decimal    `gorm:"type:decimal(20,4);default:0" json:"current_qty"`
// 	BaseUnitValue    decimal.Decimal `gorm:"type:decimal(20,4);default:0" json:"base_unit_value"`
// 	ForeignUnitValue decimal.Decimal `gorm:"type:decimal(20,4);default:0" json:"foreign_unit_value"`
// 	CurrencyId       int             `gorm:"index;not null" json:"currency_id"`
// 	ExchangeRate     decimal.Decimal `gorm:"type:decimal(20,4);default:0" json:"exchange_rate"`
// 	CreatedAt        time.Time       `gorm:"autoCreateTime" json:"created_at"`
// 	UpdatedAt        time.Time       `gorm:"autoUpdateTime" json:"updated_at"`
// }

// type NewStock struct {
// 	WarehouseId      int             `json:"warehouse_id"`
// 	Description      string          `json:"description"`
// 	ProductId        int             `json:"product_id"`
// 	ProductType      ProductType     `json:"product_type"`
// 	BatchNumber      string          `json:"batch_number"`
// 	ReceivedDate     time.Time       `json:"received_date"`
// 	Qty              decimal.Decimal `json:"qty"`
// 	BaseUnitValue    decimal.Decimal `json:"base_unit_value"`
// 	ForeignUnitValue decimal.Decimal `json:"foreign_unit_value"`
// 	CurrencyId       int             `json:"currency_id"`
// 	ExchangeRate     decimal.Decimal `json:"exchange_rate"`
// }

type ProductStock struct {
	WarehouseId  int             `json:"warehouse_id"`
	Description  string          `json:"description"`
	ProductId    int             `json:"product_id"`
	ProductType  ProductType     `json:"product_type"`
	BatchNumber  string          `json:"batch_number"`
	ReceivedDate time.Time       `json:"received_date"`
	Qty          decimal.Decimal `json:"qty"`
	CurrentQty   decimal.Decimal `json:"current_qty"`
}

// // validate input for both create & update. (id = 0 for create)

// func (input *NewStock) validate(ctx context.Context, businessId string, id int) error {

// 	// branch
// 	if err := utils.ValidateResourceId[Warehouse](ctx, businessId, input.WarehouseId); err != nil {
// 		return errors.New("warehouse not found")
// 	}
// 	// bill
// 	if err := utils.ValidateResourceId[Bill](ctx, businessId, input.BillId); err != nil {
// 		return errors.New("bill not found")
// 	}
// 	// currencyId
// 	if err := utils.ValidateResourceId[Currency](ctx, businessId, input.CurrencyId); err != nil {
// 		return errors.New("currency not found")
// 	}
// 	// contactId
// 	var err error
// 	if input.ProductId != 0 {
// 		if input.ProductType == ProductTypeSingle {
// 			err = utils.ValidateResourceId[Product](ctx, businessId, input.ProductId)
// 		} else if input.ProductType == ProductTypeGroup {
// 			err = utils.ValidateResourceId[Supplier](ctx, businessId, input.ProductId)
// 		}
// 		if err != nil {
// 			return errors.New("product not found")
// 		}
// 	}
// 	return nil
// }

// func CreateStock(ctx context.Context, input *NewStock) (*Stock, error) {

// 	businessId, ok := utils.GetBusinessIdFromContext(ctx)
// 	if !ok || businessId == "" {
// 		return nil, errors.New("business id is required")
// 	}

// 	if err := input.validate(ctx, businessId, 0); err != nil {
// 		return nil, err
// 	}

// 	stock := Stock{
// 		BusinessId:       businessId,
// 		WarehouseId:      input.WarehouseId,
// 		Description:      input.Description,
// 		BillId:           input.BillId,
// 		ProductId:        input.ProductId,
// 		ProductType:      input.ProductType,
// 		BatchNumber:      input.BatchNumber,
// 		ReceivedDate:     input.ReceivedDate,
// 		Qty:              input.Qty,
// 		CurrentQty:       input.Qty,
// 		BaseUnitValue:    input.BaseUnitValue,
// 		ForeignUnitValue: input.ForeignUnitValue,
// 		CurrencyId:       input.CurrencyId,
// 		ExchangeRate:     input.ExchangeRate,
// 	}

// 	db := config.GetDB()
// 	err := db.WithContext(ctx).Create(&stock).Error
// 	if err != nil {
// 		return nil, err
// 	}

// 	return &stock, nil
// }

// func UpdateStock(ctx context.Context, id int, input *NewStock) (*Stock, error) {

// 	businessId, ok := utils.GetBusinessIdFromContext(ctx)
// 	if !ok || businessId == "" {
// 		return nil, errors.New("business id is required")
// 	}
// 	if err := input.validate(ctx, businessId, id); err != nil {
// 		return nil, err
// 	}

// 	db := config.GetDB()
// 	stock, err := utils.FetchModel[Stock](ctx, businessId, id)
// 	if err != nil {
// 		return nil, err
// 	}

// 	// In the future, will modify the logic to check all similar stocks based on warehouse id, productid&type, batchnumber(if any)
// 	if input.Qty.Cmp(stock.CurrentQty) == -1 {
// 		return nil, errors.New("update qty cannot be less than current qty")
// 	}
// 	// db action
// 	tx := db.Begin()
// 	err = tx.WithContext(ctx).Model(&stock).Updates(map[string]interface{}{
// 		"WarehouseId":      input.WarehouseId,
// 		"Description":      input.Description,
// 		"BillId":           input.BillId,
// 		"ProductId":        input.ProductId,
// 		"ProductType":      input.ProductType,
// 		"BatchNumber":      input.BatchNumber,
// 		"ReceivedDate":     input.ReceivedDate,
// 		"Qty":              input.Qty,
// 		"CurrentQty":       input.Qty,
// 		"BaseUnitValue":    input.BaseUnitValue,
// 		"ForeignUnitValue": input.ForeignUnitValue,
// 		"CurrencyId":       input.CurrencyId,
// 		"ExchangeRate":     input.ExchangeRate,
// 	}).Error
// 	if err != nil {
// 		tx.Rollback()
// 		return nil, err
// 	}

// 	return stock, tx.Commit().Error
// }

// func DeleteStock(ctx context.Context, id int) (*Stock, error) {

// 	businessId, ok := utils.GetBusinessIdFromContext(ctx)
// 	if !ok || businessId == "" {
// 		return nil, errors.New("business id is required")
// 	}

// 	db := config.GetDB()
// 	stock, err := utils.FetchModel[Stock](ctx, businessId, id)
// 	if err != nil {
// 		return nil, err
// 	}

// 	// db action
// 	tx := db.Begin()
// 	if err := tx.Delete(&stock).Error; err != nil {
// 		tx.Rollback()
// 		return nil, err
// 	}
// 	tx.Commit()

// 	return stock, nil
// }

// func ReduceStockQty(tx *gorm.DB,ctx context.Context, businessId string, warehouseId int, item SalesInvoiceDetail) (error) {

// 		var product Product
//     	err := tx.Where("id = ? AND business_id = ?", item.ProductId, businessId).First(&product).Error
// 		if err != nil {
// 			tx.Rollback()
// 			return err
// 		}

// 		var stocks []Stock

// 		if  item.BatchNumber != "" && len(item.BatchNumber) > 0{
// 			err = tx.WithContext(ctx).Model(&Stock{}).
// 				Where("business_id = ? AND product_id = ? AND warehouse_id = ? AND current_qty > ? AND batch_number = ?",businessId, item.ProductId, warehouseId, 0, item.BatchNumber).
// 				Order("received_date").
// 				Find(&stocks).Error
// 			if err != nil {
// 				tx.Rollback()
// 				return err
// 			}
// 		}else{
// 			err = tx.WithContext(ctx).Model(&Stock{}).
// 				Where("business_id = ? AND product_id = ? AND warehouse_id = ? AND current_qty > ?",businessId, item.ProductId, warehouseId, 0).
// 				Order("received_date").
// 				Find(&stocks).Error
// 			if err != nil {
// 				tx.Rollback()
// 				return err
// 			}
// 		}

// 		count := len(stocks)
// 		 // If no stocks found, return an error
//         if count == 0 {
//             return fmt.Errorf("no stocks found for product %s", product.Name)
//         }

// 		qtyToUpdate := item.DetailQty
//         for i, stock := range stocks {

//             if stock.CurrentQty.Cmp(qtyToUpdate) >= 0 {
//                 // update the current stock row and exit the loop
//                 stock.CurrentQty = stock.CurrentQty.Sub(qtyToUpdate)
//                 err := tx.Save(&stock).Error
//                 if err != nil {
//                     tx.Rollback()
//                     return err
//                 }
//                 qtyToUpdate = decimal.NewFromFloat(0)
//             } else {
//                 // update the current stock row and continue to the next row
//                 qtyToUpdate = qtyToUpdate.Sub(stock.CurrentQty)
//                 stock.CurrentQty = decimal.NewFromFloat(0)
// 				if i + 1 == count && qtyToUpdate.Cmp(decimal.NewFromFloat(0)) > 0 {
// 					tx.Rollback()
// 					return fmt.Errorf("input quantity exceeds the total current quantity of all stocks")
// 				}
//                 err := tx.Save(&stock).Error
//                 if err != nil {
//                     tx.Rollback()
//                     return err
//                 }
//             }
// 		}

// 	return nil
// }

// func RefillStockQty(tx *gorm.DB,ctx context.Context, businessId string, item SalesInvoiceDetail) (error) {

// 	var itemStock Stock
// 	if  item.BatchNumber != "" && len(item.BatchNumber) > 0{
// 		if err := tx.WithContext(ctx).
// 			Where("business_id = ? AND product_id = ? AND batch_number = ?", businessId, item.ProductId, item.BatchNumber).
// 			First(&itemStock).Error; err != nil {
// 			tx.Rollback()
// 			return err
// 		}

// 		if err := tx.WithContext(ctx).Model(&itemStock).
// 			Updates(map[string]interface{}{
// 				"CurrentQty": itemStock.CurrentQty.Add(item.DetailQty),
// 			}).Error; err != nil {
// 				tx.Rollback()
// 				return err
// 			}
// 	}

// 	return nil
// }

// func UpdateStockQty(tx *gorm.DB,ctx context.Context, businessId string, item NewSalesInvoiceDetail, originalQty decimal.Decimal) (error) {

// 	var itemStock Stock
// 	if  item.BatchNumber != "" && len(item.BatchNumber) > 0{
// 		if err := tx.WithContext(ctx).
// 			Where("business_id = ? AND product_id = ? AND batch_number = ?", businessId, item.ProductId, item.BatchNumber).
// 			First(&itemStock).Error; err != nil {
// 			tx.Rollback()
// 			return err
// 		}

// 		var qty decimal.Decimal

// 		if item.DetailQty.Cmp(originalQty) > 0 {
// 			qty = itemStock.CurrentQty.Sub(item.DetailQty.Sub(originalQty))
// 		}else{
// 			qty = itemStock.CurrentQty.Add(originalQty.Sub(item.DetailQty))
// 		}

// 		if err := tx.WithContext(ctx).Model(&itemStock).
// 			Updates(map[string]interface{}{
// 				// "Qty": itemStock.CurrentQty.Add(item.DetailQty),
// 				"CurrentQty": qty,
// 			}).Error; err != nil {
// 				tx.Rollback()
// 				return err
// 			}
// 	}

// 	return nil
// }
