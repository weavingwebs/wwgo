package wwgraphql

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/99designs/gqlgen/graphql"
	"github.com/99designs/gqlgen/graphql/errcode"
	"github.com/99designs/gqlgen/graphql/handler"
	"github.com/99designs/gqlgen/graphql/handler/extension"
	"github.com/99designs/gqlgen/graphql/handler/transport"
	"github.com/pkg/errors"
	"github.com/rs/zerolog"
	"github.com/shopspring/decimal"
	"github.com/vektah/gqlparser/v2/gqlerror"
	"github.com/weavingwebs/wwgo"
	"github.com/weavingwebs/wwgo/wwgraphql/scalars"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var MB int64 = 1 << 20

var cognitoEmailRegexp = regexp.MustCompile(`[\p{L}\p{M}\p{S}\p{N}\p{P}]+`)
var emailRegexp = regexp.MustCompile(`^[^\s]+@[^\s]+\.[^\s]+$`)

func NewGraphQlServer(es graphql.ExecutableSchema, log zerolog.Logger, enableIntrospection bool) *handler.Server {
	srv := handler.New(es)

	srv.AddTransport(transport.Websocket{
		KeepAlivePingInterval: 10 * time.Second,
	})
	srv.AddTransport(transport.Options{})
	srv.AddTransport(transport.GET{})
	srv.AddTransport(transport.POST{})
	srv.AddTransport(transport.MultipartForm{
		MaxMemory:     64 * MB,
		MaxUploadSize: 16 * MB,
	})

	if enableIntrospection {
		srv.Use(extension.Introspection{})
	}

	srv.SetErrorPresenter(DefaultErrorPresenter(log))

	return srv
}

func DefaultErrorPresenter(log zerolog.Logger) func(ctx context.Context, e error) *gqlerror.Error {
	return func(ctx context.Context, e error) *gqlerror.Error {
		// Always log all errors, if it's wrapped by GQL then log the original.
		logEvt := log.Error().Stack()
		gqlErr, ok := e.(*gqlerror.Error)
		var originalErr error
		if ok {
			originalErr = gqlErr.Unwrap()
		}
		if originalErr != nil {
			logEvt.Err(originalErr).Str("gqlPath", gqlErr.Path.String())
		} else {
			logEvt.Err(e)
		}
		logEvt.Send()

		// Create the GQL response
		errResp := graphql.DefaultErrorPresenter(ctx, e)
		if errResp.Extensions == nil {
			errResp.Extensions = map[string]interface{}{}
		}

		// Check if it is a client error.
		var clientErr *wwgo.ClientError
		if errors.As(e, &clientErr) {
			if clientErr.GqlErrorCode() != "" {
				errResp.Extensions["code"] = clientErr.GqlErrorCode()
			}
			// Use the message from the client error directly, it may have been
			// wrapped by another error that is not client safe.
			errResp.Message = clientErr.Error()

			// If it wrapped an error, log it as well.
			if wrappedErr := clientErr.Unwrap(); wrappedErr != nil {
				log.Error().Stack().Err(wrappedErr).Send()
			}
		} else if errResp.Extensions["code"] != errcode.ValidationFailed && errResp.Extensions["code"] != errcode.ParseFailed {
			// If the error is not a ClientError or GQL validation, obfuscate it.
			errResp.Message = "An unexpected error occurred, please try again later"
			errResp.Extensions["code"] = 500
		}

		return errResp
	}
}

// @todo figure out how to put validation directives into a gqlgen plugin?

type ValidateStringRules struct {
	MinLength *int    `json:"minLength"`
	MaxLength *int    `json:"maxLength"`
	Pattern   *string `json:"pattern"`
	RegExp    *string `json:"regExp"`
}

func ValidateStringDirective(ctx context.Context, obj interface{}, next graphql.Resolver, rules ValidateStringRules) (res interface{}, err error) {
	values, ok := obj.(map[string]interface{})
	if !ok {
		return nil, errors.Wrapf(err, "obj is an unexpected type: %T", obj)
	}

	// Get value.
	fieldName := *graphql.GetPathContext(ctx).Field
	value, ok := values[fieldName]
	if !ok {
		// Do nothing if no value.
		return next(ctx)
	}
	var str string
	switch s := value.(type) {
	case nil:
		return next(ctx)

	case string:
		str = s

	case *string:
		if s == nil {
			// Ignore null.
			return next(ctx)
		}
		str = *s

	default:
		return nil, errors.Errorf("Invalid type for %s: %T", fieldName, value)
	}

	// Validate.
	if rules.MinLength != nil && len(str) < *rules.MinLength {
		return nil, wwgo.NewClientError(
			"VALIDATE_STRING_MIN_LENGTH_EXCEPTION",
			fmt.Sprintf("Cannot be less than %d %s", *rules.MinLength, wwgo.Plural(*rules.MinLength, "character", "characters")),
			nil,
		)
	}
	if rules.MaxLength != nil && len(str) > *rules.MaxLength {
		return nil, wwgo.NewClientError(
			"VALIDATE_STRING_MAX_LENGTH_EXCEPTION",
			fmt.Sprintf("Cannot be more than %d %s", *rules.MaxLength, wwgo.Plural(*rules.MaxLength, "character", "characters")),
			nil,
		)
	}
	if str != "" && rules.Pattern != nil {
		switch strings.ToUpper(*rules.Pattern) {
		case "EMAIL":
			if !cognitoEmailRegexp.MatchString(str) {
				return nil, wwgo.NewClientError(
					"VALIDATE_STRING_PATTERN_EMAIL_CHARS_EXCEPTION",
					"Email contains invalid characters",
					nil,
				)
			} else if !emailRegexp.MatchString(str) {
				return nil, wwgo.NewClientError(
					"VALIDATE_STRING_PATTERN_EMAIL_FORMAT_EXCEPTION",
					"Please enter a valid email address",
					nil,
				)
			}

		case "REGEX":
			if rules.RegExp == nil || *rules.RegExp == "" {
				return nil, errors.New("RegExp is required when Pattern is set to 'REGEX'")
			}
			exp, err := regexp.Compile(*rules.RegExp)
			if err != nil {
				return nil, errors.Wrap(err, "Invalid RegExp")
			}
			if !exp.MatchString(str) {
				return nil, wwgo.NewClientError(
					"VALIDATE_STRING_PATTERN_REGEXP_EXCEPTION",
					"Invalid format",
					nil,
				)
			}
		}
	}

	return next(ctx)
}

type ValidateDateRules struct {
	BeforeDate     *scalars.GqlDate      `json:"beforeDate"`
	BeforeRelative *ValidateDateRelative `json:"beforeRelative"`
	AfterDate      *scalars.GqlDate      `json:"afterDate"`
	AfterRelative  *ValidateDateRelative `json:"afterRelative"`
}

type ValidateDateRelative struct {
	Years  int `json:"years"`
	Months int `json:"months"`
	Days   int `json:"days"`
}

func ValidateDateDirective(ctx context.Context, obj interface{}, next graphql.Resolver, rules ValidateDateRules) (res interface{}, err error) {
	values, ok := obj.(map[string]interface{})
	if !ok {
		return nil, errors.Wrapf(err, "obj is an unexpected type: %T", obj)
	}

	// Get value.
	fieldName := *graphql.GetPathContext(ctx).Field
	value, ok := values[fieldName]
	if !ok {
		// Do nothing if no value.
		return next(ctx)
	}
	var date time.Time
	switch v := value.(type) {
	case nil:
		return next(ctx)

	case time.Time:
		date = v

	case *time.Time:
		if v == nil {
			// Ignore null.
			return next(ctx)
		}
		date = *v

	case scalars.GqlDate:
		date = time.Time(v)

	case *scalars.GqlDate:
		if v == nil {
			// Ignore null.
			return next(ctx)
		}
		date = time.Time(*v)

	default:
		return nil, errors.Errorf("Invalid type for %s: %T", fieldName, value)
	}

	// Validate.
	if rules.BeforeDate != nil && !date.Before(rules.BeforeDate.Time()) {
		return nil, wwgo.NewClientError(
			"VALIDATE_DATE_BEFORE_DATE_EXCEPTION",
			fmt.Sprintf("Must be before %s", rules.BeforeDate.Time().Format(scalars.GqlDateFormat)),
			nil,
		)
	}
	if rules.BeforeRelative != nil {
		d := time.Now().AddDate(rules.BeforeRelative.Years, rules.BeforeRelative.Months, rules.BeforeRelative.Days)
		if !date.Before(d) {
			return nil, wwgo.NewClientError(
				"VALIDATE_DATE_BEFORE_RELATIVE_EXCEPTION",
				fmt.Sprintf("Must be before %s", d.Format(scalars.GqlDateFormat)),
				nil,
			)
		}
	}
	if rules.AfterDate != nil && !date.After(rules.AfterDate.Time()) {
		return nil, wwgo.NewClientError(
			"VALIDATE_DATE_After_DATE_EXCEPTION",
			fmt.Sprintf("Must be after %s", rules.AfterDate.Time().Format(scalars.GqlDateFormat)),
			nil,
		)
	}
	if rules.AfterRelative != nil {
		d := time.Now().AddDate(rules.AfterRelative.Years, rules.AfterRelative.Months, rules.AfterRelative.Days)
		if !date.After(d) {
			return nil, wwgo.NewClientError(
				"VALIDATE_DATE_AFTER_RELATIVE_EXCEPTION",
				fmt.Sprintf("Must be after %s", d.Format(scalars.GqlDateFormat)),
				nil,
			)
		}
	}

	return next(ctx)
}

type ValidateDecimalRules struct {
	Min *decimal.Decimal `json:"min"`
	Max *decimal.Decimal `json:"max"`
}

func ValidateDecimalDirective(ctx context.Context, obj interface{}, next graphql.Resolver, rules ValidateDecimalRules) (res interface{}, err error) {
	values, ok := obj.(map[string]interface{})
	if !ok {
		return nil, errors.Wrapf(err, "obj is an unexpected type: %T", obj)
	}

	// Get value.
	fieldName := *graphql.GetPathContext(ctx).Field
	rawValue, ok := values[fieldName]
	if !ok {
		// Do nothing if no value.
		return next(ctx)
	}
	var value decimal.Decimal
	switch v := rawValue.(type) {
	case nil:
		return next(ctx)

	case int:
		value = decimal.New(int64(v), 0)
	case *int:
		if v == nil {
			// Ignore null.
			return next(ctx)
		}
		value = decimal.New(int64(*v), 0)

	case int32:
		value = decimal.New(int64(v), 0)
	case *int32:
		if v == nil {
			// Ignore null.
			return next(ctx)
		}
		value = decimal.New(int64(*v), 0)

	case int64:
		value = decimal.New(v, 0)
	case *int64:
		if v == nil {
			// Ignore null.
			return next(ctx)
		}
		value = decimal.New(*v, 0)

	case string:
		value, err = decimal.NewFromString(v)
		if err != nil {
			return nil, wwgo.NewClientError("VALIDATE_DECIMAL_INVALID_EXCEPTION", "Invalid decimal", nil)
		}
	case *string:
		if v == nil {
			// Ignore null.
			return next(ctx)
		}
		value, err = decimal.NewFromString(*v)
		if err != nil {
			return nil, wwgo.NewClientError("VALIDATE_DECIMAL_INVALID_EXCEPTION", "Invalid decimal", nil)
		}

	case json.Number:
		value, err = decimal.NewFromString(v.String())
		if err != nil {
			return nil, wwgo.NewClientError("VALIDATE_DECIMAL_INVALID_EXCEPTION", "Invalid decimal", nil)
		}

	case decimal.Decimal:
		value = v
	case *decimal.Decimal:
		if v == nil {
			// Ignore null.
			return next(ctx)
		}
		value = *v

	default:
		return nil, errors.Errorf("Invalid type for %s: %T", fieldName, rawValue)
	}

	// Validate.
	if rules.Min != nil && value.LessThan(*rules.Min) {
		return nil, wwgo.NewClientError(
			"VALIDATE_DECIMAL_MIN_EXCEPTION",
			fmt.Sprintf("Must be at least %s", rules.Min.String()),
			nil,
		)
	}
	if rules.Max != nil && value.GreaterThan(*rules.Max) {
		return nil, wwgo.NewClientError(
			"VALIDATE_DECIMAL_MAX_EXCEPTION",
			fmt.Sprintf("Must be no more than %s", rules.Max.String()),
			nil,
		)
	}

	return next(ctx)
}

type ValidateIntRules struct {
	Min *int `json:"min"`
	Max *int `json:"max"`
}

func ValidateIntDirective(ctx context.Context, obj interface{}, next graphql.Resolver, rules ValidateIntRules) (res interface{}, err error) {
	values, ok := obj.(map[string]interface{})
	if !ok {
		return nil, errors.Wrapf(err, "obj is an unexpected type: %T", obj)
	}

	// Get value.
	fieldName := *graphql.GetPathContext(ctx).Field
	rawValue, ok := values[fieldName]
	if !ok {
		// Do nothing if no value.
		return next(ctx)
	}
	var value int
	switch v := rawValue.(type) {
	case nil:
		return next(ctx)

	case int:
		value = v
	case *int:
		if v == nil {
			// Ignore null.
			return next(ctx)
		}
		value = *v

	case int32:
		value = int(v)
	case *int32:
		if v == nil {
			// Ignore null.
			return next(ctx)
		}
		value = int(*v)

	case int64:
		value = int(v)
	case *int64:
		if v == nil {
			// Ignore null.
			return next(ctx)
		}
		value = int(*v)

	case string:
		value, err = strconv.Atoi(v)
		if err != nil {
			return nil, wwgo.NewClientError("VALIDATE_INT_INVALID_EXCEPTION", "Invalid integer", nil)
		}
	case *string:
		if v == nil {
			// Ignore null.
			return next(ctx)
		}
		value, err = strconv.Atoi(*v)
		if err != nil {
			return nil, wwgo.NewClientError("VALIDATE_INT_INVALID_EXCEPTION", "Invalid integer", nil)
		}

	case json.Number:
		value, err = strconv.Atoi(v.String())
		if err != nil {
			return nil, wwgo.NewClientError("VALIDATE_INT_INVALID_EXCEPTION", "Invalid integer", nil)
		}

	default:
		return nil, errors.Errorf("Invalid type for %s: %T", fieldName, rawValue)
	}

	// Validate.
	if rules.Min != nil && value < *rules.Min {
		return nil, wwgo.NewClientError(
			"VALIDATE_INT_MIN_EXCEPTION",
			fmt.Sprintf("Must be at least %d", *rules.Min),
			nil,
		)
	}
	if rules.Max != nil && value > *rules.Max {
		return nil, wwgo.NewClientError(
			"VALIDATE_INT_MAX_EXCEPTION",
			fmt.Sprintf("Must be no more than %d", *rules.Max),
			nil,
		)
	}

	return next(ctx)
}
