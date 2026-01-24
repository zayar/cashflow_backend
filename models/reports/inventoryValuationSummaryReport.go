package reports

import (
	"context"
	"fmt"
	"time"

	"github.com/mmdatafocus/books_backend/models"
	"github.com/mmdatafocus/books_backend/utils"
)

type InventoryValuationSummaryResponse = models.InventoryValuationSummaryResponse

func GetInventoryValuationSummaryReport(ctx context.Context, currentDate models.MyDateString, warehouseId int) ([]*InventoryValuationSummaryResponse, error) {
	start := time.Now()
	defer logSlowReport(ctx, "inventory_valuation_summary_report", start, map[string]any{
		"warehouse_id": warehouseId,
		"as_of":        fmt.Sprintf("%v", time.Time(currentDate).UTC()),
	})

	if reportCacheEnabled() {
		biz, _ := utils.GetBusinessIdFromContext(ctx)
		key := fmt.Sprintf("report:inventory_valuation_summary:%s:%d:%s", biz, warehouseId, time.Time(currentDate).UTC().Format("2006-01-02"))
		var cached []*InventoryValuationSummaryResponse
		if ok, err := cacheGet(key, &cached); err == nil && ok && cached != nil {
			return cached, nil
		}
		rows, err := models.GetInventoryValuationSummaryLedger(ctx, currentDate, warehouseId)
		if err != nil {
			return nil, err
		}
		_ = cacheSet(key, rows, reportCacheTTL())
		return rows, nil
	}

	return models.GetInventoryValuationSummaryLedger(ctx, currentDate, warehouseId)
}
