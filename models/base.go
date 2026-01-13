package models

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"time"

	"bitbucket.org/mmdatafocus/books_backend/config"
	"bitbucket.org/mmdatafocus/books_backend/utils"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

// PublishToAccounting implements Phase 0 "transactional outbox":
// it writes the message record inside the caller's DB transaction but does NOT publish to Pub/Sub.
// Publishing is performed asynchronously by the outbox dispatcher after commit.
func PublishToAccounting(ctx context.Context, db *gorm.DB, businessId string, transactionDateTime time.Time, refId int, refType AccountReferenceType, obj interface{}, oldObj interface{}, msgAction PubSubMessageAction) error {

	var objInByte []byte
	var oldObjInByte []byte
	var err error

	if msgAction == PubSubMessageActionCreate || msgAction == PubSubMessageActionUpdate {
		// objInByte, err = json.Marshal(obj)
		objInByte, err = ToJSONWithoutField(obj, "Documents")
		if err != nil {
			return err
		}
	}
	if msgAction == PubSubMessageActionUpdate || msgAction == PubSubMessageActionDelete {
		// oldObjInByte, err = json.Marshal(oldObj)
		oldObjInByte, err = ToJSONWithoutField(oldObj, "Documents")
		if err != nil {
			return err
		}
	}

	record := PubSubMessageRecord{
		BusinessId:          businessId,
		TransactionDateTime: transactionDateTime,
		ReferenceId:         refId,
		ReferenceType:       refType,
		Action:              msgAction,
		NewObj:              objInByte,
		OldObj:              oldObjInByte,
		IsProcessed:         false,
		PublishStatus:       OutboxPublishStatusPending,
		CorrelationId:       correlationIdFromContextOrNew(ctx),
	}
	err = db.Create(&record).Error
	if err != nil {
		return err
	}
	return nil
}

func correlationIdFromContextOrNew(ctx context.Context) string {
	if ctx != nil {
		if v, ok := utils.GetCorrelationIdFromContext(ctx); ok && v != "" {
			return v
		}
	}
	return uuid.NewString()
}

// ToJSONWithoutField converts an object to JSON after temporarily removing a specified field
func ToJSONWithoutField(obj interface{}, fieldName string) ([]byte, error) {
	// Get the value of the object
	val := reflect.ValueOf(obj)

	// If the value is an interface, get the concrete value it holds
	if val.Kind() == reflect.Interface {
		val = val.Elem()
	}

	// If the value is not a pointer, create a pointer to it
	if val.Kind() != reflect.Ptr {
		valPtr := reflect.New(val.Type())
		valPtr.Elem().Set(val)
		val = valPtr
	}

	// Dereference the pointer
	val = val.Elem()

	// Ensure the value is a struct
	if val.Kind() != reflect.Struct {
		return nil, fmt.Errorf("expected a struct, got %v", val.Kind())
	}

	// Find the field by name
	field := val.FieldByName(fieldName)
	var err error
	var jsonData []byte
	if field.IsValid() {
		// Check if the field is a slice
		if field.Kind() == reflect.Slice {
			// Iterate over the slice elements
			for i := 0; i < field.Len(); i++ {
				elem := field.Index(i)
				if elem.Kind() == reflect.Struct {
					elemPtr := reflect.New(elem.Type())
					elemPtr.Elem().Set(elem)
					field.Index(i).Set(elemPtr.Elem())
				}
			}
		}

		// Store the original value of the field
		originalValue := reflect.New(field.Type()).Elem()
		originalValue.Set(field)

		// Clear the field value
		field.Set(reflect.Zero(field.Type()))

		// Convert the object to JSON
		jsonData, err = json.Marshal(val.Interface())

		// Restore the original value
		field.Set(originalValue)
	} else {
		// Convert the object to JSON
		jsonData, err = json.Marshal(val.Interface())
	}
	if err != nil {
		return nil, err
	}
	return jsonData, nil
}

// validate tax not knowing Individual or Group
// may return RecordNotFound error
func validateTaxExists(ctx context.Context, businessId string, taxId int, taxType TaxType) (err error) {
	if taxId == 0 {
		return nil
	}

	if taxType == TaxTypeIndividual {
		err = utils.ValidateResourceId[Tax](ctx, businessId, taxId)
	} else {
		err = utils.ValidateResourceId[TaxGroup](ctx, businessId, taxId)
	}
	if err != nil {
		return errors.New("tax not found")
	}
	return nil
}

// get transactionPrefix for module, redis or db
func getTransactionPrefix(ctx context.Context, branchId int, moduleName string) (string, error) {
	transactionPrefixes := make(map[string]string, 0) // moudleName => prefix
	redisKey := "tnsPrefixMap:" + fmt.Sprint(branchId)
	exists, err := config.GetRedisObject(redisKey, &transactionPrefixes)
	if err != nil {
		return "", err
	}
	if !exists {

		// retrieves moduleName:prefix map of the branch from db
		db := config.GetDB()
		var tnsId int
		if err := db.WithContext(ctx).Model(&Branch{}).
			Where("id = ?", branchId).Select("transaction_number_series_id").Scan(&tnsId).Error; err != nil {
			return "", err
		}
		var tnsModules []*TransactionNumberSeriesModule
		if err := db.WithContext(ctx).Model(&TransactionNumberSeriesModule{}).
			Where("series_id = ?", tnsId).Find(&tnsModules).Error; err != nil {
			return "", err
		}

		for _, modulePrefix := range tnsModules {
			transactionPrefixes[modulePrefix.ModuleName] = modulePrefix.Prefix
		}
		if err := config.SetRedisObject(redisKey, &transactionPrefixes, 0); err != nil {
			return "", err
		}
	}

	prefix, ok := transactionPrefixes[moduleName]
	if !ok || prefix == "" {
		// return "", errors.New("invalid module name")
		return "", nil
	}
	return prefix, nil
}

func calculateDueDate(date time.Time, paymentTerms PaymentTerms, customDays int) *time.Time {
	var dueDate time.Time
	switch terms := paymentTerms; terms {
	case PaymentTermsDueOnReceipt:
		dueDate = date
	case PaymentTermsNet15:
		dueDate = date.AddDate(0, 0, 15)
	case PaymentTermsNet30:
		dueDate = date.AddDate(0, 0, 30)
	case PaymentTermsNet45:
		dueDate = date.AddDate(0, 0, 45)
	case PaymentTermsNet60:
		dueDate = date.AddDate(0, 0, 60)
	case PaymentTermsDueEndOfMonth:
		year, month, _ := date.Date()
		firstOfMonth := time.Date(year, month, 1, 0, 0, 0, 0, date.Location())
		dueDate = firstOfMonth.AddDate(0, 1, -1)
	case PaymentTermsDueEndOfNextMonth:
		year, month, _ := date.Date()
		firstOfNextMonth := time.Date(year, month+1, 1, 0, 0, 0, 0, date.Location())
		dueDate = firstOfNextMonth.AddDate(0, 1, -1)
	case PaymentTermsCustom:
		dueDate = date.AddDate(0, 0, customDays)
	}
	return &dueDate
}

type TransactionLockType string

const (
	SalesTransactionLock      TransactionLockType = "SalesTransactionLock"
	PurchaseTransactionLock   TransactionLockType = "PurchaseTransactionLock"
	BankingTransactionLock    TransactionLockType = "BankingTransactionLock"
	AccountantTransactionLock TransactionLockType = "AccountantTransactionLock"
)

func validateTransactionLock(ctx context.Context, transactionDate time.Time, businessId string, lockType TransactionLockType) error {
	business, err := GetBusinessById(ctx, businessId)
	if err != nil {
		return err
	}
	var lockDate time.Time
	switch lockType {
	case SalesTransactionLock:
		lockDate = business.SalesTransactionLockDate
	case PurchaseTransactionLock:
		lockDate = business.PurchaseTransactionLockDate
	case BankingTransactionLock:
		lockDate = business.BankingTransactionLockDate
	case AccountantTransactionLock:
		lockDate = business.AccountantTransactionLockDate
	default:
		return errors.New("invalid transaction lock")
	}
	tDate, err := utils.ConvertToDate(transactionDate, business.Timezone)
	if err != nil {
		return err
	}
	lDate, err := utils.ConvertToDate(lockDate, business.Timezone)
	if err != nil {
		return err
	}
	if !tDate.After(lDate) {
		return errors.New("transaction has been locked")
	}
	mDate, err := utils.ConvertToDate(business.MigrationDate, business.Timezone)
	if err != nil {
		return err
	}
	if !tDate.After(mDate) {
		return errors.New("transaction prior to the migration date has been locked")
	}
	return nil
}

// ValidateTransactionLock enforces posting locks (period close) server-side.
// This is safe to call from both API mutations and async accounting workers.
func ValidateTransactionLock(ctx context.Context, transactionDate time.Time, businessId string, lockType TransactionLockType) error {
	return validateTransactionLock(ctx, transactionDate, businessId, lockType)
}

func ValidateValueAdjustment(ctx context.Context, businessId string, transactionDate time.Time, productType ProductType, productId int, batchNumber *string, sameday ...bool) error {
	db := config.GetDB()
	sqlTemp := `
	SELECT
		a.adjustment_date
	FROM
		inventory_adjustment_details ad
		LEFT JOIN inventory_adjustments a on ad.inventory_adjustment_id = a.id
	WHERE
		ad.product_id = @productId
		AND ad.product_type = @productType
		AND a.adjustment_date >= @transactionDate
		AND a.adjustment_type = 'Value'
		{{- if .batchNumber }} AND ad.batch_number = @batchNumber {{- end }}
		ORDER BY adjustment_date DESC LIMIT 1
`

	sql, err := utils.ExecTemplate(sqlTemp, map[string]interface{}{
		"batchNumber": utils.DereferencePtr(batchNumber),
	})
	if err != nil {
		return err
	}

	var adjustmentDate *time.Time

	business, err := GetBusinessById(ctx, businessId)
	if err != nil {
		return err
	}
	// timezone := business.Timezone
	// if timezone == "" {
	// 	timezone = "Asia/Yangon"
	// }

	// // Load the location for the given timezone
	// location, err := time.LoadLocation(timezone)
	// if err != nil {
	// 	fmt.Println("Error loading location:", err)
	// 	return err
	// }
	// transactionDateStartLocal := time.Date(
	// 	transactionDate.Year(),
	// 	transactionDate.Month(),
	// 	transactionDate.Day(),
	// 	// 23, 59, 59, 999, // Max nanoseconds
	// 	0, 0, 0, 0,
	// 	location,
	// )

	// // Convert to UTC
	// transactionDateStartUTC := transactionDateStartLocal.UTC()

	adjDate, err := utils.ConvertToDate(transactionDate, business.Timezone)
	if err != nil {
		return err
	}

	if len(sameday) == 0 || !sameday[0] {
		adjDate = time.Date(adjDate.Year(), adjDate.Month(), adjDate.Day(), 23, 59, 59, 999, adjDate.Location())
	}

	if err := db.WithContext(ctx).Raw(sql, map[string]interface{}{
		"productId":       productId,
		"productType":     productType,
		"transactionDate": adjDate,
		"batchNumber":     batchNumber,
	}).Scan(&adjustmentDate).Error; err != nil {
		return err
	}
	if adjustmentDate != nil {
		timezone := business.Timezone
		if timezone == "" {
			timezone = "Asia/Yangon"
		}

		// Load the location for the given timezone
		location, err := time.LoadLocation(timezone)
		if err != nil {
			fmt.Println("Error loading location:", err)
			return err
		}
		// Convert to local time
		localTime := adjustmentDate.In(location)

		// productName := ""
		// if productType == ProductTypeSingle {
		// 	product, err := GetProduct(ctx, productId)
		// 	if err == nil {
		// 		productName = product.Name
		// 	}
		// } else if productType == ProductTypeVariant {
		// 	product, err := GetProductVariant(ctx, productId)
		// 	if err == nil {
		// 		productName = product.Name
		// 	}
		// }

		return errors.New("not allowed. Value adjustment was done for %s on " + localTime.Format("2006-01-02"))
	}

	return nil
}

type ProductDetail struct {
	Id                 int
	Type               string
	InventoryAccountId int
	PurchaseAccountId  int
	PurchasePrice      decimal.Decimal
}

func ValidateProductId(ctx context.Context, businessId string, productId int, productType ProductType) error {
	if productId == 0 || productType == ProductTypeInput {
		return nil
	}

	var count int64
	db := config.GetDB()
	var table string
	switch productType {
	case ProductTypeSingle:
		table = "products"
	case ProductTypeGroup:
		table = "product_groups"
	case ProductTypeVariant:
		table = "product_variants"
	default:
		return errors.New("invalid product type")
	}
	if err := db.WithContext(ctx).Table(table).Where("business_id = ? AND id = ?", businessId, productId).Count(&count).Error; err != nil {
		return err
	}
	if count <= 0 {
		return errors.New("product not found")
	}
	return nil
}

// return current stock on hand (-1 if inventoryAccountId is zero or input product type)
func GetProductStock(tx *gorm.DB, ctx context.Context, businessId string, warehouseId int, batchNumber string, productType ProductType, productId int) (decimal.Decimal, error) {

	if productType == ProductTypeInput {
		return decimal.NewFromInt(-1), nil
	}

	currentStock := decimal.Zero

	if productType == ProductTypeSingle || productType == ProductTypeVariant {
		productInterface, err := GetProductOrVariant(ctx, string(productType), productId)
		if err != nil {
			return currentStock, err
		}
		if productInterface.GetInventoryAccountID() == 0 {
			return decimal.NewFromInt(-1), nil
		}
	}

	dbCtx := tx.WithContext(ctx).Model(&StockSummary{}).Where("business_id = ?", businessId)
	switch productType {
	case ProductTypeSingle:
		dbCtx.Where("product_type = 'S'")
	case ProductTypeGroup:
		dbCtx.Where("product_type = 'G'")
	case ProductTypeVariant:
		dbCtx.Where("product_type = 'V'")
	default:
		return currentStock, errors.New("invalid product type")
	}
	if err := dbCtx.Where("product_id = ? AND warehouse_id = ? AND batch_number = ?", productId, warehouseId, batchNumber).Select("current_qty").Scan(&currentStock).Error; err != nil {
		return currentStock, err
	}

	if currentStock.IsNegative() {
		return currentStock, errors.New("product stock cannot be negative")
	}
	return currentStock, nil
}

// validate whether the product has enough current stock for all product types (no need to check type forehand)
// using transaction to get the updated stock value which is not commited yet
func ValidateProductStock(tx *gorm.DB, ctx context.Context, businessId string, warehouseId int, batchNumber string, productType ProductType, productId int, outQty decimal.Decimal) error {

	currentStock, err := GetProductStock(tx, ctx, businessId, warehouseId, batchNumber, productType, productId)
	if err != nil {
		return err
	}

	// if inventory account id is -1 or input product type
	if currentStock.Equal(decimal.NewFromInt(-1)) {
		return nil
	}

	if currentStock.LessThan(outQty) {
		return errors.New("input qty is more than the current stock on hand")
	}

	return nil
}

func ValidateAccountReference(ctx context.Context, businessId string, referenceId int, referenceType AccountReferenceType) error {
	tableNames := map[AccountReferenceType]string{
		AccountReferenceTypeBill:                        "bills",
		AccountReferenceTypeInvoice:                     "sales_invoices",
		AccountReferenceTypeJournal:                     "journals",
		AccountReferenceTypeCustomerPayment:             "customer_payments",
		AccountReferenceTypeCreditNote:                  "credit_notes",
		AccountReferenceTypeExpense:                     "expenses",
		AccountReferenceTypeInventoryAdjustmentQuantity: "inventory_adjustments",
		AccountReferenceTypeInventoryAdjustmentValue:    "inventory_adjustments",
		AccountReferenceTypeSupplierPayment:             "supplier_payments",
		AccountReferenceTypeOpeningBalance:              "opening_balances",
		AccountReferenceTypeSupplierCredit:              "supplier_credits",
		AccountReferenceTypeSupplierAdvanceApplied:      "supplier_credit_bills",
		AccountReferenceTypeTransferOrder:               "transfer_orders",

		// don't know how to validate
		AccountReferenceTypeCreditNoteRefund:      "",
		AccountReferenceTypeCustomerAdvanceRefund: "",
		AccountReferenceTypeExpenseRefund:         "",
		AccountReferenceTypeSupplierCreditRefund:  "",
		AccountReferenceTypeSupplierAdvanceRefund: "",

		AccountReferenceTypeProductOpeningStock:          "",
		AccountReferenceTypeProductGroupOpeningStock:     "",
		AccountReferenceTypeProductCompositeOpeningStock: "",
		AccountReferenceTypeCustomerAdvanceApplied:       "",
		AccountReferenceTypeInvoiceWriteOff:              "",
		AccountReferenceTypeAdvanceCustomerPayment:       "",
		AccountReferenceTypeAdvanceSupplierPayment:       "",
		AccountReferenceTypeCustomerOpeningBalance:       "",
		AccountReferenceTypeSupplierOpeningBalance:       "",
		AccountReferenceTypeOwnerDrawing:                 "",
		AccountReferenceTypeOwnerContribution:            "",
		AccountReferenceTypeAccountTransfer:              "",
		AccountReferenceTypeAccountDeposit:               "",
		AccountReferenceTypeOtherIncome:                  "",
	}
	tableName, ok := tableNames[referenceType]
	if !ok {
		return errors.New("invalid reference type")
	}

	if tableName == "" {
		return nil
	}

	db := config.GetDB()
	var count int64
	dbCtx := db.WithContext(ctx).Where("business_id = ? AND id =?", businessId, referenceId)
	if err := dbCtx.Table(tableName).Count(&count).Error; err != nil {
		return err
	}
	if count <= 0 {
		return errors.New("account reference does not exist")
	}

	return nil
}

func ParseDateString(dateString string, timezone string) (time.Time, error) {

	// Parse the date string into a time.Time object
	localTime, err := time.Parse("2006-01-02T15:04:05", dateString)
	if err != nil {
		fmt.Println("Error parsing date:", err)
		return time.Time{}, err
	}

	if timezone == "" {
		timezone = "Asia/Yangon"
	}

	// Load the location for the given timezone
	location, err := time.LoadLocation(timezone)
	if err != nil {
		fmt.Println("Error loading location:", err)
		return time.Time{}, err
	}

	// Convert the local time to the specified timezone
	localTimeInZone := time.Date(
		localTime.Year(), localTime.Month(), localTime.Day(),
		localTime.Hour(), localTime.Minute(), localTime.Second(), localTime.Nanosecond(),
		location,
	)

	// Convert the time to UTC
	return localTimeInZone.UTC(), nil
}

func IsRealProduct(ctx context.Context, businessId string, productId int, productType ProductType) bool {

	if productType == ProductTypeInput {
		return false
	}
	var table string
	switch productType {
	case ProductTypeSingle:
		table = "products"
	case ProductTypeGroup:
		table = "product_groups"
	case ProductTypeVariant:
		table = "product_variants"
	default:
		return false
	}

	var count int64
	db := config.GetDB()
	if err := db.WithContext(ctx).Table(table).Where("business_id = ? AND id = ? AND inventory_account_id > 0", businessId, productId).
		Count(&count).Error; err != nil {
		// returning false in case of an error
		return false
	}
	if count <= 0 {
		return false
	}
	return true
}
