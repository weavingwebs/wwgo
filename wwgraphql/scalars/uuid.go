package scalars

import (
	"fmt"
	"github.com/99designs/gqlgen/graphql"
	"github.com/google/uuid"
	"github.com/weavingwebs/wwgo"
	"io"
)

func MarshalUUIDScalar(id *uuid.UUID) graphql.Marshaler {
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

func UnmarshalUUIDScalar(v interface{}) (*uuid.UUID, error) {
	if v == nil {
		return nil, nil
	}
	val, err := func() (uuid.UUID, error) {
		switch v := v.(type) {
		case string:
			res, err := uuid.Parse(v)
			if err != nil {
				return uuid.Nil, wwgo.NewClientError("GQL_UUID_PARSE_EXCEPTION", err.Error(), err)
			}
			return res, nil
		case []byte:
			res, err := uuid.ParseBytes(v)
			if err != nil {
				return uuid.Nil, wwgo.NewClientError("GQL_UUID_PARSE_EXCEPTION", err.Error(), err)
			}
			return res, nil
		default:
			return uuid.Nil, fmt.Errorf("%T is not a string", v)
		}
	}()
	return &val, err
}
