package models

import (
	"context"
	"errors"

	"github.com/mmdatafocus/books_backend/config"
	"github.com/mmdatafocus/books_backend/utils"
)

type State struct {
	ID          int    `gorm:"primary_key" json:"id"`
	Country     string `gorm:"index;size:50;not null" json:"country"`
	Code        string `gorm:"index;size:6;not null" json:"code"`
	StateNameEn string `gorm:"size:50;not null" json:"state_name_en"`
	StateNameMm string `gorm:"size:50;not null" json:"state_name_mm"`
	PostalCode  string `gorm:"size:10;not null" json:"postal_code"`
	IsActive    *bool  `gorm:"not null;default:true" json:"is_active"`
}

type NewState struct {
	Country     string `json:"country" binding:"required"`
	Code        string `json:"code" binding:"required"`
	StateNameEn string `json:"state_name_en" binding:"required"`
	StateNameMm string `json:"state_name_mm" binding:"required"`
	PostalCode  string `json:"postal_code" binding:"required"`
}

type StatesEdge Edge[State]
type StatesConnection struct {
	PageInfo *PageInfo     `json:"pageInfo"`
	Edges    []*StatesEdge `json:"edges"`
}

// node
// returns decoded curosr string
func (st State) GetCursor() string {
	return st.StateNameEn
}

// validate input for both create & update. (id = 0 for create)

func (input *NewState) validate(ctx context.Context, businessId string, id int) error {
	// code
	if err := utils.ValidateUnique[State](ctx, businessId, "code", input.Code, id); err != nil {
		return err
	}
	// postalcode
	if err := utils.ValidateUnique[State](ctx, businessId, "state_name_mm", input.StateNameMm, id); err != nil {
		return err
	}
	// stateNameMM
	if err := utils.ValidateUnique[State](ctx, businessId, "state_name_mm", input.StateNameMm, id); err != nil {
		return err
	}
	// stateNameEn
	if err := utils.ValidateUnique[State](ctx, businessId, "state_name_en", input.StateNameEn, id); err != nil {
		return err
	}
	return nil
}

func CreateState(ctx context.Context, input *NewState) (*State, error) {

	if err := input.validate(ctx, "", 0); err != nil {
		return nil, err
	}
	state := State{
		Country:     input.Country,
		Code:        input.Code,
		StateNameEn: input.StateNameEn,
		StateNameMm: input.StateNameMm,
	}

	// db action
	db := config.GetDB()
	err := db.WithContext(ctx).Create(&state).Error
	if err != nil {
		return nil, err
	}

	return &state, nil
}

func UpdateState(ctx context.Context, id int, input *NewState) (*State, error) {

	if err := input.validate(ctx, "", id); err != nil {
		return nil, err
	}

	// db action
	db := config.GetDB()
	var state State
	if err := db.WithContext(ctx).First(&state, id).Error; err != nil {
		return nil, err
	}
	err := db.WithContext(ctx).Model(&state).Updates(map[string]interface{}{
		"Country":     input.Country,
		"Code":        input.Code,
		"StateNameEn": input.StateNameEn,
		"StateNameMm": input.StateNameMm,
	}).Error
	if err != nil {
		return nil, err
	}

	return &state, nil
}

func DeleteState(ctx context.Context, id int) (*State, error) {

	db := config.GetDB()
	var result State

	err := db.WithContext(ctx).First(&result, id).Error
	if err != nil {
		return nil, utils.ErrorRecordNotFound
	}

	// Do not delete if any Business use this state
	var count int64
	err = db.WithContext(ctx).Model(&Business{}).Where("state_id = ?", id).Count(&count).Error
	if err != nil {
		return nil, err
	}
	if count > 0 {
		return nil, errors.New("used by business")
	}

	// db action
	err = db.WithContext(ctx).Delete(&result).Error
	if err != nil {
		return nil, err
	}
	return &result, nil
}

func GetState(ctx context.Context, id int) (*State, error) {

	result, err := utils.RetrieveRedis[State](id)
	if err != nil {
		return nil, err
	}

	if result == nil {
		db := config.GetDB()
		// db query
		err := db.WithContext(ctx).First(&result, id).Error
		if err != nil {
			return nil, utils.ErrorRecordNotFound
		}
		// caching
		if err := utils.StoreRedis[State](result, id); err != nil {
			return nil, err
		}
	}
	return result, nil
}

func GetStates(ctx context.Context, country *string, code *string, orderby *string) ([]*State, error) {

	db := config.GetDB()
	var results []*State

	fieldNames, err := utils.GetQueryFields(ctx, &State{})
	if err != nil {
		return nil, err
	}

	dbCtx := db.WithContext(ctx)
	if country != nil && len(*country) > 0 {
		dbCtx = dbCtx.Where("country = ?", *country)
	}
	if code != nil && len(*code) > 0 {
		dbCtx = dbCtx.Where("code LIKE ?", "%"+*code+"%")
	}
	orderField := "state_name_en"
	if orderby != nil && len(*orderby) > 0 {
		orderField = *orderby
	}
	err = dbCtx.Select(fieldNames).Order(orderField).Find(&results).Error
	if err != nil {
		return nil, err
	}
	return results, nil
}

func ToggleActiveState(ctx context.Context, id int, isActive bool) (*State, error) {
	return ToggleActiveModel[State](ctx, "", id, isActive)
}

func PaginateState(ctx context.Context, limit *int, after *string, country *string, code *string, postalCode *string, name *string) (*StatesConnection, error) {
	db := config.GetDB()
	dbCtx := db.WithContext(ctx).Model(&State{})
	if country != nil && *country != "" {
		dbCtx.Where("country = ?", *country)
	}
	if code != nil && *code != "" {
		dbCtx.Where("code = ?", *code)
	}
	if postalCode != nil && *postalCode != "" {
		dbCtx.Where("postalCode = ?", *postalCode)
	}
	if name != nil && *name != "" {
		dbCtx.Where("state_name_mm LIKE ? OR state_name_en LIKE ?",
			"%"+*name+"%",
			"%"+*name+"%",
		)
	}

	edges, pageInfo, err := FetchPagePureCursor[State](dbCtx, *limit, after, "state_name_en", ">")
	if err != nil {
		return nil, err
	}
	var statesConnection StatesConnection
	statesConnection.PageInfo = pageInfo
	for _, edge := range edges {
		stateEdge := StatesEdge(edge)
		statesConnection.Edges = append(statesConnection.Edges, &stateEdge)
	}
	return &statesConnection, err
}
