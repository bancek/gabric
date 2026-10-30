package gabricdocker

import (
	"context"

	"github.com/bancek/gabric"
)

// Remote returns remote operations for container. It installs a fresh Docker
// runner and rsync policy, overriding any existing runner.
func Remote(ctx context.Context, container string) *gabric.RemoteOps {
	ctx = NewContainerContext(ctx, container)
	ctx = gabric.NewRemoteRunnerContext(ctx, Runner())
	ctx = gabric.NewRsyncRshContext(ctx, gabric.NoRsyncRsh)

	return gabric.Remote(ctx)
}

// Target returns a target for container.
func Target(container string) gabric.Target {
	return gabric.NewTarget(
		func(ctx context.Context) *gabric.RemoteOps {
			return Remote(ctx, container)
		},
		gabric.AppendRemoteExecEnvContext,
		gabric.NewRemoteWorkingDirContext,
	)
}
