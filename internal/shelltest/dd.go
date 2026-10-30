package shelltest

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"mvdan.cc/sh/v3/interp"
)

type ddOptions struct {
	in       string
	out      string
	bs       int64
	count    int64
	hasCount bool
}

func (c *Core) runDD(ctx context.Context, cmd Command) error {
	hc := interp.HandlerCtx(ctx)

	opts, err := parseDDOptions(cmd.Args)
	if err != nil {
		fmt.Fprintf(hc.Stderr, "dd: %v\n", err)
		return interp.ExitStatus(1)
	}

	var (
		in      io.Reader
		inClose io.Closer
	)

	switch opts.in {
	case "":
		in = readerOrEmpty(hc.Stdin)
	case "/dev/null":
		in = bytes.NewReader(nil)
	default:
		f, err := c.fs.openAt(hc.Dir, opts.in, os.O_RDONLY, 0)
		if err != nil {
			fmt.Fprintf(hc.Stderr, "dd: %v\n", err)
			return interp.ExitStatus(1)
		}
		in = f
		inClose = f
	}

	if inClose != nil {
		defer inClose.Close()
	}

	var (
		out      io.Writer
		outClose io.Closer
	)

	switch opts.out {
	case "":
		out = hc.Stdout
	case "/dev/null":
		out = io.Discard
	default:
		f, err := c.fs.openAt(hc.Dir, opts.out, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o666)
		if err != nil {
			fmt.Fprintf(hc.Stderr, "dd: %v\n", err)
			return interp.ExitStatus(1)
		}
		out = f
		outClose = f
	}

	if opts.hasCount {
		in = io.LimitReader(in, opts.count*opts.bs)
	}

	buf := make([]byte, int(opts.bs))

	if _, err := io.CopyBuffer(out, in, buf); err != nil {
		if outClose != nil {
			outClose.Close()
		}

		fmt.Fprintf(hc.Stderr, "dd: %v\n", err)
		return interp.ExitStatus(1)
	}

	if outClose != nil {
		if err := outClose.Close(); err != nil {
			fmt.Fprintf(hc.Stderr, "dd: %v\n", err)
			return interp.ExitStatus(1)
		}
	}

	return nil
}

func parseDDOptions(args []string) (ddOptions, error) {
	opts := ddOptions{bs: 32 * 1024}

	for _, arg := range args {
		key, value, ok := strings.Cut(arg, "=")
		if !ok {
			return opts, fmt.Errorf("unsupported operand: %s", arg)
		}

		switch key {
		case "if":
			opts.in = value
		case "of":
			opts.out = value
		case "bs":
			bs, err := strconv.ParseInt(value, 10, 64)
			if err != nil || bs <= 0 {
				return opts, fmt.Errorf("invalid bs: %s", value)
			}
			opts.bs = bs
		case "count":
			count, err := strconv.ParseInt(value, 10, 64)
			if err != nil || count < 0 {
				return opts, fmt.Errorf("invalid count: %s", value)
			}
			opts.count = count
			opts.hasCount = true
		case "status":
			if value != "none" {
				return opts, fmt.Errorf("unsupported status: %s", value)
			}
		default:
			return opts, fmt.Errorf("unsupported operand: %s", key)
		}
	}

	return opts, nil
}

func readerOrEmpty(r io.Reader) io.Reader {
	if r == nil {
		return bytes.NewReader(nil)
	}

	return r
}
