package shelltest

import (
	"context"
	"fmt"
	"strings"

	"github.com/kballard/go-shellquote"
	"mvdan.cc/sh/v3/interp"
)

func (c *Core) execHandler(_ interp.ExecHandlerFunc) interp.ExecHandlerFunc {
	return func(ctx context.Context, args []string) error {
		return c.dispatch(ctx, args)
	}
}

func (c *Core) dispatch(ctx context.Context, args []string) error {
	hc := interp.HandlerCtx(ctx)

	if len(args) == 0 {
		fmt.Fprintln(hc.Stderr, "empty command")
		return interp.ExitStatus(1)
	}

	handler, ok := c.handler(args[0])
	if !ok {
		fmt.Fprintf(hc.Stderr, "%s: command not found\n", args[0])
		return interp.ExitStatus(127)
	}

	return handler.Run(ctx, Command{
		Name: args[0],
		Args: args[1:],
		Dir:  virtualDir(hc.Dir),
		Sudo: sudoFromContext(ctx),
		Env:  envFromContext(ctx),
	})
}

func (c *Core) registerShellHandlers() {
	handler := Handler{Run: c.runSh, Transparent: true}

	for _, name := range []string{"sh", "/bin/sh", "bash", "/bin/bash"} {
		c.Register(name, handler)
	}

	c.Register("sudo", Handler{Run: c.runSudo, Transparent: true})
}

func (c *Core) runSh(ctx context.Context, cmd Command) error {
	hc := interp.HandlerCtx(ctx)

	if len(cmd.Args) < 2 || cmd.Args[0] != "-c" {
		fmt.Fprintf(hc.Stderr, "%s: unsupported shell invocation\n", cmd.Name)
		return interp.ExitStatus(2)
	}

	var params []string
	if len(cmd.Args) > 3 {
		params = cmd.Args[3:]
	}

	code, err := c.runScriptInDirParams(ctx, hc.Dir, cmd.Args[1], params, envFromContext(ctx), hc.Stdin, hc.Stdout, hc.Stderr)
	if err != nil {
		return err
	}

	if code != 0 {
		return interp.ExitStatus(code)
	}

	return nil
}

func (c *Core) runSudo(ctx context.Context, cmd Command) error {
	hc := interp.HandlerCtx(ctx)

	user, flags, args, err := parseSudoArgs(cmd.Args)
	if err != nil {
		fmt.Fprintln(hc.Stderr, err)
		return interp.ExitStatus(2)
	}

	if len(args) == 0 {
		fmt.Fprintln(hc.Stderr, "sudo: missing command")
		return interp.ExitStatus(1)
	}

	script := shellquote.Join(args...)

	code, err := c.runScriptInDirParams(
		withSudo(ctx, &Sudo{User: user, Flags: flags}),
		hc.Dir,
		script,
		nil,
		envFromContext(ctx),
		hc.Stdin,
		hc.Stdout,
		hc.Stderr,
	)
	if err != nil {
		return err
	}

	if code != 0 {
		return interp.ExitStatus(code)
	}

	return nil
}

// parseSudoArgs parses sudo options into the target user and remaining flags,
// returning the command arguments. It understands -u/--user; other options are
// recorded as flags.
func parseSudoArgs(args []string) (user string, flags []string, rest []string, err error) {
	i := 0

	for i < len(args) {
		arg := args[i]

		if arg == "--" {
			i++
			break
		}
		if arg == "-" || !strings.HasPrefix(arg, "-") {
			break
		}

		switch {
		case arg == "-u" || arg == "--user":
			if i+1 >= len(args) {
				return "", nil, nil, fmt.Errorf("sudo: option %s requires an argument", arg)
			}

			i++
			user = args[i]
		case strings.HasPrefix(arg, "--user="):
			user = strings.TrimPrefix(arg, "--user=")
		case strings.HasPrefix(arg, "-u") && len(arg) > 2:
			user = strings.TrimPrefix(arg[2:], "=")
		default:
			flags = append(flags, arg)
		}

		i++
	}

	return user, flags, args[i:], nil
}
