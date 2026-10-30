package sshremote

import (
	"context"
	"errors"
	"log/slog"
	"sync/atomic"
	"testing"
	"time"

	"github.com/bancek/gabric/internal/testutils"
)

type connectionContextKey struct{}

func newConnectTestRemote(t *testing.T) *Remote {
	t.Helper()

	host := &Host{hostname: "example.invalid", port: 22}
	config := NewConfig().
		WithConnectionTimeout(time.Second).
		WithIdleConnections(1).
		WithMaxConnections(1)
	r := NewRemote(NewHostConfig(host, config))
	t.Cleanup(r.Close)

	return r
}

func startGetResource(t *testing.T, r *Remote, ctx context.Context) (<-chan struct{}, <-chan error) {
	t.Helper()

	started := make(chan struct{})
	result := make(chan error, 1)

	go func() {
		close(started)

		resource, _, err := r.getResource(ctx, testInternalLogger())
		if err != nil {
			result <- err

			return
		}

		r.pool.Release(resource)
		result <- nil
	}()

	return started, result
}

func waitForStart(t *testing.T, started <-chan struct{}) {
	t.Helper()

	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("connection creation did not start")
	}
}

func waitForError(t *testing.T, result <-chan error) error {
	t.Helper()

	select {
	case err := <-result:
		return err
	case <-time.After(time.Second):
		t.Fatal("getResource() did not return")
		return nil
	}
}

func TestRemoteConnectionCreationSurvivesOperationCancellation(t *testing.T) {
	r := newConnectTestRemote(t)

	var calls atomic.Int32
	started := make(chan struct{})
	finish := make(chan struct{})
	seenValue := make(chan any, 1)
	seenErr := make(chan error, 1)

	r.newClient = func(ctx context.Context, _ HostConfig, _ *slog.Logger) (*RemoteClient, error) {
		if calls.Add(1) == 1 {
			close(started)

			select {
			case <-finish:
				seenValue <- ctx.Value(connectionContextKey{})
				seenErr <- ctx.Err()

				return &RemoteClient{}, nil
			case <-ctx.Done():
				seenErr <- ctx.Err()

				return nil, ctx.Err()
			}
		}

		return &RemoteClient{}, nil
	}

	opCtx := context.WithValue(context.Background(), connectionContextKey{}, "value")
	ctx, cancel := context.WithCancel(opCtx)

	_, result := startGetResource(t, r, ctx)

	waitForStart(t, started)
	cancel()

	if err := waitForError(t, result); !errors.Is(err, context.Canceled) {
		t.Fatalf("getResource() error = %v, want context.Canceled", err)
	}

	close(finish)

	select {
	case value := <-seenValue:
		if value != "value" {
			t.Fatalf("connection context value = %v, want value", value)
		}
	case <-time.After(time.Second):
		t.Fatal("connection creation did not finish")
	}

	select {
	case err := <-seenErr:
		if err != nil {
			t.Fatalf("connection context was canceled: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("connection creation did not finish")
	}

	testutils.Eventually(t, time.Second, func() bool {
		r.pool.mu.Lock()
		defer r.pool.mu.Unlock()

		return len(r.pool.idle) == 1
	})

	if got := calls.Load(); got != 1 {
		t.Fatalf("newClient calls = %d, want 1", got)
	}

	resource, _, err := r.getResource(context.Background(), testInternalLogger())
	if err != nil {
		t.Fatalf("getResource() error: %v", err)
	}

	r.pool.Release(resource)

	if got := calls.Load(); got != 1 {
		t.Fatalf("newClient calls after reuse = %d, want 1", got)
	}
}

func TestRemoteConnectionCreationStopsOnClose(t *testing.T) {
	r := newConnectTestRemote(t)

	started := make(chan struct{})
	creationDone := make(chan struct{})

	r.newClient = func(ctx context.Context, _ HostConfig, _ *slog.Logger) (*RemoteClient, error) {
		close(started)
		<-ctx.Done()
		close(creationDone)

		return nil, ctx.Err()
	}

	_, result := startGetResource(t, r, context.Background())

	waitForStart(t, started)
	r.Close()

	err := waitForError(t, result)
	if !errors.Is(err, errRemotePoolClosed) {
		t.Fatalf("getResource() error = %v, want %v", err, errRemotePoolClosed)
	}

	select {
	case <-creationDone:
	case <-time.After(time.Second):
		t.Fatal("connection creation did not stop after Close")
	}

	testutils.Eventually(t, time.Second, func() bool {
		return r.pool.NumResources() == 0
	})
	requirePoolStats(t, r.pool, 0, 0)
}

func TestRemoteConnectionCreationAfterCloseReleasesResource(t *testing.T) {
	r := newConnectTestRemote(t)

	started := make(chan struct{})
	finish := make(chan struct{})

	r.newClient = func(context.Context, HostConfig, *slog.Logger) (*RemoteClient, error) {
		close(started)
		<-finish

		return &RemoteClient{}, nil
	}

	_, result := startGetResource(t, r, context.Background())

	waitForStart(t, started)
	r.Close()

	if err := waitForError(t, result); !errors.Is(err, errRemotePoolClosed) {
		t.Fatalf("getResource() error = %v, want %v", err, errRemotePoolClosed)
	}

	close(finish)

	testutils.Eventually(t, time.Second, func() bool {
		return r.pool.NumResources() == 0
	})
	requirePoolStats(t, r.pool, 0, 0)
}

func TestPoolConnectionContextTimeout(t *testing.T) {
	ctx, cancel := poolConnectionContext(context.Background(), context.Background(), 10*time.Millisecond)
	defer cancel()

	select {
	case <-ctx.Done():
		if !errors.Is(context.Cause(ctx), context.DeadlineExceeded) {
			t.Fatalf("context cause = %v, want %v", context.Cause(ctx), context.DeadlineExceeded)
		}
	case <-time.After(time.Second):
		t.Fatal("connection context did not time out")
	}
}

func TestPoolConnectionContext(t *testing.T) {
	opCtx, cancelOp := context.WithCancel(context.WithValue(context.Background(), connectionContextKey{}, "value"))
	poolCtx, cancelPool := context.WithCancel(context.Background())

	ctx, cancel := poolConnectionContext(poolCtx, opCtx, time.Minute)
	defer cancel()

	if value := ctx.Value(connectionContextKey{}); value != "value" {
		t.Fatalf("context value = %v, want value", value)
	}
	if _, ok := ctx.Deadline(); !ok {
		t.Fatal("connection context has no deadline")
	}

	cancelOp()

	if err := ctx.Err(); err != nil {
		t.Fatalf("operation cancellation leaked into connection context: %v", err)
	}

	cancelPool()

	select {
	case <-ctx.Done():
	case <-time.After(time.Second):
		t.Fatal("pool cancellation did not cancel the connection context")
	}
}
