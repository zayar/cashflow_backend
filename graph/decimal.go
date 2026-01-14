package graph

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/99designs/gqlgen/graphql"
	"github.com/shopspring/decimal"
)

func MarshalDecimal(d decimal.Decimal) graphql.Marshaler {
	return graphql.WriterFunc(func(w io.Writer) {
		w.Write([]byte(d.String()))
	})
}

func UnmarshalDecimal(i interface{}) (decimal.Decimal, error) {
	switch v := i.(type) {
	case string:
		s := strings.TrimSpace(v)
		// Accept common user-formatted strings like:
		// - "20,000"
		// - "MMK 20,000"
		// - "MMK -20,000"
		// - "Ks 20000"
		//
		// Keep digits, '.', and a leading '-' only.
		if s != "" {
			s = strings.ReplaceAll(s, ",", "")
			s = strings.ReplaceAll(s, "MMK", "")
			s = strings.ReplaceAll(s, "mmk", "")
			s = strings.ReplaceAll(s, "Ks", "")
			s = strings.ReplaceAll(s, "ks", "")
			s = strings.TrimSpace(s)
		}
		neg := false
		if strings.HasPrefix(s, "-") {
			neg = true
			s = strings.TrimSpace(strings.TrimPrefix(s, "-"))
		}
		// Strip everything except digits and '.'.
		var b strings.Builder
		b.Grow(len(s) + 1)
		for _, r := range s {
			if (r >= '0' && r <= '9') || r == '.' {
				b.WriteRune(r)
			}
		}
		clean := b.String()
		if clean == "" {
			return decimal.NewFromInt(0), fmt.Errorf("invalid value")
		}
		if neg {
			clean = "-" + clean
		}

		val, err := decimal.NewFromString(clean)
		if err != nil {
			return decimal.NewFromInt(0), err
		}
		return val, nil
	case json.Number:
		num, err := v.Float64()
		if err != nil {
			return decimal.NewFromInt(0), err
		}
		return decimal.NewFromFloat(num), nil
	default:
		return decimal.NewFromInt(0), fmt.Errorf("invalid value")
	}
}
