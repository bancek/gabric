package sshremote

import (
	"context"
	"errors"
	"log/slog"
	"sync"

	"golang.org/x/crypto/ssh"
)

// Remote manages a pooled SSH connection to a single host.
type Remote struct {
	hostConfig HostConfig
	pool       *remotePool

	// newClient is overridden in tests; NewRemote sets it to NewRemoteClient.
	newClient func(context.Context, HostConfig, *slog.Logger) (*RemoteClient, error)
}

// NewRemote returns a Remote for hc. The config is part of the Remote identity
// and carries the pooling limits. Advanced: most callers should use
// RemoteManager.GetRemote.
func NewRemote(hc HostConfig) *Remote {
	config := hc.Config()

	return &Remote{
		hostConfig: hc,
		pool:       newRemotePool(config.idleConnections, config.maxConnections),
		newClient:  NewRemoteClient,
	}
}

// Close closes idle pooled connections. Connections currently in use are
// closed when they are released. In-progress connection establishment is
// canceled. Close is idempotent.
func (r *Remote) Close() {
	r.pool.Close()
}

// String returns a human readable representation.
func (r *Remote) String() string {
	return "remote " + r.hostConfig.String()
}

func (r *Remote) getResource(ctx context.Context, logger *slog.Logger) (*remotePoolResource, bool, error) {
	logger = logger.With("host", r.hostConfig.String())

	res, err := r.pool.Acquire(ctx)
	if err != nil {
		return nil, false, err
	}

	if err := ctx.Err(); err != nil {
		r.pool.Release(res)
		return nil, false, err
	}

	if err := context.Cause(r.pool.ctx); err != nil {
		r.pool.Release(res)
		return nil, false, err
	}

	if res.client == nil {
		connected, err := r.connectResource(ctx, res, logger)
		if err != nil {
			return nil, false, err
		}

		return connected, true, nil
	}

	return res, false, nil
}

// WithSession runs f with a new ssh session on a pooled connection. Cancelling
// ctx closes the session only; the connection is reused. A stale pooled
// connection that cannot open a session is discarded and the call retries with
// another pooled connection. f itself is never retried.
func (r *Remote) WithSession(ctx context.Context, logger *slog.Logger, f func(session *ssh.Session) error) error {
	for {
		res, fresh, err := r.getResource(ctx, logger)
		if err != nil {
			return err
		}

		session, err := openSession(ctx, res, r.hostConfig.Config().SessionOpenTimeout())
		if err != nil {
			if _, ok := errors.AsType[*ssh.OpenChannelError](err); ok {
				// The server answered and refused the session; the
				// connection is still healthy.
				r.pool.Release(res)

				if ctxErr := context.Cause(ctx); ctxErr != nil {
					return ctxErr
				}

				return err
			}

			res.Close()
			r.pool.Release(res)

			if ctxErr := context.Cause(ctx); ctxErr != nil {
				return ctxErr
			}
			if errors.Is(err, ErrSessionOpenTimeout) || fresh {
				return err
			}

			continue
		}

		return r.runSession(ctx, res, session, f)
	}
}

func (r *Remote) runSession(ctx context.Context, res *remotePoolResource, session *ssh.Session, f func(session *ssh.Session) error) error {
	defer r.pool.Release(res)

	var closeOnce sync.Once
	closeSession := func() {
		closeOnce.Do(func() { _ = session.Close() })
	}
	defer closeSession()

	cancelDone := make(chan struct{})
	stop := context.AfterFunc(ctx, func() {
		closeSession()
		close(cancelDone)
	})
	defer stop()

	err := f(session)
	if !stop() {
		// stop does not wait for the callback; wait before returning so the
		// deferred closeSession cannot race with it.
		<-cancelDone

		return ctx.Err()
	}

	return err
}
