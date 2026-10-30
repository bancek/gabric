package shelltest

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"strings"

	"mvdan.cc/sh/v3/interp"
)

func (c *Core) runCat(ctx context.Context, cmd Command) error {
	hc := interp.HandlerCtx(ctx)

	status := 0
	hasFile := false
	endOptions := false
	stdinRead := false

	readStdin := func() {
		if stdinRead {
			return
		}

		stdinRead = true

		var stdin io.Reader = hc.Stdin
		if stdin == nil {
			stdin = bytes.NewReader(nil)
		}

		if _, err := io.Copy(hc.Stdout, stdin); err != nil {
			fmt.Fprintf(hc.Stderr, "cat: %v\n", err)
			status = 1
		}
	}

	for _, arg := range cmd.Args {
		if !endOptions {
			switch {
			case arg == "--":
				endOptions = true
				continue
			case arg == "-":
				hasFile = true
				readStdin()
				continue
			case strings.HasPrefix(arg, "-"):
				fmt.Fprintf(hc.Stderr, "cat: invalid option: %s\n", arg)
				status = 1
				continue
			}
		}

		hasFile = true

		if arg == "/dev/null" {
			continue
		}

		data, err := c.fs.readFileAt(hc.Dir, arg)
		if err != nil {
			fmt.Fprintf(hc.Stderr, "cat: %v\n", err)
			status = 1
			continue
		}

		if _, err := hc.Stdout.Write(data); err != nil {
			fmt.Fprintf(hc.Stderr, "cat: %v\n", err)
			status = 1
			break
		}
	}

	if !hasFile {
		readStdin()
	}

	if status != 0 {
		return interp.ExitStatus(1)
	}

	return nil
}
