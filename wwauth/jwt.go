package wwauth

import (
	"context"
	"github.com/golang-jwt/jwt/v4"
	"github.com/lestrrat-go/jwx/jwk"
	"github.com/pkg/errors"
	"github.com/rs/zerolog"
	"net/http"
	"strings"
	"time"
)

var JwtCtxKey = &contextKey{"jwt"}

type contextKey struct {
	name string
}

type Claims interface {
	Valid() error
	VerifyIssuer() bool
	VerifyAudience() bool
	GetStandardClaims() jwt.RegisteredClaims
}

// JwtMiddleware
//
// DANGER: It is very important for newClaims to return a fresh claims pointer,
// otherwise all requests will share the same JWT claims pointer!
func JwtMiddleware(
	jwks jwk.Set,
	log zerolog.Logger,
	newClaims func() Claims,
) func(http.Handler) http.Handler {
	// Manually allow for 10s clock drift to avoid IAT validation errors.
	// @todo Remove this once IAT validation has been removed.
	//   https://github.com/golang-jwt/jwt/issues/98
	jwt.TimeFunc = func() time.Time {
		return time.Now().UTC().Add(10 * time.Second)
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Check for token.
			tokenStr := TokenFromHeader(r)
			if tokenStr == "" {
				log.Trace().Msg("No JWT found")
				next.ServeHTTP(w, r)
				return
			}

			// Parse the token.
			token, err := ParseJwt(jwks, tokenStr, newClaims)
			if err != nil {
				if vErr, ok := err.(*jwt.ValidationError); ok {
					if vErr.Errors&jwt.ValidationErrorExpired > 0 {
						http.Error(w, "JWT has expired", 401)
					} else if vErr.Errors&jwt.ValidationErrorIssuedAt > 0 {
						http.Error(w, "JWT IAT validation failed", 401)
					} else if vErr.Errors&jwt.ValidationErrorNotValidYet > 0 {
						http.Error(w, "JWT is not valid yet", 401)
					} else if vErr.Errors&jwt.ValidationErrorNotValidYet > 0 {
					} else {
						http.Error(w, "JWT validation error", 401)
					}

					log.Error().Err(vErr).Stack().Msg("JWT validation error")
					return
				} else if errors.Is(err, jwt.ErrTokenInvalidIssuer) {
					http.Error(w, "Invalid JWT issuer", 401)
				} else if errors.Is(err, jwt.ErrTokenInvalidAudience) {
					http.Error(w, "Invalid JWT audience", 401)
				} else if errors.Is(err, jwt.ErrTokenInvalidClaims) {
					http.Error(w, "Invalid JWT claims", 401)
				} else {
					log.Error().Err(err).Stack().Msg("Generic JWT error")
				}
				http.Error(w, "JWT error", 500)
				return
			}

			// Add to context.
			next.ServeHTTP(w, r.WithContext(ContextWithJwt(r.Context(), token)))
		})
	}
}

// ParseJwt
//
// DANGER: It is very important for newClaims to return a fresh claims pointer,
// otherwise all requests will share the same JWT claims pointer!
func ParseJwt(jwks jwk.Set, tokenStr string, newClaims func() Claims) (*jwt.Token, error) {
	// Parse the token.
	claims := newClaims()
	token, err := jwt.ParseWithClaims(tokenStr, claims, func(token *jwt.Token) (interface{}, error) {
		kid, ok := token.Header["kid"].(string)
		if !ok {
			return nil, errors.New("kid header not found in jwt")
		}
		key, ok := jwks.LookupKeyID(kid)
		if !ok {
			return nil, errors.Errorf("key %v not found in jwks", kid)
		}
		if token.Method.Alg() != key.Algorithm() {
			return nil, errors.Errorf("Invalid jwt method: %s", token.Method.Alg())
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

	// Verify issuer.
	if tokenClaims, ok := token.Claims.(Claims); ok {
		if !tokenClaims.VerifyIssuer() {
			return nil, errors.Wrapf(jwt.ErrTokenInvalidIssuer, "Invalid JWT issuer: %s", tokenClaims.GetStandardClaims().Issuer)
		}
		if !tokenClaims.VerifyAudience() {
			return nil, errors.Wrapf(jwt.ErrTokenInvalidAudience, "Invalid JWT audience: %s", tokenClaims.GetStandardClaims().Audience)
		}
	} else {
		return nil, errors.Wrapf(jwt.ErrTokenInvalidClaims, "unknown type of Claims in jwt: %T", token.Claims)
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

func ContextWithJwt(ctx context.Context, token *jwt.Token) context.Context {
	return context.WithValue(ctx, JwtCtxKey, token)
}
