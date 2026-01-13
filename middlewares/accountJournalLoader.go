package middlewares

import (
	"context"

	"bitbucket.org/mmdatafocus/books_backend/models"
	"github.com/graph-gophers/dataloader/v7"
	"gorm.io/gorm"
)

type accountJournalReader struct {
	db *gorm.DB
}

func (r *accountJournalReader) GetAccountJournals(ctx context.Context, ids []int) []*dataloader.Result[*models.AccountJournal] {
	var results []models.AccountJournal
	err := r.db.WithContext(ctx).Where("id IN ?", ids).Find(&results).Error
	if err != nil {
		return handleError[*models.AccountJournal](len(ids), err)
	}

	return generateLoaderResults(results, ids)
	// resultMap := make(map[int]*models.AccountJournal)
	// resultMap[0] = &models.AccountJournal{}
	// for _, result := range results {
	// 	resultMap[result.ID] = result
	// }

	// loaderResults := make([]*dataloader.Result[*models.AccountJournal], 0, len(ids))
	// for _, id := range ids {
	// 	result := resultMap[id]
	// 	loaderResults = append(loaderResults, &dataloader.Result[*models.AccountJournal]{Data: result})
	// }
	// return loaderResults
}

func GetAccountJournal(ctx context.Context, id int) (*models.AccountJournal, error) {
	loaders := For(ctx)
	return loaders.accountJournalLoader.Load(ctx, id)()
}

func GetAccountJournals(ctx context.Context, ids []int) ([]*models.AccountJournal, []error) {
	loaders := For(ctx)
	return loaders.accountJournalLoader.LoadMany(ctx, ids)()
}
