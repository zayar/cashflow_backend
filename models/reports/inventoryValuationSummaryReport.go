package reports

import (
	"context"

	"github.com/mmdatafocus/books_backend/models"
)

type InventoryValuationSummaryResponse = models.InventoryValuationSummaryResponse

func GetInventoryValuationSummaryReport(ctx context.Context, currentDate models.MyDateString, warehouseId int) ([]*InventoryValuationSummaryResponse, error) {
	return models.GetInventoryValuationSummaryLedger(ctx, currentDate, warehouseId)
}
