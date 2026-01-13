package models

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"bitbucket.org/mmdatafocus/books_backend/config"
	"bitbucket.org/mmdatafocus/books_backend/utils"
	"gorm.io/gorm"
)

type Role struct {
	ID          int           `gorm:"primary_key" json:"id"`
	BusinessId  string        `gorm:"index;not null" json:"business_id" binding:"required"`
	Name        string        `gorm:"index;size:100;not null" json:"name" binding:"required"`
	RoleModules []*RoleModule `gorm:"foreignKey:RoleId"`
	CreatedAt   time.Time     `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt   time.Time     `gorm:"autoUpdateTime" json:"updated_at"`
}

type NewRole struct {
	Name           string              `json:"name" binding:"required"`
	AllowedModules []*NewAllowedModule `json:"allowed_modules"`
}

type NewAllowedModule struct {
	ModuleID       int    `json:"moduleId"`
	AllowedActions string `json:"allowedActions"`
}

func extractModuleActions(s string) []string {
	return strings.Split(strings.ToLower(s), ";")
}

// retrieve allowed query paths for role
func GetQueryPathsFromRole(ctx context.Context, roleId int) (map[string]bool, error) {
	db := config.GetDB()
	var role Role
	if err := db.WithContext(ctx).Preload("RoleModules").Preload("RoleModules.Module").Where("id = ?", roleId).First(&role).Error; err != nil {
		return nil, err
	}

	allowedPaths := make(map[string]bool, 0)
	for _, permission := range role.RoleModules {
		validActions := extractModuleActions(permission.Module.Actions)
		allowedActions := extractModuleActions(permission.AllowedActions)
		module := permission.Module.Name
		// module = utils.UppercaseFirst(module)

		for _, action := range allowedActions {
			// check if the action is valid

			if slices.Contains(validActions, action) {
				// changing case of action & module for older module name convention
				queryPrefixes, found := GetQueryPrefixMap()[module+"|"+action]
				if !found {
					// use the action as prefix by default
					queryPrefixes = append(queryPrefixes, action)
				}

				for _, qPrefix := range queryPrefixes {
					allowedPaths[fmt.Sprintf("%s%s", qPrefix, module)] = true
				}
				// switch action {
				// case "read":
				// 	allowedPaths["get"+module] = true
				// 	allowedPaths["list"+module] = true
				// 	allowedPaths["listAll"+module] = true
				// 	allowedPaths["paginate"+module] = true
				// case "update":
				// 	allowedPaths["update"+module] = true
				// 	allowedPaths["toggleActive"+module] = true
				// default:
				// 	action = utils.LowercaseFirst(action)
				// 	allowedPaths[action+module] = true
				// }
			}
		}
	}
	return allowedPaths, nil
}

func mapRoleModules(ctx context.Context, businessId string, input []*NewAllowedModule) ([]*RoleModule, error) {

	availabeModuleActions := make(map[int]string, 0) // moduleId:actions
	var modules []Module
	db := config.GetDB()
	if err := db.WithContext(ctx).Where("business_id = ?", businessId).Find(&modules).Error; err != nil {
		return nil, err
	}
	for _, m := range modules {
		availabeModuleActions[m.ID] = m.Actions
	}

	var roleModules []*RoleModule
	for _, permission := range input {

		availableActionsString, ok := availabeModuleActions[permission.ModuleID]
		if !ok || availableActionsString == "" {
			return nil, errors.New("module_id not found")
		}
		availableActions := extractModuleActions(availableActionsString)
		inputActions := extractModuleActions(permission.AllowedActions)
		for _, action := range inputActions {
			if !slices.Contains(availableActions, action) {
				return nil, errors.New("invalid module action")
			}
		}

		roleModules = append(roleModules, &RoleModule{
			ModuleId:       permission.ModuleID,
			AllowedActions: permission.AllowedActions,
		})
	}
	return roleModules, nil
}

func CreateRole(ctx context.Context, input *NewRole) (*Role, error) {

	businessId, ok := utils.GetBusinessIdFromContext(ctx)
	if !ok || businessId == "" {
		return nil, errors.New("business id is required")
	}

	// check duplicate
	if err := utils.ValidateUnique[Role](ctx, businessId, "name", input.Name, 0); err != nil {
		return nil, err
	}
	roleModules, err := mapRoleModules(ctx, businessId, input.AllowedModules)
	if err != nil {
		return nil, err
	}

	role := Role{
		Name:        input.Name,
		BusinessId:  businessId,
		RoleModules: roleModules,
	}
	db := config.GetDB()
	// tx := db.Begin()
	err = db.WithContext(ctx).Create(&role).Error
	if err != nil {
		return nil, err
	}
	return &role, nil
}

func UpdateRole(ctx context.Context, id int, input *NewRole) (*Role, error) {

	businessId, ok := utils.GetBusinessIdFromContext(ctx)
	if !ok || businessId == "" {
		return nil, errors.New("business id is required")
	}

	// check role exists
	if err := utils.ValidateResourceId[Role](ctx, businessId, id); err != nil {
		return nil, err
	}

	// check duplicate
	if err := utils.ValidateUnique[Role](ctx, businessId, "name", input.Name, id); err != nil {
		return nil, err
	}
	roleModules, err := mapRoleModules(ctx, businessId, input.AllowedModules)
	if err != nil {
		return nil, err
	}

	role := Role{
		ID:         id,
		BusinessId: businessId,
		Name:       input.Name,
	}

	db := config.GetDB()
	tx := db.Begin()

	// full replace, delete excluded
	err = tx.WithContext(ctx).Model(&role).
		Session(&gorm.Session{FullSaveAssociations: true, SkipHooks: true}).
		Association("RoleModules").Unscoped().Replace(roleModules)
	if err != nil {
		tx.Rollback()
		return nil, err
	}
	err = tx.WithContext(ctx).Model(&role).Updates(map[string]interface{}{
		"Name": input.Name,
		// "RoleModules": roleModules,
	}).Error
	if err != nil {
		tx.Rollback()
		return nil, err
	}
	// caching
	if err := utils.ClearPathsCache(id); err != nil {
		tx.Rollback()
		return nil, err
	}

	return &role, tx.Commit().Error
}

func DeleteRole(ctx context.Context, id int) (*Role, error) {

	db := config.GetDB()
	businessId, ok := utils.GetBusinessIdFromContext(ctx)
	if !ok || businessId == "" {
		return nil, errors.New("business id is required")
	}
	result, err := utils.FetchModel[Role](ctx, businessId, id)
	if err != nil {
		return nil, err
	}

	// don't allow if a user is using the role
	count, err := utils.ResourceCountWhere[User](ctx, businessId, "role_id = ?", id)
	if err != nil {
		return nil, err
	}
	if count > 0 {
		return nil, errors.New("role has been used")
	}

	tx := db.Begin()
	// delete role
	err = tx.WithContext(ctx).Select("RoleModules").Delete(&result).Error
	if err != nil {
		tx.Rollback()
		return nil, err
	}

	// remove from redis
	// caching
	if err := utils.ClearPathsCache(id); err != nil {
		tx.Rollback()
		return nil, err
	}
	return result, tx.Commit().Error
}

func GetRole(ctx context.Context, id int) (*Role, error) {

	businessId, ok := utils.GetBusinessIdFromContext(ctx)
	if !ok || businessId == "" {
		return nil, errors.New("business id is required")
	}
	result, err := utils.FetchModel[Role](ctx, businessId, id)
	if err != nil {
		return nil, err
	}

	return result, nil
}

func GetRoles(ctx context.Context, name *string) ([]*Role, error) {

	businessId, ok := utils.GetBusinessIdFromContext(ctx)
	if !ok || businessId == "" {
		return nil, errors.New("business id is required")
	}
	results, err := utils.FetchAllModels[Role](ctx, businessId)
	if err != nil {
		return nil, err
	}

	return results, nil
}
