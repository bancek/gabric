package gabricssh

import (
	"context"

	"github.com/bancek/gabric"
	"github.com/bancek/gabric/middleware"
	"github.com/bancek/gabric/sshremote"
)

// WithSSH installs the SSH config, manager, runner, and rsync rsh resolver. It
// never sets a host and only installs the runner when no other runner exists,
// so the first transport wins.
func WithSSH(ctx context.Context, next middleware.Next) error {
	if !HasConfig(ctx) {
		config, err := sshremote.NewDefaultConfig()
		if err != nil {
			return err
		}
		ctx = NewConfigContext(ctx, config)
	}

	return WithManager(ctx, func(ctx context.Context) error {
		if !gabric.HasRemoteRunner(ctx) {
			ctx = NewRunnerContext(ctx)
		}

		return next(ctx)
	})
}

// WithRemote installs SSH setup for host and runs next with the SSH runner.
// It creates a default config and manager when needed and closes a manager it
// creates.
func WithRemote(ctx context.Context, host string, next middleware.Next) error {
	return WithSSH(ctx, func(ctx context.Context) error {
		ctx = NewHostNameContext(ctx, host)
		ctx = NewRunnerContext(ctx)

		return next(ctx)
	})
}
