package models

import (
	"context"

	"github.com/mmdatafocus/books_backend/utils"
	"gorm.io/gorm"
)

func CreateDefaultCurrency(tx *gorm.DB, ctx context.Context, businessId string) (*Currency, error) {

	currency := Currency{
		BusinessId:    businessId,
		Symbol:        "MMK",
		Name:          "Myanmar Kyats",
		DecimalPlaces: "0",
		IsActive:      utils.NewTrue(),
	}

	if err := tx.WithContext(ctx).Create(&currency).Error; err != nil {
		tx.Rollback()
		return nil, err
	}

	return &currency, nil
}

func CreateDefaultModules(tx *gorm.DB, ctx context.Context, businessId string) ([]Module, error) {

	defaultModules := GetDefaultModules()

	var modules []Module
	for k, v := range defaultModules {
		modules = append(modules, Module{
			BusinessId: businessId,
			Name:       k,
			Actions:    v,
		})
	}

	if err := tx.WithContext(ctx).Create(&modules).Error; err != nil {
		tx.Rollback()
		return nil, err
	}

	return modules, nil
}

func CreateDefaultOwner(tx *gorm.DB, ctx context.Context, businessId string, email string, name string) (*User, error) {

	ownerRole := Role{
		Name:       "Owner",
		BusinessId: businessId,
	}
	if err := tx.WithContext(ctx).Create(&ownerRole).Error; err != nil {
		tx.Rollback()
		return nil, err
	}

	hashedPassword, err := utils.HashPassword("default123")
	if err != nil {
		return &User{}, err
	}

	owner := User{
		BusinessId: businessId,
		Username:   email,
		Name:       name,
		Email:      &email,
		Password:   string(hashedPassword),
		IsActive:   utils.NewTrue(),
		RoleId:     ownerRole.ID,
		Role:       UserRoleCustom,
	}
	if err := tx.WithContext(ctx).Create(&owner).Error; err != nil {
		tx.Rollback()
		return nil, err
	}

	return &owner, nil
}

func CreateDefaultTransactionNumberSeries(tx *gorm.DB, ctx context.Context, input *NewTransactionNumberSeries, businessId string) (*TransactionNumberSeries, error) {

	modules := make([]TransactionNumberSeriesModule, 0)
	for _, m := range input.Modules {
		modules = append(modules, TransactionNumberSeriesModule{
			ModuleName: m.ModuleName,
			Prefix:     m.Prefix,
		})
	}

	series := TransactionNumberSeries{
		BusinessId: businessId,
		Name:       input.Name,
		Modules:    modules,
	}

	err := tx.WithContext(ctx).Create(&series).Error
	if err != nil {
		tx.Rollback()
		return nil, err
	}
	return &series, nil
}

func CreateDefaultBranch(tx *gorm.DB, ctx context.Context, input *NewBranch, businessId string) (*Branch, error) {

	branch := Branch{
		BusinessId:                businessId,
		TransactionNumberSeriesId: input.TransactionNumberSeriesId,
		Name:                      input.Name,
		IsActive:                  utils.NewTrue(),
	}

	if err := tx.WithContext(ctx).Create(&branch).Error; err != nil {
		tx.Rollback()
		return nil, err
	}

	return &branch, nil
}

func CreateDefaultWarehouse(tx *gorm.DB, ctx context.Context, input *NewWarehouse, businessId string) error {

	warehouse := Warehouse{
		BusinessId: businessId,
		Name:       input.Name,
		BranchId:   input.BranchId,
		IsActive:   utils.NewTrue(),
	}

	if err := tx.WithContext(ctx).Create(&warehouse).Error; err != nil {
		tx.Rollback()
		return err
	}

	return nil
}

func CreateDefaultAccount(tx *gorm.DB, ctx context.Context, businessId string, currencyId int) error {

	chartOfAccounts := GetDefaultChartOfAccounts()

	for _, data := range chartOfAccounts {
		account := Account{
			BusinessId:        businessId,
			DetailType:        data.DetailType,
			MainType:          data.MainType,
			Name:              data.Name,
			Description:       data.Description,
			IsActive:          utils.NewTrue(),
			IsSystemDefault:   utils.NewTrue(),
			SystemDefaultCode: data.SystemDefaultCode,
			CurrencyId:        currencyId,
		}

		if err := tx.WithContext(ctx).Create(&account).Error; err != nil {
			tx.Rollback()
			return err
		}
	}

	return nil
}
