package middlewares

import (
	"context"

	"github.com/graph-gophers/dataloader/v7"
	"github.com/mmdatafocus/books_backend/models"
	"gorm.io/gorm"
)

type branchReader struct {
	db *gorm.DB
}

func (r *branchReader) getBranches(ctx context.Context, ids []int) []*dataloader.Result[*models.Branch] {
	var results []models.Branch
	err := r.db.WithContext(ctx).Where("id IN ?", ids).Find(&results).Error
	if err != nil {
		return handleError[*models.Branch](len(ids), err)
	}
	return generateLoaderResults(results, ids)
	// resultMap := make(map[int]*models.Branch)
	// resultMap[0] = &models.Branch{}
	// for _, result := range results {
	// 	resultMap[result.ID] = result
	// }

	// loaderResults := make([]*dataloader.Result[*models.Branch], 0, len(ids))
	// for _, id := range ids {
	// 	result := resultMap[id]
	// 	loaderResults = append(loaderResults, &dataloader.Result[*models.Branch]{Data: result})
	// }
	// return loaderResults
}

func GetBranch(ctx context.Context, id int) (*models.Branch, error) {
	loaders := For(ctx)
	return loaders.BranchLoader.Load(ctx, id)()
}

func GetBranches(ctx context.Context, ids []int) ([]*models.Branch, []error) {
	loaders := For(ctx)
	return loaders.BranchLoader.LoadMany(ctx, ids)()
}
