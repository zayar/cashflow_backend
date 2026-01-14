package models

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/mmdatafocus/books_backend/config"
	"github.com/mmdatafocus/books_backend/utils"
)

type Branch struct {
	ID                        int       `gorm:"primary_key" json:"id"`
	BusinessId                string    `gorm:"index;not null" json:"business_id"`
	TransactionNumberSeriesId int       `gorm:"index;not null" json:"transaction_number_series_id"`
	Name                      string    `gorm:"index;size:100;not null" json:"name" binding:"required"`
	Phone                     string    `gorm:"size:20" json:"phone"`
	Mobile                    string    `gorm:"size:20" json:"mobile"`
	Address                   string    `gorm:"type:text" json:"address"`
	Country                   string    `gorm:"size:100"  json:"country"`
	City                      string    `gorm:"size:100"  json:"city"`
	StateId                   int       `gorm:"index" json:"state_id"`
	TownshipId                int       `gorm:"index" json:"township_id"`
	IsActive                  *bool     `gorm:"not null;default:true" json:"is_active"`
	CreatedAt                 time.Time `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt                 time.Time `gorm:"autoUpdateTime" json:"updated_at"`
}

type NewBranch struct {
	TransactionNumberSeriesId int    `json:"transaction_number_series_id" binding:"required"`
	Name                      string `json:"name" binding:"required"`
	Phone                     string `json:"phone"`
	Mobile                    string `json:"mobile"`
	Address                   string `json:"address"`
	Country                   string `json:"country"`
	City                      string `json:"city"`
	StateId                   int    `json:"state_id"`
	TownshipId                int    `json:"township_id"`
}

// validate input for both create & update. (id = 0 for create)

func (input *NewBranch) validate(ctx context.Context, businessId string, id int) error {
	if id > 0 {
		if err := utils.ValidateResourceId[Branch](ctx, businessId, id); err != nil {
			return err
		}
	}
	// name
	if err := utils.ValidateUnique[Branch](ctx, businessId, "name", input.Name, id); err != nil {
		return err
	}
	// phone
	if len(strings.TrimSpace(input.Phone)) > 0 {
		if err := utils.ValidateUnique[Branch](ctx, businessId, "phone", input.Phone, id); err != nil {
			return err
		}
	}
	// mobile
	if len(strings.TrimSpace(input.Mobile)) > 0 {
		if err := utils.ValidateUnique[Branch](ctx, businessId, "mobile", input.Mobile, id); err != nil {
			return err
		}
	}
	if err := utils.ValidateResourceId[TransactionNumberSeries](ctx, businessId, input.TransactionNumberSeriesId); err != nil {
		return errors.New("transactionNumberSeries not found")
	}
	return nil
}

func CreateBranch(ctx context.Context, input *NewBranch) (*Branch, error) {

	businessId, ok := utils.GetBusinessIdFromContext(ctx)
	if !ok || businessId == "" {
		return nil, errors.New("business id is required")
	}

	if err := input.validate(ctx, businessId, 0); err != nil {
		return nil, err
	}

	branch := Branch{
		BusinessId:                businessId,
		TransactionNumberSeriesId: input.TransactionNumberSeriesId,
		Name:                      input.Name,
		Phone:                     input.Phone,
		Mobile:                    input.Mobile,
		Address:                   input.Address,
		Country:                   input.Country,
		City:                      input.City,
		StateId:                   input.StateId,
		TownshipId:                input.TownshipId,
		IsActive:                  utils.NewTrue(),
	}

	// db action
	db := config.GetDB()
	err := db.WithContext(ctx).Create(&branch).Error
	if err != nil {
		return nil, err
	}

	return &branch, nil
}

func UpdateBranch(ctx context.Context, id int, input *NewBranch) (*Branch, error) {

	businessId, ok := utils.GetBusinessIdFromContext(ctx)
	if !ok || businessId == "" {
		return nil, errors.New("business id is required")
	}

	if err := input.validate(ctx, businessId, id); err != nil {
		return nil, err
	}

	branch, err := utils.FetchModel[Branch](ctx, businessId, id)
	if err != nil {
		return nil, err
	}

	// db action
	db := config.GetDB()
	err = db.WithContext(ctx).Model(&branch).Updates(map[string]interface{}{
		"TransactionNumberSeriesId": input.TransactionNumberSeriesId,
		"Name":                      input.Name,
		"Phone":                     input.Phone,
		"Mobile":                    input.Mobile,
		"Address":                   input.Address,
		"Country":                   input.Country,
		"City":                      input.City,
		"StateId":                   input.StateId,
		"TownshipId":                input.TownshipId,
	}).Error
	if err != nil {
		return nil, err
	}

	return branch, nil
}

func DeleteBranch(ctx context.Context, id int) (*Branch, error) {

	businessId, ok := utils.GetBusinessIdFromContext(ctx)
	if !ok || businessId == "" {
		return nil, errors.New("business id is required")
	}

	result, err := utils.FetchModel[Branch](ctx, businessId, id)
	if err != nil {
		return nil, err
	}

	// check if the branch is used
	db := config.GetDB()
	var count int64
	if err := db.WithContext(ctx).Model(&Business{}).
		Where("id = ? AND primary_branch_id = ?", businessId, id).Count(&count).Error; err != nil {
		return nil, err
	}
	if count > 0 {
		return nil, errors.New("cannot delete primary branch")
	}
	if err := db.WithContext(ctx).Model(&AccountTransaction{}).
		Where("branch_id = ?", id).Count(&count).Error; err != nil {
		return nil, err
	}
	if count > 0 {
		return nil, errors.New("branch has transactions")
	}

	// db action
	err = db.WithContext(ctx).Delete(&result).Error
	if err != nil {
		return nil, err
	}

	return result, nil
}

func GetBranch(ctx context.Context, id int) (*Branch, error) {

	return GetResource[Branch](ctx, id)
}

func GetBranches(ctx context.Context, name *string) ([]*Branch, error) {
	businessId, ok := utils.GetBusinessIdFromContext(ctx)
	if !ok || businessId == "" {
		return nil, errors.New("business id is required")
	}

	db := config.GetDB()
	var results []*Branch

	fieldNames, err := utils.GetQueryFields(ctx, &Branch{})
	if err != nil {
		return nil, err
	}

	dbCtx := db.WithContext(ctx).Where("business_id = ?", businessId)
	if name != nil && len(*name) > 0 {
		dbCtx = dbCtx.Where("name LIKE ?", "%"+*name+"%")
	}
	// db query
	err = dbCtx.Select(fieldNames).Order("name").Find(&results).Error
	if err != nil {
		return nil, err
	}
	return results, nil
}

func ToggleActiveBranch(ctx context.Context, id int, isActive bool) (*Branch, error) {
	// <owner>
	businessId, ok := utils.GetBusinessIdFromContext(ctx)
	if !ok || businessId == "" {
		return nil, errors.New("business id is required")
	}
	if !isActive {
		db := config.GetDB()
		var count int64
		if err := db.WithContext(ctx).Model(&Business{}).
			Where("id = ? AND primary_branch_id = ?", businessId, id).Count(&count).Error; err != nil {
			return nil, err
		}
		if count > 0 {
			return nil, errors.New("cannot toggle primary branch inactive")
		}
	}
	return ToggleActiveModel[Branch](ctx, businessId, id, isActive)
}
