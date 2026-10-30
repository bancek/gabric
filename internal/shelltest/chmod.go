package shelltest

import (
	"context"
	"fmt"
	"io/fs"
	"strconv"
	"strings"

	"mvdan.cc/sh/v3/interp"
)

func (c *Core) runChmod(ctx context.Context, cmd Command) error {
	hc := interp.HandlerCtx(ctx)

	endOptions := false
	modeArg := ""
	paths := []string{}

	for _, arg := range cmd.Args {
		if !endOptions {
			switch {
			case arg == "--":
				endOptions = true
				continue
			case strings.HasPrefix(arg, "-"):
				fmt.Fprintf(hc.Stderr, "chmod: invalid option: %s\n", arg)
				return interp.ExitStatus(1)
			}
		}

		if modeArg == "" {
			modeArg = arg
			continue
		}

		paths = append(paths, arg)
	}

	if modeArg == "" {
		fmt.Fprintln(hc.Stderr, "chmod: missing operand")
		return interp.ExitStatus(1)
	}

	mode, err := parseChmodMode(modeArg)
	if err != nil {
		fmt.Fprintf(hc.Stderr, "chmod: %v\n", err)
		return interp.ExitStatus(1)
	}

	if len(paths) == 0 {
		fmt.Fprintln(hc.Stderr, "chmod: missing operand")
		return interp.ExitStatus(1)
	}

	status := 0

	for _, p := range paths {
		if err := c.fs.chmodAt(hc.Dir, p, mode); err != nil {
			fmt.Fprintf(hc.Stderr, "chmod: %v\n", err)
			status = 1
		}
	}

	if status != 0 {
		return interp.ExitStatus(1)
	}

	return nil
}

func parseChmodMode(s string) (fs.FileMode, error) {
	n, err := strconv.ParseUint(s, 8, 32)
	if err != nil || n > 0o7777 {
		return 0, fmt.Errorf("invalid mode: %s", s)
	}

	mode := fs.FileMode(n & 0o777)

	if n&0o4000 != 0 {
		mode |= fs.ModeSetuid
	}
	if n&0o2000 != 0 {
		mode |= fs.ModeSetgid
	}
	if n&0o1000 != 0 {
		mode |= fs.ModeSticky
	}

	return mode, nil
}
