package models

import (
	"context"
	"errors"

	"bitbucket.org/mmdatafocus/books_backend/config"
	"bitbucket.org/mmdatafocus/books_backend/utils"
)

type Township struct {
	ID             int    `gorm:"primary_key" json:"id"`
	Code           string `gorm:"index;size:15;not null" json:"code"`
	StateCode      string `gorm:"index;size:6;not null" json:"state_code"`
	TownshipNameEn string `gorm:"size:50;not null" json:"township_name_en"`
	TownshipNameMm string `gorm:"size:50;not null" json:"township_name_mm"`
	PostalCode     string `gorm:"size:10;not null" json:"postal_code"`
	IsActive       *bool  `gorm:"not null;default:true" json:"is_active"`
}

type NewTownship struct {
	Code           string `json:"code" binding:"required"`
	StateCode      string `json:"state_code" binding:"required"`
	TownshipNameEn string `json:"township_name_en" binding:"required"`
	TownshipNameMm string `json:"township_name_mm" binding:"required"`
	PostalCode     string `json:"postal_code" binding:"required"`
}

type TownshipsEdge Edge[Township]
type TownshipsConnection struct {
	PageInfo *PageInfo        `json:"pageInfo"`
	Edges    []*TownshipsEdge `json:"edges"`
}

// node
// returns decoded curosr string
func (ts Township) GetCursor() string {
	return ts.TownshipNameEn
}

// validate input for both create & update. (id = 0 for create)

func (input *NewTownship) validate(ctx context.Context, businessId string, id int) error {
	// code
	if err := utils.ValidateUnique[Township](ctx, businessId, "code", input.Code, id); err != nil {
		return err
	}
	// postalCode
	if err := utils.ValidateUnique[Township](ctx, businessId, "postal_code", input.PostalCode, id); err != nil {
		return err
	}
	// townshipNameMM
	if err := utils.ValidateUnique[Township](ctx, businessId, "township_name_mm", input.TownshipNameMm, id); err != nil {
		return err
	}
	// townshipNameEn
	if err := utils.ValidateUnique[Township](ctx, businessId, "township_name_en", input.TownshipNameEn, id); err != nil {
		return err
	}
	return nil
}

func CreateTownship(ctx context.Context, input *NewTownship) (*Township, error) {

	if err := input.validate(ctx, "", 0); err != nil {
		return nil, err
	}

	township := Township{
		Code:           input.Code,
		StateCode:      input.StateCode,
		TownshipNameEn: input.TownshipNameEn,
		TownshipNameMm: input.TownshipNameMm,
		PostalCode:     input.PostalCode,
	}

	db := config.GetDB()
	err := db.WithContext(ctx).Create(&township).Error
	if err != nil {
		return nil, err
	}
	return &township, nil
}

func UpdateTownship(ctx context.Context, id int, input *NewTownship) (*Township, error) {

	if err := input.validate(ctx, "", id); err != nil {
		return nil, err
	}

	var township Township
	// db action
	db := config.GetDB()
	if err := db.WithContext(ctx).First(&township, id).Error; err != nil {
		return nil, utils.ErrorRecordNotFound
	}
	err := db.WithContext(ctx).Model(&township).Updates(map[string]interface{}{
		"Code":           input.Code,
		"StateCode":      input.StateCode,
		"TownshipNameEn": input.TownshipNameEn,
		"TownshipNameMm": input.TownshipNameMm,
		"PostalCode":     input.PostalCode,
	}).Error
	if err != nil {
		return nil, err
	}
	return &township, nil
}

func DeleteTownship(ctx context.Context, id int) (*Township, error) {

	db := config.GetDB()
	var result Township

	err := db.WithContext(ctx).First(&result, id).Error
	if err != nil {
		return nil, utils.ErrorRecordNotFound
	}

	// Do not delete if any Business use this township
	var count int64
	err = db.WithContext(ctx).Model(&Business{}).Where("township_id = ?", id).Count(&count).Error
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

func GetTownship(ctx context.Context, id int) (*Township, error) {

	result, err := utils.RetrieveRedis[Township](id)
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
		if err := utils.StoreRedis[Township](result, id); err != nil {
			return nil, err
		}
	}

	return result, nil
}

func GetTownships(ctx context.Context, stateCode *string, code *string, orderby *string) ([]*Township, error) {

	db := config.GetDB()
	var results []*Township

	fieldNames, err := utils.GetQueryFields(ctx, &Township{})
	if err != nil {
		return nil, err
	}

	dbCtx := db.WithContext(ctx)
	if stateCode != nil && len(*stateCode) > 0 {
		dbCtx = dbCtx.Where("state_code = ?", *stateCode)
	}
	if code != nil && len(*code) > 0 {
		dbCtx = dbCtx.Where("code LIKE ?", "%"+*code+"%")
	}
	orderField := "township_name_en"
	if orderby != nil && len(*orderby) > 0 {
		orderField = *orderby
	}
	err = dbCtx.Select(fieldNames).Order(orderField).Find(&results).Error
	if err != nil {
		return nil, err
	}
	return results, nil
}

func ToggleActiveTownship(ctx context.Context, id int, isActive bool) (*Township, error) {
	return ToggleActiveModel[Township](ctx, "", id, isActive)
}

func PaginateTownship(ctx context.Context, limit *int, after *string, country *string, code *string, stateCode *string, postalCode *string, name *string) (*TownshipsConnection, error) {

	db := config.GetDB()
	dbCtx := db.WithContext(ctx).Model(&Township{})

	if country != nil && *country != "" {
		dbCtx.Where("country = ?", *country)
	}
	if code != nil && *code != "" {
		dbCtx.Where("code = ?", *code)
	}
	if stateCode != nil && *stateCode != "" {
		dbCtx.Where("state_code = ?", *stateCode)
	}
	if postalCode != nil && *postalCode != "" {
		dbCtx.Where("postal_code = ?", *postalCode)
	}
	if name != nil && *name != "" {
		dbCtx.Where("township_name_mm LIKE ? OR township_name_en LIKE ?",
			"%"+*name+"%",
			"%"+*name+"%",
		)
	}

	edges, pageInfo, err := FetchPagePureCursor[Township](dbCtx, *limit, after, "township_name_en", ">")
	if err != nil {
		return nil, err
	}
	var townshipsConnection TownshipsConnection
	townshipsConnection.PageInfo = pageInfo
	for _, edge := range edges {
		townshipEdge := TownshipsEdge(edge)
		townshipsConnection.Edges = append(townshipsConnection.Edges, &townshipEdge)
	}
	return &townshipsConnection, err
}
