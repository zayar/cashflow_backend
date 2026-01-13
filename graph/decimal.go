package graph

import (
	"encoding/json"
	"fmt"
	"io"

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
		val, err := decimal.NewFromString(v)
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
