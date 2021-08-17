package scalars

import (
	"github.com/pkg/errors"
	"github.com/weavingwebs/wwgo"
	"io"
	"time"
)

const GqlDateFormat = "2006-01-02"

type GqlDate time.Time

func (d GqlDate) Time() time.Time {
	return time.Time(d)
}

func (d GqlDate) String() string {
	return d.Time().Format(GqlDateFormat)
}

func (d *GqlDate) UnmarshalGQL(v interface{}) error {
	if v == nil {
		return nil
	}
	str, ok := v.(string)
	if !ok {
		return errors.Errorf("invalid type %T, expected string", v)
	}

	t, err := time.Parse(GqlDateFormat, str)
	if err != nil {
		return wwgo.NewClientError("GQL_DATE_PARSE_EXCEPTION", "Invalid Date", err)
	}
	*d = GqlDate(t)
	return nil
}

func (d GqlDate) MarshalGQL(w io.Writer) {
	if d.Time().IsZero() {
		_, _ = w.Write([]byte("null"))
		return
	}
	_, _ = w.Write([]byte(`"`))
	_, _ = w.Write([]byte(d.String()))
	_, _ = w.Write([]byte(`"`))
}
