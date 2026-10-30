package gabricdocker

import (
	"context"

	"github.com/bancek/gabric"
	"github.com/bancek/gabric/middleware"
)

// Install registers the Docker flags and setup middleware on g. It must run
// before Main parses flags.
func Install(g *gabric.Gabric) {
	if g.FlagSet.Lookup("docker-container") != nil {
		panic("gabricdocker: already installed")
	}

	container := g.FlagSet.String("docker-container", "", "Docker container")
	g.UseBuiltin(
		WithDocker,
		withDockerContainer(container),
	)
}

// InstallDefault installs Docker on gabric.Default.
func InstallDefault() {
	Install(gabric.Default)
}

func withDockerContainer(container *string) middleware.Middleware {
	return func(ctx context.Context, next middleware.Next) error {
		return WithContainer(*container)(ctx, next)
	}
}

// WithDocker installs the Docker runner and rsync fallback when no other
// transport is installed.
func WithDocker(ctx context.Context, next middleware.Next) error {
	if !gabric.HasRemoteRunner(ctx) {
		ctx = gabric.NewRemoteRunnerContext(ctx, Runner())
		ctx = gabric.NewRsyncRshContext(ctx, gabric.NoRsyncRsh)
	}

	return next(ctx)
}

// WithContainer sets the Docker container. It does not install a runner.
func WithContainer(container string) middleware.Middleware {
	return func(ctx context.Context, next middleware.Next) error {
		return next(NewContainerContext(ctx, container))
	}
}
