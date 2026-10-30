package shelltest

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"

	"github.com/kballard/go-shellquote"
	"mvdan.cc/sh/v3/expand"
	"mvdan.cc/sh/v3/interp"
	"mvdan.cc/sh/v3/syntax"
)

// Core runs shell scripts in-process for tests.
type Core struct {
	recorder *Recorder
	fs       *FS

	handlersMu sync.RWMutex
	handlers   map[string]Handler
}

// New returns a shell test core.
func New() *Core {
	core := &Core{
		recorder: &Recorder{},
		fs:       NewFS(),
		handlers: map[string]Handler{},
	}

	core.registerShellHandlers()
	core.Register("cat", Handler{Run: core.runCat})
	core.Register("dd", Handler{Run: core.runDD})
	core.Register("mkdir", Handler{Run: core.runMkdir})
	core.Register("mv", Handler{Run: core.runMV})
	core.Register("rm", Handler{Run: core.runRM})
	core.Register("chmod", Handler{Run: core.runChmod})
	core.Register("chown", Handler{Run: core.runChown})
	core.Register("sha256sum", Handler{Run: core.runSha256Sum})
	core.Register("stat", Handler{Run: core.runStat})
	core.Register("uname", Handler{Run: core.runUname})

	return core
}

// FS returns the core's in-memory filesystem.
func (c *Core) FS() *FS {
	return c.fs
}

// RunScript runs script and returns its exit status. A non-nil error means the
// script could not be run or the shell failed outside a normal exit status.
func (c *Core) RunScript(ctx context.Context, script string, stdin io.Reader, stdout, stderr io.Writer) (int, error) {
	return c.RunScriptEnv(ctx, script, nil, stdin, stdout, stderr)
}

// RunScriptEnv runs script with env and returns its exit status.
func (c *Core) RunScriptEnv(ctx context.Context, script string, env []string, stdin io.Reader, stdout, stderr io.Writer) (int, error) {
	return c.runScriptInDirParams(ctx, "/", script, nil, env, stdin, stdout, stderr)
}

// RunArgs runs args as a command in dir and returns its exit status.
func (c *Core) RunArgs(ctx context.Context, dir string, args []string, stdin io.Reader, stdout, stderr io.Writer) (int, error) {
	return c.RunArgsEnv(ctx, dir, args, nil, stdin, stdout, stderr)
}

// RunArgsEnv runs args as a command in dir with env and returns its exit
// status.
func (c *Core) RunArgsEnv(ctx context.Context, dir string, args []string, env []string, stdin io.Reader, stdout, stderr io.Writer) (int, error) {
	return c.runScriptInDirParams(ctx, dir, shellquote.Join(args...), nil, env, stdin, stdout, stderr)
}

func (c *Core) runScriptInDirParams(ctx context.Context, dir, script string, params []string, env []string, stdin io.Reader, stdout, stderr io.Writer) (int, error) {
	dir = virtualDir(dir)
	ctx = withEnv(ctx, env)

	parser := syntax.NewParser(syntax.Variant(syntax.LangPOSIX))

	node, err := parser.Parse(strings.NewReader(script), "")
	if err != nil {
		if stderr != nil {
			fmt.Fprintln(stderr, err)
		}

		return 1, err
	}

	environ := make([]string, 0, 1+len(env))
	environ = append(environ, "PATH=/usr/bin:/bin")
	environ = append(environ, env...)

	opts := []interp.RunnerOption{
		interp.Env(expand.ListEnviron(environ...)),
		interp.Dir(hostRoot()),
		interp.StdIO(stdin, stdout, stderr),
		interp.CallHandler(c.callHandler),
		interp.ExecHandlers(c.execHandler),
		interp.StatHandler(c.statHandler),
		interp.ReadDirHandler2(c.readDirHandler),
		interp.AccessHandler(c.accessHandler),
		interp.OpenHandler(c.openHandler),
	}
	if params != nil {
		opts = append(opts, interp.Params(append([]string{"--"}, params...)...))
	}

	runner, err := interp.New(opts...)
	if err != nil {
		if stderr != nil {
			fmt.Fprintln(stderr, err)
		}

		return 1, err
	}

	// interp.Dir stats the host filesystem, so start at the host root and set
	// the sandboxed working directory directly.
	runner.Dir = hostDir(dir)

	if err := runner.Run(ctx, node); err != nil {
		if status, ok := errors.AsType[interp.ExitStatus](err); ok {
			return int(status), nil
		}

		if stderr != nil {
			fmt.Fprintln(stderr, err)
		}

		return 1, err
	}

	return 0, nil
}
