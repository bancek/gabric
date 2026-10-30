package sshremote

import (
	"context"
	"fmt"
	"time"

	"golang.org/x/crypto/ssh"
)

// ErrSessionOpenTimeout is returned when SessionOpenTimeout expires while
// opening a session. It matches errors.Is(err, context.DeadlineExceeded).
var ErrSessionOpenTimeout = fmt.Errorf("sshremote: session open timeout: %w", context.DeadlineExceeded)

// openSession opens an SSH session on a checked-out pooled resource. The
// timeout bounds only NewSession, not the operation that uses the session.
// Cancellation or timeout closes the connection to unblock NewSession.
func openSession(ctx context.Context, res *remotePoolResource, timeout time.Duration) (*ssh.Session, error) {
	if err := context.Cause(ctx); err != nil {
		return nil, err
	}

	type result struct {
		session *ssh.Session
		err     error
	}

	resultCh := make(chan result, 1)
	go func() {
		session, err := res.client.Client().NewSession()
		resultCh <- result{session: session, err: err}
	}()

	cancelOpen := func(err error) (*ssh.Session, error) {
		res.Close()

		result := <-resultCh
		if result.session != nil {
			_ = result.session.Close()
		}

		return nil, err
	}

	openCtx := ctx
	if timeout > 0 {
		var cancel context.CancelFunc
		openCtx, cancel = context.WithTimeoutCause(ctx, timeout, ErrSessionOpenTimeout)
		defer cancel()
	}

	select {
	case result := <-resultCh:
		return result.session, result.err
	case <-openCtx.Done():
		return cancelOpen(context.Cause(openCtx))
	}
}
