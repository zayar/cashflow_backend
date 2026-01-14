package models

import (
	"context"
	"errors"
	"slices"
	"strings"
	"time"

	"github.com/mmdatafocus/books_backend/config"
	"github.com/mmdatafocus/books_backend/utils"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

type ProductGroup struct {
	ID               int               `gorm:"primary_key" json:"id"`
	BusinessId       string            `gorm:"index;not null" json:"business_id" binding:"required"`
	Name             string            `gorm:"size:100;not null" json:"name" binding:"required"`
	Description      string            `gorm:"type:text" json:"description"`
	CategoryId       int               `gorm:"index;not null;default:0" json:"category_id"`
	Variants         []ProductVariant  `gorm:"foreignKey:ProductGroupId" json:"product_variants"`
	Modifiers        []ProductModifier `gorm:"many2many:productgroups_link_modifiers" json:"modifiers"`
	Options          []ProductOption   `gorm:"foreignKey:ProductGroupId" json:"options"`
	Images           []*Image          `gorm:"polymorphic:Reference" json:"images"`
	SupplierId       int               `json:"supplier_id"`
	IsActive         *bool             `gorm:"not null;default:true" json:"is_active"`
	IsBatchTracking  *bool             `gorm:"not null;default:false" json:"is_batch_traking"`
	ExternalSystemId string            `gorm:"index" json:"external_system_id"`
	CreatedAt        time.Time         `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt        time.Time         `gorm:"autoUpdateTime" json:"updated_at"`
}

type ProductOption struct {
	Id             int    `gorm:"primaryKey" json:"id"`
	ProductGroupId int    `gorm:"not null;index" json:"product_group_id" binding:"required"`
	OptionName     string `gorm:"not null;index;size:50" json:"option_name" binding:"required"`
	OptionUnits    string `gorm:"type:text" json:"option_units" binding:"required"`
}

type NewProductGroup struct {
	Name            string                        `json:"name" binding:"required"`
	Description     string                        `json:"description"`
	CategoryId      int                           `json:"category_id"`
	Variants        []NewProductVariant           `json:"product_variants"`
	Modifiers       []NewProductGroupLinkModifier `json:"modifiers"`
	Options         []NewProductOption            `json:"options"`
	Images          []*NewImage                   `json:"image_urls"`
	SupplierId      int                           `json:"supplier_id"`
	IsBatchTracking *bool                         `json:"is_batch_traking"`
}

type NewProductOption struct {
	Id          int    `json:"id"`
	OptionName  string `json:"option_name" binding:"required"`
	OptionUnits string `json:"option_units" binding:"required"`
}

type NewProductGroupLinkModifier struct {
	ModifierId int `json:"modifier_id" binding:"required"`
}

type ProductGroupsEdge Edge[ProductGroup]

type ProductGroupsConnection struct {
	Edges    []*ProductGroupsEdge `json:"edges"`
	PageInfo *PageInfo            `json:"pageInfo"`
}

type NewOpeningStockGroup struct {
	ProductVariantId int             `json:"product_variant_id"`
	WarehouseId      int             `json:"warehouse_id"`
	BatchNumber      string          `json:"batch_number"`
	Qty              decimal.Decimal `json:"qty"`
	UnitValue        decimal.Decimal `json:"unit_value"`
}

type ProductGroupOpeningStock struct { // use in PubSub
	InventoryAccountId int                              `json:"inventory_account_id"`
	ProductGroupId     int                              `json:"product_id"`
	Details            []ProductGroupOpeningStockDetail `json:"details"`
}

type ProductGroupOpeningStockDetail struct { // use in PubSub
	WarehouseId      int             `json:"warehouse_id"`
	ProductVariantId int             `json:"product_variant_id"`
	BatchNumber      string          `json:"batch_number"`
	Qty              decimal.Decimal `json:"qty"`
	UnitValue        decimal.Decimal `json:"unit_value"`
}

func (pg *ProductGroup) validateTransactions(ctx context.Context) error {

	var count int64
	var err error

	var variantIds []int
	for _, v := range pg.Variants {
		variantIds = append(variantIds, v.ID)
	}
	if len(variantIds) <= 0 {
		return nil
	}

	count, err = utils.ResourceCountWhere[PurchaseOrderDetail](ctx, "", "product_type = ? AND product_id IN ?", ProductTypeVariant, variantIds)
	if err != nil {
		return err
	}
	if count > 0 {
		return errors.New("transaction already exists")
	}

	count, err = utils.ResourceCountWhere[BillDetail](ctx, "", "product_type = ? AND product_id IN ?", ProductTypeVariant, variantIds)
	if err != nil {
		return err
	}
	if count > 0 {
		return errors.New("transaction already exists")
	}

	count, err = utils.ResourceCountWhere[SupplierCreditDetail](ctx, "", "product_type = ? AND product_id IN ?", ProductTypeVariant, variantIds)
	if err != nil {
		return err
	}
	if count > 0 {
		return errors.New("transaction already exists")
	}

	count, err = utils.ResourceCountWhere[SalesOrderDetail](ctx, "", "product_type = ? AND product_id IN ?", ProductTypeVariant, variantIds)
	if err != nil {
		return err
	}
	if count > 0 {
		return errors.New("transaction already exists")
	}

	count, err = utils.ResourceCountWhere[SalesInvoiceDetail](ctx, "", "product_type = ? AND product_id IN ?", ProductTypeVariant, variantIds)
	if err != nil {
		return err
	}
	if count > 0 {
		return errors.New("transaction already exists")
	}

	count, err = utils.ResourceCountWhere[CreditNoteDetail](ctx, "", "product_type = ? AND product_id IN ?", ProductTypeVariant, variantIds)
	if err != nil {
		return err
	}
	if count > 0 {
		return errors.New("transaction already exists")
	}

	return nil

}

func CreateOpeningStockGroup(ctx context.Context, groupId int, input []*NewOpeningStockGroup) ([]*ProductStock, error) {

	businessId, ok := utils.GetBusinessIdFromContext(ctx)
	if !ok || businessId == "" {
		return nil, errors.New("business id is required")
	}

	business, err := GetBusinessById(ctx, businessId)
	if err != nil {
		return nil, err
	}

	db := config.GetDB()
	tx := db.Begin()

	// validate warehouseIds
	var warehouseIds []int
	for _, i := range input {
		warehouseIds = append(warehouseIds, i.WarehouseId)
	}
	if err := utils.ValidateResourcesId[Warehouse](ctx, businessId, warehouseIds); err != nil {
		return nil, errors.New("warehouse not found")
	}

	// create stocks one by one
	productStocks := make([]*ProductStock, 0, len(input))
	var count int64
	var inventoryAccountId int
	groupOpeningStockDetails := make([]ProductGroupOpeningStockDetail, 0)
	for _, openingStock := range input {

		if !openingStock.Qty.IsPositive() {
			continue
		}
		// check if variant belongs to the group and inventory account exists
		var variant ProductVariant
		err := db.WithContext(ctx).First(&variant, openingStock.ProductVariantId).Error
		if err != nil {
			tx.Rollback()
			return nil, err
		}
		if variant.ProductGroupId != groupId || variant.InventoryAccountId <= 0 {
			continue
		}

		// if openingStock already exists
		if err := db.WithContext(ctx).Table("stock_summaries").
			Where("product_id = ? AND product_type = ?", openingStock.ProductVariantId, ProductTypeVariant).
			Count(&count).Error; err != nil {
			tx.Rollback()
			return nil, err
		}
		if count > 0 {
			continue
		}

		inventoryAccountId = variant.InventoryAccountId

		// create opening stock
		if err := UpdateStockSummaryReceivedQty(tx, businessId, openingStock.WarehouseId, openingStock.ProductVariantId, string(ProductTypeVariant), "", openingStock.Qty, business.MigrationDate); err != nil {
			tx.Rollback()
			return nil, err
		}

		groupOpeningStockDetails = append(groupOpeningStockDetails, ProductGroupOpeningStockDetail{
			WarehouseId:      openingStock.WarehouseId,
			ProductVariantId: openingStock.ProductVariantId,
			BatchNumber:      openingStock.BatchNumber,
			Qty:              openingStock.Qty,
			UnitValue:        openingStock.UnitValue,
		})

		err = tx.Create(&OpeningStock{
			ProductId:          openingStock.ProductVariantId,
			ProductType:        ProductTypeVariant,
			BatchNumber:        openingStock.BatchNumber,
			WarehouseId:        openingStock.WarehouseId,
			Qty:                openingStock.Qty,
			UnitValue:          openingStock.UnitValue,
			ProductGroupId:     groupId,
			InventoryAccountId: inventoryAccountId,
		}).Error
		if err != nil {
			tx.Rollback()
			return nil, err
		}

		pStock := ProductStock{
			WarehouseId:  openingStock.WarehouseId,
			Description:  "Opening Stock",
			ProductId:    openingStock.ProductVariantId,
			ProductType:  ProductTypeVariant,
			BatchNumber:  openingStock.BatchNumber,
			ReceivedDate: business.MigrationDate,
			Qty:          openingStock.Qty,
		}
		productStocks = append(productStocks, &pStock)
	}

	productGroupOpeningStock := ProductGroupOpeningStock{
		InventoryAccountId: inventoryAccountId,
		ProductGroupId:     groupId,
		Details:            groupOpeningStockDetails,
	}
	err = PublishToAccounting(ctx, tx, businessId, business.MigrationDate, groupId, AccountReferenceTypeProductGroupOpeningStock, productGroupOpeningStock, nil, PubSubMessageActionCreate)
	if err != nil {
		tx.Rollback()
		return nil, err
	}

	return productStocks, tx.Commit().Error
}

// returns ids of associated modifiers
func (p ProductGroup) ModifierIds(ctx context.Context) (ids []int, err error) {
	db := config.GetDB()
	err = db.WithContext(ctx).Table("productgroups_link_modifiers").
		Where("product_group_id = ?", p.ID).
		Select("product_modifier_id").Scan(&ids).Error
	return
}

// implements methods for pagination

// node
// returns decoded curosr string
func (pg ProductGroup) GetCursor() string {
	return pg.Name
}

// validate input for both create & update. (id = 0 for create)

func (input *NewProductGroup) validate(ctx context.Context, businessId string, id int) error {
	// name
	if err := utils.ValidateUnique[ProductGroup](ctx, businessId, "name", input.Name, id); err != nil {
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

	return nil
}

// clear cache after update/delete an instance

func mapProductGroupModifierInput(ctx context.Context, businessId string, input []NewProductGroupLinkModifier) ([]ProductModifier, error) {
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

func mapProductVariantsInput(ctx context.Context, businessId string, productGroupId int, input NewProductGroup, options []ProductOption) ([]ProductVariant, error) {
	variants := make([]ProductVariant, 0)
	// names := make([]string, 0)
	// barcodes := make([]string, 0)
	taxIds := make([]int, 0)
	taxGroupIds := make([]int, 0)
	accountIds := make([]int, 0)

	validateVariantName := productVariantValidator(input.Name, options)

	for _, variant := range input.Variants {

		if err := validateVariantName(variant.Name); err != nil {
			return nil, err
		}

		variants = append(variants, ProductVariant{
			ProductGroupId:      productGroupId,
			Name:                variant.Name,
			BusinessId:          businessId,
			Sku:                 variant.Sku,
			UnitId:              variant.UnitId,
			Barcode:             variant.Barcode,
			SalesPrice:          variant.SalesPrice,
			SalesAccountId:      variant.SalesAccountId,
			SalesTaxId:          variant.SalesTaxId,
			SalesTaxType:        variant.SalesTaxType,
			IsSalesTaxInclusive: variant.IsSalesTaxInclusive,
			PurchasePrice:       variant.PurchasePrice,
			PurchaseAccountId:   variant.PurchaseAccountId,
			PurchaseTaxId:       variant.PurchaseTaxId,
			PurchaseTaxType:     variant.PurchaseTaxType,
			InventoryAccountId:  variant.InventoryAccountId,
		})
		// names = append(names, variant.Name)
		// barcodes = append(barcodes, variant.Barcode)

		if variant.SalesTaxId > 0 && variant.SalesTaxType != nil {
			if *variant.SalesTaxType == TaxTypeIndividual {
				taxIds = append(taxIds, variant.SalesTaxId)
			} else {
				taxGroupIds = append(taxGroupIds, variant.SalesTaxId)
			}
		}
		if variant.PurchaseTaxId > 0 && variant.PurchaseTaxType != nil {
			if *variant.PurchaseTaxType == TaxTypeIndividual {
				taxIds = append(taxIds, variant.PurchaseTaxId)
			} else {
				taxGroupIds = append(taxGroupIds, variant.PurchaseTaxId)
			}
		}
		accountIds = append(accountIds, variant.PurchaseAccountId)
		accountIds = append(accountIds, variant.SalesAccountId)
		if variant.InventoryAccountId > 0 {
			accountIds = append(accountIds, variant.InventoryAccountId)
		}
	}

	// check duplicate name
	// count, err := utils.ResourceCountWhere[ProductVariant](ctx, businessId,
	// 	"name IN ?", names)
	// if err != nil {
	// 	return nil, err
	// }
	// if count > 0 {
	// 	return nil, errors.New("duplicate variant name")
	// }

	// check duplicate barcode
	// count, err = utils.ResourceCountWhere[ProductVariant](ctx, businessId,
	// 	"barcode IN ?", barcodes)
	// if err != nil {
	// 	return nil, err
	// }
	// if count > 0 {
	// 	return nil, errors.New("duplicate variant barcode")
	// }

	// check tax id exists
	if err := utils.ValidateResourcesId[Tax](ctx, businessId, taxIds); err != nil {
		return nil, errors.New("tax not found")
	}
	// taxIds = utils.UniqueSlice(taxIds)
	// count, err = utils.ResourceCountWhere[Tax](ctx, businessId,
	// 	"id IN ?", taxIds)
	// if err != nil {
	// 	return nil, err
	// }
	// if count != int64(len(taxIds)) {
	// 	return nil, errors.New("variant tax not found")
	// }

	// check taxGroup id exists
	if err := utils.ValidateResourcesId[TaxGroup](ctx, businessId, taxGroupIds); err != nil {
		return nil, errors.New("tax group not found")
	}
	// taxGroupIds = utils.UniqueSlice(taxGroupIds)
	// count, err = utils.ResourceCountWhere[TaxGroup](ctx, businessId,
	// 	"id IN ?", taxGroupIds)
	// if err != nil {
	// 	return nil, err
	// }
	// if count != int64(len(taxGroupIds)) {
	// 	return nil, errors.New("variant taxGroup not found")
	if err := utils.ValidateResourcesId[Account](ctx, businessId, accountIds); err != nil {
		return nil, errors.New("account not found")
	}

	// check account id exists
	return variants, nil
}

func mapProductOptionsInput(input []NewProductOption) ([]ProductOption, error) {
	var options []ProductOption
	for _, o := range input {
		options = append(options, ProductOption{
			OptionName:  o.OptionName,
			OptionUnits: o.OptionUnits,
		})
	}
	return options, nil
}

// func (pg *ProductGroup) mapProductOptions(input []ProductOption) error {
// 	if len(pg.Options) != len(input) {
// 		return errors.New("cannot add a new product option")
// 	}

// 	for i := 0; i < len(input); i++ {
// 		pg.Options[i].OptionName = input[i].OptionName
// 		existingUnits := strings.Split(pg.Options[i].OptionUnits, "|")
// 		inputUnits := strings.Split(input[i].OptionUnits, "|")
// 		if len(inputUnits) < len(existingUnits) {
// 			return errors.New("cannot delete a product value")
// 		}
// 		for _,
// 	}
// }

// both existing and input options must be sorted in the same order
func mapProductOptions(existingOptions []ProductOption, input []NewProductOption) ([]ProductOption, error) {
	if len(existingOptions) != len(input) {
		return nil, errors.New("invalid product options")
	}
	for i := 0; i < len(input); i++ {
		if existingOptions[i].Id != input[i].Id {
			return nil, errors.New("invalid product option id / arrangement")
		}
		if len(strings.Split(existingOptions[i].OptionUnits, "|")) != len(strings.Split(input[i].OptionUnits, "|")) {
			return nil, errors.New("invalid option unit length")
		}

		existingOptions[i].OptionName = input[i].OptionName
		existingOptions[i].OptionUnits = input[i].OptionUnits
	}
	return existingOptions, nil
}

// using closure to keep track of validOptionUnits
func productVariantValidator(productGroupName string, options []ProductOption) func(string) error {

	validOptionUnits := make([][]string, 0, len(options))
	for _, op := range options {
		optionUnits := strings.Split(op.OptionUnits, "|")
		validOptionUnits = append(validOptionUnits, optionUnits)
	}
	usedVariantNames := make(map[string]bool)

	// T-Shirt - Yellow / Big
	return func(s string) error {

		if b := usedVariantNames[s]; b {
			return errors.New("duplicate product variant name")
		}

		// example = "Shirt - Red / Medium"
		slc := strings.Split(s, " - ")
		if len(slc) != 2 {
			return errors.New("error parsing variant name:" + s)
		}

		if slc[0] != productGroupName {
			return errors.New("invalid product group name")
		}

		optionUnits := strings.Split(slc[1], " / ")
		if len(optionUnits) != len(validOptionUnits) {
			return errors.New("variant name missing one/more option units:" + s)
		}

		for i := 0; i < len(optionUnits); i++ {
			unit := optionUnits[i]
			// validate if inputUnit exists in array
			idx := slices.Index(validOptionUnits[i], unit)
			if idx == -1 {
				return errors.New("invalid variant unit:" + unit)
			}
		}
		usedVariantNames[s] = true
		return nil
	}
}

// func GenerateProductVariantNames(allUnits [][]string, s string, i int, results []string) {
// 	if i == len(allUnits) {
// 		results = append(results, s)
// 		return
// 	}

// 	for _, unit := range allUnits[i] {
// 		GenerateProductVariantNames(allUnits, s+" / "+unit, i+1, results)
// 	}
// }

// func generateUnitNames(optionUnits []string) []string {

// 	for _, unitStr := range optionUnits {
// 		s := strings.Join(strings.Split(unitStr, "|"), "Pen - ")
// 	}
// }

func CreateProductGroup(ctx context.Context, input *NewProductGroup) (*ProductGroup, error) {
	db := config.GetDB()

	businessId, ok := utils.GetBusinessIdFromContext(ctx)
	if !ok || businessId == "" {
		return nil, errors.New("business id is required")
	}

	// validate product
	if err := input.validate(ctx, businessId, 0); err != nil {
		return nil, err
	}

	// construct productModifiers
	modifiers, err := mapProductGroupModifierInput(ctx, businessId, input.Modifiers)
	if err != nil {
		return nil, err
	}
	// create productOption
	options, err := mapProductOptionsInput(input.Options)
	if err != nil {
		return nil, err
	}

	// create productVariant
	variants, err := mapProductVariantsInput(ctx, businessId, 0, *input, options)
	if err != nil {
		return nil, err
	}

	// construct Images
	images, err := mapNewImages(input.Images, "product_groups", 0)
	if err != nil {
		return nil, err
	}

	// store product
	productGroup := ProductGroup{
		BusinessId:      businessId,
		Name:            input.Name,
		Description:     input.Description,
		CategoryId:      input.CategoryId,
		Modifiers:       modifiers,
		Variants:        variants,
		Options:         options,
		SupplierId:      input.SupplierId,
		Images:          images,
		IsBatchTracking: input.IsBatchTracking,
	}
	// db action
	err = db.WithContext(ctx).Omit("Modifiers.*", "Variants.ID").Create(&productGroup).Error
	if err != nil {
		return nil, err
	}

	return &productGroup, nil
}

func UpdateProductGroup(ctx context.Context, id int, input *NewProductGroup) (*ProductGroup, error) {

	businessId, ok := utils.GetBusinessIdFromContext(ctx)
	if !ok || businessId == "" {
		return nil, errors.New("business id is required")
	}

	// validate product group
	if err := input.validate(ctx, businessId, id); err != nil {
		return nil, err
	}

	// fetch product group
	productGroup, err := utils.FetchModel[ProductGroup](ctx, businessId, id)
	if err != nil {
		return nil, err
	}

	db := config.GetDB()

	var existingProductOptions []ProductOption
	// fetch product options explicitly order by id
	if err := db.WithContext(ctx).Model(&ProductOption{}).Where("product_group_id = ? ORDER BY id", id).Find(&existingProductOptions).Error; err != nil {
		return nil, err
	}

	// sort the input Product Options
	slices.SortFunc(input.Options, func(a NewProductOption, b NewProductOption) int {
		return a.Id - b.Id
	})

	inputOptions, err := mapProductOptions(existingProductOptions, input.Options)
	if err != nil {
		return nil, err
	}
	productGroup.Options = existingProductOptions

	validateProductVariantName := productVariantValidator(input.Name, inputOptions)

	// // if stock(s) exist, inventory account cannot be null
	// var count int64
	// for _, variant := range variants {
	// 	if variant.InventoryAccountId == 0 {
	// 		if err := db.WithContext(ctx).Model(&StockSummary{}).
	// 			Where("product_id = ? AND product_type = ?", variant.ID, ProductTypeVariant).Count(&count).Error; err != nil {
	// 			return nil, err
	// 		}
	// 		if count > 0 {
	// 			return nil, errors.New("cannot disable inventory tracking as stock(s) exist")
	// 		}
	// 	}
	// }

	// check if stock exists if disable IsBatchTracking
	if input.IsBatchTracking != nil {
		if *productGroup.IsBatchTracking && !*input.IsBatchTracking {
			if b, err := productGroup.checkStockExists(db.WithContext(ctx)); err != nil || b {
				if err == nil {
					err = errors.New("cannot disable IsBatchTracking: product group have stocks")
				}
				return nil, err
			}
		}
	}

	tx := db.Begin()
	err = tx.WithContext(ctx).Model(&productGroup).Omit("Modifiers.*").Updates(map[string]interface{}{
		"Name":            input.Name,
		"Description":     input.Description,
		"CategoryId":      input.CategoryId,
		"SupplierId":      input.SupplierId,
		"IsBatchTracking": input.IsBatchTracking,
	}).Error
	if err != nil {
		tx.Rollback()
		return nil, err
	}

	// using gorm Session to update product options
	if err := tx.WithContext(ctx).Model(&productGroup).
		Session(&gorm.Session{FullSaveAssociations: true, SkipHooks: true}).
		Association("Options").
		Unscoped().
		Replace(&inputOptions); err != nil {
		tx.Rollback()
		return nil, err
	}

	images, err := UpsertImages(ctx, tx, input.Images, "product_groups", id)
	if err != nil {
		tx.Rollback()
		return nil, err
	}
	productGroup.Images = images

	// edit variant info
	// var inputVariantIds []int
	// var count int64
	for _, iv := range input.Variants {

		// // check if product variant has stock
		// count, err := utils.ResourceCountWhere[StockSummary](ctx, businessId, "product_id = ? AND product_type = 'V'", iv.VariantId)
		// if err != nil {
		// 	tx.Rollback()
		// 	return nil, err
		// }
		// if count > 0 {
		// 	tx.Rollback()
		// 	return nil, errors.New("not allowed to change product variant that has stocks")
		// }

		// validate input product variant name according to product options
		variantNameErr := validateProductVariantName(iv.Name)
		if variantNameErr != nil {
			return nil, variantNameErr
			// do nothing if variant name is invalid but don't have stock as it will simply get deleted in the end
			// if err := db.WithContext(ctx).Model(&StockSummary{}).
			// 	Where("product_type = 'V' AND product_id = ?", iv.VariantId).
			// 	Count(&count).Error; err != nil {
			// 	tx.Rollback()
			// 	return nil, err
			// }
			// if count > 0 {
			// 	tx.Rollback()
			// 	return nil, variantNameErr
			// }
			// continue
		}

		// if iv.VariantId == 0 {
		// 	newVariant := ProductVariant{
		// 		ProductGroupId:      id,
		// 		Name:                iv.Name,
		// 		BusinessId:          businessId,
		// 		Sku:                 iv.Sku,
		// 		UnitId:              iv.UnitId,
		// 		Barcode:             iv.Barcode,
		// 		SalesPrice:          iv.SalesPrice,
		// 		SalesAccountId:      iv.SalesAccountId,
		// 		SalesTaxId:          iv.SalesTaxId,
		// 		SalesTaxType:        iv.SalesTaxType,
		// 		IsSalesTaxInclusive: iv.IsSalesTaxInclusive,
		// 		PurchasePrice:       iv.PurchasePrice,
		// 		PurchaseAccountId:   iv.PurchaseAccountId,
		// 		PurchaseTaxId:       iv.PurchaseTaxId,
		// 		PurchaseTaxType:     iv.PurchaseTaxType,
		// 		InventoryAccountId:  iv.InventoryAccountId,
		// 		IsBatchTracking:     input.IsBatchTracking,
		// 	}
		// 	// create variant
		// 	if err := tx.WithContext(ctx).Create(&newVariant).Error; err != nil {
		// 		tx.Rollback()
		// 		return nil, err
		// 	}
		// 	iv.VariantId = newVariant.ID
		// } else {
		if err := tx.WithContext(ctx).Model(&ProductVariant{}).
			Where("product_group_id = ? AND id = ?", id, iv.VariantId).
			Updates(map[string]interface{}{
				"Name": iv.Name,
				//@ allow to change unit id
				"UnitId":        iv.UnitId,
				"Barcode":       iv.Barcode,
				"Sku":           iv.Sku,
				"SalesPrice":    iv.SalesPrice,
				"PurchasePrice": iv.PurchasePrice,
			}).Error; err != nil {
			tx.Rollback()
			return nil, err
		}
		// }
		// inputVariantIds = append(inputVariantIds, iv.VariantId)
	}

	// var leftVariantIds []int
	// if err := db.WithContext(ctx).Model(&ProductVariant{}).Where("product_group_id = ? AND NOT id IN ?", id, inputVariantIds).Select("id").Scan(&leftVariantIds).Error; err != nil {
	// 	tx.Rollback()
	// 	return nil, err
	// }

	// // check if any left variant has stock
	// if len(leftVariantIds) > 0 {
	// 	if err := db.WithContext(ctx).Model(&StockSummary{}).Where("product_type = 'V' AND product_id IN ?", leftVariantIds).Count(&count).Error; err != nil {
	// 		tx.Rollback()
	// 		return nil, err
	// 	}
	// 	if count > 0 {
	// 		tx.Rollback()
	// 		return nil, errors.New("input cannot leave variant that has stock")
	// 	}
	// 	// delete left variants
	// 	if err := tx.WithContext(ctx).Where("id IN ?", leftVariantIds).Delete(&ProductVariant{}).Error; err != nil {
	// 		tx.Rollback()
	// 		return nil, err
	// 	}

	// }

	return productGroup, tx.Commit().Error
}

// check if product group's variants have any stocks
func (pg *ProductGroup) checkStockExists(db *gorm.DB) (bool, error) {
	var count int64
	// check if related product variants have stock
	sql := `SELECT count(*) FROM stock_summaries WHERE product_type = 'V' AND product_id IN ( SELECT id FROM product_variants WHERE product_group_id = ?);`
	if err := db.Raw(sql, pg.ID).Scan(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

func DeleteProductGroup(ctx context.Context, id int) (*ProductGroup, error) {

	businessId, ok := utils.GetBusinessIdFromContext(ctx)
	if !ok || businessId == "" {
		return nil, errors.New("business id is required")
	}

	result, err := utils.FetchModel[ProductGroup](ctx, businessId, id, "Images", "Variants")
	if err != nil {
		return nil, err
	}

	db := config.GetDB()

	// check if related product variants have stock
	if b, err := result.checkStockExists(db.WithContext(ctx)); err != nil || b {
		if err == nil {
			err = errors.New("cannot delete product group that have stocks")
		}
		return nil, err
	}

	if err := result.validateTransactions(ctx); err != nil {
		return nil, err
	}

	tx := db.Begin()
	// clearing association but not deleting associated data
	err = tx.WithContext(ctx).Model(&result).Association("Modifiers").Clear()
	if err != nil {
		tx.Rollback()
		return nil, err
	}

	// deleting associated variants
	err = tx.WithContext(ctx).Model(&result).Association("Variants").Unscoped().Clear()
	if err != nil {
		tx.Rollback()
		return nil, err
	}

	// deleting associated options
	err = tx.WithContext(ctx).Model(&result).Association("Options").Unscoped().Clear()
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

func GetProductGroup(ctx context.Context, id int) (*ProductGroup, error) {

	return GetResource[ProductGroup](ctx, id, "Options")
}

func GetProductGroups(ctx context.Context, name *string) ([]*ProductGroup, error) {
	db := config.GetDB()
	var results []*ProductGroup

	// fieldNames, err := utils.GetQueryFields(ctx, &ProductGroup{})
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
	// err = dbCtx.Select(fieldNames).Order("name").
	err := dbCtx.Order("name").
		Preload("Options").
		Find(&results).Error
	if err != nil {
		return nil, err
	}
	return results, nil
}

func ToggleActiveProductGroup(ctx context.Context, id int, isActive bool) (*ProductGroup, error) {
	businessId, ok := utils.GetBusinessIdFromContext(ctx)
	if !ok || businessId == "" {
		return nil, errors.New("business id is required")
	}

	return ToggleActiveModel[ProductGroup](ctx, businessId, id, isActive)
}

func PaginateProductGroup(ctx context.Context, limit *int, after *string,
	name *string, sku *string) (*ProductGroupsConnection, error) {
	businessId, ok := utils.GetBusinessIdFromContext(ctx)
	if !ok || businessId == "" {
		return nil, errors.New("business id is required")
	}

	db := config.GetDB()
	dbCtx := db.WithContext(ctx).Where("product_groups.business_id = ?", businessId)
	if name != nil && *name != "" {
		dbCtx.Where("name LIKE ?", "%"+*name+"%")
	}
	if sku != nil && *sku != "" {
		dbCtx = dbCtx.Joins("JOIN product_variants ON product_variants.product_group_id = product_groups.id").
			Where("product_variants.sku = ?", *sku)
	}
	dbCtx.Preload("Options")

	edges, pageInfo, err := FetchPageCompositeCursor[ProductGroup](dbCtx, *limit, after, "name", ">")
	if err != nil {
		return nil, err
	}

	var productGroupsConnection ProductGroupsConnection
	productGroupsConnection.PageInfo = pageInfo
	for _, edge := range edges {
		productGroupsEdge := ProductGroupsEdge(edge)
		productGroupsConnection.Edges = append(productGroupsConnection.Edges, &productGroupsEdge)
	}

	return &productGroupsConnection, nil
}
