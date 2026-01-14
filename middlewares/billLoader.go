package middlewares

import (
	"context"

	"github.com/graph-gophers/dataloader/v7"
	"github.com/mmdatafocus/books_backend/models"
	"gorm.io/gorm"
)

type billReader struct {
	db *gorm.DB
}

func (r *billReader) getBills(ctx context.Context, ids []int) []*dataloader.Result[*models.Bill] {
	var results []models.Bill
	err := r.db.WithContext(ctx).Where("id IN ?", ids).Find(&results).Error
	if err != nil {
		return handleError[*models.Bill](len(ids), err)
	}

	return generateLoaderResults(results, ids)
	// resultMap := make(map[int]*models.Bill)
	// resultMap[0] = &models.Bill{}
	// for _, result := range results {
	// 	resultMap[result.ID] = result
	// }

	// loaderResults := make([]*dataloader.Result[*models.Bill], 0, len(ids))
	// for _, id := range ids {
	// 	result := resultMap[id]
	// 	loaderResults = append(loaderResults, &dataloader.Result[*models.Bill]{Data: result})
	// }
	// return loaderResults
}

func GetBill(ctx context.Context, id int) (*models.Bill, error) {
	loaders := For(ctx)
	return loaders.billLoader.Load(ctx, id)()
}

func GetBills(ctx context.Context, ids []int) ([]*models.Bill, []error) {
	loaders := For(ctx)
	return loaders.billLoader.LoadMany(ctx, ids)()
}
