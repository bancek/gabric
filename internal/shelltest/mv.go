package shelltest

import (
	"context"
	"fmt"
	"path"
	"strings"

	"mvdan.cc/sh/v3/interp"
)

func (c *Core) runMV(ctx context.Context, cmd Command) error {
	hc := interp.HandlerCtx(ctx)

	endOptions := false
	args := []string{}

	for _, arg := range cmd.Args {
		if !endOptions {
			switch {
			case arg == "--":
				endOptions = true
				continue
			case strings.HasPrefix(arg, "-"):
				fmt.Fprintf(hc.Stderr, "mv: invalid option: %s\n", arg)
				return interp.ExitStatus(1)
			}
		}

		args = append(args, arg)
	}

	if len(args) < 2 {
		fmt.Fprintln(hc.Stderr, "mv: missing operand")
		return interp.ExitStatus(1)
	}

	dest := args[len(args)-1]
	sources := args[:len(args)-1]

	destInfo, destErr := c.fs.statAt(hc.Dir, dest, true)
	destIsDir := destErr == nil && destInfo.IsDir()

	if len(sources) > 1 && !destIsDir {
		fmt.Fprintln(hc.Stderr, "mv: target is not a directory")
		return interp.ExitStatus(1)
	}

	status := 0

	for _, src := range sources {
		target := dest
		if destIsDir {
			target = path.Join(dest, path.Base(src))
		}

		if err := c.fs.renameAt(hc.Dir, src, target); err != nil {
			fmt.Fprintf(hc.Stderr, "mv: %v\n", err)
			status = 1
		}
	}

	if status != 0 {
		return interp.ExitStatus(1)
	}

	return nil
}
