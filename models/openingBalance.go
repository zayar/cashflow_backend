package models

import (
	"context"
	"errors"
	"time"

	"bitbucket.org/mmdatafocus/books_backend/config"
	"bitbucket.org/mmdatafocus/books_backend/utils"
	"github.com/shopspring/decimal"
)

type OpeningBalance struct {
	ID            int                    `gorm:"primary_key" json:"id"`
	BusinessId    string                 `gorm:"index;not null" json:"business_id" binding:"required"`
	BranchId      int                    `gorm:"index;not null" json:"branch_id" binding:"required"`
	MigrationDate time.Time              `gorm:"not null" json:"migration_date" binding:"required"`
	Details       []OpeningBalanceDetail `gorm:"foreignKey:OpeningBalanceId" json:"details"`
	CreatedAt     time.Time              `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt     time.Time              `gorm:"autoUpdateTime" json:"updated_at"`
}

type OpeningBalanceDetail struct {
	ID               int             `gorm:"primary_key" json:"id"`
	OpeningBalanceId int             `gorm:"index;not null" json:"opening_balance_id" binding:"required"`
	AccountId        int             `gorm:"not null" json:"account_id"`
	Debit            decimal.Decimal `gorm:"type:decimal(20,4);default:0" json:"debit"`
	Credit           decimal.Decimal `gorm:"type:decimal(20,4);default:0" json:"credit"`
}

type NewOpeningBalance struct {
	MigrationDate time.Time                 `json:"migration_date"`
	BranchId      int                       `json:"branch_id"`
	Details       []NewOpeningBalanceDetail `json:"new_opening_balance"`
}

type NewOpeningBalanceDetail struct {
	DetailId  int             `json:"detail_id"`
	AccountId int             `json:"account_id"`
	Debit     decimal.Decimal `json:"debit"`
	Credit    decimal.Decimal `json:"credit"`
}

type OtherOpeningBalanceDetail struct {
	AccountId int             `json:"account_id"`
	Debit     decimal.Decimal `json:"debit"`
	Credit    decimal.Decimal `json:"credit"`
}

type PayableOpeningBalanceDetail struct {
	SupplierId   int             `json:"supplier_id"`
	SupplierName string          `json:"supplier_name"`
	DebitFCY     decimal.Decimal `json:"debit_fcy"`
	CreditFCY    decimal.Decimal `json:"credit_fcy"`
	// CurrencyId   int             `json:"currency_id"`
	DecimalPlaces         DecimalPlaces   `json:"decimal_places"`
	ForeignCurrencySymbol string          `json:"foreign_currency_symbol"`
	ExchangeRate          decimal.Decimal `json:"exchange_rate"`
	Debit                 decimal.Decimal `json:"debit"`
	Credit                decimal.Decimal `json:"credit"`
}

type ReceivableOpeningBalanceDetail struct {
	CustomerId   int             `json:"customer_id"`
	CustomerName string          `json:"customer_name"`
	DebitFCY     decimal.Decimal `json:"debit_fcy"`
	CreditFCY    decimal.Decimal `json:"credit_fcy"`
	// CurrencyId   int             `json:"currency_id"`
	DecimalPlaces         DecimalPlaces   `json:"decimal_places"`
	ForeignCurrencySymbol string          `json:"foreign_currency_symbol"`
	ExchangeRate          decimal.Decimal `json:"exchange_rate"`
	Debit                 decimal.Decimal `json:"debit"`
	Credit                decimal.Decimal `json:"credit"`
}

type StockOpeningBalanceDetail struct {
	WarehouseId    int             `json:"warehouse_id"`
	WarehouseName  string          `json:"warehouse_name"`
	ProductId      int             `json:"product_id"`
	ProductName    string          `json:"product_name"`
	ProductType    string          `json:"product_type"`
	BatchNumber    string          `json:"batch_number"`
	Qty            decimal.Decimal `json:"qty"`
	UnitValue      decimal.Decimal `json:"unit_value"`
	TotalValue     decimal.Decimal `json:"total_value"`
	DecimalPlaces  DecimalPlaces   `json:"decimal_places"`
	CurrencySymbol string          `json:"currency_symbol"`
}

func UpdateOpeningBalance(ctx context.Context, input *NewOpeningBalance) (*OpeningBalance, error) {
	var openingBalance OpeningBalance
	businessId, ok := utils.GetBusinessIdFromContext(ctx)
	if !ok || businessId == "" {
		return nil, errors.New("business id is required")
	}
	business, err := GetBusinessById(ctx, businessId)
	if err != nil {
		return &openingBalance, err
	}

	db := config.GetDB()
	tx := db.Begin()
	// Check Migration Date
	bmDate, err := utils.ConvertToDate(business.MigrationDate, business.Timezone)
	if err != nil {
		return &openingBalance, err
	}
	imDate, err := utils.ConvertToDate(input.MigrationDate, business.Timezone)
	if err != nil {
		return &openingBalance, err
	}
	if !bmDate.Equal(imDate) {
		var count int64
		if err := tx.WithContext(ctx).Model(&AccountJournal{}).Where("business_id = ? AND reference_type != ?", businessId, AccountReferenceTypeOpeningBalance).Count(&count).Error; err != nil {
			return &openingBalance, err
		}
		if count > 0 {
			return &openingBalance, errors.New("other transactions already exist. not allowed to change migration date")
		}
		// If Migration Date is ok, update in Business
		err := tx.WithContext(ctx).Model(&business).Updates(map[string]interface{}{
			"MigrationDate": input.MigrationDate,
		}).Error
		if err != nil {
			tx.Rollback()
			return &openingBalance, err
		}
		// caching
		if err := business.RemoveRedis(); err != nil {
			tx.Rollback()
			return &openingBalance, err
		}
		if err := utils.ClearRedisAdmin[Business](); err != nil {
			tx.Rollback()
			return &openingBalance, err
		}
	}

	// Delete old Opening Balance if exists
	var result OpeningBalance
	err = tx.WithContext(ctx).Preload("Details").Where("business_id = ? AND branch_id = ?", businessId, input.BranchId).Find(&result).Error
	if err != nil {
		return nil, err
	}
	if result.ID > 0 {
		err = tx.WithContext(ctx).Model(&result).Association("Details").Unscoped().Clear()
		if err != nil {
			tx.Rollback()
			return nil, err
		}

		err = tx.WithContext(ctx).Delete(&result).Error
		if err != nil {
			tx.Rollback()
			return nil, err
		}
	}

	// Loop and record opening Balances
	var openingBalanceDetails []OpeningBalanceDetail
	for _, item := range input.Details {
		detail := OpeningBalanceDetail{
			AccountId: item.AccountId,
			Debit:     item.Debit,
			Credit:    item.Credit,
		}
		openingBalanceDetails = append(openingBalanceDetails, detail)
	}

	openingBalance = OpeningBalance{
		BusinessId:    businessId,
		BranchId:      input.BranchId,
		MigrationDate: input.MigrationDate,
		Details:       openingBalanceDetails,
	}
	err = tx.WithContext(ctx).Create(&openingBalance).Error
	if err != nil {
		tx.Rollback()
		return nil, err
	}

	err = PublishToAccounting(ctx, tx, businessId, openingBalance.MigrationDate, openingBalance.ID, AccountReferenceTypeOpeningBalance, openingBalance, nil, PubSubMessageActionCreate)
	if err != nil {
		tx.Rollback()
		return nil, err
	}

	// Commit the transaction
	if err := tx.Commit().Error; err != nil {
		return nil, err
	}

	return &openingBalance, nil
}

func GetOpeningBalance(ctx context.Context, branchId int) (*OpeningBalance, error) {
	db := config.GetDB()
	var result OpeningBalance
	var others []OtherOpeningBalanceDetail

	businessId, ok := utils.GetBusinessIdFromContext(ctx)
	if !ok || businessId == "" {
		return &result, errors.New("business id is required")
	}

	err := db.WithContext(ctx).Raw(`
			SELECT at.account_id, SUM(at.base_debit) AS debit, SUM(at.base_credit) AS credit FROM account_transactions at 
			INNER JOIN account_journals aj ON at.journal_id = aj.id
			INNER join accounts ac ON at.account_id = ac.id
			WHERE aj.business_id = ? AND aj.branch_id = ? 
			AND aj.reference_type in ('OB','SOB','COB','POS','PCOS','PGOS')
			AND ac.main_type in ('Asset','Liability','Equity')
			GROUP BY at.account_id
	`, businessId, branchId).Find(&others).Error
	if err != nil {
		return &result, err
	}

	err = db.WithContext(ctx).Preload("Details").Where("business_id = ? AND branch_id = ?", businessId, branchId).Find(&result).Error
	if err != nil {
		return &result, err
	}
	if result.ID <= 0 {
		result.BusinessId = businessId
		result.BranchId = branchId
		business, err := GetBusinessById(ctx, businessId)
		if err != nil {
			return &result, err
		}
		result.MigrationDate = business.MigrationDate
	}
	openingDetails := make([]OpeningBalanceDetail, 0)
	if len(others) > 0 {
		for i, other := range others {
			openingDetails = append(openingDetails, OpeningBalanceDetail{
				ID:               i + 1,
				OpeningBalanceId: result.ID,
				AccountId:        other.AccountId,
				Debit:            other.Debit,
				Credit:           other.Credit,
			})
		}
	}
	result.Details = openingDetails
	return &result, nil
}

func GetPayableOpeningBalanceDetails(ctx context.Context, branchId int) ([]*PayableOpeningBalanceDetail, error) {
	db := config.GetDB()
	var results []*PayableOpeningBalanceDetail

	businessId, ok := utils.GetBusinessIdFromContext(ctx)
	if !ok || businessId == "" {
		return nil, errors.New("business id is required")
	}

	systemAccounts, err := GetSystemAccounts(businessId)
	if err != nil {
		return nil, err
	}

	err = db.WithContext(ctx).Raw(`
		SELECT s.id AS supplier_id, s.name AS supplier_name, at.foreign_debit AS debit_fcy, at.foreign_credit AS credit_fcy,
		at.exchange_rate AS exchange_rate, at.base_debit AS debit, at.base_credit AS credit,
		cu.symbol foreign_currency_symbol, cu.decimal_places
		FROM account_transactions at
		INNER JOIN account_journals aj ON at.journal_id = aj.id
		INNER JOIN suppliers s ON aj.supplier_id = s.id
		INNER JOIN currencies cu ON at.foreign_currency_id = cu.id
		WHERE aj.business_id = ? AND aj.branch_id = ? AND aj.reference_type = 'SOB' AND at.account_id = ?
	`, businessId, branchId, systemAccounts[AccountCodeAccountsPayable]).Find(&results).Error
	if err != nil {
		return nil, err
	}
	return results, nil
}

func GetReceivableOpeningBalanceDetails(ctx context.Context, branchId int) ([]*ReceivableOpeningBalanceDetail, error) {
	db := config.GetDB()
	var results []*ReceivableOpeningBalanceDetail

	businessId, ok := utils.GetBusinessIdFromContext(ctx)
	if !ok || businessId == "" {
		return nil, errors.New("business id is required")
	}

	systemAccounts, err := GetSystemAccounts(businessId)
	if err != nil {
		return nil, err
	}

	err = db.WithContext(ctx).Raw(`
		SELECT c.id AS customer_id, c.name AS customer_name, at.foreign_debit AS debit_fcy, at.foreign_credit AS credit_fcy,
		at.exchange_rate AS exchange_rate, at.base_debit AS debit, at.base_credit AS credit,
		cu.symbol foreign_currency_symbol, cu.decimal_places
		FROM account_transactions at
		INNER JOIN account_journals aj ON at.journal_id = aj.id
		INNER JOIN customers c ON aj.customer_id = c.id
		INNER JOIN currencies cu ON at.foreign_currency_id = cu.id
		WHERE aj.business_id = ? AND aj.branch_id = ? AND aj.reference_type = 'COB' AND at.account_id = ?
	`, businessId, branchId, systemAccounts[AccountCodeAccountsReceivable]).Find(&results).Error
	if err != nil {
		return nil, err
	}
	return results, nil
}

func GetStockOpeningBalanceDetails(ctx context.Context, branchId int, accountId int) ([]*StockOpeningBalanceDetail, error) {
	db := config.GetDB()
	var results []*StockOpeningBalanceDetail

	businessId, ok := utils.GetBusinessIdFromContext(ctx)
	if !ok || businessId == "" {
		return nil, errors.New("business id is required")
	}

	err := db.WithContext(ctx).Raw(`
		SELECT w.id AS warehouse_id, w.name AS warehouse_name, os.product_id AS product_id,
		CASE WHEN os.product_type = 'S' THEN p.name ELSE pv.name END as product_name,
		os.product_type AS product_type, os.batch_number AS batch_number, os.qty AS qty,
		os.unit_value AS unit_value, os.qty * os.unit_value AS total_value
		FROM opening_stocks os
		INNER JOIN warehouses w ON os.warehouse_id = w.id
		LEFT JOIN products p ON os.product_id = p.id AND os.product_type = 'S'
		LEFT JOIN product_variants pv ON os.product_id = pv.id AND os.product_type = 'V'
		WHERE w.business_id = ? AND w.branch_id = ? AND os.inventory_account_id = ?;
	`, businessId, branchId, accountId).Find(&results).Error
	if err != nil {
		return nil, err
	}
	return results, nil
}
