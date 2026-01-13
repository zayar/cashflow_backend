package middlewares

import (
	"context"

	"bitbucket.org/mmdatafocus/books_backend/models"
	"github.com/graph-gophers/dataloader/v7"
	"gorm.io/gorm"
)

type deliveryMethodReader struct {
	db *gorm.DB
}

func (r *deliveryMethodReader) getDeliveryMethods(ctx context.Context, ids []int) []*dataloader.Result[*models.DeliveryMethod] {
	var results []models.DeliveryMethod
	err := r.db.WithContext(ctx).Where("id IN ?", ids).Find(&results).Error
	if err != nil {
		return handleError[*models.DeliveryMethod](len(ids), err)
	}

	return generateLoaderResults(results, ids)
	// resultMap := make(map[int]*models.DeliveryMethod)
	// resultMap[0] = &models.DeliveryMethod{}
	// for _, result := range results {
	// 	resultMap[result.ID] = result
	// }

	// loaderResults := make([]*dataloader.Result[*models.DeliveryMethod], 0, len(ids))
	// for _, id := range ids {
	// 	result := resultMap[id]
	// 	loaderResults = append(loaderResults, &dataloader.Result[*models.DeliveryMethod]{Data: result})
	// }
	// return loaderResults
}

func GetDeliveryMethod(ctx context.Context, id int) (*models.DeliveryMethod, error) {
	loaders := For(ctx)
	return loaders.deliveryMethodLoader.Load(ctx, id)()
}

func GetDeliveryMethods(ctx context.Context, ids []int) ([]*models.DeliveryMethod, []error) {
	loaders := For(ctx)
	return loaders.deliveryMethodLoader.LoadMany(ctx, ids)()
}
