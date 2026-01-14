package models

import (
	"context"
	"errors"
	"time"

	"github.com/mmdatafocus/books_backend/config"
	"github.com/mmdatafocus/books_backend/utils"
	"github.com/shopspring/decimal"
)

type TaxGroup struct {
	ID         int             `gorm:"primary_key" json:"id"`
	BusinessId string          `gorm:"index;not null" json:"business_id" binding:"required"`
	Name       string          `gorm:"size:100;not null" json:"name" binding:"required"`
	Rate       decimal.Decimal `gorm:"type:decimal(20,4);default:0" json:"rate" binding:"required"`
	Taxes      []Tax           `gorm:"many2many:group_taxes" json:"-"`
	IsActive   *bool           `gorm:"not null;default:true" json:"is_active"`
	CreatedAt  time.Time       `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt  time.Time       `gorm:"autoUpdateTime" json:"updated_at"`
}

type NewTaxGroup struct {
	Name  string        `json:"name" binding:"required"`
	Taxes []NewGroupTax `json:"taxes"`
}

type NewGroupTax struct {
	TaxId int `json:"tax_id" binding:"required"`
}

// returns ids of taxes associated with tax group
func (t TaxGroup) TaxIds(ctx context.Context) (ids []int, err error) {
	db := config.GetDB()
	err = db.WithContext(ctx).Table("group_taxes").Where("tax_group_id = ?", t.ID).
		Select("tax_id").Scan(&ids).Error
	return
}

// validate input for both create & update. (id = 0 for create)

func (input *NewTaxGroup) validate(ctx context.Context, businessId string, id int) error {
	if id > 0 {
		if err := utils.ValidateResourceId[TaxGroup](ctx, businessId, id); err != nil {
			return err
		}
	}
	// name
	if err := utils.ValidateUnique[TaxGroup](ctx, businessId, "name", input.Name, id); err != nil {
		return err
	}
	return nil
}

// fetch input taxes from db
func mapTaxesInput(ctx context.Context, businessId string, input []NewGroupTax) ([]Tax, error) {
	tax_ids := make([]int, 0)
	for _, m := range input {
		tax_ids = append(tax_ids, m.TaxId)
	}
	var taxes []Tax
	db := config.GetDB()
	err := db.WithContext(ctx).Where("business_id = ?", businessId).
		Where("id IN ?", tax_ids).Find(&taxes).Error
	if err != nil {
		return nil, err
	}

	return taxes, nil
}

// return taxGroup's rate, Taxes must be preloaded
func (g TaxGroup) CalculateRate() decimal.Decimal {

	// tax group rate = compound rate + sum of non compound rate
	// find compound tax rate, if exists, calculate it with sum of noncompound rates
	// var compoundTaxRate *decimal.Decimal
	// var compoundRate decimal.Decimal
	// var nonCompoundRateSum decimal.Decimal
	compoundTaxRate := decimal.NewFromInt(0)
	compoundRate := decimal.NewFromInt(0)
	nonCompoundRateSum := decimal.NewFromInt(0)

	for _, tax := range g.Taxes {
		if *tax.IsCompoundTax {
			// assigning compoundTaxRate pointer to a temporary variable instead of
			// assigning the address of dynamic loop variable tax, as the value of tax will change in the next round
			compoundTaxRate = compoundTaxRate.Add(tax.Rate)
		} else {
			// nonCompoundRateSum += tax.Rate
			nonCompoundRateSum = nonCompoundRateSum.Add(tax.Rate)
		}
	}

	// calculate compoundRate if compoundTax exists
	if !compoundTaxRate.IsZero() {
		// compoundRate = (100 + nonCompoundRateSum) * (*compoundTaxRate / 100)
		v1 := nonCompoundRateSum.Add(decimal.NewFromInt(100))
		v2 := compoundTaxRate.DivRound(decimal.NewFromInt(100), 4)
		compoundRate = v1.Mul(v2)
	}

	// taxGroupRate := nonCompoundRateSum + compoundRate
	taxGroupRate := nonCompoundRateSum.Add(compoundRate)
	return taxGroupRate
}

// check if tax group's taxes have more than one compound, Taxes have to be preloaded
func (g TaxGroup) ValidateCompoundRule() error {
	compoundCount := 0
	for _, tax := range g.Taxes {
		if *tax.IsCompoundTax {
			compoundCount++
		}
	}
	if compoundCount > 1 {
		return errors.New("tax group cannot have more than one compound tax")
	}
	return nil
}

func CreateTaxGroup(ctx context.Context, input *NewTaxGroup) (*TaxGroup, error) {

	businessId, ok := utils.GetBusinessIdFromContext(ctx)
	if !ok || businessId == "" {
		return nil, errors.New("business id is required")
	}

	if err := input.validate(ctx, businessId, 0); err != nil {
		return nil, err
	}

	// get taxes from db
	taxes, err := mapTaxesInput(ctx, businessId, input.Taxes)
	if err != nil {
		return nil, err
	}
	// construct object to be stored in database
	taxGroup := TaxGroup{
		BusinessId: businessId,
		Name:       input.Name,
		// Rate:       taxGroupRate,
		Taxes: taxes,
	}
	// check if taxes contain more than one compound
	if err := taxGroup.ValidateCompoundRule(); err != nil {
		return nil, err
	}
	taxGroup.Rate = taxGroup.CalculateRate()

	db := config.GetDB()
	// db action
	err = db.WithContext(ctx).Omit("Taxes.*").Create(&taxGroup).Error
	if err != nil {
		return nil, err
	}
	return &taxGroup, nil
}

func UpdateTaxGroup(ctx context.Context, id int, input *NewTaxGroup) (*TaxGroup, error) {
	businessId, ok := utils.GetBusinessIdFromContext(ctx)
	if !ok || businessId == "" {
		return nil, errors.New("business id is required")
	}

	if err := input.validate(ctx, businessId, id); err != nil {
		return nil, err
	}

	// construct taxes from input
	taxes, err := mapTaxesInput(ctx, businessId, input.Taxes)
	if err != nil {
		return nil, err
	}

	// construct object to be updated to db
	taxGroup := TaxGroup{
		ID:         id,
		BusinessId: businessId,
		Name:       input.Name,
		Taxes:      taxes,
	}

	// validate taxes
	if err := taxGroup.ValidateCompoundRule(); err != nil {
		return nil, err
	}
	taxGroup.Rate = taxGroup.CalculateRate()

	db := config.GetDB()
	tx := db.Begin()
	// db action
	// syncing Taxes
	if err := tx.WithContext(ctx).Model(&taxGroup).Association("Taxes").Replace(taxGroup.Taxes); err != nil {
		tx.Rollback()
		return nil, err
	}

	// db action
	err = tx.WithContext(ctx).Model(&taxGroup).Omit("Taxes.*").Updates(map[string]interface{}{
		"Name":  taxGroup.Name,
		"Taxes": taxGroup.Taxes,
		"Rate":  taxGroup.Rate,
	}).Error
	if err != nil {
		tx.Rollback()
		return nil, err
	}

	if err := tx.Commit().Error; err != nil {
		return nil, err
	}

	return &taxGroup, nil
}

func DeleteTaxGroup(ctx context.Context, id int) (*TaxGroup, error) {
	businessId, ok := utils.GetBusinessIdFromContext(ctx)
	if !ok || businessId == "" {
		return nil, errors.New("business id is required")
	}

	db := config.GetDB()
	result, err := utils.FetchModel[TaxGroup](ctx, businessId, id)
	if err != nil {
		return nil, err
	}

	// check if tax group is used
	var count int64
	if err := db.WithContext(ctx).Model(&Product{}).
		Where("sales_tax_type = 'G' AND sales_tax_id = ?", id).
		Or("purchase_tax_type = 'G' AND purchase_tax_id = ?", id).
		Count(&count).Error; err != nil {
		return nil, err
	}
	if count > 0 {
		return nil, errors.New("used by product")
	}
	if err := db.WithContext(ctx).Model(&Customer{}).
		Where("customer_tax_type = 'G' AND customer_tax_id = ?", id).
		Count(&count).Error; err != nil {
		return nil, err
	}
	if count > 0 {
		return nil, errors.New("used by customer")
	}
	if err := db.WithContext(ctx).Model(&Supplier{}).
		Where("supplier_tax_type = 'G' AND supplier_tax_id = ?", id).
		Count(&count).Error; err != nil {
		return nil, err
	}
	if count > 0 {
		return nil, errors.New("used by supplier")
	}

	// db action
	tx := db.Begin()
	if err := tx.WithContext(ctx).Model(&result).Association("Taxes").Clear(); err != nil {
		tx.Rollback()
		return nil, err
	}

	// db action
	err = tx.WithContext(ctx).Omit("Taxes.*").Delete(&result).Error
	if err != nil {
		tx.Rollback()
		return nil, err
	}

	if err := tx.Commit().Error; err != nil {
		return nil, err
	}

	return result, nil
}

func GetTaxGroup(ctx context.Context, id int) (*TaxGroup, error) {
	return GetResource[TaxGroup](ctx, id)
}

func GetTaxGroups(ctx context.Context, name *string) ([]*TaxGroup, error) {
	businessId, ok := utils.GetBusinessIdFromContext(ctx)
	if !ok || businessId == "" {
		return nil, errors.New("business id is required")
	}

	var results []*TaxGroup
	db := config.GetDB()
	// returns mysql syntax error when nested fields
	// fieldNames, err := utils.GetQueryFields(ctx, &TaxGroup{})
	// if err != nil {
	// 	return nil, err
	// }

	dbCtx := db.WithContext(ctx).Where("business_id = ?", businessId)
	if name != nil && *name != "" {
		dbCtx = dbCtx.Where("name LIKE ?", "%"+*name+"%")
	}
	// db query
	err := dbCtx.Preload("Taxes").Order("name").Find(&results).Error
	if err != nil {
		return nil, err
	}

	return results, nil
}

func ToggleActiveTaxGroup(ctx context.Context, id int, isActive bool) (*TaxGroup, error) {
	businessId, ok := utils.GetBusinessIdFromContext(ctx)
	if !ok || businessId == "" {
		return nil, errors.New("business id is required")
	}

	return ToggleActiveModel[TaxGroup](ctx, businessId, id, isActive)
}
