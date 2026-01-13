package middlewares

import (
	"context"

	"bitbucket.org/mmdatafocus/books_backend/models"
	"github.com/graph-gophers/dataloader/v7"
	"gorm.io/gorm"
)

type allAccountReader struct {
	db *gorm.DB
}

func (r *allAccountReader) getAllAccounts(ctx context.Context, ids []int) []*dataloader.Result[*models.AllAccount] {
	resultMap, err := models.MapAllAccount(ctx)
	if err != nil {
		return handleError[*models.AllAccount](len(ids), err)
	}
	var loaderResults = make([]*dataloader.Result[*models.AllAccount], 0, len(ids))
	for _, id := range ids {
		result, ok := resultMap[id]
		if !ok {
			var v models.AllAccount
			v.ID = id
			result = &v
		}
		loaderResults = append(loaderResults, &dataloader.Result[*models.AllAccount]{Data: result})
	}
	return loaderResults
}

func GetAllAccount(ctx context.Context, id int) (*models.AllAccount, error) {
	loaders := For(ctx)
	return loaders.allAccountLoader.Load(ctx, id)()
}

func GetAllAccounts(ctx context.Context, ids []int) ([]*models.AllAccount, []error) {
	loaders := For(ctx)
	return loaders.allAccountLoader.LoadMany(ctx, ids)()
}
