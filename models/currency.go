package models

import (
	"context"
	"errors"
	"time"

	"github.com/mmdatafocus/books_backend/config"
	"github.com/mmdatafocus/books_backend/utils"
)

type Currency struct {
	ID            int           `gorm:"primary_key" json:"id"`
	BusinessId    string        `gorm:"index;not null" json:"business_id" binding:"required"`
	Symbol        string        `gorm:"index;size:3;not null" json:"symbol" binding:"required"`
	Name          string        `gorm:"index;size:100;not null" json:"name" binding:"required"`
	DecimalPlaces DecimalPlaces `gorm:"type:enum('0','2','3');default:'0';size:1;not null" json:"decimal_places" binding:"required"`
	IsActive      *bool         `gorm:"not null;default:true" json:"is_active"`
	CreatedAt     time.Time     `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt     time.Time     `gorm:"autoUpdateTime" json:"updated_at"`
}

type NewCurrency struct {
	Symbol        string        `json:"symbol" binding:"required"`
	Name          string        `json:"name" binding:"required"`
	DecimalPlaces DecimalPlaces `json:"decimal_places" binding:"required"`
}

// validate input for both create & update. (id = 0 for create)

func (input *NewCurrency) validate(ctx context.Context, businessId string, id int) error {
	if id > 0 {
		if err := utils.ValidateResourceId[Currency](ctx, businessId, id); err != nil {
			return err
		}
	}
	// name
	if err := utils.ValidateUnique[Currency](ctx, businessId, "name", input.Name, id); err != nil {
		return err
	}
	// symbol
	if err := utils.ValidateUnique[Currency](ctx, businessId, "symbol", input.Symbol, id); err != nil {
		return err
	}
	return nil
}

func CreateCurrency(ctx context.Context, input *NewCurrency) (*Currency, error) {

	businessId, ok := utils.GetBusinessIdFromContext(ctx)
	if !ok || businessId == "" {
		return nil, errors.New("business id is required")
	}

	if err := input.validate(ctx, businessId, 0); err != nil {
		return nil, err
	}

	currency := Currency{
		BusinessId:    businessId,
		Symbol:        input.Symbol,
		Name:          input.Name,
		DecimalPlaces: input.DecimalPlaces,
		IsActive:      utils.NewTrue(),
	}

	// db action
	db := config.GetDB()
	err := db.WithContext(ctx).Create(&currency).Error
	if err != nil {
		return nil, err
	}
	return &currency, nil
}

func UpdateCurrency(ctx context.Context, id int, input *NewCurrency) (*Currency, error) {

	businessId, ok := utils.GetBusinessIdFromContext(ctx)
	if !ok || businessId == "" {
		return nil, errors.New("business id is required")
	}

	if err := input.validate(ctx, businessId, id); err != nil {
		return nil, err
	}

	currency, err := utils.FetchModel[Currency](ctx, businessId, id)
	if err != nil {
		return nil, err
	}

	// db action
	db := config.GetDB()
	err = db.WithContext(ctx).Model(&currency).Updates(map[string]interface{}{
		"Name":          input.Name,
		"Symbol":        input.Symbol,
		"DecimalPlaces": input.DecimalPlaces,
	}).Error
	if err != nil {
		return nil, err
	}

	return currency, nil
}

func DeleteCurrency(ctx context.Context, id int) (*Currency, error) {

	businessId, ok := utils.GetBusinessIdFromContext(ctx)
	if !ok || businessId == "" {
		return nil, errors.New("business id is required")
	}

	result, err := utils.FetchModel[Currency](ctx, businessId, id)
	if err != nil {
		return nil, err
	}

	db := config.GetDB()
	// check if the currency is used
	var count int64
	if err := db.WithContext(ctx).Model(&Business{}).
		Where(&Business{BaseCurrencyId: id}).Count(&count).Error; err != nil {
		return nil, err
	}
	if count > 0 {
		return nil, errors.New("currency has been used in business")
	}
	if err := db.WithContext(ctx).Model(&MoneyAccount{}).
		Where(&MoneyAccount{AccountCurrencyId: id}).Count(&count).Error; err != nil {
		return nil, err
	}
	if count > 0 {
		return nil, errors.New("currency has been used in money account")
	}
	if err := db.WithContext(ctx).Model(&AccountTransaction{}).
		Where(&AccountTransaction{ForeignCurrencyId: id}).Count(&count).Error; err != nil {
		return nil, err
	}
	if count > 0 {
		return nil, errors.New("currency has been used in account transcation")
	}
	if err := db.WithContext(ctx).Model(&Customer{}).
		Where(&Customer{CurrencyId: id}).Count(&count).Error; err != nil {
		return nil, err
	}
	if count > 0 {
		return nil, errors.New("currency has been used in customer")
	}
	if err := db.WithContext(ctx).Model(&Supplier{}).
		Where(&Supplier{CurrencyId: id}).Count(&count).Error; err != nil {
		return nil, err
	}
	if count > 0 {
		return nil, errors.New("currency has been used in supplier")
	}

	// db action
	err = db.WithContext(ctx).Delete(&result).Error
	if err != nil {
		return nil, err
	}

	return result, nil
}

func GetCurrency(ctx context.Context, id int) (*Currency, error) {

	return GetResource[Currency](ctx, id)
}

func GetCurrencies(ctx context.Context) ([]*Currency, error) {
	businessId, ok := utils.GetBusinessIdFromContext(ctx)
	if !ok || businessId == "" {
		return nil, errors.New("business id is required")
	}
	return utils.FetchAllModels[Currency](ctx, businessId)
}

func ToggleActiveCurrency(ctx context.Context, id int, isActive bool) (*Currency, error) {
	businessId, ok := utils.GetBusinessIdFromContext(ctx)
	if !ok || businessId == "" {
		return nil, errors.New("business id is required")
	}
	if !isActive {
		db := config.GetDB()
		var count int64
		if err := db.WithContext(ctx).Model(&Business{}).
			Where("id = ? AND base_currency_id = ?", businessId, id).Count(&count).Error; err != nil {
			return nil, err
		}
		if count > 0 {
			return nil, errors.New("cannot toggle base currency inactive")
		}
	}
	return ToggleActiveModel[Currency](ctx, businessId, id, isActive)
}
