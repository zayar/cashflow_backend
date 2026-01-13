package models

import (
	"context"
	"errors"
	"time"

	"bitbucket.org/mmdatafocus/books_backend/config"
	"bitbucket.org/mmdatafocus/books_backend/utils"
	"github.com/shopspring/decimal"
)

type TaxInfo struct {
	ID       string          `json:"id"`
	Name     string          `json:"name"`
	Rate     decimal.Decimal `json:"rate"`
	Type     TaxType         `json:"type"`
	IsActive bool            `json:"isActive"`
}

type Tax struct {
	ID            int             `gorm:"primary_key" json:"id"`
	BusinessId    string          `gorm:"index;not null" json:"business_id" binding:"required"`
	Name          string          `gorm:"size:100;not null" json:"name" binding:"required"`
	Rate          decimal.Decimal `gorm:"type:decimal(20,4);default:0" json:"rate" binding:"required"`
	IsCompoundTax *bool           `gorm:"not null" json:"is_compound_tax" binding:"required"`
	IsActive      *bool           `gorm:"not null;default:true" json:"is_active"`
	CreatedAt     time.Time       `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt     time.Time       `gorm:"autoUpdateTime" json:"updated_at"`
}

type NewTax struct {
	Name          string          `json:"name" binding:"required"`
	Rate          decimal.Decimal `json:"rate" binding:"required"`
	IsCompoundTax *bool           `json:"is_compound_tax" binding:"required"`
}

// validate input for both create & update. (id = 0 for create)

func (input *NewTax) validate(ctx context.Context, businessId string, id int) error {
	// name
	if err := utils.ValidateUnique[Tax](ctx, businessId, "name", input.Name, id); err != nil {
		return err
	}
	return nil
}

func checkCompoundExistInTaxGroupIds(ctx context.Context, taxGroupIds []int) error {
	db := config.GetDB()
	taxIds := db.WithContext(ctx).Table("group_taxes").Select("tax_id").Where("tax_group_id IN ?", taxGroupIds)
	var count int64
	err := db.WithContext(ctx).Table("taxes").Where("is_compound_tax", true).Where("id IN (?)", taxIds).Count(&count).Error
	if err != nil {
		return err
	}
	if count > 0 {
		return errors.New("one of tax groups has other compound")
	}
	return nil
}
func CreateTax(ctx context.Context, input *NewTax) (*Tax, error) {
	businessId, ok := utils.GetBusinessIdFromContext(ctx)
	if !ok || businessId == "" {
		return nil, errors.New("business id is required")
	}

	if err := input.validate(ctx, businessId, 0); err != nil {
		return nil, err
	}

	tax := Tax{
		BusinessId:    businessId,
		Name:          input.Name,
		Rate:          input.Rate,
		IsCompoundTax: input.IsCompoundTax,
	}

	db := config.GetDB()
	// db action
	err := db.WithContext(ctx).Create(&tax).Error
	if err != nil {
		return nil, err
	}
	return &tax, nil
}

func UpdateTax(ctx context.Context, id int, input *NewTax) (*Tax, error) {
	businessId, ok := utils.GetBusinessIdFromContext(ctx)
	if !ok || businessId == "" {
		return nil, errors.New("business id is required")
	}

	// validate compound tax rule
	beforeUpdate, err := utils.FetchModel[Tax](ctx, businessId, id)
	if err != nil {
		return nil, err
	}
	if err := input.validate(ctx, businessId, id); err != nil {
		return nil, err
	}

	// validate isCompoundTax
	// check for other compound in related tax groups if modified as compound
	db := config.GetDB()
	var taxGroupIds []int
	err = db.WithContext(ctx).Table("group_taxes").
		Where("tax_id = ?", id).Pluck("tax_group_id", &taxGroupIds).Error
	if err != nil {
		return nil, err
	}

	if !*beforeUpdate.IsCompoundTax && *input.IsCompoundTax {
		err := checkCompoundExistInTaxGroupIds(ctx, taxGroupIds)
		if err != nil {
			return nil, err
		}
	}

	updateTax := Tax{
		ID:            id,
		BusinessId:    businessId,
		Name:          input.Name,
		Rate:          input.Rate,
		IsCompoundTax: input.IsCompoundTax,
	}
	// update tax
	// db action
	tx := db.Begin()
	err = tx.WithContext(ctx).Model(&updateTax).Updates(map[string]interface{}{
		"Name":          updateTax.Name,
		"Rate":          updateTax.Rate,
		"IsCompoundTax": updateTax.IsCompoundTax,
	}).Error
	if err != nil {
		tx.Rollback()
		return nil, err
	}

	// update related tax group's rate
	if beforeUpdate.Rate != updateTax.Rate || beforeUpdate.IsCompoundTax != updateTax.IsCompoundTax {
		// get related tax groups, preloaded taxes, to calculate rate
		var relatedTaxGroups []TaxGroup
		err := db.WithContext(ctx).Model(&TaxGroup{}).
			Preload("Taxes").Where("id IN ?", taxGroupIds).Find(&relatedTaxGroups).Error
		if err != nil {
			tx.Rollback()
			return nil, err
		}
		for _, g := range relatedTaxGroups {
			// update current tax's rate
			// new rate has not been updated in database yet, due to transaction
			for i, t := range g.Taxes {
				if t.ID == id {
					g.Taxes[i] = updateTax
				}
			}
			g.Rate = g.CalculateRate()
			// db action
			err := tx.WithContext(ctx).Model(&g).Updates(map[string]interface{}{
				"Rate": g.Rate,
			}).Error
			if err != nil {
				tx.Rollback()
				return nil, err
			}
		}
	}

	if err := tx.Commit().Error; err != nil {
		return nil, err
	}

	return &updateTax, nil
}

func DeleteTax(ctx context.Context, id int) (*Tax, error) {
	businessId, ok := utils.GetBusinessIdFromContext(ctx)
	if !ok || businessId == "" {
		return nil, errors.New("business id is required")
	}

	db := config.GetDB()

	// check exists
	result, err := utils.FetchModel[Tax](ctx, businessId, id)
	if err != nil {
		return nil, err
	}

	// check if tax is used
	var count int64
	if err := db.WithContext(ctx).Table("group_taxes").Where("tax_id = ?", id).Count(&count).Error; err != nil {
		return nil, err
	}
	if count > 0 {
		return nil, errors.New("used by tax group")
	}
	if err := db.WithContext(ctx).Model(&Product{}).
		Where("sales_tax_type = 'I' AND sales_tax_id = ?", id).
		Or("purchase_tax_type = 'I' AND purchase_tax_id = ?", id).
		Count(&count).Error; err != nil {
		return nil, err
	}
	if count > 0 {
		return nil, errors.New("used by product")
	}
	if err := db.WithContext(ctx).Model(&Customer{}).
		Where("customer_tax_type = 'I' AND customer_tax_id = ?", id).
		Count(&count).Error; err != nil {
		return nil, err
	}
	if count > 0 {
		return nil, errors.New("used by customer")
	}
	if err := db.WithContext(ctx).Model(&Supplier{}).
		Where("supplier_tax_type = 'I' AND supplier_tax_id = ?", id).
		Count(&count).Error; err != nil {
		return nil, err
	}
	if count > 0 {
		return nil, errors.New("used by supplier")
	}

	// db action
	err = db.WithContext(ctx).Delete(&result).Error
	if err != nil {
		return nil, err
	}

	return result, nil
}

func GetTax(ctx context.Context, id int) (*Tax, error) {
	return GetResource[Tax](ctx, id)
}

func GetTaxes(ctx context.Context, name *string) ([]*Tax, error) {
	businessId, ok := utils.GetBusinessIdFromContext(ctx)
	if !ok || businessId == "" {
		return nil, errors.New("business id is required")
	}

	var results []*Tax
	db := config.GetDB()
	fieldNames, err := utils.GetQueryFields(ctx, &Tax{})
	if err != nil {
		return nil, err
	}

	dbCtx := db.WithContext(ctx).Where("business_id = ?", businessId)
	if name != nil && *name != "" {
		dbCtx = dbCtx.Where("name LIKE ?", "%"+*name+"%")
	}
	// db query
	err = dbCtx.Select(fieldNames).Order("name").Find(&results).Error
	if err != nil {
		return nil, err
	}
	return results, nil
}

func ToggleActiveTax(ctx context.Context, id int, isActive bool) (*Tax, error) {
	businessId, ok := utils.GetBusinessIdFromContext(ctx)
	if !ok || businessId == "" {
		return nil, errors.New("business id is required")
	}

	return ToggleActiveModel[Tax](ctx, businessId, id, isActive)
}

// func GetTaxes(ctx context.Context, first *int, after *string) (*TaxPagination, error) {

// 	decodedCursor, _ := DecodeCursor(after)
// 	edges := make([]*TaxEdge, *first)
// 	count := 0
// 	hasNextPage := false

// 	db := config.GetDB()
// 	var results []Tax
// 	var err error

// 	if decodedCursor == "" {
// 		err = db.WithContext(ctx).Order("name").Limit(*first + 1).Find(&results).Error
// 	} else {
// 		err = db.WithContext(ctx).Order("name").Limit(*first+1).Where("name > ?", decodedCursor).Find(&results).Error
// 	}

// 	if err != nil {
// 		return nil, err
// 	}

// 	for _, result := range results {
// 		// If there are any elements left after the current page
// 		// we indicate that in the response
// 		if count == *first {
// 			hasNextPage = true
// 		}

// 		if count < *first {
// 			edges[count] = &TaxEdge{
// 				Cursor: EncodeCursor(result.Name),
// 				Node:   result,
// 			}
// 			count++
// 		}
// 	}

// 	pageInfo := PageInfo{
// 		StartCursor: EncodeCursor(edges[0].Node.Name),
// 		EndCursor:   EncodeCursor(edges[count-1].Node.Name),
// 		HasNextPage: &hasNextPage,
// 	}

// 	taxes := TaxPagination{
// 		Edges:    edges[:count],
// 		PageInfo: &pageInfo,
// 	}

// 	return &taxes, nil
// }

// func SearchTaxes(ctx context.Context, name *string) ([]*Tax, error) {

// 	db := config.GetDB()
// 	var results []*Tax

// 	err := db.WithContext(ctx).Where("name LIKE ?", "%"+*name+"%").Limit(config.SearchLimit).Find(&results).Error
// 	if err != nil {
// 		return nil, err
// 	}
// 	return results, nil
// }
