package wwhttp

import (
	"context"
	"golang.org/x/sync/errgroup"
	"net/http"
	"os"
	"os/signal"
	"syscall"
)

type Daemon struct {
	gos *errgroup.Group
}

// Wait will block until the context is cancelled and will exit/panic or return.
func (d *Daemon) Wait() {
	if err := d.gos.Wait(); err != nil {
		if err == http.ErrServerClosed {
			os.Exit(130)
		}
		panic(err)
	}
}

// RunDaemon runs the given server in the background and automatically attempts
// graceful shutdown on interrupt.
func RunDaemon(ctx context.Context, srv *Server) *Daemon {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, os.Interrupt, syscall.SIGTERM) // SIGTERM for docker.
	srvCtx, cancel := context.WithCancel(ctx)
	d := &Daemon{gos: &errgroup.Group{}}
	d.gos.Go(func() error {
		<-ch
		cancel()
		return nil
	})
	d.gos.Go(func() error {
		return srv.Start(srvCtx)
	})
	return d
}
