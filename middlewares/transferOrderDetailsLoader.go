package middlewares

import (
	"context"

	"bitbucket.org/mmdatafocus/books_backend/models"
	"github.com/graph-gophers/dataloader/v7"
	"gorm.io/gorm"
)

type transferOrderDetailReader struct {
	db *gorm.DB
}

func (r *transferOrderDetailReader) GetTransferOrderDetails(ctx context.Context, Ids []int) []*dataloader.Result[[]*models.TransferOrderDetail] {
	var results []models.TransferOrderDetail
	err := r.db.WithContext(ctx).Where("transfer_order_id IN ?", Ids).Find(&results).Error
	if err != nil {
		return handleError[[]*models.TransferOrderDetail](len(Ids), err)
	}

	return generateLoaderArrayResults(results, Ids)
}

func GetTransferOrderDetails(ctx context.Context, transferOrderId int) ([]*models.TransferOrderDetail, error) {
	loaders := For(ctx)
	return loaders.transferOrderDetailLoader.Load(ctx, transferOrderId)()
}
