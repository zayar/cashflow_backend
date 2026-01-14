package appctx

import "context"

// ContextKey is the shared type for all context keys in this codebase.
// Keeping it in a tiny package avoids import cycles (config <-> utils).
type ContextKey string

func (c ContextKey) String() string { return string(c) }

var (
	ContextKeyToken         = ContextKey("Token")
	ContextKeyBusinessId    = ContextKey("BusinessId")
	ContextKeyUsername      = ContextKey("Username")
	ContextKeyUserId        = ContextKey("UserId")
	ContextKeyUserName      = ContextKey("UserName")
	ContextKeyBranchId      = ContextKey("BranchId")
	ContextKeyCorrelationId = ContextKey("CorrelationId")

	// ContextKeyIsAdmin is true for platform admins. Used for tenant-scope bypass.
	ContextKeyIsAdmin = ContextKey("IsAdmin")

	// ContextKeySkipTenantScope forces tenant scoping to be disabled for the request.
	// Use sparingly (internal ops only).
	ContextKeySkipTenantScope = ContextKey("SkipTenantScope")
)

func GetString(ctx context.Context, key ContextKey) (string, bool) {
	v, ok := ctx.Value(key).(string)
	return v, ok
}

func GetBool(ctx context.Context, key ContextKey) (bool, bool) {
	v, ok := ctx.Value(key).(bool)
	return v, ok
}

func GetInt(ctx context.Context, key ContextKey) (int, bool) {
	v, ok := ctx.Value(key).(int)
	return v, ok
}

func Set(ctx context.Context, key ContextKey, value any) context.Context {
	return context.WithValue(ctx, key, value)
}
