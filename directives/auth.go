package directives

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strconv"
	"time"

	"bitbucket.org/mmdatafocus/books_backend/config"
	"bitbucket.org/mmdatafocus/books_backend/models"
	"bitbucket.org/mmdatafocus/books_backend/utils"
	"github.com/99designs/gqlgen/graphql"
	"github.com/vektah/gqlparser/v2/gqlerror"
	"gorm.io/gorm"
)

// retrieve user from redis or db
func getUser(username string, ctx context.Context) (*models.User, error) {
	var user models.User
	exists, err := config.GetRedisObject("User:"+username, &user)
	if err != nil {
		return nil, err
	}

	if !exists {

		db := config.GetDB()
		if err := db.WithContext(ctx).Model(&models.User{}).Where("username = ?", username).Take(&user).Error; err != nil {
			return nil, err
		}

		token_lifespan, err := strconv.Atoi(os.Getenv("TOKEN_HOUR_LIFESPAN"))
		if err != nil {
			return nil, err
		}

		if err := config.SetRedisObject("User:"+user.Username, &user, time.Duration(token_lifespan)*time.Hour); err != nil {
			return nil, err
		}
	}
	return &user, nil
}

// retrieve role's allowed query paths from redis and check if the gqlpath is allowed
func authorizeUser(ctx context.Context, roleId int, gqlpath string) error {
	var queryPaths map[string]bool
	exists, err := config.GetRedisObject("AllowedPaths:Role:"+fmt.Sprint(roleId), &queryPaths)
	if err != nil {
		return err
	}

	if !exists {

		queryPaths, err = models.GetQueryPathsFromRole(ctx, roleId)
		if err != nil {
			return err
		}

		// store in redis
		if err := config.SetRedisObject("AllowedPaths:Role:"+fmt.Sprint(roleId), &queryPaths, 0); err != nil {
			return err
		}
	}

	// check if current path is allowed for current user
	// using a map for faster look up, non-existent key will return false, default zero for boolean
	if allowed := queryPaths[gqlpath]; !allowed {
		if defaultAllowed := models.GetDefaultAllowedPaths()[gqlpath]; defaultAllowed {
			return nil
		}
		return errors.New("Unauthorized")
	}
	return nil
}

func Auth(ctx context.Context, obj interface{}, next graphql.Resolver) (interface{}, error) {

	username, ok := utils.GetUsernameFromContext(ctx)
	if !ok || username == "" {
		return nil, &gqlerror.Error{
			Message: "Access Denied",
		}
	}

	user, err := getUser(username, ctx)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			// destroy current session if user has been deleted
			models.Logout(ctx)
		}
		return nil, &gqlerror.Error{
			Message: err.Error(),
		}
	}
	if !*user.IsActive {
		return nil, &gqlerror.Error{
			Message: "User is disabled",
		}
	}

	gqlpath := graphql.GetPath(ctx).String()

	// if admin, only allow admin paths
	// if custom/owner, check its role for allowed paths
	if user.Role == models.UserRoleAdmin {
		if adminAllowed := models.GetAdminPaths()[gqlpath]; !adminAllowed {
			return nil, &gqlerror.Error{
				Message: "Unauthorized",
			}
		}
	} else {
		// user is either owner or custom
		if err := authorizeUser(ctx, user.RoleId, gqlpath); err != nil {
			return nil, &gqlerror.Error{
				Message: err.Error(),
			}
		}
	}

	ctx = context.WithValue(ctx, utils.ContextKeyBusinessId, user.BusinessId)
	ctx = context.WithValue(ctx, utils.ContextKeyUserId, user.ID)
	ctx = context.WithValue(ctx, utils.ContextKeyUserName, user.Name)
	ctx = utils.SetIsAdminInContext(ctx, user.Role == models.UserRoleAdmin)

	return next(ctx)
}
