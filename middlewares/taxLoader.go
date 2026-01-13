package middlewares

import (
	"context"

	"bitbucket.org/mmdatafocus/books_backend/models"
	"github.com/graph-gophers/dataloader/v7"
	"gorm.io/gorm"
)

type taxReader struct {
	db *gorm.DB
}

func (r *taxReader) getTaxes(ctx context.Context, ids []int) []*dataloader.Result[*models.Tax] {
	var results []models.Tax
	err := r.db.WithContext(ctx).
		Where("id IN ?", ids).Find(&results).Error
	if err != nil {
		return handleError[*models.Tax](len(ids), err)
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

func GetTax(ctx context.Context, id int) (*models.Tax, error) {
	loaders := For(ctx)
	return loaders.taxLoader.Load(ctx, id)()
}

func GetTaxes(ctx context.Context, ids []int) ([]*models.Tax, []error) {
	loaders := For(ctx)
	return loaders.taxLoader.LoadMany(ctx, ids)()
}

// call loaders for tax or tax group
func ResolveTaxInfo(ctx context.Context, taxId int, taxType *models.TaxType) (*models.TaxInfo, error) {
	if taxType == nil {
		return nil, nil
	}
	if *taxType == models.TaxTypeIndividual {
		tax, err := GetAllTax(ctx, taxId)
		if err != nil {
			return nil, err
		}
		taxInfo := tax.Info()
		return &taxInfo, nil
	} else {
		taxGroup, err := GetAllTaxGroup(ctx, taxId)
		if err != nil {
			return nil, err
		}
		taxInfo := taxGroup.Info()
		return &taxInfo, nil
	}
}
