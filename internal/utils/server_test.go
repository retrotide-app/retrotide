package utils

import (
	"context"
	"errors"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

const (
	serveTestWait = 5 * time.Second
)

func TestServe_ReturnsNilSoonAfterItsContextIsCancelled(t *testing.T) {
	// given
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	listening := make(chan net.Addr, 1)
	srv := &http.Server{
		Addr:    "127.0.0.1:0",
		Handler: http.NotFoundHandler(),
	}

	served := make(chan error, 1)
	go func() {
		served <- Serve(ctx, srv, func(addr net.Addr) error {
			listening <- addr
			return nil
		})
	}()

	select {
	case <-listening:
	case err := <-served:
		t.Fatalf("Serve returned before listening: %v", err)
	case <-time.After(serveTestWait):
		t.Fatalf("Serve did not start listening within %s", serveTestWait)
	}

	// when
	cancel()

	// then
	select {
	case err := <-served:
		require.NoError(t, err)
	case <-time.After(serveTestWait):
		t.Fatalf("Serve did not return within %s of its context being cancelled", serveTestWait)
	}
}

func TestServe_ReturnsTheListenFailureWhenItsAddressIsHeld(t *testing.T) {
	// given
	held, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, held.Close())
	})

	ctx, cancel := context.WithTimeout(t.Context(), serveTestWait)
	defer cancel()

	srv := &http.Server{
		Addr:    held.Addr().String(),
		Handler: http.NotFoundHandler(),
	}

	announced := false

	// when
	err = Serve(ctx, srv, func(net.Addr) error {
		announced = true
		return nil
	})

	// then
	opErr, ok := errors.AsType[*net.OpError](err)
	require.True(t, ok, "want an error wrapping a *net.OpError, got %v", err)
	require.Equal(t, "listen", opErr.Op)
	require.ErrorContains(t, err, "serve "+srv.Addr)
	require.False(t, announced)
}

func TestServe_ReleasesTheAddressWhenTheAnnouncementFails(t *testing.T) {
	// given
	ctx, cancel := context.WithTimeout(t.Context(), serveTestWait)
	defer cancel()

	var bound net.Addr
	srv := &http.Server{
		Addr:    "127.0.0.1:0",
		Handler: http.NotFoundHandler(),
	}

	// when
	err := Serve(ctx, srv, func(addr net.Addr) error {
		bound = addr
		return errWriteFailed
	})

	// then
	require.ErrorIs(t, err, errWriteFailed)
	require.ErrorContains(t, err, "announce "+bound.String())

	ln, listenErr := net.Listen("tcp", bound.String())
	require.NoError(t, listenErr)
	require.NoError(t, ln.Close())
}
