package models

import (
	"context"
	"errors"
	"time"

	"bitbucket.org/mmdatafocus/books_backend/config"
	"bitbucket.org/mmdatafocus/books_backend/utils"
	"github.com/shopspring/decimal"
)

type Supplier struct {
	ID                             int              `gorm:"primary_key" json:"id"`
	BusinessId                     string           `gorm:"index;not null" json:"business_id" binding:"required"`
	Name                           string           `gorm:"size:100;not null" json:"name" binding:"required"`
	Email                          string           `gorm:"size:100" json:"email"`
	Phone                          string           `gorm:"size:20" json:"phone"`
	Mobile                         string           `gorm:"size:20" json:"mobile"`
	CurrencyId                     int              `gorm:"not null" json:"currency_id" binding:"required"`
	ExchangeRate                   decimal.Decimal  `gorm:"type:decimal(20,4);default:0" json:"exchange_rate"`
	SupplierTaxId                  int              `json:"supplier_tax_id"`
	SupplierTaxType                *TaxType         `gorm:"type:enum('I','G');default:'I'" json:"supplier_tax_type"`
	SupplierPaymentTerms           PaymentTerms     `gorm:"type:enum('Net15', 'Net30', 'Net45', 'Net60', 'DueMonthEnd', 'DueNextMonthEnd', 'DueOnReceipt', 'Custom');not null;default:'DueOnReceipt'" json:"supplier_payment_terms" binding:"required"`
	SupplierPaymentTermsCustomDays int              `gorm:"default:0" json:"supplier_payment_terms_custom_days"`
	Notes                          string           `gorm:"type:text" json:"notes"`
	BillingAddress                 BillingAddress   `gorm:"polymorphic:Reference" json:"-"`
	ShippingAddress                ShippingAddress  `gorm:"polymorphic:Reference" json:"-"`
	ContactPersons                 []*ContactPerson `gorm:"polymorphic:Reference" json:"-"`
	Documents                      []*Document      `gorm:"polymorphic:Reference" json:"-"`
	IsActive                       *bool            `gorm:"not null;default:true" json:"is_active"`
	OpeningBalanceBranchId         int              `gorm:"default:0" json:"opening_balance_branch_id"`
	OpeningBalance                 decimal.Decimal  `gorm:"type:decimal(20,4);default:0" json:"opening_balance"`
	CreatedAt                      time.Time        `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt                      time.Time        `gorm:"autoUpdateTime" json:"updated_at"`
}

type SupplierOpeningBalance struct { // use in PubSub
	ID                     int             `json:"id"`
	SupplierName           string          `json:"supplier_name"`
	CurrencyId             int             `json:"currency_id"`
	ExchangeRate           decimal.Decimal `json:"exchange_rate"`
	OpeningBalanceBranchId int             `json:"opening_balance_branch_id"`
	OpeningBalance         decimal.Decimal `json:"opening_balance"`
	CreatedAt              time.Time       `json:"created_at"`
}

type NewSupplier struct {
	Name                           string              `json:"name" binding:"required"`
	Email                          string              `json:"email"`
	Phone                          string              `json:"phone"`
	Mobile                         string              `json:"mobile"`
	CurrencyId                     int                 `json:"currency_id" binding:"required"`
	ExchangeRate                   decimal.Decimal     `json:"exchange_rate"`
	SupplierTaxId                  int                 `json:"supplier_tax_id"`
	SupplierTaxType                *TaxType            `json:"supplier_tax_type"`
	OpeningBalanceBranchId         int                 `json:"opening_balance_branch_id"`
	OpeningBalance                 decimal.Decimal     `json:"opening_balance"`
	SupplierPaymentTerms           PaymentTerms        `json:"supplier_payment_terms" binding:"required"`
	SupplierPaymentTermsCustomDays int                 `json:"supplier_payment_terms_custom_days"`
	Notes                          string              `json:"notes"`
	BillingAddress                 *NewBillingAddress  `json:"billing_address"`
	ShippingAddress                *NewShippingAddress `json:"shipping_address"`
	ContactPersons                 []*NewContactPerson `json:"contact_persons"`
	Documents                      []*NewDocument      `json:"documents"`
}

// When creating supplier with opening balance, create account transactions "SupplierOpeningBalance" (credit to OtherExpenses, debit to AccountPayable)
// Don't allow to update opening balance
// Don't delete if used in transactions

// CreateSupplier(newSupplier) (Supplier,error) <Owner/Custom>
// UpdateSupplier(id, newSupplier) (Supplier,error) <Owner/Custom>
// DeleteSupplier(id) (Supplier,error) <Owner/Custom>

// GetSupplier(id) (Supplier,error) <Owner/Custom>
// ListSupplier(businessId, name, code) ([]Supplier,error) <Owner/Custom>

// ToggleActiveSupplier(id, isActive) (Supplier,error) <Owner/Custom>
// validate input for both create & update. (id = 0 for create)

type SuppliersEdge Edge[Supplier]
type SuppliersConnection struct {
	PageInfo *PageInfo        `json:"pageInfo"`
	Edges    []*SuppliersEdge `json:"edges"`
}

// node
// returns decoded curosr string
func (s Supplier) GetCursor() string {
	return s.CreatedAt.String()
}

func (input *NewSupplier) validate(ctx context.Context, businessId string, id int) error {
	// validate unique name
	if err := utils.ValidateUnique[Supplier](ctx, businessId, "name", input.Name, id); err != nil {
		return err
	}
	// validate email
	if input.Email != "" && len(input.Email) > 0 {
		if err := utils.ValidateUnique[Supplier](ctx, businessId, "email", input.Email, id); err != nil {
			return err
		}
	}
	// validate phone
	if input.Phone != "" && len(input.Phone) > 0 {
		if err := utils.ValidateUnique[Supplier](ctx, businessId, "phone", input.Phone, id); err != nil {
			return err
		}
	}
	// validate currency
	if err := utils.ValidateResourceId[Currency](ctx, businessId, input.CurrencyId); err != nil {
		return errors.New("currency not found")
	}
	// validate tax
	if input.SupplierTaxType != nil {
		if err := validateTaxExists(ctx, businessId, input.SupplierTaxId, *input.SupplierTaxType); err != nil {
			return errors.New("tax not found")
		}
	}
	return nil
}

func CreateSupplier(ctx context.Context, input *NewSupplier) (*Supplier, error) {

	businessId, ok := utils.GetBusinessIdFromContext(ctx)
	if !ok || businessId == "" {
		return nil, errors.New("business id is required")
	}

	business, err := GetBusinessById(ctx, businessId)
	if err != nil {
		return nil, err
	}

	if err := input.validate(ctx, businessId, 0); err != nil {
		return nil, err
	}

	contactPersons := mapNewContactPersons(input.ContactPersons, "suppliers", 0)
	documents, err := mapNewDocuments(input.Documents, "suppliers", 0)
	if err != nil {
		return nil, err
	}
	supplier := Supplier{
		BusinessId:                     businessId,
		Name:                           input.Name,
		Email:                          input.Email,
		Phone:                          input.Phone,
		Mobile:                         input.Mobile,
		CurrencyId:                     input.CurrencyId,
		ExchangeRate:                   input.ExchangeRate,
		SupplierTaxId:                  input.SupplierTaxId,
		SupplierTaxType:                input.SupplierTaxType,
		SupplierPaymentTerms:           input.SupplierPaymentTerms,
		SupplierPaymentTermsCustomDays: input.SupplierPaymentTermsCustomDays,
		Notes:                          input.Notes,
		IsActive:                       utils.NewTrue(),
		OpeningBalanceBranchId:         input.OpeningBalanceBranchId,
		OpeningBalance:                 input.OpeningBalance,
		// associations
		ContactPersons: contactPersons,
		Documents:      documents,
	}

	if input.BillingAddress != nil {
		supplier.BillingAddress = mapBillingAddressInput(*input.BillingAddress)
	}
	if input.ShippingAddress != nil {
		supplier.ShippingAddress = mapShippingAddressInput(*input.ShippingAddress)
	}

	db := config.GetDB()
	tx := db.Begin()
	// db action
	err = tx.WithContext(ctx).Create(&supplier).Error
	if err != nil {
		return nil, err
	}

	if !input.OpeningBalance.IsZero() {
		bill := Bill{
			BusinessId:                 businessId,
			SupplierId:                 supplier.ID,
			BranchId:                   input.OpeningBalanceBranchId,
			BillDate:                   business.MigrationDate,
			BillNumber:                 "Supplier Opening Balance",
			SequenceNo:                 decimal.NewFromInt(0),
			BillPaymentTerms:           PaymentTermsDueOnReceipt,
			BillPaymentTermsCustomDays: 0,
			BillDueDate:                &business.MigrationDate,
			CurrencyId:                 supplier.CurrencyId,
			ExchangeRate:               input.ExchangeRate,
			BillSubtotal:               input.OpeningBalance,
			BillTotalAmount:            input.OpeningBalance,
			RemainingBalance:           input.OpeningBalance,
			CurrentStatus:              BillStatusConfirmed,
			WarehouseId:                0,
		}
		err = tx.WithContext(ctx).Create(&bill).Error
		if err != nil {
			return nil, err
		}

		supplierOpeningBalance := SupplierOpeningBalance{
			ID:                     supplier.ID,
			SupplierName:           supplier.Name,
			CurrencyId:             supplier.CurrencyId,
			ExchangeRate:           input.ExchangeRate,
			OpeningBalanceBranchId: input.OpeningBalanceBranchId,
			OpeningBalance:         input.OpeningBalance,
			CreatedAt:              supplier.CreatedAt,
		}
		err = PublishToAccounting(ctx, tx, businessId, supplier.CreatedAt, supplier.ID, AccountReferenceTypeSupplierOpeningBalance, supplierOpeningBalance, nil, PubSubMessageActionCreate)
		if err != nil {
			tx.Rollback()
			return nil, err
		}
	}

	if err != nil {
		tx.Rollback()
		return nil, err
	}

	if err = tx.Commit().Error; err != nil {
		return nil, err
	}

	return &supplier, nil
}

func UpdateSupplier(ctx context.Context, id int, input *NewSupplier) (*Supplier, error) {

	businessId, ok := utils.GetBusinessIdFromContext(ctx)
	if !ok || businessId == "" {
		return nil, errors.New("business id is required")
	}
	oldSupplier, err := utils.FetchModel[Supplier](ctx, businessId, id)
	if err != nil {
		return nil, err
	}

	if err := input.validate(ctx, businessId, id); err != nil {
		return nil, err
	}

	supplier, err := utils.FetchModel[Supplier](ctx, businessId, id)
	if err != nil {
		return nil, err
	}
	db := config.GetDB()
	tx := db.Begin()
	err = tx.WithContext(ctx).Model(supplier).
		Updates(map[string]interface{}{
			"Name":                           input.Name,
			"Email":                          input.Email,
			"Phone":                          input.Phone,
			"Mobile":                         input.Mobile,
			"CurrencyId":                     input.CurrencyId,
			"SupplierTaxId":                  input.SupplierTaxId,
			"SupplierTaxType":                input.SupplierTaxType,
			"SupplierPaymentTerms":           input.SupplierPaymentTerms,
			"SupplierPaymentTermsCustomDays": input.SupplierPaymentTermsCustomDays,
			"Notes":                          input.Notes,
			"OpeningBalanceBranchId":         input.OpeningBalanceBranchId,
			"OpeningBalance":                 input.OpeningBalance,
		}).Error
	if err != nil {
		tx.Rollback()
		return nil, err
	}
	// updating related tables
	// if err := tx.WithContext(ctx).Model(&supplier).Association("BillingAddress").Replace(&billingAddress); err != nil {
	// 	tx.Rollback()
	// 	return nil, err
	// }
	// if err := tx.WithContext(ctx).Model(&supplier).Association("ShippingAddress").Replace(&shippingAddress); err != nil {
	// 	tx.Rollback()
	// 	return nil, err
	// }

	// upserting association
	if input.BillingAddress != nil {
		if err := upsertBillingAddress(tx, ctx, *input.BillingAddress, "suppliers", id); err != nil {
			tx.Rollback()
			return nil, err
		}
	}
	if input.ShippingAddress != nil {
		if err := upsertShippingAddress(tx, ctx, *input.ShippingAddress, "suppliers", id); err != nil {
			tx.Rollback()
			return nil, err
		}
	}
	if _, err := upsertContactPersons(ctx, tx, input.ContactPersons, "suppliers", id); err != nil {
		tx.Rollback()
		return nil, err
	}
	if _, err := upsertDocuments(ctx, tx, input.Documents, "suppliers", id); err != nil {
		tx.Rollback()
		return nil, err
	}
	if !input.OpeningBalance.IsZero() && oldSupplier.OpeningBalance.IsZero() {
		business, err := GetBusinessById(ctx, businessId)
		if err != nil {
			tx.Rollback()
			return nil, err
		}
		bill := Bill{
			BusinessId:                 businessId,
			SupplierId:                 supplier.ID,
			BranchId:                   input.OpeningBalanceBranchId,
			BillDate:                   business.MigrationDate,
			BillNumber:                 "Supplier Opening Balance",
			SequenceNo:                 decimal.NewFromInt(0),
			BillPaymentTerms:           PaymentTermsDueOnReceipt,
			BillPaymentTermsCustomDays: 0,
			BillDueDate:                &business.MigrationDate,
			CurrencyId:                 supplier.CurrencyId,
			ExchangeRate:               input.ExchangeRate,
			BillSubtotal:               input.OpeningBalance,
			BillTotalAmount:            input.OpeningBalance,
			RemainingBalance:           input.OpeningBalance,
			CurrentStatus:              BillStatusConfirmed,
			WarehouseId:                0,
		}
		err = tx.WithContext(ctx).Create(&bill).Error
		if err != nil {
			return nil, err
		}

		supplierOpeningBalance := SupplierOpeningBalance{
			ID:                     supplier.ID,
			SupplierName:           supplier.Name,
			CurrencyId:             supplier.CurrencyId,
			ExchangeRate:           input.ExchangeRate,
			OpeningBalanceBranchId: input.OpeningBalanceBranchId,
			OpeningBalance:         input.OpeningBalance,
			CreatedAt:              supplier.CreatedAt,
		}
		err = PublishToAccounting(ctx, tx, businessId, supplier.CreatedAt, supplier.ID, AccountReferenceTypeSupplierOpeningBalance, supplierOpeningBalance, nil, PubSubMessageActionCreate)
		if err != nil {
			tx.Rollback()
			return nil, err
		}
	} else if !input.OpeningBalance.IsZero() && !oldSupplier.OpeningBalance.IsZero() {
		var bill Bill
		err = tx.WithContext(ctx).Where("supplier_id = ? AND bill_number = ?", supplier.ID, "Supplier Opening Balance").First(&bill).Error
		if err != nil {
			tx.Rollback()
			return nil, err
		}
		if bill.CurrentStatus != BillStatusConfirmed {
			tx.Rollback()
			return nil, errors.New("bill already paid")
		} else {
			bill.BranchId = input.OpeningBalanceBranchId
			bill.CurrencyId = supplier.CurrencyId
			bill.ExchangeRate = input.ExchangeRate
			bill.BillSubtotal = input.OpeningBalance
			bill.BillTotalAmount = input.OpeningBalance
			bill.RemainingBalance = input.OpeningBalance
			err = tx.WithContext(ctx).Save(&bill).Error
			if err != nil {
				tx.Rollback()
				return nil, err
			}
		}
		oldSupplierOpeningBalance := SupplierOpeningBalance{
			ID:                     oldSupplier.ID,
			SupplierName:           oldSupplier.Name,
			CurrencyId:             oldSupplier.CurrencyId,
			ExchangeRate:           oldSupplier.ExchangeRate,
			OpeningBalanceBranchId: oldSupplier.OpeningBalanceBranchId,
			OpeningBalance:         oldSupplier.OpeningBalance,
			CreatedAt:              oldSupplier.CreatedAt,
		}
		supplierOpeningBalance := SupplierOpeningBalance{
			ID:                     supplier.ID,
			SupplierName:           input.Name,
			CurrencyId:             input.CurrencyId,
			ExchangeRate:           input.ExchangeRate,
			OpeningBalanceBranchId: input.OpeningBalanceBranchId,
			OpeningBalance:         input.OpeningBalance,
			CreatedAt:              supplier.CreatedAt,
		}
		err = PublishToAccounting(ctx, tx, businessId, supplier.CreatedAt, supplier.ID, AccountReferenceTypeSupplierOpeningBalance, supplierOpeningBalance, oldSupplierOpeningBalance, PubSubMessageActionUpdate)
		if err != nil {
			tx.Rollback()
			return nil, err
		}
	} else if input.OpeningBalance.IsZero() && !oldSupplier.OpeningBalance.IsZero() {
		supplierOpeningBalance := SupplierOpeningBalance{
			ID:                     supplier.ID,
			SupplierName:           input.Name,
			CurrencyId:             input.CurrencyId,
			ExchangeRate:           input.ExchangeRate,
			OpeningBalanceBranchId: input.OpeningBalanceBranchId,
			OpeningBalance:         input.OpeningBalance,
			CreatedAt:              supplier.CreatedAt,
		}
		err = PublishToAccounting(ctx, tx, businessId, supplierOpeningBalance.CreatedAt, supplierOpeningBalance.ID, AccountReferenceTypeSupplierOpeningBalance, nil, supplierOpeningBalance, PubSubMessageActionDelete)
		if err != nil {
			tx.Rollback()
			return nil, err
		}
	}

	if err := tx.Commit().Error; err != nil {
		return nil, err
	}
	return supplier, nil
}

func DeleteSupplier(ctx context.Context, id int) (*Supplier, error) {
	businessId, ok := utils.GetBusinessIdFromContext(ctx)
	if !ok || businessId == "" {
		return nil, errors.New("business id is required")
	}

	result, err := utils.FetchModel[Supplier](ctx, businessId, id, "Documents")
	if err != nil {
		return nil, utils.ErrorRecordNotFound
	}

	count, err := utils.ResourceCountWhere[Product](ctx, businessId, "supplier_id = ?", id)
	if err != nil {
		return nil, err
	}
	if count > 0 {
		return nil, errors.New("product associated with supplier exists")
	}

	count, err = utils.ResourceCountWhere[ProductGroup](ctx, businessId, "supplier_id = ?", id)
	if err != nil {
		return nil, err
	}
	if count > 0 {
		return nil, errors.New("product group order associated with supplier exists")
	}

	count, err = utils.ResourceCountWhere[PurchaseOrder](ctx, businessId, "supplier_id = ?", id)
	if err != nil {
		return nil, err
	}
	if count > 0 {
		return nil, errors.New("purchase order associated with supplier exists")
	}

	count, err = utils.ResourceCountWhere[Bill](ctx, businessId, "supplier_id = ?", id)
	if err != nil {
		return nil, err
	}
	if count > 0 {
		return nil, errors.New("bill associated with supplier exists")
	}

	count, err = utils.ResourceCountWhere[SupplierCredit](ctx, businessId, "supplier_id = ?", id)
	if err != nil {
		return nil, err
	}
	if count > 0 {
		return nil, errors.New("supplier credit associated with supplier exists")
	}

	count, err = utils.ResourceCountWhere[BankingTransaction](ctx, businessId, "supplier_id = ?", id)
	if err != nil {
		return nil, err
	}
	if count > 0 {
		return nil, errors.New("banking transaction associated with supplier exists")
	}

	count, err = utils.ResourceCountWhere[Expense](ctx, businessId, "supplier_id = ?", id)
	if err != nil {
		return nil, err
	}
	if count > 0 {
		return nil, errors.New("expense associated with supplier exists")
	}

	count, err = utils.ResourceCountWhere[Journal](ctx, businessId, "supplier_id = ?", id)
	if err != nil {
		return nil, err
	}
	if count > 0 {
		return nil, errors.New("journal associated with supplier exists")
	}

	db := config.GetDB()
	tx := db.Begin()
	err = tx.WithContext(ctx).Delete(&result).Error
	if err != nil {
		tx.Rollback()
		return nil, err
	}

	// clearing associated data
	// using Unscoped() to delete actual records instead of setting null value
	if err := tx.WithContext(ctx).Model(&result).Association("BillingAddress").Unscoped().Clear(); err != nil {
		tx.Rollback()
		return nil, err
	}
	if err := tx.WithContext(ctx).Model(&result).Association("ShippingAddress").Unscoped().Clear(); err != nil {
		tx.Rollback()
		return nil, err
	}
	if err := tx.WithContext(ctx).Model(&result).Association("ContactPersons").Unscoped().Clear(); err != nil {
		tx.Rollback()
		return nil, err
	}
	if err := deleteDocuments(ctx, tx, result.Documents); err != nil {
		tx.Rollback()
		return nil, err
	}
	if !result.OpeningBalance.IsZero() {
		err = tx.WithContext(ctx).Delete(&Bill{}, "supplier_id = ? AND bill_number = ?", result.ID, "Supplier Opening Balance").Error
		if err != nil {
			return nil, err
		}
		supplierOpeningBalance := SupplierOpeningBalance{
			ID:                     result.ID,
			SupplierName:           result.Name,
			CurrencyId:             result.CurrencyId,
			ExchangeRate:           result.ExchangeRate,
			OpeningBalanceBranchId: result.OpeningBalanceBranchId,
			OpeningBalance:         result.OpeningBalance,
			CreatedAt:              result.CreatedAt,
		}
		err = PublishToAccounting(ctx, tx, businessId, result.CreatedAt, result.ID, AccountReferenceTypeSupplierOpeningBalance, nil, supplierOpeningBalance, PubSubMessageActionDelete)
		if err != nil {
			tx.Rollback()
			return nil, err
		}
	}

	if err := tx.Commit().Error; err != nil {
		return nil, err
	}

	return result, nil

}

func GetSupplier(ctx context.Context, id int) (*Supplier, error) {
	return GetResource[Supplier](ctx, id)
}

func GetSuppliers(ctx context.Context, name *string) ([]*Supplier, error) {
	db := config.GetDB()
	var results []*Supplier

	businessId, ok := utils.GetBusinessIdFromContext(ctx)
	if !ok || businessId == "" {
		return nil, errors.New("business id is required")
	}

	dbCtx := db.WithContext(ctx).Where("business_id = ?", businessId)
	if name != nil && len(*name) > 0 {
		dbCtx = dbCtx.Where("name LIKE ?", "%"+*name+"%")
	}
	err := dbCtx.
		Find(&results).Error
	if err != nil {
		return nil, err
	}
	return results, nil
}

func ToggleActiveSupplier(ctx context.Context, id int, isActive bool) (*Supplier, error) {
	// ? should toggle active related products as well?
	businessId, ok := utils.GetBusinessIdFromContext(ctx)
	if !ok || businessId == "" {
		return nil, errors.New("business id is required")
	}

	return ToggleActiveModel[Supplier](ctx, businessId, id, isActive)
}

func PaginateSupplier(ctx context.Context, limit *int, after *string,
	name *string, phone *string, mobile *string, email *string, isActive *bool) (*SuppliersConnection, error) {
	businessId, ok := utils.GetBusinessIdFromContext(ctx)
	if !ok || businessId == "" {
		return nil, errors.New("business id is required")
	}

	db := config.GetDB()
	dbCtx := db.WithContext(ctx).Where("business_id = ?", businessId)
	if name != nil && *name != "" {
		dbCtx.Where("name LIKE ?", "%"+*name+"%")
	}
	if phone != nil && *phone != "" {
		dbCtx.Where("phone LIKE ?", "%"+*phone+"%")
	}
	if mobile != nil && *mobile != "" {
		dbCtx.Where("mobile LIKE ?", "%"+*mobile+"%")
	}
	if email != nil && *email != "" {
		dbCtx.Where("email LIKE ?", "%"+*email+"%")
	}
	if isActive != nil {
		dbCtx.Where("is_active = ?", isActive)
	}
	edges, pageInfo, err := FetchPageCompositeCursor[Supplier](dbCtx, *limit, after, "created_at", "<")
	if err != nil {
		return nil, err
	}
	var suppliersConnection SuppliersConnection
	suppliersConnection.PageInfo = pageInfo
	for _, edge := range edges {
		supplierEdge := SuppliersEdge(edge)
		suppliersConnection.Edges = append(suppliersConnection.Edges, &supplierEdge)
	}
	return &suppliersConnection, err
}

func GetTotalOutstandingPayable(ctx context.Context, supplierId int) (*decimal.Decimal, error) {
	db := config.GetDB()

	business, err := GetBusiness(ctx)
	if err != nil {
		return nil, err
	}

	var totalOutstanding decimal.Decimal

	status := []string{
		string(BillStatusConfirmed),
		string(BillStatusPartialPaid),
	}
	result := db.WithContext(ctx).Model(&Bill{}).
		Where("supplier_id = ?", supplierId).
		Where("current_status IN (?)", status).
		Select("COALESCE(SUM(CASE WHEN currency_id = ? THEN remaining_balance ELSE remaining_balance * exchange_rate END), 0)", business.BaseCurrencyId).
		Scan(&totalOutstanding)

	if result.Error != nil {
		return nil, result.Error
	}

	return &totalOutstanding, nil
}

func GetTotalUnusedSupplierCredit(ctx context.Context, supplierId int) (*decimal.Decimal, error) {
	db := config.GetDB()

	business, err := GetBusiness(ctx)
	if err != nil {
		return nil, err
	}

	var totalOutstandingCredit decimal.Decimal
	var totalOutstandingAdvance decimal.Decimal

	// status := []string{
	// 	string(SupplierCreditStatusClosed),
	// 	string(SupplierCreditStatusVoid),
	// }

	// for total outstanding credit
	result := db.WithContext(ctx).Model(&SupplierCredit{}).
		Where("supplier_id = ?", supplierId).
		Where("current_status = ?", SupplierCreditStatusConfirmed).
		// Select("COALESCE(SUM(remaining_balance), 0)").Scan(&totalOutstandingCredit)
		Select("COALESCE(SUM(CASE WHEN currency_id = ? THEN remaining_balance ELSE remaining_balance * exchange_rate END), 0)", business.BaseCurrencyId).
		Scan(&totalOutstandingCredit)
	if result.Error != nil {
		return nil, result.Error
	}

	// for total outstanding advance
	result = db.WithContext(ctx).Model(&SupplierCreditAdvance{}).
		Where("supplier_id = ?", supplierId).
		Where("current_status = ?", SupplierAdvanceStatusConfirmed).
		Select("COALESCE(SUM(CASE WHEN currency_id = ? THEN remaining_balance ELSE remaining_balance * exchange_rate END), 0)", business.BaseCurrencyId).
		Scan(&totalOutstandingAdvance)
	if result.Error != nil {
		return nil, result.Error
	}

	// Sum the total outstanding credit and advances
	totalOutstanding := totalOutstandingCredit.Add(totalOutstandingAdvance)

	return &totalOutstanding, nil
}
