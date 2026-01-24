package middlewares

import (
	"context"

	"github.com/graph-gophers/dataloader/v7"
	"github.com/mmdatafocus/books_backend/models"
	"gorm.io/gorm"
)

type creditNoteByIDReader struct {
	db *gorm.DB
}

func (r *creditNoteByIDReader) getCreditNotesByID(ctx context.Context, ids []int) []*dataloader.Result[*models.CreditNote] {
	var results []models.CreditNote
	err := r.db.WithContext(ctx).Where("id IN ?", ids).Find(&results).Error
	if err != nil {
		return handleError[*models.CreditNote](len(ids), err)
	}

	resultMap := make(map[int]*models.CreditNote, len(results))
	for i := range results {
		resultMap[results[i].ID] = &results[i]
	}

	loaderResults := make([]*dataloader.Result[*models.CreditNote], 0, len(ids))
	for _, id := range ids {
		if v, ok := resultMap[id]; ok {
			loaderResults = append(loaderResults, &dataloader.Result[*models.CreditNote]{Data: v})
		} else {
			loaderResults = append(loaderResults, &dataloader.Result[*models.CreditNote]{Error: gorm.ErrRecordNotFound})
		}
	}
	return loaderResults
}

func GetCreditNote(ctx context.Context, id int) (*models.CreditNote, error) {
	loaders := For(ctx)
	return loaders.creditNoteByIDLoader.Load(ctx, id)()
}

func GetCreditNotes(ctx context.Context, ids []int) ([]*models.CreditNote, []error) {
	loaders := For(ctx)
	return loaders.creditNoteByIDLoader.LoadMany(ctx, ids)()
}

