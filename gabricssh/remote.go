package gabricssh

import (
	"context"

	"github.com/bancek/gabric"
)

// Remote returns remote operations for host. It requires an SSH config and
// manager in the context, for example from WithSSH, WithRemote, Install, or
// NewConfigContext and NewManagerContext.
func Remote(ctx context.Context, host string) *gabric.RemoteOps {
	_ = GetConfig(ctx)
	_ = GetManager(ctx)

	ctx = NewHostNameContext(ctx, host)
	ctx = NewRunnerContext(ctx)

	return gabric.Remote(ctx)
}

// Target returns a target for host.
func Target(host string) gabric.Target {
	return gabric.NewTarget(
		func(ctx context.Context) *gabric.RemoteOps {
			return Remote(ctx, host)
		},
		gabric.AppendRemoteExecEnvContext,
		gabric.NewRemoteWorkingDirContext,
	)
}
