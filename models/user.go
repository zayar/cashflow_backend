package models

import (
	"context"
	"errors"
	"fmt"
	"html"
	"os"
	"strconv"
	"strings"
	"time"

	"bitbucket.org/mmdatafocus/books_backend/config"
	"bitbucket.org/mmdatafocus/books_backend/utils"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

type User struct {
	ID         int       `gorm:"primary_key" json:"id"`
	BusinessId string    `gorm:"index" json:"business_id"`
	Username   string    `gorm:"size:100;not null;unique" json:"username" binding:"required"`
	Name       string    `gorm:"size:100;not null" json:"name" binding:"required"`
	Email      *string   `gorm:"size:100;unique" json:"email"`
	Phone      string    `gorm:"size:20" json:"phone"`
	Mobile     string    `gorm:"size:20" json:"mobile"`
	ImageUrl   string    `json:"image_url"`
	Password   string    `gorm:"size:255;not null" json:"password"`
	IsActive   *bool     `gorm:"not null" json:"is_active"`
	RoleId     int       `gorm:"not null;default:0" json:"role_id" binding:"required"`
	Role       UserRole  `gorm:"type:enum('A', 'O', 'C');default:C" json:"role"`
	Branches   string    `json:"branches"`
	CreatedAt  time.Time `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt  time.Time `gorm:"autoUpdateTime" json:"updated_at"`
}

type NewUser struct {
	BusinessId string   `json:"business_id"`
	Username   string   `json:"username" binding:"required"`
	Name       string   `binding:"required"`
	Email      string   `json:"email"`
	Phone      string   `json:"phone"`
	Mobile     string   `json:"mobile"`
	ImageUrl   string   `json:"image_url"`
	Password   string   `json:"password" binding:"required"`
	IsActive   *bool    `json:"is_active" binding:"required"`
	RoleId     int      `json:"role_id" binding:"required"`
	Role       UserRole `json:"role" binding:"required"`
	Branches   string   `json:"branches"`
}

/*
caches:
	User:$username
	UserAccountList:$businessId
*/

func (user User) RemoveInstanceRedis() error {
	if err := config.RemoveRedisKey("User:" + user.Username); err != nil {
		return err
	}
	return nil
}

func (user User) RemoveAllRedis() error {
	if err := config.RemoveRedisKey("UserAccountList:" + user.BusinessId); err != nil {
		return err
	}
	return nil
}

type LoginInfo struct {
	Token            string          `json:"token"`
	Name             string          `json:"name"`
	Role             string          `json:"role"`
	Modules          []AllowedModule `json:"modules"`
	BusinessName     string          `json:"business_name"`
	BaseCurrencyId   int             `json:"base_currency_id"`
	BaseCurrencyName string          `json:"base_currency_name"`
	FiscalYear       FiscalYear      `json:"fiscal_year"`
	Timezone         string          `json:"timezone"`
}

type AllowedModule struct {
	ModuleName     string `json:"module_name"`
	AllowedActions string `json:"allowed_actions"`
}

func (result *User) PrepareGive() {
	result.Password = ""
}

// destroy current session
func Logout(ctx context.Context) (bool, error) {
	token, ok := utils.GetTokenFromContext(ctx)
	if !ok || token == "" {
		return false, errors.New("token is required")
	}
	err := config.RemoveRedisKey("Token:" + fmt.Sprint(token))
	if err != nil {
		return false, nil
	}
	// remove current token from tokens list
	username, ok := utils.GetUsernameFromContext(ctx)
	if !ok || username == "" {
		return false, errors.New("user not found")
	}
	if err := config.RemoveRedisSetMember("Tokens:"+username, token); err != nil {
		return false, err
	}
	return true, nil
}

func ClearRedis(ctx context.Context) (string, error) {
	// token, ok := utils.GetTokenFromContext(ctx)
	// if !ok || token == "" {
	// 	return false, errors.New("token is required")
	// }
	err := config.ClearRedis(ctx)
	if err != nil {
		return "Failed to clear redis", nil
	}
	return "OK", nil
}

func Login(ctx context.Context, username string, password string) (*LoginInfo, error) {

	db := config.GetDB()
	var err error
	var result LoginInfo

	user := User{}

	// get User info
	exists, err := config.GetRedisObject("User:"+username, &user)
	if err != nil {
		return &result, err
	}
	if !exists {
		err = db.WithContext(ctx).Model(&User{}).Where("username = ?", username).Take(&user).Error

		if err != nil {
			return &result, errors.New("invalid username or password")
		}
	}

	// check login credentials
	err = utils.ComparePassword(user.Password, password)

	if err != nil && err == bcrypt.ErrMismatchedHashAndPassword {
		return &result, errors.New("invalid username or password")
	}

	isActive := *user.IsActive
	if !isActive {
		return &result, errors.New("user is disabled")
	}

	// generate token & response
	token := uuid.New()
	result.Token = token.String()
	result.Name = user.Username
	if user.RoleId == 0 {
		result.Role = "Admin"
	} else {
		var userRole Role
		if err := db.WithContext(ctx).Model(&Role{}).
			Preload("RoleModules").Preload("RoleModules.Module").
			Where("id = ?", user.RoleId).First(&userRole, user.RoleId).Error; err != nil {
			return nil, err
		}
		result.Role = userRole.Name
		var allowedModules []AllowedModule
		for _, rm := range userRole.RoleModules {
			allowedModules = append(allowedModules, AllowedModule{
				ModuleName:     rm.Module.Name,
				AllowedActions: rm.AllowedActions,
			})
		}
		result.Modules = allowedModules

		var business Business
		if err := db.WithContext(ctx).Model(&Business{}).Where("id = ?", user.BusinessId).First(&business).Error; err != nil {
			return nil, err
		}
		var currency Currency
		if err := db.WithContext(ctx).Model(&Currency{}).Where("id = ?", business.BaseCurrencyId).First(&currency).Error; err != nil {
			return nil, err
		}
		result.BusinessName = business.Name
		result.BaseCurrencyId = business.BaseCurrencyId
		result.BaseCurrencyName = currency.Name
		result.FiscalYear = business.FiscalYear
		result.Timezone = business.Timezone
	}
	// if err = db.WithContext(ctx).Preload("RoleModules").Preload("RoleModules.Module").Where("id = ?", roleId).First(&role).Error; err != nil {
	// 	return
	// }

	// store token in redis
	token_lifespan, err := strconv.Atoi(os.Getenv("TOKEN_HOUR_LIFESPAN"))
	if err != nil {
		return &result, err
	}

	// add new token to the user's tokens set
	if err := config.AddRedisSet("Tokens:"+user.Username, token.String()); err != nil {
		return nil, err
	}
	if err := config.SetRedisValue("Token:"+token.String(), user.Username, time.Duration(token_lifespan)*time.Hour); err != nil {
		return &result, err
	}
	// if !exists {
	// 	if err := config.SetRedisObject("User:"+user.Username, &user, time.Duration(token_lifespan)*time.Hour); err != nil {
	// 		return &result, err
	// 	}
	// }

	return &result, nil
}

func GetAllUsers(ctx context.Context) ([]*User, error) {

	db := config.GetDB()
	var results []*User

	if err := db.WithContext(ctx).Find(&results).Error; err != nil {
		return results, errors.New("no user")
	}

	for i, u := range results {
		u.Password = ""
		results[i] = u
	}

	return results, nil
}

func CreateUser(ctx context.Context, input *NewUser) (*User, error) {

	db := config.GetDB()
	var count int64

	if input.Email != "" && !utils.IsValidEmail(input.Email) {
		return &User{}, errors.New("invalid email address")
	}

	err := db.WithContext(ctx).Model(&User{}).Where("username = ?", input.Username).Or("email = ?", input.Email).Count(&count).Error
	if err != nil {
		return &User{}, err
	}
	if count > 0 {
		return &User{}, errors.New("duplicate username or email")
	}

	hashedPassword, err := utils.HashPassword(input.Password)
	if err != nil {
		return &User{}, err
	}
	input.Email = strings.ToLower(input.Email)

	user := User{
		Username:   html.EscapeString(strings.TrimSpace(input.Username)),
		BusinessId: input.BusinessId,
		Name:       input.Name,
		Email:      utils.NilIfEmpty(input.Email),
		Phone:      input.Phone,
		Mobile:     input.Mobile,
		ImageUrl:   input.ImageUrl,
		Password:   string(hashedPassword),
		IsActive:   input.IsActive,
		Role:       input.Role,
		RoleId:     input.RoleId,
		Branches:   input.Branches,
	}

	err = db.WithContext(ctx).Create(&user).Error
	if err != nil {
		return &User{}, err
	}
	user.Password = ""
	return &user, nil
}

func GetUser(ctx context.Context, id int) (*User, error) {

	db := config.GetDB()
	var result User

	err := db.WithContext(ctx).First(&result, id).Error

	if err != nil {
		return &result, utils.ErrorRecordNotFound
	}

	result.PrepareGive()

	return &result, nil
}

func (input *User) UpdateUser(id int) (*User, error) {

	db := config.GetDB()
	var count int64

	err := db.Model(&User{}).Where("id = ?", id).Count(&count).Error
	if err != nil {
		return &User{}, err
	}
	if count <= 0 {
		return nil, utils.ErrorRecordNotFound
	}

	if err = db.Model(&User{}).
		Where("username = ? OR email = ?", input.Username, input.Email).
		Not("id = ?", id).
		Count(&count).Error; err != nil {
		return nil, err
	}
	if count > 0 {
		return &User{}, errors.New("duplicate email or username")
	}

	err = db.Model(&input).Updates(User{Name: input.Name, Email: input.Email, Username: input.Username, IsActive: input.IsActive}).Error
	if err != nil {
		return &User{}, err
	}
	return input, nil
}

func (input *User) DeleteUser(id int) (*User, error) {

	db := config.GetDB()

	err := db.Model(&User{}).Where("id = ?", id).First(&input).Error
	if err != nil {
		return nil, utils.ErrorRecordNotFound
	}

	err = db.Delete(&input).Error
	if err != nil {
		return &User{}, err
	}
	return input, nil
}

func (input *User) ChangeUserPassword() (*User, error) {

	db := config.GetDB()
	//turn password into hash
	hashedPassword, err := utils.HashPassword(input.Password)
	if err != nil {
		return &User{}, err
	}
	input.Password = string(hashedPassword)

	err = db.Model(&User{}).Where("id = ?", input.ID).First(&input).Error
	if err != nil {
		return nil, utils.ErrorRecordNotFound
	}

	err = db.Model(&input).Updates(User{Password: input.Password}).Error
	if err != nil {
		return &User{}, err
	}
	return input, nil
}

func (user *User) DestroyAllSessions(ctx context.Context) error {
	allTokens, err := config.GetRedisSetMembers("Tokens:" + user.Username)
	if err != nil {
		return err
	}
	for _, token := range allTokens {
		if err := config.RemoveRedisKey("Token:" + token); err != nil {
			return err
		}
	}
	if err := config.RemoveRedisKey("Tokens:" + user.Username); err != nil {
		return err
	}

	return nil
}

func ChangePassword(ctx context.Context, oldPassword string, newPassword string) (*User, error) {
	userId, ok := utils.GetUserIdFromContext(ctx)
	if !ok || userId == 0 {
		return nil, errors.New("user id is required")
	}

	var user User
	db := config.GetDB()
	if err := db.WithContext(ctx).First(&user, userId).Error; err != nil {
		return nil, err
	}
	// check oldPassword
	if err := utils.ComparePassword(user.Password, oldPassword); err != nil {
		return nil, errors.New("old password is wrong")
	}

	//turn password into hash
	hashedPassword, err := utils.HashPassword(newPassword)
	if err != nil {
		return nil, err
	}
	newPassword = string(hashedPassword)

	tx := db.Begin()
	if err := tx.WithContext(ctx).Model(&user).UpdateColumn("password", newPassword).Error; err != nil {
		tx.Rollback()
		return nil, err
	}
	if err := config.RemoveRedisKey("User:" + user.Username); err != nil {
		tx.Rollback()
		return nil, err
	}

	// destroying all session tokens
	if err := user.DestroyAllSessions(ctx); err != nil {
		tx.Rollback()
		return nil, err
	}
	//? log history?

	return &user, tx.Commit().Error
}
