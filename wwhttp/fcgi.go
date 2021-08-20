package wwhttp

import (
	"context"
	"github.com/pkg/errors"
	"golang.org/x/sync/errgroup"
	"net"
	"net/http"
	"net/http/fcgi"
	"os"
	"path"
)

type FcgiServer struct {
	Router     http.Handler
	SocketPath string
	// Dev only, should not be used in production.
	GlobalRwx bool
}

func NewFcgiServerFromEnv(router http.Handler) *FcgiServer {
	fcgiSocket := os.Getenv("WHALEBLAZER_FCGI_SOCK")
	if fcgiSocket == "" {
		panic("WHALEBLAZER_FCGI_SOCK is not defined")
	}
	return &FcgiServer{
		Router:     router,
		SocketPath: fcgiSocket,
		GlobalRwx:  os.Getenv("FCGI_SOCK_GLOBAL_RWX") == "1",
	}
}

func (srv *FcgiServer) Start(ctx context.Context) error {
	if srv.SocketPath == "" {
		return errors.Errorf("SocketPath cannot be empty")
	}
	if err := os.MkdirAll(path.Dir(srv.SocketPath), 0770); err != nil {
		return err
	}

	listener, err := net.Listen("unix", srv.SocketPath)
	if err != nil {
		return err
	}

	// HACK: chmod the socket file for local dev so apache can write to the socket
	// regardless of the user we run as.
	// In production, the server should be run as the apache user instead.
	if srv.GlobalRwx {
		if err := os.Chmod(path.Dir(srv.SocketPath), 0777); err != nil {
			return err
		}
		if err := os.Chmod(srv.SocketPath, 0777); err != nil {
			return err
		}
	}

	// Start server.
	gos, gosCtx := errgroup.WithContext(ctx)
	gos.Go(func() error {
		return fcgi.Serve(listener, srv.Router)
	})

	// Close when cancelled.
	closed := false
	gos.Go(func() error {
		select {
		case <-ctx.Done():
			closed = true
		case <-gosCtx.Done():
		}
		_ = listener.Close()
		return nil
	})

	// Wait.
	err = gos.Wait()

	// Suppress 'closing' error if we initiated the close.
	// @todo would be nice to close gracefully like http.Server.Shutdown().
	if err != nil && (!closed || !errors.Is(err, net.ErrClosed)) {
		return err
	}
	return http.ErrServerClosed
}
