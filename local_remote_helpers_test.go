package gabric

import (
	"context"
	"fmt"
	"io"
	"testing"

	"github.com/bancek/gabric/internal/shelltest"
)

type localExecHandler func(cmd string, stdin io.Reader, stdout, stderr io.Writer) int

func localRemoteContext(t *testing.T) (*shelltest.Core, *shelltest.Assertions, context.Context) {
	t.Helper()

	return localRemoteContextWithExec(t, nil, false)
}

func localRemoteContextWithExec(t *testing.T, handler localExecHandler, sudo bool) (*shelltest.Core, *shelltest.Assertions, context.Context) {
	t.Helper()

	core := shelltest.New()

	runner := func(ctx context.Context, cmd string, stdin io.Reader, stdout, stderr io.Writer) error {
		var code int
		var err error

		if handler != nil {
			code = handler(cmd, stdin, stdout, stderr)
		} else {
			code, err = core.RunScript(ctx, cmd, stdin, stdout, stderr)
		}
		if err != nil {
			return err
		}
		if code != 0 {
			return &ExecExitError{Status: code}
		}

		return nil
	}

	ctx := NewRemoteRunnerContext(testContext(t), runner)
	if sudo {
		ctx = NewRemoteSudoContext(ctx, &Sudo{})
	}

	return core, shelltest.NewAssertions(t, core.Commands), ctx
}

// localRemoteTargetContext wires LocalRemoteTarget to the shelltest core. The
// target resets the remote runner to localRemoteRunner, which executes through
// the local backend, so that backend must also use the core.
func localRemoteTargetContext(t *testing.T) (*shelltest.Core, *shelltest.Assertions, context.Context) {
	t.Helper()

	core := shelltest.New()

	backend := LocalBackendFunc{
		ExecFunc: func(ctx context.Context, req ExecRequest) error {
			if len(req.Cmd) != 3 || req.Cmd[0] != "/bin/sh" || req.Cmd[1] != "-c" {
				return fmt.Errorf("unexpected local command: %v", req.Cmd)
			}

			code, err := core.RunScriptEnv(
				ctx,
				req.Cmd[2],
				req.Env.Environ(),
				req.Stdin,
				req.Stdout,
				req.Stderr,
			)
			if err != nil {
				return err
			}
			if code != 0 {
				return &ExecExitError{Status: code}
			}

			return nil
		},
	}

	ctx := NewLocalBackendContext(testContext(t), backend)
	ctx = NewRemoteSudoContext(ctx, &Sudo{})
	ctx = NewTargetContext(ctx, LocalRemoteTarget)

	return core, shelltest.NewAssertions(t, core.Commands), ctx
}
