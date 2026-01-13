package models

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"bitbucket.org/mmdatafocus/books_backend/config"
	"bitbucket.org/mmdatafocus/books_backend/utils"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

type History struct {
	ID            int       `gorm:"primary_key" json:"id"`
	BusinessId    string    `gorm:"index;not null" json:"business_id"`
	ActionType    string    `gorm:"size:10;not null" json:"action_type" binding:"required"`
	Before        string    `gorm:"type:text" json:"before"`
	After         string    `gorm:"type:text" json:"after"`
	Description   string    `gorm:"type:text;not null" json:"description"`
	ReferenceID   int       `gorm:"index" json:"reference_id"`
	ReferenceType string    `gorm:"size:255" json:"reference_type"`
	UserId        int       `gorm:"index;not null" json:"user_id"`
	UserName      string    `gorm:"size:100" json:"user_name"`
	CreatedAt     time.Time `gorm:"autoCreateTime" json:"created_at"`
}

type NewHistory struct {
	BusinessId    string `json:"business_id" binding:"required"`
	ActionType    string `json:"action_type" binding:"required"`
	Before        string `json:"before"`
	After         string `json:"after"`
	Description   string `json:"description"`
	ReferenceID   int    `json:"reference_id"`
	ReferenceType string `json:"reference_type"`
	UserId        int    `json:"user_id"`
	UserName      string `json:"user_name"`
}

func describeTotalAmountCreated(ctx context.Context, typename string, currencyId int, totalAmount decimal.Decimal) (string, error) {

	currency, err := GetResource[Currency](ctx, currencyId)
	if err != nil {
		return "", err
	}
	description := fmt.Sprintf("%s created for %s.%v.", typename, currency.Symbol, totalAmount)
	return description, nil
}

func createHistory(tx *gorm.DB,
	actionType string,
	referenceId int,
	referenceType string,
	before interface{},
	after interface{},
	description string) (err error) {

	var history History

	b, _ := json.Marshal(before)
	a, _ := json.Marshal(after)

	ctx := tx.Statement.Context
	// get businessId, userId, userName from context
	businessId, ok := utils.GetBusinessIdFromContext(ctx)
	if !ok || businessId == "" {
		return errors.New("business id is required")
	}
	userId, ok := utils.GetUserIdFromContext(ctx)
	if !ok {
		return errors.New("user id is required")
	}
	userName, ok := utils.GetUserNameFromContext(ctx)
	if !ok {
		return errors.New("user id is required")
	}

	history.BusinessId = businessId
	history.ActionType = actionType
	history.Before = string(b)
	history.After = string(a)
	history.Description = description
	history.ReferenceID = referenceId
	history.ReferenceType = referenceType
	history.UserId = userId
	history.UserName = userName

	//err = tx.WithContext(ctx).Create(&history).Error
	err = tx.Create(&history).Error
	// fmt.Printf("%#v\n", history)
	return err
}

// func createHistoryOLD(tx *gorm.DB,
// 	actionType string,
// 	id int,
// 	before interface{}, after interface{},
// 	description string) (err error) {

// 	var history History
// 	// get context from *gorm.DB
// 	ctx := tx.Statement.Context

// 	b, _ := json.Marshal(before)
// 	a, _ := json.Marshal(after)

// 	businessId, ok := utils.GetBusinessIdFromContext(ctx)
// 	if !ok || businessId == "" {
// 		return errors.New("business id is required")
// 	}
// 	userId, ok := utils.GetUserIdFromContext(ctx)
// 	if !ok {
// 		return errors.New("user id is required")
// 	}
// 	userName, ok := utils.GetUserNameFromContext(ctx)
// 	if !ok {
// 		return errors.New("user id is required")
// 	}

// 	history.BusinessId = businessId
// 	history.ActionType = actionType
// 	history.Before = string(b)
// 	history.After = string(a)
// 	history.Description = description
// 	history.ReferenceID = id
// 	history.ReferenceType = tx.Statement.Table
// 	history.UserId = userId
// 	history.UserName = userName

// 	//err = tx.WithContext(ctx).Create(&history).Error
// 	err = tx.Create(&history).Error
// 	// fmt.Printf("%#v\n", history)
// 	return err
// }

func SaveHistoryCreate(tx *gorm.DB, id int, obj interface{}, description string) error {
	return createHistory(tx, "CREATE", id, tx.Statement.Table, nil, obj, description)
}

// func SaveHistoryUpdateOLD[T any](tx *gorm.DB, id int, obj *T, desc ...string) error {
// 	// fields := tx.Statement.Dest.(map[string]interface{})
// 	// fmt.Printf("%#v\n", fields)
// 	description := "Updated " + utils.GetTypeName[T]()
// 	if len(description) > 0 {
// 		description = desc[0]
// 	}

// 	if tx.Statement.Changed("IsActive") {
// 		var actionType string
// 		// get IsActive value of updated object
// 		if (tx.Statement.Dest.(map[string]interface{})["IsActive"]).(bool) {
// 			actionType = "*ACTIVE*"
// 		} else {
// 			actionType = "*INACTIVE*"
// 		}
// 		return createHistoryOLD(tx, actionType, id, nil, nil, "Toogled")
// 	}

// 	// find in redis first
// 	var old T
// 	cached, _ := utils.RetrieveRedis[T](id)

// 	// if not found in redis
// 	if cached == nil {
// 		// fetch from db
// 		if err := tx.First(&old, id).Error; err != nil {
// 			return err
// 		}
// 	} else {
// 		old = *cached
// 	}

// 	return createHistoryOLD(tx, "UPDATE", id, &old, obj, "")
// }

func SaveHistoryUpdate(tx *gorm.DB, id int, currentValue interface{}, description string) error {

	var newValue = tx.Statement.Dest

	return createHistory(tx, "UPDATE", id, tx.Statement.Table, currentValue, newValue, description)
}

// func SaveHistoryUpdate(tx *gorm.DB, id int, currentValue interface{}, description string) error {
// 	// Convert currentValue to map
// 	currentMap, err := structToMap(currentValue)
// 	if err != nil {
// 		return err
// 	}

// 	// Convert tx.Statement.Dest to map
// 	destMap, err := structToMap(tx.Statement.Dest)
// 	if err != nil {
// 		return err
// 	}

// 	return createHistory(tx, "UPDATE", id, tx.Statement.Table, currentMap, destMap, description)
// }

// func structToMap(obj interface{}) (map[string]interface{}, error) {
// 	data := make(map[string]interface{})
// 	val := reflect.ValueOf(obj).Elem()
// 	typ := val.Type()

// 	for i := 0; i < val.NumField(); i++ {
// 		field := val.Field(i)
// 		fieldName := typ.Field(i).Name
// 		data[fieldName] = field.Interface()
// 	}

// 	return data, nil
// }

// func SaveHitoryToggle(ctx context.Context, tx *gorm.DB, id int, isActive bool, referenceType string, descripition string) error {
// func SaveHitoryToggle(ctx context.Context, tx *gorm.DB, id int, isActive bool, , descripition string) error {
// 	var actionType string
// 	if isActive {
// 		actionType = "*ACTIVE*"
// 	} else {
// 		actionType = "*INACTIVE*"
// 	}
// 	return createHistory(tx, actionType, id, nil, nil, descripition)
// }

func SaveHistoryDelete(tx *gorm.DB, id int, obj interface{}, description string) error {
	return createHistory(tx, "DELETE", id, tx.Statement.Table, obj, nil, description)
}

type HistoriesConnection struct {
	Edges    []*HistoriesEdge `json:"edges"`
	PageInfo *PageInfo        `json:"pageInfo"`
}

type HistoriesEdge Edge[History]

func (obj History) GetId() int {
	return obj.ID
}

func (h History) GetCursor() string {
	return h.CreatedAt.String()
}

func CreateManualHistory(ctx context.Context, input *NewHistory) (*History, error) {
	db := config.GetDB()

	history := History{
		BusinessId:    input.BusinessId,
		ActionType:    input.ActionType,
		Before:        input.Before,
		After:         input.After,
		Description:   input.Description,
		ReferenceID:   input.ReferenceID,
		ReferenceType: input.ReferenceType,
		UserId:        input.UserId,
		UserName:      input.UserName,
	}

	err := db.WithContext(ctx).Create(&history).Error
	if err != nil {
		return nil, err
	}
	return &history, nil
}

func DeleteHistory(ctx context.Context, id int) (*History, error) {

	db := config.GetDB()
	var result History

	err := db.WithContext(ctx).First(&result, id).Error
	if err != nil {
		return nil, utils.ErrorRecordNotFound
	}

	err = db.WithContext(ctx).Delete(&result).Error
	if err != nil {
		return nil, err
	}
	return &result, nil
}

func GetHistory(ctx context.Context, id int) (*History, error) {

	db := config.GetDB()
	var result History

	fieldNames, err := utils.GetQueryFields(ctx, &History{})
	if err != nil {
		return nil, err
	}

	err = db.WithContext(ctx).Select(fieldNames).First(&result, id).Error
	if err != nil {
		return nil, utils.ErrorRecordNotFound
	}
	return &result, nil
}

func GetHistories(ctx context.Context, referenceId *int, referenceType *string, userId *int) ([]*History, error) {

	db := config.GetDB()
	var results []*History

	fieldNames, err := utils.GetQueryFields(ctx, &History{})
	if err != nil {
		return nil, err
	}

	businessId, ok := utils.GetBusinessIdFromContext(ctx)
	if !ok || businessId == "" {
		return nil, errors.New("business id is required")
	}

	dbCtx := db.WithContext(ctx).Where("business_id = ?", businessId)
	if referenceId != nil && *referenceId > 0 {
		dbCtx = dbCtx.Where("reference_id = ?", referenceId)
	}
	if referenceType != nil && len(*referenceType) > 0 {
		dbCtx = dbCtx.Where("reference_type = ?", referenceType)
	}
	if userId != nil && *userId > 0 {
		dbCtx = dbCtx.Where("user_id = ?", userId)
	}
	err = dbCtx.Select(fieldNames).Order("created_at DESC").Find(&results).Error
	if err != nil {
		return nil, err
	}
	return results, nil
}
func PaginateHistory(ctx context.Context,
	limit *int,
	after *string,
	referenceType *string,
	referenceID *int,
	userID *int,
	actionType *string,
) (*HistoriesConnection, error) {
	businessId, ok := utils.GetBusinessIdFromContext(ctx)
	if !ok || businessId == "" {
		return nil, errors.New("business id is required")
	}

	db := config.GetDB()
	dbCtx := db.WithContext(ctx).Where("business_id = ?", businessId)
	if referenceType != nil && *referenceType != "" {
		dbCtx.Where("reference_type = ?", *referenceType)
	}
	if referenceID != nil && *referenceID > 0 {
		dbCtx.Where("reference_id = ?", *referenceID)
	}
	if userID != nil && *userID > 0 {
		dbCtx.Where("user_id = ?", *userID)
	}
	if actionType != nil && *actionType != "" {
		dbCtx.Where("action_type = ?", *actionType)
	}

	edges, pageInfo, err := FetchPageCompositeCursor[History](dbCtx, *limit, after, "created_at", "<")
	if err != nil {
		return nil, err
	}
	var historysConnection HistoriesConnection
	historysConnection.PageInfo = pageInfo
	for _, edge := range edges {
		historysEdge := HistoriesEdge(edge)
		historysConnection.Edges = append(historysConnection.Edges, &historysEdge)
	}

	return &historysConnection, err
}
