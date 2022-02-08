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

func (d *GqlDate) UnmarshalJSON(data []byte) error {
	// Ignore null, like in the main JSON package.
	if string(data) == "null" {
		return nil
	}
	t, err := time.Parse(`"`+GqlDateFormat+`"`, string(data))
	*d = GqlDate(t)
	return err
}

func (d GqlDate) MarshalJSON() ([]byte, error) {
	if d.Time().IsZero() {
		return []byte("null"), nil
	}
	if y := d.Time().Year(); y < 0 || y >= 10000 {
		return nil, errors.New("GqlDate.MarshalJSON: year outside of range [0,9999]")
	}

	b := make([]byte, 0, len(GqlDateFormat)+2)
	b = append(b, '"')
	b = d.Time().AppendFormat(b, GqlDateFormat)
	b = append(b, '"')
	return b, nil
}
