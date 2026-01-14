package middlewares

import (
	"context"

	"github.com/graph-gophers/dataloader/v7"
	"github.com/mmdatafocus/books_backend/models"
	"gorm.io/gorm"
)

type inventoryAdjustmentDetailReader struct {
	db *gorm.DB
}

func (r *inventoryAdjustmentDetailReader) GetInventoryAdjustmentDetails(ctx context.Context, Ids []int) []*dataloader.Result[[]*models.InventoryAdjustmentDetail] {
	var results []models.InventoryAdjustmentDetail
	err := r.db.WithContext(ctx).Where("inventory_adjustment_id IN ?", Ids).Find(&results).Error
	if err != nil {
		return handleError[[]*models.InventoryAdjustmentDetail](len(Ids), err)
	}

	return generateLoaderArrayResults(results, Ids)
}

func GetInventoryAdjustmentDetails(ctx context.Context, orderId int) ([]*models.InventoryAdjustmentDetail, error) {
	loaders := For(ctx)
	return loaders.inventoryAdjustmentDetailLoader.Load(ctx, orderId)()
}
