package sshremote

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/bancek/gabric/internal/testutils"
)

type remotePoolAcquireResult struct {
	resource *remotePoolResource
	err      error
}

func mustAcquireFromPool(t *testing.T, p *remotePool) *remotePoolResource {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	resource, err := p.Acquire(ctx)
	if err != nil {
		t.Fatalf("Acquire() error: %v", err)
	}

	return resource
}

func requirePoolStats(t *testing.T, p *remotePool, total, idle int) {
	t.Helper()

	if got := p.NumResources(); got != total {
		t.Fatalf("NumResources() = %d, want %d", got, total)
	}
	if got := p.NumIdleResources(); got != idle {
		t.Fatalf("NumIdleResources() = %d, want %d", got, idle)
	}
}

func receivePoolAcquireResult(t *testing.T, ch <-chan remotePoolAcquireResult) remotePoolAcquireResult {
	t.Helper()

	select {
	case result := <-ch:
		return result
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for pool result")
		return remotePoolAcquireResult{}
	}
}

func TestRemotePoolReusesIdleResource(t *testing.T) {
	p := newRemotePool(1, 1)

	first := mustAcquireFromPool(t, p)

	if got := len(p.slots); got != 1 {
		t.Fatalf("checked-out tokens = %d, want 1", got)
	}

	p.Release(first)

	if got := len(p.slots); got != 0 {
		t.Fatalf("checked-out tokens after Release = %d, want 0", got)
	}

	second := mustAcquireFromPool(t, p)
	if second != first {
		t.Fatal("Acquire() did not reuse the idle resource")
	}

	p.Close()

	if second.IsClosed() {
		t.Fatal("Close() closed a checked-out resource")
	}

	p.Release(second)

	if !second.IsClosed() {
		t.Fatal("Release() after Close() did not close the resource")
	}

	if got := len(p.slots); got != 0 {
		t.Fatalf("checked-out tokens after Release = %d, want 0", got)
	}
}

func TestRemotePoolWaitsForRelease(t *testing.T) {
	p := newRemotePool(1, 1)

	first := mustAcquireFromPool(t, p)

	started := make(chan struct{})
	result := make(chan remotePoolAcquireResult, 1)

	go func() {
		close(started)

		resource, err := p.Acquire(context.Background())
		result <- remotePoolAcquireResult{resource: resource, err: err}
	}()

	<-started

	testutils.Consistently(t, 50*time.Millisecond, func() bool {
		select {
		case got := <-result:
			t.Fatalf("Acquire() returned before a release: resource=%v err=%v", got.resource, got.err)
			return false
		default:
			return true
		}
	})

	p.Release(first)

	got := receivePoolAcquireResult(t, result)
	if got.err != nil {
		t.Fatalf("Acquire() error: %v", got.err)
	}
	if got.resource != first {
		t.Fatal("Acquire() did not receive the released resource")
	}

	p.Release(got.resource)
	p.Close()
}

func TestRemotePoolCanceledWaiter(t *testing.T) {
	p := newRemotePool(1, 1)

	first := mustAcquireFromPool(t, p)

	ctx, cancel := context.WithCancel(context.Background())
	started := make(chan struct{})
	result := make(chan error, 1)

	go func() {
		close(started)
		_, err := p.Acquire(ctx)
		result <- err
	}()

	<-started

	testutils.Consistently(t, 50*time.Millisecond, func() bool {
		select {
		case err := <-result:
			t.Fatalf("Acquire() returned before cancellation: %v", err)
			return false
		default:
			return true
		}
	})

	cancel()

	select {
	case err := <-result:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("Acquire() error = %v, want context.Canceled", err)
		}
	case <-time.After(time.Second):
		t.Fatal("Acquire() did not return after cancellation")
	}

	p.Release(first)

	p.mu.Lock()
	idle := len(p.idle)
	p.mu.Unlock()

	if idle != 1 {
		t.Fatalf("idle resources = %d, want 1", idle)
	}
	if got := len(p.slots); got != 0 {
		t.Fatalf("checked-out tokens = %d, want 0", got)
	}

	second := mustAcquireFromPool(t, p)
	if second != first {
		t.Fatal("Acquire() did not reuse the idle resource after a canceled waiter")
	}

	p.Release(second)
	p.Close()
}

func TestRemotePoolClosedResourceWakesWaiter(t *testing.T) {
	p := newRemotePool(1, 1)

	first := mustAcquireFromPool(t, p)

	started := make(chan struct{})
	result := make(chan remotePoolAcquireResult, 1)

	go func() {
		close(started)

		resource, err := p.Acquire(context.Background())
		result <- remotePoolAcquireResult{resource: resource, err: err}
	}()

	<-started

	testutils.Consistently(t, 50*time.Millisecond, func() bool {
		select {
		case got := <-result:
			t.Fatalf("Acquire() returned before the resource was released: resource=%v err=%v", got.resource, got.err)
			return false
		default:
			return true
		}
	})

	first.Close()
	p.Release(first)

	got := receivePoolAcquireResult(t, result)
	if got.err != nil {
		t.Fatalf("Acquire() error: %v", got.err)
	}
	if got.resource == nil || got.resource == first {
		t.Fatal("Acquire() did not receive a fresh resource")
	}
	if !first.IsClosed() {
		t.Fatal("the closed resource was not closed")
	}

	p.mu.Lock()
	idle := len(p.idle)
	p.mu.Unlock()

	if idle != 0 {
		t.Fatalf("idle resources = %d, want 0", idle)
	}
	if got := len(p.slots); got != 1 {
		t.Fatalf("checked-out tokens = %d, want 1", got)
	}

	p.Release(got.resource)
	p.Close()
}

func TestRemotePoolCloseWakesWaiter(t *testing.T) {
	p := newRemotePool(1, 1)

	first := mustAcquireFromPool(t, p)

	started := make(chan struct{})
	result := make(chan remotePoolAcquireResult, 1)

	go func() {
		close(started)

		resource, err := p.Acquire(context.Background())
		result <- remotePoolAcquireResult{resource: resource, err: err}
	}()

	<-started

	testutils.Consistently(t, 50*time.Millisecond, func() bool {
		select {
		case got := <-result:
			t.Fatalf("Acquire() returned before Close: resource=%v err=%v", got.resource, got.err)
			return false
		default:
			return true
		}
	})

	p.Close()

	got := receivePoolAcquireResult(t, result)
	if !errors.Is(got.err, errRemotePoolClosed) {
		t.Fatalf("Acquire() error = %v, want %v", got.err, errRemotePoolClosed)
	}

	if first.IsClosed() {
		t.Fatal("Close() closed a checked-out resource")
	}

	if _, err := p.Acquire(context.Background()); !errors.Is(err, errRemotePoolClosed) {
		t.Fatalf("Acquire() after Close() error = %v, want %v", err, errRemotePoolClosed)
	}

	p.Release(first)

	if !first.IsClosed() {
		t.Fatal("Release() after Close() did not close the resource")
	}
	if got := len(p.slots); got != 0 {
		t.Fatalf("checked-out tokens after Release = %d, want 0", got)
	}

	p.Close()
}

func TestRemotePoolStats(t *testing.T) {
	p := newRemotePool(1, 2)

	requirePoolStats(t, p, 0, 0)

	first := mustAcquireFromPool(t, p)
	requirePoolStats(t, p, 1, 0)

	second := mustAcquireFromPool(t, p)
	requirePoolStats(t, p, 2, 0)

	p.Release(first)
	requirePoolStats(t, p, 2, 1)

	p.Close()
	requirePoolStats(t, p, 1, 0)

	p.Release(second)
	requirePoolStats(t, p, 0, 0)
}

func TestRemotePoolIdleCapacity(t *testing.T) {
	p := newRemotePool(0, 2)

	resource := mustAcquireFromPool(t, p)
	p.Release(resource)

	if !resource.IsClosed() {
		t.Fatal("Release() did not close the resource when idle capacity is zero")
	}

	p.mu.Lock()
	idle := len(p.idle)
	p.mu.Unlock()

	if idle != 0 {
		t.Fatalf("idle resources = %d, want 0", idle)
	}
	if got := len(p.slots); got != 0 {
		t.Fatalf("checked-out tokens = %d, want 0", got)
	}

	p.Close()
}

func TestRemotePoolUnlimited(t *testing.T) {
	p := newRemotePool(-1, -1)

	resources := []*remotePoolResource{
		mustAcquireFromPool(t, p),
		mustAcquireFromPool(t, p),
		mustAcquireFromPool(t, p),
	}

	for _, resource := range resources {
		p.Release(resource)
	}

	p.mu.Lock()
	idle := len(p.idle)
	p.mu.Unlock()

	if idle != len(resources) {
		t.Fatalf("idle resources = %d, want %d", idle, len(resources))
	}

	requirePoolStats(t, p, len(resources), len(resources))

	p.Close()
	requirePoolStats(t, p, 0, 0)

	for _, resource := range resources {
		if !resource.IsClosed() {
			t.Fatal("Close() did not close an idle resource")
		}
	}
}

func TestRemotePoolSlotRollbackOnCancellation(t *testing.T) {
	for range 100 {
		p := newRemotePool(0, 1)
		held := mustAcquireFromPool(t, p)

		ctx, cancel := context.WithCancel(context.Background())
		result := make(chan remotePoolAcquireResult, 1)
		released := make(chan struct{})

		go func() {
			resource, err := p.Acquire(ctx)
			result <- remotePoolAcquireResult{resource: resource, err: err}
		}()

		go func() {
			p.Release(held)
			close(released)
		}()

		cancel()

		got := receivePoolAcquireResult(t, result)
		switch {
		case got.err == nil:
			p.Release(got.resource)
		case errors.Is(got.err, context.Canceled):
		default:
			t.Fatalf("Acquire() error = %v, want context.Canceled or a resource", got.err)
		}

		<-released
		requirePoolStats(t, p, 0, 0)

		p.Close()
	}
}

func TestRemotePoolConcurrentReleaseAndClose(t *testing.T) {
	for range 100 {
		p := newRemotePool(1, 1)
		resource := mustAcquireFromPool(t, p)

		released := make(chan struct{})

		go func() {
			p.Release(resource)
			close(released)
		}()

		p.Close()

		select {
		case <-released:
		case <-time.After(time.Second):
			t.Fatal("Release() did not return after Close()")
		}

		if !resource.IsClosed() {
			t.Fatal("resource was not closed after Close() and Release()")
		}
		if got := len(p.slots); got != 0 {
			t.Fatalf("checked-out tokens = %d, want 0", got)
		}

		requirePoolStats(t, p, 0, 0)
	}
}

func TestNewRemotePoolRejectsInvalidConfig(t *testing.T) {
	tests := []struct {
		name         string
		idleCapacity int
		maxResources int
	}{
		{name: "zero max", idleCapacity: 1, maxResources: 0},
		{name: "negative max below -1", idleCapacity: 1, maxResources: -2},
		{name: "negative idle below -1", idleCapacity: -2, maxResources: 1},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			didPanic := false

			func() {
				defer func() {
					didPanic = recover() != nil
				}()

				newRemotePool(test.idleCapacity, test.maxResources)
			}()

			if !didPanic {
				t.Fatal("newRemotePool() did not panic")
			}
		})
	}
}
