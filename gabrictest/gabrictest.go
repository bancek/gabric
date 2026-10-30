package gabrictest

import (
	"context"
	"io"
	"log/slog"
	"testing"

	"github.com/bancek/gabric"
	"github.com/bancek/gabric/logger"
)

// Test is a high-level, in-process test harness.
type Test struct {
	t      testing.TB
	local  *side
	remote *side
}

// New returns a harness for t.
func New(t testing.TB) *Test {
	t.Helper()

	return &Test{
		t:      t,
		local:  newSide(t),
		remote: newSide(t),
	}
}

// Local returns the local sandbox.
func (g *Test) Local() Side {
	return g.local
}

// Remote returns the remote sandbox.
func (g *Test) Remote() Side {
	return g.remote
}

// Command returns the expected command record for name and args in the harness
// default working directory.
func (g *Test) Command(name string, args ...string) Command {
	return Command{Name: name, Args: args, Dir: "/"}
}

// Context returns a fresh gabric context wired to the harness backends. Local
// and remote working directories are rooted at / so relative paths stay inside
// their sandboxes.
func (g *Test) Context() context.Context {
	g.t.Helper()

	return g.WithContext(g.t.Context())
}

// WithContext adds the harness defaults to ctx without replacing a logger,
// working directory, or backend that ctx already carries.
func (g *Test) WithContext(ctx context.Context) context.Context {
	g.t.Helper()

	if !logger.HasLogger(ctx) {
		discard := slog.New(slog.NewTextHandler(io.Discard, nil))
		ctx = logger.NewLoggerContext(ctx, discard)
	}

	if !gabric.HasLocalWorkingDir(ctx) {
		ctx = gabric.NewLocalWorkingDirContext(ctx, "/")
	}

	if !gabric.HasRemoteWorkingDir(ctx) {
		ctx = gabric.NewRemoteWorkingDirContext(ctx, "/")
	}

	if !gabric.HasLocalBackend(ctx) {
		ctx = gabric.NewLocalBackendContext(ctx, localBackend{side: g.local})
	}

	if !gabric.HasRemoteBackend(ctx) {
		ctx = gabric.NewRemoteBackendContext(ctx, remoteBackend{side: g.remote})
	}

	if !gabric.HasTarget(ctx) {
		ctx = gabric.NewTargetContext(ctx, gabric.RemoteTarget)
	}

	return ctx
}

// WithRemoteExecError returns the harness context with a remote backend whose
// Exec fails with err.
func (g *Test) WithRemoteExecError(err error) context.Context {
	g.t.Helper()

	return gabric.NewRemoteBackendContext(g.Context(), gabric.RemoteBackendFunc{
		ExecFunc: func(context.Context, gabric.ExecRequest) error {
			return err
		},
	})
}
