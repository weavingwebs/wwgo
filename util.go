package wwgo

import (
	"bytes"
	"crypto/rand"
	"database/sql"
	"fmt"
	"github.com/google/uuid"
	"github.com/pkg/errors"
	"github.com/shopspring/decimal"
	"io"
	"math/big"
	"strconv"
	"strings"
	"text/template"
	"time"
	"unicode/utf8"
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

// FormatDate supports ordinal days i.e. 'Monday 2nd January 2006 15:04:05'.
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
// Deprecated: use DiffSlice instead.
func ArrayDiffInt32(a []int32, b []int32) []int32 {
	return DiffSlice(a, b)
}

// ArrayDiffInt returns a slice of all values from a that are not in b.
// Deprecated: use DiffSlice instead.
func ArrayDiffInt(a []int, b []int) []int {
	return DiffSlice(a, b)
}

// ArrayDiffStr returns a slice of all values from a that are not in b.
// Deprecated: use DiffSlice instead.
func ArrayDiffStr(a []string, b []string) []string {
	return DiffSlice(a, b)
}

// ArrayDiffUuid returns a slice of all values from a that are not in b.
// Deprecated: use DiffSlice instead.
func ArrayDiffUuid(a []uuid.UUID, b []uuid.UUID) []uuid.UUID {
	return DiffSlice(a, b)
}

// ArrayDiffUuidRef returns a slice of all values from a that are not in b.
// Deprecated: use DiffSlice instead.
func ArrayDiffUuidRef(a []*uuid.UUID, b []*uuid.UUID) []*uuid.UUID {
	return DiffSlice(a, b)
}

// ArrayFilterFnStr returns a slice of all values from a that pass the filter function.
// Deprecated: use FilterSlice instead.
func ArrayFilterFnStr(a []string, fn func(v string) bool) []string {
	return FilterSlice(a, fn)
}

// ArrayFilterStr returns a slice of all values from a that are not empty.
func ArrayFilterStr(a []string) []string {
	return FilterSlice(a, func(v string) bool {
		return v != ""
	})
}

// ArrayMapStr returns a slice of all values from a mapped by fn().
// Deprecated: use MapSlice instead.
func ArrayMapStr(a []string, fn func(v string) string) []string {
	return MapSlice(a, fn)
}

// ArrayFilterAndJoinStr returns a string of all values from a that are not empty, joined by sep.
func ArrayFilterAndJoinStr(a []string, sep string) string {
	return strings.Join(ArrayFilterStr(MapSlice(a, strings.TrimSpace)), sep)
}

// ArrayIncludesInt returns true if the slice contains the value.
// Deprecated: use SliceIncludes instead.
func ArrayIncludesInt(haystack []int, needle int) bool {
	return SliceIncludes(haystack, needle)
}

// ArrayIncludesInt32 returns true if the slice contains the value.
// Deprecated: use SliceIncludes instead.
func ArrayIncludesInt32(haystack []int32, needle int32) bool {
	return SliceIncludes(haystack, needle)
}

// ArrayIncludesStr returns true if the slice contains the value.
// Deprecated: use SliceIncludes instead.
func ArrayIncludesStr(haystack []string, needle string) bool {
	return SliceIncludes(haystack, needle)
}

// ArrayIncludesUUID returns true if the slice contains the value.
// Deprecated: use SliceIncludes instead.
func ArrayIncludesUUID(haystack []uuid.UUID, needle uuid.UUID) bool {
	return SliceIncludes(haystack, needle)
}

func IntArray2StrArray(in []int) []string {
	out := make([]string, len(in))
	for i, v := range in {
		out[i] = strconv.Itoa(v)
	}
	return out
}

func JoinIntArray(in []int, sep string) string {
	return strings.Join(IntArray2StrArray(in), sep)
}

// UuidRef returns a fresh pointer for the value.
// Deprecated: use ToPtr instead.
func UuidRef(id uuid.UUID) *uuid.UUID {
	return ToPtr(id)
}

// StrRef returns a fresh pointer for the value.
// Deprecated: use ToPtr instead.
func StrRef(v string) *string {
	return ToPtr(v)
}

// StrFromRef returns the value of the pointer, or "" if nil.
func StrFromRef(v *string) string {
	if v == nil {
		return ""
	}
	return *v
}

// StrNilIfEmpty returns nil if the string is empty.
func StrNilIfEmpty(v string) *string {
	if v == "" {
		return nil
	}
	return &v
}

// BoolRef returns a fresh pointer for the value.
// Deprecated: use ToPtr instead.
func BoolRef(v bool) *bool {
	return ToPtr(v)
}

// IntRef returns a fresh pointer for the value.
// Deprecated: use ToPtr instead.
func IntRef(v int) *int {
	return ToPtr(v)
}

// IntFromRef returns the value of the pointer, or 0 if nil.
func IntFromRef(v *int) int {
	if v == nil {
		return 0
	}
	return *v
}

// Int32Ref returns a fresh pointer for the value.
// Deprecated: use ToPtr instead.
func Int32Ref(v int32) *int32 {
	return ToPtr(v)
}

// Int32FromRef returns the value of the pointer, or 0 if nil.
func Int32FromRef(v *int32) int32 {
	if v == nil {
		return 0
	}
	return *v
}

// Int64Ref returns a fresh pointer for the value.
// Deprecated: use ToPtr instead.
func Int64Ref(v int64) *int64 {
	return ToPtr(v)
}

// Int64FromRef returns the value of the pointer, or 0 if nil.
func Int64FromRef(v *int64) int64 {
	if v == nil {
		return 0
	}
	return *v
}

// TimeRef returns a fresh pointer for the value.
// Deprecated: use ToPtr instead.
func TimeRef(v time.Time) *time.Time {
	return ToPtr(v)
}

// TimeFromRef returns the value of the pointer, or the zero time if nil.
func TimeFromRef(v *time.Time) time.Time {
	if v == nil {
		return time.Time{}
	}
	return *v
}

// DecimalRef returns a fresh pointer for the value.
// Deprecated: use ToPtr instead.
func DecimalRef(v decimal.Decimal) *decimal.Decimal {
	return ToPtr(v)
}

// DecimalFromRef returns the value of the pointer, or the zero decimal if nil.
func DecimalFromRef(v *decimal.Decimal) decimal.Decimal {
	if v == nil {
		return decimal.Decimal{}
	}
	return *v
}

// SqlNullStr will return a sql 'null' value if the string is empty.
func SqlNullStr(v string) sql.NullString {
	return sql.NullString{String: v, Valid: v != ""}
}

// SqlNullStrRef will return a sql 'null' value if the pointer is nil.
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

// SqlNullIntRef will return a sql 'null' value if the pointer is nil.
func SqlNullIntRef(v *int) sql.NullInt32 {
	if v == nil {
		return sql.NullInt32{}
	}
	return sql.NullInt32{Int32: int32(*v), Valid: true}
}

// SqlNullTime will return a sql 'null' value if the time is 0.
func SqlNullTime(v time.Time) sql.NullTime {
	return sql.NullTime{Time: v, Valid: !v.IsZero()}
}

// SqlNullTimeRef will return a sql 'null' value if the pointer is nil.
func SqlNullTimeRef(v *time.Time) sql.NullTime {
	if v == nil {
		return sql.NullTime{}
	}
	return sql.NullTime{Time: *v, Valid: true}
}

// SqlNullBoolRef will return a sql 'null' value if the pointer is nil.
func SqlNullBoolRef(v *bool) sql.NullBool {
	if v == nil {
		return sql.NullBool{}
	}
	return sql.NullBool{Bool: *v, Valid: true}
}

// SqlNullUuid will return a sql 'null' value if the uuid is empty.
func SqlNullUuid(v uuid.UUID) uuid.NullUUID {
	return uuid.NullUUID{UUID: v, Valid: v != uuid.Nil}
}

// SqlNullUuidRef will return a sql 'null' value if the pointer is nil.
func SqlNullUuidRef(v *uuid.UUID) uuid.NullUUID {
	if v == nil {
		return uuid.NullUUID{}
	}
	return uuid.NullUUID{UUID: *v, Valid: true}
}

// UuidRefFromSql will return nil for a 'null' SQL value.
func UuidRefFromSql(v uuid.NullUUID) *uuid.UUID {
	if !v.Valid {
		return nil
	}
	return ToPtr(v.UUID)
}

// SqlNullDecimal will return a sql 'null' value if the decimal is zero.
func SqlNullDecimal(v decimal.Decimal) decimal.NullDecimal {
	return decimal.NullDecimal{Decimal: v, Valid: !v.IsZero()}
}

// SqlNullDecimalRef will return a sql 'null' value if the pointer is nil.
func SqlNullDecimalRef(v *decimal.Decimal) decimal.NullDecimal {
	if v == nil {
		return decimal.NullDecimal{}
	}
	return decimal.NullDecimal{Decimal: *v, Valid: true}
}

// StrRefFromSql will return nil for a 'null' SQL value.
func StrRefFromSql(v sql.NullString) *string {
	if !v.Valid {
		return nil
	}
	return ToPtr(v.String)
}

// IntRefFromSql will return nil for a 'null' SQL value.
func IntRefFromSql(v sql.NullInt32) *int {
	if !v.Valid {
		return nil
	}
	return ToPtr(int(v.Int32))
}

// DecimalRefFromSql will return nil for a 'null' SQL value.
func DecimalRefFromSql(v decimal.NullDecimal) *decimal.Decimal {
	if !v.Valid {
		return nil
	}
	return ToPtr(v.Decimal)
}

// TimeRefFromSql will return nil for a 'null' SQL value.
func TimeRefFromSql(v sql.NullTime) *time.Time {
	if !v.Valid {
		return nil
	}
	return ToPtr(v.Time)
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
	return ToPtr(GqlTime(v.Time))
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
	return ToPtr(t)
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
		res[i] = ToPtr(s)
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

func Plural(count int, singular string, plural string) string {
	if count == 1 {
		return singular
	}
	return plural
}

// TruncateStr multibyte/UTF-8 safe truncate to a maximum number of characters.
func TruncateStr(str string, maxLen int) string {
	if len([]rune(str)) < maxLen {
		return str
	}
	return string([]rune(str)[:maxLen])
}

// TruncateStrBytes multibyte/UTF-8 safe truncate to a maximum number of bytes.
func TruncateStrBytes(str string, maxBytes int) string {
	end := 0
	for i, r := range []rune(str) {
		newEnd := utf8.RuneLen(r) + i
		if newEnd <= maxBytes {
			end = newEnd
		} else {
			break
		}
	}
	return string([]byte(str)[:end])
}

// ScreamingSnakeCaseToHuman converts HUMAN_READABLE to 'human readable'.
func ScreamingSnakeCaseToHuman(s string) string {
	return strings.Join(MapSlice(strings.Split(s, "_"), func(v string) string {
		if len(v) == 0 {
			return ""
		}
		if len(v) == 1 {
			return strings.ToUpper(v)
		}
		return strings.ToUpper(v[0:1]) + strings.ToLower(v[1:])
	}), " ")
}

// ToPtr returns a fresh pointer for the value.
func ToPtr[T any](v T) *T {
	return &v
}

// DerefPtrSlice converts a slice of pointers to a slice of values.
func DerefPtrSlice[T any](s []*T) []T {
	res := make([]T, len(s))
	for i, v := range s {
		res[i] = *v
	}
	return res
}

// SliceToPtrSlice converts a slice of values to a slice of pointers.
func SliceToPtrSlice[T any](s []T) []*T {
	if s == nil {
		return nil
	}
	res := make([]*T, len(s))
	for i, v := range s {
		res[i] = ToPtr(v)
	}
	return res
}

// DerefPtrSliceSlice converts a slice of slices of pointers to a slice of slices of values.
func DerefPtrSliceSlice[T any](s [][]*T) [][]T {
	res := make([][]T, len(s))
	for i, v := range s {
		res[i] = DerefPtrSlice(v)
	}
	return res
}

// MapSlice returns a slice of all values from s mapped by f().
func MapSlice[IN any, OUT any](s []IN, f func(IN) OUT) []OUT {
	res := make([]OUT, len(s))
	for i, v := range s {
		res[i] = f(v)
	}
	return res
}

// SliceIncludes returns true if the slice contains the value.
func SliceIncludes[T comparable](s []T, v T) bool {
	for _, vv := range s {
		if vv == v {
			return true
		}
	}
	return false
}

// FilterSlice returns a slice of all values from s that pass the filter function.
func FilterSlice[T any](s []T, f func(T) bool) []T {
	res := make([]T, 0, len(s))
	for _, v := range s {
		if f(v) {
			res = append(res, v)
		}
	}
	return res
}

// DiffSlice returns a slice of all values from a that are not in b.
func DiffSlice[T comparable](a []T, b []T) []T {
	diff := make([]T, 0)
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

// SplitMapAndFilterString splits a string, applies mapping function(s) and
// filters out empty strings.
// i.e. SplitTrimAndFilterString("test1, test2", ",", strings.TrimSpace, strings.ToUpper)
func SplitMapAndFilterString(in string, sep string, mapFns ...func(in string) string) []string {
	var res []string
	for _, s := range strings.Split(in, sep) {
		for _, mapFn := range mapFns {
			s = mapFn(s)
		}
		if s != "" {
			res = append(res, s)
		}
	}
	return res
}

type StringFn = func(in string) string

// SplitTrimAndFilterString splits a string, trims each part and filters out
// empty strings.
func SplitTrimAndFilterString(in string, sep string, mapFns ...StringFn) []string {
	mapFns = append([]StringFn{strings.TrimSpace}, mapFns...)
	return SplitMapAndFilterString(in, sep, mapFns...)
}

// IfThenElse is a ternary helper function.
func IfThenElse[T any](cond bool, t T, f T) T {
	if cond {
		return t
	}
	return f
}

func ToStringSlice[T fmt.Stringer](input []T) []string {
	return MapSlice(input, func(v T) string {
		return v.String()
	})
}

func JoinStringers[T fmt.Stringer](items []T, sep string) string {
	return strings.Join(ToStringSlice(items), sep)
}
