package models

import (
	"context"
	"errors"
	"time"

	"github.com/mmdatafocus/books_backend/config"
	"github.com/mmdatafocus/books_backend/utils"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

type InventoryAdjustment struct {
	ID              int                         `gorm:"primary_key" json:"id"`
	BusinessId      string                      `gorm:"index;not null" json:"business_id" binding:"required"`
	ReferenceNumber string                      `gorm:"size:255;default:null" json:"reference_number"`
	AdjustmentType  InventoryAdjustmentType     `gorm:"type:enum('Quantity', 'Value');not null" json:"adjustment_type" binding:"required"`
	AdjustmentDate  time.Time                   `gorm:"not null" json:"adjustment_date" binding:"required"`
	AccountId       int                         `gorm:"not null" json:"account_id" binding:"required"`
	BranchId        int                         `gorm:"not null" json:"branch_id" binding:"required"`
	WarehouseId     int                         `gorm:"not null" json:"warehouse_id" binding:"required"`
	CurrentStatus   InventoryAdjustmentStatus   `gorm:"type:enum('Draft', 'Adjusted');not null" json:"current_status" binding:"required"`
	ReasonId        int                         `gorm:"not null" json:"reasonId"`
	Description     string                      `gorm:"type:text;default:null" json:"description"`
	Documents       []*Document                 `gorm:"polymorphic:Reference" json:"documents"`
	Details         []InventoryAdjustmentDetail `gorm:"foreignKey:InventoryAdjustmentId" json:"details"`
	CreatedBy       int                         `gorm:"not null" json:"created_by" binding:"required"`
	CreatedAt       time.Time                   `gorm:"autoCreateTime" json:"created_at"`
	UpdatedBy       int                         `json:"updated_by"`
	UpdatedAt       time.Time                   `gorm:"autoUpdateTime" json:"updated_at"`
}

type NewInventoryAdjustment struct {
	ReferenceNumber string                         `json:"reference_number"`
	AdjustmentType  InventoryAdjustmentType        `json:"adjustment_type" binding:"required"`
	AdjustmentDate  time.Time                      `json:"invoice_date" binding:"required"`
	AccountId       int                            `json:"account_id" binding:"required"`
	BranchId        int                            `json:"branch_id" binding:"required"`
	WarehouseId     int                            `json:"warehouse_id" binding:"required"`
	CurrentStatus   InventoryAdjustmentStatus      `json:"current_status" binding:"required"`
	ReasonId        int                            `json:"reason_id" binding:"required"`
	Description     string                         `json:"description"`
	Documents       []*NewDocument                 `json:"documents"`
	Details         []NewInventoryAdjustmentDetail `json:"details"`
}

type InventoryAdjustmentDetail struct {
	ID                    int                 `gorm:"primary_key" json:"id"`
	InventoryAdjustmentId int                 `gorm:"index;not null" json:"inventory_adjustment_id" binding:"required"`
	InventoryAdjustment   InventoryAdjustment `gorm:"foreignKey:InventoryAdjustmentId" json:"inventory_adjustment"`
	ProductId             int                 `gorm:"default:null" json:"product_id"`
	ProductType           ProductType         `gorm:"type:enum('S','G','C','V','I');default:S" json:"product_type"`
	BatchNumber           string              `gorm:"size:100;default:null" json:"batch_number"`
	Name                  string              `gorm:"size:100" json:"name" binding:"required"`
	Description           string              `gorm:"size:255;default:null" json:"description"`
	AdjustedValue         decimal.Decimal     `gorm:"type:decimal(20,4);default:0" json:"adjusted_value" binding:"required"`
	CostPrice             decimal.Decimal     `gorm:"type:decimal(20,4);default:0" json:"cost_price"`
	CreatedAt             time.Time           `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt             time.Time           `gorm:"autoUpdateTime" json:"updated_at"`
}

type NewInventoryAdjustmentDetail struct {
	DetailId      int             `json:"detail_id"`
	ProductId     int             `json:"product_id"`
	ProductType   ProductType     `json:"product_type"`
	BatchNumber   string          `json:"batch_number"`
	Name          string          `json:"name" binding:"required"`
	Description   string          `json:"description"`
	AdjustedValue decimal.Decimal `json:"adjusted_value" binding:"required"`
	CostPrice     decimal.Decimal `json:"cost_price" binding:"required"`
	IsDeletedItem *bool           `json:"is_deleted_item"`
}

type InventoryAdjustmentsConnection struct {
	Edges    []*InventoryAdjustmentsEdge `json:"edges"`
	PageInfo *PageInfo                   `json:"pageInfo"`
}

type InventoryAdjustmentsEdge Edge[InventoryAdjustment]

func (obj InventoryAdjustment) GetId() int {
	return obj.ID
}

// returns decoded curosr string
func (invAdj InventoryAdjustment) GetCursor() string {
	return invAdj.CreatedAt.String()
}

func (invAdj *InventoryAdjustment) GetFieldValues(tx *gorm.DB) (*utils.DetailFieldValues, error) {
	return utils.FetchDetailFieldValues(tx, &InventoryAdjustmentDetail{}, "inventory_adjustment_id", invAdj.ID)
}

func (input NewInventoryAdjustment) validate(ctx context.Context, businessId string, _ int) error {
	if err := utils.ValidateResourceId[Warehouse](ctx, businessId, input.WarehouseId); err != nil {
		return errors.New("warehouse not found")
	}
	if err := utils.ValidateResourceId[Account](ctx, businessId, input.AccountId); err != nil {
		return errors.New("account not found")
	}
	if err := utils.ValidateResourceId[Branch](ctx, businessId, input.BranchId); err != nil {
		return errors.New("branch not found")
	}
	if err := utils.ValidateResourceId[Reason](ctx, businessId, input.ReasonId); err != nil {
		return errors.New("reason not found")
	}
	// validate each product for inventory adjustment date
	business, err := GetBusinessById(ctx, businessId)
	if err != nil {
		return err
	}
	adjDate, err := utils.ConvertToDate(input.AdjustmentDate, business.Timezone)
	if err != nil {
		return err
	}
	for _, inputDetail := range input.Details {
		if err := ValidateValueAdjustment(ctx, businessId, input.AdjustmentDate, inputDetail.ProductType, inputDetail.ProductId, &inputDetail.BatchNumber, input.AdjustmentType == InventoryAdjustmentTypeValue); err != nil {
			return err
		}

		// Guardrails for VALUE adjustments:
		// - Prevent creating negative inventory value unless explicitly supported.
		// - Value adjustments operate on existing stock on hand; disallow when qty <= 0.
		if input.AdjustmentType == InventoryAdjustmentTypeValue && inputDetail.ProductId > 0 {
			db := config.GetDB()
			var last StockHistory
			if err := db.WithContext(ctx).
				Where("business_id = ? AND warehouse_id = ? AND product_id = ? AND product_type = ? AND batch_number = ? AND stock_date <= ?",
					businessId, input.WarehouseId, inputDetail.ProductId, inputDetail.ProductType, inputDetail.BatchNumber, adjDate).
				Order("stock_date DESC, cumulative_sequence DESC").
				Limit(1).
				Find(&last).Error; err != nil {
				return err
			}
			if last.ID <= 0 {
				// This is also checked later during status transition; keep this early for better UX.
				return errors.New("inventory valuation is not ready for this item yet (stock history missing). Please wait a moment and try again, or ensure opening stock posting has completed.")
			}
			if last.ClosingQty.LessThanOrEqual(decimal.Zero) {
				return errors.New("cannot adjust inventory value when stock on hand is zero")
			}
			finalValue := last.ClosingAssetValue.Add(inputDetail.AdjustedValue)
			if finalValue.IsNegative() {
				return errors.New("value adjustment would make inventory value negative")
			}
		}
	}

	return nil
}

func CreateInventoryAdjustment(ctx context.Context, input *NewInventoryAdjustment) (*InventoryAdjustment, error) {
	db := config.GetDB()

	businessId, ok := utils.GetBusinessIdFromContext(ctx)
	if !ok || businessId == "" {
		return nil, errors.New("business id is required")
	}
	userId, ok := utils.GetUserIdFromContext(ctx)
	if !ok || userId == 0 {
		return nil, errors.New("user id is required")
	}
	// validate InventoryAdjustment
	if err := input.validate(ctx, businessId, 0); err != nil {
		return nil, err
	}
	// construct Images
	documents, err := mapNewDocuments(input.Documents, "inventory_adjustments", 0)
	if err != nil {
		return nil, err
	}

	var adjustmentItems []InventoryAdjustmentDetail

	for _, item := range input.Details {
		if !IsRealProduct(ctx, businessId, item.ProductId, item.ProductType) {
			return nil, errors.New("product's inventory has not been tracked")
		}
		if item.AdjustedValue.IsZero() {
			return nil, errors.New("adjusted value cannot be zero")
		}

		adjustmentItem := InventoryAdjustmentDetail{
			ProductId:     item.ProductId,
			ProductType:   item.ProductType,
			BatchNumber:   item.BatchNumber,
			Name:          item.Name,
			Description:   item.Description,
			AdjustedValue: item.AdjustedValue,
			CostPrice:     item.CostPrice,
		}
		// Add the item to the InventoryAdjustment
		adjustmentItems = append(adjustmentItems, adjustmentItem)
	}

	// store InventoryAdjustment
	business, err := GetBusinessById(ctx, businessId)
	if err != nil {
		return nil, err
	}
	adjDate, err := utils.ConvertToDate(input.AdjustmentDate, business.Timezone)
	if err != nil {
		return nil, err
	}
	inventoryAdjustment := InventoryAdjustment{
		BusinessId:      businessId,
		AdjustmentType:  input.AdjustmentType,
		ReferenceNumber: input.ReferenceNumber,
		AdjustmentDate:  adjDate,
		ReasonId:        input.ReasonId,
		Description:     input.Description,
		AccountId:       input.AccountId,
		BranchId:        input.BranchId,
		WarehouseId:     input.WarehouseId,
		CurrentStatus:   InventoryAdjustmentStatusDraft,
		Documents:       documents,
		Details:         adjustmentItems,
		CreatedBy:       userId,
	}

	tx := db.Begin()

	err = tx.WithContext(ctx).Create(&inventoryAdjustment).Error
	if err != nil {
		tx.Rollback()
		return nil, err
	}

	// If requested "Adjusted", apply the status transition deterministically (Draft -> Adjusted).
	requestedStatus := input.CurrentStatus
	if requestedStatus == InventoryAdjustmentStatusAdjusted {
		if err := tx.WithContext(ctx).Model(&inventoryAdjustment).Update("CurrentStatus", InventoryAdjustmentStatusAdjusted).Error; err != nil {
			tx.Rollback()
			return nil, err
		}
		inventoryAdjustment.CurrentStatus = InventoryAdjustmentStatusAdjusted

		// Apply inventory side-effects deterministically for quantity adjustments.
		if config.UseStockCommandsFor("INVENTORY_ADJUSTMENT") {
			if err := ApplyInventoryAdjustmentStockForStatusTransition(tx.WithContext(ctx), &inventoryAdjustment, InventoryAdjustmentStatusDraft); err != nil {
				tx.Rollback()
				return nil, err
			}
		}

		// Guardrail for VALUE adjustments:
		// Value adjustments (IVAV) require existing stock history baseline to revalue inventory.
		// If stock_histories are not ready yet (e.g. opening stock not posted/worker lag),
		// the async workflow will fail and the UI will show no journal + no valuation changes.
		// Fail fast with a clear error instead of silently creating an un-postable adjustment.
		if inventoryAdjustment.AdjustmentType == InventoryAdjustmentTypeValue {
			stockDate, err := utils.ConvertToDate(inventoryAdjustment.AdjustmentDate, business.Timezone)
			if err != nil {
				tx.Rollback()
				return nil, err
			}
			for _, d := range inventoryAdjustment.Details {
				if d.ProductId <= 0 {
					continue
				}
				var exists int
				if err := tx.WithContext(ctx).Raw(`
SELECT 1
FROM stock_histories
WHERE business_id = ?
  AND warehouse_id = ?
  AND product_id = ?
  AND product_type = ?
  AND batch_number = ?
  AND stock_date <= ?
LIMIT 1
`, inventoryAdjustment.BusinessId, inventoryAdjustment.WarehouseId, d.ProductId, d.ProductType, d.BatchNumber, stockDate).Scan(&exists).Error; err != nil {
					tx.Rollback()
					return nil, err
				}
				if exists != 1 {
					tx.Rollback()
					return nil, errors.New("inventory valuation is not ready for this item yet (stock history missing). Please wait a moment and try again, or ensure opening stock posting has completed.")
				}
			}
		}

		// Write outbox record only when adjusted (posting should not happen for Draft).
		if inventoryAdjustment.AdjustmentType == InventoryAdjustmentTypeQuantity {
			if err := PublishToAccounting(ctx, tx, businessId, inventoryAdjustment.AdjustmentDate, inventoryAdjustment.ID, AccountReferenceTypeInventoryAdjustmentQuantity, inventoryAdjustment, nil, PubSubMessageActionCreate); err != nil {
				tx.Rollback()
				return nil, err
			}
		} else {
			if err := PublishToAccounting(ctx, tx, businessId, inventoryAdjustment.AdjustmentDate, inventoryAdjustment.ID, AccountReferenceTypeInventoryAdjustmentValue, inventoryAdjustment, nil, PubSubMessageActionCreate); err != nil {
				tx.Rollback()
				return nil, err
			}
		}
	} else {
		// Not adjusted yet: do not publish posting.
	}

	if err := tx.Commit().Error; err != nil {
		return nil, err
	}

	return &inventoryAdjustment, nil
}

// func UpdateInventoryAdjustment(ctx context.Context, id int, input *NewInventoryAdjustment) (*InventoryAdjustment, error) {
// 	db := config.GetDB()

// 	businessId, ok := utils.GetBusinessIdFromContext(ctx)
// 	if !ok || businessId == "" {
// 		return nil, errors.New("business id is required")
// 	}

// 	userId, ok := utils.GetUserIdFromContext(ctx)
// 	if !ok || userId == 0 {
// 		return nil, errors.New("user id is required")
// 	}

// 	if err := input.validate(ctx, businessId, id); err != nil {
// 		return nil, err
// 	}

// 	existingData, err := utils.FetchModel[InventoryAdjustment](ctx, businessId, id, "Details")
// 	if err != nil {
// 		return nil, err
// 	}

// 	if existingData.AdjustmentType == InventoryAdjustmentTypeValue {
// 		return nil, errors.New("cannot update because current inventory adjustment type is value")
// 	}

// 	oldStatus := existingData.CurrentStatus

// 	// Update the fields of the existing adjustment with the provided updated details
// 	existingData.ReferenceNumber = input.ReferenceNumber
// 	existingData.AdjustmentDate = input.AdjustmentDate
// 	existingData.ReasonId = input.ReasonId
// 	existingData.Description = input.Description
// 	existingData.AccountId = input.AccountId
// 	existingData.BranchId = input.BranchId
// 	existingData.WarehouseId = input.WarehouseId
// 	existingData.CurrentStatus = input.CurrentStatus
// 	existingData.UpdatedBy = userId

// 	tx := db.Begin()

// 	// Iterate through the updated items
// 	for _, updatedItem := range input.Details {
// 		var existingItem *InventoryAdjustmentDetail

// 		// Check if the item already exists in the adjustment
// 		for _, item := range existingData.Details {
// 			if item.ID == updatedItem.DetailId {
// 				existingItem = &item
// 				break
// 			}
// 		}

// 		// If the item doesn't exist, add it to the adjustment
// 		if existingItem == nil {
// 			newItem := InventoryAdjustmentDetail{
// 				InventoryAdjustmentId: id,
// 				ProductId:             updatedItem.ProductId,
// 				ProductType:           updatedItem.ProductType,
// 				BatchNumber:           updatedItem.BatchNumber,
// 				Name:                  updatedItem.Name,
// 				Description:           updatedItem.Description,
// 				AdjustedValue:         updatedItem.AdjustedValue,
// 				CostPrice:             updatedItem.CostPrice,
// 			}
// 			existingData.Details = append(existingData.Details, newItem)

// 		} else {
// 			if updatedItem.IsDeletedItem != nil && *updatedItem.IsDeletedItem {
// 				// Find the index of the item to delete
// 				for i, item := range existingData.Details {
// 					if item.ID == updatedItem.DetailId {
// 						// Delete the item from the database
// 						if err := tx.WithContext(ctx).Delete(&existingData.Details[i]).Error; err != nil {
// 							tx.Rollback()
// 							return nil, err
// 						}

// 						if item.ProductId > 0 && existingData.CurrentStatus == InventoryAdjustmentStatusAdjusted && existingData.AdjustmentType == InventoryAdjustmentTypeQuantity {
// 							product, err := GetProductOrVariant(ctx, string(item.ProductType), item.ProductId)
// 							if err != nil {
// 								tx.Rollback()
// 								return nil, err
// 							}

// 							if product.GetInventoryAccountID() > 0 {
// 								if err := UpdateStockSummaryAdjustedQty(tx, businessId, existingData.WarehouseId, item.ProductId, string(item.ProductType), item.BatchNumber, item.AdjustedValue.Neg(), existingData.AdjustmentDate); err != nil {
// 									tx.Rollback()
// 									return nil, err
// 								}
// 							}
// 						}
// 						// Remove the item from the slice
// 						existingData.Details = append(existingData.Details[:i], existingData.Details[i+1:]...)
// 						break // Exit the loop after deleting the item
// 					}
// 				}
// 			} else {
// 				// Update existing item details
// 				existingItem.ProductId = updatedItem.ProductId
// 				existingItem.ProductType = updatedItem.ProductType
// 				existingItem.BatchNumber = updatedItem.BatchNumber
// 				existingItem.Name = updatedItem.Name
// 				existingItem.Description = updatedItem.Description
// 				existingItem.AdjustedValue = updatedItem.AdjustedValue
// 				existingItem.CostPrice = updatedItem.CostPrice

// 				if err := tx.WithContext(ctx).Save(&existingItem).Error; err != nil {
// 					tx.Rollback()
// 					return nil, err
// 				}
// 			}
// 		}
// 	}

// 	// Save the updated adjustment to the database
// 	if err := tx.WithContext(ctx).Save(&existingData).Error; err != nil {
// 		tx.Rollback()
// 		return nil, err
// 	}

// 	// Refresh the existingBill to get the latest details
// 	if err := tx.WithContext(ctx).Preload("Details").First(&existingData, id).Error; err != nil {
// 		tx.Rollback()
// 		return nil, err
// 	}

// 	if existingData.AdjustmentType == InventoryAdjustmentTypeQuantity {
// 		if err := existingData.AfterUpdateCurrentStatus(tx.WithContext(ctx), string(oldStatus)); err != nil {
// 			tx.Rollback()
// 			return nil, err
// 		}
// 	}

// 	documents, err := upsertDocuments(ctx, tx, input.Documents, "inventory_adjustments", id)
// 	if err != nil {
// 		tx.Rollback()
// 		return nil, err
// 	}
// 	existingData.Documents = documents

// 	if err := tx.Commit().Error; err != nil {
// 		return nil, err
// 	}

// 	return existingData, nil
// }

// func ConvertInventoryAdjustment(ctx context.Context, id int) (*InventoryAdjustment, error) {
// 	businessId, ok := utils.GetBusinessIdFromContext(ctx)
// 	if !ok || businessId == "" {
// 		return nil, errors.New("business id is required")
// 	}

// 	result, err := utils.FetchModel[InventoryAdjustment](ctx, businessId, id)
// 	if err != nil {
// 		return nil, err
// 	}

// 	oldStatus := result.CurrentStatus

// 	// db action
// 	db := config.GetDB()
// 	tx := db.Begin()
// 	err = tx.WithContext(ctx).Model(&result).Updates(map[string]interface{}{
// 		"CurrentStatus": InventoryAdjustmentStatusAdjusted,
// 	}).Error

// 	if err != nil {
// 		tx.Rollback()
// 		return nil, err
// 	}

// 	if result.AdjustmentType == InventoryAdjustmentTypeQuantity {
// 		if err := result.AfterUpdateCurrentStatus(tx.WithContext(ctx), string(oldStatus)); err != nil {
// 			tx.Rollback()
// 			return nil, err
// 		}
// 	}

// 	if err := createHistory(tx.WithContext(ctx), "Update", id, "inventory_adjustments", nil, nil, "Updated CurrentStatus to Adjusted"); err != nil {
// 		tx.Rollback()
// 		return nil, err
// 	}

// 	return result, tx.Commit().Error
// }

func DeleteInventoryAdjustment(ctx context.Context, id int) (*InventoryAdjustment, error) {
	businessId, ok := utils.GetBusinessIdFromContext(ctx)
	if !ok || businessId == "" {
		return nil, errors.New("business id is required")
	}

	result, err := utils.FetchModel[InventoryAdjustment](ctx, businessId, id, "Details", "Documents")
	if err != nil {
		return nil, err
	}

	db := config.GetDB()
	tx := db.Begin()

	// reduced received qty from stock summary if bill is confirmed
	if result.CurrentStatus == InventoryAdjustmentStatusAdjusted && result.AdjustmentType == InventoryAdjustmentTypeQuantity {
		for _, detailItem := range result.Details {
			if detailItem.ProductId > 0 {
				product, err := GetProductOrVariant(ctx, string(detailItem.ProductType), detailItem.ProductId)
				if err != nil {
					tx.Rollback()
					return nil, err
				}
				if product.GetInventoryAccountID() > 0 {
					if detailItem.AdjustedValue.GreaterThan(decimal.NewFromFloat(0)) {
						if err := UpdateStockSummaryAdjustedQtyIn(tx, result.BusinessId, result.WarehouseId, detailItem.ProductId, string(detailItem.ProductType), detailItem.BatchNumber, detailItem.AdjustedValue.Neg(), result.AdjustmentDate); err != nil {
							tx.Rollback()
							return nil, err
						}
					} else {
						if err := UpdateStockSummaryAdjustedQtyOut(tx, result.BusinessId, result.WarehouseId, detailItem.ProductId, string(detailItem.ProductType), detailItem.BatchNumber, detailItem.AdjustedValue.Neg(), result.AdjustmentDate); err != nil {
							tx.Rollback()
							return nil, err
						}
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

	if result.AdjustmentType == InventoryAdjustmentTypeQuantity {
		err = PublishToAccounting(ctx, tx, businessId, result.AdjustmentDate, result.ID, AccountReferenceTypeInventoryAdjustmentQuantity, nil, result, PubSubMessageActionDelete)
		if err != nil {
			tx.Rollback()
			return nil, err
		}
	} else {
		err = PublishToAccounting(ctx, tx, businessId, result.AdjustmentDate, result.ID, AccountReferenceTypeInventoryAdjustmentValue, nil, result, PubSubMessageActionDelete)
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

func GetInventoryAdjustment(ctx context.Context, id int) (*InventoryAdjustment, error) {
	businessId, ok := utils.GetBusinessIdFromContext(ctx)
	if !ok || businessId == "" {
		return nil, errors.New("business id is required")
	}

	return utils.FetchModel[InventoryAdjustment](ctx, businessId, id)
}

func PaginateInventoryAdjustment(
	ctx context.Context, limit *int, after *string,
	referenceNumber *string,
	branchID *int,
	warehouseID *int,
	accountID *int,
	currentStatus *InventoryAdjustmentStatus,
	adjustmentType *InventoryAdjustmentType,
	startDate *MyDateString,
	endDate *MyDateString,
) (*InventoryAdjustmentsConnection, error) {

	businessId, ok := utils.GetBusinessIdFromContext(ctx)
	if !ok || businessId == "" {
		return nil, errors.New("business id is required")
	}
	business, err := GetBusiness(ctx)
	if err != nil {
		return nil, errors.New("business id is required")
	}
	if err := startDate.StartOfDayUTCTime(business.Timezone); err != nil {
		return nil, err
	}
	if err := endDate.EndOfDayUTCTime(business.Timezone); err != nil {
		return nil, err
	}

	db := config.GetDB()
	dbCtx := db.WithContext(ctx).Where("business_id = ?", businessId)

	if referenceNumber != nil && *referenceNumber != "" {
		dbCtx.Where("reference_number LIKE ?", "%"+*referenceNumber+"%")
	}
	if branchID != nil && *branchID > 0 {
		dbCtx.Where("branch_id = ?", *branchID)
	}
	if accountID != nil && *accountID > 0 {
		dbCtx.Where("account_id = ?", *accountID)
	}
	if warehouseID != nil && *warehouseID > 0 {
		dbCtx.Where("warehouse_id = ?", *warehouseID)
	}
	if currentStatus != nil {
		dbCtx.Where("current_status = ?", *currentStatus)
	}
	if adjustmentType != nil {
		dbCtx.Where("adjustment_type = ?", *adjustmentType)
	}
	if startDate != nil && endDate != nil {
		dbCtx.Where("adjustment_date BETWEEN ? AND ?", startDate, endDate)
	}

	edges, pageInfo, err := FetchPageCompositeCursor[InventoryAdjustment](dbCtx, *limit, after, "created_at", "<")
	if err != nil {
		return nil, err
	}
	var inventoryAdjustmentsConnection InventoryAdjustmentsConnection
	inventoryAdjustmentsConnection.PageInfo = pageInfo
	for _, edge := range edges {
		inventoryAdjustmentsEdge := InventoryAdjustmentsEdge(edge)
		inventoryAdjustmentsConnection.Edges = append(inventoryAdjustmentsConnection.Edges, &inventoryAdjustmentsEdge)
	}

	return &inventoryAdjustmentsConnection, err
}
