package models

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/99designs/gqlgen/graphql"
	"github.com/mmdatafocus/books_backend/config"
	"github.com/mmdatafocus/books_backend/utils"
	"github.com/shopspring/decimal"
	"github.com/xuri/excelize/v2"
	"gorm.io/gorm"
)

type Product struct {
	ID                  int               `gorm:"primary_key" json:"id"`
	BusinessId          string            `gorm:"index;not null" json:"business_id" binding:"required"`
	Name                string            `gorm:"size:100;not null" json:"name" binding:"required"`
	Description         string            `gorm:"type:text" json:"description"`
	CategoryId          int               `gorm:"index;not null;default:0" json:"category_id"`
	Modifiers           []ProductModifier `gorm:"many2many:products_link_modifiers" json:"-"`
	Images              []*Image          `gorm:"polymorphic:Reference" json:"-"`
	UnitId              int               `json:"product_unit_id"`
	SupplierId          int               `json:"supplier_id"`
	Sku                 string            `gorm:"size:100;not null" json:"sku" binding:"required"`
	Barcode             string            `gorm:"index;size:100;not null" json:"barcode" binding:"required"`
	SalesPrice          decimal.Decimal   `gorm:"type:decimal(20,4);default:0" json:"sales_price"`
	SalesAccountId      int               `json:"sales_account_id"`
	SalesTaxId          int               `json:"sales_tax_id"`
	SalesTaxType        *TaxType          `gorm:"type:enum('I', 'G'); default:null" json:"sales_tax_type"`
	IsSalesTaxInclusive *bool             `gorm:"not null;default:true" json:"is_sales_tax_inclusive"`
	PurchasePrice       decimal.Decimal   `gorm:"type:decimal(20,4);default:0" json:"purchase_price"`
	PurchaseAccountId   int               `json:"purchase_account_id"`
	PurchaseTaxId       int               `json:"purchase_tax_id"`
	PurchaseTaxType     *TaxType          `gorm:"type:enum('I', 'G'); default:null" json:"purchase_tax_type"`
	InventoryAccountId  int               `json:"inventory_account_id"`
	IsActive            *bool             `gorm:"not null;default:true" json:"is_active"`
	IsBatchTracking     *bool             `gorm:"not null;default:false" json:"is_batch_traking"`
	ExternalSystemId    string            `gorm:"index" json:"external_system_id"`
	// Stocks              []Stock           `gorm:"foreignkey:ProductId" json:"stocks"`
	CreatedAt time.Time `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt time.Time `gorm:"autoUpdateTime" json:"updated_at"`
}

type OpeningStock struct {
	ProductId          int             `gorm:"primary_key" json:"product_id"`
	ProductType        ProductType     `gorm:"type:enum('S','G','C','V','I');default:S;primary_key" json:"product_type"`
	WarehouseId        int             `gorm:"primary_key" json:"warehouse_id"`
	BatchNumber        string          `gorm:"size:100" json:"batch_number"`
	Qty                decimal.Decimal `gorm:"type:decimal(20,4);default:0" json:"qty"`
	UnitValue          decimal.Decimal `gorm:"type:decimal(20,4);default:0" json:"unit_value"`
	ProductGroupId     int             `gorm:"default:0" json:"product_group_id"`
	InventoryAccountId int             `gorm:"not null" json:"inventory_account_id"`
}

type ProductOpeningStock struct { // use in PubSub
	InventoryAccountId int                         `json:"inventory_account_id"`
	ProductId          int                         `json:"product_id"`
	Details            []ProductOpeningStockDetail `json:"details"`
}

type ProductOpeningStockDetail struct { // use in PubSub
	WarehouseId int             `json:"warehouse_id"`
	BatchNumber string          `json:"batch_number"`
	Qty         decimal.Decimal `json:"qty"`
	UnitValue   decimal.Decimal `json:"unit_value"`
}

type NewProduct struct {
	Name                string                   `json:"name" binding:"required"`
	Description         string                   `json:"description"`
	CategoryId          int                      `json:"category_id"`
	Modifiers           []NewProductLinkModifier `json:"modifiers"`
	Images              []*NewImage              `json:"image_urls"`
	UnitId              int                      `json:"product_unit_id"`
	SupplierId          int                      `json:"supplier_id"`
	Sku                 string                   `json:"sku" binding:"required"`
	Barcode             string                   `json:"barcode" binding:"required"`
	SalesPrice          decimal.Decimal          `json:"sales_price"`
	SalesAccountId      int                      `json:"sales_account_id"`
	SalesTaxId          int                      `json:"sales_tax_id"`
	SalesTaxType        *TaxType                 `json:"sales_tax_type"`
	IsSalesTaxInclusive *bool                    `json:"is_sales_tax_inclusive"`
	PurchasePrice       decimal.Decimal          `json:"purchase_price"`
	PurchaseAccountId   int                      `json:"purchase_account_id"`
	PurchaseTaxId       int                      `json:"purchase_tax_id"`
	PurchaseTaxType     *TaxType                 `json:"purchase_tax_type"`
	InventoryAccountId  int                      `json:"inventory_account_id"`
	OpeningStocks       []NewOpeningStock        `json:"opening_stocks"`
	IsBatchTracking     *bool                    `json:"is_batch_traking"`
}

type NewOpeningStock struct {
	WarehouseId int             `json:"warehouse_id"`
	BatchNumber string          `json:"batch_number"`
	Qty         decimal.Decimal `json:"qty"`
	UnitValue   decimal.Decimal `json:"unit_value"`
}

type NewProductLinkModifier struct {
	ModifierId int `json:"modifier_id" binding:"required"`
}

type ProductsEdge Edge[Product]

type ProductsConnection struct {
	PageInfo *PageInfo
	// Edges    ProductsEdges
	Edges []*ProductsEdge
}

type AllProduct struct {
	ID            string          `json:"id"`
	Name          string          `json:"name"`
	Sku           *string         `json:"sku,omitempty"`
	Barcode       *string         `json:"barcode,omitempty"`
	UnitId        int             `json:"product_unit_id"`
	SalesPrice    decimal.Decimal `json:"salesPrice"`
	PurchasePrice decimal.Decimal `json:"purchasePrice"`

	PurchaseAccountId int      `json:"purchase_account_id"`
	PurchaseTaxId     int      `json:"purchase_tax_id"`
	PurchaseTaxType   *TaxType `json:"purchase_tax_type"`

	SalesAccountId int      `json:"sales_account_id"`
	SalesTaxId     int      `json:"sales_tax_id"`
	SalesTaxType   *TaxType `json:"sales_tax_type"`

	InventoryAccountId int `json:"inventory_account_id"`

	IsActive        bool `json:"isActive"`
	IsBatchTracking bool `json:"is_batch_tracking"`
}

func (p *Product) GetPurchasePrice() decimal.Decimal {
	return p.PurchasePrice
}

func (p *Product) GetInventoryAccountID() int {
	return p.InventoryAccountId
}

func (p *Product) GetIsBatchTracking() bool {
	return *p.IsBatchTracking
}

// returns ids of associated modifiers
func (p Product) ModifierIds(ctx context.Context) (ids []int, err error) {
	db := config.GetDB()
	err = db.WithContext(ctx).Table("products_link_modifiers").
		Where("product_id = ?", p.ID).
		Select("product_modifier_id").Scan(&ids).Error
	return
}

// implements Node
func (p Product) GetCursor() string {
	return p.CreatedAt.String()
}

func (p *Product) GetIntegrationData(tx *gorm.DB) (data []map[string]interface{}, err error) {
	query := `
        SELECT 
        p.id AS id,
        p.name as name,
        COALESCE(s.warehouse_id, 0.0) AS warehouseId,
        COALESCE(s.current_qty, 0.0) AS currentStock,
        p.sales_price AS price
    FROM
        products AS p
            LEFT JOIN
           (SELECT 
              *
            FROM
            stock_summaries
            WHERE
            product_type = 'S') AS s ON p.id = s.product_id
    WHERE
        p.business_id = ?
        AND p.id = ?`

	// Execute the raw query
	rows, err := tx.Raw(query, p.BusinessId, p.ID).Rows()
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	// Get the column names
	columns, err := rows.Columns()
	if err != nil {
		return nil, err
	}

	// Create a slice to hold the values for each row dynamically
	values := make([]interface{}, len(columns))

	// Read the results into a map
	for rows.Next() {
		// Create a slice of empty interfaces for the current row
		for i := range values {
			values[i] = new(interface{})
		}

		// Scan the row values into the slice
		err := rows.Scan(values...)
		if err != nil {
			return nil, err
		}

		// Create a map for the current row with column names as keys
		result := make(map[string]interface{})
		for i, col := range columns {
			// Dereference the value and add to the map
			val := *(values[i].(*interface{}))

			// Check if the value is a string (base64-encoded)
			switch v := val.(type) {
			case string:
				// Attempt base64 decode if it's a string
				if decoded, err := base64.StdEncoding.DecodeString(v); err == nil {
					result[col] = string(decoded) // store decoded string
				} else {
					result[col] = v // keep original if decoding fails
				}
			case []byte:
				// If it's already a []byte, attempt base64 decode
				decoded := string(v) // Decode from []byte to string directly
				result[col] = decoded
			default:
				// If it's not base64-encoded string, store the value as is
				result[col] = v
			}
		}

		// Append the result to the data slice
		data = append(data, result)
	}

	// Check for errors from iterating over rows
	if err = rows.Err(); err != nil {
		return nil, err
	}

	return data, nil
}

// validate input for both create & update. (id = 0 for create)

func (input *NewProduct) validate(ctx context.Context, businessId string, id int) error {
	if err := utils.ValidateUnique[Product](ctx, businessId, "name", input.Name, id); err != nil {
		return err
	}
	// exists category
	if input.CategoryId > 0 {
		if err := utils.ValidateResourceId[ProductCategory](ctx, businessId, input.CategoryId); err != nil {
			return errors.New("product category not found")
		}
	}

	// exists supplier
	if input.SupplierId > 0 {
		if err := utils.ValidateResourceId[Supplier](ctx, businessId, input.SupplierId); err != nil {
			return errors.New("supplier not found")
		}
	}

	// exists unit
	if input.UnitId > 0 {
		if err := utils.ValidateResourceId[ProductUnit](ctx, businessId, input.UnitId); err != nil {
			return errors.New("product unit not found")
		}
	}

	// exists warehouse
	if len(input.OpeningStocks) > 0 {
		var warehouseIds []int
		for _, openingStock := range input.OpeningStocks {
			if openingStock.WarehouseId <= 0 {
				return errors.New("warehouse is required for opening stock")
			}
			if openingStock.Qty.LessThanOrEqual(decimal.Zero) {
				return errors.New("opening stock qty must be positive")
			}
			if slices.Contains(warehouseIds, openingStock.WarehouseId) {
				return errors.New("duplicate warehouse")
			}
			warehouseIds = append(warehouseIds, openingStock.WarehouseId)
		}
		if err := utils.ValidateResourcesId[Warehouse](ctx, businessId, warehouseIds); err != nil {
			return errors.New("warehouse not found")
		}
	}

	// validate sales + purchase tax
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
	return nil
}

func mapProductModifierInput(ctx context.Context, businessId string, input []NewProductLinkModifier) ([]ProductModifier, error) {
	modifiers := make([]ProductModifier, 0)
	modifierIds := make([]int, 0)
	// construct modifiers array
	for _, m := range input {
		modifiers = append(modifiers, ProductModifier{
			ID: m.ModifierId,
		})
		modifierIds = append(modifierIds, m.ModifierId)
	}
	// check if all modifiers exist and belong to the user
	if err := utils.ValidateResourcesId[ProductModifier](ctx, businessId, modifierIds); err != nil {
		return nil, errors.New("modifier not found")
	}

	return modifiers, nil
}

func CreateProduct(ctx context.Context, input *NewProduct) (*Product, error) {

	businessId, ok := utils.GetBusinessIdFromContext(ctx)
	if !ok || businessId == "" {
		return nil, errors.New("business id is required")
	}
	// validate product
	if err := input.validate(ctx, businessId, 0); err != nil {
		return nil, err
	}

	// construct productModifiers
	modifiers, err := mapProductModifierInput(ctx, businessId, input.Modifiers)
	if err != nil {
		return nil, err
	}
	// construct Images
	images, err := mapNewImages(input.Images, "products", 0)
	if err != nil {
		return nil, err
	}
	// store product
	product := Product{
		BusinessId:          businessId,
		Name:                input.Name,
		Description:         input.Description,
		CategoryId:          input.CategoryId,
		UnitId:              input.UnitId,
		SupplierId:          input.SupplierId,
		Sku:                 input.Sku,
		Barcode:             input.Barcode,
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
		IsActive:            utils.NewTrue(),
		IsBatchTracking:     input.IsBatchTracking,
		// asssociation
		Modifiers: modifiers,
		Images:    images,
	}

	db := config.GetDB()
	tx := db.Begin()

	err = tx.WithContext(ctx).Omit("Modifiers.*").Create(&product).Error
	if err != nil {
		tx.Rollback()
		return nil, err
	}

	// create product stock
	if input.InventoryAccountId > 0 && len(input.OpeningStocks) > 0 {
		business, err := GetBusinessById(ctx, businessId)
		if err != nil {
			return nil, err
		}

		openingStockDetails := make([]ProductOpeningStockDetail, 0)
		for _, openingStock := range input.OpeningStocks {
			UpdateStockSummaryOpeningQty(tx, businessId, openingStock.WarehouseId, product.ID, string(ProductTypeSingle), "", openingStock.Qty, business.MigrationDate)
			openingStockDetails = append(openingStockDetails, ProductOpeningStockDetail{
				BatchNumber: openingStock.BatchNumber,
				WarehouseId: openingStock.WarehouseId,
				Qty:         openingStock.Qty,
				UnitValue:   openingStock.UnitValue,
			})

			err = tx.Create(&OpeningStock{
				ProductId:          product.ID,
				ProductType:        ProductTypeSingle,
				BatchNumber:        openingStock.BatchNumber,
				WarehouseId:        openingStock.WarehouseId,
				Qty:                openingStock.Qty,
				UnitValue:          openingStock.UnitValue,
				InventoryAccountId: input.InventoryAccountId,
			}).Error
			if err != nil {
				tx.Rollback()
				return nil, err
			}
		}

		productOpeningStock := ProductOpeningStock{
			InventoryAccountId: input.InventoryAccountId,
			ProductId:          product.ID,
			Details:            openingStockDetails,
		}
		err = PublishToAccounting(ctx, tx, businessId, business.MigrationDate, product.ID, AccountReferenceTypeProductOpeningStock, productOpeningStock, nil, PubSubMessageActionCreate)
		if err != nil {
			tx.Rollback()
			return nil, err
		}
	}
	if err := tx.Commit().Error; err != nil {
		tx.Rollback()
		return nil, err
	}

	return &product, nil
}

func UpdateProduct(ctx context.Context, id int, input *NewProduct) (*Product, error) {

	businessId, ok := utils.GetBusinessIdFromContext(ctx)
	if !ok || businessId == "" {
		return nil, errors.New("business id is required")
	}

	// id exists
	product, err := utils.FetchModel[Product](ctx, businessId, id)
	if err != nil {
		return nil, err
	}
	// validate product
	if err := input.validate(ctx, businessId, id); err != nil {
		return nil, err
	}

	db := config.GetDB()

	// if stock(s) exist, inventory account cannot be null
	var count int64
	if input.InventoryAccountId == 0 {
		if err := db.WithContext(ctx).Model(&StockSummary{}).
			Where("product_id = ? AND product_type = ?", id, ProductTypeSingle).Count(&count).Error; err != nil {
			return nil, err
		}
		if count > 0 {
			return nil, errors.New("cannot disable inventory tracking as stock(s) exist")
		}
	}
	hasValidInventory := input.InventoryAccountId > 0 && len(input.OpeningStocks) > 0
	if hasValidInventory {
		if err := db.WithContext(ctx).Model(&StockSummary{}).
			Where("product_id = ? AND product_type = ?", id, ProductTypeSingle).Count(&count).Error; err != nil {
			return nil, err
		}
		if count > 0 {
			return nil, errors.New("cannot create opening stock as stock(s) exist")
		}
		if err := db.WithContext(ctx).Model(&PurchaseOrderDetail{}).
			Where("product_id = ? AND product_type = ?", id, ProductTypeSingle).Count(&count).Error; err != nil {
			return nil, err
		}
		if count > 0 {
			return nil, errors.New("cannot enable inventory tracking as transaction(s) exist")
		}
		if err := db.WithContext(ctx).Model(&BillDetail{}).
			Where("product_id = ? AND product_type = ?", id, ProductTypeSingle).Count(&count).Error; err != nil {
			return nil, err
		}
		if count > 0 {
			return nil, errors.New("cannot enable inventory tracking as transaction(s) exist")
		}
		if err := db.WithContext(ctx).Model(&SupplierCreditDetail{}).
			Where("product_id = ? AND product_type = ?", id, ProductTypeSingle).Count(&count).Error; err != nil {
			return nil, err
		}
		if count > 0 {
			return nil, errors.New("cannot enable inventory tracking as transaction(s) exist")
		}
		if err := db.WithContext(ctx).Model(&SalesOrderDetail{}).
			Where("product_id = ? AND product_type = ?", id, ProductTypeSingle).Count(&count).Error; err != nil {
			return nil, err
		}
		if count > 0 {
			return nil, errors.New("cannot enable inventory tracking as transaction(s) exist")
		}
		if err := db.WithContext(ctx).Model(&SalesInvoiceDetail{}).
			Where("product_id = ? AND product_type = ?", id, ProductTypeSingle).Count(&count).Error; err != nil {
			return nil, err
		}
		if count > 0 {
			return nil, errors.New("cannot enable inventory tracking as transaction(s) exist")
		}
		if err := db.WithContext(ctx).Model(&CreditNoteDetail{}).
			Where("product_id = ? AND product_type = ?", id, ProductTypeSingle).Count(&count).Error; err != nil {
			return nil, err
		}
		if count > 0 {
			return nil, errors.New("cannot enable inventory tracking as transaction(s) exist")
		}
	}

	tx := db.Begin()

	err = tx.WithContext(ctx).Model(&product).Updates(map[string]interface{}{
		"Name":                input.Name,
		"Description":         input.Description,
		"CategoryId":          input.CategoryId,
		"UnitId":              input.UnitId,
		"SupplierId":          input.SupplierId,
		"Sku":                 input.Sku,
		"Barcode":             input.Barcode,
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
		tx.Rollback()
		return nil, err
	}

	// var modifierIds []int
	// for _, pLm := range input.Modifiers {
	// 	modifierIds = append(modifierIds, pLm.ModifierId)
	// }

	// if err := UpsertJoinTable(tx, "products_link_modifiers", "product_id", "product_modifier_id", id, modifierIds); err != nil {
	// 	return nil, err
	// }

	if len(input.Images) > 0 {
		images, err := UpsertImages(ctx, tx, input.Images, "products", id)
		if err != nil {
			tx.Rollback()
			return nil, err
		}
		product.Images = images
	}
	// create product stock
	if hasValidInventory {
		business, err := GetBusinessById(ctx, businessId)
		if err != nil {
			tx.Rollback()
			return nil, err
		}
		openingStockDetails := make([]ProductOpeningStockDetail, 0)
		for _, openingStock := range input.OpeningStocks {
			UpdateStockSummaryReceivedQty(tx, businessId, openingStock.WarehouseId, product.ID, string(ProductTypeSingle), "", openingStock.Qty, business.MigrationDate)
			openingStockDetails = append(openingStockDetails, ProductOpeningStockDetail{
				BatchNumber: openingStock.BatchNumber,
				WarehouseId: openingStock.WarehouseId,
				Qty:         openingStock.Qty,
				UnitValue:   openingStock.UnitValue,
			})
			err = tx.Create(&OpeningStock{
				ProductId:          product.ID,
				ProductType:        ProductTypeSingle,
				BatchNumber:        openingStock.BatchNumber,
				WarehouseId:        openingStock.WarehouseId,
				Qty:                openingStock.Qty,
				UnitValue:          openingStock.UnitValue,
				InventoryAccountId: input.InventoryAccountId,
			}).Error
			if err != nil {
				tx.Rollback()
				return nil, err
			}
		}
		productOpeningStock := ProductOpeningStock{
			InventoryAccountId: input.InventoryAccountId,
			ProductId:          product.ID,
			Details:            openingStockDetails,
		}
		err = PublishToAccounting(ctx, tx, businessId, business.MigrationDate, product.ID, AccountReferenceTypeProductOpeningStock, productOpeningStock, nil, PubSubMessageActionCreate)
		if err != nil {
			tx.Rollback()
			return nil, err
		}
	}
	if err := tx.Commit().Error; err != nil {
		return nil, err
	}

	return product, nil
}

func DeleteProduct(ctx context.Context, id int) (*Product, error) {
	businessId, ok := utils.GetBusinessIdFromContext(ctx)
	if !ok || businessId == "" {
		return nil, errors.New("business id is required")
	}

	result, err := utils.FetchModel[Product](ctx, businessId, id, "Images")
	if err != nil {
		return nil, err
	}

	count, err := utils.ResourceCountWhere[StockSummary](ctx, "", "product_id = ? AND product_type = ?", id, ProductTypeSingle)
	if err != nil {
		return nil, err
	}
	if count > 0 {
		return nil, errors.New("stock already exists")
	}
	count, err = utils.ResourceCountWhere[PurchaseOrderDetail](ctx, "", "product_id = ? AND product_type = ?", id, ProductTypeSingle)
	if err != nil {
		return nil, err
	}
	if count > 0 {
		return nil, errors.New("transaction already exists")
	}
	count, err = utils.ResourceCountWhere[BillDetail](ctx, "", "product_id = ? AND product_type = ?", id, ProductTypeSingle)
	if err != nil {
		return nil, err
	}
	if count > 0 {
		return nil, errors.New("transaction already exists")
	}
	count, err = utils.ResourceCountWhere[SupplierCreditDetail](ctx, "", "product_id = ? AND product_type = ?", id, ProductTypeSingle)
	if err != nil {
		return nil, err
	}
	if count > 0 {
		return nil, errors.New("transaction already exists")
	}
	count, err = utils.ResourceCountWhere[SalesOrderDetail](ctx, "", "product_id = ? AND product_type = ?", id, ProductTypeSingle)
	if err != nil {
		return nil, err
	}
	if count > 0 {
		return nil, errors.New("transaction already exists")
	}
	count, err = utils.ResourceCountWhere[SalesInvoiceDetail](ctx, "", "product_id = ? AND product_type = ?", id, ProductTypeSingle)
	if err != nil {
		return nil, err
	}
	if count > 0 {
		return nil, errors.New("transaction already exists")
	}
	count, err = utils.ResourceCountWhere[CreditNoteDetail](ctx, "", "product_id = ? AND product_type = ?", id, ProductTypeSingle)
	if err != nil {
		return nil, err
	}
	if count > 0 {
		return nil, errors.New("transaction already exists")
	}

	// clearing association but not deleting associated data
	// db action
	db := config.GetDB()
	tx := db.Begin()

	err = tx.WithContext(ctx).Model(&result).Association("Modifiers").Clear()
	if err != nil {
		tx.Rollback()
		return nil, err
	}

	for _, img := range result.Images {
		if err := img.Delete(tx, ctx); err != nil {
			tx.Rollback()
			return nil, err
		}
	}

	// db action
	err = tx.WithContext(ctx).Delete(&result).Error
	if err != nil {
		tx.Rollback()
		return nil, err
	}

	if err := tx.Commit().Error; err != nil {
		return nil, err
	}

	return result, nil
}

func GetProduct(ctx context.Context, id int) (*Product, error) {
	return GetResource[Product](ctx, id)
}

func GetProducts(ctx context.Context, name *string) ([]*Product, error) {
	db := config.GetDB()
	var results []*Product

	// fieldNames, err := utils.GetQueryFields(ctx, &Product{})
	// if err != nil {
	// 	return nil, err
	// }

	businessId, ok := utils.GetBusinessIdFromContext(ctx)
	if !ok || businessId == "" {
		return nil, errors.New("business id is required")
	}

	dbCtx := db.WithContext(ctx).Where("business_id = ?", businessId)
	if name != nil && len(*name) > 0 {
		dbCtx = dbCtx.Where("name LIKE ?", "%"+*name+"%")
	}

	err := dbCtx.Order("name").
		Find(&results).Error
	if err != nil {
		return nil, err
	}
	return results, nil
}

func GetProductIDsByCategoryID(categoryID int) ([]int, error) {

	db := config.GetDB()
	var productIDs []int
	result := db.Model(&Product{}).
		Where("category_id = ?", categoryID).
		Pluck("id", &productIDs)

	if result.Error != nil {
		return nil, result.Error
	}

	return productIDs, nil
}

func GetProductIDsBySupplierID(supplierId int) ([]int, error) {

	db := config.GetDB()
	var productIDs []int
	result := db.Model(&Product{}).
		Where("supplier_id = ?", supplierId).
		Pluck("id", &productIDs)

	if result.Error != nil {
		return nil, result.Error
	}

	return productIDs, nil
}

func ToggleActiveProduct(ctx context.Context, id int, isActive bool) (*Product, error) {
	businessId, ok := utils.GetBusinessIdFromContext(ctx)
	if !ok || businessId == "" {
		return nil, errors.New("business id is required")
	}

	return ToggleActiveModel[Product](ctx, businessId, id, isActive)
}

func PaginateProduct(ctx context.Context, limit *int, after *string, name *string, sku *string) (*ProductsConnection, error) {

	businessId, ok := utils.GetBusinessIdFromContext(ctx)
	if !ok || businessId == "" {
		return nil, errors.New("business id is required")
	}
	ctxWithTimeout, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	db := config.GetDB()
	dbCtx := db.WithContext(ctxWithTimeout).Model(&Product{}).Where("business_id = ?", businessId)

	if name != nil && *name != "" {
		dbCtx.Where("name LIKE ?", "%"+*name+"%")
	}
	if sku != nil && *sku != "" {
		dbCtx.Where("sku = ?", *sku)
	}

	edges, pageInfo, err := FetchPageCompositeCursor[Product](dbCtx, *limit, after, "created_at", "<")
	if err != nil {
		return nil, err
	}

	var productConnection ProductsConnection
	productConnection.PageInfo = pageInfo
	for _, edge := range edges {
		productEdge := ProductsEdge(edge)
		productConnection.Edges = append(productConnection.Edges, &productEdge)
	}

	return &productConnection, nil
}

func ListAllProduct(ctx context.Context, name *string) ([]*AllProduct, error) {
	businessId, ok := utils.GetBusinessIdFromContext(ctx)
	if !ok || businessId == "" {
		return nil, errors.New("business id is required")
	}
	var allProducts []*AllProduct
	db := config.GetDB()
	dbCtx := db.WithContext(ctx).Where("business_id = ?", businessId)

	if name != nil && *name != "" {
		dbCtx.Where("name LIKE ?", "%"+*name+"%")
	}

	if err := dbCtx.Model(&Product{}).
		Find(&allProducts).Error; err != nil {
		return nil, err
	}
	return allProducts, nil
}

func GetProductStocks(ctx context.Context, productId int, productType string, warehouseID *int) ([]*ProductStock, error) {

	db := config.GetDB()
	product, err := GetProductOrVariant(ctx, productType, productId)
	if err != nil {
		return nil, err
	}

	var stocks []*ProductStock

	dbCtx := db.WithContext(ctx).Where("product_id = ?", product.GetId())

	if warehouseID != nil && *warehouseID > 0 {
		dbCtx = dbCtx.Where("warehouse_id = ?", warehouseID)
	}

	// if err := dbCtx.Model(&Stock{}).Where("current_qty > ?", 0).Find(&stocks).Error; err != nil {
	// 	return nil, err
	// }

	if !product.GetIsBatchTracking() {
		var totalQty decimal.Decimal
		for _, stock := range stocks {
			totalQty = totalQty.Add(stock.CurrentQty)
		}
		return []*ProductStock{{Qty: totalQty}}, nil
	}

	batchQtyMap := make(map[string]decimal.Decimal)
	for _, stock := range stocks {
		batchQtyMap[stock.BatchNumber] = batchQtyMap[stock.BatchNumber].Add(stock.CurrentQty)
	}

	// Convert the map to a slice of ProductStock
	var summarizedStocks []*ProductStock
	for batchNumber, qty := range batchQtyMap {
		summarizedStocks = append(summarizedStocks, &ProductStock{
			BatchNumber: batchNumber,
			Qty:         qty,
		})
	}
	return summarizedStocks, nil
}

type ExcelRow struct {
	Name                         string
	Description                  string
	CategoryName                 string
	UnitName                     string
	UnitAbbreviation             string
	UnitPrecision                Precision
	Sku                          string
	Barcode                      string
	SalesPrice                   decimal.Decimal
	PurchasePrice                decimal.Decimal
	ExternalSystemId             string
	TrackInventory               bool
	WarehouseName                string
	OpeningQtyPerWarehouse       decimal.Decimal
	OpeningUnitValuePerWarehouse decimal.Decimal
}

func validateImportData(ctx context.Context, tx *gorm.DB, businessId string, rows [][]string) error {
	for idx, row := range rows[1:] {
		// Populate ExcelRow
		excelRow, err := PopulateExcelRow(row)

		if err != nil {
			return fmt.Errorf("error in row %d: %v", idx+2, err)
		}

		if len(excelRow.Name) == 0 {
			return fmt.Errorf("product name is null in row %d: %v", idx+2, err)
		}

		if len(excelRow.CategoryName) == 0 {
			return fmt.Errorf("category name is null in row %d: %v", idx+2, err)
		}

		if len(excelRow.UnitName) > 0 {
			var unit ProductUnit
			err = tx.WithContext(ctx).Where("business_id = ? AND name = ?", businessId, excelRow.UnitName).First(&unit).Error
			if err != nil && err != gorm.ErrRecordNotFound {
				return fmt.Errorf("error finding category in row %d: %v", idx+2, err)
			}
			if err == gorm.ErrRecordNotFound {
				// Ensure precisionInt is within the valid range (0-4)
				switch excelRow.UnitPrecision {
				case PrecisionZero, PrecisionOne, PrecisionTwo, PrecisionThree, PrecisionFour:
					// Valid precision, no action needed
				default:
					return fmt.Errorf("invalid precision value in row %d: %v", idx+2, excelRow.UnitPrecision)
				}
				if len(excelRow.UnitAbbreviation) == 0 {
					return fmt.Errorf("unit abbreviation is null in row %d: %v", idx+2, err)
				}
			}
		} else {
			return fmt.Errorf("unit name is null in row %d: %v", idx+2, err)
		}

		// Handle inventory-related fields if TrackInventory is true
		if excelRow.TrackInventory {

			excelRow.WarehouseName = row[12]

			var warehouse Warehouse
			err = tx.WithContext(ctx).Where("business_id = ? AND name = ?", businessId, excelRow.WarehouseName).First(&warehouse).Error
			if err != nil {
				return fmt.Errorf("warehouse not found in row %d: %v", idx+2, err)
			}

			openingQty, err := utils.ParseDecimal(row[13])
			if err != nil {
				return fmt.Errorf("could not parse opening quantity in row %d: %v", idx+2, err)
			}
			excelRow.OpeningQtyPerWarehouse = openingQty

			openingUnitValue, err := utils.ParseDecimal(row[14])
			if err != nil {

				return fmt.Errorf("could not parse opening unit value in row %d: %v", idx+2, err)
			}
			excelRow.OpeningUnitValuePerWarehouse = openingUnitValue
		}
	}
	return nil
}

func uploadFile(ctx context.Context, fileName string, file graphql.Upload) (string, error) {
	objectName := "importProducts/" + fileName
	err := utils.UploadFileToGCS(ctx, objectName, file.File)
	if err != nil {
		return "", fmt.Errorf("failed to upload file to storage provider: %v", err)
	}
	return getCloudURL(objectName), nil
}

func readExcelFileFromURL(fileURL string) (*excelize.File, error) {
	// Download file content from the given URL
	resp, err := http.Get(fileURL)
	if err != nil {
		return nil, fmt.Errorf("failed to download file from URL: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to download file: received status code %d", resp.StatusCode)
	}

	// Create an Excel reader
	f, err := excelize.OpenReader(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to open Excel file: %v", err)
	}

	return f, nil
}

func ImportProductsFromXlsx(ctx context.Context, file graphql.Upload) (string, error) {
	if file.File == nil {
		return "", errors.New("nil file provided")
	}

	if !strings.HasSuffix(file.Filename, ".xlsx") {
		return "", fmt.Errorf("invalid file type: only .xlsx files are allowed")
	}

	businessId, _ := utils.GetBusinessIdFromContext(ctx)
	business, err := GetBusinessById(ctx, businessId)

	if err != nil {
		return "", err
	}

	uniqueFilename := businessId + "_" + utils.GenerateUniqueFilename() + "_*.xlsx"

	fileURL, err := uploadFile(ctx, uniqueFilename, file)
	if err != nil {
		return "", err
	}

	f, err := readExcelFileFromURL(fileURL)
	if err != nil {
		return "", err
	}

	// tempFile, err := os.CreateTemp(filepath.Join("uploads"), uniqueFilename)
	// if err != nil {
	// 	return "", fmt.Errorf("could not create file: %v", err)
	// }
	// defer os.Remove(tempFile.Name())
	// defer tempFile.Close()

	// _, err = io.Copy(tempFile, file.File)
	// if err != nil {
	// 	return "", fmt.Errorf("could not write file: %v", err)
	// }

	// f, err := excelize.(tempFile.Name())
	// if err != nil {
	// 	return "", fmt.Errorf("unable to open Excel file: %v", err)
	// }

	// f, err := excelize.OpenFile(tempFile.Name())
	// if err != nil {
	// 	return "", fmt.Errorf("unable to open Excel file: %v", err)
	// }

	rows, err := f.GetRows("Sheet1")
	if err != nil {
		return "", fmt.Errorf("unable to read sheet: %v", err)
	}

	db := config.GetDB()
	tx := db.Begin()

	err = validateImportData(ctx, tx, businessId, rows)
	if err != nil {
		return "", err
	}

	duplicateRows := make([]string, 0)

	systemAccounts, err := GetSystemAccounts(businessId)
	if err != nil {
		return "", err
	}

	salesAccountId := systemAccounts[AccountCodeSales]
	purchaseAccountId := systemAccounts[AccountCodeCostOfGoodsSold]

	err = utils.BusinessLock(ctx, businessId, "lock", "product.go", "ImportProductsFromXlsx")

	if err != nil {
		return "", err
	}

	taxTypeIndividual := TaxTypeIndividual

	for idx, row := range rows[1:] {

		// Populate ExcelRow
		excelRow, err := PopulateExcelRow(row)
		if err != nil {
			return "", err
		}

		warehouseId := 0
		inventoryAccountId := 0
		// Handle inventory-related fields if TrackInventory is true
		if excelRow.TrackInventory {

			excelRow.WarehouseName = row[12]

			var warehouse Warehouse
			err = tx.WithContext(ctx).Where("business_id = ? AND name = ?", businessId, excelRow.WarehouseName).First(&warehouse).Error
			if err != nil {
				tx.Rollback()
				return "", fmt.Errorf("warehouse not found in row %d: %v", idx+2, err)
			}
			warehouseId = warehouse.ID

			openingQty, err := utils.ParseDecimal(row[13])
			if err != nil {
				tx.Rollback()
				return "", fmt.Errorf("could not parse opening quantity in row %d: %v", idx+2, err)
			}
			excelRow.OpeningQtyPerWarehouse = openingQty

			openingUnitValue, err := utils.ParseDecimal(row[14])
			if err != nil {
				tx.Rollback()
				return "", fmt.Errorf("could not parse opening unit value in row %d: %v", idx+2, err)
			}
			excelRow.OpeningUnitValuePerWarehouse = openingUnitValue

			inventoryAccountId = systemAccounts[AccountCodeInventoryAsset]
		}

		// Check for existing products by name, SKU, or barcode
		var existingProduct Product
		err = tx.WithContext(ctx).Where("business_id = ? AND name = ?", businessId, excelRow.Name).First(&existingProduct).Error
		if err == nil {
			// Product already exists, skip this row
			duplicateRows = append(duplicateRows, fmt.Sprintf("Row %d: Duplicate found for product with Name: %s", idx+2, excelRow.Name))
			continue
		} else if err != gorm.ErrRecordNotFound {
			tx.Rollback()
			return "", fmt.Errorf("error checking for duplicates in row %d: %v", idx+2, err)
		}

		// Find or create category
		category, err := FindOrCreateCategory(ctx, tx, businessId, excelRow.CategoryName)
		if err != nil {
			tx.Rollback()
			return "", err
		}

		// Find or create unit
		unit, err := FindOrCreateProductUnit(ctx, tx, businessId, excelRow.UnitName, excelRow.UnitAbbreviation, excelRow.UnitPrecision)
		if err != nil {
			tx.Rollback()
			return "", err
		}

		product := Product{
			BusinessId:         businessId,
			SalesAccountId:     salesAccountId,
			PurchaseAccountId:  purchaseAccountId,
			Name:               excelRow.Name,
			Description:        excelRow.Description,
			CategoryId:         category.ID,
			UnitId:             unit.ID,
			Sku:                excelRow.Sku,
			Barcode:            excelRow.Barcode,
			SalesPrice:         excelRow.SalesPrice,
			PurchasePrice:      excelRow.PurchasePrice,
			InventoryAccountId: inventoryAccountId,
			IsActive:           utils.NewTrue(),
			SalesTaxType:       &taxTypeIndividual,
			PurchaseTaxType:    &taxTypeIndividual,
			// Add other fields as necessary
		}

		if err := tx.WithContext(ctx).Create(&product).Error; err != nil {
			tx.Rollback()
			return "err", fmt.Errorf("could not create product: %v", err)
		}

		// Handle inventory
		// if excelRow.TrackInventory && warehouseId > 0 {

		// 	productStock := ProductOpeningStock{
		// 		InventoryAccountId: inventoryAccountId,
		// 		ProductId:          product.ID,
		// 		Details: []ProductOpeningStockDetail{
		// 			{
		// 				WarehouseId: warehouseId,
		// 				Qty:         excelRow.OpeningQtyPerWarehouse,
		// 				UnitValue:   excelRow.OpeningUnitValuePerWarehouse,
		// 			},
		// 		},
		// 	}
		// 	err = PublishToAccounting(tx, businessId, business.MigrationDate, product.ID, AccountReferenceTypeProductOpeningStock, productStock, nil, PubSubMessageActionCreate)
		// 	if err != nil {
		// 		tx.Rollback()
		// 		return "", err
		// 	}
		// }

		// Handle inventory
		if excelRow.TrackInventory && warehouseId > 0 {

			UpdateStockSummaryReceivedQty(tx, businessId, warehouseId, product.ID, string(ProductTypeSingle), "", excelRow.OpeningQtyPerWarehouse, business.MigrationDate)

			err = tx.Create(&OpeningStock{
				ProductId:          product.ID,
				ProductType:        ProductTypeSingle,
				WarehouseId:        warehouseId,
				Qty:                excelRow.OpeningQtyPerWarehouse,
				UnitValue:          excelRow.OpeningUnitValuePerWarehouse,
				InventoryAccountId: inventoryAccountId,
			}).Error
			if err != nil {
				tx.Rollback()
				return "", err
			}

			productStock := ProductOpeningStock{
				InventoryAccountId: inventoryAccountId,
				ProductId:          product.ID,
				Details: []ProductOpeningStockDetail{
					{
						WarehouseId: warehouseId,
						Qty:         excelRow.OpeningQtyPerWarehouse,
						UnitValue:   excelRow.OpeningUnitValuePerWarehouse,
					},
				},
			}
			err = PublishToAccounting(ctx, tx, businessId, business.MigrationDate, product.ID, AccountReferenceTypeProductOpeningStock, productStock, nil, PubSubMessageActionCreate)
			if err != nil {
				tx.Rollback()
				return "", err
			}
		}
	}

	if err := tx.Commit().Error; err != nil {
		tx.Rollback()
		return "err", err
	}

	if len(duplicateRows) > 0 {
		return fmt.Sprintf("imported successfully with duplicates: %v", duplicateRows), nil
	}

	return "imported successfully", nil
}

func PopulateExcelRow(row []string) (ExcelRow, error) {
	// Parse sales price
	salesPrice, err := utils.ParseDecimal(row[8])
	if err != nil {
		return ExcelRow{}, fmt.Errorf("could not parse sales price: %v", err)
	}

	// Parse purchase price
	purchasePrice, err := utils.ParseDecimal(row[9])
	if err != nil {
		return ExcelRow{}, fmt.Errorf("could not parse cost price: %v", err)
	}

	// Populate and return ExcelRow struct
	excelRow := ExcelRow{
		Name:             row[0],
		Description:      row[1],
		CategoryName:     row[2],
		UnitName:         row[3],
		UnitAbbreviation: row[4],
		UnitPrecision:    Precision(row[5]),
		Sku:              row[6],
		Barcode:          row[7],
		SalesPrice:       salesPrice,
		PurchasePrice:    purchasePrice,
		ExternalSystemId: row[10],
		TrackInventory:   row[11] == "T",
	}

	return excelRow, nil
}

func FindOrCreateCategory(ctx context.Context, tx *gorm.DB, businessId, categoryName string) (ProductCategory, error) {
	var category ProductCategory
	err := tx.WithContext(ctx).Where("business_id = ? AND name = ?", businessId, categoryName).First(&category).Error
	if err != nil && err != gorm.ErrRecordNotFound {
		return category, fmt.Errorf("error finding category: %v", err)
	}

	if err == gorm.ErrRecordNotFound {
		category = ProductCategory{
			BusinessId: businessId,
			Name:       categoryName,
			IsActive:   utils.NewTrue(),
		}
		if err := tx.WithContext(ctx).Create(&category).Error; err != nil {
			return category, fmt.Errorf("could not create category: %v", err)
		}
	}

	return category, nil
}

func FindOrCreateProductUnit(ctx context.Context, tx *gorm.DB, businessId, unitName, unitAbbreviation string, unitPrecision Precision) (ProductUnit, error) {
	var unit ProductUnit
	err := tx.WithContext(ctx).Where("business_id = ? AND name = ?", businessId, unitName).First(&unit).Error
	if err != nil && err != gorm.ErrRecordNotFound {
		return unit, fmt.Errorf("error finding unit: %v", err)
	}

	if err == gorm.ErrRecordNotFound {
		unit = ProductUnit{
			BusinessId:   businessId,
			Name:         unitName,
			Abbreviation: unitAbbreviation,
			Precision:    unitPrecision,
			IsActive:     utils.NewTrue(),
		}
		if err := tx.WithContext(ctx).Create(&unit).Error; err != nil {
			return unit, fmt.Errorf("could not create unit: %v", err)
		}
	}

	return unit, nil
}
