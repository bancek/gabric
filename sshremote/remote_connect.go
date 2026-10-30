package sshremote

import (
	"context"
	"log/slog"
	"time"
)

// This file owns connection establishment for pooled resources. The pool
// hands out empty remotePoolResource values; connectResource turns one into a
// usable connection without tying its lifetime to the operation that happened
// to request it.

// connectResource establishes the SSH connection for an empty pooled resource.
// Connection establishment is pool-owned: it keeps the values from ctx but is
// not canceled when ctx is, so a canceled operation does not throw away a
// connection that the pool can reuse.
func (r *Remote) connectResource(ctx context.Context, resource *remotePoolResource, logger *slog.Logger) (*remotePoolResource, error) {
	timeout := r.hostConfig.Config().ConnectionTimeout()
	if timeout <= 0 {
		timeout = DefaultConnectionTimeout
	}

	type resResult struct {
		resource *remotePoolResource
		err      error
	}

	// resultCh must stay unbuffered: a successful send tells the connection
	// goroutine that the caller has taken ownership. With a buffer, the
	// goroutine could return while the caller is selecting on cancellation,
	// leaking the resource and its pool slot.
	resultCh := make(chan resResult)

	go func() {
		newClientCtx, cancel := poolConnectionContext(r.pool.ctx, ctx, timeout)
		defer cancel()

		client, err := r.newClient(newClientCtx, r.hostConfig, logger)
		if err != nil {
			logger.With("error", err).Debug("Remote NewRemoteClient error")
			resource.Close()
			r.pool.Release(resource)

			select {
			case resultCh <- resResult{err: err}:
			case <-ctx.Done():
			case <-r.pool.ctx.Done():
			}

			return
		}

		resource.client = client

		select {
		case resultCh <- resResult{resource: resource}:
		case <-ctx.Done():
			r.pool.Release(resource)
		case <-r.pool.ctx.Done():
			r.pool.Release(resource)
		}
	}()

	select {
	case result := <-resultCh:
		if err := ctx.Err(); err != nil {
			if result.resource != nil {
				r.pool.Release(result.resource)
			}

			return nil, err
		}

		if result.err != nil {
			if err := context.Cause(r.pool.ctx); err != nil {
				return nil, err
			}

			return nil, result.err
		}

		if err := context.Cause(r.pool.ctx); err != nil {
			r.pool.Release(result.resource)
			return nil, err
		}

		return result.resource, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-r.pool.ctx.Done():
		return nil, context.Cause(r.pool.ctx)
	}
}

// poolConnectionContext returns the context used for pool-owned connection
// establishment. It preserves values from opCtx but not its cancellation or
// deadline, and is canceled when poolCtx is done or timeout elapses.
func poolConnectionContext(poolCtx, opCtx context.Context, timeout time.Duration) (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithTimeout(context.WithoutCancel(opCtx), timeout)
	stop := context.AfterFunc(poolCtx, cancel)

	return ctx, func() {
		stop()
		cancel()
	}
}
