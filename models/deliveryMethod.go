package models

import (
	"context"
	"errors"
	"time"

	"bitbucket.org/mmdatafocus/books_backend/config"
	"bitbucket.org/mmdatafocus/books_backend/utils"
)

type DeliveryMethod struct {
	ID         int       `gorm:"primary_key" json:"id"`
	BusinessId string    `gorm:"primary_key;autoIncrement:false;not null" json:"business_id" binding:"required"`
	Name       string    `gorm:"size:100;not null" json:"name" binding:"required"`
	IsActive   *bool     `gorm:"not null;default:true" json:"is_active"`
	CreatedAt  time.Time `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt  time.Time `gorm:"autoUpdateTime" json:"updated_at"`
}

type NewDeliveryMethod struct {
	Name string `json:"name" binding:"required"`
}

// validate input for both create & update. (id = 0 for create)

func (input *NewDeliveryMethod) validate(ctx context.Context, businessId string, id int) error {
	if id > 0 {
		if err := utils.ValidateResourceId[DeliveryMethod](ctx, businessId, id); err != nil {
			return err
		}
	}
	// name
	if err := utils.ValidateUnique[DeliveryMethod](ctx, businessId, "name", input.Name, id); err != nil {
		return err
	}
	return nil
}

func CreateDeliveryMethod(ctx context.Context, input *NewDeliveryMethod) (*DeliveryMethod, error) {

	businessId, ok := utils.GetBusinessIdFromContext(ctx)
	if !ok || businessId == "" {
		return nil, errors.New("business id is required")
	}

	if err := input.validate(ctx, businessId, 0); err != nil {
		return nil, err
	}

	deliveryMethod := DeliveryMethod{
		Name:       input.Name,
		BusinessId: businessId,
		IsActive:   utils.NewTrue(),
	}

	// db action
	db := config.GetDB()
	err := db.WithContext(ctx).Create(&deliveryMethod).Error
	if err != nil {
		return nil, err
	}

	return &deliveryMethod, nil
}

func UpdateDeliveryMethod(ctx context.Context, id int, input *NewDeliveryMethod) (*DeliveryMethod, error) {

	businessId, ok := utils.GetBusinessIdFromContext(ctx)
	if !ok || businessId == "" {
		return nil, errors.New("business id is required")
	}

	if err := input.validate(ctx, businessId, id); err != nil {
		return nil, err
	}

	deliveryMethod, err := utils.FetchModel[DeliveryMethod](ctx, businessId, id)
	if err != nil {
		return nil, err
	}

	// db action
	db := config.GetDB()
	err = db.WithContext(ctx).Model(&deliveryMethod).Updates(map[string]interface{}{
		"Name": input.Name,
	}).Error
	if err != nil {
		return nil, err
	}
	return deliveryMethod, nil
}

func DeleteDeliveryMethod(ctx context.Context, id int) (*DeliveryMethod, error) {

	businessId, ok := utils.GetBusinessIdFromContext(ctx)
	if !ok || businessId == "" {
		return nil, errors.New("business id is required")
	}
	db := config.GetDB()

	result, err := utils.FetchModel[DeliveryMethod](ctx, businessId, id)
	if err != nil {
		return nil, err
	}

	// db action
	err = db.WithContext(ctx).Delete(&result).Error
	if err != nil {
		return nil, err
	}

	return result, nil
}

func GetDeliveryMethod(ctx context.Context, id int) (*DeliveryMethod, error) {

	businessId, ok := utils.GetBusinessIdFromContext(ctx)
	if !ok || businessId == "" {
		return nil, errors.New("business id is required")
	}
	return utils.FetchModel[DeliveryMethod](ctx, businessId, id)
}

func GetDeliveryMethods(ctx context.Context, name *string) ([]*DeliveryMethod, error) {

	db := config.GetDB()
	var results []*DeliveryMethod

	fieldNames, err := utils.GetQueryFields(ctx, &DeliveryMethod{})
	if err != nil {
		return nil, err
	}

	businessId, ok := utils.GetBusinessIdFromContext(ctx)
	if !ok || businessId == "" {
		return nil, errors.New("business id is required")
	}

	dbCtx := db.WithContext(ctx).Where("business_id = ?", businessId)
	if name != nil && len(*name) > 0 {
		dbCtx = dbCtx.Where("name LIKE ?", "%"+*name+"%")
	}
	err = dbCtx.Select(fieldNames).Order("name").Find(&results).Error
	if err != nil {
		return nil, err
	}
	return results, nil
}

func ToggleActiveDeliveryMethod(ctx context.Context, id int, isActive bool) (*DeliveryMethod, error) {
	businessId, ok := utils.GetBusinessIdFromContext(ctx)
	if !ok || businessId == "" {
		return nil, errors.New("business id is required")
	}

	return ToggleActiveModel[DeliveryMethod](ctx, businessId, id, isActive)
}
