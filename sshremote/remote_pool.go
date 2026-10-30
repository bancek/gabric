package sshremote

import (
	"context"
	"errors"
	"sync"
)

// This file implements the connection pool behind Remote. Acquire and Release
// are the hot paths, so the pool keeps its state behind a mutex and uses a
// channel as a counting semaphore instead of a dispatcher goroutine or a
// waiter queue.
//
// The pool creates empty remotePoolResource values. Dialing and the SSH
// handshake happen in Remote.getResource after a resource has been acquired,
// so the pool itself never performs network I/O in its critical sections.

var errRemotePoolClosed = errors.New("remote pool is closed")

// remotePoolResource is one pooled SSH connection. The client is set by
// Remote.getResource after the resource is acquired. Close is idempotent
// because a caller may close a broken resource and then release it; Release
// also closes resources that it discards.
type remotePoolResource struct {
	client *RemoteClient
	// closed is touched only by the resource's current owner. The pool also
	// touches it while the resource is idle, with remotePool.mu held. A
	// resource is never owned by two parties at once.
	closed bool
}

func (r *remotePoolResource) Close() {
	if r.closed {
		return
	}

	r.closed = true

	if r.client != nil {
		r.client.Close()
	}
}

func (r *remotePoolResource) IsClosed() bool {
	return r.closed
}

// remotePool pools connections for a single Remote.
//
// slots holds one token for each resource that is checked out or being
// created. Idle resources do not hold tokens. A caller must acquire a token
// before taking an idle resource or creating one, which bounds checked-out
// resources to cap(slots). A nil slots channel means no limit.
//
// idleCapacity is the maximum number of idle resources, or -1 for no limit.
// The idle slice never holds more than the number of resources that can be
// checked out, so a finite slots channel also bounds the idle slice.
type remotePool struct {
	idleCapacity int
	slots        chan struct{}

	// ctx is the pool lifetime. Close cancels it with errRemotePoolClosed, and
	// its cancellation also aborts in-progress connection establishment.
	ctx    context.Context
	cancel context.CancelCauseFunc

	mu    sync.Mutex
	idle  []*remotePoolResource
	total int
}

// newRemotePool returns a pool with the given limits. idleCapacity and
// maxResources may be -1 for no limit. maxResources must otherwise be
// positive. Invalid configurations panic because NewRemote cannot return an
// error.
func newRemotePool(idleCapacity, maxResources int) *remotePool {
	if maxResources == 0 || maxResources < -1 {
		panic("sshremote: MaxConnections must be positive or -1")
	}
	if idleCapacity < -1 {
		panic("sshremote: IdleConnections must be -1 or greater")
	}

	ctx, cancel := context.WithCancelCause(context.Background())

	p := &remotePool{
		idleCapacity: idleCapacity,
		ctx:          ctx,
		cancel:       cancel,
	}

	if maxResources != -1 {
		p.slots = make(chan struct{}, maxResources)
	}

	return p
}

// Acquire returns an idle resource or creates a new empty one. If the pool is
// at maxResources, Acquire waits for a Release, for ctx to be canceled, or for
// the pool to close.
func (p *remotePool) Acquire(ctx context.Context) (*remotePoolResource, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	if p.slots != nil {
		select {
		case p.slots <- struct{}{}:
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-p.ctx.Done():
			return nil, context.Cause(p.ctx)
		}

		// The select may have won the slot at the same time as cancellation
		// or shutdown; give it back before returning.
		if err := ctx.Err(); err != nil {
			p.releaseSlot()
			return nil, err
		}
	}

	p.mu.Lock()

	if err := context.Cause(p.ctx); err != nil {
		p.mu.Unlock()
		p.releaseSlot()
		return nil, err
	}

	if n := len(p.idle); n > 0 {
		resource := p.idle[n-1]
		p.idle[n-1] = nil
		p.idle = p.idle[:n-1]
		p.mu.Unlock()
		return resource, nil
	}

	p.total++
	p.mu.Unlock()

	return &remotePoolResource{}, nil
}

// Release returns a resource to the pool. It keeps the resource idle when
// there is room, or closes it when the pool is closed, the resource is already
// closed, or the idle capacity is full.
func (p *remotePool) Release(resource *remotePoolResource) {
	p.mu.Lock()

	if context.Cause(p.ctx) != nil || resource.IsClosed() {
		p.total--
		p.mu.Unlock()
		resource.Close()
		p.releaseSlot()
		return
	}

	if p.idleCapacity == -1 || len(p.idle) < p.idleCapacity {
		p.idle = append(p.idle, resource)
		p.mu.Unlock()
		p.releaseSlot()
		return
	}

	p.total--
	p.mu.Unlock()
	resource.Close()
	p.releaseSlot()
}

// releaseSlot gives back the token acquired by Acquire. A resource acquired
// from a pool with no limit has no token.
func (p *remotePool) releaseSlot() {
	if p.slots != nil {
		<-p.slots
	}
}

// Close closes idle resources and wakes callers waiting for capacity or an
// idle resource. Checked-out resources are not interrupted; they are closed
// when released. Close is idempotent and does not wait for checked-out
// resources.
func (p *remotePool) Close() {
	p.mu.Lock()

	if context.Cause(p.ctx) != nil {
		p.mu.Unlock()
		return
	}

	p.cancel(errRemotePoolClosed)
	idle := p.idle
	p.idle = nil
	p.total -= len(idle)
	p.mu.Unlock()

	for _, resource := range idle {
		resource.Close()
	}
}

// NumResources returns the number of resources known to the pool: idle plus
// checked out or being created. It is a snapshot, not a synchronization
// primitive.
func (p *remotePool) NumResources() int {
	p.mu.Lock()
	defer p.mu.Unlock()

	return p.total
}

// NumIdleResources returns the number of currently idle resources. It is a
// snapshot, not a synchronization primitive.
func (p *remotePool) NumIdleResources() int {
	p.mu.Lock()
	defer p.mu.Unlock()

	return len(p.idle)
}
