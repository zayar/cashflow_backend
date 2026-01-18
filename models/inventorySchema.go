package models

import (
	"fmt"
	"os"
	"strings"

	"github.com/mmdatafocus/books_backend/config"
)

// EnsureInventoryLedgerSchema enforces strict schema constraints for stock_histories.
// This is intended for clean-start environments where legacy NULLs are not expected.
func EnsureInventoryLedgerSchema() error {
	db := config.GetDB()
	if db == nil {
		return fmt.Errorf("database not initialized")
	}
	if strings.ToLower(strings.TrimSpace(os.Getenv("INVENTORY_STRICT_SCHEMA"))) == "false" {
		return nil
	}

	var badCount int64
	if err := db.Model(&StockHistory{}).
		Where("warehouse_id IS NULL OR warehouse_id = 0").
		Count(&badCount).Error; err != nil {
		return err
	}
	if badCount > 0 {
		return fmt.Errorf("stock_histories has %d rows with NULL/0 warehouse_id; clean start required before enforcing schema", badCount)
	}

	if err := db.Exec("ALTER TABLE stock_histories MODIFY warehouse_id INT NOT NULL").Error; err != nil {
		return err
	}

	var idxCount int64
	if err := db.Raw(`
		SELECT COUNT(1)
		FROM INFORMATION_SCHEMA.STATISTICS
		WHERE TABLE_SCHEMA = DATABASE()
		  AND TABLE_NAME = 'stock_histories'
		  AND INDEX_NAME = 'idx_stock_histories_ledger'
	`).Scan(&idxCount).Error; err != nil {
		return err
	}
	if idxCount == 0 {
		if err := db.Exec(`
			CREATE INDEX idx_stock_histories_ledger
			ON stock_histories (business_id, warehouse_id, product_id, product_type, batch_number, stock_date, cumulative_sequence, id)
		`).Error; err != nil {
			return err
		}
	}
	return nil
}
