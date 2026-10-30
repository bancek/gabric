package gabricdocker

import (
	"context"
	"io"

	"github.com/bancek/gabric"
)

// Runner returns a remote runner that executes commands with docker exec.
func Runner() gabric.RemoteRunner {
	return func(ctx context.Context, cmd string, stdin io.Reader, stdout, stderr io.Writer) error {
		container, err := GetContainer(ctx)
		if err != nil {
			return err
		}

		runner := gabric.NewLocalRunner(func(command string) []string {
			return []string{"docker", "exec", "-i", container, "/bin/sh", "-c", command}
		})

		return runner(ctx, cmd, stdin, stdout, stderr)
	}
}
