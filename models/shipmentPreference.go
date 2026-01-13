package models

import (
	"context"
	"errors"
	"time"

	"bitbucket.org/mmdatafocus/books_backend/config"
	"bitbucket.org/mmdatafocus/books_backend/utils"
)

type ShipmentPreference struct {
	ID         int       `gorm:"primary_key" json:"id"`
	BusinessId string    `gorm:"primary_key;autoIncrement:false;not null" json:"business_id" binding:"required"`
	Name       string    `gorm:"size:100;not null" json:"name" binding:"required"`
	IsActive   *bool     `gorm:"not null;default:true" json:"is_active"`
	CreatedAt  time.Time `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt  time.Time `gorm:"autoUpdateTime" json:"updated_at"`
}

type NewShipmentPreference struct {
	Name string `json:"name" binding:"required"`
}

// validate input for both create & update. (id = 0 for create)

func (input *NewShipmentPreference) validate(ctx context.Context, businessId string, id int) error {
	// name
	if err := utils.ValidateUnique[ShipmentPreference](ctx, businessId, "name", input.Name, id); err != nil {
		return err
	}
	return nil
}

func CreateShipmentPreference(ctx context.Context, input *NewShipmentPreference) (*ShipmentPreference, error) {

	db := config.GetDB()

	// validate name
	businessId, ok := utils.GetBusinessIdFromContext(ctx)
	if !ok || businessId == "" {
		return nil, errors.New("business id is required")
	}

	if err := input.validate(ctx, businessId, 0); err != nil {
		return nil, err
	}

	shipmentPreference := ShipmentPreference{
		Name:       input.Name,
		BusinessId: businessId,
	}

	err := db.WithContext(ctx).Create(&shipmentPreference).Error
	if err != nil {
		return nil, err
	}
	return &shipmentPreference, nil
}

func UpdateShipmentPreference(ctx context.Context, id int, input *NewShipmentPreference) (*ShipmentPreference, error) {

	db := config.GetDB()

	businessId, ok := utils.GetBusinessIdFromContext(ctx)
	if !ok || businessId == "" {
		return nil, errors.New("business id is required")
	}

	shipmentPreference, err := utils.FetchModel[ShipmentPreference](ctx, businessId, id)
	if err != nil {
		return nil, err
	}

	if err := input.validate(ctx, businessId, id); err != nil {
		return nil, err
	}

	err = db.WithContext(ctx).Model(&shipmentPreference).Updates(map[string]interface{}{
		"Name": input.Name,
	}).Error
	if err != nil {
		return nil, err
	}
	return shipmentPreference, nil
}

func DeleteShipmentPreference(ctx context.Context, id int) (*ShipmentPreference, error) {

	db := config.GetDB()
	businessId, ok := utils.GetBusinessIdFromContext(ctx)
	if !ok || businessId == "" {
		return nil, errors.New("business id is required")
	}
	result, err := utils.FetchModel[ShipmentPreference](ctx, businessId, id)
	if err != nil {
		return nil, err
	}
	err = db.WithContext(ctx).Delete(&result).Error
	if err != nil {
		return nil, err
	}
	return result, nil
}

func GetShipmentPreference(ctx context.Context, id int) (*ShipmentPreference, error) {

	businessId, ok := utils.GetBusinessIdFromContext(ctx)
	if !ok || businessId == "" {
		return nil, errors.New("business id is required")
	}
	return utils.FetchModel[ShipmentPreference](ctx, businessId, id)
}

func GetShipmentPreferences(ctx context.Context, name *string) ([]*ShipmentPreference, error) {

	db := config.GetDB()
	var results []*ShipmentPreference

	fieldNames, err := utils.GetQueryFields(ctx, &ShipmentPreference{})
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

func ToggleActiveShipmentPreference(ctx context.Context, id int, isActive bool) (*ShipmentPreference, error) {
	// <owner>
	businessId, ok := utils.GetBusinessIdFromContext(ctx)
	if !ok || businessId == "" {
		return nil, errors.New("business id is required")
	}

	return ToggleActiveModel[ShipmentPreference](ctx, businessId, id, isActive)
}
