package models

import (
	"context"
	"errors"
	"time"

	"bitbucket.org/mmdatafocus/books_backend/config"
	"bitbucket.org/mmdatafocus/books_backend/utils"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

type Account struct {
	ID                int               `gorm:"primary_key" json:"id"`
	BusinessId        string            `gorm:"index;not null" json:"business_id"`
	DetailType        AccountDetailType `gorm:"type:enum('OtherAsset', 'OtherCurrentAsset', 'Cash', 'Bank', 'FixedAsset', 'Stock', 'PaymentClearing', 'InputTax', 'OtherCurrentLiability', 'CreditCard', 'LongTermLiability', 'OtherLiability', 'OverseasTaxPayable', 'OutputTax', 'Equity', 'Income', 'OtherIncome', 'Expense', 'CostOfGoodsSold', 'OtherExpense','AccountsReceivable','AccountsPayable');default:'Expense';index;size:50;not null" json:"detailType" binding:"required"`
	MainType          AccountMainType   `gorm:"type:enum('Asset', 'Liability', 'Equity', 'Income', 'Expense');default:'Expense';index;size:10;not null" json:"mainType" binding:"required"`
	// Reporting & cashflow classification (Phase A)
	// Stored on the account to make reports stable and avoid code heuristics.
	NormalBalance    NormalBalance     `gorm:"size:16;not null;default:'DEBIT';index" json:"normal_balance"`
	ReportGroup      AccountReportGroup `gorm:"size:64;index" json:"report_group"`
	CashflowActivity CashflowActivity  `gorm:"size:16;index" json:"cashflow_activity"`
	Name              string            `gorm:"index;size:100;not null" json:"name" binding:"required"`
	AccountNumber     string            `gorm:"index;size:100" json:"account_number" binding:"required"`
	CurrencyId        int               `gorm:"index;size:100" json:"currency_id"`
	Branches          string            `gorm:"type:string" json:"branches"`
	Code              string            `gorm:"size:100" json:"code"`
	Description       string            `gorm:"type:text" json:"description"`
	ParentAccountId   int               `gorm:"index;not null" json:"parentAccountId"`
	IsActive          *bool             `gorm:"not null;default:true" json:"is_active"`
	IsSystemDefault   *bool             `gorm:"not null;default:false" json:"is_system_default"`
	SystemDefaultCode string            `gorm:"index;size:3" json:"system_default_code"`
	CreatedAt         time.Time         `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt         time.Time         `gorm:"autoUpdateTime" json:"updated_at"`
}

type NewAccount struct {
	AccountNumber   string            `json:"account_number" binding:"required"`
	CurrencyId      int               `json:"currency_id"`
	Branches        string            `json:"branches"`
	DetailType      AccountDetailType `json:"detailType" binding:"required"`
	MainType        AccountMainType   `json:"mainType" binding:"required"`
	Name            string            `json:"name" binding:"required"`
	Code            string            `json:"code"`
	Description     string            `json:"description"`
	ParentAccountId int               `json:"parentAccountId" binding:"required"`
}

type NewSystemAccount struct {
	DetailType        AccountDetailType `json:"detailType" binding:"required"`
	MainType          AccountMainType   `json:"mainType" binding:"required"`
	Name              string            `json:"name" binding:"required"`
	Description       string            `json:"description"`
	SystemDefaultCode string            `json:"system_default_code"`
}

// validate input for both create & update. (id = 0 for create)

func (input *NewAccount) validate(ctx context.Context, businessId string, id int) error {
	if id > 0 {
		if id == input.ParentAccountId {
			return errors.New("self-parent not allowed")
		}
		if err := utils.ValidateResourceId[Account](ctx, businessId, id); err != nil {
			return err
		}
	}
	// name
	if err := utils.ValidateUnique[Account](ctx, businessId, "name", input.Name, id); err != nil {
		return err
	}
	// code
	if input.Code != "" {
		if err := utils.ValidateUnique[Account](ctx, businessId, "code", input.Code, id); err != nil {
			return err
		}
	}

	if input.CurrencyId > 0 {
		if err := utils.ValidateResourceId[Currency](ctx, businessId, input.CurrencyId); err != nil {
			return errors.New("currency not found")
		}

	}

	if input.ParentAccountId > 0 {
		if err := utils.ValidateResourceId[Account](ctx, businessId, input.ParentAccountId); err != nil {
			return errors.New("parent not found")
		}
	}
	return nil
}

func CreateAccount(ctx context.Context, input *NewAccount) (*Account, error) {

	businessId, ok := utils.GetBusinessIdFromContext(ctx)
	if !ok || businessId == "" {
		return nil, errors.New("business id is required")
	}

	if err := input.validate(ctx, businessId, 0); err != nil {
		return nil, err
	}

	account := Account{
		BusinessId:      businessId,
		AccountNumber:   input.AccountNumber,
		CurrencyId:      input.CurrencyId,
		Branches:        input.Branches,
		DetailType:      input.DetailType,
		MainType:        input.MainType,
		Name:            input.Name,
		Code:            input.Code,
		Description:     input.Description,
		ParentAccountId: input.ParentAccountId,
		IsActive:        utils.NewTrue(),
		IsSystemDefault: utils.NewFalse(),
	}

	if account.CurrencyId == 0 {
		business, err := GetBusiness(ctx)
		if err != nil {
			return nil, err
		}
		account.CurrencyId = business.BaseCurrencyId
	}

	db := config.GetDB()
	err := db.WithContext(ctx).Create(&account).Error
	if err != nil {
		return nil, err
	}
	return &account, nil
}

func UpdateAccount(ctx context.Context, id int, input *NewAccount) (*Account, error) {

	businessId, ok := utils.GetBusinessIdFromContext(ctx)
	if !ok || businessId == "" {
		return nil, errors.New("business id is required")
	}

	if err := input.validate(ctx, businessId, id); err != nil {
		return nil, err
	}

	account, err := utils.FetchModel[Account](ctx, businessId, id)
	if err != nil {
		return nil, err
	}

	db := config.GetDB()
	if input.CurrencyId > 0 && input.CurrencyId != account.CurrencyId {
		var count int64
		if err := db.WithContext(ctx).Model(&AccountTransaction{}).Where("account_id = ?", account.ID).Count(&count).Error; err != nil {
			return nil, err
		}
		if count > 0 {
			return nil, errors.New("not allowed to change account currency when account transactions exist")
		}
	}

	updates := map[string]interface{}{
		"Name":        input.Name,
		"Code":        input.Code,
		"Description": input.Description,
		// "ParentAccountId": input.ParentAccountId,
	}

	if !*account.IsSystemDefault {
		updates["AccountNumber"] = input.AccountNumber
		updates["Branches"] = input.Branches
		updates["DetailType"] = input.DetailType
		updates["MainType"] = input.MainType
		if input.CurrencyId > 0 {
			updates["CurrencyId"] = input.CurrencyId
		}
		if input.ParentAccountId > 0 {
			updates["ParentAccountId"] = input.ParentAccountId
		}
	}

	err = db.WithContext(ctx).Model(&account).Updates(updates).Error
	if err != nil {
		return nil, err
	}

	return account, nil
}

func MarkAccountActive(ctx context.Context, id int, isActive bool) (*Account, error) {

	db := config.GetDB()
	var main *Account

	err := db.WithContext(ctx).First(&main, id).Error
	if err != nil {
		return nil, utils.ErrorRecordNotFound
	}
	tx := db.Begin()
	err = markChildAccountsActive(tx, ctx, main, isActive)
	if err != nil {
		tx.Rollback()
		return nil, err
	}
	return main, tx.Commit().Error
}

func markChildAccountsActive(tx *gorm.DB, ctx context.Context, main *Account, isActive bool) error {
	err := tx.WithContext(ctx).Model(&main).Updates(Account{
		IsActive: &isActive,
	}).Error
	if err != nil {
		return err
	}

	// find & update child accounts
	var children []*Account
	err = tx.WithContext(ctx).Where("parent_account_id = ?", main.ID).Find(&children).Error
	if err != nil {
		return err
	}
	for _, child := range children {
		markChildAccountsActive(tx, ctx, child, isActive)
	}
	return nil
}

func DeleteAccount(ctx context.Context, id int) (*Account, error) {

	businessId, ok := utils.GetBusinessIdFromContext(ctx)
	if !ok || businessId == "" {
		return nil, errors.New("business id is required")
	}

	db := config.GetDB()

	result, err := utils.FetchModel[Account](ctx, businessId, id)
	if err != nil {
		return nil, err
	}

	if result.IsSystemDefault != nil && *result.IsSystemDefault {
		return nil, errors.New("cannot delete system-default account")
	}

	var count int64
	if err := db.WithContext(ctx).Model(&Account{}).
		Where("parent_account_id = ?", id).Count(&count).Error; err != nil {
		return nil, err
	}
	if count > 0 {
		return nil, errors.New("this account has child account(s)")
	}

	if err := db.WithContext(ctx).Model(&AccountTransaction{}).
		Where("account_id = ?", id).Count(&count).Error; err != nil {
		return nil, err
	}
	if count > 0 {
		return nil, errors.New("this account has transactions")
	}

	err = db.WithContext(ctx).Delete(&result).Error
	if err != nil {
		return nil, err
	}

	return result, nil
}

func GetAccount(ctx context.Context, id int) (*Account, error) {

	return GetResource[Account](ctx, id)
}

func GetAccounts(ctx context.Context, name *string, code *string) ([]*Account, error) {

	db := config.GetDB()
	var results []*Account

	businessId, ok := utils.GetBusinessIdFromContext(ctx)
	if !ok || businessId == "" {
		return nil, errors.New("business id is required")
	}

	dbCtx := db.WithContext(ctx).Where("business_id = ?", businessId)
	if name != nil && len(*name) > 0 {
		dbCtx = dbCtx.Where("name LIKE ?", "%"+*name+"%")
	}
	if code != nil && len(*code) > 0 {
		dbCtx = dbCtx.Where("code LIKE ?", "%"+*code+"%")
	}
	err := dbCtx.Order("name").Find(&results).Error
	if err != nil {
		return nil, err
	}
	return results, nil
}

func GetAccountClosingBalance(ctx context.Context, accountId int) (*decimal.Decimal, error) {
	db := config.GetDB()
	var result AccountCurrencyDailyBalance

	err := db.Where("account_id = ?", accountId).Order("id DESC").First(&result).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			zero := decimal.New(0, 0)
			return &zero, nil
		}
		return nil, err
	}

	return &result.RunningBalance, nil
}

func GetSystemAccounts(businessId string) (map[string]int, error) {
	var accounts []*Account
	var sysAccounts map[string]int

	exists, err := config.GetRedisObject("SystemAccounts:"+businessId, &sysAccounts)
	if err != nil {
		return nil, err
	}
	if !exists {
		db := config.GetDB()
		businessUuid, err := uuid.Parse(businessId)
		if err != nil {
			return nil, err
		}
		if err := db.Select("id", "system_default_code").Where("business_id = ?", businessUuid).Where("is_system_default = ?", true).Find(&accounts).Error; err != nil {
			return nil, err
		}
		sysAccounts = make(map[string]int)
		for _, acc := range accounts {
			sysAccounts[acc.SystemDefaultCode] = acc.ID
		}
		if err := config.SetRedisObject("SystemAccounts:"+businessId, &sysAccounts, 0); err != nil {
			return nil, err
		}
	}
	return sysAccounts, nil
}
