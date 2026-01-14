package config

import (
	"context"
	"strings"

	"github.com/mmdatafocus/books_backend/appctx"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// TenantGuardPlugin enforces multi-tenant isolation by automatically scoping
// queries/updates/deletes to the request's business_id when the model has a business_id column.
//
// NOTE:
// - This does NOT apply to Raw SQL queries. Those must include business_id manually.
// - Admin/internal bypass is explicit via context flags.
type TenantGuardPlugin struct{}

func NewTenantGuardPlugin() *TenantGuardPlugin { return &TenantGuardPlugin{} }

func (p *TenantGuardPlugin) Name() string { return "tenant_guard" }

func (p *TenantGuardPlugin) Initialize(db *gorm.DB) error {
	// Query
	if err := db.Callback().Query().Before("gorm:query").Register("tenant_guard:query", tenantGuardCallback); err != nil {
		return err
	}
	// Row (First/Take)
	if err := db.Callback().Row().Before("gorm:row").Register("tenant_guard:row", tenantGuardCallback); err != nil {
		return err
	}
	// Update
	if err := db.Callback().Update().Before("gorm:update").Register("tenant_guard:update", tenantGuardCallback); err != nil {
		return err
	}
	// Delete
	if err := db.Callback().Delete().Before("gorm:delete").Register("tenant_guard:delete", tenantGuardCallback); err != nil {
		return err
	}
	return nil
}

func tenantGuardCallback(db *gorm.DB) {
	if db == nil || db.Statement == nil {
		return
	}
	ctx := db.Statement.Context
	if ctx == nil {
		return
	}
	if shouldBypassTenantScope(ctx) {
		return
	}
	businessID := businessIdFromContext(ctx)
	if businessID == "" {
		return
	}

	// Only apply if the current model/table includes a business_id column.
	if db.Statement.Schema == nil {
		return
	}
	hasBusinessID := false
	for _, f := range db.Statement.Schema.Fields {
		if strings.EqualFold(f.DBName, "business_id") {
			hasBusinessID = true
			break
		}
	}
	if !hasBusinessID {
		return
	}

	// Don't duplicate an explicit tenant filter.
	if whereHasBusinessID(db.Statement.Clauses["WHERE"]) {
		return
	}

	db.Statement.AddClause(clause.Where{
		Exprs: []clause.Expression{
			clause.Eq{
				Column: clause.Column{Table: db.Statement.Table, Name: "business_id"},
				Value:  businessID,
			},
		},
	})
}

func businessIdFromContext(ctx context.Context) string {
	if v, ok := ctx.Value(appctx.ContextKeyBusinessId).(string); ok && v != "" {
		return v
	}
	return ""
}

func shouldBypassTenantScope(ctx context.Context) bool {
	if v, ok := ctx.Value(appctx.ContextKeySkipTenantScope).(bool); ok && v {
		return true
	}
	if v, ok := ctx.Value(appctx.ContextKeyIsAdmin).(bool); ok && v {
		return true
	}
	return false
}

func whereHasBusinessID(c clause.Clause) bool {
	if c.Expression == nil {
		return false
	}
	w, ok := c.Expression.(clause.Where)
	if !ok {
		return false
	}
	for _, e := range w.Exprs {
		if exprHasBusinessID(e) {
			return true
		}
	}
	return false
}

func exprHasBusinessID(e clause.Expression) bool {
	switch v := e.(type) {
	case clause.Eq:
		return colIsBusinessID(v.Column)
	case clause.Neq:
		return colIsBusinessID(v.Column)
	case clause.Gt:
		return colIsBusinessID(v.Column)
	case clause.Gte:
		return colIsBusinessID(v.Column)
	case clause.Lt:
		return colIsBusinessID(v.Column)
	case clause.Lte:
		return colIsBusinessID(v.Column)
	case clause.IN:
		return colIsBusinessID(v.Column)
	case clause.AndConditions:
		for _, x := range v.Exprs {
			if exprHasBusinessID(x) {
				return true
			}
		}
		return false
	case clause.OrConditions:
		for _, x := range v.Exprs {
			if exprHasBusinessID(x) {
				return true
			}
		}
		return false
	case clause.Expr:
		// Best-effort for raw expressions.
		return strings.Contains(strings.ToLower(v.SQL), "business_id")
	default:
		return false
	}
}

func colIsBusinessID(col any) bool {
	switch c := col.(type) {
	case string:
		return strings.EqualFold(c, "business_id")
	case clause.Column:
		return strings.EqualFold(c.Name, "business_id")
	default:
		return false
	}
}
