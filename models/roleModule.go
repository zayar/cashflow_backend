package models

import (
	"context"
	"errors"
	"fmt"
	"time"

	"bitbucket.org/mmdatafocus/books_backend/config"
	"bitbucket.org/mmdatafocus/books_backend/utils"
)

type RoleModule struct {
	BusinessId     string    `gorm:"primary_key;autoIncrement:false;not null" json:"business_id" binding:"required"`
	RoleId         int       `gorm:"primary_key;autoIncrement:false;not null" json:"role_id" binding:"required"`
	ModuleId       int       `gorm:"primary_key;autoIncrement:false;not null" json:"module_id" binding:"required"`
	AllowedActions string    `gorm:"not null" json:"allowed_actions" binding:"required"`
	CreatedAt      time.Time `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt      time.Time `gorm:"autoUpdateTime" json:"updated_at"`
	Role           Role      `json:"role"`
	Module         Module    `json:"module"`
}

type NewRoleModule struct {
	RoleId         int    `json:"role_id" binding:"required"`
	ModuleId       int    `json:"module_id" binding:"required"`
	AllowedActions string `json:"allowed_actions" binding:"required"`
}

/*
cache
	ListRoleModule:$roleId
*/

func (rm RoleModule) GetReferenceId() int {
	return rm.RoleId
}

func SaveRoleModule(ctx context.Context, input *NewRoleModule) (*RoleModule, error) {

	// only owner can access
	db := config.GetDB()

	businessId, ok := utils.GetBusinessIdFromContext(ctx)
	if !ok || businessId == "" {
		return nil, errors.New("business id is required")
	}

	// validate exists
	var count int64
	if err := db.WithContext(ctx).Model(&Role{}).
		Where("business_id = ? AND id = ?", businessId, input.RoleId).Count(&count).Error; err != nil {
		return nil, err
	}
	if count <= 0 {
		return nil, errors.New("roleId does not exist")
	}
	if err := db.WithContext(ctx).Model(&Module{}).
		Where("business_id = ? AND id = ?", businessId, input.ModuleId).Count(&count).Error; err != nil {
		return nil, err
	}
	if count <= 0 {
		return nil, errors.New("moduleId does not exist")
	}

	roleModule := RoleModule{
		BusinessId:     businessId,
		RoleId:         input.RoleId,
		ModuleId:       input.ModuleId,
		AllowedActions: input.AllowedActions,
	}

	err := db.WithContext(ctx).Save(&roleModule).Error
	if err != nil {
		return nil, err
	}
	// remove from redis
	if err := config.RemoveRedisKey("RoleModuleList:" + fmt.Sprint(input.RoleId)); err != nil {
		return nil, err
	}
	if err := utils.ClearPathsCache(input.RoleId); err != nil {
		return nil, err
	}
	return &roleModule, nil
}

func DeleteRoleModule(ctx context.Context, input *NewRoleModule) (*RoleModule, error) {
	// only owner can access
	db := config.GetDB()
	var result RoleModule

	businessId, ok := utils.GetBusinessIdFromContext(ctx)
	if !ok || businessId == "" {
		return nil, errors.New("business id is required")
	}

	err := db.WithContext(ctx).Model(&RoleModule{}).
		Where("business_id = ? AND role_id = ? AND module_id = ?", businessId, input.RoleId, input.ModuleId).First(&result).Error
	if err != nil {
		return nil, utils.ErrorRecordNotFound
	}

	tx := db.Begin()
	err = tx.WithContext(ctx).Delete(&result).Error
	if err != nil {
		tx.Rollback()
		return nil, err
	}
	// remove from redis
	// caching
	if err := config.RemoveRedisKey("RoleModuleList:" + fmt.Sprint(input.RoleId)); err != nil {
		tx.Rollback()
		return nil, err
	}
	if err := utils.ClearPathsCache(input.RoleId); err != nil {
		tx.Rollback()
		return nil, err
	}
	return &result, tx.Commit().Error
}

func GetRoleModules(ctx context.Context, roleId *int) ([]*RoleModule, error) {
	// only owner can access

	businessId, ok := utils.GetBusinessIdFromContext(ctx)
	if !ok || businessId == "" {
		return nil, errors.New("business id is required")
	}

	var results []*RoleModule
	db := config.GetDB()
	if roleId != nil && *roleId > 0 {
		exists, err := config.GetRedisObject("RoleModuleList:"+fmt.Sprint(*roleId), &results)
		if err != nil {
			return nil, err
		}
		if !exists {
			err := db.WithContext(ctx).Where("business_id = ? AND role_id = ?", businessId, *roleId).
				Preload("Role").Preload("Module").
				Find(&results).Error
			if err != nil {
				return nil, err
			}
			if err := config.SetRedisObject("RoleModuleList:"+fmt.Sprint(*roleId), &results, 0); err != nil {
				return nil, err
			}
		}
	} else {
		err := db.WithContext(ctx).Where("business_id = ?", businessId).Find(&results).Error
		if err != nil {
			return nil, err
		}
	}

	return results, nil
}
