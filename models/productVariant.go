package models

import (
	"context"
	"errors"
	"time"

	"bitbucket.org/mmdatafocus/books_backend/config"
	"bitbucket.org/mmdatafocus/books_backend/utils"
	"github.com/shopspring/decimal"
)

type ProductVariant struct {
	ID                  int             `gorm:"primary_key" json:"id"`
	BusinessId          string          `gorm:"index;not null" json:"business_id" binding:"required"`
	ProductGroupId      int             `gorm:"index;not null" json:"product_group_id" binding:"required"`
	Name                string          `gorm:"size:255;not null" json:"name" binding:"required"`
	Sku                 string          `gorm:"size:100;not null" json:"sku" binding:"required"`
	Barcode             string          `gorm:"index;size:100;not null" json:"barcode" binding:"required"`
	UnitId              int             `json:"product_unit_id"`
	SalesPrice          decimal.Decimal `gorm:"type:decimal(20,4);default:0" json:"sales_price"`
	SalesAccountId      int             `json:"sales_account_id"`
	SalesTaxId          int             `json:"sales_tax_id"`
	SalesTaxType        *TaxType        `gorm:"type:enum('I', 'G'); default:null" json:"sales_tax_type"`
	IsSalesTaxInclusive *bool           `gorm:"not null;default:true" json:"is_sales_tax_inclusive"`
	PurchasePrice       decimal.Decimal `gorm:"type:decimal(20,4);default:0" json:"purchase_price"`
	PurchaseAccountId   int             `json:"purchase_account_id"`
	PurchaseTaxId       int             `json:"purchase_tax_id"`
	PurchaseTaxType     *TaxType        `gorm:"type:enum('I', 'G'); default:null" json:"purchase_tax_type"`
	InventoryAccountId  int             `json:"inventory_account_id"`
	IsBatchTracking     *bool           `gorm:"not null;default:false" json:"is_batch_traking"`
	IsActive            *bool           `gorm:"not null;default:true" json:"is_active"`
	ExternalSystemId    string          `gorm:"index" json:"external_system_id"`
	CreatedAt           time.Time       `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt           time.Time       `gorm:"autoUpdateTime" json:"updated_at"`
}

type NewProductVariant struct {
	VariantId           int             `json:"variant_id"`
	ProductGroupId      int             `json:"product_id" binding:"required"`
	Name                string          `json:"name" binding:"required"`
	Sku                 string          `json:"sku" binding:"required"`
	Barcode             string          `json:"barcode" binding:"required"`
	UnitId              int             `json:"product_unit_id"`
	SalesPrice          decimal.Decimal `json:"sales_price"`
	SalesAccountId      int             `json:"sales_account_id"`
	SalesTaxId          int             `json:"sales_tax_id"`
	SalesTaxType        *TaxType        `json:"sales_tax_type"`
	IsSalesTaxInclusive *bool           `json:"is_sales_tax_inclusive"`
	PurchasePrice       decimal.Decimal `json:"purchase_price"`
	PurchaseAccountId   int             `json:"purchase_account_id"`
	PurchaseTaxId       int             `json:"purchase_tax_id"`
	PurchaseTaxType     *TaxType        `json:"purchase_tax_type"`
	InventoryAccountId  int             `json:"inventory_account_id"`
	IsBatchTracking     *bool           `json:"is_batch_traking"`
}

type AllProductVariant struct {
	ID                  int             `json:"id"`
	ProductGroupID      int             `json:"productGroupId"`
	Name                string          `json:"name"`
	Sku                 *string         `json:"sku,omitempty"`
	Barcode             *string         `json:"barcode,omitempty"`
	UnitId              int             `json:"product_unit_id"`
	SalesPrice          decimal.Decimal `json:"salesPrice"`
	PurchasePrice       decimal.Decimal `json:"purchasePrice"`
	IsSalesTaxInclusive bool            `json:"isSalesTaxInclusive"`
	PurchaseAccountId   int             `json:"purchase_account_id"`
	PurchaseTaxId       int             `json:"purchase_tax_id"`
	PurchaseTaxType     *TaxType        `json:"purchase_tax_type"`

	SalesAccountId     int      `json:"sales_account_id"`
	SalesTaxId         int      `json:"sales_tax_id"`
	SalesTaxType       *TaxType `json:"sales_tax_type"`
	InventoryAccountId int      `json:"inventory_account_id"`
	// ProductUnit         *AllProductUnit `json:"productUnit,omitempty"`
	// Stocks              []*ProductStock `json:"stocks,omitempty"`
	IsActive        bool `json:"isActive"`
	IsBatchTracking bool `json:"is_batch_tracking"`
}

type ProductVariantsEdge Edge[ProductVariant]
type ProductVariantsConnection struct {
	Edges    []*ProductVariantsEdge `json:"edges"`
	PageInfo *PageInfo              `json:"pageInfo"`
}

func (pv *ProductVariant) GetPurchasePrice() decimal.Decimal {
	return pv.PurchasePrice
}

func (pv *ProductVariant) GetInventoryAccountID() int {
	return pv.InventoryAccountId
}

func (pv *ProductVariant) GetIsBatchTracking() bool {
	return *pv.IsBatchTracking
}

// check if transactions exist when deleted
func (v ProductVariant) validateTransactions(ctx context.Context) error {
	var count int64
	var err error

	count, err = utils.ResourceCountWhere[PurchaseOrderDetail](ctx, v.BusinessId, "product_id = ? AND product_type = ?", v.ID, ProductTypeVariant)
	if err != nil {
		return err
	}
	if count > 0 {
		return errors.New("transaction already exists")
	}

	count, err = utils.ResourceCountWhere[BillDetail](ctx, v.BusinessId, "product_id = ? AND product_type = ?", v.ID, ProductTypeVariant)
	if err != nil {
		return err
	}
	if count > 0 {
		return errors.New("transaction already exists")
	}

	count, err = utils.ResourceCountWhere[SupplierCreditDetail](ctx, v.BusinessId, "product_id = ? AND product_type = ?", v.ID, ProductTypeVariant)
	if err != nil {
		return err
	}
	if count > 0 {
		return errors.New("transaction already exists")
	}

	count, err = utils.ResourceCountWhere[SalesOrderDetail](ctx, v.BusinessId, "product_id = ? AND product_type = ?", v.ID, ProductTypeVariant)
	if err != nil {
		return err
	}
	if count > 0 {
		return errors.New("transaction already exists")
	}

	count, err = utils.ResourceCountWhere[SalesInvoiceDetail](ctx, v.BusinessId, "product_id = ? AND product_type = ?", v.ID, ProductTypeVariant)
	if err != nil {
		return err
	}
	if count > 0 {
		return errors.New("transaction already exists")
	}

	count, err = utils.ResourceCountWhere[CreditNoteDetail](ctx, v.BusinessId, "product_id = ? AND product_type = ?", v.ID, ProductTypeVariant)
	if err != nil {
		return err
	}
	if count > 0 {
		return errors.New("transaction already exists")
	}

	return nil
}

func (v ProductVariant) fillable() map[string]interface{} {
	f := map[string]interface{}{
		"Name":           v.Name,
		"Sku":            v.Sku,
		"Barcode":        v.Barcode,
		"UnitId":         v.UnitId,
		"SalesPrice":     v.SalesPrice,
		"SalesAccountId": v.SalesAccountId,
		"SalesTaxId":     v.SalesTaxId,
		"SalesTaxType":   v.SalesTaxType,
		// "IsSalesTaxInclusive": v.IsSalesTaxInclusive,
		"PurchasePrice":      v.PurchasePrice,
		"PurchaseAccountId":  v.PurchaseAccountId,
		"PurchaseTaxId":      v.PurchaseTaxId,
		"PurchaseTaxType":    v.PurchaseTaxType,
		"InventoryAccountId": v.InventoryAccountId,
		"IsBatchTracking":    v.IsBatchTracking,
	}
	if v.IsSalesTaxInclusive != nil {
		f["IsSalesTaxInclusive"] = v.IsSalesTaxInclusive
	}
	return f
}

// func upsertProductVariant(ctx context.Context, tx *gorm.DB, input []ProductVariant, productGroupId int) error {
// 	return UpsertAssociation(ctx, tx, input, "product_group_id = ?", productGroupId)
// }

// implements methods for pagination

// node
// returns decoded curosr string
func (pv ProductVariant) GetCursor() string {
	return pv.Name
}

// validate input for both create & update. (id = 0 for create)

func (input *NewProductVariant) validate(ctx context.Context, businessId string, id int) error {
	// validate product_group_id
	if err := utils.ValidateResourceId[ProductGroup](ctx, businessId, input.ProductGroupId); err != nil {
		return errors.New("product group not found")
	}
	// name
	if err := utils.ValidateUnique[ProductVariant](ctx, businessId, "name", input.Name, id); err != nil {
		return err
	}
	// barcode
	if err := utils.ValidateUnique[ProductVariant](ctx, businessId, "barcode", input.Barcode, id); err != nil {
		return err
	}
	// exists unit
	if input.UnitId > 0 {
		if err := utils.ValidateResourceId[ProductUnit](ctx, businessId, input.UnitId); err != nil {
			return errors.New("product unit not found")
		}
	}
	// sales+purchase tax
	if input.SalesTaxType != nil {
		if err := validateTaxExists(ctx, businessId, input.SalesTaxId, *input.SalesTaxType); err != nil {
			return errors.New("sales tax not found")
		}
	}
	if input.PurchaseTaxType != nil {
		if err := validateTaxExists(ctx, businessId, input.PurchaseTaxId, *input.PurchaseTaxType); err != nil {
			return errors.New("purchase tax not found")
		}
	}
	// sales+purchase account
	if err := utils.ValidateResourceId[Account](ctx, businessId, input.SalesAccountId); err != nil {
		return errors.New("sales account not found")
	}
	if err := utils.ValidateResourceId[Account](ctx, businessId, input.PurchaseAccountId); err != nil {
		return errors.New("purchase account not found")
	}
	// inventory account
	if err := utils.ValidateResourceId[Account](ctx, businessId, input.InventoryAccountId); err != nil {
		return errors.New("inventory account not found")
	}
	return nil
}

func CreateProductVariant(ctx context.Context, input *NewProductVariant) (*ProductVariant, error) {
	businessId, ok := utils.GetBusinessIdFromContext(ctx)
	if !ok || businessId == "" {
		return nil, errors.New("business id is required")
	}

	if err := input.validate(ctx, businessId, 0); err != nil {
		return nil, err
	}

	productVariant := ProductVariant{
		ProductGroupId:      input.ProductGroupId,
		BusinessId:          businessId,
		Name:                input.Name,
		Sku:                 input.Sku,
		Barcode:             input.Barcode,
		UnitId:              input.UnitId,
		SalesPrice:          input.SalesPrice,
		SalesAccountId:      input.SalesAccountId,
		SalesTaxId:          input.SalesTaxId,
		SalesTaxType:        input.SalesTaxType,
		IsSalesTaxInclusive: input.IsSalesTaxInclusive,
		PurchasePrice:       input.PurchasePrice,
		PurchaseAccountId:   input.PurchaseAccountId,
		PurchaseTaxId:       input.PurchaseTaxId,
		PurchaseTaxType:     input.PurchaseTaxType,
		InventoryAccountId:  input.InventoryAccountId,
		IsBatchTracking:     input.IsBatchTracking,
	}

	db := config.GetDB()
	// db action
	err := db.WithContext(ctx).Create(&productVariant).Error
	if err != nil {
		return nil, err
	}
	return &productVariant, nil
}

func UpdateProductVariant(ctx context.Context, id int, input *NewProductVariant) (*ProductVariant, error) {

	businessId, ok := utils.GetBusinessIdFromContext(ctx)
	if !ok || businessId == "" {
		return nil, errors.New("business id is required")
	}

	// validate exists
	productVariant, err := utils.FetchModel[ProductVariant](ctx, businessId, id)
	if err != nil {
		return nil, utils.ErrorRecordNotFound
	}

	if err := input.validate(ctx, businessId, id); err != nil {
		return nil, err
	}

	db := config.GetDB()
	// db action
	err = db.WithContext(ctx).Model(&productVariant).Updates(map[string]interface{}{
		"ProductGroupId":      input.ProductGroupId,
		"Name":                input.Name,
		"Sku":                 input.Sku,
		"Barcode":             input.Barcode,
		"UnitId":              input.UnitId,
		"SalesPrice":          input.SalesPrice,
		"SalesAccountId":      input.SalesAccountId,
		"SalesTaxId":          input.SalesTaxId,
		"SalesTaxType":        input.SalesTaxType,
		"IsSalesTaxInclusive": input.IsSalesTaxInclusive,
		"PurchasePrice":       input.PurchasePrice,
		"PurchaseAccountId":   input.PurchaseAccountId,
		"PurchaseTaxId":       input.PurchaseTaxId,
		"PurchaseTaxType":     input.PurchaseTaxType,
		"InventoryAccountId":  input.InventoryAccountId,
		"IsBatchTracking":     input.IsBatchTracking,
	}).Error
	if err != nil {
		return nil, err
	}
	// // caching
	// // if product group id is modified
	// if beforeUpdate.ProductGroupId != update.ProductGroupId {
	// 	// remove cache for previous product group as well
	// 	groupId, _ := strconv.Atoi(beforeUpdate.ProductGroupId)
	// 	if err := utils.RemoveRedisItem[ProductGroup](groupId); err != nil {
	// 		return nil, err
	// 	}
	// }
	// groupId, _ := strconv.Atoi(input.ProductGroupId)
	// if err := utils.RemoveRedisBothOLD[ProductGroup](businessId, groupId); err != nil {
	// 	return nil, err
	// }
	return productVariant, nil
}

func DeleteProductVariant(ctx context.Context, id int) (*ProductVariant, error) {

	businessId, ok := utils.GetBusinessIdFromContext(ctx)
	if !ok || businessId == "" {
		return nil, errors.New("business id is required")
	}
	// fetch from db
	result, err := utils.FetchModel[ProductVariant](ctx, businessId, id)
	if err != nil {
		return nil, utils.ErrorRecordNotFound
	}

	if err := result.validateTransactions(ctx); err != nil {
		return nil, err
	}

	// db action
	db := config.GetDB()
	if err := db.WithContext(ctx).Delete(&result).Error; err != nil {
		return nil, err
	}

	return result, nil
}

func GetProductVariant(ctx context.Context, id int) (*ProductVariant, error) {
	// check if product variant belongs to user
	businessId, ok := utils.GetBusinessIdFromContext(ctx)
	if !ok || businessId == "" {
		return nil, errors.New("business id is required")
	}

	return utils.FetchModel[ProductVariant](ctx, businessId, id)
}

func ListAllProductVariant(ctx context.Context, name *string) ([]*AllProductVariant, error) {
	businessId, ok := utils.GetBusinessIdFromContext(ctx)
	if !ok || businessId == "" {
		return nil, errors.New("business id is required")
	}

	var results []*AllProductVariant
	db := config.GetDB()
	dbCtx := db.WithContext(ctx).Where("business_id = ?", businessId)
	if name != nil && *name != "" {
		dbCtx.Where("name LIKE ?", "%"+*name+"%")
	}
	if err := dbCtx.Model(&ProductVariant{}).Find(&results).Error; err != nil {
		return nil, err
	}
	return results, nil
}

func ToggleActiveProductVariant(ctx context.Context, id int, isActive bool) (*ProductVariant, error) {
	businessId, ok := utils.GetBusinessIdFromContext(ctx)
	if !ok || businessId == "" {
		return nil, errors.New("business id is required")
	}
	return ToggleActiveModel[ProductVariant](ctx, businessId, id, isActive)
}

func PaginateProductVariant(ctx context.Context, limit *int, after *string,
	groupId int, name *string) (*ProductVariantsConnection, error) {

	businessId, ok := utils.GetBusinessIdFromContext(ctx)
	if !ok || businessId == "" {
		return nil, errors.New("business id is required")
	}

	db := config.GetDB()
	dbCtx := db.WithContext(ctx).Where("business_id = ?", businessId)
	dbCtx.Where("product_group_id = ?", groupId)
	if name != nil && *name != "" {
		dbCtx.Where("name LIKE ?", "%"+*name+"%")
	}

	edges, pageInfo, err := FetchPagePureCursor[ProductVariant](dbCtx, *limit, after, "name", ">")
	if err != nil {
		return nil, err
	}

	var productVariantsConnection ProductVariantsConnection
	productVariantsConnection.PageInfo = pageInfo
	for _, edge := range edges {
		productVariantsEdge := ProductVariantsEdge(edge)
		productVariantsConnection.Edges = append(productVariantsConnection.Edges, &productVariantsEdge)
	}

	return &productVariantsConnection, nil
}
