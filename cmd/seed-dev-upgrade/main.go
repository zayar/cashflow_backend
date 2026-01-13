package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"

	"bitbucket.org/mmdatafocus/books_backend/config"
	"bitbucket.org/mmdatafocus/books_backend/models"
	"bitbucket.org/mmdatafocus/books_backend/utils"
	"gorm.io/gorm"
)

func getenv(key, def string) string {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return def
	}
	return v
}

func main() {
	// Env-first, flags override env for convenience.
	defaultBusinessName := getenv("SEED_BUSINESS_NAME", "dev-upgrade")
	defaultOwnerEmail := getenv("SEED_OWNER_EMAIL", "dev-upgrade@local")
	defaultOwnerPassword := strings.TrimSpace(os.Getenv("SEED_OWNER_PASSWORD"))

	businessName := flag.String("business-name", defaultBusinessName, "Business name to create/reuse")
	ownerEmail := flag.String("owner-email", defaultOwnerEmail, "Owner user email/username to create/reuse")
	ownerPassword := flag.String("owner-password", defaultOwnerPassword, "Owner user password to set (required)")
	flag.Parse()

	if strings.TrimSpace(*ownerPassword) == "" {
		fmt.Fprintln(os.Stderr, "missing required owner password: set SEED_OWNER_PASSWORD or pass --owner-password")
		os.Exit(2)
	}

	ctx := context.Background()
	// config no longer connects DB in init(); do it explicitly here.
	config.ConnectDatabaseWithRetry()
	db := config.GetDB()
	if db == nil {
		fmt.Fprintln(os.Stderr, "database not initialized (config.GetDB returned nil)")
		os.Exit(1)
	}

	// Many model hooks (history/audit) require user_id + user_name in context.
	actorUserID := 1
	if v := strings.TrimSpace(os.Getenv("SEED_ACTOR_USER_ID")); v != "" {
		if parsed, err := strconv.Atoi(v); err == nil && parsed > 0 {
			actorUserID = parsed
		}
	}
	actorUserName := getenv("SEED_ACTOR_USER_NAME", "Seed")
	ctx = context.WithValue(ctx, utils.ContextKeyUserId, actorUserID)
	ctx = context.WithValue(ctx, utils.ContextKeyUserName, actorUserName)

	// 1) Find or create business (idempotent)
	var business models.Business
	bizQuery := db.WithContext(ctx).Model(&models.Business{})
	if strings.TrimSpace(*ownerEmail) != "" {
		bizQuery = bizQuery.Where("email = ?", strings.TrimSpace(*ownerEmail))
	}
	if strings.TrimSpace(*businessName) != "" {
		// If email lookup didn't find it, allow name lookup.
		bizQuery = bizQuery.Or("name = ?", strings.TrimSpace(*businessName))
	}
	bizErr := bizQuery.First(&business).Error

	if bizErr != nil {
		if bizErr != gorm.ErrRecordNotFound {
			fmt.Fprintf(os.Stderr, "failed to lookup business: %v\n", bizErr)
			os.Exit(1)
		}

		created, err := models.CreateBusiness(ctx, &models.NewBusiness{
			Name:  strings.TrimSpace(*businessName),
			Email: strings.TrimSpace(*ownerEmail),
		})
		if err != nil {
			fmt.Fprintf(os.Stderr, "failed to create business: %v\n", err)
			os.Exit(1)
		}
		business = *created
	}

	// Ensure business id is available in context for model hooks (history/audit).
	ctx = context.WithValue(ctx, utils.ContextKeyBusinessId, business.ID.String())

	// 2) Ensure owner user exists and set password
	if err := db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Find or create Owner role
		var ownerRole models.Role
		roleErr := tx.WithContext(ctx).
			Model(&models.Role{}).
			Where("business_id = ? AND name = ?", business.ID.String(), "Owner").
			First(&ownerRole).Error
		if roleErr != nil {
			if roleErr != gorm.ErrRecordNotFound {
				return fmt.Errorf("failed to lookup Owner role: %w", roleErr)
			}
			ownerRole = models.Role{
				BusinessId: business.ID.String(),
				Name:       "Owner",
			}
			if err := tx.WithContext(ctx).Create(&ownerRole).Error; err != nil {
				return fmt.Errorf("failed to create Owner role: %w", err)
			}
		}

		// Find or create owner user
		email := strings.TrimSpace(*ownerEmail)
		var owner models.User
		userErr := tx.WithContext(ctx).Model(&models.User{}).Where("username = ?", email).First(&owner).Error
		if userErr != nil {
			if userErr != gorm.ErrRecordNotFound {
				return fmt.Errorf("failed to lookup owner user: %w", userErr)
			}

			hashedPassword, err := utils.HashPassword(strings.TrimSpace(*ownerPassword))
			if err != nil {
				return fmt.Errorf("failed to hash password: %w", err)
			}

			owner = models.User{
				BusinessId: business.ID.String(),
				Username:   email,
				Name:       business.Name,
				Email:      utils.NilIfEmpty(email),
				Password:   string(hashedPassword),
				IsActive:   utils.NewTrue(),
				RoleId:     ownerRole.ID,
				Role:       models.UserRoleCustom,
			}
			if err := tx.WithContext(ctx).Create(&owner).Error; err != nil {
				return fmt.Errorf("failed to create owner user: %w", err)
			}
			return nil
		}

		// If user exists, ensure it's assigned to this business.
		if owner.BusinessId != business.ID.String() {
			return fmt.Errorf("owner user exists but belongs to another business (username=%s business_id=%s)", owner.Username, owner.BusinessId)
		}

		// Update password + ensure active + role
		hashedPassword, err := utils.HashPassword(strings.TrimSpace(*ownerPassword))
		if err != nil {
			return fmt.Errorf("failed to hash password: %w", err)
		}
		if err := tx.WithContext(ctx).Model(&models.User{}).Where("id = ?", owner.ID).Updates(map[string]any{
			"password":  string(hashedPassword),
			"is_active": utils.NewTrue(),
			"role_id":   ownerRole.ID,
			"role":      models.UserRoleCustom,
		}).Error; err != nil {
			return fmt.Errorf("failed to update owner user: %w", err)
		}

		// Best-effort cache invalidation (won't break seed if it fails later once we make redis optional).
		_ = owner.RemoveInstanceRedis()
		_ = owner.RemoveAllRedis()
		return nil
	}); err != nil {
		fmt.Fprintf(os.Stderr, "seed transaction failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("Seed complete")
	fmt.Printf("BusinessID: %s\n", business.ID.String())
	fmt.Printf("OwnerUsername: %s\n", strings.TrimSpace(*ownerEmail))
	fmt.Println("OwnerPassword: (set)")
}

