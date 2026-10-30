package gabric

import (
	"context"

	"github.com/bancek/gabric/middleware"
)

// NewLocalRemoteContext returns a context configured for LocalRemote.
func NewLocalRemoteContext(ctx context.Context) context.Context {
	ctx = NewRemoteRunnerContext(ctx, localRemoteRunner)
	ctx = NewRsyncRshContext(ctx, NoRsyncRsh)

	return ctx
}

// LocalRemote returns remote operations that execute commands on the local
// machine without SSH.
func LocalRemote(ctx context.Context) *RemoteOps {
	return Remote(NewLocalRemoteContext(ctx))
}

// WithLocalRemoteTarget returns middleware that runs next with a LocalRemote
// target.
func WithLocalRemoteTarget(ctx context.Context, next middleware.Next) error {
	return next(NewTargetContext(ctx, LocalRemoteTarget))
}

var localRemoteRunner = NewLocalRunner(func(cmd string) []string {
	return []string{"/bin/sh", "-c", cmd}
})
