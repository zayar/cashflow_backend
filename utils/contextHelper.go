package utils

import (
	"context"

	"github.com/mmdatafocus/books_backend/appctx"
)

// Alias the shared context key type so existing code keeps working.
type contextKey = appctx.ContextKey

var (
	ContextKeyToken         = appctx.ContextKeyToken
	ContextKeyBusinessId    = appctx.ContextKeyBusinessId
	ContextKeyUsername      = appctx.ContextKeyUsername
	ContextKeyUserId        = appctx.ContextKeyUserId
	ContextKeyUserName      = appctx.ContextKeyUserName
	ContextKeyBranchId      = appctx.ContextKeyBranchId
	ContextKeyCorrelationId = appctx.ContextKeyCorrelationId

	ContextKeyIsAdmin         = appctx.ContextKeyIsAdmin
	ContextKeySkipTenantScope = appctx.ContextKeySkipTenantScope
)

func GetTokenFromContext(ctx context.Context) (string, bool) {
	return appctx.GetString(ctx, ContextKeyToken)
}

func GetBusinessIdFromContext(ctx context.Context) (string, bool) {
	return appctx.GetString(ctx, ContextKeyBusinessId)
}

func GetUsernameFromContext(ctx context.Context) (string, bool) {
	return appctx.GetString(ctx, ContextKeyUsername)
}

func GetUserIdFromContext(ctx context.Context) (int, bool) {
	return appctx.GetInt(ctx, ContextKeyUserId)
}

func GetUserNameFromContext(ctx context.Context) (string, bool) {
	return appctx.GetString(ctx, ContextKeyUserName)
}

func GetBranchIdFromContext(ctx context.Context) (int, bool) {
	return appctx.GetInt(ctx, ContextKeyBranchId)
}

func GetCorrelationIdFromContext(ctx context.Context) (string, bool) {
	return appctx.GetString(ctx, ContextKeyCorrelationId)
}

func SetTokenInContext(ctx context.Context, token string) context.Context {
	return appctx.Set(ctx, ContextKeyToken, token)
}

func SetBusinessIdInContext(ctx context.Context, businessId string) context.Context {
	return appctx.Set(ctx, ContextKeyBusinessId, businessId)
}

func SetUsernameInContext(ctx context.Context, username string) context.Context {
	return appctx.Set(ctx, ContextKeyUsername, username)
}

func SetUserIdInContext(ctx context.Context, userId int) context.Context {
	return appctx.Set(ctx, ContextKeyUserId, userId)
}

func SetUserNameInContext(ctx context.Context, userName string) context.Context {
	return appctx.Set(ctx, ContextKeyUserName, userName)
}

func SetBranchIdInContext(ctx context.Context, branchId int) context.Context {
	return appctx.Set(ctx, ContextKeyBranchId, branchId)
}

func SetCorrelationIdInContext(ctx context.Context, correlationId string) context.Context {
	return appctx.Set(ctx, ContextKeyCorrelationId, correlationId)
}

func GetIsAdminFromContext(ctx context.Context) (bool, bool) {
	return appctx.GetBool(ctx, ContextKeyIsAdmin)
}

func SetIsAdminInContext(ctx context.Context, isAdmin bool) context.Context {
	return appctx.Set(ctx, ContextKeyIsAdmin, isAdmin)
}

func GetSkipTenantScopeFromContext(ctx context.Context) (bool, bool) {
	return appctx.GetBool(ctx, ContextKeySkipTenantScope)
}

func SetSkipTenantScopeInContext(ctx context.Context, skip bool) context.Context {
	return appctx.Set(ctx, ContextKeySkipTenantScope, skip)
}
