package models

import (
	"context"
	"errors"
	"time"

	"github.com/mmdatafocus/books_backend/config"
	"github.com/mmdatafocus/books_backend/utils"
)

type Reason struct {
	ID         int       `gorm:"primary_key" json:"id"`
	BusinessId string    `gorm:"primary_key;autoIncrement:false;not null" json:"business_id" binding:"required"`
	Name       string    `gorm:"size:100;not null" json:"name" binding:"required"`
	IsActive   *bool     `gorm:"not null;default:true" json:"is_active"`
	CreatedAt  time.Time `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt  time.Time `gorm:"autoUpdateTime" json:"updated_at"`
}

type NewReason struct {
	Name string `json:"name" binding:"required"`
}

func (input *NewReason) validate(ctx context.Context, businessId string, id int) error {
	if err := utils.ValidateUnique[Reason](ctx, businessId, "name", input.Name, id); err != nil {
		return err
	}
	return nil
}

func CreateReason(ctx context.Context, input *NewReason) (*Reason, error) {
	businessId, ok := utils.GetBusinessIdFromContext(ctx)
	if !ok || businessId == "" {
		return nil, errors.New("business id is required")
	}

	if err := input.validate(ctx, businessId, 0); err != nil {
		return nil, err
	}

	db := config.GetDB()
	reason := Reason{
		BusinessId: businessId,
		Name:       input.Name,
		IsActive:   utils.NewTrue(),
	}
	if err := db.WithContext(ctx).Create(&reason).Error; err != nil {
		return nil, err
	}

	return &reason, nil
}

func UpdateReason(ctx context.Context, id int, input *NewReason) (*Reason, error) {

	businessId, ok := utils.GetBusinessIdFromContext(ctx)
	if !ok || businessId == "" {
		return nil, errors.New("business id is required")
	}

	db := config.GetDB()
	reason, err := utils.FetchModel[Reason](ctx, businessId, id)
	if err != nil {
		return nil, err
	}
	if err := input.validate(ctx, businessId, id); err != nil {
		return nil, err
	}
	if err := db.WithContext(ctx).Model(&reason).Updates(map[string]interface{}{
		"Name": input.Name,
	}).Error; err != nil {
		return nil, err
	}

	return reason, nil
}

func DeleteReason(ctx context.Context, id int) (*Reason, error) {

	businessId, ok := utils.GetBusinessIdFromContext(ctx)
	if !ok || businessId == "" {
		return nil, errors.New("business id is required")
	}

	db := config.GetDB()
	reason, err := utils.FetchModel[Reason](ctx, businessId, id)
	if err != nil {
		return nil, err
	}
	if err = db.WithContext(ctx).Delete(&reason).Error; err != nil {
		return nil, err
	}
	return reason, nil
}

func ToggleActiveReason(ctx context.Context, id int, isActive bool) (*Reason, error) {
	businessId, ok := utils.GetBusinessIdFromContext(ctx)
	if !ok || businessId == "" {
		return nil, errors.New("business id is required")
	}

	return ToggleActiveModel[Reason](ctx, businessId, id, isActive)
}

func GetReason(ctx context.Context, id int) (*Reason, error) {
	businessId, ok := utils.GetBusinessIdFromContext(ctx)
	if !ok || businessId == "" {
		return nil, errors.New("business id is required")
	}

	reason, err := utils.FetchModel[Reason](ctx, businessId, id)
	if err != nil {
		return nil, err
	}
	return reason, nil

}
