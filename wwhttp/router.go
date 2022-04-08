package wwhttp

import (
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/httplog"
	"github.com/rs/zerolog"
	"strings"
	"time"
)

func NewRouter(logger zerolog.Logger, serviceName string) *chi.Mux {
	httpLogger := logger.With().Str("service", strings.ToLower(serviceName))

	r := chi.NewRouter()
	r.Use(middleware.SetHeader("X-Clacks-Overhead", "GNU Terry Pratchett"))
	r.Use(httplog.Handler(httpLogger.Logger()))
	r.Use(middleware.Recoverer)
	r.Use(middleware.RequestID)
	r.Use(RequestIDHeaderMiddleware)
	r.Use(IpContextMiddleware)
	r.Use(middleware.ThrottleBacklog(100, 200, 60*time.Second))
	r.Use(middleware.Timeout(120 * time.Second))
	return r
}
