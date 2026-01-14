package models

import (
	"context"
	"errors"
	"time"

	"github.com/mmdatafocus/books_backend/config"
	"github.com/mmdatafocus/books_backend/utils"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

type ProductModifier struct {
	ID            int                   `gorm:"primary_key" json:"id"`
	BusinessId    string                `gorm:"index;not null" json:"business_id" binding:"required"`
	Name          string                `gorm:"size:100;not null" json:"name" binding:"required"`
	ModifierUnits []ProductModifierUnit `gorm:"foreignKey:ModifierId" json:"modifier_units" binding:"required"`
	IsActive      *bool                 `gorm:"not null;default:true" json:"is_active"`
	CreatedAt     time.Time             `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt     time.Time             `gorm:"autoUpdateTime" json:"updated_at"`
}

type ProductModifierUnit struct {
	ModifierId int             `gorm:"primaryKey;autoIncrement:false" json:"-" binding:"required"`
	UnitName   string          `gorm:"primaryKey;autoIncrement:false" json:"unit_name" binding:"required"`
	Price      decimal.Decimal `gorm:"type:decimal(20,4);default:0" json:"price"`
}

type NewProductModifier struct {
	Name          string                   `json:"name" binding:"required"`
	ModifierUnits []NewProductModifierUnit `json:"modifier_units"`
}

type NewProductModifierUnit struct {
	UnitName string          `json:"unit_name" binding:"required"`
	Price    decimal.Decimal `json:"price"`
}

type ProductModifiersEdge Edge[ProductModifier]
type ProductModifiersConnection struct {
	PageInfo *PageInfo               `json:"pageInfo"`
	Edges    []*ProductModifiersEdge `json:"edges"`
}

// node
// returns decoded curosr string
func (pm ProductModifier) GetCursor() string {
	return pm.Name
}

// validate input for both create & update. (id = 0 for create)

func (input *NewProductModifier) validate(ctx context.Context, businessId string, id int) error {
	// name
	if err := utils.ValidateUnique[ProductModifier](ctx, businessId, "name", input.Name, id); err != nil {
		return err
	}
	return nil
}

func mapProductModifierUnitsInput(input []NewProductModifierUnit) ([]ProductModifierUnit, error) {
	units := make([]ProductModifierUnit, 0)
	for _, u := range input {
		units = append(units, ProductModifierUnit{
			UnitName: u.UnitName,
			Price:    u.Price,
		})
	}

	return units, nil
}

func CreateProductModifier(ctx context.Context, input *NewProductModifier) (*ProductModifier, error) {

	businessId, ok := utils.GetBusinessIdFromContext(ctx)
	if !ok || businessId == "" {
		return nil, errors.New("business id is required")
	}

	if err := input.validate(ctx, businessId, 0); err != nil {
		return nil, err
	}

	units, err := mapProductModifierUnitsInput(input.ModifierUnits)
	if err != nil {
		return nil, err
	}

	modifier := ProductModifier{
		BusinessId:    businessId,
		Name:          input.Name,
		ModifierUnits: units,
	}

	db := config.GetDB()
	// db action
	err = db.WithContext(ctx).Create(&modifier).Error
	if err != nil {
		return nil, err
	}
	return &modifier, nil
}

func UpdateProductModifier(ctx context.Context, id int, input *NewProductModifier) (*ProductModifier, error) {

	businessId, ok := utils.GetBusinessIdFromContext(ctx)
	if !ok || businessId == "" {
		return nil, errors.New("business id is required")
	}

	if err := input.validate(ctx, businessId, id); err != nil {
		return nil, err
	}

	modifier, err := utils.FetchModel[ProductModifier](ctx, businessId, id)
	if err != nil {
		return nil, err
	}

	db := config.GetDB()
	tx := db.Begin()
	// db action
	err = tx.WithContext(ctx).Model(&modifier).Updates(map[string]interface{}{
		"Name": input.Name,
	}).Error
	if err != nil {
		tx.Rollback()
		return nil, err
	}
	// sync ModifierUnits

	units, err := mapProductModifierUnitsInput(input.ModifierUnits)
	if err != nil {
		return nil, err
	}

	// using gorm Session to upsert
	if err := tx.WithContext(ctx).Model(&modifier).
		Session(&gorm.Session{FullSaveAssociations: true, SkipHooks: true}).
		Association("ModifierUnits").
		Unscoped().
		Replace(units); err != nil {
		tx.Rollback()
		return nil, err
	}
	// save upsert history 2d
	if err := tx.Commit().Error; err != nil {
		return nil, err
	}

	return modifier, nil
}

func DeleteProductModifier(ctx context.Context, id int) (*ProductModifier, error) {

	businessId, ok := utils.GetBusinessIdFromContext(ctx)
	if !ok || businessId == "" {
		return nil, errors.New("business id is required")
	}
	result, err := utils.FetchModel[ProductModifier](ctx, businessId, id, "ModifierUnits")
	if err != nil {
		return nil, err
	}

	// don't delete if any product/productGroup use this modifier
	db := config.GetDB()
	var count int64
	err = db.WithContext(ctx).Table("products_link_modifiers").
		Where("product_modifier_id = ?", id).Count(&count).Error
	if err != nil {
		return nil, err
	}
	if count > 0 {
		return nil, errors.New("used by product")
	}
	err = db.WithContext(ctx).Table("productgroups_link_modifiers").
		Where("product_modifier_id = ?", id).Count(&count).Error
	if err != nil {
		return nil, err
	}
	if count > 0 {
		return nil, errors.New("used by product group")
	}

	// Do not delete if any Branch use this modifier
	// var count int64
	// err = db.WithContext(ctx).Model(&Branch{}).Where("transaction_number_series_id = ?", id).Count(&count).Error
	// if err != nil {
	// 	return nil, err
	// }
	// if count > 0 {
	// 	return nil, errors.New("used by branch")
	// }

	// removing including associated records
	// db action
	err = db.WithContext(ctx).Select("ModifierUnits").Delete(&result).Error
	if err != nil {
		return nil, err
	}
	return result, nil
}

func GetProductModifier(ctx context.Context, id int) (*ProductModifier, error) {

	return GetResource[ProductModifier](ctx, id, "ModifierUnits")
}

func GetProductModifiers(ctx context.Context, name *string) ([]*ProductModifier, error) {

	db := config.GetDB()
	var results []*ProductModifier

	businessId, ok := utils.GetBusinessIdFromContext(ctx)
	if !ok || businessId == "" {
		return nil, errors.New("business id is required")
	}

	dbCtx := db.WithContext(ctx).Where("business_id = ?", businessId)
	if name != nil && len(*name) > 0 {
		dbCtx = dbCtx.Where("name LIKE ?", "%"+*name+"%")
	}
	err := dbCtx.Preload("ModifierUnits").Order("name").Find(&results).Error
	if err != nil {
		return nil, err
	}
	return results, nil
}

func ToggleActiveProductModifier(ctx context.Context, id int, isActive bool) (*ProductModifier, error) {
	businessId, ok := utils.GetBusinessIdFromContext(ctx)
	if !ok || businessId == "" {
		return nil, errors.New("business id is required")
	}

	return ToggleActiveModel[ProductModifier](ctx, businessId, id, isActive)
}

func PaginateProductModifier(ctx context.Context, limit *int, after *string, name *string) (*ProductModifiersConnection, error) {
	businessId, ok := utils.GetBusinessIdFromContext(ctx)
	if !ok || businessId == "" {
		return nil, errors.New("business id is required")
	}

	db := config.GetDB()
	dbCtx := db.WithContext(ctx).Where("business_id = ?", businessId)
	dbCtx.Preload("ModifierUnits")
	if name != nil && *name != "" {
		dbCtx.Where("name LIKE ?", "%"+*name+"%")
	}
	edges, pageInfo, err := FetchPagePureCursor[ProductModifier](dbCtx, *limit, after, "name", ">")
	if err != nil {
		return nil, err
	}
	var productModifiersConnection ProductModifiersConnection
	productModifiersConnection.PageInfo = pageInfo
	for _, edge := range edges {
		productModifierEdge := ProductModifiersEdge(edge)
		productModifiersConnection.Edges = append(productModifiersConnection.Edges, &productModifierEdge)
	}
	return &productModifiersConnection, err
}
