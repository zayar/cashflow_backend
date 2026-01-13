package models

import (
	"context"
	"errors"
	"time"

	"bitbucket.org/mmdatafocus/books_backend/config"
	"bitbucket.org/mmdatafocus/books_backend/utils"
	"github.com/shopspring/decimal"
)

type CurrencyExchange struct {
	ID                int             `gorm:"primary_key" json:"id"`
	BusinessId        string          `gorm:"index;not null" json:"business_id" binding:"required"`
	ForeignCurrencyId int             `gorm:"index;not null" json:"foreign_currency_id" binding:"required"`
	ExchangeDate      time.Time       `gorm:"index;not null" json:"exchange_date" binding:"required"`
	ExchangeRate      decimal.Decimal `gorm:"type:decimal(20,4);default:0" json:"exchange_rate"`
	Notes             string          `gorm:"size:255" json:"notes"`
	CreatedAt         time.Time       `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt         time.Time       `gorm:"autoUpdateTime" json:"updated_at"`
}

type NewCurrencyExchange struct {
	ForeignCurrencyId int             `json:"foreign_currency_id" binding:"required"`
	ExchangeDate      time.Time       `json:"exchange_date" binding:"required"`
	ExchangeRate      decimal.Decimal `json:"exchange_rate" binding:"required"`
	Notes             string          `json:"notes"`
}

func (ce CurrencyExchange) CheckTransactionLock(ctx context.Context) error {
	return validateTransactionLock(ctx, ce.ExchangeDate, ce.BusinessId, AccountantTransactionLock)
}

func (input *NewCurrencyExchange) validate(ctx context.Context, businessId string) error {
	if err := utils.ValidateResourceId[Currency](ctx, businessId, input.ForeignCurrencyId); err != nil {
		return errors.New("ForeignCurrencyId not found")
	}
	if err := validateTransactionLock(ctx, input.ExchangeDate, businessId, AccountantTransactionLock); err != nil {
		return err
	}
	return nil

}

func CreateCurrencyExchange(ctx context.Context, input *NewCurrencyExchange) (*CurrencyExchange, error) {

	db := config.GetDB()
	businessId, ok := utils.GetBusinessIdFromContext(ctx)
	if !ok || businessId == "" {
		return nil, errors.New("business id is required")
	}

	if err := input.validate(ctx, businessId); err != nil {
		return nil, err
	}

	exchange := CurrencyExchange{
		BusinessId:        businessId,
		ExchangeDate:      input.ExchangeDate,
		ForeignCurrencyId: input.ForeignCurrencyId,
		ExchangeRate:      input.ExchangeRate,
		Notes:             input.Notes,
	}

	err := db.WithContext(ctx).Create(&exchange).Error
	if err != nil {
		return nil, err
	}
	return &exchange, nil
}

func UpdateCurrencyExchange(ctx context.Context, id int, input *NewCurrencyExchange) (*CurrencyExchange, error) {

	db := config.GetDB()

	businessId, ok := utils.GetBusinessIdFromContext(ctx)
	if !ok || businessId == "" {
		return nil, errors.New("business id is required")
	}

	if err := input.validate(ctx, businessId); err != nil {
		return nil, err
	}

	exchange, err := utils.FetchModelForChange[CurrencyExchange](ctx, businessId, id)
	if err != nil {
		return nil, err
	}

	err = db.WithContext(ctx).Model(&exchange).Updates(map[string]interface{}{
		"ExchangeDate":      input.ExchangeDate,
		"ForeignCurrencyId": input.ForeignCurrencyId,
		"ExchangeRate":      input.ExchangeRate,
		"Notes":             input.Notes,
	}).Error
	if err != nil {
		return nil, err
	}
	return exchange, nil
}

func DeleteCurrencyExchange(ctx context.Context, id int) (*CurrencyExchange, error) {

	db := config.GetDB()

	businessId, ok := utils.GetBusinessIdFromContext(ctx)
	if !ok || businessId == "" {
		return nil, errors.New("business id is required")
	}

	result, err := utils.FetchModelForChange[CurrencyExchange](ctx, businessId, id)
	if err != nil {
		return nil, err
	}
	err = db.WithContext(ctx).Delete(&result).Error
	if err != nil {
		return nil, err
	}
	return result, nil
}

func GetCurrencyExchange(ctx context.Context, id int) (*CurrencyExchange, error) {

	db := config.GetDB()
	var result CurrencyExchange

	fieldNames, err := utils.GetQueryFields(ctx, &CurrencyExchange{})
	if err != nil {
		return nil, err
	}

	err = db.WithContext(ctx).Select(fieldNames).First(&result, id).Error
	if err != nil {
		return nil, utils.ErrorRecordNotFound
	}
	return &result, nil
}

func GetCurrencyExchanges(ctx context.Context, exchangeDate *time.Time, toCurrencyId *int, fromCurrencyId *int) ([]*CurrencyExchange, error) {

	db := config.GetDB()
	var results []*CurrencyExchange

	fieldNames, err := utils.GetQueryFields(ctx, &CurrencyExchange{})
	if err != nil {
		return nil, err
	}

	businessId, ok := utils.GetBusinessIdFromContext(ctx)
	if !ok || businessId == "" {
		return nil, errors.New("business id is required")
	}

	dbCtx := db.WithContext(ctx).Where("business_id = ?", businessId)
	if exchangeDate != nil {
		dbCtx = dbCtx.Where("exchange_date = ?", exchangeDate)
	}
	if fromCurrencyId != nil && *fromCurrencyId > 0 {
		dbCtx = dbCtx.Where("from_currency_id = ?", fromCurrencyId)
	}
	if toCurrencyId != nil && *toCurrencyId > 0 {
		dbCtx = dbCtx.Where("to_currency_id = ?", toCurrencyId)
	}
	err = dbCtx.Select(fieldNames).Order("exchange_date desc, from_currency_id, to_currency_id").Find(&results).Error
	if err != nil {
		return nil, err
	}
	return results, nil
}
