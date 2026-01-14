package middlewares

import (
	"context"

	"github.com/graph-gophers/dataloader/v7"
	"github.com/mmdatafocus/books_backend/models"
	"gorm.io/gorm"
)

type creditNoteDetailsReader struct {
	db *gorm.DB
}

func (r *creditNoteDetailsReader) GetDetails(ctx context.Context, ids []int) []*dataloader.Result[[]*models.CreditNoteDetail] {
	var results []models.CreditNoteDetail
	err := r.db.Where("credit_note_id IN ?", ids).Find(&results).Error
	if err != nil {
		return handleError[[]*models.CreditNoteDetail](len(results), err)
	}
	return generateLoaderArrayResults(results, ids)
}

func GetCreditNoteDetails(ctx context.Context, cnId int) ([]*models.CreditNoteDetail, error) {
	loaders := For(ctx)
	return loaders.creditNoteDetailsLoader.Load(ctx, cnId)()
}
