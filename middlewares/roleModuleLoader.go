package middlewares

import (
	"context"

	"bitbucket.org/mmdatafocus/books_backend/models"
	"github.com/graph-gophers/dataloader/v7"
	"gorm.io/gorm"
)

type RoleModuleReader struct {
	db *gorm.DB
}

func (r *RoleModuleReader) getRoleModules(ctx context.Context, roleIds []int) []*dataloader.Result[[]*models.RoleModule] {
	var results []models.RoleModule
	err := r.db.WithContext(ctx).Model(&models.RoleModule{}).
		Where("role_id in ?", roleIds).Find(&results).Error
	if err != nil {
		return handleError[[]*models.RoleModule](len(roleIds), err)
	}
	return generateLoaderArrayResults(results, roleIds)
	// return generateLoaderResults(results, roleIds)
}

// func GetAllowedModules(ctx context.Context)

func GetRoleModules(ctx context.Context, roleId int) ([]*models.RoleModule, error) {
	loaders := For(ctx)
	return loaders.RoleModuleLoader.Load(ctx, roleId)()
}
