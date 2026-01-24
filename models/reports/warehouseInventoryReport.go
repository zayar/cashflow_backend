package reports

import (
	"context"
	"fmt"
	"time"

	"github.com/mmdatafocus/books_backend/models"
	"github.com/mmdatafocus/books_backend/utils"
)

type WarehouseInventoryResponse = models.WarehouseInventoryResponse

func GetWarehouseInventoryReport(ctx context.Context, toDate models.MyDateString) ([]*WarehouseInventoryResponse, error) {
	start := time.Now()
	defer logSlowReport(ctx, "warehouse_inventory_report", start, map[string]any{
		"to_date": fmt.Sprintf("%v", time.Time(toDate).UTC()),
	})

	if reportCacheEnabled() {
		biz, _ := utils.GetBusinessIdFromContext(ctx)
		key := fmt.Sprintf("report:warehouse_inventory:%s:%s", biz, time.Time(toDate).UTC().Format("2006-01-02"))
		var cached []*WarehouseInventoryResponse
		if ok, err := cacheGet(key, &cached); err == nil && ok && cached != nil {
			return cached, nil
		}
		rows, err := models.GetWarehouseInventoryLedger(ctx, toDate)
		if err != nil {
			return nil, err
		}
		_ = cacheSet(key, rows, reportCacheTTL())
		return rows, nil
	}

	return models.GetWarehouseInventoryLedger(ctx, toDate)
}
