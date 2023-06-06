package wwhttp

import (
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/rs/zerolog"
	"strings"
	"time"
)

func NewRouter(logger zerolog.Logger, serviceName string, cloudflare bool) *chi.Mux {
	httpLogger := logger.With().Str("service", strings.ToLower(serviceName)).Logger()

	r := chi.NewRouter()
	r.Use(middleware.SetHeader("X-Clacks-Overhead", "GNU Terry Pratchett"))
	r.Use(middleware.RequestID)
	if cloudflare {
		r.Use(CloudflareMiddleware(logger.With().Str("service", "cloudflare").Logger()))
	}
	r.Use(LoggerMiddleware(httpLogger, cloudflare))
	r.Use(middleware.Recoverer)
	r.Use(RequestIDHeaderMiddleware)
	r.Use(IpContextMiddleware)
	r.Use(middleware.ThrottleBacklog(100, 200, 60*time.Second))
	r.Use(middleware.Timeout(120 * time.Second))
	r.Use(middleware.Heartbeat("/ping"))
	return r
}
