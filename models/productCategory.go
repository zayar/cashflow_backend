package models

import (
	"context"
	"errors"
	"time"

	"github.com/mmdatafocus/books_backend/config"
	"github.com/mmdatafocus/books_backend/utils"
	"gorm.io/gorm"
)

type ProductCategory struct {
	ID               int       `gorm:"primary_key" json:"id"`
	BusinessId       string    `gorm:"index;not null" json:"business_id"`
	Name             string    `gorm:"index;size:100;not null" json:"name" binding:"required"`
	ParentCategoryId int       `gorm:"index;not null" json:"parentCategoryId"`
	IsActive         *bool     `gorm:"not null;default:true" json:"is_active"`
	CreatedAt        time.Time `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt        time.Time `gorm:"autoUpdateTime" json:"updated_at"`
}

type NewProductCategory struct {
	Name             string `json:"name" binding:"required"`
	ParentCategoryId int    `json:"parentCategoryId" binding:"required"`
}

// implements methods for pagination
type ProductCategoriesEdge Edge[ProductCategory]
type ProductCategoriesConnection struct {
	PageInfo *PageInfo                `json:"pageInfo"`
	Edges    []*ProductCategoriesEdge `json:"edges"`
}

// node
// returns decoded curosr string
func (pc ProductCategory) GetCursor() string {
	return pc.CreatedAt.String()
}

// get ids of associated products
func (pc ProductCategory) ProductIds(ctx context.Context) (ids []int, err error) {
	db := config.GetDB()
	err = db.WithContext(ctx).Model(&Product{}).
		Where("category_id = ?", pc.ID).
		Select("id").Scan(&ids).Error
	return
}

// validate input for both create & update. (id = 0 for create)

func (input *NewProductCategory) validate(ctx context.Context, businessId string, id int) error {
	if id > 0 {
		if id == input.ParentCategoryId {
			return errors.New("self-parent not allowed")
		}
	}
	// name
	if err := utils.ValidateUnique[ProductCategory](ctx, businessId, "name", input.Name, id); err != nil {
		return err
	}
	// parent category
	if input.ParentCategoryId > 0 {
		if err := utils.ValidateResourceId[ProductCategory](ctx, businessId, input.ParentCategoryId); err != nil {
			return errors.New("parent not found")
		}
	}
	return nil
}

func CreateProductCategory(ctx context.Context, input *NewProductCategory) (*ProductCategory, error) {

	businessId, ok := utils.GetBusinessIdFromContext(ctx)
	if !ok || businessId == "" {
		return nil, errors.New("business id is required")
	}

	if err := input.validate(ctx, businessId, 0); err != nil {
		return nil, err
	}

	category := ProductCategory{
		BusinessId:       businessId,
		Name:             input.Name,
		ParentCategoryId: input.ParentCategoryId,
		IsActive:         utils.NewTrue(),
	}

	db := config.GetDB()
	err := db.WithContext(ctx).Create(&category).Error
	if err != nil {
		return nil, err
	}

	return &category, nil
}

func UpdateProductCategory(ctx context.Context, id int, input *NewProductCategory) (*ProductCategory, error) {

	businessId, ok := utils.GetBusinessIdFromContext(ctx)
	if !ok || businessId == "" {
		return nil, errors.New("business id is required")
	}
	if err := input.validate(ctx, businessId, id); err != nil {
		return nil, err
	}

	category, err := utils.FetchModel[ProductCategory](ctx, businessId, id)
	if err != nil {
		return nil, err
	}

	db := config.GetDB()
	err = db.WithContext(ctx).Model(&category).Updates(map[string]interface{}{
		"Name":             input.Name,
		"ParentCategoryId": input.ParentCategoryId,
	}).Error
	if err != nil {
		return nil, err
	}
	return category, nil
}

func DeleteProductCategory(ctx context.Context, id int) (*ProductCategory, error) {

	businessId, ok := utils.GetBusinessIdFromContext(ctx)
	if !ok || businessId == "" {
		return nil, errors.New("business id is required")
	}
	db := config.GetDB()
	result, err := utils.FetchModel[ProductCategory](ctx, businessId, id)
	if err != nil {
		return nil, err
	}

	// don't delete if productCategory has childern
	count, err := utils.ResourceCountWhere[ProductCategory](ctx, businessId, "parent_category_id = ?", id)
	if err != nil {
		return nil, err
	}
	if count > 0 {
		return nil, errors.New("category has children")
	}

	// don't delete if productCategory is used by product or product variant
	count, err = utils.ResourceCountWhere[Product](ctx, businessId, "category_id = ?", id)
	if err != nil {
		return nil, err
	}
	if count > 0 {
		return nil, errors.New("used by product")
	}
	count, err = utils.ResourceCountWhere[ProductGroup](ctx, businessId, "category_id = ?", id)
	if err != nil {
		return nil, err
	}
	if count > 0 {
		return nil, errors.New("used by product group")
	}

	err = db.WithContext(ctx).Delete(&result).Error
	if err != nil {
		return nil, err
	}

	return result, nil
}

func GetProductCategory(ctx context.Context, id int) (*ProductCategory, error) {

	return GetResource[ProductCategory](ctx, id)
}

func GetProductCategories(ctx context.Context, name *string) ([]*ProductCategory, error) {

	db := config.GetDB()
	var results []*ProductCategory

	fieldNames, err := utils.GetQueryFields(ctx, &ProductCategory{})
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

func ToggleActiveProductCategory(ctx context.Context, id int, isActive bool) (*ProductCategory, error) {
	businessId, ok := utils.GetBusinessIdFromContext(ctx)
	if !ok || businessId == "" {
		return nil, errors.New("business id is required")
	}

	var category ProductCategory
	db := config.GetDB()
	if err := db.WithContext(ctx).Where("business_id = ? AND id = ?", businessId, id).Find(&category).Error; err != nil {
		return nil, utils.ErrorRecordNotFound
	}

	tx := db.Begin()
	if err := tx.Model(&category).Updates(map[string]interface{}{
		"is_active": isActive,
	}).Error; err != nil {
		tx.Rollback()
		return nil, err
	}

	if err := toggleChildrenCategories(ctx, tx, id, isActive); err != nil {
		tx.Rollback()
		return &category, err
	}

	return &category, tx.Commit().Error
}

// toggle children of the parent recursively, parent is assumed to have toggled
func toggleChildrenCategories(ctx context.Context, tx *gorm.DB, parentId int, isActive bool) error {
	// get children ids
	// toggle them
	// toggle children of each child
	// break when a parent has no children

	var childrenIds []int
	if err := tx.WithContext(ctx).
		Model(&ProductCategory{}).
		Where("parent_category_id = ?", parentId).
		Select("id").
		Scan(&childrenIds).Error; err != nil {
		return err
	}

	// base case
	// break when parent has no children
	if len(childrenIds) == 0 {
		return nil
	}

	if err := tx.WithContext(ctx).Model(&ProductCategory{}).
		Where("id IN ?", childrenIds).Updates(map[string]interface{}{
		"is_active": isActive,
	}).Error; err != nil {
		return err
	}

	for _, childId := range childrenIds {
		// each child becomes a parent
		if err := toggleChildrenCategories(ctx, tx, childId, isActive); err != nil {
			return err
		}
	}
	return nil
}

func PaginateProductCategories(ctx context.Context, limit *int, after *string, name *string, parentCategoryId *int) (*ProductCategoriesConnection, error) {

	db := config.GetDB()
	businessId, ok := utils.GetBusinessIdFromContext(ctx)
	if !ok || businessId == "" {
		return nil, errors.New("business id is required")
	}
	dbCtx := db.WithContext(ctx).Where("business_id = ?", businessId)

	if name != nil && *name != "" {
		dbCtx.Where("name LIKE ?", "%"+*name+"%")
	}
	if parentCategoryId != nil && *parentCategoryId > 0 {
		dbCtx.Where("parent_category_id = ?", *parentCategoryId)
	}
	edges, pageInfo, err := FetchPageCompositeCursor[ProductCategory](dbCtx, *limit, after, "created_at", "<")
	if err != nil {
		return nil, err
	}
	var productCategoriesConnection ProductCategoriesConnection
	productCategoriesConnection.PageInfo = pageInfo
	for _, edge := range edges {
		productCategoryEdge := ProductCategoriesEdge(edge)
		productCategoriesConnection.Edges = append(productCategoriesConnection.Edges, &productCategoryEdge)
	}
	return &productCategoriesConnection, nil
}
