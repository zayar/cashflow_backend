package middlewares

import (
	"context"

	"bitbucket.org/mmdatafocus/books_backend/models"
	"github.com/graph-gophers/dataloader/v7"
	"gorm.io/gorm"
)

type allMoneyAccountReader struct {
	db *gorm.DB
}

func (r *allMoneyAccountReader) getAllMoneyAccounts(ctx context.Context, ids []int) []*dataloader.Result[*models.AllMoneyAccount] {
	resultMap, err := models.MapAllMoneyAccount(ctx)
	if err != nil {
		return handleError[*models.AllMoneyAccount](len(ids), err)
	}
	var loaderResults = make([]*dataloader.Result[*models.AllMoneyAccount], 0, len(ids))
	for _, id := range ids {
		result, ok := resultMap[id]
		if !ok {
			var v models.AllMoneyAccount
			v.ID = id
			result = &v
		}
		loaderResults = append(loaderResults, &dataloader.Result[*models.AllMoneyAccount]{Data: result})
	}
	return loaderResults
}

func GetAllMoneyAccount(ctx context.Context, id int) (*models.AllMoneyAccount, error) {
	loaders := For(ctx)
	return loaders.allMoneyAccountLoader.Load(ctx, id)()
}

func GetAllMoneyAccounts(ctx context.Context, ids []int) ([]*models.AllMoneyAccount, []error) {
	loaders := For(ctx)
	return loaders.allMoneyAccountLoader.LoadMany(ctx, ids)()
}
