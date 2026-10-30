package gabric

import (
	"context"
	"io"
)

// NewLocalRunner returns a RemoteRunner that runs argv(cmd) through the local
// backend. The local backend returns ExecExitError for a non-zero command
// status.
//
// It is the primitive behind LocalRemote. For example, a system ssh runner:
//
//	gabric.NewLocalRunner(func(cmd string) []string {
//		return []string{"ssh", "deploy@example.com", "/bin/sh", "-c", cmd}
//	})
func NewLocalRunner(argv func(cmd string) []string) RemoteRunner {
	return func(ctx context.Context, cmd string, stdin io.Reader, stdout, stderr io.Writer) error {
		return GetLocalBackend(ctx).Exec(ctx, ExecRequest{
			Cmd:    argv(cmd),
			Env:    GetLocalExecEnv(ctx),
			Stdin:  stdin,
			Stdout: stdout,
			Stderr: stderr,
		})
	}
}
