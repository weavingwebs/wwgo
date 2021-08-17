package wwgraphql

import (
	"context"
	"fmt"
	"github.com/99designs/gqlgen/graphql"
	"github.com/99designs/gqlgen/graphql/errcode"
	"github.com/99designs/gqlgen/graphql/handler"
	"github.com/99designs/gqlgen/graphql/handler/extension"
	"github.com/99designs/gqlgen/graphql/handler/transport"
	"github.com/pkg/errors"
	"github.com/rs/zerolog"
	"github.com/vektah/gqlparser/v2/gqlerror"
	"github.com/weavingwebs/wwgo"
	"regexp"
	"strings"
	"time"
)

var MB int64 = 1 << 20

var cognitoEmailRegexp = regexp.MustCompile(`[\p{L}\p{M}\p{S}\p{N}\p{P}]+`)
var emailRegexp = regexp.MustCompile(`^[^\s]+@[^\s]+\.[^\s]+$`)

func NewGraphQlServer(es graphql.ExecutableSchema, log zerolog.Logger) *handler.Server {
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

	srv.Use(extension.Introspection{})

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
}

func ValidateStringDirective(ctx context.Context, obj interface{}, next graphql.Resolver, rules ValidateStringRules) (res interface{}, err error) {
	values, ok := obj.(map[string]interface{})
	if !ok {
		// @todo gql internal error
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
		}
	}

	return next(ctx)
}
