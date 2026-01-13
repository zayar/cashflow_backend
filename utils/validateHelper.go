package utils

import (
	"context"
	"errors"
	"reflect"

	"bitbucket.org/mmdatafocus/books_backend/config"
)

// check if id exists, using ctx's business_id in WHERE, return RecordNOtFound Error
func ValidateResourceId[T any](ctx context.Context, businessId string, id interface{}) error {

	count, err := ResourceCountWhere[T](ctx, businessId, "id = ?", id)
	if err != nil {
		return err
	}
	if count <= 0 {
		return ErrorRecordNotFound
	}

	return nil
}

type ValidationRule[ID comparable] struct {
	Model   interface{}
	Ids     []ID
	Message string
	Filter  Filter
}

type Filter struct {
	Cond   string
	Values []interface{}
}

func MassValidateResourceIds[ID comparable](ctx context.Context, rules []ValidationRule[ID]) error {
	db := config.GetDB()
	var count int64
	for _, rule := range rules {
		if len(rule.Ids) <= 0 {
			continue
		}

		unqIds := UniqueSlice(rule.Ids)

		err := db.WithContext(ctx).Model(&rule.Model).
			Where("id IN ?", unqIds).
			Where(rule.Filter.Cond, rule.Filter.Values...).
			Count(&count).Error
		if err != nil {
			return err
		}
		if count != int64(len(unqIds)) {
			return errors.New(rule.Message)
		}
	}

	return nil
}

// check if ALL id exists, using ctx's business_id in WHERE, return RecordNOtFound Error
func ValidateResourcesId[M any, ID comparable](ctx context.Context, businessId string, ids []ID) error {
	unqIds := UniqueSlice(ids)

	count, err := ResourceCountWhere[M](ctx, businessId, "id IN ?", unqIds)
	if err != nil {
		return err
	}
	if count != int64(len(unqIds)) {
		return ErrorRecordNotFound
	}

	return nil
}

func ValidateUnique[T any](ctx context.Context, businessId string, column string, value interface{}, exceptId interface{}) error {
	var count int64
	var err error
	if reflect.ValueOf(exceptId).IsZero() {
		count, err = ResourceCountWhere[T](ctx, businessId, column+" = ?", value)
	} else {
		count, err = ResourceCountWhere[T](ctx, businessId, column+" = ? AND NOT id = ?", value, exceptId)
	}

	if err != nil {
		return err
	}
	if count > 0 {
		return errors.New("duplicate " + column)
	}
	return nil
}

// count records, using WHERE business_id = ? AND $condition
// business_id can be blank for admin user
func ResourceCountWhere[T any](ctx context.Context, businessId string, condition string, value ...interface{}) (int64, error) {
	var model T

	db := config.GetDB()
	dbCtx := db.WithContext(ctx).Model(&model)
	var count int64
	if businessId != "" {
		dbCtx.Where("business_id = ?", businessId)
	}
	dbCtx.Where(condition, value...)
	if err := dbCtx.Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}
