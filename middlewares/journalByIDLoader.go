package middlewares

import (
	"context"

	"github.com/graph-gophers/dataloader/v7"
	"github.com/mmdatafocus/books_backend/models"
	"gorm.io/gorm"
)

type journalByIDReader struct {
	db *gorm.DB
}

func (r *journalByIDReader) getJournalsByID(ctx context.Context, ids []int) []*dataloader.Result[*models.Journal] {
	var results []models.Journal
	err := r.db.WithContext(ctx).
		Preload("Transactions").
		Where("id IN ?", ids).
		Find(&results).Error
	if err != nil {
		return handleError[*models.Journal](len(ids), err)
	}

	resultMap := make(map[int]*models.Journal, len(results))
	for i := range results {
		resultMap[results[i].ID] = &results[i]
	}

	loaderResults := make([]*dataloader.Result[*models.Journal], 0, len(ids))
	for _, id := range ids {
		if v, ok := resultMap[id]; ok {
			loaderResults = append(loaderResults, &dataloader.Result[*models.Journal]{Data: v})
		} else {
			loaderResults = append(loaderResults, &dataloader.Result[*models.Journal]{Error: gorm.ErrRecordNotFound})
		}
	}
	return loaderResults
}

func GetJournal(ctx context.Context, id int) (*models.Journal, error) {
	loaders := For(ctx)
	return loaders.journalByIDLoader.Load(ctx, id)()
}
