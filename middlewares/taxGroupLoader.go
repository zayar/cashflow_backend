package middlewares

import (
	"context"

	"bitbucket.org/mmdatafocus/books_backend/models"
	"github.com/graph-gophers/dataloader/v7"
	"gorm.io/gorm"
)

type taxGroupReader struct {
	db *gorm.DB
}

func (r *taxGroupReader) getTaxGroups(ctx context.Context, ids []int) []*dataloader.Result[*models.TaxGroup] {
	var results []models.TaxGroup
	err := r.db.WithContext(ctx).
		Where("id IN ?", ids).Find(&results).Error
	if err != nil {
		return handleError[*models.TaxGroup](len(ids), err)
	}

	return generateLoaderResults(results, ids)
	// resultMap := make(map[int]*models.Township)
	// resultMap[0] = &models.Township{}
	// for _, result := range results {
	// 	resultMap[result.ID] = result
	// }

	// loaderResults := make([]*dataloader.Result[*models.Township], 0, len(ids))
	// for _, id := range ids {
	// 	result := resultMap[id]
	// 	loaderResults = append(loaderResults, &dataloader.Result[*models.Township]{Data: result})
	// }

	// return loaderResults
}

func GetTaxGroup(ctx context.Context, id int) (*models.TaxGroup, error) {
	loaders := For(ctx)
	return loaders.taxGroupLoader.Load(ctx, id)()
}

func GetTaxGroups(ctx context.Context, ids []int) ([]*models.TaxGroup, []error) {
	loaders := For(ctx)
	return loaders.taxGroupLoader.LoadMany(ctx, ids)()
}
