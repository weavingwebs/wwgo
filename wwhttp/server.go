package wwhttp

import (
	"context"
	"crypto/tls"
	"github.com/pkg/errors"
	"golang.org/x/sync/errgroup"
	"net/http"
	"time"
)

type Server struct {
	Domain    string
	Router    http.Handler
	AutoCert  bool
	CertFile  string
	CertKey   string
	HttpPort  string
	HttpsPort string
}

func (srv *Server) Start(ctx context.Context) error {
	if srv.HttpPort == "" {
		srv.HttpPort = "8080"
	}
	if srv.HttpsPort == "" {
		srv.HttpsPort = "8443"
	}

	httpServer := makeHttpServer(srv.Router, srv.HttpPort)

	var httpsServer *http.Server
	if srv.AutoCert || srv.CertFile != "" {
		// Redirect all http -> https.
		// NOTE: It is important to set this before calling ServerWithAutoCert so
		// we do not interfere with its wrapping of the handler.
		redirectRouter := &http.ServeMux{}
		redirectRouter.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusOK)
				return
			}
			newURI := "https://" + r.Host + r.URL.String()
			http.Redirect(w, r, newURI, http.StatusMovedPermanently)
		})
		httpServer.Handler = redirectRouter

		// Create HTTPS server.
		httpsServer = makeHttpsServer(srv.Router, srv.HttpsPort, srv.Domain)

		// Add autocert.
		if srv.AutoCert {
			ServerWithAutoCert(srv.Domain, httpServer, httpsServer)
		}
	}

	// Start servers.
	gos := &errgroup.Group{}
	gos.Go(panicIfErrFnExceptServerClosed(httpServer.ListenAndServe))
	if httpsServer != nil {
		gos.Go(func() error {
			return panicIfErrExceptServerClosed(httpsServer.ListenAndServeTLS(srv.CertFile, srv.CertKey))
		})
	}

	// Attempt graceful shutdown when cancelled.
	gos.Go(func() error {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()
		servers := []*http.Server{
			httpServer,
			httpsServer,
		}
		for _, srv := range servers {
			if srv == nil {
				continue
			}
			if err := srv.Shutdown(shutdownCtx); err != nil {
				return errors.Wrapf(err, "failed to shutdown http server %s", srv.Addr)
			}
		}

		return nil
	})

	return gos.Wait()
}

func makeHttpServer(r http.Handler, port string) *http.Server {
	return &http.Server{
		Addr:         ":" + port,
		Handler:      r,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  120 * time.Second,
	}
}

func makeHttpsServer(r http.Handler, port string, domain string) *http.Server {
	httpsServer := makeHttpServer(r, port)
	httpsServer.TLSConfig = &tls.Config{
		PreferServerCipherSuites: true,
		MinVersion:               tls.VersionTLS12,
		ServerName:               domain,
		CipherSuites: []uint16{
			tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305,
			tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305,
			tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
			tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
		},
	}
	return httpsServer
}

func panicIfErrExceptServerClosed(err error) error {
	if err == nil {
		return nil
	}
	if err == http.ErrServerClosed {
		return err
	}
	panic(err)
}

func panicIfErrFnExceptServerClosed(fn func() error) func() error {
	return func() error {
		return panicIfErrExceptServerClosed(fn())
	}
}
