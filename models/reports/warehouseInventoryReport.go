package reports

import (
	"context"

	"github.com/mmdatafocus/books_backend/models"
)

type WarehouseInventoryResponse = models.WarehouseInventoryResponse

func GetWarehouseInventoryReport(ctx context.Context, toDate models.MyDateString) ([]*WarehouseInventoryResponse, error) {
	return models.GetWarehouseInventoryLedger(ctx, toDate)
}
