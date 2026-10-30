package gabricssh

import (
	"context"
	"errors"
	"io"

	"golang.org/x/crypto/ssh"

	"github.com/bancek/gabric"
)

// Runner returns the SSH remote runner.
func Runner() gabric.RemoteRunner {
	return sshRunner
}

// NewRunnerContext installs the SSH runner and rsync rsh resolver. It assumes
// the SSH config, manager, and host are already in the context or will be set
// later; it does not create them.
func NewRunnerContext(ctx context.Context) context.Context {
	ctx = gabric.NewRemoteRunnerContext(ctx, Runner())
	return gabric.NewRsyncRshContext(ctx, sshRsyncRsh)
}

func sshRunner(ctx context.Context, cmd string, stdin io.Reader, stdout, stderr io.Writer) error {
	err := GetRemote(ctx).WithSession(ctx, gabric.Logger(ctx), func(session *ssh.Session) error {
		session.Stdin = stdin
		session.Stdout = stdout
		session.Stderr = stderr

		return session.Run(cmd)
	})
	if err != nil {
		if exitErr, ok := errors.AsType[*ssh.ExitError](err); ok {
			return &gabric.ExecExitError{
				Status: exitErr.ExitStatus(),
				Err:    err,
			}
		}

		return err
	}

	return nil
}
