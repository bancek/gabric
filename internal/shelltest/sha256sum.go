package shelltest

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"strings"

	"mvdan.cc/sh/v3/interp"
)

func (c *Core) runSha256Sum(ctx context.Context, cmd Command) error {
	hc := interp.HandlerCtx(ctx)

	endOptions := false
	paths := []string{}

	for _, arg := range cmd.Args {
		if !endOptions {
			switch {
			case arg == "--":
				endOptions = true
				continue
			case arg == "-":
				paths = append(paths, arg)
				continue
			case strings.HasPrefix(arg, "-"):
				fmt.Fprintf(hc.Stderr, "sha256sum: invalid option: %s\n", arg)
				return interp.ExitStatus(1)
			}
		}

		paths = append(paths, arg)
	}

	if len(paths) == 0 {
		paths = []string{"-"}
	}

	status := 0

	for _, p := range paths {
		var (
			data []byte
			err  error
		)

		switch p {
		case "-":
			data, err = io.ReadAll(readerOrEmpty(hc.Stdin))
		case "/dev/null":
			data = nil
		default:
			data, err = c.fs.readFileAt(hc.Dir, p)
		}

		if err != nil {
			fmt.Fprintf(hc.Stderr, "sha256sum: %v\n", err)
			status = 1
			continue
		}

		sum := sha256.Sum256(data)
		fmt.Fprintf(hc.Stdout, "%s  %s\n", hex.EncodeToString(sum[:]), p)
	}

	if status != 0 {
		return interp.ExitStatus(1)
	}

	return nil
}
