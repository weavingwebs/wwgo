package wwhttp

import (
	"context"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"net"
	"net/http"
	"strings"
)

var remoteAddrCtxKey = &contextKey{"remoteAddr"}

type contextKey struct {
	name string
}

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
		ip, _, _ := net.SplitHostPort(r.RemoteAddr)
		userIP := net.ParseIP(ip)
		ctx := context.WithValue(r.Context(), remoteAddrCtxKey, userIP)
		next.ServeHTTP(w, r.WithContext(ctx))
	}
	return http.HandlerFunc(fn)
}

func IpForContext(ctx context.Context) net.IP {
	token, _ := ctx.Value(remoteAddrCtxKey).(net.IP)
	return token
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
