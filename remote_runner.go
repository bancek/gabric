package gabric

import (
	"context"
	"errors"
	"io"
)

// RemoteRunner executes a complete shell command with the given streams.
//
// The runner must execute cmd with a shell, wire stdin, stdout and stderr
// directly, and not allocate a TTY. It must honor ctx cancellation, return nil
// for a zero exit status, and return ExecExitError for a non-zero command
// status. Start and transport failures are returned as ordinary errors.
type RemoteRunner func(ctx context.Context, cmd string, stdin io.Reader, stdout, stderr io.Writer) error

const remoteRunnerKey contextKey[RemoteRunner] = "gabric.remoteRunner"

// ErrNoRemoteRunner reports that ctx carries no remote runner.
var ErrNoRemoteRunner = errors.New("gabric: no remote runner in context")

// NewRemoteRunnerContext returns a context that uses runner to execute remote
// shell commands.
func NewRemoteRunnerContext(ctx context.Context, runner RemoteRunner) context.Context {
	if runner == nil {
		panic("gabric: nil remote runner")
	}

	return withContextValue(ctx, remoteRunnerKey, runner)
}

// GetRemoteRunner returns the remote runner from ctx. It panics with
// ErrNoRemoteRunner when no runner is installed.
func GetRemoteRunner(ctx context.Context) RemoteRunner {
	runner, ok := contextValue(ctx, remoteRunnerKey)
	if !ok || runner == nil {
		panic(ErrNoRemoteRunner)
	}

	return runner
}

// HasRemoteRunner reports whether ctx carries a remote runner.
func HasRemoteRunner(ctx context.Context) bool {
	runner, ok := contextValue(ctx, remoteRunnerKey)

	return ok && runner != nil
}
