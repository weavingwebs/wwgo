package wwhttp

import (
	"context"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"net"
	"net/http"
	"strings"
)

type contextKey struct {
	name string
}

var remoteAddrCtxKey = &contextKey{"remoteAddr"}

func RequestIDHeaderMiddleware(next http.Handler) http.Handler {
	fn := func(w http.ResponseWriter, r *http.Request) {
		reqId := middleware.GetReqID(r.Context())
		if reqId != "" {
			w.Header().Set("X-Request-Id", reqId)
		}
		next.ServeHTTP(w, r)
	}
	return http.HandlerFunc(fn)
}

func IpContextMiddleware(next http.Handler) http.Handler {
	fn := func(w http.ResponseWriter, r *http.Request) {
		var ip net.IP
		cfContext := CloudflareFromContext(r.Context())
		if cfContext.IsFromCloudflare && cfContext.ClientIp != nil {
			ip = cfContext.ClientIp
		} else {
			ipStr, _, _ := net.SplitHostPort(r.RemoteAddr)
			ip = net.ParseIP(ipStr)
		}
		ctx := context.WithValue(r.Context(), remoteAddrCtxKey, ip)
		next.ServeHTTP(w, r.WithContext(ctx))
	}
	return http.HandlerFunc(fn)
}

func IpForContext(ctx context.Context) net.IP {
	ip, _ := ctx.Value(remoteAddrCtxKey).(net.IP)
	return ip
}

func CorsMiddleware(allowedOrigins []string) func(next http.Handler) http.Handler {
	return cors.Handler(cors.Options{
		AllowOriginFunc: func(r *http.Request, origin string) bool {
			for _, o := range allowedOrigins {
				if o == "*" {
					return true
				}

				// Allow wildcards i.e. for netlify deploy previews.
				if strings.Contains(o, "*") {
					parts := strings.SplitN(o, "*", 2)
					if strings.HasPrefix(origin, parts[0]) && strings.HasSuffix(origin, parts[1]) {
						return true
					}
					continue
				}

				if origin == o {
					return true
				}
			}
			return false
		},
		AllowedMethods: []string{
			http.MethodOptions,
			http.MethodHead,
			http.MethodGet,
			http.MethodPost,
		},
		AllowedHeaders:   []string{"*"},
		AllowCredentials: true,
	})
}

var userAgentCtxKey = &contextKey{"userAgent"}

func ContextWithUserAgent(ctx context.Context, userAgent string) context.Context {
	return context.WithValue(ctx, userAgentCtxKey, userAgent)
}

func UserAgentFromContext(ctx context.Context) string {
	return ctx.Value(userAgentCtxKey).(string)
}

func UserAgentMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := ContextWithUserAgent(r.Context(), r.UserAgent())
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

var basicAuthCtxKey = &contextKey{"basicAuth"}

type BasicAuth struct {
	Username string
	Password string
}

func ContextWithBasicAuth(ctx context.Context, basicAuth *BasicAuth) context.Context {
	return context.WithValue(ctx, basicAuthCtxKey, basicAuth)
}

func BasicAuthFromContext(ctx context.Context) *BasicAuth {
	return ctx.Value(basicAuthCtxKey).(*BasicAuth)
}

func BasicAuthOptionalMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		if username, password, ok := r.BasicAuth(); ok {
			ctx = ContextWithBasicAuth(ctx, &BasicAuth{Username: username, Password: password})
		}
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
