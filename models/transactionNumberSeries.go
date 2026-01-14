package models

import (
	"context"
	"errors"
	"time"

	"github.com/mmdatafocus/books_backend/config"
	"github.com/mmdatafocus/books_backend/utils"
	"gorm.io/gorm"
)

type TransactionNumberSeries struct {
	ID         int                             `gorm:"primary_key" json:"id"`
	BusinessId string                          `gorm:"index;not null" json:"business_id" binding:"required"`
	Name       string                          `gorm:"size:100;not null" json:"name" binding:"required"`
	Modules    []TransactionNumberSeriesModule `gorm:"foreignKey:SeriesId" json:"modules"`
	CreatedAt  time.Time                       `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt  time.Time                       `gorm:"autoUpdateTime" json:"updated_at"`
}

type TransactionNumberSeriesModule struct {
	SeriesId   int    `gorm:"primaryKey;autoIncrement:false" json:"series_id" binding:"required"`
	ModuleName string `gorm:"primaryKey;autoIncrement:false" json:"module_name" binding:"required"`
	Prefix     string `gorm:"size:10" json:"prefix"`
}

type NewTransactionNumberSeries struct {
	Name    string                             `json:"name" binding:"required"`
	Modules []NewTransactionNumberSeriesModule `json:"modules"`
}

type NewTransactionNumberSeriesModule struct {
	ModuleName string `json:"module_name" binding:"required"`
	Prefix     string `json:"prefix"`
}

// validate input for both create & update. (id = 0 for create)

func (input *NewTransactionNumberSeries) validate(ctx context.Context, businessId string, id int) error {
	// name
	if err := utils.ValidateUnique[TransactionNumberSeries](ctx, businessId, "name", input.Name, id); err != nil {
		return err
	}
	return nil
}

func (series TransactionNumberSeries) getBranchIds(ctx context.Context) ([]int, error) {
	var branchIds []int
	db := config.GetDB()
	if err := db.WithContext(ctx).Model(&Branch{}).Where("transaction_number_series_id = ?", series.ID).Select("id").Scan(&branchIds).Error; err != nil {
		return nil, err
	}
	return branchIds, nil
}

func mapTransactionNumberSeriesModule(input []NewTransactionNumberSeriesModule) ([]TransactionNumberSeriesModule, error) {
	modules := make([]TransactionNumberSeriesModule, 0)
	for _, m := range input {
		modules = append(modules, TransactionNumberSeriesModule{
			ModuleName: m.ModuleName,
			Prefix:     m.Prefix,
		})
	}

	return modules, nil
}

func CreateTransactionNumberSeries(ctx context.Context, input *NewTransactionNumberSeries) (*TransactionNumberSeries, error) {

	businessId, ok := utils.GetBusinessIdFromContext(ctx)
	if !ok || businessId == "" {
		return nil, errors.New("business id is required")
	}

	// validate name
	if err := input.validate(ctx, businessId, 0); err != nil {
		return nil, err
	}

	modules, err := mapTransactionNumberSeriesModule(input.Modules)
	if err != nil {
		return nil, err
	}

	series := TransactionNumberSeries{
		BusinessId: businessId,
		Name:       input.Name,
		Modules:    modules,
	}

	db := config.GetDB()
	// db action
	err = db.WithContext(ctx).Create(&series).Error
	if err != nil {
		return nil, err
	}
	return &series, nil
}

func UpdateTransactionNumberSeries(ctx context.Context, id int, input *NewTransactionNumberSeries) (*TransactionNumberSeries, error) {
	businessId, ok := utils.GetBusinessIdFromContext(ctx)
	if !ok || businessId == "" {
		return nil, errors.New("business id is required")
	}

	if err := input.validate(ctx, businessId, id); err != nil {
		return nil, err
	}

	modules, err := mapTransactionNumberSeriesModule(input.Modules)
	if err != nil {
		return nil, err
	}

	series, err := utils.FetchModel[TransactionNumberSeries](ctx, businessId, id)
	if err != nil {
		return nil, err
	}

	db := config.GetDB()
	tx := db.Begin()
	// db action
	if err = tx.WithContext(ctx).Model(&series).
		Updates(map[string]interface{}{
			"Name": input.Name,
		}).Error; err != nil {
		tx.Rollback()
		return nil, err
	}
	if err = tx.WithContext(ctx).Model(&series).
		Session(&gorm.Session{FullSaveAssociations: true, SkipHooks: true}).
		Association("Modules").
		Unscoped().Replace(&modules); err != nil {
		tx.Rollback()
		return nil, err
	}
	// save upsert history

	// using hooks to clear redis cache by getting branch ids related to the series

	if err := tx.Commit().Error; err != nil {
		return nil, err
	}

	return series, nil
}

func DeleteTransactionNumberSeries(ctx context.Context, id int) (*TransactionNumberSeries, error) {
	businessId, ok := utils.GetBusinessIdFromContext(ctx)
	if !ok || businessId == "" {
		return nil, errors.New("business id is required")
	}

	db := config.GetDB()
	result, err := utils.FetchModel[TransactionNumberSeries](ctx, businessId, id, "Modules")
	if err != nil {
		return nil, err
	}

	// Do not delete if any Branch use this series
	var count int64
	if err = db.WithContext(ctx).Model(&Branch{}).
		Where("transaction_number_series_id = ?", result.ID).Count(&count).Error; err != nil {
		return nil, err
	}
	if count > 0 {
		return nil, errors.New("used by branch")
	}

	// db action
	err = db.WithContext(ctx).Select("Modules").Delete(&result).Error
	if err != nil {
		return nil, err
	}
	return result, nil
}

func GetTransactionNumberSeries(ctx context.Context, id int) (*TransactionNumberSeries, error) {
	return GetResource[TransactionNumberSeries](ctx, id, "Modules")
}

func GetTransactionNumberSeriesAll(ctx context.Context, name *string) ([]*TransactionNumberSeries, error) {
	businessId, ok := utils.GetBusinessIdFromContext(ctx)
	if !ok || businessId == "" {
		return nil, errors.New("business id is required")
	}

	db := config.GetDB()
	var results []*TransactionNumberSeries

	dbCtx := db.WithContext(ctx).Where("business_id = ?", businessId)
	if name != nil && len(*name) > 0 {
		dbCtx = dbCtx.Where("name LIKE ?", "%"+*name+"%")
	}
	// db query
	err := dbCtx.Preload("Modules").Order("name").Find(&results).Error
	if err != nil {
		return nil, err
	}
	return results, nil
}
