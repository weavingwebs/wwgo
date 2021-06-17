package wwhttp

import (
	"context"
	"crypto/tls"
	"fmt"
	"golang.org/x/crypto/acme/autocert"
	"net/http"
	"os"
)

func ServerWithAutoCert(
	domain string,
	httpServer *http.Server,
	httpsServer *http.Server,
) {
	// Autocert manager will deal with obtaining, caching & renewing the cert.
	autoCert := &autocert.Manager{
		Prompt: autocert.AcceptTOS,
		Cache:  autocert.DirCache("/certs"),
		Email:  os.Getenv("AUTOCERT_EMAIL"),
	}

	// The autocert manager will wait until a request before trying to obtain a
	// certificate, this function allows us to whitelist domain(s) we want a
	// certificate for.
	autoCert.HostPolicy = func(ctx context.Context, host string) error {
		if host == domain {
			return nil
		}
		return fmt.Errorf("acme/autocert: only %s host is allowed", domain)
	}

	// Set the HTTPS server to use autocert to get the certificate.
	if httpsServer.TLSConfig == nil {
		httpsServer.TLSConfig = &tls.Config{}
	}
	httpsServer.TLSConfig.GetCertificate = autoCert.GetCertificate

	// Wrap the HTTP handler with autocert so it can intercept DNS verifications.
	httpServer.Handler = autoCert.HTTPHandler(httpServer.Handler)
}
