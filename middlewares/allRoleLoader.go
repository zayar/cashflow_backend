package middlewares

import (
	"context"

	"github.com/graph-gophers/dataloader/v7"
	"github.com/mmdatafocus/books_backend/models"
	"gorm.io/gorm"
)

type allRoleReader struct {
	db *gorm.DB
}

func (r *allRoleReader) getAllRoles(ctx context.Context, ids []int) []*dataloader.Result[*models.AllRole] {
	resultMap, err := models.MapAllRole(ctx)
	if err != nil {
		return handleError[*models.AllRole](len(ids), err)
	}
	var loaderResults = make([]*dataloader.Result[*models.AllRole], 0, len(ids))
	for _, id := range ids {
		result, ok := resultMap[id]
		if !ok {
			var v models.AllRole
			v.ID = id
			result = &v
		}
		loaderResults = append(loaderResults, &dataloader.Result[*models.AllRole]{Data: result})
	}
	return loaderResults
}

func GetAllRole(ctx context.Context, id int) (*models.AllRole, error) {
	loaders := For(ctx)
	return loaders.allRoleLoader.Load(ctx, id)()
}

func GetAllRoles(ctx context.Context, ids []int) ([]*models.AllRole, []error) {
	loaders := For(ctx)
	return loaders.allRoleLoader.LoadMany(ctx, ids)()
}
