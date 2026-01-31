// seed-admin creates or updates the admin console user (username: cashflowAdmin).
// Admin users have role_id = 0 and role = 'A'; the backend returns role "Admin" for login.
//
// Usage (from backend directory):
//   DB_USER=... DB_PASSWORD=... DB_HOST=... DB_PORT=... DB_NAME_2=... go run ./cmd/seed-admin
//
// Or use scripts/seed-admin.sh (same env as other backend scripts).
package main

import (
	"context"
	"fmt"
	"os"

	"github.com/mmdatafocus/books_backend/config"
	"github.com/mmdatafocus/books_backend/models"
	"github.com/mmdatafocus/books_backend/utils"
	"gorm.io/gorm"
)

const (
	adminUsername = "cashflowAdmin"
	adminPassword = "C@$$flowAdmin"
	adminName     = "Cashflow Admin"
)

func main() {
	ctx := context.Background()
	config.ConnectDatabaseWithRetry()
	db := config.GetDB()
	if db == nil {
		fmt.Fprintln(os.Stderr, "database not initialized (config.GetDB returned nil). Set DB_* env vars.")
		os.Exit(1)
	}

	// Model history hooks require business_id + user info in context.
	// We attach a real business id (first business in DB) and mark this as admin/bypass tenant scope.
	var biz models.Business
	if err := db.WithContext(ctx).Model(&models.Business{}).Select("id").First(&biz).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			fmt.Fprintln(os.Stderr, "no businesses found in DB. Create a business first (e.g. run ./cmd/seed-dev-upgrade), then rerun seed-admin.")
			os.Exit(2)
		}
		fmt.Fprintf(os.Stderr, "failed to lookup business: %v\n", err)
		os.Exit(1)
	}

	businessID := biz.ID.String()
	ctx = utils.SetBusinessIdInContext(ctx, businessID)
	ctx = utils.SetUserIdInContext(ctx, 1)
	ctx = utils.SetUserNameInContext(ctx, "Seed")
	ctx = utils.SetUsernameInContext(ctx, adminUsername)
	ctx = utils.SetIsAdminInContext(ctx, true)
	ctx = utils.SetSkipTenantScopeInContext(ctx, true)

	hashed, err := utils.HashPassword(adminPassword)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to hash password: %v\n", err)
		os.Exit(1)
	}
	hashedStr := string(hashed)

	var existing models.User
	err = db.WithContext(ctx).Model(&models.User{}).Where("username = ?", adminUsername).First(&existing).Error
	if err != nil {
		if err != gorm.ErrRecordNotFound {
			fmt.Fprintf(os.Stderr, "failed to lookup user: %v\n", err)
			os.Exit(1)
		}
		// Create new admin user
		u := models.User{
			Username:   adminUsername,
			Name:       adminName,
			Password:   hashedStr,
			IsActive:   utils.NewTrue(),
			RoleId:     0,
			Role:       models.UserRoleAdmin,
			BusinessId: businessID,
		}
		if err := db.WithContext(ctx).Create(&u).Error; err != nil {
			fmt.Fprintf(os.Stderr, "failed to create admin user: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Created admin user: username=%q (role_id=0, role=Admin)\n", adminUsername)
		return
	}

	// Update existing user: ensure password and admin role
	if err := db.WithContext(ctx).Model(&models.User{}).Where("username = ?", adminUsername).Updates(map[string]any{
		"password":  hashedStr,
		"name":      adminName,
		"is_active": utils.NewTrue(),
		"business_id": businessID,
		"role_id":   0,
		"role":      models.UserRoleAdmin,
	}).Error; err != nil {
		fmt.Fprintf(os.Stderr, "failed to update admin user: %v\n", err)
		os.Exit(1)
	}
	_ = existing.RemoveInstanceRedis()
	fmt.Printf("Updated admin user: username=%q (role_id=0, role=Admin)\n", adminUsername)
}
