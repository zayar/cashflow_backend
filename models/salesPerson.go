package models

import (
	"context"
	"errors"
	"time"

	"github.com/mmdatafocus/books_backend/config"
	"github.com/mmdatafocus/books_backend/utils"
)

type SalesPerson struct {
	ID         int       `gorm:"primary_key" json:"id"`
	BusinessId string    `gorm:"primary_key;autoIncrement:false;not null" json:"business_id" binding:"required"`
	Name       string    `gorm:"size:100;not null" json:"name" binding:"required"`
	Email      string    `gorm:"size:100" json:"email"`
	IsActive   *bool     `gorm:"not null;default:true" json:"is_active"`
	CreatedAt  time.Time `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt  time.Time `gorm:"autoUpdateTime" json:"updated_at"`
}

type NewSalesPerson struct {
	Name  string `json:"name" binding:"required"`
	Email string `json:"email"`
}

type SalesPersonsEdge Edge[SalesPerson]
type SalesPersonsConnection struct {
	PageInfo *PageInfo           `json:"pageInfo"`
	Edges    []*SalesPersonsEdge `json:"edges"`
}

// implements methods for pagination

// node
// returns decoded curosr string
func (sp SalesPerson) GetCursor() string {
	return sp.Name
}

// validate input for both create & update. (id = 0 for create)

func (input *NewSalesPerson) validate(ctx context.Context, businessId string, id int) error {
	// name
	if err := utils.ValidateUnique[SalesPerson](ctx, businessId, "name", input.Name, id); err != nil {
		return err
	}
	// email
	if len(input.Email) > 0 {
		if err := utils.ValidateUnique[SalesPerson](ctx, businessId, "email", input.Email, id); err != nil {
			return err
		}
	}
	return nil
}

func CreateSalesPerson(ctx context.Context, input *NewSalesPerson) (*SalesPerson, error) {

	businessId, ok := utils.GetBusinessIdFromContext(ctx)
	if !ok || businessId == "" {
		return nil, errors.New("business id is required")
	}
	if err := input.validate(ctx, businessId, 0); err != nil {
		return nil, err
	}

	salesPerson := SalesPerson{
		Name:       input.Name,
		BusinessId: businessId,
		Email:      input.Email,
	}

	db := config.GetDB()
	err := db.WithContext(ctx).Create(&salesPerson).Error
	if err != nil {
		return nil, err
	}
	return &salesPerson, nil
}

func UpdateSalesPerson(ctx context.Context, id int, input *NewSalesPerson) (*SalesPerson, error) {

	businessId, ok := utils.GetBusinessIdFromContext(ctx)
	if !ok || businessId == "" {
		return nil, errors.New("business id is required")
	}
	if err := input.validate(ctx, businessId, id); err != nil {
		return nil, err
	}

	salesPerson, err := utils.FetchModel[SalesPerson](ctx, businessId, id)
	if err != nil {
		return nil, err
	}

	db := config.GetDB()
	err = db.WithContext(ctx).Model(&salesPerson).Updates(map[string]interface{}{
		"Name":  input.Name,
		"Email": input.Email,
	}).Error
	if err != nil {
		return nil, err
	}
	return salesPerson, nil
}

func DeleteSalesPerson(ctx context.Context, id int) (*SalesPerson, error) {

	businessId, ok := utils.GetBusinessIdFromContext(ctx)
	if !ok || businessId == "" {
		return nil, errors.New("business id is required")
	}
	result, err := utils.FetchModel[SalesPerson](ctx, businessId, id)
	if err != nil {
		return nil, err
	}

	db := config.GetDB()
	err = db.WithContext(ctx).Delete(&result).Error
	if err != nil {
		return nil, err
	}
	return result, nil
}

func GetSalesPerson(ctx context.Context, id int) (*SalesPerson, error) {

	return GetResource[SalesPerson](ctx, id)
}

func GetSalesPersons(ctx context.Context, name *string) ([]*SalesPerson, error) {

	db := config.GetDB()
	var results []*SalesPerson

	fieldNames, err := utils.GetQueryFields(ctx, &SalesPerson{})
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

func ToggleActiveSalesPerson(ctx context.Context, id int, isActive bool) (*SalesPerson, error) {
	// <owner>
	businessId, ok := utils.GetBusinessIdFromContext(ctx)
	if !ok || businessId == "" {
		return nil, errors.New("business id is required")
	}

	return ToggleActiveModel[SalesPerson](ctx, businessId, id, isActive)
}

func PaginateSalesPerson(ctx context.Context, limit *int, after *string,
	name *string) (*SalesPersonsConnection, error) {
	businessId, ok := utils.GetBusinessIdFromContext(ctx)
	if !ok || businessId == "" {
		return nil, errors.New("business id is required")
	}

	db := config.GetDB()
	dbCtx := db.WithContext(ctx).Where("business_id = ?", businessId)
	if name != nil && *name != "" {
		dbCtx.Where("name LIKE ?", "%"+*name+"%")
	}
	edges, pageInfo, err := FetchPagePureCursor[SalesPerson](dbCtx, *limit, after, "name", ">")
	if err != nil {
		return nil, err
	}
	var salesPersonsConnection SalesPersonsConnection
	salesPersonsConnection.PageInfo = pageInfo
	for _, edge := range edges {
		salesPersonEdge := SalesPersonsEdge(edge)
		salesPersonsConnection.Edges = append(salesPersonsConnection.Edges, &salesPersonEdge)
	}
	return &salesPersonsConnection, err
}
