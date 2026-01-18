package reports

import (
	"context"

	"github.com/mmdatafocus/books_backend/models"
)

type InventorySummaryResponse = models.InventorySummaryResponse

func GetInventorySummaryReport(ctx context.Context, toDate models.MyDateString, warehouseId *int) ([]*InventorySummaryResponse, error) {
	return models.GetInventorySummaryLedger(ctx, toDate, warehouseId)
}
