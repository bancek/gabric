package shelltest

import (
	"context"
	"fmt"
	"io/fs"
	"strings"

	"mvdan.cc/sh/v3/interp"
)

func (c *Core) runStat(ctx context.Context, cmd Command) error {
	hc := interp.HandlerCtx(ctx)

	format := ""
	dereference := false
	endOptions := false
	paths := []string{}

	for i := 0; i < len(cmd.Args); i++ {
		arg := cmd.Args[i]

		if !endOptions {
			switch {
			case arg == "--":
				endOptions = true
				continue
			case arg == "-c":
				i++
				if i >= len(cmd.Args) {
					fmt.Fprintln(hc.Stderr, "stat: missing format")
					return interp.ExitStatus(1)
				}
				format = cmd.Args[i]
				continue
			case strings.HasPrefix(arg, "--format="):
				format = strings.TrimPrefix(arg, "--format=")
				continue
			case arg == "--format":
				i++
				if i >= len(cmd.Args) {
					fmt.Fprintln(hc.Stderr, "stat: missing format")
					return interp.ExitStatus(1)
				}
				format = cmd.Args[i]
				continue
			case arg == "-L" || arg == "--dereference":
				dereference = true
				continue
			case strings.HasPrefix(arg, "-"):
				fmt.Fprintf(hc.Stderr, "stat: invalid option: %s\n", arg)
				return interp.ExitStatus(1)
			}
		}

		paths = append(paths, arg)
	}

	if format == "" {
		fmt.Fprintln(hc.Stderr, "stat: format required")
		return interp.ExitStatus(1)
	}

	if !validStatFormat(format) {
		fmt.Fprintf(hc.Stderr, "stat: unsupported format: %s\n", format)
		return interp.ExitStatus(1)
	}

	if len(paths) == 0 {
		fmt.Fprintln(hc.Stderr, "stat: missing operand")
		return interp.ExitStatus(1)
	}

	status := 0

	for _, p := range paths {
		info, err := c.fs.statAt(hc.Dir, p, dereference)
		if err != nil {
			fmt.Fprintf(hc.Stderr, "stat: %v\n", err)
			status = 1
			continue
		}

		user, group, err := c.fs.ownerAt(hc.Dir, p, dereference)
		if err != nil {
			fmt.Fprintf(hc.Stderr, "stat: %v\n", err)
			status = 1
			continue
		}

		fmt.Fprintln(hc.Stdout, expandStatFormat(format, info, user, group))
	}

	if status != 0 {
		return interp.ExitStatus(1)
	}

	return nil
}

func validStatFormat(format string) bool {
	for i := 0; i < len(format); i++ {
		if format[i] != '%' {
			continue
		}

		i++
		if i >= len(format) {
			return false
		}

		switch format[i] {
		case '%', 'a', 'f', 'U', 'G':
		default:
			return false
		}
	}

	return true
}

func expandStatFormat(format string, info fs.FileInfo, user, group string) string {
	var b strings.Builder

	for i := 0; i < len(format); i++ {
		if format[i] != '%' {
			b.WriteByte(format[i])
			continue
		}

		i++

		switch format[i] {
		case '%':
			b.WriteByte('%')
		case 'a':
			fmt.Fprintf(&b, "%o", info.Mode().Perm())
		case 'f':
			fmt.Fprintf(&b, "%x", unixRawMode(info.Mode()))
		case 'U':
			b.WriteString(user)
		case 'G':
			b.WriteString(group)
		}
	}

	return b.String()
}

func unixRawMode(mode fs.FileMode) uint32 {
	raw := uint32(mode.Perm())

	if mode&fs.ModeSetuid != 0 {
		raw |= 0o4000
	}
	if mode&fs.ModeSetgid != 0 {
		raw |= 0o2000
	}
	if mode&fs.ModeSticky != 0 {
		raw |= 0o1000
	}

	switch {
	case mode.IsDir():
		raw |= 0o040000
	case mode&fs.ModeSymlink != 0:
		raw |= 0o120000
	case mode&fs.ModeNamedPipe != 0:
		raw |= 0o010000
	case mode&fs.ModeSocket != 0:
		raw |= 0o140000
	case mode&fs.ModeCharDevice != 0:
		raw |= 0o020000
	case mode&fs.ModeDevice != 0:
		raw |= 0o060000
	default:
		raw |= 0o100000
	}

	return raw
}
