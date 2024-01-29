package wwauth

import (
	"context"
	"github.com/golang-jwt/jwt/v5"
	"github.com/lestrrat-go/jwx/jwk"
	"github.com/pkg/errors"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"net/http"
	"strings"
	"time"
)

var JwtCtxKey = &contextKey{"jwt"}

type contextKey struct {
	name string
}

type JwtAuthOpt struct {
	Jwks     jwk.Set
	Issuer   string
	Audience string
	// DANGER: It is very important for newClaims to return a fresh claims pointer,
	// otherwise all requests will share the same JWT claims pointer!
	NewClaims func() jwt.Claims
}

type JwtAuth struct {
	log zerolog.Logger
	JwtAuthOpt
}

// NewJwtAuth
// DANGER: It is very important for newClaims to return a fresh claims pointer,
// otherwise all requests will share the same JWT claims pointer!
func NewJwtAuth(log zerolog.Logger, opt JwtAuthOpt) *JwtAuth {
	return &JwtAuth{
		log:        log,
		JwtAuthOpt: opt,
	}
}

func (auth *JwtAuth) JwtMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check for token.
		tokenStr := TokenFromHeader(r)
		if tokenStr == "" {
			log.Trace().Msg("No JWT found")
			next.ServeHTTP(w, r)
			return
		}

		// Parse the token.
		token, err := auth.ParseJwt(tokenStr)
		if err != nil {
			log.Error().Err(err).Stack().Msg("JWT error")
			if errors.Is(err, jwt.ErrTokenExpired) {
				http.Error(w, "JWT has expired", http.StatusUnauthorized)
			}
			http.Error(w, "JWT error", http.StatusUnauthorized)
			return
		}

		// Add to context.
		next.ServeHTTP(w, r.WithContext(ContextWithJwt(r.Context(), token)))
	})
}

func (auth *JwtAuth) ParseJwt(tokenStr string) (*jwt.Token, error) {
	// Parse the token.
	claims := auth.NewClaims()
	parser := jwt.NewParser(
		jwt.WithIssuer(auth.Issuer),
		jwt.WithAudience(auth.Audience),
		jwt.WithLeeway(10*time.Second),
	)
	token, err := parser.ParseWithClaims(tokenStr, claims, func(token *jwt.Token) (interface{}, error) {
		kid, ok := token.Header["kid"].(string)
		if !ok {
			return nil, errors.New("kid header not found in jwt")
		}
		key, ok := auth.Jwks.LookupKeyID(kid)
		if !ok {
			return nil, errors.Errorf("key %v not found in jwks", kid)
		}
		if key.Algorithm() != "" && token.Method.Alg() != key.Algorithm() {
			return nil, errors.Errorf("Invalid jwt method: %s (expected %s)", token.Method.Alg(), key.Algorithm())
		}

		var raw interface{}
		return raw, key.Raw(&raw)
	})
	if err != nil {
		return nil, err
	}

	// Check the validity of the token.
	if !token.Valid {
		return nil, errors.Errorf("Invalid token")
	}

	return token, nil
}

func TokenFromHeader(r *http.Request) string {
	bearer := r.Header.Get("Authorization")
	if len(bearer) > 7 && strings.ToUpper(bearer[0:6]) == "BEARER" {
		return bearer[7:]
	}
	return ""
}

func ContextWithJwt(ctx context.Context, jwt interface{}) context.Context {
	return context.WithValue(ctx, JwtCtxKey, jwt)
}

func JwtFromContext(ctx context.Context) string {
	return ctx.Value(JwtCtxKey).(string)
}

func JwtMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := ContextWithJwt(r.Context(), TokenFromHeader(r))
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
