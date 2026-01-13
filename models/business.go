package models

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"bitbucket.org/mmdatafocus/books_backend/config"
	"bitbucket.org/mmdatafocus/books_backend/utils"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

type Business struct {
	ID                            uuid.UUID   `gorm:"primary_key" json:"id"`
	LogoUrl                       string      `json:"logo_url"`
	Name                          string      `gorm:"index;size:100;not null" json:"name" binding:"required"`
	ContactName                   string      `gorm:"size:100" json:"contact_name"`
	Email                         string      `gorm:"size:255" json:"email"`
	Phone                         string      `gorm:"size:20" json:"phone"`
	Mobile                        string      `gorm:"size:20" json:"mobile"`
	Website                       string      `gorm:"size:255" json:"website"`
	About                         string      `gorm:"type:text" json:"about"`
	Address                       string      `gorm:"type:text" json:"address"`
	Country                       string      `gorm:"size:100"  json:"country"`
	City                          string      `gorm:"size:100"  json:"city"`
	StateId                       int         `gorm:"index" json:"state_id"`
	TownshipId                    int         `gorm:"index" json:"township_id"`
	BaseCurrencyId                int         `json:"base_currency_id"`
	FiscalYear                    FiscalYear  `gorm:"type:enum('Jan', 'Feb', 'Mar', 'Apr', 'May', 'Jun', 'Jul', 'Aug', 'Sep', 'Oct', 'Nov', 'Dec')" json:"fiscal_year"`
	ReportBasis                   ReportBasis `gorm:"type:enum('Accrual', 'Cash');default:Accrual" json:"report_basis"`
	Timezone                      string      `gorm:"size:50" json:"timezone"`
	CompanyId                     string      `gorm:"size:100" json:"company_id"`
	TaxId                         string      `gorm:"size: 100" json:"tax_id"`
	IsTaxInclusive                *bool       `gorm:"default:false;not null" json:"is_tax_inclusive"`
	IsTaxExclusive                *bool       `gorm:"default:false;not null" json:"is_tax_exclusive"`
	MigrationDate                 time.Time   `json:"migration_date"`
	SalesTransactionLockDate      time.Time   `json:"sales_transaction_lock_date"`
	PurchaseTransactionLockDate   time.Time   `json:"purchase_transaction_lock_date"`
	BankingTransactionLockDate    time.Time   `json:"banking_transaction_lock_date"`
	AccountantTransactionLockDate time.Time   `json:"accountant_transaction_lock_date"`
	// user create?
	PrimaryBranchId int       `gorm:"not null" json:"primary_branch_id"`
	IsActive        *bool     `gorm:"not null;default:true" json:"is_active"`
	CreatedAt       time.Time `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt       time.Time `gorm:"autoUpdateTime" json:"updated_at"`

	//integration ID
	IntegrationId *string `gorm:"size:255;default:NULL" json:"integration_id"`
}

type NewBusiness struct {
	LogoUrl        string      `json:"logo_url"`
	Name           string      `json:"name" binding:"required"`
	ContactName    string      `json:"contact_name"`
	Email          string      `json:"email" binding:"required"`
	Phone          string      `json:"phone"`
	Mobile         string      `json:"mobile"`
	Website        string      `json:"website"`
	About          string      `json:"about"`
	Address        string      `json:"address"`
	Country        string      `json:"country"`
	City           string      `json:"city"`
	StateId        int         `json:"state_id"`
	TownshipId     int         `json:"township_id"`
	BaseCurrencyId int         `json:"base_currency_id"`
	FiscalYear     FiscalYear  `json:"fiscal_year"`
	ReportBasis    ReportBasis `json:"report_basis"`
	Timezone       string      `json:"timezone"`
	CompanyId      string      `json:"company_id"`
	TaxId          string      `json:"tax_id"`
	MigrationDate  time.Time   `json:"migration_date"`
}

type NewTaxSetting struct {
	IsTaxInclusive *bool `json:"is_tax_inclusive" binding:"required"`
	IsTaxExclusive *bool `json:"name" binding:"required"`
}

type NewTransactionLocking struct {
	SalesTransactionLockDate      time.Time `json:"sales_transaction_lock_date"`
	PurchaseTransactionLockDate   time.Time `json:"purchase_transaction_lock_date"`
	BankingTransactionLockDate    time.Time `json:"banking_transaction_lock_date"`
	AccountantTransactionLockDate time.Time `json:"accountant_transaction_lock_date"`
	Reason                        string    `json:"reason"`
}

type TransactionLockingRecord struct {
	ID                            int       `gorm:"primary_key" json:"id"`
	BusinessId                    string    `gorm:"index;not null" json:"business_id"`
	SalesTransactionLockDate      time.Time `json:"sales_transaction_lock_date"`
	PurchaseTransactionLockDate   time.Time `json:"purchase_transaction_lock_date"`
	BankingTransactionLockDate    time.Time `json:"banking_transaction_lock_date"`
	AccountantTransactionLockDate time.Time `json:"accountant_transaction_lock_date"`
	Reason                        string    `gorm:"default:null" json:"reason"`
	UserId                        int       `gorm:"index;not null" json:"user_id"`
	UserName                      string    `gorm:"size:100" json:"user_name"`
	CreatedAt                     time.Time `gorm:"autoCreateTime" json:"created_at"`
}

func (business *Business) StoreRedis() error {
	return config.SetRedisObject("Business:"+fmt.Sprint(business.ID), business, 0)
}

func (business *Business) RemoveRedis() error {
	return config.RemoveRedisKey("Business:" + fmt.Sprint(business.ID))
}
func (business *Business) GetIntegration() (provider, id string, err error) {
	if business.IntegrationId != nil && *business.IntegrationId != "" {
		parts := strings.SplitN(*business.IntegrationId, ":", 2)
		if len(parts) == 2 {
			return parts[0], parts[1], nil
		}
	}
	return "", "", errors.New("disabled integration")
}

func (b *Business) ProcessProductIntegrationWorkflow(tx *gorm.DB, productId int) error {
	if productId <= 0 {
		return errors.New("invalid productId")
	}
	provider, id, err := b.GetIntegration()
	if err != nil {
		return err
	}
	p := &Product{ID: productId, BusinessId: b.ID.String()}
	data, _ := p.GetIntegrationData(tx)
	if len(data) > 0 {
		if err = config.PublicIntegrationWorkflow("onProductUpdate", map[string]interface{}{"provider": provider, "id": id, "data": data}); err != nil {
			return err
		}
	}
	return nil
}

func (input *NewBusiness) validate(ctx context.Context, id string) error {
	// name
	if err := utils.ValidateUnique[Business](ctx, "", "name", input.Name, id); err != nil {
		return err
	}
	// email
	if err := utils.ValidateUnique[Business](ctx, "", "email", input.Email, id); err != nil {
		return err
	}
	// phone
	if input.Phone != "" {
		if err := utils.ValidateUnique[Business](ctx, "", "phone", input.Phone, id); err != nil {
			return err
		}
	}
	// mobile
	if input.Mobile != "" {
		if err := utils.ValidateUnique[Business](ctx, "", "mobile", input.Mobile, id); err != nil {
			return err
		}
	}
	// website
	if input.Website != "" {

		if err := utils.ValidateUnique[Business](ctx, "", "website", input.Website, id); err != nil {
			return err
		}
	}
	// baseCurrencyId
	// check if baseCurrecy exists and belongs to the business
	if input.BaseCurrencyId != 0 {
		if err := utils.ValidateResourceId[Currency](ctx, id, input.BaseCurrencyId); err != nil {
			return errors.New("currency not found")
		}
	}

	// stateId
	if input.StateId != 0 {
		if err := utils.ValidateResourceId[State](ctx, "", input.StateId); err != nil {
			return errors.New("state not found")
		}
	}
	// townshipId
	if input.TownshipId != 0 {
		if err := utils.ValidateResourceId[Township](ctx, "", input.TownshipId); err != nil {
			return errors.New("township not found")
		}
	}

	return nil
}

func CreateBusiness(ctx context.Context, input *NewBusiness) (*Business, error) {
	// only admin have access

	// When creating a business,
	// - create default currency, warehouse and branch.
	// - create default chart of accounts.
	// - create modules
	// - create 'Owner' user and 'Owner' Role
	if err := input.validate(ctx, ""); err != nil {
		return nil, err
	}
	db := config.GetDB()

	tx := db.Begin()

	BID := uuid.New()
	timezone := "Asia/Yangon"
	if input.Timezone != "" {
		timezone = input.Timezone
	}

	// Defaults to satisfy MySQL enum constraints.
	// If these are empty, MySQL will error with "Data truncated for column ...".
	fiscalYear := input.FiscalYear
	if fiscalYear == "" {
		fiscalYear = FiscalYearJan
	}
	reportBasis := input.ReportBasis
	if reportBasis == "" {
		reportBasis = ReportBasisAccrual
	}

	migrateDate := input.MigrationDate
	if migrateDate.IsZero() {
		migrateDate = time.Now()
	}
	business := Business{
		ID:                            BID,
		LogoUrl:                       input.LogoUrl,
		Name:                          input.Name,
		ContactName:                   input.ContactName,
		Email:                         input.Email,
		Phone:                         input.Phone,
		Website:                       input.Website,
		About:                         input.About,
		Address:                       input.Address,
		Country:                       input.Country,
		City:                          input.City,
		StateId:                       input.StateId,
		TownshipId:                    input.TownshipId,
		BaseCurrencyId:                input.BaseCurrencyId,
		FiscalYear:                    fiscalYear,
		ReportBasis:                   reportBasis,
		Timezone:                      timezone,
		CompanyId:                     input.CompanyId,
		TaxId:                         input.TaxId,
		IsActive:                      utils.NewTrue(),
		MigrationDate:                 migrateDate,
		SalesTransactionLockDate:      time.Now(),
		PurchaseTransactionLockDate:   time.Now(),
		AccountantTransactionLockDate: time.Now(),
		BankingTransactionLockDate:    time.Now(),
	}

	// create business
	err := tx.WithContext(ctx).Create(&business).Error
	if err != nil {
		tx.Rollback()
		return nil, err
	}

	// create defaults before creating business
	businessId := business.ID.String()
	ctx = context.WithValue(ctx, utils.ContextKeyBusinessId, businessId)
	currency, err := CreateDefaultCurrency(tx, ctx, businessId)
	if err != nil {
		tx.Rollback()
		return nil, err
	}

	// create modules for business
	modules, err := CreateDefaultModules(tx, ctx, businessId)
	if err != nil {
		tx.Rollback()
		return nil, err
	}

	owner, err := CreateDefaultOwner(tx, ctx, businessId, business.Email, business.Name)
	if err != nil {
		tx.Rollback()
		return nil, err
	}

	// gives permission to owner
	for _, module := range modules {
		roleModule := RoleModule{
			BusinessId:     businessId,
			RoleId:         owner.RoleId,
			ModuleId:       module.ID,
			AllowedActions: module.Actions,
		}
		if err := tx.WithContext(ctx).Create(&roleModule).Error; err != nil {
			tx.Rollback()
			return nil, err
		}
	}

	// Create Default TransactionNumberSeries
	seriesInput := GetTransactionNumberSeriesDefault()
	series, err := CreateDefaultTransactionNumberSeries(tx, ctx, seriesInput, businessId)

	if err != nil {
		tx.Rollback()
		return nil, err
	}

	// Create Default Branch
	branchInput := &NewBranch{
		TransactionNumberSeriesId: series.ID,
		Name:                      "Primary Branch",
	}

	branch, err := CreateDefaultBranch(tx, ctx, branchInput, businessId)
	if err != nil {
		tx.Rollback()
		return nil, err
	}

	// Create Default Warehouse
	warehouseInput := &NewWarehouse{
		BranchId: branch.ID,
		Name:     "Primary Warehouse",
	}

	err = CreateDefaultWarehouse(tx, ctx, warehouseInput, businessId)
	if err != nil {
		tx.Rollback()
		return nil, err
	}

	err = CreateDefaultAccount(tx, ctx, businessId, currency.ID)
	if err != nil {
		tx.Rollback()
		return nil, err
	}

	// Update Base Currency, Primary Branch
	err = tx.WithContext(ctx).Model(&business).Updates(map[string]interface{}{
		"BaseCurrencyId":  currency.ID,
		"PrimaryBranchId": branch.ID,
	}).Error
	if err != nil {
		tx.Rollback()
		return nil, err
	}

	if err = tx.Commit().Error; err != nil {
		return nil, err
	}

	// caching
	if err := utils.ClearRedisAdmin[Business](); err != nil {
		return nil, err
	}
	// !call runAccountingWorkFlow() for newly created business

	return &business, nil
}

func UpdateBusiness(ctx context.Context, input *NewBusiness) (*Business, error) {

	// + Allow base currency change only if no transactions
	// + Clear Currency Exchange records if base currency is changed
	businessId, ok := utils.GetBusinessIdFromContext(ctx)
	if !ok || businessId == "" {
		return nil, errors.New("business id is required")
	}

	if err := input.validate(ctx, businessId); err != nil {
		return nil, err
	}

	// db action
	db := config.GetDB()
	tx := db.Begin()
	var business Business
	if err := db.WithContext(ctx).Where("id = ?", businessId).First(&business).Error; err != nil {
		return nil, err
	}

	// don't allow to update base currency if account transaction exists
	if input.BaseCurrencyId > 0 && input.BaseCurrencyId != business.BaseCurrencyId {
		var count int64
		if err := db.WithContext(ctx).Model(&AccountTransaction{}).Where("business_id = ?", businessId).Count(&count).Error; err != nil {
			return nil, err
		}
		if count > 0 {
			return nil, errors.New("not allowed to change base currency when account transactions exist")
		}
		if err := tx.WithContext(ctx).Model(&Account{}).Where("business_id = ? AND is_system_default = true", businessId).Update("currency_id", input.BaseCurrencyId).Error; err != nil {
			return nil, err
		}
	}
	err := tx.WithContext(ctx).Model(&business).Updates(map[string]interface{}{
		"LogoUrl":        input.LogoUrl,
		"Name":           input.Name,
		"ContactName":    input.ContactName,
		"Email":          input.Email,
		"Phone":          input.Phone,
		"Mobile":         input.Mobile,
		"Website":        input.Website,
		"About":          input.About,
		"Address":        input.Address,
		"Country":        input.Country,
		"City":           input.City,
		"StateId":        input.StateId,
		"TownshipId":     input.TownshipId,
		"BaseCurrencyId": input.BaseCurrencyId,
		"FiscalYear":     input.FiscalYear,
		"ReportBasis":    input.ReportBasis,
		// "Timezone":       input.Timezone,
		"CompanyId": input.CompanyId,
		"TaxId":     input.TaxId,
		// "MigrationDate":  input.MigrationDate,
	}).Error
	if err != nil {
		tx.Rollback()
		return nil, err
	}

	// caching
	if err := business.RemoveRedis(); err != nil {
		tx.Rollback()
		return nil, err
	}
	if err := utils.ClearRedisAdmin[Business](); err != nil {
		tx.Rollback()
		return nil, err
	}
	return &business, tx.Commit().Error
}

func UpdateTransactionLocking(ctx context.Context, input NewTransactionLocking) (*Business, error) {
	db := config.GetDB()

	businessId, ok := utils.GetBusinessIdFromContext(ctx)
	if !ok || businessId == "" {
		return nil, errors.New("business id is required")
	}

	// check exists
	var business Business
	if err := db.WithContext(ctx).Where("id = ?", businessId).First(&business).Error; err != nil {
		return nil, utils.ErrorRecordNotFound
	}

	userId, ok := utils.GetUserIdFromContext(ctx)
	if !ok {
		return nil, errors.New("user id is required")
	}
	userName, ok := utils.GetUserNameFromContext(ctx)
	if !ok {
		return nil, errors.New("user id is required")
	}

	// db action
	tx := db.Begin()
	err := tx.WithContext(ctx).Model(&business).Updates(map[string]interface{}{
		"SalesTransactionLockDate":      input.SalesTransactionLockDate,
		"PurchaseTransactionLockDate":   input.PurchaseTransactionLockDate,
		"AccountantTransactionLockDate": input.AccountantTransactionLockDate,
		"BankingTransactionLockDate":    input.BankingTransactionLockDate,
	}).Error
	if err != nil {
		tx.Rollback()
		return nil, err
	}

	transactionLockingRecord := TransactionLockingRecord{
		BusinessId:                    businessId,
		SalesTransactionLockDate:      input.SalesTransactionLockDate,
		PurchaseTransactionLockDate:   input.PurchaseTransactionLockDate,
		AccountantTransactionLockDate: input.AccountantTransactionLockDate,
		BankingTransactionLockDate:    input.BankingTransactionLockDate,
		Reason:                        input.Reason,
		UserId:                        userId,
		UserName:                      userName,
	}
	err = tx.WithContext(ctx).Create(&transactionLockingRecord).Error
	if err != nil {
		tx.Rollback()
		return nil, err
	}
	if err := tx.Commit().Error; err != nil {
		return nil, err
	}

	// caching
	if err := business.RemoveRedis(); err != nil {
		return nil, err
	}
	if err := utils.ClearRedisAdmin[Business](); err != nil {
		return nil, err
	}
	return &business, nil
}

func ToggleActiveBusiness(ctx context.Context, id uuid.UUID, isActive bool) (*Business, error) {

	db := config.GetDB()
	var result Business

	// check exists
	err := db.WithContext(ctx).Where("id = ?", id).First(&result).Error
	if err != nil {
		return nil, utils.ErrorRecordNotFound
	}

	// db action
	tx := db.Begin()
	err = tx.WithContext(ctx).Model(&result).Updates(map[string]interface{}{
		"IsActive": isActive,
	}).Error
	if err != nil {
		tx.Rollback()
		return nil, err
	}

	// toggling related users
	// db action
	err = tx.WithContext(ctx).Model(&User{}).Where("business_id = ?", id).Updates(map[string]interface{}{
		"IsActive": isActive,
	}).Error
	if err != nil {
		tx.Rollback()
		return nil, err
	}

	// caching
	if err := result.RemoveRedis(); err != nil {
		tx.Rollback()
		return nil, err
	}
	if err := utils.ClearRedisAdmin[Business](); err != nil {
		tx.Rollback()
		return nil, err
	}
	return &result, tx.Commit().Error
}

func UpdateTaxSetting(ctx context.Context, input *NewTaxSetting) (*Business, error) {

	db := config.GetDB()

	businessId, ok := utils.GetBusinessIdFromContext(ctx)
	if !ok || businessId == "" {
		return nil, errors.New("business id is required")
	}

	// check exists
	var business Business
	if err := db.WithContext(ctx).Where("id = ?", businessId).First(&business).Error; err != nil {
		return nil, utils.ErrorRecordNotFound
	}

	// db action
	tx := db.Begin()
	err := tx.WithContext(ctx).Model(&business).Updates(map[string]interface{}{
		"IsTaxInclusive": input.IsTaxInclusive,
		"IsTaxExclusive": input.IsTaxExclusive,
	}).Error
	if err != nil {
		tx.Rollback()
		return nil, err
	}
	// caching
	if err := business.RemoveRedis(); err != nil {
		tx.Rollback()
		return nil, err
	}
	// not caching as active business is not affected
	return &business, tx.Commit().Error
}

func ReconcileAccounting(ctx context.Context, businessId string) (bool, error) {
	// businessId, ok := utils.GetBusinessIdFromContext(ctx)
	// if !ok || businessId == "" {
	// 	return false, errors.New("business id is required")
	// }

	// Phase 0: run drift detection immediately and persist results.
	if _, err := RunPhase0ReconciliationChecks(ctx, businessId); err != nil {
		return false, err
	}

	// Trigger reprocessing of any unprocessed outbox records via worker reconcile flow.
	msg := config.PubSubMessage{
		BusinessId:      businessId,
		ReferenceType:   "Reconcile",
		CorrelationId:   "",
	}
	// attach correlation_id if present
	if cid, ok := utils.GetCorrelationIdFromContext(ctx); ok {
		msg.CorrelationId = cid
	}

	_, err := config.PublishAccountingWorkflowWithResult(ctx, businessId, msg)
	if err != nil {
		return false, err
	}
	return true, nil
}

func GetBusinessById(ctx context.Context, id string) (*Business, error) {

	var result Business

	exists, err := config.GetRedisObject("Business:"+id, &result)
	if err != nil {
		return nil, err
	}

	if !exists {
		db := config.GetDB()
		// db query
		err := db.WithContext(ctx).Where("id = ?", id).First(&result).Error
		if err != nil {
			return nil, utils.ErrorRecordNotFound
		}
		// caching
		if err := result.StoreRedis(); err != nil {
			return nil, err
		}
	}
	return &result, nil
}

func GetBusinessById2(tx *gorm.DB, id string) (*Business, error) {

	var result Business

	exists, err := config.GetRedisObject("Business:"+id, &result)
	if err != nil {
		return nil, err
	}

	if !exists {
		// db query
		err := tx.Where("id = ?", id).First(&result).Error
		if err != nil {
			return nil, utils.ErrorRecordNotFound
		}
		// caching
		if err := result.StoreRedis(); err != nil {
			return nil, err
		}
	}
	return &result, nil
}

func GetBusiness(ctx context.Context) (*Business, error) {

	businessId, ok := utils.GetBusinessIdFromContext(ctx)
	if !ok || businessId == "" {
		return nil, errors.New("business id is required")
	}
	return GetBusinessById(ctx, businessId)
}

func GetBusinesses(ctx context.Context, name *string) ([]*Business, error) {

	db := config.GetDB()
	var results []*Business

	fieldNames, err := utils.GetQueryFields(ctx, &Business{})
	if err != nil {
		return nil, err
	}

	dbCtx := db.WithContext(ctx)
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

func GetTransactionLockingRecords(ctx context.Context, userId *int) ([]*TransactionLockingRecord, error) {

	db := config.GetDB()
	var results []*TransactionLockingRecord

	fieldNames, err := utils.GetQueryFields(ctx, &TransactionLockingRecord{})
	if err != nil {
		return nil, err
	}

	businessId, ok := utils.GetBusinessIdFromContext(ctx)
	if !ok || businessId == "" {
		return nil, errors.New("business id is required")
	}

	dbCtx := db.WithContext(ctx).Where("business_id = ?", businessId)
	if userId != nil && *userId > 0 {
		dbCtx = dbCtx.Where("user_id = ?", userId)
	}
	err = dbCtx.Select(fieldNames).Order("created_at DESC").Find(&results).Error
	if err != nil {
		return nil, err
	}
	return results, nil
}

func GetIntegrationCredentials(
	ctx context.Context,
	bizId string,
	provider string) (string, error) {

	data := make(map[string]interface{})
	business, err := GetBusinessById(ctx, bizId)
	if err != nil {
		return "", err
	}
	data["business"] = business
	ctx = utils.SetBusinessIdInContext(ctx, business.ID.String())

	branches, _ := ListAllBranch(ctx)
	data["branches"] = branches

	warehouses, _ := ListAllWarehouse(ctx)
	data["warehouses"] = warehouses

	accounts, _ := ListAllAccount(ctx)
	data["accounts"] = accounts

	jsonStr, err := utils.MarshalToJSON(data)
	if err != nil {
		return "", err
	}
	return jsonStr, nil
}

// already assumed currencies are different
func (b Business) AdjustCurrency(fromCurrencyId int, fromAmount decimal.Decimal, exchangeRate decimal.Decimal) decimal.Decimal {

	var adjustedValue decimal.Decimal
	if fromCurrencyId == b.BaseCurrencyId {
		adjustedValue = fromAmount.DivRound(exchangeRate, 4)
	} else {
		adjustedValue = fromAmount.Mul(exchangeRate)
	}
	return adjustedValue
}
