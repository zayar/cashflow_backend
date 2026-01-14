package models

import (
	"context"
	"errors"
	"html"
	"strings"

	"github.com/mmdatafocus/books_backend/config"
	"github.com/mmdatafocus/books_backend/utils"
)

type NewUserAccount struct {
	Username string `json:"username"`
	Name     string `json:"name"`
	Email    string `json:"email,omitempty"`
	Phone    string `json:"phone,omitempty"`
	Mobile   string `json:"mobile,omitempty"`
	ImageUrl string `json:"imageUrl,omitempty"`
	// IsActive *bool  `json:"isActive"`
	Password string `json:"password"`
	RoleId   *int   `json:"roleId,omitempty"`
	Branches string `json:"branches,omitempty"`
}

type UserAccount struct {
	ID       int    `json:"id"`
	Username string `json:"username"`
	Name     string `json:"name"`
	Email    string `json:"email,omitempty"`
	Phone    string `json:"phone,omitempty"`
	Mobile   string `json:"mobile,omitempty"`
	ImageUrl string `json:"imageUrl,omitempty"`
	IsActive *bool  `json:"isActive"`
	RoleId   *int   `json:"roleId,omitempty"`
	Branches string `json:"branches,omitempty"`
}

func (user User) UserAccount() *UserAccount {
	return &UserAccount{
		ID:       user.ID,
		Username: user.Username,
		Name:     user.Name,
		Email:    utils.DereferencePtr(user.Email),
		Phone:    user.Phone,
		Mobile:   user.Mobile,
		ImageUrl: user.ImageUrl,
		IsActive: user.IsActive,
		RoleId:   &user.RoleId,
		Branches: user.Branches,
	}
}

// validate input as well as sanitzing it
func (input *NewUserAccount) validate(ctx context.Context, businessId string, id int) error {

	input.Username = html.EscapeString(strings.TrimSpace(input.Username))
	input.Email = strings.ToLower(input.Email)

	db := config.GetDB()
	if input.Email != "" && !utils.IsValidEmail(input.Email) {
		return errors.New("invalid email address")
	}
	//? validate phone, mobile

	if input.RoleId != nil {
		if err := utils.ValidateResourceId[Role](ctx, businessId, input.RoleId); err != nil {
			return errors.New("invalid role id")
		}
	}

	var count int64
	dbCtx := db.WithContext(ctx).Model(&User{}).Where("business_id = ?", businessId)
	if input.Email != "" {
		dbCtx.Where("username = ? OR email = ?", input.Username, input.Email)
	} else {
		dbCtx.Where("username = ?", input.Username)
	}

	if id > 0 {
		dbCtx.Not("id = ?", id)
	}

	if err := dbCtx.Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return errors.New("duplicate username or email")
	}

	return nil
}

func CreateUserAccount(ctx context.Context, input NewUserAccount) (*UserAccount, error) {
	businessId, ok := utils.GetBusinessIdFromContext(ctx)
	if !ok || businessId == "" {
		return nil, errors.New("business id is required")
	}

	if err := input.validate(ctx, businessId, 0); err != nil {
		return nil, err
	}

	db := config.GetDB()

	hashedPassword, err := utils.HashPassword(input.Password)
	if err != nil {
		return nil, err
	}

	user := User{
		Username:   input.Username,
		BusinessId: businessId,
		Name:       input.Name,
		Email:      utils.NilIfEmpty(input.Email),
		Phone:      input.Phone,
		Mobile:     input.Mobile,
		ImageUrl:   input.ImageUrl,
		Password:   string(hashedPassword),
		IsActive:   utils.NewTrue(),
		Role:       UserRoleCustom,
		RoleId:     utils.DereferencePtr(input.RoleId),
		Branches:   input.Branches,
	}

	tx := db.Begin()
	err = tx.WithContext(ctx).Create(&user).Error
	if err != nil {
		tx.Rollback()
		return nil, err
	}
	if err := user.RemoveAllRedis(); err != nil {
		tx.Rollback()
		return nil, err
	}
	// user.Password = ""
	return user.UserAccount(), tx.Commit().Error
}

func UpdateUserAccount(ctx context.Context, id int, input NewUserAccount) (*UserAccount, error) {
	businessId, ok := utils.GetBusinessIdFromContext(ctx)
	if !ok || businessId == "" {
		return nil, errors.New("business id is required")
	}
	if err := input.validate(ctx, businessId, id); err != nil {
		return nil, err
	}
	user, err := utils.FetchModel[User](ctx, businessId, id)
	if err != nil {
		return nil, err
	}
	db := config.GetDB()
	tx := db.Begin()
	if err := tx.WithContext(ctx).Model(&user).Updates(map[string]interface{}{
		"Username": input.Username,
		"Name":     input.Name,
		"Email":    input.Email,
		"Phone":    input.Phone,
		"Mobile":   input.Mobile,
		"ImageUrl": input.ImageUrl,
		"RoleId":   utils.DereferencePtr(input.RoleId),
		"Branches": input.Branches,
	}).Error; err != nil {
		tx.Rollback()
		return nil, err
	}
	if err := RemoveRedisBoth(user); err != nil {
		tx.Rollback()
		return nil, err
	}
	return user.UserAccount(), tx.Commit().Error
}

func DeleteUserAccount(ctx context.Context, userId int) (*UserAccount, error) {
	businessId, ok := utils.GetBusinessIdFromContext(ctx)
	if !ok || businessId == "" {
		return nil, errors.New("business id is required")
	}

	user, err := utils.FetchModel[User](ctx, businessId, userId)
	if err != nil {
		return nil, err
	}
	db := config.GetDB()
	tx := db.Begin()
	if err := tx.WithContext(ctx).Delete(&user).Error; err != nil {
		tx.Rollback()
		return nil, err
	}
	if err := RemoveRedisBoth(user); err != nil {
		tx.Rollback()
		return nil, err
	}
	if err := user.DestroyAllSessions(ctx); err != nil {
		tx.Rollback()
		return nil, err
	}
	return user.UserAccount(), tx.Commit().Error
}

func ListUserAccount(ctx context.Context) ([]*UserAccount, error) {
	userAccounts, err := ListAllResource[User, UserAccount](ctx)
	if err != nil {
		return nil, err
	}

	return userAccounts, nil
}

func ToggleActiveUserAccount(ctx context.Context, userId int, isActive bool) (*UserAccount, error) {
	businessId, ok := utils.GetBusinessIdFromContext(ctx)
	if !ok || businessId == "" {
		return nil, errors.New("business id is required")
	}

	user, err := ToggleActiveModel[User](ctx, businessId, userId, isActive)
	if err != nil {
		return nil, err
	}

	return user.UserAccount(), nil
}

func GetUserAccount(ctx context.Context, userId int) (*UserAccount, error) {
	businessId, ok := utils.GetBusinessIdFromContext(ctx)
	if !ok || businessId == "" {
		return nil, errors.New("business id is required")
	}

	user, err := utils.FetchModel[User](ctx, businessId, userId)
	if err != nil {
		return nil, err
	}
	return user.UserAccount(), nil
}
