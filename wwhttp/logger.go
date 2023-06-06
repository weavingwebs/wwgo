package wwhttp

import (
	"bytes"
	"fmt"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/httplog"
	"github.com/rs/zerolog"
	"io"
	"net/http"
	"strings"
	"time"
)

// LoggerMiddleware is a copy of httplog.RequestLogger, but with Cloudflare
// header support.
func LoggerMiddleware(logger zerolog.Logger, cloudflare bool, optSkipPaths ...[]string) func(next http.Handler) http.Handler {
	var f middleware.LogFormatter = &requestLogger{
		Logger:     logger,
		Cloudflare: cloudflare,
	}

	skipPaths := map[string]struct{}{}
	if len(optSkipPaths) > 0 {
		for _, path := range optSkipPaths[0] {
			skipPaths[path] = struct{}{}
		}
	}

	return func(next http.Handler) http.Handler {
		fn := func(w http.ResponseWriter, r *http.Request) {
			// Skip the logger if the path is in the skip list
			if len(skipPaths) > 0 {
				_, skip := skipPaths[r.URL.Path]
				if skip {
					next.ServeHTTP(w, r)
					return
				}
			}

			// Log the request
			entry := f.NewLogEntry(r)
			ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)

			buf := newLimitBuffer(512)
			ww.Tee(buf)

			t1 := time.Now()
			defer func() {
				var respBody []byte
				if ww.Status() >= 400 {
					respBody, _ = io.ReadAll(buf)
				}
				entry.Write(ww.Status(), ww.BytesWritten(), ww.Header(), time.Since(t1), respBody)
			}()

			next.ServeHTTP(ww, middleware.WithLogEntry(r, entry))
		}
		return http.HandlerFunc(fn)
	}
}

type requestLogger struct {
	Logger     zerolog.Logger
	Cloudflare bool
}

func (l *requestLogger) NewLogEntry(r *http.Request) middleware.LogEntry {
	entry := &httplog.RequestLoggerEntry{}
	msg := fmt.Sprintf("Request: %s %s", r.Method, r.URL.Path)
	entry.Logger = l.Logger.With().Fields(requestLogFields(r, true, l.Cloudflare)).Logger()
	if !httplog.DefaultOptions.Concise {
		entry.Logger.Info().Fields(requestLogFields(r, httplog.DefaultOptions.Concise, l.Cloudflare)).Msgf(msg)
	}
	return entry
}

func requestLogFields(r *http.Request, concise bool, cloudflare bool) map[string]interface{} {
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	requestURL := fmt.Sprintf("%s://%s%s", scheme, r.Host, r.RequestURI)

	requestFields := map[string]interface{}{
		"requestURL":    requestURL,
		"requestMethod": r.Method,
		"requestPath":   r.URL.Path,
		"remoteIP":      r.RemoteAddr,
		"proto":         r.Proto,
	}
	if reqID := middleware.GetReqID(r.Context()); reqID != "" {
		requestFields["requestID"] = reqID
	}
	if cloudflare {
		if cf := CloudflareFromContext(r.Context()); cf.IsFromCloudflare {
			requestFields["cloudflareProxyIP"] = requestFields["remoteIP"]
			requestFields["remoteIP"] = cf.ClientIp
		}
	}

	if concise {
		return map[string]interface{}{
			"httpRequest": requestFields,
		}
	}

	requestFields["scheme"] = scheme

	if len(r.Header) > 0 {
		requestFields["header"] = headerLogField(r.Header)
	}

	return map[string]interface{}{
		"httpRequest": requestFields,
	}
}

func headerLogField(header http.Header) map[string]string {
	headerField := map[string]string{}
	for k, v := range header {
		k = strings.ToLower(k)
		switch {
		case len(v) == 0:
			continue
		case len(v) == 1:
			headerField[k] = v[0]
		default:
			headerField[k] = fmt.Sprintf("[%s]", strings.Join(v, "], ["))
		}
		if k == "authorization" || k == "cookie" || k == "set-cookie" {
			headerField[k] = "***"
		}

		for _, skip := range httplog.DefaultOptions.SkipHeaders {
			if k == skip {
				headerField[k] = "***"
				break
			}
		}
	}
	return headerField
}

// limitBuffer is used to pipe response body information from the
// response writer to a certain limit amount. The idea is to read
// a portion of the response body such as an error response so we
// may log it.
type limitBuffer struct {
	*bytes.Buffer
	limit int
}

func newLimitBuffer(size int) io.ReadWriter {
	return limitBuffer{
		Buffer: bytes.NewBuffer(make([]byte, 0, size)),
		limit:  size,
	}
}

func (b limitBuffer) Write(p []byte) (n int, err error) {
	if b.Buffer.Len() >= b.limit {
		return len(p), nil
	}
	limit := b.limit
	if len(p) < limit {
		limit = len(p)
	}
	return b.Buffer.Write(p[:limit])
}

func (b limitBuffer) Read(p []byte) (n int, err error) {
	return b.Buffer.Read(p)
}
