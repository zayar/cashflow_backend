package models

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/mmdatafocus/books_backend/config"
	"github.com/mmdatafocus/books_backend/utils"
)

type Warehouse struct {
	ID         int       `gorm:"primary_key" json:"id"`
	BusinessId string    `gorm:"index;not null" json:"business_id"`
	BranchId   int       `gorm:"not null" json:"branch_id"`
	Name       string    `gorm:"size:100;not null" json:"name" binding:"required"`
	Phone      string    `gorm:"size:20" json:"phone"`
	Mobile     string    `gorm:"size:20" json:"mobile"`
	Address    string    `gorm:"type:text" json:"address"`
	Country    string    `gorm:"size:100"  json:"country"`
	City       string    `gorm:"size:100"  json:"city"`
	StateId    int       `gorm:"index" json:"state_id"`
	TownshipId int       `gorm:"index" json:"township_id"`
	IsActive   *bool     `gorm:"not null;default:true" json:"is_active"`
	CreatedAt  time.Time `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt  time.Time `gorm:"autoUpdateTime" json:"updated_at"`
}

type NewWarehouse struct {
	BranchId   int    `json:"branch_id" binding:"required"`
	Name       string `json:"name" binding:"required"`
	Phone      string `json:"phone"`
	Mobile     string `json:"mobile"`
	Address    string `json:"address"`
	Country    string `json:"country"`
	City       string `json:"city"`
	StateId    int    `json:"state_id"`
	TownshipId int    `json:"township_id"`
}

// validate input for both create & update. (id = 0 for create)

func (input *NewWarehouse) validate(ctx context.Context, businessId string, id int) error {
	// name
	if err := utils.ValidateUnique[Warehouse](ctx, businessId, "name", input.Name, id); err != nil {
		return err
	}
	// check if branch is not owned by other business
	if err := utils.ValidateResourceId[Branch](ctx, businessId, input.BranchId); err != nil {
		return errors.New("branch not found")
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
	return nil
}

func CreateWarehouse(ctx context.Context, input *NewWarehouse) (*Warehouse, error) {

	// <custom>
	businessId, ok := utils.GetBusinessIdFromContext(ctx)
	if !ok || businessId == "" {
		return nil, errors.New("business id is required")
	}

	if err := input.validate(ctx, businessId, 0); err != nil {
		return nil, err
	}

	warehouse := Warehouse{
		BusinessId: businessId,
		BranchId:   input.BranchId,
		Name:       input.Name,
		Phone:      input.Phone,
		Mobile:     input.Mobile,
		Address:    input.Address,
		Country:    input.Country,
		City:       input.City,
		StateId:    input.StateId,
		TownshipId: input.TownshipId,
		IsActive:   utils.NewTrue(),
	}

	// db action
	db := config.GetDB()
	err := db.WithContext(ctx).Create(&warehouse).Error
	if err != nil {
		return nil, err
	}
	return &warehouse, nil
}

func UpdateWarehouse(ctx context.Context, id int, input *NewWarehouse) (*Warehouse, error) {

	businessId, ok := utils.GetBusinessIdFromContext(ctx)
	if !ok || businessId == "" {
		return nil, errors.New("business id is required")
	}

	if err := input.validate(ctx, businessId, id); err != nil {
		return nil, err
	}

	warehouse, err := utils.FetchModel[Warehouse](ctx, businessId, id)
	if err != nil {
		return nil, err
	}

	// db action
	db := config.GetDB()
	err = db.WithContext(ctx).Model(&warehouse).Updates(map[string]interface{}{
		"BranchId":   input.BranchId,
		"Name":       input.Name,
		"Phone":      input.Phone,
		"Mobile":     input.Mobile,
		"Address":    input.Address,
		"Country":    input.Country,
		"City":       input.City,
		"StateId":    input.StateId,
		"TownshipId": input.TownshipId,
	}).Error
	if err != nil {
		return nil, err
	}

	return warehouse, nil
}

func DeleteWarehouse(ctx context.Context, id int) (*Warehouse, error) {

	businessId, ok := utils.GetBusinessIdFromContext(ctx)
	if !ok || businessId == "" {
		return nil, errors.New("business id is required")
	}

	db := config.GetDB()
	result, err := utils.FetchModel[Warehouse](ctx, businessId, id)
	if err != nil {
		return nil, err
	}

	// check if warehouse is used
	var count int64
	if err := db.WithContext(ctx).Model(&StockSummary{}).
		Where("warehouse_id = ?", id).Count(&count).Error; err != nil {
		return nil, err
	}
	if count > 0 {
		return nil, errors.New("warehouse has stock")
	}

	// db action
	err = db.WithContext(ctx).Delete(&result).Error
	if err != nil {
		return nil, err
	}
	return result, nil
}

func GetWarehouse(ctx context.Context, id int) (*Warehouse, error) {
	return GetResource[Warehouse](ctx, id)
}

func ListWarehouse(ctx context.Context, name *string) ([]*Warehouse, error) {
	businessId, ok := utils.GetBusinessIdFromContext(ctx)
	if !ok || businessId == "" {
		return nil, errors.New("business id is required")
	}

	db := config.GetDB()
	var results []*Warehouse

	fieldNames, err := utils.GetQueryFields(ctx, &Warehouse{})
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

func ToggleActiveWarehouse(ctx context.Context, id int, isActive bool) (*Warehouse, error) {
	businessId, ok := utils.GetBusinessIdFromContext(ctx)
	if !ok || businessId == "" {
		return nil, errors.New("business id is required")
	}
	return ToggleActiveModel[Warehouse](ctx, businessId, id, isActive)
}
