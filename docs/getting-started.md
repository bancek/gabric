# Getting started

This guide walks through defining tasks, running them, and doing local and
remote work. For the underlying model, see [architecture.md](architecture.md).

## Install

```bash
go get github.com/bancek/gabric
```

## Minimal program

A task is an ordinary Go function. `gabric.Task` registers it under a name and
returns it for composition; `gabric.Main` runs the CLI.

```go
package main

import (
	"context"

	"github.com/bancek/gabric"
)

var HelloLocal = gabric.Task("hello_local", func(ctx context.Context) {
	gabric.Local(ctx).Exec("echo", "Hello local").Run()
})

var Hello = gabric.Task("hello", func(ctx context.Context) {
	HelloLocal(ctx)
})

func main() {
	gabric.Main()
}
```

`gabric.Task` is panic-style: panics from the body are recovered at the task
boundary and reported as task failures (`Main` logs and exits non-zero). For a
task that returns errors explicitly, use `gabric.TaskErr` with a
`func(ctx context.Context) error` body.

List tasks:

```bash
go run . 2> /dev/null
```

```
hello
hello_local
```

Run one:

```bash
go run . hello_local
```

`HelloLocal(ctx)` inside `Hello` is a normal Go call. There is no subprocess
boundary.

## Flags

Core flags:

| Flag          | Meaning                      | Example            |
| ------------- | ---------------------------- | ------------------ |
| `-level`      | log level (default `debug`)  | `-level info`      |
| `-log-format` | log format: `text` or `json` | `-log-format json` |
| `-dry-run`    | probe only; do not mutate    | `-dry-run`         |

After `gabricssh.Install`, the SSH flags are also available:

| Flag            | Meaning                           | Example                           |
| --------------- | --------------------------------- | --------------------------------- |
| `-ssh-host`     | SSH host in `user@host:port` form | `-ssh-host deploy@example.com`    |
| `-ssh-identity` | SSH identity file; repeatable     | `-ssh-identity ~/.ssh/deploy_key` |

```bash
go run . -ssh-host deploy@example.com hello
```

`-dry-run` is a built-in runner flag; tasks check `dryrun.Enabled(ctx)` before
mutating.

Application flags are registered on the runner's flag set:

```go
var stage = gabric.Default.FlagSet.String("stage", "staging", "deployment stage")

gabric.Main(gabric.WithMiddleware(func(ctx context.Context, next middleware.Next) error {
	ctx = context.WithValue(ctx, stageKey{}, *stage)
	return next(ctx)
}))
```

## Task arguments

Tasks are invoked as `name:arg`:

```bash
go run . deploy:staging
```

Read the argument inside the task with `gabric.GetCurrentTaskArg(ctx)`:

```go
var Deploy = gabric.Task("deploy", func(ctx context.Context) {
	stage := gabric.GetCurrentTaskArg(ctx)
	gabric.Local(ctx).Exec("echo", "deploying to", stage).Run()
})
```

## Errors and probes

`Task` bodies panic. `TaskErr` bodies return errors. Most operations have both forms, so you can choose whether a failure should stop the task:

```go
var Check = gabric.TaskErr("check", func(ctx context.Context) error {
	return gabric.Local(ctx).Exec("test", "-d", "/srv/app").RunErr()
})

var Deploy = gabric.Task("deploy", func(ctx context.Context) {
	if err := Check(ctx); err != nil {
		gabric.Logger(ctx).Warn("app is not ready", "error", err)
	}

	exists, err := gabric.Local(ctx).Exists("/srv/app").Dir().RunErr()
	if err != nil {
		panic(err)
	}
	if !exists {
		panic("missing /srv/app")
	}
})
```

`gabric.Must(err)` panics when the error is not nil. It is useful when a panic-style task calls an error-first helper:

```go
gabric.Must(Check(ctx))
```

## Remote operations

`gabric.Remote(ctx)` runs commands, transfers files and rsyncs against the host in
the context. Install the SSH transport and set `-ssh-host`:

```go
import (
	"github.com/bancek/gabric"
	"github.com/bancek/gabric/gabricssh"
)

func main() {
	gabricssh.InstallDefault()
	gabric.Main()
}
```

```go
var HelloRemote = gabric.Task("hello_remote", func(ctx context.Context) {
	gabric.Remote(ctx).Exec("echo", "Hello remote").Run()

	gabric.Remote(ctx).Rsync().
		Local("./app").
		Remote("/srv/app").
		Upload().
		Run()
})
```

```bash
go run . -ssh-host deploy@example.com hello_remote
```

Hosts are resolved through `~/.ssh/config`, default private keys and the
known_hosts file. Users default to the current OS user.

## Files and directories

`Put`, `Get`, `Mkdir`, `Remove`, `Exists`, and `Stat` are builders. `List` returns entries directly. They work with both `Local(ctx)` and `Remote(ctx)`.

```go
gabric.Remote(ctx).Put("/srv/app/config").String(config).Mode(0o600).Run()
data := gabric.Remote(ctx).Get("/srv/app/version").Bytes()
gabric.Remote(ctx).Mkdir("/srv/app/state").Recursive().Mode(0o700).Run()
gabric.Remote(ctx).Remove("/srv/app/old").Recursive().Force().Run()

if gabric.Remote(ctx).Exists("/srv/app/app").Run() {
	// ...
}

stat := gabric.Remote(ctx).Stat("/srv/app").Run()

entries := gabric.Remote(ctx).List("/srv/app")
```

`Put` takes its content from `Reader`, `String`, or `Bytes`. `Get` can return bytes or a string, copy to a writer, or pass the live reader to a callback. `Remove` deletes a file or directory; use `Recursive` for non-empty directories and `Force` to ignore a missing path. Every operation has a panic form and an `*Err` form. `List` panics on failure; `ListErr` returns the error.

## Interactive shell

`gabricssh.Shell(ctx)` opens an interactive login shell on the host from the
context, over Gabric's own SSH stack (no system `ssh`):

```go
var Ssh = gabricssh.RegisterSshTask(gabric.Default)
```

```bash
go run . -ssh-host deploy@example.com ssh
```

The builder uses the process stdio, requests a PTY and follows the local
terminal size and resizes. Use `Term`, `Size`, `Pty` and
`Stdin`/`Stdout`/`Stderr` to override the defaults. See
[ssh/shell.md](ssh/shell.md) for details.

## Local port forwarding

`gabricssh.Forward(ctx)` opens a local `ssh -L` tunnel to the host from the
context, over Gabric's own SSH stack:

```go
var Forward = gabricssh.RegisterForwardTask(gabric.Default)
```

```bash
go run . -ssh-host deploy@example.com forward:127.0.0.1:8080,127.0.0.1:8080
```

The task argument is `<local>,<remote>` in `host:port` form; a single address
applies to both sides. See [ssh/forward.md](ssh/forward.md) for details.

## Target operations

Reconcile modules use `gabric.TargetOps(ctx)` to choose between remote and local
operations. `Main` and `Run` use a remote target by default.

How `TargetOps(ctx)` behaves for each built-in target:

| Target              | Commands                | File ops       | Sudo             |
| ------------------- | ----------------------- | -------------- | ---------------- |
| `LocalTarget`       | direct Go `exec`        | native Go `os` | none             |
| `LocalRemoteTarget` | `/bin/sh`               | coreutils      | `WithRemoteSudo` |
| `RemoteTarget`      | installed remote runner | coreutils      | `WithRemoteSudo` |

Selecting a target changes only `TargetOps`:

| Active target       | `TargetOps(ctx)` | `Remote(ctx)`           | `Local(ctx)` |
| ------------------- | ---------------- | ----------------------- | ------------ |
| `LocalTarget`       | `Local`          | installed remote runner | `Local`      |
| `LocalRemoteTarget` | `LocalRemote`    | installed remote runner | `Local`      |
| `RemoteTarget`      | `Remote`         | installed remote runner | `Local`      |

`Remote(ctx)` and `Local(ctx)` are explicit escapes. In particular,
`LocalRemoteTarget` does not make `Remote(ctx)` local.

Use `LocalTarget` for native local work, such as using modules to create local
files and directories. Use `LocalRemoteTarget` when a binary that also runs on a
remote Unix host should execute in place with the same shell, coreutils, paths,
environment, and sudo behavior. `LocalTarget` operations work on Windows;
individual module support still depends on the module. `LocalRemoteTarget` needs
`/bin/sh` and coreutils and is Unix-like only, and is the local target to use
when you need sudo or rsync.

See [target operations](api.md#target-operations) in the API reference for
transport-provided targets and examples.

Override the default with middleware:

```go
gabric.Main(gabric.WithMiddleware(gabric.WithLocalTarget))
```

Or set the target directly:

```go
ctx = gabric.NewTargetContext(ctx, gabric.LocalTarget)
```

`LocalTarget` uses `Local`, `AppendLocalExecEnvContext` and
`NewLocalWorkingDirContext`; `RemoteTarget` and `LocalRemoteTarget` use the
remote equivalents. Target operations accept explicit working-directory and
exec-environment overlays:

```go
ctx = gabric.NewTargetOpsWorkingDirContext(ctx, "/srv/app")
ctx = gabric.NewTargetOpsExecEnvContext(ctx, gabric.Env("DEBIAN_FRONTEND", "noninteractive"))
```

`ops.WithWorkingDir(...)` and `ops.WithExecEnv(...)` override those overlays for
their callback.

Direct Reconcile use must set a target explicitly:

```go
ctx = gabric.NewTargetContext(ctx, gabric.RemoteTarget)
```

Use `LocalRemoteTarget` when commands should run on the local machine but keep
remote command, working-directory, and sudo behavior:

```go
ctx = gabric.NewTargetContext(ctx, gabric.LocalRemoteTarget)
```

or:

```go
gabric.Main(gabric.WithMiddleware(gabric.WithLocalRemoteTarget))
```

If remote sudo is configured, `LocalRemote` still needs working sudo
permissions. Without it, commands run as the current user. `LocalRemote` covers
commands, files, and rsync (through the native transport, Unix only); shell and
forwarding stay remote.

For privileged local operations, use `LocalRemoteTarget` with
`NewRemoteSudoContext` or `WithRemoteSudo`. There is no separate local-sudo
context API; remote sudo is the sudo setting for remote-style operations,
including `LocalRemote`.

Use `HasTarget(ctx)` when you need to check whether a target is already set.
Target ops overlays apply only to target operations; direct `Local(ctx)` /
`Remote(ctx)` calls do not see them.

## Initialization

`Main` installs logging, remote configuration and host state before tasks run.
Runtime state is extended with middleware. Main middleware runs after the
built-in context setup and before global middleware, and it can use application
flags:

```go
var _ = gabric.Use(gabric.WithReconcile)

gabric.Main(gabric.WithMiddleware(func(ctx context.Context, next middleware.Next) error {
	// run remote commands and transfers with sudo as root
	ctx = gabric.NewRemoteSudoContext(ctx, &gabric.Sudo{})

	// add your own values
	ctx = context.WithValue(ctx, appKey, loadApp())

	return next(ctx)
}))
```

Middleware is composable: each receives a context and calls `next` with a
derived one. `WithContext(ctx)` supplies a base context, for example one with a
custom logger.

## Capturing output

By default command output streams to the process. Capture it when you need the
value:

```go
rev := gabric.Local(ctx).
	Exec("git", "rev-parse", "HEAD").
	CaptureStdout().
	Run().
	StdoutStr()
```

For steps where you want a quiet log but errors surfaced, use `Quiet` or
`LogErrors`:

```go
gabric.Remote(ctx).Exec("systemctl", "restart", "app").Quiet().Run()
```

See [architecture.md](architecture.md#command-execution) for the full output
model.

## Writing your own main

`gabric.Main` is optional. The phases are explicit:

```go
g := gabric.New()
gabricssh.Install(g)
g.Task("hello", func(ctx context.Context) {
	gabric.Remote(ctx).Exec("uname", "-a").Run()
})
g.Args = []string{"-ssh-host", "deploy@example.com", "hello"}

if err := g.ParseFlags(g.Args); err != nil {
	panic(err)
}

calls, err := g.ParseTasks(g.FlagSet.Args())
if err != nil {
	panic(err)
}

if err := g.Run(context.Background(), calls); err != nil {
	panic(err)
}
```

`Run` applies core built-in middleware (dry-run and default remote target),
installed transport setup from `UseBuiltin`, and global middleware registered
with `Use`. Supply a base context to `WithContext` when you use the `Main`
convenience path, or compose `BuiltinMiddlewares`/`GlobalMiddlewares` with
`middleware.Apply` yourself.

## Next steps

- [architecture.md](architecture.md): execution model, context layout,
  concurrency, middleware, and error semantics.
- [ssh/gabricssh.md](ssh/gabricssh.md): SSH installation, context and shell.
- [ssh/sshremote.md](ssh/sshremote.md): hosts, config, pooling, auth, and host keys.
- `go doc github.com/bancek/gabric`: API reference.
