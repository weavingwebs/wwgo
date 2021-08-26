package scalars

import (
	"github.com/pkg/errors"
	"github.com/weavingwebs/wwgo"
	"io"
	"regexp"
)

var durationExp = regexp.MustCompile(`^\d+:\d+(:\d+)?$`)

type GqlDuration string

func (d GqlDuration) String() string {
	return string(d)
}

func (d GqlDuration) IsValid() bool {
	return durationExp.MatchString(string(d))
}

func (d *GqlDuration) UnmarshalGQL(v interface{}) error {
	if v == nil {
		return nil
	}
	str, ok := v.(string)
	if !ok {
		return errors.Errorf("invalid type %T, expected string", v)
	}

	t := GqlDuration(str)
	if !t.IsValid() {
		return wwgo.NewClientError("GQL_DURATION_PARSE_EXCEPTION", "Invalid duration", nil)
	}
	*d = t
	return nil
}

func (d GqlDuration) MarshalGQL(w io.Writer) {
	_, _ = w.Write([]byte(`"`))
	_, _ = w.Write([]byte(d.String()))
	_, _ = w.Write([]byte(`"`))
}
