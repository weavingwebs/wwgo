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

type DaemonServer interface {
	Start(ctx context.Context) error
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
func RunDaemon(ctx context.Context, srv DaemonServer) *Daemon {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, os.Interrupt, syscall.SIGTERM) // SIGTERM for docker.
	ctx, cancel := context.WithCancel(ctx)
	gos, srvCtx := errgroup.WithContext(ctx)
	d := &Daemon{gos: gos}
	d.gos.Go(func() error {
		select {
		case <-ch:
			cancel()
		case <-srvCtx.Done():
		}
		return nil
	})
	d.gos.Go(func() error {
		return srv.Start(srvCtx)
	})
	return d
}
