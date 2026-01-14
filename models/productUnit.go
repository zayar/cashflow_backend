package models

import (
	"context"
	"errors"
	"time"

	"github.com/mmdatafocus/books_backend/config"
	"github.com/mmdatafocus/books_backend/utils"
)

type ProductUnit struct {
	ID           int       `gorm:"primary_key" json:"id"`
	BusinessId   string    `gorm:"index;not null" json:"business_id"`
	Name         string    `gorm:"size:20;not null" json:"name" binding:"required"`
	Abbreviation string    `gorm:"size:7;not null" json:"abbreviation" binding:"required"`
	Precision    Precision `gorm:"type:enum('0','1','2','3','4');default:'0';size:1;not null" json:"precision" binding:"required"`
	IsActive     *bool     `gorm:"not null;default:true" json:"is_active"`
	CreatedAt    time.Time `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt    time.Time `gorm:"autoUpdateTime" json:"updated_at"`
}

type NewProductUnit struct {
	Name         string    `json:"name" binding:"required"`
	Abbreviation string    `json:"abbreviation" binding:"required"`
	Precision    Precision `json:"precision" binding:"required"`
}

type ProductUnitsEdge Edge[ProductUnit]
type ProductUnitsConnection struct {
	PageInfo *PageInfo           `json:"pageInfo"`
	Edges    []*ProductUnitsEdge `json:"edges"`
}

// node
// returns decoded curosr string
func (pu ProductUnit) GetCursor() string {
	return pu.Name
}

func (input *NewProductUnit) validate(ctx context.Context, businessId string, id int) error {
	if err := utils.ValidateUnique[ProductUnit](ctx, businessId, "name", input.Name, id); err != nil {
		return err
	}
	if err := utils.ValidateUnique[ProductUnit](ctx, businessId, "abbreviation", input.Abbreviation, id); err != nil {
		return err
	}

	return nil
}

func CreateProductUnit(ctx context.Context, input *NewProductUnit) (*ProductUnit, error) {

	businessId, ok := utils.GetBusinessIdFromContext(ctx)
	if !ok || businessId == "" {
		return nil, errors.New("business id is required")
	}

	if err := input.validate(ctx, businessId, 0); err != nil {
		return nil, err
	}

	unit := ProductUnit{
		BusinessId:   businessId,
		Name:         input.Name,
		Abbreviation: input.Abbreviation,
		Precision:    input.Precision,
	}

	db := config.GetDB()
	err := db.WithContext(ctx).Create(&unit).Error
	if err != nil {
		return nil, err
	}
	return &unit, nil
}

func UpdateProductUnit(ctx context.Context, id int, input *NewProductUnit) (*ProductUnit, error) {

	businessId, ok := utils.GetBusinessIdFromContext(ctx)
	if !ok || businessId == "" {
		return nil, errors.New("business id is required")
	}

	if err := input.validate(ctx, businessId, id); err != nil {
		return nil, err
	}

	unit, err := utils.FetchModel[ProductUnit](ctx, businessId, id)
	if err != nil {
		return nil, err
	}

	db := config.GetDB()
	err = db.WithContext(ctx).Model(&unit).Updates(map[string]interface{}{
		"Name":         input.Name,
		"Abbreviation": input.Abbreviation,
		"Precision":    input.Precision,
	}).Error
	if err != nil {
		return nil, err
	}
	return unit, nil
}

func DeleteProductUnit(ctx context.Context, id int) (*ProductUnit, error) {

	businessId, ok := utils.GetBusinessIdFromContext(ctx)
	if !ok || businessId == "" {
		return nil, errors.New("business id is required")
	}
	result, err := utils.FetchModel[ProductUnit](ctx, businessId, id)
	if err != nil {
		return nil, err
	}

	// don't delete if productUnit is used by product or product variant
	count, err := utils.ResourceCountWhere[Product](ctx, businessId, "unit_id = ?", id)
	if err != nil {
		return nil, err
	}
	if count > 0 {
		return nil, errors.New("used by product")
	}
	count, err = utils.ResourceCountWhere[ProductVariant](ctx, businessId, "unit_id = ?", id)
	if err != nil {
		return nil, err
	}
	if count > 0 {
		return nil, errors.New("used by product variant")
	}

	db := config.GetDB()
	// db action
	err = db.WithContext(ctx).Delete(&result).Error
	if err != nil {
		return nil, err
	}
	return result, nil
}

func GetProductUnit(ctx context.Context, id int) (*ProductUnit, error) {

	return GetResource[ProductUnit](ctx, id)
}

func GetProductUnits(ctx context.Context, name *string) ([]*ProductUnit, error) {

	db := config.GetDB()
	var results []*ProductUnit

	fieldNames, err := utils.GetQueryFields(ctx, &ProductUnit{})
	if err != nil {
		return nil, err
	}

	businessId, ok := utils.GetBusinessIdFromContext(ctx)
	if !ok || businessId == "" {
		return nil, errors.New("business id is required")
	}

	dbCtx := db.WithContext(ctx).Where("business_id = ?", businessId)
	if name != nil && len(*name) > 0 {
		dbCtx = dbCtx.Where("name LIKE ?", "%"+*name+"%")
	}
	err = dbCtx.Select(fieldNames).Order("name").Find(&results).Error
	if err != nil {
		return nil, err
	}
	return results, nil
}

func ToggleActiveProductUnit(ctx context.Context, id int, isActive bool) (*ProductUnit, error) {
	// <owner>
	businessId, ok := utils.GetBusinessIdFromContext(ctx)
	if !ok || businessId == "" {
		return nil, errors.New("business id is required")
	}

	return ToggleActiveModel[ProductUnit](ctx, businessId, id, isActive)
}

func PaginateProductUnit(ctx context.Context, limit *int, after *string, name *string) (*ProductUnitsConnection, error) {
	businessId, ok := utils.GetBusinessIdFromContext(ctx)
	if !ok || businessId == "" {
		return nil, errors.New("business id is required")
	}

	db := config.GetDB()
	dbCtx := db.WithContext(ctx).Where("business_id = ?", businessId)
	if name != nil && *name != "" {
		dbCtx.Where("name LIKE ?", "%"+*name+"%")
	}
	edges, pageInfo, err := FetchPagePureCursor[ProductUnit](dbCtx, *limit, after, "name", ">")
	if err != nil {
		return nil, err
	}
	var productUnitsConnection ProductUnitsConnection
	productUnitsConnection.PageInfo = pageInfo
	for _, edge := range edges {
		productUnitEdge := ProductUnitsEdge(edge)
		productUnitsConnection.Edges = append(productUnitsConnection.Edges, &productUnitEdge)
	}
	return &productUnitsConnection, err
}
