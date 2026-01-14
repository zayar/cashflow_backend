package utils

import (
	"context"

	"github.com/mmdatafocus/books_backend/config"
	"gorm.io/gorm"
)

type ModelChangeLocker interface {
	CheckTransactionLock(context.Context) error
}

/* DB fetching */

// fetch model from db
// (may return RecordNotFound)
func FetchSingleModel[T any](ctx context.Context, id int, associations ...string) (*T, error) {

	db := config.GetDB()
	dbCtx := db.WithContext(ctx)
	// preloading
	for _, field := range associations {
		dbCtx.Preload(field)
	}
	var result T
	err := dbCtx.First(&result, id).Error
	if err != nil {
		return nil, ErrorRecordNotFound
	}
	return &result, nil
}

// fetch model from db
// (ctx's business_id is used in query's WHERE, may return RecordNotFound)
func FetchModel[T any](ctx context.Context, businessId string, id int, associations ...string) (*T, error) {

	db := config.GetDB()
	dbCtx := db.WithContext(ctx).Where("business_id = ?", businessId)
	// preloading
	for _, field := range associations {
		dbCtx.Preload(field)
	}
	var result T
	err := dbCtx.First(&result, id).Error
	if err != nil {
		return nil, ErrorRecordNotFound
	}
	return &result, nil
}

// fetch model and check if model is locked by transaction date
func FetchModelForChange[T ModelChangeLocker](ctx context.Context, businessId string, id int, associations ...string) (*T, error) {
	result, err := FetchModel[T](ctx, businessId, id, associations...)
	if err != nil {
		return nil, err
	}
	if err := (*result).CheckTransactionLock(ctx); err != nil {
		return nil, err
	}
	return result, nil
}

// fetch all models from db
// (ctx's business_id is used in query's WHERE)
func FetchAllModels[T any](ctx context.Context, businessId string, associations ...string) ([]*T, error) {

	db := config.GetDB()
	dbCtx := db.WithContext(ctx).Where("business_id = ?", businessId)
	// preloading
	for _, field := range associations {
		dbCtx.Preload(field)
	}
	var results []*T
	err := dbCtx.Find(&results).Error
	if err != nil {
		return nil, ErrorRecordNotFound
	}
	return results, nil
}

// // read model list, redis or db, cache result
// func ListModel[T any](ctx context.Context, businessId string, associations ...string) ([]*T, error) {
// 	results, err := RetrieveRedisList[T](businessId)
// 	if err != nil {
// 		return nil, err
// 	}
// 	// if not exists in redis
// 	if results == nil {
// 		results, err = FetchAllModels[T](ctx, businessId, associations...)
// 		if err != nil {
// 			return nil, err
// 		}
// 		// caching
// 		if err := StoreRedisList[T](results, businessId); err != nil {
// 			return nil, err
// 		}
// 	}

// 	return results, nil
// }

func GetPolymorphicId[T any](ctx context.Context, referenceType string, referenceId int) (int, error) {
	db := config.GetDB()
	var v T
	var id int
	err := db.WithContext(ctx).Model(&v).Where("reference_type = ? AND reference_id = ?", referenceType, referenceId).Select("id").Scan(&id).Error
	if err == gorm.ErrRecordNotFound {
		return 0, nil
	}
	return id, err
}
