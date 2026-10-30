package gabricssh

import (
	"context"
	"strings"

	"github.com/bancek/gabric"
)

// RegisterSshTask registers an "ssh" task that opens an interactive shell on
// the host in the execution context.
func RegisterSshTask(g *gabric.Gabric) func(ctx context.Context) {
	return g.Task("ssh", func(ctx context.Context) {
		Shell(ctx).Run()
	})
}

// RegisterForwardTask registers a "forward" task that opens a local port
// forward. The task argument is "<local>,<remote>" in host:port form; a single
// address is used for both sides.
func RegisterForwardTask(g *gabric.Gabric) func(ctx context.Context) {
	return g.Task("forward", func(ctx context.Context) {
		local, remote := parseForwardArg(gabric.GetCurrentTaskArg(ctx))

		if remote == "" {
			panic("forward task requires a <local>,<remote> argument")
		}

		ops := Forward(ctx).Remote(remote)
		if local != "" {
			ops = ops.Local(local)
		}

		ops.Run()
	})
}

// parseForwardArg parses the forward task argument. A single address applies to
// both sides.
func parseForwardArg(arg string) (local, remote string) {
	if arg == "" {
		return "", ""
	}

	local, remote, ok := strings.Cut(arg, ",")
	if !ok {
		return arg, arg
	}

	return local, remote
}
