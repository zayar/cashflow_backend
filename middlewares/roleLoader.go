package middlewares

import (
	"context"

	"bitbucket.org/mmdatafocus/books_backend/models"
	"github.com/graph-gophers/dataloader/v7"
	"gorm.io/gorm"
)

type roleReader struct {
	db *gorm.DB
}

func (r *roleReader) getRoles(ctx context.Context, ids []int) []*dataloader.Result[*models.Role] {
	var results []models.Role
	err := r.db.WithContext(ctx).Where("id IN ?", ids).Find(&results).Error
	if err != nil {
		return handleError[*models.Role](len(ids), err)
	}

	return generateLoaderResults(results, ids)
	// resultMap := make(map[int]*models.Role)
	// resultMap[0] = &models.Role{}
	// for _, result := range results {
	// 	resultMap[result.ID] = result
	// }

	// loaderResults := make([]*dataloader.Result[*models.Role], 0, len(ids))
	// for _, id := range ids {
	// 	result := resultMap[id]
	// 	loaderResults = append(loaderResults, &dataloader.Result[*models.Role]{Data: result})
	// }

	// return loaderResults
}

func GetRole(ctx context.Context, id int) (*models.Role, error) {
	loaders := For(ctx)
	return loaders.RoleLoader.Load(ctx, id)()
}

func GetRoles(ctx context.Context, ids []int) ([]*models.Role, []error) {
	loaders := For(ctx)
	return loaders.RoleLoader.LoadMany(ctx, ids)()
}
