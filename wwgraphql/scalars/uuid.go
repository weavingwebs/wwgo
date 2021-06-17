package scalars

import (
	"fmt"
	"github.com/99designs/gqlgen/graphql"
	"github.com/google/uuid"
	"io"
)

func MarshalUUIDScalar(id *uuid.UUID) graphql.Marshaler {
	return graphql.WriterFunc(func(w io.Writer) {
		if id == nil {
			_, _ = w.Write([]byte("null"))
		}
		b, _ := id.MarshalText()
		_, _ = w.Write([]byte(`"`))
		_, _ = w.Write(b)
		_, _ = w.Write([]byte(`"`))
	})
}

func UnmarshalUUIDScalar(v interface{}) (*uuid.UUID, error) {
	if v == nil {
		return nil, nil
	}
	val, err := func() (uuid.UUID, error) {
		switch v := v.(type) {
		case string:
			return uuid.Parse(v)
		case []byte:
			return uuid.ParseBytes(v)
		default:
			return uuid.Nil, fmt.Errorf("%T is not a string", v)
		}
	}()
	return &val, err
}
