package models

import (
	"context"
	"errors"

	"bitbucket.org/mmdatafocus/books_backend/config"
	"bitbucket.org/mmdatafocus/books_backend/utils"
)

type Resource interface {
	GetBusinessId() string
}

// first find in redis, then in db, using ctx's business_id in WHERE, cache result
// (may return RecordNotFound error)
func GetResource[T Resource](ctx context.Context, id int, associations ...string) (*T, error) {

	businessId, ok := utils.GetBusinessIdFromContext(ctx)
	if !ok || businessId == "" {
		return nil, errors.New("business id is required")
	}
	// find in redis
	result, err := utils.RetrieveRedis[T](id)
	if err != nil {
		return nil, err
	}
	// if not found in redis
	if result == nil {
		// fetch from db
		result, err = utils.FetchModel[T](ctx, businessId, id, associations...)
		if err != nil {
			return nil, err
		}

		// store in redis
		if err := utils.StoreRedis[T](result, id); err != nil {
			return nil, err
		}
	} else {
		// if found in redis
		// check if business ids match
		if (*result).GetBusinessId() != businessId {
			return nil, errors.New("cannot access resource owned by other business")
		}
	}

	return result, nil
}

// list all resources, redis or db, cache result
func ListAllResource[ModelT any, AllModelT any](ctx context.Context, orders ...string) ([]*AllModelT, error) {

	businessId, ok := utils.GetBusinessIdFromContext(ctx)
	if !ok || businessId == "" {
		return nil, errors.New("business id is required")
	}

	// first try redis cache
	results, err := utils.RetrieveRedisList[AllModelT](businessId)
	if err != nil {
		return nil, err
	}
	// if not exists in redis
	if results == nil {
		// fetch from db
		db := config.GetDB()
		var model ModelT
		dbCtx := db.WithContext(ctx).Where("business_id = ?", businessId)
		for _, order := range orders {
			dbCtx.Order(order)
		}
		// db query
		if err = dbCtx.Model(&model).Find(&results).Error; err != nil {
			return nil, err
		}

		// caching the result
		if err := utils.StoreRedisList[AllModelT](results, businessId); err != nil {
			return nil, err
		}
	}

	return results, nil
}

func ToggleActiveModel[T RedisCleaner](ctx context.Context, businessId string, id int, isActive bool) (*T, error) {

	var result *T
	var err error
	db := config.GetDB()

	// fetch model before updating
	if businessId == "" {
		err = db.WithContext(ctx).First(&result, id).Error
	} else {
		err = db.WithContext(ctx).Where("business_id = ?", businessId).First(&result, id).Error
	}
	if err != nil {
		return nil, err
	}

	// update db
	tx := db.Begin()
	Tx := tx.WithContext(ctx).Model(&result).
		UpdateColumn("IsActive", isActive)
	if Tx.Error != nil {
		tx.Rollback()
		return nil, err
	}

	referenceType := Tx.Statement.Table
	var actionType string
	if isActive {
		actionType = "*ACTIVE*"
	} else {
		actionType = "*INACTIVE*"
	}

	// create history without hook
	if err := createHistory(tx.WithContext(ctx), actionType, id, referenceType, nil, nil, "toggled "+utils.GetTypeName[T]()); err != nil {
		tx.Rollback()
		return nil, err
	}

	// clear cache
	if err := RemoveRedisBoth(*result); err != nil {
		return nil, err
	}

	return result, tx.Commit().Error
}

// list all resources, redis or db, cache result
func ListAllAdmin[ModelT any, AllModelT any](ctx context.Context, fields ...string) ([]*AllModelT, error) {

	// first try redis cache
	results, err := utils.RetrieveRedisList[AllModelT]("")
	if err != nil {
		return nil, err
	}
	// if not exists in redis
	if results == nil {
		// fetch from db
		db := config.GetDB()
		var model ModelT
		dbCtx := db.WithContext(ctx).Model(&model)
		dbCtx.Select(fields)
		// db query
		if err = dbCtx.Scan(&results).Error; err != nil {
			return nil, err
		}

		// caching the result
		if err := utils.StoreRedisList[AllModelT](results, ""); err != nil {
			return nil, err
		}
	}

	return results, nil
}
