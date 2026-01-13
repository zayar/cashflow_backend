package middlewares

import (
	"context"

	"bitbucket.org/mmdatafocus/books_backend/models"
	"github.com/graph-gophers/dataloader/v7"
	"gorm.io/gorm"
)

type moduleReader struct {
	db *gorm.DB
}

func (r *moduleReader) getModules(ctx context.Context, ids []int) []*dataloader.Result[*models.Module] {
	var results []models.Module
	err := r.db.WithContext(ctx).Where("id IN ?", ids).Find(&results).Error
	if err != nil {
		return handleError[*models.Module](len(ids), err)
	}

	return generateLoaderResults(results, ids)
	// resultMap := make(map[int]*models.Module)
	// resultMap[0] = &models.Module{}
	// for _, result := range results {
	// 	resultMap[result.ID] = result
	// }

	// loaderResults := make([]*dataloader.Result[*models.Module], 0, len(ids))
	// for _, id := range ids {
	// 	result := resultMap[id]
	// 	loaderResults = append(loaderResults, &dataloader.Result[*models.Module]{Data: result})
	// }

	// return loaderResults
}

func GetModule(ctx context.Context, id int) (*models.Module, error) {
	loaders := For(ctx)
	return loaders.ModuleLoader.Load(ctx, id)()
}

func GetModules(ctx context.Context, ids []int) ([]*models.Module, []error) {
	loaders := For(ctx)
	return loaders.ModuleLoader.LoadMany(ctx, ids)()
}
