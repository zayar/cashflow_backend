package models

import (
	"context"
	"errors"
	"time"

	"bitbucket.org/mmdatafocus/books_backend/config"
	"bitbucket.org/mmdatafocus/books_backend/utils"
)

type PaymentMode struct {
	ID         int       `gorm:"primary_key" json:"id"`
	BusinessId string    `gorm:"primary_key;autoIncrement:false;not null" json:"business_id" binding:"required"`
	Name       string    `gorm:"size:100;not null" json:"name" binding:"required"`
	IsActive   *bool     `gorm:"not null;default:true" json:"is_active"`
	CreatedAt  time.Time `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt  time.Time `gorm:"autoUpdateTime" json:"updated_at"`
}

type NewPaymentMode struct {
	Name string `json:"name" binding:"required"`
}

// validate input for both create & update. (id = 0 for create)

func (input *NewPaymentMode) validate(ctx context.Context, businessId string, id int) error {
	// name
	if err := utils.ValidateUnique[PaymentMode](ctx, businessId, "name", input.Name, id); err != nil {
		return err
	}
	return nil
}

func CreatePaymentMode(ctx context.Context, input *NewPaymentMode) (*PaymentMode, error) {

	db := config.GetDB()
	businessId, ok := utils.GetBusinessIdFromContext(ctx)
	if !ok || businessId == "" {
		return nil, errors.New("business id is required")
	}

	if err := input.validate(ctx, businessId, 0); err != nil {
		return nil, err
	}

	paymentMode := PaymentMode{
		Name:       input.Name,
		BusinessId: businessId,
	}

	err := db.WithContext(ctx).Create(&paymentMode).Error
	if err != nil {
		return nil, err
	}

	return &paymentMode, nil
}

func UpdatePaymentMode(ctx context.Context, id int, input *NewPaymentMode) (*PaymentMode, error) {

	businessId, ok := utils.GetBusinessIdFromContext(ctx)
	if !ok || businessId == "" {
		return nil, errors.New("business id is required")
	}
	if err := input.validate(ctx, businessId, id); err != nil {
		return nil, err
	}

	paymentMode, err := utils.FetchModel[PaymentMode](ctx, businessId, id)
	if err != nil {
		return nil, err
	}

	db := config.GetDB()
	// db action
	err = db.WithContext(ctx).Model(&paymentMode).Updates(map[string]interface{}{
		"Name": input.Name,
	}).Error
	if err != nil {
		return nil, err
	}
	return paymentMode, nil
}

func DeletePaymentMode(ctx context.Context, id int) (*PaymentMode, error) {

	businessId, ok := utils.GetBusinessIdFromContext(ctx)
	if !ok || businessId == "" {
		return nil, errors.New("business id is required")
	}

	result, err := utils.FetchModel[PaymentMode](ctx, businessId, id)
	if err != nil {
		return nil, err
	}
	db := config.GetDB()
	err = db.WithContext(ctx).Delete(&result).Error
	if err != nil {
		return nil, err
	}
	return result, nil
}

func GetPaymentMode(ctx context.Context, id int) (*PaymentMode, error) {

	businessId, ok := utils.GetBusinessIdFromContext(ctx)
	if !ok || businessId == "" {
		return nil, errors.New("business id is required")
	}
	return utils.FetchModel[PaymentMode](ctx, businessId, id)
}

func GetPaymentModes(ctx context.Context, name *string) ([]*PaymentMode, error) {

	db := config.GetDB()
	var results []*PaymentMode

	fieldNames, err := utils.GetQueryFields(ctx, &PaymentMode{})
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

func ToggleActivePaymentMode(ctx context.Context, id int, isActive bool) (*PaymentMode, error) {
	// <owner>
	businessId, ok := utils.GetBusinessIdFromContext(ctx)
	if !ok || businessId == "" {
		return nil, errors.New("business id is required")
	}

	return ToggleActiveModel[PaymentMode](ctx, businessId, id, isActive)
}
