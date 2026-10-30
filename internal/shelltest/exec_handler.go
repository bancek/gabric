package shelltest

import (
	"context"
	"io"

	"github.com/bancek/gabric/internal/sshtest"
)

// ExecHandler returns an sshtest exec handler that runs commands with ctx.
func (c *Core) ExecHandler(ctx context.Context) sshtest.ExecHandler {
	return func(cmd string, stdin io.Reader, stdout, stderr io.Writer) int {
		code, err := c.RunScript(ctx, cmd, stdin, stdout, stderr)
		if err != nil && code == 0 {
			// RunScript has already reported the error on stderr; sshtest
			// only accepts an exit code, so make sure it is non-zero.
			return 1
		}

		return code
	}
}
