package models

import (
	"fmt"
	"time"

	"github.com/mmdatafocus/books_backend/config"
	"github.com/mmdatafocus/books_backend/utils"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

type StockSummaryDailyBalance struct {
	BusinessId      string          `gorm:"primaryKey" json:"business_id"`
	WarehouseId     int             `gorm:"primaryKey" json:"warehouse_id"`
	ProductId       int             `gorm:"primaryKey" json:"product_id"`
	ProductType     ProductType     `gorm:"primaryKey;type:enum('S','G','C','V','I');default:'S'" json:"product_type"`
	BatchNumber     string          `gorm:"primaryKey;size:100" json:"batch_number"`
	TransactionDate time.Time       `gorm:"primaryKey" json:"transaction_date"`
	OpeningQty      decimal.Decimal `gorm:"type:decimal(20,4);default:0" json:"opening_qty"`
	OrderQty        decimal.Decimal `gorm:"type:decimal(20,4);default:0" json:"order_qty"`
	ReceivedQty     decimal.Decimal `gorm:"type:decimal(20,4);default:0" json:"received_qty"`
	SaleQty         decimal.Decimal `gorm:"type:decimal(20,4);default:0" json:"sale_qty"`
	CommittedQty    decimal.Decimal `gorm:"type:decimal(20,4);default:0" json:"committed_qty"`
	TransferQtyIn   decimal.Decimal `gorm:"type:decimal(20,4);default:0" json:"transfer_qty_in"`
	TransferQtyOut  decimal.Decimal `gorm:"type:decimal(20,4);default:0" json:"transfer_qty_out"`
	AdjustedQtyIn   decimal.Decimal `gorm:"type:decimal(20,4);default:0" json:"adjusted_qty_in"`
	AdjustedQtyOut  decimal.Decimal `gorm:"type:decimal(20,4);default:0" json:"adjusted_qty_out"`
	CurrentQty      decimal.Decimal `gorm:"type:decimal(20,4);default:0" json:"current_qty"`
	CreatedAt       time.Time       `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt       time.Time       `gorm:"autoUpdateTime" json:"updated_at"`
}

func UpsertStockSummaryDailyBalance(tx *gorm.DB, businessId string, warehouseId int, productId int, productType string, batchNumber string, quantity decimal.Decimal, date time.Time, fieldType string) error {
	// No-batch mode: ignore any provided batch number.
	if config.NoBatchMode() {
		batchNumber = ""
	}
	// dateOnly := date.Truncate(24 * time.Hour)
	// dateOnly := time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, date.Location())
	business, err := GetBusinessById2(tx, businessId)
	if err != nil {
		return err
	}
	dateOnly, err := utils.ConvertToDate(date, business.Timezone)
	if err != nil {
		return err
	}

	// Prepare insert fields and update values based on field type
	var insertFields, insertPlaceholders, updateValues string
	var args []interface{}

	switch fieldType {
	case "opening_qty":
		insertFields = "opening_qty, current_qty"
		insertPlaceholders = "?, ?"
		updateValues = "opening_qty = opening_qty + VALUES(opening_qty), current_qty = current_qty + VALUES(opening_qty)"
		args = append(args, businessId, warehouseId, productId, productType, batchNumber, dateOnly, quantity, quantity)
	case "order_qty":
		insertFields = "order_qty"
		insertPlaceholders = "?"
		updateValues = "order_qty = order_qty + VALUES(order_qty)"
		args = append(args, businessId, warehouseId, productId, productType, batchNumber, dateOnly, quantity)
	case "received_qty":
		insertFields = "received_qty, current_qty"
		insertPlaceholders = "?, ?"
		updateValues = "received_qty = received_qty + VALUES(received_qty), current_qty = current_qty + VALUES(received_qty)"
		args = append(args, businessId, warehouseId, productId, productType, batchNumber, dateOnly, quantity, quantity)
	case "committed_qty":
		insertFields = "committed_qty"
		insertPlaceholders = "?"
		updateValues = "committed_qty = committed_qty + VALUES(committed_qty)"
		args = append(args, businessId, warehouseId, productId, productType, batchNumber, dateOnly, quantity)
	case "sale_qty":
		insertFields = "sale_qty, current_qty"
		insertPlaceholders = "?, ?"
		updateValues = "sale_qty = sale_qty + VALUES(sale_qty), current_qty = current_qty - VALUES(sale_qty)"
		args = append(args, businessId, warehouseId, productId, productType, batchNumber, dateOnly, quantity, quantity.Neg())
	case "adjusted_qty_in":
		insertFields = "adjusted_qty_in, current_qty"
		insertPlaceholders = "?, ?"
		updateValues = "adjusted_qty_in = adjusted_qty_in + VALUES(adjusted_qty_in), current_qty = current_qty + VALUES(adjusted_qty_in)"
		args = append(args, businessId, warehouseId, productId, productType, batchNumber, dateOnly, quantity, quantity)
	case "adjusted_qty_out":
		insertFields = "adjusted_qty_out, current_qty"
		insertPlaceholders = "?, ?"
		updateValues = "adjusted_qty_out = adjusted_qty_out + VALUES(adjusted_qty_out), current_qty = current_qty - VALUES(adjusted_qty_out)"
		args = append(args, businessId, warehouseId, productId, productType, batchNumber, dateOnly, quantity.Neg(), quantity)
	case "transfer_qty_in":
		insertFields = "transfer_qty_in, current_qty"
		insertPlaceholders = "?, ?"
		updateValues = "transfer_qty_in = transfer_qty_in + VALUES(transfer_qty_in), current_qty = current_qty + VALUES(transfer_qty_in)"
		args = append(args, businessId, warehouseId, productId, productType, batchNumber, dateOnly, quantity, quantity)
	case "transfer_qty_out":
		insertFields = "transfer_qty_out, current_qty"
		insertPlaceholders = "?, ?"
		updateValues = "transfer_qty_out = transfer_qty_out + VALUES(transfer_qty_out), current_qty = current_qty + VALUES(transfer_qty_out)"
		args = append(args, businessId, warehouseId, productId, productType, batchNumber, dateOnly, quantity.Neg(), quantity)
	default:
		return fmt.Errorf("invalid field type: %s", fieldType)
	}

	// Insert or update the stock summary daily balance
	err = tx.Exec(fmt.Sprintf(`
        INSERT INTO stock_summary_daily_balances (business_id, warehouse_id, product_id, product_type, batch_number, transaction_date, %s)
        VALUES (?, ?, ?, ?, ?, ?, %s)
        ON DUPLICATE KEY UPDATE %s
    `, insertFields, insertPlaceholders, updateValues), args...).Error

	if err != nil {
		return err
	}

	return nil
}
