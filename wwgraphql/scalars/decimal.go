package scalars

import (
	"fmt"
	"github.com/99designs/gqlgen/graphql"
	"github.com/shopspring/decimal"
	"io"
)

func MarshalDecimalScalar(id *decimal.Decimal) graphql.Marshaler {
	return graphql.WriterFunc(func(w io.Writer) {
		if id == nil {
			_, _ = w.Write([]byte("null"))
			return
		}
		b, _ := id.MarshalText()
		_, _ = w.Write([]byte(`"`))
		_, _ = w.Write(b)
		_, _ = w.Write([]byte(`"`))
	})
}

func UnmarshalDecimalScalar(v interface{}) (*decimal.Decimal, error) {
	if v == nil {
		return nil, nil
	}
	val, err := func() (decimal.Decimal, error) {
		switch v := v.(type) {
		case string:
			return decimal.NewFromString(v)
		case int:
			return decimal.NewFromInt(int64(v)), nil
		case int64:
			return decimal.NewFromInt(v), nil
		default:
			return decimal.Zero, fmt.Errorf("%T is not a string or int", v)
		}
	}()
	return &val, err
}
