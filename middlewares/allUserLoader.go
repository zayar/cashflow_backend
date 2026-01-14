package middlewares

import (
	"context"

	"github.com/graph-gophers/dataloader/v7"
	"github.com/mmdatafocus/books_backend/models"
	"gorm.io/gorm"
)

type allUserReader struct {
	db *gorm.DB
}

func (r *allUserReader) getAllUsers(ctx context.Context, ids []int) []*dataloader.Result[*models.AllUser] {
	resultMap, err := models.MapAllUser(ctx)
	if err != nil {
		return handleError[*models.AllUser](len(ids), err)
	}
	var loaderResults = make([]*dataloader.Result[*models.AllUser], 0, len(ids))
	for _, id := range ids {
		result, ok := resultMap[id]
		if !ok {
			var v models.AllUser
			v.ID = id
			result = &v
		}
		loaderResults = append(loaderResults, &dataloader.Result[*models.AllUser]{Data: result})
	}
	return loaderResults
}

func GetAllUser(ctx context.Context, id int) (*models.AllUser, error) {
	loaders := For(ctx)
	return loaders.allUserLoader.Load(ctx, id)()
}

func GetAllUsers(ctx context.Context, ids []int) ([]*models.AllUser, []error) {
	loaders := For(ctx)
	return loaders.allUserLoader.LoadMany(ctx, ids)()
}
