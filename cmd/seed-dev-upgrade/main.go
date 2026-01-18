package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/mmdatafocus/books_backend/config"
	"github.com/mmdatafocus/books_backend/models"
	"github.com/mmdatafocus/books_backend/utils"
	"gorm.io/gorm"
)

func getenv(key, def string) string {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return def
	}
	return v
}

func getenvInt(key string, def int) int {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return def
	}
	return n
}

func slugify(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = strings.ReplaceAll(s, " ", "-")
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' {
			b.WriteRune(r)
		}
	}
	out := b.String()
	if out == "" {
		return "dev-upgrade"
	}
	return out
}

type boolFlag struct {
	v   bool
	set bool
}

func (b *boolFlag) String() string {
	if b == nil {
		return ""
	}
	if b.v {
		return "true"
	}
	return "false"
}

func (b *boolFlag) Set(s string) error {
	v, err := strconv.ParseBool(strings.TrimSpace(s))
	if err != nil {
		return err
	}
	b.v = v
	b.set = true
	return nil
}

func (b *boolFlag) IsBoolFlag() bool { return true }

func main() {
	// Env-first, flags override env for convenience.
	defaultBusinessName := getenv("SEED_BUSINESS_NAME", "dev-upgrade")
	defaultOwnerEmail := getenv("SEED_OWNER_EMAIL", "dev-upgrade@local")
	defaultOwnerPassword := strings.TrimSpace(os.Getenv("SEED_OWNER_PASSWORD"))
	defaultDemoPassword := strings.TrimSpace(os.Getenv("SEED_DEMO_PASSWORD"))
	defaultCompanyCount := getenvInt("SEED_COMPANY_COUNT", 1)

	businessName := flag.String("business-name", defaultBusinessName, "Business name to create/reuse")
	ownerEmail := flag.String("owner-email", defaultOwnerEmail, "Owner user email/username to create/reuse")
	ownerPassword := flag.String("owner-password", defaultOwnerPassword, "Owner user password to set (required)")
	demoPassword := flag.String("demo-password", defaultDemoPassword, "Password for 5 demo users (defaults to owner password if empty)")
	companyCount := flag.Int("companies", defaultCompanyCount, "How many companies (businesses) to seed (creates -01..-NN variants when > 1)")
	var createDemoUsersFlag boolFlag
	flag.Var(&createDemoUsersFlag, "create-demo-users", "Also create 5 demo users per company (default: true when --companies=1, false when > 1)")
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

	baseBizName := strings.TrimSpace(*businessName)
	baseOwnerEmail := strings.TrimSpace(*ownerEmail)
	bizSlug := slugify(baseBizName)

	n := *companyCount
	if n < 1 {
		n = 1
	}
	createDemoUsers := createDemoUsersFlag.v
	if !createDemoUsersFlag.set {
		// Preserve old behavior for 1 company: create demo users by default.
		createDemoUsers = n == 1
	}

	type seeded struct {
		businessID    string
		businessName  string
		ownerUsername string
	}
	seededList := make([]seeded, 0, n)

	for i := 1; i <= n; i++ {
		var desiredBizName string
		var desiredOwnerUsername string
		if n == 1 {
			desiredBizName = baseBizName
			desiredOwnerUsername = baseOwnerEmail
		} else {
			desiredBizName = fmt.Sprintf("%s-%02d", baseBizName, i)
			desiredOwnerUsername = fmt.Sprintf("%s-owner%02d@local", bizSlug, i)
		}
		desiredOwnerUsername = strings.TrimSpace(desiredOwnerUsername)
		if desiredOwnerUsername == "" {
			// Fallback to unique pattern if user didn't provide an owner email.
			desiredOwnerUsername = fmt.Sprintf("%s-owner%02d@local", bizSlug, i)
		}

		// 1) Find or create business (idempotent)
		var business models.Business
		bizQuery := db.WithContext(ctx).Model(&models.Business{})
		// Business lookup is best-effort by email and/or name.
		bizQuery = bizQuery.Where("email = ?", desiredOwnerUsername).Or("name = ?", strings.TrimSpace(desiredBizName))
		bizErr := bizQuery.First(&business).Error

		if bizErr != nil {
			if bizErr != gorm.ErrRecordNotFound {
				fmt.Fprintf(os.Stderr, "failed to lookup business: %v\n", bizErr)
				os.Exit(1)
			}

			created, err := models.CreateBusiness(ctx, &models.NewBusiness{
				Name:  strings.TrimSpace(desiredBizName),
				Email: desiredOwnerUsername,
			})
			if err != nil {
				fmt.Fprintf(os.Stderr, "failed to create business: %v\n", err)
				os.Exit(1)
			}
			business = *created
		}

		// Ensure business id is available in context for model hooks (history/audit).
		bizCtx := context.WithValue(ctx, utils.ContextKeyBusinessId, business.ID.String())

		// 2) Ensure owner user exists and set password
		if err := db.WithContext(bizCtx).Transaction(func(tx *gorm.DB) error {
			// Find or create Owner role
			var ownerRole models.Role
			roleErr := tx.WithContext(bizCtx).
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
				if err := tx.WithContext(bizCtx).Create(&ownerRole).Error; err != nil {
					return fmt.Errorf("failed to create Owner role: %w", err)
				}
			}

			hashedOwnerPassword, err := utils.HashPassword(strings.TrimSpace(*ownerPassword))
			if err != nil {
				return fmt.Errorf("failed to hash owner password: %w", err)
			}

			// Find or create owner user
			var owner models.User
			userErr := tx.WithContext(bizCtx).Model(&models.User{}).Where("username = ?", desiredOwnerUsername).First(&owner).Error
			if userErr != nil {
				if userErr != gorm.ErrRecordNotFound {
					return fmt.Errorf("failed to lookup owner user: %w", userErr)
				}

				owner = models.User{
					BusinessId: business.ID.String(),
					Username:   desiredOwnerUsername,
					Name:       business.Name,
					Email:      utils.NilIfEmpty(desiredOwnerUsername),
					Password:   string(hashedOwnerPassword),
					IsActive:   utils.NewTrue(),
					RoleId:     ownerRole.ID,
					Role:       models.UserRoleCustom,
				}
				if err := tx.WithContext(bizCtx).Create(&owner).Error; err != nil {
					return fmt.Errorf("failed to create owner user: %w", err)
				}
			} else {
				// If user exists, ensure it's assigned to this business.
				if owner.BusinessId != business.ID.String() {
					return fmt.Errorf("owner user exists but belongs to another business (username=%s business_id=%s)", owner.Username, owner.BusinessId)
				}

				// Update password + ensure active + role
				if err := tx.WithContext(bizCtx).Model(&models.User{}).Where("id = ?", owner.ID).Updates(map[string]any{
					"password":  string(hashedOwnerPassword),
					"is_active": utils.NewTrue(),
					"role_id":   ownerRole.ID,
					"role":      models.UserRoleCustom,
					"name":      business.Name,
					"email":     utils.NilIfEmpty(desiredOwnerUsername),
				}).Error; err != nil {
					return fmt.Errorf("failed to update owner user: %w", err)
				}

				// Best-effort cache invalidation.
				_ = owner.RemoveInstanceRedis()
				_ = owner.RemoveAllRedis()
			}

			// 3) Optional demo users
			if createDemoUsers {
				pass := strings.TrimSpace(*demoPassword)
				if pass == "" {
					pass = strings.TrimSpace(*ownerPassword)
				}
				hashedDemoPassword, err := utils.HashPassword(pass)
				if err != nil {
					return fmt.Errorf("failed to hash demo password: %w", err)
				}

				prefix := slugify(business.Name)
				makeUsername := func(j int) string {
					return fmt.Sprintf("%s-user%02d@local", prefix, j)
				}
				for j := 1; j <= 5; j++ {
					username := makeUsername(j)
					displayName := fmt.Sprintf("%s User %02d", business.Name, j)

					var existing models.User
					err := tx.WithContext(bizCtx).Model(&models.User{}).Where("username = ?", username).First(&existing).Error
					if err != nil {
						if err != gorm.ErrRecordNotFound {
							return fmt.Errorf("failed to lookup demo user %s: %w", username, err)
						}

						u := models.User{
							BusinessId: business.ID.String(),
							Username:   username,
							Name:       displayName,
							Email:      utils.NilIfEmpty(username),
							Password:   string(hashedDemoPassword),
							IsActive:   utils.NewTrue(),
							RoleId:     ownerRole.ID,
							Role:       models.UserRoleCustom,
						}
						if err := tx.WithContext(bizCtx).Create(&u).Error; err != nil {
							return fmt.Errorf("failed to create demo user %s: %w", username, err)
						}
						continue
					}

					if existing.BusinessId != business.ID.String() {
						return fmt.Errorf("demo user exists but belongs to another business (username=%s business_id=%s)", existing.Username, existing.BusinessId)
					}
					if err := tx.WithContext(bizCtx).Model(&models.User{}).Where("id = ?", existing.ID).Updates(map[string]any{
						"password":  string(hashedDemoPassword),
						"is_active": utils.NewTrue(),
						"role_id":   ownerRole.ID,
						"role":      models.UserRoleCustom,
						"name":      displayName,
						"email":     utils.NilIfEmpty(username),
					}).Error; err != nil {
						return fmt.Errorf("failed to update demo user %s: %w", username, err)
					}
					_ = existing.RemoveInstanceRedis()
					_ = existing.RemoveAllRedis()
				}
			}

			return nil
		}); err != nil {
			fmt.Fprintf(os.Stderr, "seed transaction failed: %v\n", err)
			os.Exit(1)
		}

		seededList = append(seededList, seeded{
			businessID:    business.ID.String(),
			businessName:  business.Name,
			ownerUsername: desiredOwnerUsername,
		})
	}

	fmt.Println("Seed complete")
	for _, s := range seededList {
		fmt.Printf("BusinessID: %s | BusinessName: %s | OwnerUsername: %s\n", s.businessID, s.businessName, s.ownerUsername)
	}
	fmt.Println("OwnerPassword: (set)")
}
