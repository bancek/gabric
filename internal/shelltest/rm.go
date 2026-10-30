package shelltest

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"strings"

	"mvdan.cc/sh/v3/interp"
)

func (c *Core) runRM(ctx context.Context, cmd Command) error {
	hc := interp.HandlerCtx(ctx)

	recursive := false
	force := false
	endOptions := false
	paths := []string{}

	for _, arg := range cmd.Args {
		if !endOptions {
			switch {
			case arg == "--":
				endOptions = true
				continue
			case arg == "--force":
				force = true
				continue
			case arg == "--recursive":
				recursive = true
				continue
			case arg == "-" || !strings.HasPrefix(arg, "-"):
			default:
				for _, flag := range arg[1:] {
					switch flag {
					case 'f':
						force = true
					case 'r', 'R':
						recursive = true
					default:
						fmt.Fprintf(hc.Stderr, "rm: invalid option: %s\n", arg)
						return interp.ExitStatus(1)
					}
				}

				continue
			}
		}

		paths = append(paths, arg)
	}

	if len(paths) == 0 {
		if force {
			return nil
		}

		fmt.Fprintln(hc.Stderr, "rm: missing operand")
		return interp.ExitStatus(1)
	}

	status := 0

	for _, p := range paths {
		var err error

		if recursive {
			if !force {
				if _, statErr := c.fs.statAt(hc.Dir, p, false); statErr != nil {
					fmt.Fprintf(hc.Stderr, "rm: %v\n", statErr)
					status = 1
					continue
				}
			}

			err = c.fs.removeAllAt(hc.Dir, p)
		} else {
			err = c.fs.removeAt(hc.Dir, p)
			if err != nil && force && errors.Is(err, fs.ErrNotExist) {
				err = nil
			}
		}

		if err != nil {
			fmt.Fprintf(hc.Stderr, "rm: %v\n", err)
			status = 1
		}
	}

	if status != 0 {
		return interp.ExitStatus(1)
	}

	return nil
}
