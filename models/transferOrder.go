package models

import (
	"context"
	"errors"
	"os"
	"strings"
	"time"

	"github.com/mmdatafocus/books_backend/config"
	"github.com/mmdatafocus/books_backend/utils"
	"github.com/shopspring/decimal"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

type TransferOrder struct {
	ID                     int                   `gorm:"primary_key" json:"id"`
	BusinessId             string                `gorm:"index;not null" json:"business_id" binding:"required"`
	OrderNumber            string                `gorm:"size:255;not null" json:"order_number"`
	TransferDate           time.Time             `gorm:"not null" json:"transfer_date" binding:"required"`
	ReasonId               int                   `gorm:"not null" json:"reasonId"`
	SourceWarehouseId      int                   `gorm:"index;not null" json:"source_warehouse_id" binding:"required"`
	DestinationWarehouseId int                   `gorm:"index;not null" json:"destination_warehouse_id" binding:"required"`
	TotalTransferQty       decimal.Decimal       `gorm:"type:decimal(20,4);default:0" json:"total_transfer_qty"`
	CurrentStatus          TransferOrderStatus   `gorm:"type:enum('Draft', 'Confirmed', 'Closed');not null" json:"current_status" binding:"required"`
	Documents              []*Document           `gorm:"polymorphic:Reference" json:"documents"`
	Details                []TransferOrderDetail `gorm:"foreignKey:TransferOrderId" json:"details"`
	CreatedAt              time.Time             `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt              time.Time             `gorm:"autoUpdateTime" json:"updated_at"`
}

type NewTransferOrder struct {
	OrderNumber            string                   `json:"order_number"`
	TransferDate           time.Time                `json:"transfer_date"`
	ReasonId               int                      `json:"reason_id"`
	SourceWarehouseId      int                      `json:"source_warehouse_id"`
	DestinationWarehouseId int                      `json:"destination_warehouse_id"`
	CurrentStatus          TransferOrderStatus      `json:"current_status"`
	Documents              []*NewDocument           `json:"documents"`
	Details                []NewTransferOrderDetail `json:"details"`
}

type TransferOrderDetail struct {
	ID              int             `gorm:"primary_key" json:"id"`
	TransferOrderId int             `gorm:"index;not null" json:"transfer_order_id" binding:"required"`
	ProductId       int             `gorm:"default:null" json:"product_id"`
	ProductType     ProductType     `gorm:"type:enum('S','G','C','V','I');default:S" json:"product_type"`
	BatchNumber     string          `gorm:"size:100" json:"batch_number"`
	Name            string          `gorm:"size:100" json:"name" binding:"required"`
	Description     string          `gorm:"size:255;default:null" json:"description"`
	TransferQty     decimal.Decimal `gorm:"type:decimal(20,4);default:0" json:"transfer_qty" binding:"required"`
	CreatedAt       time.Time       `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt       time.Time       `gorm:"autoUpdateTime" json:"updated_at"`
}

type NewTransferOrderDetail struct {
	DetailId      int             `json:"detail_id"`
	ProductId     int             `json:"product_id"`
	ProductType   ProductType     `json:"product_type"`
	BatchNumber   string          `json:"batch_number"`
	Name          string          `json:"name"`
	Description   string          `json:"description"`
	TransferQty   decimal.Decimal `json:"transfer_qty"`
	IsDeletedItem *bool           `json:"is_deleted_item"`
}

type TransferOrdersConnection struct {
	Edges    []*TransferOrdersEdge `json:"edges"`
	PageInfo *PageInfo             `json:"pageInfo"`
}

type TransferOrdersEdge Edge[TransferOrder]

func (obj TransferOrder) GetId() int {
	return obj.ID
}

// returns decoded curosr string
func (po TransferOrder) GetCursor() string {
	return po.CreatedAt.String()
}

func (to *TransferOrder) GetFieldValues(tx *gorm.DB) (*utils.DetailFieldValues, error) {
	return utils.FetchDetailFieldValues(tx, &TransferOrderDetail{}, "transfer_order_id", to.ID)
}

func (input NewTransferOrder) validate(ctx context.Context, businessId string, _ int) error {
	if input.SourceWarehouseId == input.DestinationWarehouseId {
		return errors.New("transfers cannot be made within the same warehouse. please choose a different one and proceed")
	}
	// exists warehouse
	if err := utils.ValidateResourceId[Warehouse](ctx, businessId, input.SourceWarehouseId); err != nil {
		return errors.New("source warehouse not found")
	}
	if err := utils.ValidateResourceId[Warehouse](ctx, businessId, input.DestinationWarehouseId); err != nil {
		return errors.New("destination warehouse not found")
	}
	if err := utils.ValidateResourceId[Reason](ctx, businessId, input.ReasonId); err != nil {
		return errors.New("reason not found")
	}
	// validate each product for inventory adjustment date
	for _, inputDetail := range input.Details {
		if err := ValidateValueAdjustment(ctx, businessId, input.TransferDate, inputDetail.ProductType, inputDetail.ProductId, &inputDetail.BatchNumber); err != nil {
			return err
		}
	}

	return nil
}

func CreateTransferOrder(ctx context.Context, input *NewTransferOrder) (*TransferOrder, error) {
	db := config.GetDB()

	businessId, ok := utils.GetBusinessIdFromContext(ctx)
	if !ok || businessId == "" {
		return nil, errors.New("business id is required")
	}
	logger := config.GetLogger()
	debug := strings.EqualFold(strings.TrimSpace(os.Getenv("DEBUG_TRANSFER_ORDER")), "true")
	// validate TransferOrder
	if err := input.validate(ctx, businessId, 0); err != nil {
		return nil, err
	}
	// construct Images
	documents, err := mapNewDocuments(input.Documents, "transfer_orders", 0)
	if err != nil {
		return nil, err
	}

	var transferItems []TransferOrderDetail
	var totalQty decimal.Decimal

	for _, item := range input.Details {
		if !IsRealProduct(ctx, businessId, item.ProductId, item.ProductType) {
			return nil, errors.New("product's inventory has not been tracked")
		}
		if item.TransferQty.IsZero() {
			return nil, errors.New("transfer quantity cannot be zero")
		}
		transferItem := TransferOrderDetail{
			ProductId:   item.ProductId,
			ProductType: item.ProductType,
			BatchNumber: item.BatchNumber,
			Name:        item.Name,
			Description: item.Description,
			TransferQty: item.TransferQty,
		}
		// Add the item to the TransferOrder
		transferItems = append(transferItems, transferItem)
		totalQty = totalQty.Add(item.TransferQty)
	}

	// store transferOrder
	transferOrder := TransferOrder{
		BusinessId:             businessId,
		OrderNumber:            input.OrderNumber,
		TransferDate:           input.TransferDate,
		ReasonId:               input.ReasonId,
		SourceWarehouseId:      input.SourceWarehouseId,
		DestinationWarehouseId: input.DestinationWarehouseId,
		CurrentStatus:          TransferOrderStatusDraft,
		TotalTransferQty:       totalQty,
		Documents:              documents,
		Details:                transferItems,
	}

	if debug {
		logger.WithFields(logrus.Fields{
			"field":                    "CreateTransferOrder",
			"business_id":              businessId,
			"order_number":             transferOrder.OrderNumber,
			"transfer_date":            transferOrder.TransferDate,
			"requested_status":         input.CurrentStatus,
			"source_warehouse_id":      transferOrder.SourceWarehouseId,
			"destination_warehouse_id": transferOrder.DestinationWarehouseId,
			"details_count":            len(transferOrder.Details),
		}).Info("begin transfer order create")
	}

	tx := db.Begin()

	err = tx.WithContext(ctx).Create(&transferOrder).Error
	if err != nil {
		if debug {
			logger.WithFields(logrus.Fields{
				"field":       "CreateTransferOrder",
				"business_id": businessId,
				"stage":       "create",
				"error":       err.Error(),
			}).Error("transfer order create failed; rollback")
		}
		tx.Rollback()
		return nil, err
	}

	// If requested "Confirmed", apply the status transition deterministically (Draft -> Confirmed).
	requestedStatus := input.CurrentStatus
	if requestedStatus == TransferOrderStatusConfirmed {
		if debug {
			logger.WithFields(logrus.Fields{
				"field":             "CreateTransferOrder",
				"business_id":       businessId,
				"transfer_order_id": transferOrder.ID,
				"stage":             "status_transition",
				"from_status":       TransferOrderStatusDraft,
				"to_status":         TransferOrderStatusConfirmed,
			}).Info("applying transfer order status transition")
		}
		if err := tx.WithContext(ctx).Model(&transferOrder).Update("CurrentStatus", TransferOrderStatusConfirmed).Error; err != nil {
			if debug {
				logger.WithFields(logrus.Fields{
					"field":             "CreateTransferOrder",
					"business_id":       businessId,
					"transfer_order_id": transferOrder.ID,
					"stage":             "status_update",
					"error":             err.Error(),
				}).Error("transfer order status update failed; rollback")
			}
			tx.Rollback()
			return nil, err
		}
		transferOrder.CurrentStatus = TransferOrderStatusConfirmed

		// Apply inventory side-effects deterministically.
		//
		// IMPORTANT:
		// This must run regardless of STOCK_COMMANDS_DOCS flag.
		// TransferOrder is created as Draft then transitioned to Confirmed via Update(),
		// which does NOT trigger the legacy model-hook stock updates (AfterUpdateCurrentStatus).
		// If we only run this when stock commands are enabled, confirmed transfers will post accounting
		// but never update stock_summary_daily_balances, causing warehouse reports to remain unchanged.
		if debug {
			logger.WithFields(logrus.Fields{
				"field":                  "CreateTransferOrder",
				"business_id":            businessId,
				"transfer_order_id":      transferOrder.ID,
				"stage":                  "apply_stock",
				"stock_commands_enabled": config.UseStockCommandsFor("TRANSFER_ORDER"),
			}).Info("applying transfer order stock side-effects")
		}
		if err := ApplyTransferOrderStockForStatusTransition(tx.WithContext(ctx), &transferOrder, TransferOrderStatusDraft); err != nil {
			if debug {
				logger.WithFields(logrus.Fields{
					"field":             "CreateTransferOrder",
					"business_id":       businessId,
					"transfer_order_id": transferOrder.ID,
					"stage":             "apply_stock",
					"error":             err.Error(),
				}).Error("transfer order stock side-effects failed; rollback")
			}
			tx.Rollback()
			return nil, err
		}

		// Write outbox record only when confirmed.
		if err := PublishToAccounting(ctx, tx, businessId, transferOrder.TransferDate, transferOrder.ID, AccountReferenceTypeTransferOrder, transferOrder, nil, PubSubMessageActionCreate); err != nil {
			if debug {
				logger.WithFields(logrus.Fields{
					"field":             "CreateTransferOrder",
					"business_id":       businessId,
					"transfer_order_id": transferOrder.ID,
					"stage":             "outbox",
					"error":             err.Error(),
				}).Error("transfer order outbox write failed; rollback")
			}
			tx.Rollback()
			return nil, err
		}
	} else {
		// Draft: do not publish posting.
	}

	if err := tx.Commit().Error; err != nil {
		return nil, err
	}

	if debug {
		logger.WithFields(logrus.Fields{
			"field":             "CreateTransferOrder",
			"business_id":       businessId,
			"transfer_order_id": transferOrder.ID,
			"status":            transferOrder.CurrentStatus,
		}).Info("transfer order committed")
	}

	return &transferOrder, nil
}

// func UpdateTransferOrder(ctx context.Context, transferOrderID int, updatedOrder *NewTransferOrder) (*TransferOrder, error) {
// 	db := config.GetDB()

// 	businessId, ok := utils.GetBusinessIdFromContext(ctx)
// 	if !ok || businessId == "" {
// 		return nil, errors.New("business id is required")
// 	}

// 	if err := updatedOrder.validate(ctx, businessId, transferOrderID); err != nil {
// 		return nil, err
// 	}
// 	// Fetch the existing transfer order
// 	var existingOrder TransferOrder
// 	if err := db.WithContext(ctx).Preload("Details").First(&existingOrder, transferOrderID).Error; err != nil {
// 		return nil, err
// 	}

// 	oldStatus := existingOrder.CurrentStatus

// 	// Update the fields of the existing transfer order with the provided updated details
// 	existingOrder.OrderNumber = updatedOrder.OrderNumber
// 	existingOrder.TransferDate = updatedOrder.TransferDate
// 	existingOrder.ReasonId = updatedOrder.ReasonId
// 	existingOrder.SourceWarehouseId = updatedOrder.SourceWarehouseId
// 	existingOrder.DestinationWarehouseId = updatedOrder.DestinationWarehouseId
// 	existingOrder.CurrentStatus = updatedOrder.CurrentStatus

// 	tx := db.Begin()

// 	var totalQty decimal.Decimal

// 	// Iterate through the updated items
// 	for _, updatedItem := range updatedOrder.Details {
// 		var existingItem *TransferOrderDetail

// 		// Check if the item already exists in the transfer order
// 		for _, item := range existingOrder.Details {
// 			if item.ID == updatedItem.DetailId {
// 				existingItem = &item
// 				break
// 			}
// 		}

// 		// If the item doesn't exist, add it to the transfer order
// 		if existingItem == nil {
// 			newItem := TransferOrderDetail{
// 				TransferOrderId: transferOrderID,
// 				ProductId:       updatedItem.ProductId,
// 				ProductType:     updatedItem.ProductType,
// 				BatchNumber:     updatedItem.BatchNumber,
// 				Name:            updatedItem.Name,
// 				Description:     updatedItem.Description,
// 				TransferQty:     updatedItem.TransferQty,
// 			}
// 			existingOrder.Details = append(existingOrder.Details, newItem)
// 			totalQty = totalQty.Add(updatedItem.TransferQty)

// 		} else {
// 			if updatedItem.IsDeletedItem != nil && *updatedItem.IsDeletedItem {
// 				// Find the index of the item to delete
// 				for i, item := range existingOrder.Details {
// 					if item.ID == updatedItem.DetailId {
// 						// Delete the item from the database
// 						if err := tx.WithContext(ctx).Delete(&existingOrder.Details[i]).Error; err != nil {
// 							tx.Rollback()
// 							return nil, err
// 						}

// 						if item.ProductId > 0 && existingOrder.CurrentStatus == TransferOrderStatusConfirmed {
// 							product, err := GetProductOrVariant(ctx, string(item.ProductType), item.ProductId)
// 							if err != nil {
// 								tx.Rollback()
// 								return nil, err
// 							}

// 							if product.GetInventoryAccountID() > 0 {
// 								if err := UpdateStockSummaryTransferQty(tx, businessId, existingOrder.SourceWarehouseId, item.ProductId, string(item.ProductType), item.BatchNumber, item.TransferQty, existingOrder.TransferDate); err != nil {
// 									tx.Rollback()
// 									return nil, err
// 								}
// 								if err := UpdateStockSummaryTransferQty(tx, businessId, existingOrder.DestinationWarehouseId, item.ProductId, string(item.ProductType), item.BatchNumber, item.TransferQty.Neg(), existingOrder.TransferDate); err != nil {
// 									tx.Rollback()
// 									return nil, err
// 								}
// 							}
// 						}
// 						// Remove the item from the slice
// 						existingOrder.Details = append(existingOrder.Details[:i], existingOrder.Details[i+1:]...)
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
// 				existingItem.TransferQty = updatedItem.TransferQty

// 				if err := tx.WithContext(ctx).Save(&existingItem).Error; err != nil {
// 					tx.Rollback()
// 					return nil, err
// 				}
// 				totalQty = totalQty.Add(updatedItem.TransferQty)
// 			}
// 		}
// 	}

// 	existingOrder.TotalTransferQty = totalQty

// 	// Save the updated transfer order to the database
// 	if err := tx.WithContext(ctx).Save(&existingOrder).Error; err != nil {
// 		tx.Rollback()
// 		return nil, err
// 	}

// 	// Refresh the existingBill to get the latest details
// 	if err := tx.WithContext(ctx).Preload("Details").First(&existingOrder, transferOrderID).Error; err != nil {
// 		tx.Rollback()
// 		return nil, err
// 	}

// 	if err := existingOrder.AfterUpdateCurrentStatus(tx.WithContext(ctx), string(oldStatus)); err != nil {
// 		tx.Rollback()
// 		return nil, err
// 	}

// 	documents, err := upsertDocuments(ctx, tx, updatedOrder.Documents, "transfer_orders", transferOrderID)
// 	if err != nil {
// 		tx.Rollback()
// 		return nil, err
// 	}
// 	existingOrder.Documents = documents

// 	if err := tx.Commit().Error; err != nil {
// 		return nil, err
// 	}

// 	return &existingOrder, nil
// }

// func DeleteTransferOrder(ctx context.Context, id int) (*TransferOrder, error) {
// 	businessId, ok := utils.GetBusinessIdFromContext(ctx)
// 	if !ok || businessId == "" {
// 		return nil, errors.New("business id is required")
// 	}

// 	result, err := utils.FetchModel[TransferOrder](ctx, businessId, id, "Details", "Documents")
// 	if err != nil {
// 		return nil, utils.ErrorRecordNotFound
// 	}

// 	db := config.GetDB()
// 	tx := db.Begin()

// 	// reduced received qty from stock summary if bill is confirmed
// 	if result.CurrentStatus == TransferOrderStatusConfirmed {
// 		for _, detailItem := range result.Details {
// 			if detailItem.ProductId > 0 {
// 				product, err := GetProductOrVariant(ctx, string(detailItem.ProductType), detailItem.ProductId)
// 				if err != nil {
// 					tx.Rollback()
// 					return nil, err
// 				}
// 				if product.GetInventoryAccountID() > 0 {
// 					if err := UpdateStockSummaryTransferQty(tx, result.BusinessId, result.SourceWarehouseId, detailItem.ProductId, string(detailItem.ProductType), detailItem.BatchNumber, detailItem.TransferQty, result.TransferDate); err != nil {
// 						tx.Rollback()
// 						return nil, err
// 					}
// 					if err := UpdateStockSummaryTransferQty(tx, result.BusinessId, result.DestinationWarehouseId, detailItem.ProductId, string(detailItem.ProductType), detailItem.BatchNumber, detailItem.TransferQty.Neg(), result.TransferDate); err != nil {
// 						tx.Rollback()
// 						return nil, err
// 					}
// 				}
// 			}
// 		}
// 	}

// 	err = tx.WithContext(ctx).Model(&result).Association("Details").Unscoped().Clear()
// 	if err != nil {
// 		tx.Rollback()
// 		return nil, err
// 	}

// 	err = tx.WithContext(ctx).Delete(&result).Error
// 	if err != nil {
// 		tx.Rollback()
// 		return nil, err
// 	}

// 	if err := deleteDocuments(ctx, tx, result.Documents); err != nil {
// 		tx.Rollback()
// 		return nil, err
// 	}

// 	if err := tx.Commit().Error; err != nil {
// 		return nil, err
// 	}

// 	return result, nil
// }

func GetTransferOrder(ctx context.Context, id int) (*TransferOrder, error) {
	businessId, ok := utils.GetBusinessIdFromContext(ctx)
	if !ok || businessId == "" {
		return nil, errors.New("business id is required")
	}

	return utils.FetchModel[TransferOrder](ctx, businessId, id)
}

func PaginateTransferOrder(
	ctx context.Context, limit *int, after *string,
	orderNumber *string,
	currentStatus *TransferOrderStatus,
) (*TransferOrdersConnection, error) {

	businessId, ok := utils.GetBusinessIdFromContext(ctx)
	if !ok || businessId == "" {
		return nil, errors.New("business id is required")
	}

	db := config.GetDB()
	dbCtx := db.WithContext(ctx).Where("business_id = ?", businessId)

	if orderNumber != nil && *orderNumber != "" {
		dbCtx.Where("order_number LIKE ?", "%"+*orderNumber+"%")
	}

	if currentStatus != nil {
		dbCtx.Where("current_status = ?", *currentStatus)
	}

	edges, pageInfo, err := FetchPageCompositeCursor[TransferOrder](dbCtx, *limit, after, "created_at", "<")
	if err != nil {
		return nil, err
	}
	var transferOrdersConnection TransferOrdersConnection
	transferOrdersConnection.PageInfo = pageInfo
	for _, edge := range edges {
		transferOrdersEdge := TransferOrdersEdge(edge)
		transferOrdersConnection.Edges = append(transferOrdersConnection.Edges, &transferOrdersEdge)
	}

	return &transferOrdersConnection, err
}
