package wwauth

import (
	"context"
	"encoding/json"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/lestrrat-go/jwx/v2/jwk"
	"github.com/pkg/errors"
	"github.com/rs/zerolog"
	"github.com/weavingwebs/wwgo"
	"net/http"
	"strings"
	"time"
)

type EntraAuth struct {
	*JwtAuth
	EntraPublicSettings
}

type EntraPublicSettings struct {
	TenantId string `json:"tenantId"`
	ClientId string `json:"clientId"`
}

type NewEntraAuthInput struct {
	Log zerolog.Logger
	EntraPublicSettings
	// Usually an API scope i.e. "api://<client-id>/my-api"
	Audience string
	// MSAL.js seems to use v1.0, v2.0 is supposed to be more standards compliant
	// [citation needed].
	Version string
}

func NewEntraAuth(ctx context.Context, input NewEntraAuthInput) (*EntraAuth, error) {
	if input.Audience == "" {
		return nil, errors.Errorf("audience is required")
	}
	oidcUrl := "https://login.microsoftonline.com/" + input.TenantId + "/"
	if input.Version != "" && input.Version != "v1.0" {
		oidcUrl += input.Version + "/"
	}
	oidcUrl += ".well-known/openid-configuration"

	config, err := getOidcConfig(ctx, oidcUrl)
	if err != nil {
		return nil, errors.Wrapf(err, "error getting OIDC config")
	}
	input.Log.Info().Msgf("Downloaded OIDC config from %s", oidcUrl)

	jwks := jwk.NewCache(ctx)

	// Tell *jwk.Cache that we only want to refresh this JWKS
	// when it needs to (based on Cache-Control or Expires header from
	// the HTTP response). If the calculated minimum refresh interval is less
	// than 15 minutes, don't go refreshing any earlier than 15 minutes.
	if err := jwks.Register(config.JwksUri, jwk.WithMinRefreshInterval(15*time.Minute)); err != nil {
		return nil, errors.Wrapf(err, "error registering JWKs from %s", config.JwksUri)
	}

	// Refresh now to ensure it's working.
	if _, err := jwks.Refresh(ctx, config.JwksUri); err != nil {
		return nil, errors.Wrapf(err, "error downloading JWKs from %s", config.JwksUri)
	}
	input.Log.Info().Msgf("Downloaded JWKs from %s", config.JwksUri)

	jwtAuth := NewJwtAuth(input.Log, JwtAuthOpt{
		Jwks:     jwks,
		Issuer:   "https://sts.windows.net/" + input.TenantId + "/",
		Audience: input.Audience,
		NewClaims: func() jwt.Claims {
			return &EntraClaims{}
		},
	})

	return &EntraAuth{
		JwtAuth:             jwtAuth,
		EntraPublicSettings: input.EntraPublicSettings,
	}, nil
}

type EntraClaims struct {
	Email string   `json:"email"`
	Name  string   `json:"name"`
	Oid   string   `json:"oid"`
	Roles []string `json:"roles"`
	jwt.RegisteredClaims
}

func (user EntraClaims) HasRole(role string) bool {
	return hasRole(user.Roles, role)
}

func (user EntraClaims) UserId() uuid.UUID {
	return uuid.MustParse(user.Oid)
}

func hasRole(haystack []string, needle string) bool {
	return wwgo.SliceIncludes(wwgo.MapSlice(haystack, strings.ToUpper), strings.ToUpper(needle))
}

// OidcConfig i.e. https://login.microsoftonline.com/common/v2.0/.well-known/openid-configuration
type OidcConfig struct {
	JwksUri string `json:"jwks_uri"`
	Issuer  string `json:"issuer"`
}

func getOidcConfig(ctx context.Context, oidcConfigUrl string) (*OidcConfig, error) {
	// Download the OIDC config.
	client := &http.Client{
		Timeout: 5 * time.Second,
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, oidcConfigUrl, nil)
	if err != nil {
		return nil, errors.Wrapf(err, "error creating request for %s", oidcConfigUrl)
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, errors.Wrapf(err, "error downloading %s", oidcConfigUrl)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, errors.Errorf("error downloading %s: %s", oidcConfigUrl, resp.Status)
	}
	o := &OidcConfig{}
	if err := json.NewDecoder(resp.Body).Decode(o); err != nil {
		return nil, errors.Wrapf(err, "error decoding %s", oidcConfigUrl)
	}
	return o, nil
}
