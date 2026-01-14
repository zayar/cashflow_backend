package models

import (
	"context"
	"errors"
	"time"

	"github.com/mmdatafocus/books_backend/config"
	"github.com/mmdatafocus/books_backend/utils"
	"github.com/shopspring/decimal"
)

type MoneyAccount struct {
	ID                    int              `gorm:"primary_key" json:"id"`
	BusinessId            string           `gorm:"index;not null" json:"business_id"`
	AccountType           MoneyAccountType `gorm:"type:enum('cash','bank','card');default:'cash';size:12;not null" json:"account_type" binding:"required"`
	AccountName           string           `gorm:"index;size:100;not null" json:"account_name" binding:"required"`
	AccountCode           string           `gorm:"size:50" json:"account_code"`
	AccountCurrencyId     int              `gorm:"not null" json:"account_currency_id" binding:"required"`
	AccountNumber         string           `gorm:"size:50" json:"account_number"`
	BankName              string           `gorm:"size:100" json:"bank_name"`
	RoutingNumber         string           `gorm:"size:50" json:"routing_number"`
	Branches              string           `json:"branches"`
	CurrentBaseBalance    decimal.Decimal  `gorm:"type:decimal(20,4);default:0" json:"current_base_balance"`
	CurrentForeignBalance decimal.Decimal  `gorm:"type:decimal(20,4);default:0" json:"current_foreign_balance"`
	Description           string           `gorm:"type:text" json:"description"`
	IsActive              *bool            `gorm:"not null;default:true" json:"is_active"`
	CreatedAt             time.Time        `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt             time.Time        `gorm:"autoUpdateTime" json:"updated_at"`
}

type NewMoneyAccount struct {
	AccountType       MoneyAccountType `json:"account_type" binding:"required"`
	AccountName       string           `json:"account_name" binding:"required"`
	AccountCode       string           `json:"account_code"`
	AccountCurrencyId int              `json:"account_currency_id" binding:"required"`
	AccountNumber     string           `json:"account_number"`
	BankName          string           `json:"bank_name"`
	RoutingNumber     string           `json:"routing_number"`
	Branches          string           `json:"branches"`
	Description       string           `json:"description"`
}

type MoneyAccountsEdge Edge[MoneyAccount]
type MoneyAccountsConnection struct {
	PageInfo *PageInfo            `json:"pageInfo"`
	Edges    []*MoneyAccountsEdge `json:"edges"`
}

// node
// returns decoded curosr string
func (ma MoneyAccount) GetCursor() string {
	return ma.CreatedAt.String()
}

func (ma MoneyAccount) GetId() int {
	return ma.ID
}

// validate input for both create & update. (id = 0 for create)

func (input *NewMoneyAccount) validate(ctx context.Context, businessId string, id int) error {
	if id > 0 {
		if err := utils.ValidateResourceId[MoneyAccount](ctx, businessId, id); err != nil {
			return err
		}
	}
	// name
	if err := utils.ValidateUnique[MoneyAccount](ctx, businessId, "account_name", input.AccountName, id); err != nil {
		return err
	}
	// accountCurrencyNumber
	if err := utils.ValidateResourceId[Currency](ctx, businessId, input.AccountCurrencyId); err != nil {
		return errors.New("account currency not found")
	}
	return nil
}

func CreateMoneyAccount(ctx context.Context, input *NewMoneyAccount) (*MoneyAccount, error) {

	businessId, ok := utils.GetBusinessIdFromContext(ctx)
	if !ok || businessId == "" {
		return nil, errors.New("business id is required")
	}

	if err := input.validate(ctx, businessId, 0); err != nil {
		return nil, err
	}
	// if err := utils.ValidateUnique[MoneyAccount](ctx, businessId, "account_name", input.AccountName, 0); err != nil {
	// 	return nil, err
	// }
	account := MoneyAccount{
		BusinessId:        businessId,
		AccountType:       input.AccountType,
		AccountName:       input.AccountName,
		AccountCode:       input.AccountCode,
		AccountCurrencyId: input.AccountCurrencyId,
		AccountNumber:     input.AccountNumber,
		BankName:          input.BankName,
		RoutingNumber:     input.RoutingNumber,
		Branches:          input.Branches,
		Description:       input.Description,
		IsActive:          utils.NewTrue(),
	}

	db := config.GetDB()
	err := db.WithContext(ctx).Create(&account).Error
	if err != nil {
		return nil, err
	}
	return &account, nil
}

func UpdateMoneyAccount(ctx context.Context, id int, input *NewMoneyAccount) (*MoneyAccount, error) {

	businessId, ok := utils.GetBusinessIdFromContext(ctx)
	if !ok || businessId == "" {
		return nil, errors.New("business id is required")
	}

	if err := input.validate(ctx, businessId, id); err != nil {
		return nil, err
	}

	account, err := utils.FetchModel[MoneyAccount](ctx, businessId, id)
	if err != nil {
		return nil, err
	}

	db := config.GetDB()
	// db action
	err = db.WithContext(ctx).Model(&account).Updates(map[string]interface{}{
		"AccountType":       input.AccountType,
		"AccountName":       input.AccountName,
		"AccountCode":       input.AccountCode,
		"AccountCurrencyId": input.AccountCurrencyId,
		"AccountNumber":     input.AccountNumber,
		"BankName":          input.BankName,
		"RoutingNumber":     input.RoutingNumber,
		"Branches":          input.Branches,
		"Description":       input.Description,
	}).Error
	if err != nil {
		return nil, err
	}
	return account, nil
}

func DeleteMoneyAccount(ctx context.Context, id int) (*MoneyAccount, error) {

	// // Do not delete if any Warehouse use this branch
	// var count int64
	// err = db.WithContext(ctx).Model(&Warehouse{}).Where("branch_id = ?", id).Count(&count).Error
	// if err != nil {
	// 	return nil, err
	// }
	// if count > 0 {
	// 	return nil, errors.New("used by warehouse")
	// }

	businessId, ok := utils.GetBusinessIdFromContext(ctx)
	if !ok || businessId == "" {
		return nil, errors.New("business id is required")
	}
	result, err := utils.FetchModel[MoneyAccount](ctx, businessId, id)
	if err != nil {
		return nil, err
	}

	db := config.GetDB()
	// db action
	err = db.WithContext(ctx).Delete(&result).Error
	if err != nil {
		return nil, err
	}

	return result, nil
}

func GetMoneyAccount(ctx context.Context, id int) (*MoneyAccount, error) {

	return GetResource[MoneyAccount](ctx, id)
}

func GetMoneyAccounts(ctx context.Context, accountType *string, accountName *string, branchId *int) ([]*MoneyAccount, error) {

	db := config.GetDB()
	var results []*MoneyAccount

	fieldNames, err := utils.GetQueryFields(ctx, &MoneyAccount{})
	if err != nil {
		return nil, err
	}

	businessId, ok := utils.GetBusinessIdFromContext(ctx)
	if !ok || businessId == "" {
		return nil, errors.New("business id is required")
	}

	dbCtx := db.WithContext(ctx).Where("business_id = ?", businessId)
	if accountType != nil && len(*accountType) > 0 {
		dbCtx = dbCtx.Where("account_type = ?", accountType)
	}
	if accountName != nil && len(*accountName) > 0 {
		dbCtx = dbCtx.Where("account_name LIKE ?", "%"+*accountName+"%")
	}
	if branchId != nil && *branchId > 0 {
		dbCtx = dbCtx.Where("branch_id = ?", branchId)
	}
	err = dbCtx.Select(fieldNames).Order("account_name").Find(&results).Error
	if err != nil {
		return nil, err
	}
	return results, nil
}

func ToggleActiveMoneyAccount(ctx context.Context, id int, isActive bool) (*MoneyAccount, error) {
	businessId, ok := utils.GetBusinessIdFromContext(ctx)
	if !ok || businessId == "" {
		return nil, errors.New("business id is required")
	}

	return ToggleActiveModel[MoneyAccount](ctx, businessId, id, isActive)
}

func PaginateMoneyAccount(ctx context.Context, limit *int, after *string,
	name *string) (*MoneyAccountsConnection, error) {

	businessId, ok := utils.GetBusinessIdFromContext(ctx)
	if !ok || businessId == "" {
		return nil, errors.New("business id is required")
	}

	db := config.GetDB()
	dbCtx := db.WithContext(ctx).Where("business_id = ?", businessId)
	if name != nil && *name != "" {
		dbCtx.Where("name LIKE ?", "%"+*name+"%")
	}
	edges, pageInfo, err := FetchPageCompositeCursor[MoneyAccount](dbCtx, *limit, after, "created_at", "<")
	if err != nil {
		return nil, err
	}
	var moneyAccountsConnection MoneyAccountsConnection
	moneyAccountsConnection.PageInfo = pageInfo
	for _, edge := range edges {
		moneyAccountEdge := MoneyAccountsEdge(edge)
		moneyAccountsConnection.Edges = append(moneyAccountsConnection.Edges, &moneyAccountEdge)
	}
	return &moneyAccountsConnection, err
}
