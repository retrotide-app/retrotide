package utils

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"time"
)

const (
	httpShutdownTimeout = 8 * time.Second
)

func Serve(ctx context.Context, srv *http.Server, onListen func(net.Addr) error) error {
	ln, err := net.Listen("tcp", srv.Addr)
	if err != nil {
		return fmt.Errorf("serve %s: %w", srv.Addr, err)
	}

	if err := onListen(ln.Addr()); err != nil {
		return errors.Join(fmt.Errorf("announce %s: %w", ln.Addr(), err), ln.Close())
	}

	serveErr := make(chan error, 1)
	go func() {
		serveErr <- srv.Serve(ln)
	}()

	select {
	case err := <-serveErr:
		return fmt.Errorf("serve %s: %w", srv.Addr, err)
	case <-ctx.Done():
	}

	shutdownCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), httpShutdownTimeout)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		return errors.Join(fmt.Errorf("shutdown: %w", err), srv.Close())
	}

	if err := <-serveErr; !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("serve %s: %w", srv.Addr, err)
	}

	return nil
}
