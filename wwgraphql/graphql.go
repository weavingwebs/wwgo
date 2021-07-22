package wwgraphql

import (
	"context"
	"github.com/99designs/gqlgen/graphql"
	"github.com/99designs/gqlgen/graphql/handler"
	"github.com/99designs/gqlgen/graphql/handler/extension"
	"github.com/99designs/gqlgen/graphql/handler/transport"
	"github.com/pkg/errors"
	"github.com/weavingwebs/wwgo"
	"regexp"
	"strings"
	"time"
)

var MB int64 = 1 << 20

var cognitoEmailRegexp = regexp.MustCompile(`[\p{L}\p{M}\p{S}\p{N}\p{P}]+`)
var emailRegexp = regexp.MustCompile(`^[^\s]+@[^\s]+\.[^\s]+$`)

func NewGraphQlServer(es graphql.ExecutableSchema) *handler.Server {
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

	return srv
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
		// @todo gql error code.
		return nil, errors.Errorf("Cannot be less than %d %s", *rules.MinLength, wwgo.Plural(*rules.MinLength, "character", "characters"))
	}
	if rules.MaxLength != nil && len(str) > *rules.MaxLength {
		// @todo gql error code.
		return nil, errors.Errorf("Cannot be more than %d %s", *rules.MaxLength, wwgo.Plural(*rules.MaxLength, "character", "characters"))
	}
	if rules.Pattern != nil {
		switch strings.ToUpper(*rules.Pattern) {
		case "EMAIL":
			if !cognitoEmailRegexp.MatchString(str) {
				// @todo gql error code.
				return nil, errors.New("Email contains invalid characters")
			} else if !emailRegexp.MatchString(str) {
				// @todo gql error code.
				return nil, errors.New("Please enter a valid email address")
			}
		}
	}

	return next(ctx)
}
