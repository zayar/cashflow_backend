package middlewares

import (
	"context"

	"github.com/graph-gophers/dataloader/v7"
	"github.com/mmdatafocus/books_backend/models"
	"gorm.io/gorm"
)

type allBranchReader struct {
	db *gorm.DB
}

func (r *allBranchReader) getAllBranchs(ctx context.Context, ids []int) []*dataloader.Result[*models.AllBranch] {
	resultMap, err := models.MapAllBranch(ctx)
	if err != nil {
		return handleError[*models.AllBranch](len(ids), err)
	}
	var loaderResults = make([]*dataloader.Result[*models.AllBranch], 0, len(ids))
	for _, id := range ids {
		result, ok := resultMap[id]
		if !ok {
			var v models.AllBranch
			v.ID = id
			result = &v
		}
		loaderResults = append(loaderResults, &dataloader.Result[*models.AllBranch]{Data: result})
	}
	return loaderResults
}

func GetAllBranch(ctx context.Context, id int) (*models.AllBranch, error) {
	loaders := For(ctx)
	return loaders.allBranchLoader.Load(ctx, id)()
}

func GetAllBranchs(ctx context.Context, ids []int) ([]*models.AllBranch, []error) {
	loaders := For(ctx)
	return loaders.allBranchLoader.LoadMany(ctx, ids)()
}
