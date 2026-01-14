package models

import (
	"context"
	"errors"
	"time"

	"github.com/mmdatafocus/books_backend/config"
	"github.com/mmdatafocus/books_backend/utils"
	"github.com/shopspring/decimal"
)

type Customer struct {
	ID                             int              `gorm:"primary_key" json:"id"`
	BusinessId                     string           `gorm:"index;not null" json:"business_id" binding:"required"`
	Name                           string           `gorm:"size:100;not null" json:"name" binding:"required"`
	Email                          string           `gorm:"size:100" json:"email"`
	Phone                          string           `gorm:"size:20" json:"phone"`
	Mobile                         string           `gorm:"size:20" json:"mobile"`
	CurrencyId                     int              `gorm:"not null" json:"currency_id" binding:"required"`
	ExchangeRate                   decimal.Decimal  `gorm:"type:decimal(20,4);default:0" json:"exchange_rate"`
	CustomerTaxId                  int              `json:"customer_tax_id"`
	CustomerTaxType                *TaxType         `gorm:"type:enum('I','G');default:'I'" json:"customer_tax_type"`
	CustomerPaymentTerms           PaymentTerms     `gorm:"type:enum('Net15', 'Net30', 'Net45', 'Net60', 'DueMonthEnd', 'DueNextMonthEnd', 'DueOnReceipt', 'Custom');not null;default:'DueOnReceipt'" json:"customer_payment_terms" binding:"required"`
	CustomerPaymentTermsCustomDays int              `gorm:"default:0" json:"customer_payment_terms_custom_days"`
	Notes                          string           `gorm:"type:text" json:"notes"`
	CreditLimit                    decimal.Decimal  `gorm:"type:decimal(20,4);default:0" json:"credit_limit"`
	BillingAddress                 BillingAddress   `gorm:"polymorphic:Reference" json:"billing_address"`
	ShippingAddress                ShippingAddress  `gorm:"polymorphic:Reference" json:"shipping_address"`
	ContactPersons                 []*ContactPerson `gorm:"polymorphic:Reference" json:"contact_persons"`
	Documents                      []*Document      `gorm:"polymorphic:Reference" json:"documents"`
	IsActive                       *bool            `gorm:"not null;default:true" json:"is_active"`
	OpeningBalanceBranchId         int              `gorm:"default:0" json:"opening_balance_branch_id"`
	OpeningBalance                 decimal.Decimal  `gorm:"type:decimal(20,4);default:0" json:"opening_balance"`
	CreatedAt                      time.Time        `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt                      time.Time        `gorm:"autoUpdateTime" json:"updated_at"`
}

type CustomerOpeningBalance struct { // use in PubSub
	ID                     int             `json:"id"`
	CustomerName           string          `json:"customer_name"`
	CurrencyId             int             `json:"currency_id"`
	ExchangeRate           decimal.Decimal `json:"exchange_rate"`
	OpeningBalanceBranchId int             `json:"opening_balance_branch_id"`
	OpeningBalance         decimal.Decimal `json:"opening_balance"`
	CreatedAt              time.Time       `json:"created_at"`
}

type NewCustomer struct {
	Name                           string              `json:"name" binding:"required"`
	Email                          string              `json:"email"`
	Phone                          string              `json:"phone"`
	Mobile                         string              `json:"mobile"`
	CurrencyId                     int                 `json:"currency_id" binding:"required"`
	ExchangeRate                   decimal.Decimal     `json:"exchange_rate"`
	CustomerTaxId                  int                 `json:"customer_tax_id"`
	CustomerTaxType                *TaxType            `json:"customer_tax_type"`
	OpeningBalanceBranchId         int                 `json:"opening_balance_branch_id"`
	OpeningBalance                 decimal.Decimal     `json:"opening_balance"`
	CustomerPaymentTerms           PaymentTerms        `json:"customer_payment_terms" binding:"required"`
	CustomerPaymentTermsCustomDays int                 `json:"customer_payment_terms_custom_days"`
	Notes                          string              `json:"notes"`
	CreditLimit                    decimal.Decimal     `json:"credit_limit"`
	BillingAddress                 *NewBillingAddress  `json:"billing_address"`
	ShippingAddress                *NewShippingAddress `json:"shipping_address"`
	ContactPersons                 []*NewContactPerson `json:"contact_persons"`
	Documents                      []*NewDocument      `json:"documents"`
}

type CustomersEdge Edge[Customer]
type CustomersConnection struct {
	Edges    []*CustomersEdge `json:"edges"`
	PageInfo *PageInfo        `json:"pageInfo"`
}

// returns decoded curosr string
func (c Customer) GetCursor() string {
	return c.CreatedAt.String()
}

// When creating customer with opening balance, create account transactions "CustomerOpeningBalance" (credit to Sales, debit to AccountReceivable)
// Don't allow to update opening balance
// Don't delete if used in transactions

// CreateCustomer(newCustomer) (Customer,error) <Owner/Custom>
// UpdateCustomer(id, newCustomer) (Customer,error) <Owner/Custom>
// DeleteCustomer(id) (Customer,error) <Owner/Custom>

// GetCustomer(id) (Customer,error) <Owner/Custom>
// ListCustomer(businessId, name, code) ([]Customer,error) <Owner/Custom>

// ToggleActiveCustomer(id, isActive) (Customer,error) <Owner/Custom>

func (input *NewCustomer) validate(ctx context.Context, businessId string, id int) error {
	if id > 0 {
		if err := utils.ValidateResourceId[Customer](ctx, businessId, id); err != nil {
			return err
		}
	}
	// validate unique name
	if err := utils.ValidateUnique[Customer](ctx, businessId, "name", input.Name, id); err != nil {
		return err
	}
	// validate email
	if input.Email != "" && len(input.Email) > 0 {
		if err := utils.ValidateUnique[Customer](ctx, businessId, "email", input.Email, id); err != nil {
			return err
		}
	}
	// validate phone
	if input.Phone != "" && len(input.Phone) > 0 {
		if err := utils.ValidateUnique[Customer](ctx, businessId, "phone", input.Phone, id); err != nil {
			return err
		}
	}
	// validate currency
	if err := utils.ValidateResourceId[Currency](ctx, businessId, input.CurrencyId); err != nil {
		return errors.New("currency not found")
	}
	// validate tax
	if input.CustomerTaxType != nil {
		if err := validateTaxExists(ctx, businessId, input.CustomerTaxId, *input.CustomerTaxType); err != nil {
			return errors.New("tax not found")
		}
	}
	return nil
}

func CreateCustomer(ctx context.Context, input *NewCustomer) (*Customer, error) {
	db := config.GetDB()
	contactPersons := mapNewContactPersons(input.ContactPersons, "customers", 0)
	documents, err := mapNewDocuments(input.Documents, "customers", 0)
	if err != nil {
		return nil, err
	}

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

	customer := Customer{
		BusinessId:                     businessId,
		Name:                           input.Name,
		Email:                          input.Email,
		Phone:                          input.Phone,
		Mobile:                         input.Mobile,
		CurrencyId:                     input.CurrencyId,
		ExchangeRate:                   input.ExchangeRate,
		CustomerTaxId:                  input.CustomerTaxId,
		CustomerTaxType:                input.CustomerTaxType,
		CustomerPaymentTerms:           input.CustomerPaymentTerms,
		CustomerPaymentTermsCustomDays: input.CustomerPaymentTermsCustomDays,
		Notes:                          input.Notes,
		CreditLimit:                    input.CreditLimit,
		ContactPersons:                 contactPersons,
		Documents:                      documents,
		IsActive:                       utils.NewTrue(),
		OpeningBalanceBranchId:         input.OpeningBalanceBranchId,
		OpeningBalance:                 input.OpeningBalance,
	}

	if input.BillingAddress != nil {
		customer.BillingAddress = mapBillingAddressInput(*input.BillingAddress)
	}
	if input.ShippingAddress != nil {
		customer.ShippingAddress = mapShippingAddressInput(*input.ShippingAddress)
	}

	tx := db.Begin()
	err = tx.WithContext(ctx).Create(&customer).Error
	if err != nil {
		tx.Rollback()
		return nil, err
	}

	if !input.OpeningBalance.IsZero() {
		invoice := SalesInvoice{
			BusinessId:                    businessId,
			CustomerId:                    customer.ID,
			BranchId:                      input.OpeningBalanceBranchId,
			InvoiceDate:                   business.MigrationDate,
			InvoiceNumber:                 "Customer Opening Balance",
			SequenceNo:                    decimal.NewFromInt(0),
			InvoicePaymentTerms:           PaymentTermsDueOnReceipt,
			InvoicePaymentTermsCustomDays: 0,
			InvoiceDueDate:                &business.MigrationDate,
			CurrencyId:                    customer.CurrencyId,
			ExchangeRate:                  input.ExchangeRate,
			InvoiceSubtotal:               input.OpeningBalance,
			InvoiceTotalAmount:            input.OpeningBalance,
			RemainingBalance:              input.OpeningBalance,
			CurrentStatus:                 SalesInvoiceStatusConfirmed,
			WarehouseId:                   0,
		}
		err = tx.WithContext(ctx).Create(&invoice).Error
		if err != nil {
			return nil, err
		}

		customerOpeningBalance := CustomerOpeningBalance{
			ID:                     customer.ID,
			CustomerName:           customer.Name,
			CurrencyId:             customer.CurrencyId,
			ExchangeRate:           input.ExchangeRate,
			OpeningBalanceBranchId: input.OpeningBalanceBranchId,
			OpeningBalance:         input.OpeningBalance,
			CreatedAt:              customer.CreatedAt,
		}
		err = PublishToAccounting(ctx, tx, businessId, customer.CreatedAt, customer.ID, AccountReferenceTypeCustomerOpeningBalance, customerOpeningBalance, nil, PubSubMessageActionCreate)
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

	return &customer, nil
}

func UpdateCustomer(ctx context.Context, id int, input *NewCustomer) (*Customer, error) {
	businessId, ok := utils.GetBusinessIdFromContext(ctx)
	if !ok || businessId == "" {
		return nil, errors.New("business id is required")
	}

	if err := input.validate(ctx, businessId, id); err != nil {
		return nil, err
	}

	oldCustomer, err := utils.FetchModel[Customer](ctx, businessId, id)
	if err != nil {
		return nil, err
	}

	customer, err := utils.FetchModel[Customer](ctx, businessId, id)
	if err != nil {
		return nil, err
	}
	db := config.GetDB()
	tx := db.Begin()
	err = tx.WithContext(ctx).Model(&customer).Updates(map[string]interface{}{
		"Name":                           input.Name,
		"Email":                          input.Email,
		"Phone":                          input.Phone,
		"Mobile":                         input.Mobile,
		"CurrencyId":                     input.CurrencyId,
		"CustomerTaxId":                  input.CustomerTaxId,
		"CustomerTaxType":                input.CustomerTaxType,
		"CustomerPaymentTerms":           input.CustomerPaymentTerms,
		"CustomerPaymentTermsCustomDays": input.CustomerPaymentTermsCustomDays,
		"Notes":                          input.Notes,
		"CreditLimit":                    input.CreditLimit,
		"OpeningBalanceBranchId":         input.OpeningBalanceBranchId,
		"OpeningBalance":                 input.OpeningBalance,
		// "ContactPersons":                 customer.ContactPersons,
		// "Documents":                      customer.Documents,
	}).Error
	if err != nil {
		tx.Rollback()
		return nil, err
	}

	// updating related tables
	if input.BillingAddress != nil {
		if err := upsertBillingAddress(tx, ctx, *input.BillingAddress, "customers", id); err != nil {
			tx.Rollback()
			return nil, err
		}
	}
	if input.ShippingAddress != nil {
		if err := upsertShippingAddress(tx, ctx, *input.ShippingAddress, "customers", id); err != nil {
			tx.Rollback()
			return nil, err
		}
	}
	contactPersons, err := upsertContactPersons(ctx, tx, input.ContactPersons, "customers", id)
	if err != nil {
		tx.Rollback()
		return nil, err
	}
	customer.ContactPersons = contactPersons
	documents, err := upsertDocuments(ctx, tx, input.Documents, "customers", id)
	if err != nil {
		tx.Rollback()
		return nil, err
	}
	customer.Documents = documents

	if !input.OpeningBalance.IsZero() && oldCustomer.OpeningBalance.IsZero() {
		business, err := GetBusinessById(ctx, businessId)
		if err != nil {
			tx.Rollback()
			return nil, err
		}
		invoice := SalesInvoice{
			BusinessId:                    businessId,
			CustomerId:                    customer.ID,
			BranchId:                      input.OpeningBalanceBranchId,
			InvoiceDate:                   business.MigrationDate,
			InvoiceNumber:                 "Customer Opening Balance",
			SequenceNo:                    decimal.NewFromInt(0),
			InvoicePaymentTerms:           PaymentTermsDueOnReceipt,
			InvoicePaymentTermsCustomDays: 0,
			InvoiceDueDate:                &business.MigrationDate,
			CurrencyId:                    customer.CurrencyId,
			ExchangeRate:                  input.ExchangeRate,
			InvoiceSubtotal:               input.OpeningBalance,
			InvoiceTotalAmount:            input.OpeningBalance,
			RemainingBalance:              input.OpeningBalance,
			CurrentStatus:                 SalesInvoiceStatusConfirmed,
			WarehouseId:                   0,
		}
		err = tx.WithContext(ctx).Create(&invoice).Error
		if err != nil {
			return nil, err
		}

		customerOpeningBalance := CustomerOpeningBalance{
			ID:                     customer.ID,
			CustomerName:           customer.Name,
			CurrencyId:             customer.CurrencyId,
			ExchangeRate:           input.ExchangeRate,
			OpeningBalanceBranchId: input.OpeningBalanceBranchId,
			OpeningBalance:         input.OpeningBalance,
			CreatedAt:              customer.CreatedAt,
		}
		err = PublishToAccounting(ctx, tx, businessId, customer.CreatedAt, customer.ID, AccountReferenceTypeCustomerOpeningBalance, customerOpeningBalance, nil, PubSubMessageActionCreate)
		if err != nil {
			tx.Rollback()
			return nil, err
		}
	} else if !input.OpeningBalance.IsZero() && !oldCustomer.OpeningBalance.IsZero() {
		var invoice SalesInvoice
		err = tx.WithContext(ctx).Where("customer_id = ? AND invoice_number = ?", customer.ID, "Customer Opening Balance").First(&invoice).Error
		if err != nil {
			tx.Rollback()
			return nil, err
		}
		if invoice.CurrentStatus != SalesInvoiceStatusConfirmed {
			tx.Rollback()
			return nil, errors.New("invoice already paid")
		} else {
			invoice.BranchId = input.OpeningBalanceBranchId
			invoice.CurrencyId = customer.CurrencyId
			invoice.ExchangeRate = input.ExchangeRate
			invoice.InvoiceSubtotal = input.OpeningBalance
			invoice.InvoiceTotalAmount = input.OpeningBalance
			invoice.RemainingBalance = input.OpeningBalance
			err = tx.WithContext(ctx).Save(&invoice).Error
			if err != nil {
				tx.Rollback()
				return nil, err
			}
		}
		oldCustomerOpeningBalance := CustomerOpeningBalance{
			ID:                     oldCustomer.ID,
			CustomerName:           oldCustomer.Name,
			CurrencyId:             oldCustomer.CurrencyId,
			ExchangeRate:           oldCustomer.ExchangeRate,
			OpeningBalanceBranchId: oldCustomer.OpeningBalanceBranchId,
			OpeningBalance:         oldCustomer.OpeningBalance,
			CreatedAt:              oldCustomer.CreatedAt,
		}
		customerOpeningBalance := CustomerOpeningBalance{
			ID:                     customer.ID,
			CustomerName:           customer.Name,
			CurrencyId:             customer.CurrencyId,
			ExchangeRate:           input.ExchangeRate,
			OpeningBalanceBranchId: input.OpeningBalanceBranchId,
			OpeningBalance:         input.OpeningBalance,
			CreatedAt:              customer.CreatedAt,
		}
		err = PublishToAccounting(ctx, tx, businessId, customer.CreatedAt, customer.ID, AccountReferenceTypeCustomerOpeningBalance, customerOpeningBalance, oldCustomerOpeningBalance, PubSubMessageActionUpdate)
		if err != nil {
			tx.Rollback()
			return nil, err
		}
	} else if input.OpeningBalance.IsZero() && !oldCustomer.OpeningBalance.IsZero() {
		customerOpeningBalance := CustomerOpeningBalance{
			ID:                     customer.ID,
			CustomerName:           input.Name,
			CurrencyId:             input.CurrencyId,
			ExchangeRate:           input.ExchangeRate,
			OpeningBalanceBranchId: input.OpeningBalanceBranchId,
			OpeningBalance:         input.OpeningBalance,
			CreatedAt:              customer.CreatedAt,
		}
		err = PublishToAccounting(ctx, tx, businessId, customerOpeningBalance.CreatedAt, customerOpeningBalance.ID, AccountReferenceTypeCustomerOpeningBalance, nil, customerOpeningBalance, PubSubMessageActionDelete)
		if err != nil {
			tx.Rollback()
			return nil, err
		}
	}

	if err := tx.Commit().Error; err != nil {
		return nil, err
	}
	return customer, nil
}

func DeleteCustomer(ctx context.Context, id int) (*Customer, error) {
	businessId, ok := utils.GetBusinessIdFromContext(ctx)
	if !ok || businessId == "" {
		return nil, errors.New("business id is required")
	}

	result, err := utils.FetchModel[Customer](ctx, businessId, id, "Documents")
	if err != nil {
		return nil, err
	}

	count, err := utils.ResourceCountWhere[SalesOrder](ctx, businessId, "customer_id = ?", id)
	if err != nil {
		return nil, err
	}
	if count > 0 {
		return nil, errors.New("sales order associated with customer exists")
	}

	count, err = utils.ResourceCountWhere[SalesInvoice](ctx, businessId, "customer_id = ?", id)
	if err != nil {
		return nil, err
	}
	if count > 0 {
		return nil, errors.New("sales invoice associated with customer exists")
	}

	count, err = utils.ResourceCountWhere[CreditNote](ctx, businessId, "customer_id = ?", id)
	if err != nil {
		return nil, err
	}
	if count > 0 {
		return nil, errors.New("credite note associated with customer exists")
	}

	count, err = utils.ResourceCountWhere[BankingTransaction](ctx, businessId, "customer_id = ?", id)
	if err != nil {
		return nil, err
	}
	if count > 0 {
		return nil, errors.New("banking transaction associated with customer exists")
	}

	count, err = utils.ResourceCountWhere[Expense](ctx, businessId, "customer_id = ?", id)
	if err != nil {
		return nil, err
	}
	if count > 0 {
		return nil, errors.New("expense associated with customer exists")
	}

	count, err = utils.ResourceCountWhere[Journal](ctx, businessId, "customer_id = ?", id)
	if err != nil {
		return nil, err
	}
	if count > 0 {
		return nil, errors.New("journal associated with customer exists")
	}

	db := config.GetDB()
	tx := db.Begin()
	err = tx.WithContext(ctx).Delete(&result).Error
	if err != nil {
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
		err = tx.WithContext(ctx).Delete(&SalesInvoice{}, "customer_id = ? AND invoice_number = ?", result.ID, "Customer Opening Balance").Error
		if err != nil {
			tx.Rollback()
			return nil, err
		}
		customerOpeningBalance := CustomerOpeningBalance{
			ID:                     result.ID,
			CustomerName:           result.Name,
			CurrencyId:             result.CurrencyId,
			ExchangeRate:           result.ExchangeRate,
			OpeningBalanceBranchId: result.OpeningBalanceBranchId,
			OpeningBalance:         result.OpeningBalance,
			CreatedAt:              result.CreatedAt,
		}
		err = PublishToAccounting(ctx, tx, businessId, result.CreatedAt, result.ID, AccountReferenceTypeCustomerOpeningBalance, nil, customerOpeningBalance, PubSubMessageActionDelete)
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

func ToggleActiveCustomer(ctx context.Context, id int, isActive bool) (*Customer, error) {
	businessId, ok := utils.GetBusinessIdFromContext(ctx)
	if !ok || businessId == "" {
		return nil, errors.New("business id is required")
	}

	return ToggleActiveModel[Customer](ctx, businessId, id, isActive)
}

func GetCustomer(ctx context.Context, id int) (*Customer, error) {
	// fieldNames, err := utils.GetQueryFields(ctx, &Customer{})
	// if err != nil {
	// 	return nil, err
	// }

	businessId, ok := utils.GetBusinessIdFromContext(ctx)
	if !ok || businessId == "" {
		return nil, errors.New("business id is required")
	}

	return utils.FetchModel[Customer](ctx, businessId, id)
}

func GetCustomers(ctx context.Context, name *string) ([]*Customer, error) {
	db := config.GetDB()
	// fieldNames, err := utils.GetQueryFields(ctx, &Customer{})
	// if err != nil {
	// 	return nil, err
	// }

	businessId, ok := utils.GetBusinessIdFromContext(ctx)
	if !ok || businessId == "" {
		return nil, errors.New("business id is required")
	}

	var results []*Customer
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

func PaginateCustomer(ctx context.Context, limit *int, after *string,
	name *string, phone *string, mobile *string, email *string, isActive *bool) (*CustomersConnection, error) {

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

	edges, pageInfo, err := FetchPageCompositeCursor[Customer](dbCtx, *limit, after, "created_at", "<")
	if err != nil {
		return nil, err
	}

	var customersConnection CustomersConnection
	customersConnection.PageInfo = pageInfo
	for _, edge := range edges {
		customerEdge := CustomersEdge(edge)
		customersConnection.Edges = append(customersConnection.Edges, &customerEdge)
	}

	return &customersConnection, err
}

func GetTotalOutstandingReceivable(ctx context.Context, customerId int) (*decimal.Decimal, error) {
	db := config.GetDB()

	business, err := GetBusiness(ctx)
	if err != nil {
		return nil, err
	}

	var totalOutstanding decimal.Decimal

	status := []string{
		string(SalesInvoiceStatusConfirmed),
		string(SalesInvoiceStatusPartialPaid),
	}
	result := db.WithContext(ctx).Model(&SalesInvoice{}).
		Where("customer_id = ?", customerId).
		Where("current_status IN (?)", status).
		Select("COALESCE(SUM(CASE WHEN currency_id = ? THEN remaining_balance ELSE remaining_balance * exchange_rate END), 0)", business.BaseCurrencyId).
		Scan(&totalOutstanding)

	if result.Error != nil {
		return nil, result.Error
	}

	return &totalOutstanding, nil
}

func GetTotalUnusedCustomerCredit(ctx context.Context, customerId int) (*decimal.Decimal, error) {
	db := config.GetDB()

	business, err := GetBusiness(ctx)
	if err != nil {
		return nil, err
	}

	var totalOutstandingCredit decimal.Decimal
	var totalOutstandingAdvance decimal.Decimal

	// creditNoteStatuses := []string{
	// 	string(CreditNoteStatusClosed),
	// 	string(CreditNoteStatusVoid),
	// }

	// Query for total outstanding credit
	result := db.WithContext(ctx).Model(&CreditNote{}).
		Where("customer_id = ?", customerId).
		Where("current_status = ?", CreditNoteStatusConfirmed).
		// Select("COALESCE(SUM(remaining_balance), 0)")
		Select("COALESCE(SUM(CASE WHEN currency_id = ? THEN remaining_balance ELSE remaining_balance * exchange_rate END), 0)", business.BaseCurrencyId).
		Scan(&totalOutstandingCredit)
	if result.Error != nil {
		return nil, result.Error
	}

	// Query for total outstanding advance
	result = db.WithContext(ctx).Model(&CustomerCreditAdvance{}).
		Where("customer_id = ?", customerId).
		Where("current_status = ?", CustomerAdvanceStatusConfirmed).
		Select("COALESCE(SUM(CASE WHEN currency_id = ? THEN remaining_balance ELSE remaining_balance * exchange_rate END), 0)", business.BaseCurrencyId).
		Scan(&totalOutstandingAdvance)
	if result.Error != nil {
		return nil, result.Error
	}

	// Sum the total outstanding credit and advances
	totalOutstanding := totalOutstandingCredit.Add(totalOutstandingAdvance)

	return &totalOutstanding, nil
}
