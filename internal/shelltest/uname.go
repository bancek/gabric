package shelltest

import (
	"context"
	"fmt"

	"mvdan.cc/sh/v3/interp"
)

func (c *Core) runUname(ctx context.Context, cmd Command) error {
	hc := interp.HandlerCtx(ctx)

	if len(cmd.Args) == 0 || (len(cmd.Args) == 1 && (cmd.Args[0] == "-s" || cmd.Args[0] == "--kernel-name")) {
		fmt.Fprintln(hc.Stdout, "Linux")

		return nil
	}

	fmt.Fprintf(hc.Stderr, "uname: unsupported: %v\n", cmd.Args)

	return interp.ExitStatus(1)
}
