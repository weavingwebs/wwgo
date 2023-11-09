package wwauth

import (
	"context"
	"encoding/json"
	"github.com/golang-jwt/jwt/v5"
	"github.com/lestrrat-go/jwx/jwk"
	"github.com/pkg/errors"
	"github.com/rs/zerolog"
	"github.com/weavingwebs/wwgo"
	"net/http"
	"strings"
	"time"
)

type NewJwtAuthFromEntraInput struct {
	Log      zerolog.Logger
	TenantId string
	ClientId string
}

func NewJwtAuthFromEntra(ctx context.Context, input NewJwtAuthFromEntraInput) (*JwtAuth, error) {
	config, err := getOidcConfig(ctx, "https://login.microsoftonline.com/"+input.TenantId+"/v2.0/.well-known/openid-configuration")
	if err != nil {
		return nil, errors.Wrapf(err, "error getting OIDC config")
	}
	input.Log.Info().Msgf("Downloaded OIDC config from %s", "https://login.microsoftonline.com/"+input.TenantId+"/v2.0/.well-known/openid-configuration")

	jwks, err := jwk.Fetch(ctx, config.JwksUri)
	if err != nil {
		return nil, errors.Wrapf(err, "error downloading JWKs from %s", config.JwksUri)
	}
	input.Log.Info().Msgf("Downloaded JWKs from %s", config.JwksUri)

	return NewJwtAuth(input.Log, JwtAuthOpt{
		Jwks:     jwks,
		Issuer:   config.Issuer,
		Audience: input.ClientId,
		NewClaims: func() jwt.Claims {
			return &EntraClaims{}
		},
	}), nil
}

type EntraClaims struct {
	Email string   `json:"email"`
	Name  string   `json:"name"`
	Roles []string `json:"roles"`
	jwt.RegisteredClaims
}

func (user EntraClaims) HasRole(role string) bool {
	return hasRole(user.Roles, role)
}

func hasRole(haystack []string, needle string) bool {
	return wwgo.ArrayIncludesStr(wwgo.ArrayMapStr(haystack, strings.ToUpper), strings.ToUpper(needle))
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
