package models

import (
	"context"
	"errors"
	"time"

	"github.com/mmdatafocus/books_backend/config"
	"github.com/mmdatafocus/books_backend/utils"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type StockSummary struct {
	ID             int             `gorm:"primary_key" json:"id"`
	BusinessId     string          `gorm:"primary_key;index;not null" json:"business_id"`
	WarehouseId    int             `gorm:"primary_key;index;not null" json:"warehouse_id"`
	ProductId      int             `gorm:"primary_key;index;not null" json:"product_id"`
	ProductType    ProductType     `gorm:"type:enum('S','G','C','V','I');default:S" json:"product_type"`
	BatchNumber    string          `gorm:"primary_key;size:100" json:"batch_number"`
	OpeningQty     decimal.Decimal `gorm:"type:decimal(20,4);default:0" json:"opening_qty"`
	OrderQty       decimal.Decimal `gorm:"type:decimal(20,4);default:0" json:"order_qty"`
	ReceivedQty    decimal.Decimal `gorm:"type:decimal(20,4);default:0" json:"received_qty"`
	SaleQty        decimal.Decimal `gorm:"type:decimal(20,4);default:0" json:"sale_qty"`
	CommittedQty   decimal.Decimal `gorm:"type:decimal(20,4);default:0" json:"committed_qty"`
	TransferQtyIn  decimal.Decimal `gorm:"type:decimal(20,4);default:0" json:"transfer_qty_in"`
	TransferQtyOut decimal.Decimal `gorm:"type:decimal(20,4);default:0" json:"transfer_qty_out"`
	AdjustedQtyIn  decimal.Decimal `gorm:"type:decimal(20,4);default:0" json:"adjusted_qty_in"`
	AdjustedQtyOut decimal.Decimal `gorm:"type:decimal(20,4);default:0" json:"adjusted_qty_out"`
	CurrentQty     decimal.Decimal `gorm:"type:decimal(20,4);default:0" json:"current_qty"`
	CreatedAt      time.Time       `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt      time.Time       `gorm:"autoUpdateTime" json:"updated_at"`
}

func FirstOrCreateStockSummary(tx *gorm.DB, businessId string, warehouseId int, productId int, productType string, batchNumber string) (*StockSummary, bool, error) {
	// No-batch mode: ignore any provided batch number.
	if config.NoBatchMode() {
		batchNumber = ""
	}
	isNew := false
	stockSummary := StockSummary{
		BusinessId:  businessId,
		ProductId:   productId,
		ProductType: ProductType(productType),
		BatchNumber: batchNumber,
		WarehouseId: warehouseId,
	}
	// FirstOrCreate will try to find a record matching the conditions, and if it doesn't find one, it will create a new record
	result := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("business_id = ? AND product_id = ? AND product_type = ? AND warehouse_id = ? AND batch_number = ?",
		businessId, productId, productType, warehouseId, batchNumber).
		FirstOrCreate(&stockSummary)
	if result.Error != nil {
		tx.Rollback()
		return nil, isNew, result.Error
	}
	if result.RowsAffected == 1 {
		// if created , let do integration
		ProcessStockIntegration(tx, businessId, productType, productId)
		isNew = true
	}

	return &stockSummary, isNew, nil
}

func BulkLockStockSummary(tx *gorm.DB, businessId string, warehouseId int, fieldValues *utils.DetailFieldValues) error {
	var stockSummary []StockSummary
	q := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("business_id = ? AND product_id IN (?) AND warehouse_id = ? AND product_type IN (?)",
			businessId, fieldValues.ProductIDs, warehouseId, fieldValues.ProductTypes)
	// No-batch mode: do not lock per-batch rows.
	if !config.NoBatchMode() {
		q = q.Where("batch_number IN (?)", fieldValues.BatchNumbers)
	}
	if err := q.Find(&stockSummary).Error; err != nil {
		return err
	}
	return nil
}

func UpdateStockSummaryOrderQty(tx *gorm.DB, businessId string, warehouseId int, productId int, productType string, batchNumber string, quantity decimal.Decimal, date time.Time) error {
	if config.NoBatchMode() {
		batchNumber = ""
	}
	if productId > 0 {
		stockSummary, _, err := FirstOrCreateStockSummary(tx, businessId, warehouseId, productId, productType, batchNumber)
		if err != nil {
			tx.Rollback()
			return err
		}

		if err := tx.Exec("UPDATE stock_summaries SET order_qty = order_qty + ? WHERE id = ? ", quantity, stockSummary.ID).Error; err != nil {
			tx.Rollback()
			return err
		}

		if err := UpsertStockSummaryDailyBalance(tx, businessId, warehouseId, productId, productType, batchNumber, quantity, date, "order_qty"); err != nil {
			tx.Rollback()
			return err
		}
		// UpdateStockSummaryDailyBalanceOrderQty(tx, businessId, warehouseId, productId, productType, batchNumber, quantity, date)
	}

	return nil
}

// UpdateStockSummaryOpeningQty applies migration/opening stock quantities.
// This keeps stock_summaries and stock_summary_daily_balances consistent with stock_histories opening stock postings.
func UpdateStockSummaryOpeningQty(tx *gorm.DB, businessId string, warehouseId int, productId int, productType string, batchNumber string, quantity decimal.Decimal, date time.Time) error {
	if config.NoBatchMode() {
		batchNumber = ""
	}
	if productId > 0 {
		stockSummary, isNew, err := FirstOrCreateStockSummary(tx, businessId, warehouseId, productId, productType, batchNumber)
		if err != nil {
			tx.Rollback()
			return err
		}

		if err := tx.Exec("UPDATE stock_summaries SET opening_qty = opening_qty + ?, current_qty = current_qty + ? WHERE id = ? ", quantity, quantity, stockSummary.ID).Error; err != nil {
			tx.Rollback()
			return err
		}

		if err := UpsertStockSummaryDailyBalance(tx, businessId, warehouseId, productId, productType, batchNumber, quantity, date, "opening_qty"); err != nil {
			tx.Rollback()
			return err
		}
		// For new rows, integrations are already triggered in FirstOrCreateStockSummary.
		if !isNew {
			ProcessStockIntegration(tx, businessId, productType, productId)
		}
	}
	return nil
}

func UpdateStockSummaryReceivedQty(tx *gorm.DB, businessId string, warehouseId int, productId int, productType string, batchNumber string, quantity decimal.Decimal, date time.Time) error {
	if config.NoBatchMode() {
		batchNumber = ""
	}
	if productId > 0 {
		stockSummary, isNew, err := FirstOrCreateStockSummary(tx, businessId, warehouseId, productId, productType, batchNumber)
		if err != nil {
			tx.Rollback()
			return err
		}
		if err := tx.Exec("UPDATE stock_summaries SET received_qty = received_qty + ?, current_qty = current_qty + ? WHERE id = ? ", quantity, quantity, stockSummary.ID).Error; err != nil {
			tx.Rollback()
			return err
		}

		if err := UpsertStockSummaryDailyBalance(tx, businessId, warehouseId, productId, productType, batchNumber, quantity, date, "received_qty"); err != nil {
			tx.Rollback()
			return err
		}
		if !isNew {
			ProcessStockIntegration(tx, businessId, productType, productId)
		}
	}

	return nil
}

func UpdateStockSummaryCommittedQty(tx *gorm.DB, businessId string, warehouseId int, productId int, productType string, batchNumber string, quantity decimal.Decimal, date time.Time) error {
	if config.NoBatchMode() {
		batchNumber = ""
	}
	if productId > 0 {
		stockSummary, _, err := FirstOrCreateStockSummary(tx, businessId, warehouseId, productId, productType, batchNumber)
		if err != nil {
			tx.Rollback()
			return err
		}

		if err := tx.Exec("UPDATE stock_summaries SET committed_qty = committed_qty + ? WHERE id = ? ", quantity, stockSummary.ID).Error; err != nil {
			tx.Rollback()
			return err
		}

		if err := UpsertStockSummaryDailyBalance(tx, businessId, warehouseId, productId, productType, batchNumber, quantity, date, "committed_qty"); err != nil {
			tx.Rollback()
			return err
		}
	}

	return nil
}

func UpdateStockSummarySaleQty(tx *gorm.DB, businessId string, warehouseId int, productId int, productType string, batchNumber string, quantity decimal.Decimal, date time.Time) error {
	if config.NoBatchMode() {
		batchNumber = ""
	}
	if productId > 0 {
		stockSummary, _, err := FirstOrCreateStockSummary(tx, businessId, warehouseId, productId, productType, batchNumber)
		if err != nil {
			tx.Rollback()
			return err
		}

		if err := tx.Exec("UPDATE stock_summaries SET sale_qty = sale_qty + ?, current_qty = current_qty - ? WHERE id = ? ", quantity, quantity, stockSummary.ID).Error; err != nil {
			tx.Rollback()
			return err
		}

		if err := UpsertStockSummaryDailyBalance(tx, businessId, warehouseId, productId, productType, batchNumber, quantity, date, "sale_qty"); err != nil {
			tx.Rollback()
			return err
		}
		ProcessStockIntegration(tx, businessId, productType, productId)
	}

	return nil
}

func UpdateStockSummaryAdjustedQtyIn(tx *gorm.DB, businessId string, warehouseId int, productId int, productType string, batchNumber string, quantity decimal.Decimal, date time.Time) error {
	if config.NoBatchMode() {
		batchNumber = ""
	}
	if productId > 0 {
		stockSummary, _, err := FirstOrCreateStockSummary(tx, businessId, warehouseId, productId, productType, batchNumber)
		if err != nil {
			tx.Rollback()
			return err
		}

		if err := tx.Exec("UPDATE stock_summaries SET adjusted_qty_in = adjusted_qty_in + ?, current_qty = current_qty + ? WHERE id = ? ", quantity, quantity, stockSummary.ID).Error; err != nil {
			tx.Rollback()
			return err
		}

		if err := UpsertStockSummaryDailyBalance(tx, businessId, warehouseId, productId, productType, batchNumber, quantity, date, "adjusted_qty_in"); err != nil {
			tx.Rollback()
			return err
		}
		ProcessStockIntegration(tx, businessId, productType, productId)
	}

	return nil
}

func UpdateStockSummaryAdjustedQtyOut(tx *gorm.DB, businessId string, warehouseId int, productId int, productType string, batchNumber string, quantity decimal.Decimal, date time.Time) error {
	if config.NoBatchMode() {
		batchNumber = ""
	}
	if productId > 0 {
		stockSummary, _, err := FirstOrCreateStockSummary(tx, businessId, warehouseId, productId, productType, batchNumber)
		if err != nil {
			tx.Rollback()
			return err
		}

		if err := tx.Exec("UPDATE stock_summaries SET adjusted_qty_out = adjusted_qty_out + ?, current_qty = current_qty + ? WHERE id = ? ", quantity.Neg(), quantity, stockSummary.ID).Error; err != nil {
			tx.Rollback()
			return err
		}

		if err := UpsertStockSummaryDailyBalance(tx, businessId, warehouseId, productId, productType, batchNumber, quantity, date, "adjusted_qty_out"); err != nil {
			tx.Rollback()
			return err
		}
		ProcessStockIntegration(tx, businessId, productType, productId)
	}

	return nil
}

func UpdateStockSummaryTransferQtyIn(tx *gorm.DB, businessId string, warehouseId int, productId int, productType string, batchNumber string, quantity decimal.Decimal, date time.Time) error {
	if config.NoBatchMode() {
		batchNumber = ""
	}
	if productId > 0 {
		stockSummary, _, err := FirstOrCreateStockSummary(tx, businessId, warehouseId, productId, productType, batchNumber)
		if err != nil {
			tx.Rollback()
			return err
		}

		if err := tx.Exec("UPDATE stock_summaries SET transfer_qty_in = transfer_qty_in + ?, current_qty = current_qty + ? WHERE id = ? ", quantity, quantity, stockSummary.ID).Error; err != nil {
			tx.Rollback()
			return err
		}

		if err := UpsertStockSummaryDailyBalance(tx, businessId, warehouseId, productId, productType, batchNumber, quantity, date, "transfer_qty_in"); err != nil {
			tx.Rollback()
			return err
		}
		ProcessStockIntegration(tx, businessId, productType, productId)
	}

	return nil
}

func UpdateStockSummaryTransferQtyOut(tx *gorm.DB, businessId string, warehouseId int, productId int, productType string, batchNumber string, quantity decimal.Decimal, date time.Time) error {
	if config.NoBatchMode() {
		batchNumber = ""
	}
	if productId > 0 {
		stockSummary, _, err := FirstOrCreateStockSummary(tx, businessId, warehouseId, productId, productType, batchNumber)
		if err != nil {
			tx.Rollback()
			return err
		}

		if err := tx.Exec("UPDATE stock_summaries SET transfer_qty_out = transfer_qty_out + ?, current_qty = current_qty + ? WHERE id = ? ", quantity.Neg(), quantity, stockSummary.ID).Error; err != nil {
			tx.Rollback()
			return err
		}

		if err := UpsertStockSummaryDailyBalance(tx, businessId, warehouseId, productId, productType, batchNumber, quantity, date, "transfer_qty_out"); err != nil {
			tx.Rollback()
			return err
		}
		ProcessStockIntegration(tx, businessId, productType, productId)
	}

	return nil
}

func GetAvailableStocks(ctx context.Context, warehouseId int, asOf *MyDateString) ([]*StockSummary, error) {

	businessId, ok := utils.GetBusinessIdFromContext(ctx)
	if !ok || businessId == "" {
		return nil, errors.New("business id is required")
	}
	// check if warehouse exists and belong to the business
	if err := utils.ValidateResourceId[Warehouse](ctx, businessId, warehouseId); err != nil {
		return nil, errors.New("warehouse not found")
	}

	// Canonical: compute from ledger-of-record (stock_histories) to avoid stale caches.
	// Important: we must respect stock_date to avoid future-dated transactions
	// reducing "stock on hand" in operational screens like Transfer Orders.
	snapshotDate := MyDateString(time.Now().In(time.UTC))
	if biz, err := GetBusiness(ctx); err == nil {
		if biz.Timezone != "" {
			if location, locErr := time.LoadLocation(biz.Timezone); locErr == nil {
				snapshotDate = MyDateString(time.Now().In(location))
			}
		}
	}
	if asOf != nil {
		snapshotDate = *asOf
	}
	rows, err := InventorySnapshotByProductWarehouse(ctx, snapshotDate, &warehouseId, nil, nil, nil)
	if err != nil {
		return nil, err
	}

	out := make([]*StockSummary, 0, len(rows))
	for _, r := range rows {
		if r.StockOnHand.IsZero() {
			continue
		}
		pt := r.ProductType
		if pt == "" {
			pt = ProductTypeSingle
		}
		out = append(out, &StockSummary{
			BusinessId:  businessId,
			WarehouseId: warehouseId,
			ProductId:   r.ProductId,
			ProductType: pt,
			BatchNumber: "",
			CurrentQty:  r.StockOnHand,
		})
	}
	return out, nil
}

func GetStockInHand(ctx context.Context, productId int, productType string) (decimal.Decimal, error) {

	businessId, ok := utils.GetBusinessIdFromContext(ctx)
	if !ok || businessId == "" {
		return decimal.Zero, errors.New("business id is required")
	}

	// Canonical: compute from the ledger-of-record (stock_histories) to avoid stale caches.
	today := MyDateString(time.Now().In(time.UTC))
	if biz, err := GetBusiness(ctx); err == nil {
		if biz.Timezone != "" {
			if location, locErr := time.LoadLocation(biz.Timezone); locErr == nil {
				today = MyDateString(time.Now().In(location))
			}
		}
	}
	var pType *ProductType
	switch productType {
	case string(ProductTypeGroup):
		// For product groups, aggregate variants owned by the group.
		variantIds, err := getVariantIdsByGroup(ctx, businessId, productId)
		if err != nil {
			return decimal.Zero, err
		}
		total := decimal.Zero
		for _, vid := range variantIds {
			vType := ProductTypeVariant
			pid := vid
			rows, err := InventorySnapshotByProduct(ctx, today, &pid, &vType)
			if err != nil {
				return decimal.Zero, err
			}
			for _, r := range rows {
				total = total.Add(r.StockOnHand)
			}
		}
		return total, nil
	default:
		pt := ProductType(productType)
		pType = &pt
	}

	pid := productId
	rows, err := InventorySnapshotByProduct(ctx, today, &pid, pType)
	if err != nil {
		return decimal.Zero, err
	}
	if len(rows) == 0 {
		return decimal.Zero, nil
	}
	total := decimal.Zero
	for _, r := range rows {
		total = total.Add(r.StockOnHand)
	}
	return total, nil
}

func ProcessStockIntegration(tx *gorm.DB, businessId, productType string, productId int) error {
	if productType == "S" {
		ctx := tx.Statement.Context
		biz, _ := GetBusinessById(ctx, businessId)
		biz.ProcessProductIntegrationWorkflow(tx, productId)
	}
	return nil
}

// getVariantIdsByGroup lists variant ids for a product group within a business.
func getVariantIdsByGroup(ctx context.Context, businessId string, groupId int) ([]int, error) {
	db := config.GetDB()
	if db == nil {
		return nil, errors.New("database not initialized")
	}
	var ids []int
	if err := db.WithContext(ctx).
		Model(&ProductVariant{}).
		Where("business_id = ? AND product_group_id = ?", businessId, groupId).
		Pluck("id", &ids).Error; err != nil {
		return nil, err
	}
	return ids, nil
}
