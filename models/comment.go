package models

import (
	"context"
	"errors"
	"time"

	"github.com/mmdatafocus/books_backend/config"
	"github.com/mmdatafocus/books_backend/utils"
)

type Comment struct {
	ID            int       `gorm:"primary_key" json:"id"`
	BusinessId    string    `gorm:"index;not null" json:"business_id"`
	Description   string    `gorm:"type:text;not null" json:"description" binding:"required"`
	ReferenceID   int       `gorm:"index" json:"reference_id"`
	ReferenceType string    `gorm:"size:255" json:"reference_type"`
	UserId        int       `gorm:"index;not null" json:"user_id"`
	UserName      string    `gorm:"size:100" json:"user_name"`
	CreatedAt     time.Time `gorm:"autoCreateTime" json:"created_at"`
}

type NewComment struct {
	Description   string `json:"description" binding:"required"`
	ReferenceID   int    `json:"reference_id"`
	ReferenceType string `json:"reference_type"`
}

func CreateComment(ctx context.Context, input *NewComment) (*Comment, error) {

	db := config.GetDB()
	businessId, ok := utils.GetBusinessIdFromContext(ctx)
	if !ok || businessId == "" {
		return nil, errors.New("business id is required")
	}

	userId, ok := utils.GetUserIdFromContext(ctx)
	if !ok || userId <= 0 {
		return nil, errors.New("user id is required")
	}
	userName, ok := utils.GetUserNameFromContext(ctx)
	if !ok || userName == "" {
		return nil, errors.New("user name is required")
	}

	comment := Comment{
		BusinessId:    businessId,
		Description:   input.Description,
		ReferenceID:   input.ReferenceID,
		ReferenceType: input.ReferenceType,
		UserId:        userId,
		UserName:      userName,
	}

	err := db.WithContext(ctx).Create(&comment).Error
	if err != nil {
		return nil, err
	}
	return &comment, nil
}

func DeleteComment(ctx context.Context, id int) (*Comment, error) {

	db := config.GetDB()

	businessId, ok := utils.GetBusinessIdFromContext(ctx)
	if !ok || businessId == "" {
		return nil, errors.New("business id is required")
	}

	result, err := utils.FetchModel[Comment](ctx, businessId, id)
	if err != nil {
		return nil, utils.ErrorRecordNotFound
	}

	err = db.WithContext(ctx).Delete(&result).Error
	if err != nil {
		return nil, err
	}
	return result, nil
}

func GetComment(ctx context.Context, id int) (*Comment, error) {

	// db := config.GetDB()
	// var result Comment
	businessId, ok := utils.GetBusinessIdFromContext(ctx)
	if !ok || businessId == "" {
		return nil, errors.New("business id is required")
	}
	return utils.FetchModel[Comment](ctx, businessId, id)
}

func GetComments(ctx context.Context, referenceId *int, referenceType *string, userId *int) ([]*Comment, error) {

	db := config.GetDB()
	var results []*Comment

	fieldNames, err := utils.GetQueryFields(ctx, &Comment{})
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
