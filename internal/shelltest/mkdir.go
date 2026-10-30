package shelltest

import (
	"context"
	"fmt"
	"strings"

	"mvdan.cc/sh/v3/interp"
)

func (c *Core) runMkdir(ctx context.Context, cmd Command) error {
	hc := interp.HandlerCtx(ctx)

	recursive := false
	endOptions := false
	paths := []string{}

	for _, arg := range cmd.Args {
		if !endOptions {
			switch {
			case arg == "--":
				endOptions = true
				continue
			case arg == "-p":
				recursive = true
				continue
			case strings.HasPrefix(arg, "-"):
				fmt.Fprintf(hc.Stderr, "mkdir: invalid option: %s\n", arg)
				return interp.ExitStatus(1)
			}
		}

		paths = append(paths, arg)
	}

	if len(paths) == 0 {
		fmt.Fprintln(hc.Stderr, "mkdir: missing operand")
		return interp.ExitStatus(1)
	}

	status := 0

	for _, p := range paths {
		var err error

		if recursive {
			err = c.fs.mkdirAllAt(hc.Dir, p, 0o755)
		} else {
			err = c.fs.mkdirAt(hc.Dir, p, 0o755)
		}

		if err != nil {
			fmt.Fprintf(hc.Stderr, "mkdir: %v\n", err)
			status = 1
		}
	}

	if status != 0 {
		return interp.ExitStatus(1)
	}

	return nil
}
