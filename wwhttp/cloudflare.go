package wwhttp

import (
	"context"
	"github.com/rs/zerolog"
	"net"
	"net/http"
	"time"
)

var cloudflareCtxKey = &contextKey{"cloudflare"}

type CloudflareContext struct {
	ClientIp         net.IP
	IsFromCloudflare bool
}

func CloudflareMiddleware(log zerolog.Logger) func(next http.Handler) http.Handler {
	getter := NewGetter(GetterConfig{
		Log:           log,
		Url:           "https://api.cloudflare.com/client/v4/ips",
		CheckInterval: time.Hour * 24,
	})
	type cfIpsResponse struct {
		Result struct {
			Ipv4Cidrs []string `json:"ipv4_cidrs"`
			Ipv6Cidrs []string `json:"ipv6_cidrs"`
		} `json:"result"`
	}
	isIpFromCloudflare := func(ctx context.Context, ip net.IP) bool {
		resp := &cfIpsResponse{}
		if err := getter.GetJson(ctx, resp); err != nil {
			log.Error().Err(err).Msg("error getting cloudflare ips")
			return false
		}

		// Check if IP is ipv4 or ipv6.
		if ip.To4() != nil {
			for _, cidr := range resp.Result.Ipv4Cidrs {
				_, ipNet, err := net.ParseCIDR(cidr)
				if err != nil {
					log.Error().Err(err).Msgf("error parsing cidr %s", cidr)
					continue
				}
				if ipNet.Contains(ip) {
					return true
				}
			}
		} else {
			for _, cidr := range resp.Result.Ipv6Cidrs {
				_, ipNet, err := net.ParseCIDR(cidr)
				if err != nil {
					log.Error().Err(err).Msgf("error parsing cidr %s", cidr)
					continue
				}
				if ipNet.Contains(ip) {
					return true
				}
			}
		}
		return false
	}

	return func(next http.Handler) http.Handler {
		fn := func(w http.ResponseWriter, r *http.Request) {
			// Check if the request is from Cloudflare.
			ip, _, _ := net.SplitHostPort(r.RemoteAddr)
			cfCtx := CloudflareContext{
				ClientIp:         nil,
				IsFromCloudflare: isIpFromCloudflare(r.Context(), net.ParseIP(ip)),
			}
			if cfCtx.IsFromCloudflare {
				// Get the client IP.
				cf := r.Header.Get("Cf-Connecting-Ip")
				if cf != "" {
					cfCtx.ClientIp = net.ParseIP(cf)
					return
				}
			}

			ctx := context.WithValue(r.Context(), cloudflareCtxKey, cfCtx)
			next.ServeHTTP(w, r.WithContext(ctx))
		}
		return http.HandlerFunc(fn)
	}
}

func CloudflareFromContext(ctx context.Context) CloudflareContext {
	token, _ := ctx.Value(cloudflareCtxKey).(CloudflareContext)
	return token
}

func RequireCloudflareMiddleware(log zerolog.Logger) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		fn := func(w http.ResponseWriter, r *http.Request) {
			cfCtx := CloudflareFromContext(r.Context())
			if !cfCtx.IsFromCloudflare {
				log.Warn().Msg("request not from cloudflare")
				http.Error(w, "ip not allowed", http.StatusForbidden)
				return
			}
			next.ServeHTTP(w, r)
		}
		return http.HandlerFunc(fn)
	}
}
