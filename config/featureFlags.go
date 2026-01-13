package config

import (
	"os"
	"strings"
)

// StrictInventoryDocImmutability enables fintech-grade guardrails:
// inventory-affecting documents cannot be edited after Confirmed/Adjusted; they must be voided and recreated.
//
// Set via env:
// - STRICT_INVENTORY_DOC_IMMUTABLE=true
func StrictInventoryDocImmutability() bool {
	v := strings.ToLower(strings.TrimSpace(os.Getenv("STRICT_INVENTORY_DOC_IMMUTABLE")))
	return v == "1" || v == "true" || v == "yes" || v == "y"
}

// UseStockCommandsFor enables incremental migration away from GORM model-hook side-effects.
//
// Set via env:
// - STOCK_COMMANDS_DOCS="SALES_INVOICE,SALES_ORDER,PURCHASE_ORDER,BILL,SUPPLIER_CREDIT,CREDIT_NOTE,INVENTORY_ADJUSTMENT,TRANSFER_ORDER"
//
// Doc keys are case-insensitive.
func UseStockCommandsFor(doc string) bool {
	doc = strings.ToUpper(strings.TrimSpace(doc))
	if doc == "" {
		return false
	}
	raw := os.Getenv("STOCK_COMMANDS_DOCS")
	if strings.TrimSpace(raw) == "" {
		return false
	}
	for _, part := range strings.Split(raw, ",") {
		if strings.ToUpper(strings.TrimSpace(part)) == doc {
			return true
		}
	}
	return false
}

