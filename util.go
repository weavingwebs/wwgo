package wwgo

import (
	"bytes"
	"crypto/rand"
	"database/sql"
	"github.com/google/uuid"
	"github.com/pkg/errors"
	"io"
	"math/big"
	"strconv"
	"strings"
	"text/template"
	"time"
)

func GenerateRandomKey(length int) []byte {
	k := make([]byte, length)
	if _, err := io.ReadFull(rand.Reader, k); err != nil {
		panic(err)
	}
	return k
}

func GenerateRandomString(length int, charset []rune) string {
	res := make([]rune, length)
	max := big.NewInt(int64(len(charset) - 1))
	for i := 0; i < length; i++ {
		randInt, err := rand.Int(rand.Reader, max)
		if err != nil {
			panic(errors.Wrapf(err, "failed to generate random int"))
		}
		res[i] = charset[randInt.Int64()]
	}
	return string(res)
}

// FormatDate supports ordinal days.
func FormatDate(t time.Time, format string) string {
	if strings.Contains(format, "2nd") {
		suffix := "th"
		switch t.Day() {
		case 1, 21, 31:
			suffix = "st"
		case 2, 22:
			suffix = "nd"
		case 3, 23:
			suffix = "rd"
		}
		format = strings.ReplaceAll(format, "2nd", "2"+suffix)
	}
	return t.Format(format)
}

// ArrayDiffInt32 returns a slice of all values from a that are not in b.
func ArrayDiffInt32(a []int32, b []int32) []int32 {
	diff := make([]int32, 0)
	for _, aItem := range a {
		found := false
		for _, bItem := range b {
			if bItem == aItem {
				found = true
				break
			}
		}
		if !found {
			diff = append(diff, aItem)
		}
	}
	return diff
}

// ArrayDiffStr returns a slice of all values from a that are not in b.
func ArrayDiffStr(a []string, b []string) []string {
	diff := make([]string, 0)
	for _, aItem := range a {
		found := false
		for _, bItem := range b {
			if bItem == aItem {
				found = true
				break
			}
		}
		if !found {
			diff = append(diff, aItem)
		}
	}
	return diff
}

func ArrayFilterFnStr(a []string, fn func(v string) bool) []string {
	res := make([]string, 0)
	for _, v := range a {
		if fn(v) {
			res = append(res, v)
		}
	}
	return res
}

func ArrayFilterStr(a []string) []string {
	return ArrayFilterFnStr(a, func(v string) bool {
		return v != ""
	})
}

func ArrayMapStr(a []string, fn func(v string) string) []string {
	for i := range a {
		a[i] = fn(a[i])
	}
	return a
}

func ArrayFilterAndJoinStr(a []string, sep string) string {
	return strings.Join(ArrayFilterStr(ArrayMapStr(a, strings.TrimSpace)), sep)
}

func ArrayIncludesInt(haystack []int, needle int) bool {
	for _, v := range haystack {
		if v == needle {
			return true
		}
	}
	return false
}

func ArrayIncludesInt32(haystack []int32, needle int32) bool {
	for _, v := range haystack {
		if v == needle {
			return true
		}
	}
	return false
}

func ArrayIncludesStr(haystack []string, needle string) bool {
	for _, v := range haystack {
		if v == needle {
			return true
		}
	}
	return false
}

func ArrayIncludesUUID(haystack []uuid.UUID, needle uuid.UUID) bool {
	for _, v := range haystack {
		if v == needle {
			return true
		}
	}
	return false
}

func UuidRef(id uuid.UUID) *uuid.UUID {
	return &id
}

func StrRef(v string) *string {
	return &v
}

func StrFromRef(v *string) string {
	if v == nil {
		return ""
	}
	return *v
}

func BoolRef(v bool) *bool {
	return &v
}

// StrRefSql will return nil for a 'null' SQL string.
func StrRefSql(v sql.NullString) *string {
	if !v.Valid {
		return nil
	}
	return StrRef(v.String)
}

// IntRefSql will return nil for a 'null' SQL string.
func IntRefSql(v sql.NullInt32) *int {
	if !v.Valid {
		return nil
	}
	return IntRef(int(v.Int32))
}

func IntRef(v int) *int {
	return &v
}

func Int32Ref(v int32) *int32 {
	return &v
}

func Int64Ref(v int64) *int64 {
	return &v
}

func TimeRef(v time.Time) *time.Time {
	return &v
}

// SqlNullStr will return a sql 'null' value if the string is empty.
func SqlNullStr(v string) sql.NullString {
	return sql.NullString{String: v, Valid: v != ""}
}

// SqlNullStrRef will return a sql 'null' value if the string is nil.
func SqlNullStrRef(v *string) sql.NullString {
	if v == nil {
		return sql.NullString{}
	}
	return sql.NullString{String: *v, Valid: true}
}

// SqlNullInt32 will return a sql 'null' value if the value is 0.
func SqlNullInt32(v int32) sql.NullInt32 {
	return sql.NullInt32{Int32: v, Valid: v != 0}
}

// SqlNullTime will return a sql 'null' value if the time is 0.
func SqlNullTime(v time.Time) sql.NullTime {
	return sql.NullTime{Time: v, Valid: !v.IsZero()}
}

func SqlTinyIntFromBool(v bool) int8 {
	if v {
		return 1
	}
	return 0
}

func BoolFromSqlTinyInt(v int8) bool {
	if v < 1 {
		return false
	}
	return true
}

func GqlTime(v time.Time) string {
	return v.Format(time.RFC3339)
}

func GqlTimeRefSql(v sql.NullTime) *string {
	if !v.Valid {
		return nil
	}
	return StrRef(GqlTime(v.Time))
}

func TimeFromGqlTime(v string) (time.Time, error) {
	return time.Parse(time.RFC3339, v)
}

func TimeRefFromGqlTimeRef(v *string) *time.Time {
	if v == nil {
		return nil
	}
	t, err := time.Parse(time.RFC3339, *v)
	if err != nil {
		return nil
	}
	return TimeRef(t)
}

func SqlTimeToStrRef(v sql.NullTime) *string {
	if v.Valid {
		return StrRef(GqlTime(v.Time))
	}
	return nil
}

func StrSliceToStrRefSlice(v []string) []*string {
	res := make([]*string, len(v))
	for i, s := range v {
		res[i] = StrRef(s)
	}
	return res
}

func Int32MustParse(v string) int32 {
	vInt, err := strconv.ParseInt(v, 10, 32)
	if err != nil {
		panic(errors.Wrapf(err, "failed to parse int32 '%s'", v))
	}
	return int32(vInt)
}

func Int64MustParse(v string) int64 {
	vInt, err := strconv.ParseInt(v, 10, 64)
	if err != nil {
		panic(errors.Wrapf(err, "failed to parse int64 '%s'", v))
	}
	return vInt
}

func RenderTemplatePlain(tplStr string, data interface{}) (string, error) {
	tpl := template.New("RenderTemplatePlain")
	tpl, err := tpl.Parse(tplStr)
	if err != nil {
		return "", errors.Wrapf(err, "failed to compile template")
	}
	w := &bytes.Buffer{}
	if err := tpl.Execute(w, data); err != nil {
		return "", errors.Wrapf(err, "failed to execute template")
	}
	return w.String(), nil
}

func ArrayFillStr(value string, count int) []string {
	res := make([]string, count)
	for i := 0; i < count; i++ {
		res[i] = value
	}
	return res
}
