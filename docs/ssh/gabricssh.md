# The `gabricssh` package

`gabricssh` is the Gabric SSH integration. It installs the SSH runner, manages
the host, config and connection manager in context, and provides the interactive
shell, port forwarding and SSH task registration.

The low-level client is in [`sshremote`](sshremote.md).

## Install and flags

```go
package main

import (
	"github.com/bancek/gabric"
	"github.com/bancek/gabric/gabricssh"
)

func main() {
	gabricssh.InstallDefault()
	gabric.Main()
}
```

`Install(g)` installs on an explicit `*gabric.Gabric`. `InstallDefault()`
installs on `gabric.Default`. `Install` must run before `Main` parses flags.
Calling it twice panics.

`Install` registers flags and setup middleware. It does not register tasks.

| Flag            | Meaning                            |
| --------------- | ---------------------------------- |
| `-ssh-host`     | SSH host, in `user@host:port` form |
| `-ssh-identity` | SSH identity file; repeatable      |

The host is stored lazily. It is resolved on the first remote use, so a main
middleware can change the config before resolution. Identities are appended
after the config defaults.

## Context

Host:

```go
func NewHostContext(ctx context.Context, host *sshremote.Host) context.Context
func NewHostNameContext(ctx context.Context, spec string) context.Context
func GetHost(ctx context.Context) *sshremote.Host
func GetHostErr(ctx context.Context) (*sshremote.Host, error)
func HasHost(ctx context.Context) bool
func ResolveHost(ctx context.Context, spec string) (*sshremote.Host, error)
```

`NewHostNameContext` stores the full spec, including an optional user and port.
Resolution is lazy and memoized. `GetHost` panics when no host is present or
resolution fails; `GetHostErr` returns the error. The error is `ErrNoHost` when
the context has no host.

Config:

```go
func NewConfigContext(ctx context.Context, config *sshremote.Config) context.Context
func GetConfig(ctx context.Context) *sshremote.Config
func HasConfig(ctx context.Context) bool
```

Manager:

```go
func NewManagerContext(ctx context.Context, manager *sshremote.RemoteManager) context.Context
func GetManager(ctx context.Context) *sshremote.RemoteManager
func HasManager(ctx context.Context) bool
func WithManager(ctx context.Context, next middleware.Next) error
func GetHostConfig(ctx context.Context) sshremote.HostConfig
func GetRemote(ctx context.Context) *sshremote.Remote
```

`WithManager` creates a manager when absent, closes only the manager it created,
and leaves an existing manager alone.

`GetRemote` returns the pooled remote for the current host and config.

## Runner

```go
func Runner() gabric.RemoteRunner
func NewRunnerContext(ctx context.Context) context.Context
```

`Runner` returns the SSH runner. It runs each command in a session, wires the
streams, and maps `*ssh.ExitError` to `*gabric.ExecExitError`:

```go
err := gabric.Remote(ctx).Exec("systemctl", "restart", "app").RunErr()

if exitErr, ok := errors.AsType[*gabric.ExecExitError](err); ok {
	// exitErr.Status is the remote status
}
```

The underlying `*ssh.ExitError` is still reachable through `Unwrap`.

`NewRunnerContext` installs the SSH runner and rsync rsh resolver explicitly:

```go
ctx = gabricssh.NewRunnerContext(ctx)
```

It assumes the SSH config, manager and host are already in the context, or will
be set later. It does not create a manager.

## Choosing a form

| Goal                                               | API                                     |
| -------------------------------------------------- | --------------------------------------- |
| Run tasks with `-ssh-host`                         | `Install` plus `gabric.Remote(ctx)`     |
| Run one operation with a managed manager           | `gabricssh.WithRemote(ctx, host, next)` |
| Run one operation with existing config and manager | `gabricssh.Remote(ctx, host)`           |
| Make `TargetOps` and Reconcile modules use SSH     | `gabricssh.Target(host)`                |
| Run a whole scope with SSH setup                   | `WithSSH` plus `WithHost`               |
| Build a runner without installing context          | `gabricssh.Runner()`                    |

## Middleware

```go
func WithSSH(ctx context.Context, next middleware.Next) error
func WithHost(host string) middleware.Middleware
func WithRemote(ctx context.Context, host string, next middleware.Next) error
```

`WithSSH` always installs:

- an SSH config, unless one already exists;
- a connection manager, unless one already exists.

It installs the SSH runner and rsync rsh resolver only when no remote runner is
already installed. The first transport wins. It does not set a host.

If it creates the manager, it closes it after `next`. An app-supplied manager is
left open.

`WithHost` only sets a host. It does not install a runner.

`WithRemote` is the scoped setup for one host:

```go
err := gabricssh.WithRemote(ctx, "deploy@example.com", func(ctx context.Context) error {
	return gabric.Remote(ctx).Exec("uname", "-a").RunErr()
})
```

It creates the default config and manager if needed, sets the host, installs the
SSH runner and rsync resolver, and closes a manager it created. An existing
config or manager is reused.

## Explicit remote operations

```go
func Remote(ctx context.Context, host string) *gabric.RemoteOps
func Target(host string) gabric.Target
```

### `Remote`

```go
ops := gabricssh.Remote(ctx, "deploy@example.com")
ops.Exec("uname", "-a").Run()
```

`Remote` requires an SSH config and manager in context. It sets the host, installs
the SSH runner, and returns `gabric.Remote(ctx)`. It does not create or close a
manager.

The caller's `ctx` is unchanged. The returned ops use a derived context.

### `Target`

`Target` returns a provider, not ops:

```go
func Target(host string) gabric.Target
```

It is used with `NewTargetContext`:

```go
targetCtx := gabric.NewTargetContext(ctx, gabricssh.Target("deploy@example.com"))
gabric.TargetOps(targetCtx).Exec("uname", "-a").Run()
```

The target calls `Remote(ctx, host)` with the context passed to `TargetOps` and
appends the remote exec environment for SSH commands. Reconcile modules need
this wiring.

`Target` supplies the provider without setting the global target.

### `Remote` versus `Target`

| API                 | Result              | Use                       |
| ------------------- | ------------------- | ------------------------- |
| `Remote(ctx, host)` | `*gabric.RemoteOps` | Run operations now        |
| `Target(host)`      | `gabric.Target`     | Wire SSH into `TargetOps` |

## Rsync fallback

`NewRunnerContext` also installs the SSH rsync rsh resolver. It is used only
when the native rsync transport is unavailable. The resolver returns the host
spec and the system `ssh` command with the effective SSH options.

The resolver is installed only with the SSH runner. A custom runner does not get
an SSH fallback.

## Shell

```go
func Shell(ctx context.Context) *ShellOps
```

`Shell` opens an interactive login shell on the current host. It requests a PTY
when stdin is a terminal, forwards terminal resizes, and uses the process stdio
by default. See [shell.md](shell.md).

A non-zero shell exit is returned as `*gabric.ExecExitError` wrapping
`*ssh.ExitError`.

## Forward

```go
func Forward(ctx context.Context) *ForwardOps
```

`Forward` opens a local port forward through the SSH server. It owns a dedicated
connection and blocks until the context is cancelled. See
[forward.md](forward.md).

## Tasks

```go
var Ssh = gabricssh.RegisterSshTask(gabric.Default)
var Forward = gabricssh.RegisterForwardTask(gabric.Default)
```

`RegisterSshTask(g)` registers an `ssh` task that runs `Shell(ctx)`.
`RegisterForwardTask(g)` registers a `forward` task with a
`<local>,<remote>` argument.

`Install` does not register these tasks.

## Library use

One-shot:

```go
err := gabricssh.WithRemote(ctx, "deploy@example.com", func(ctx context.Context) error {
	return gabric.Remote(ctx).Exec("uname", "-a").RunErr()
})
```

With middleware:

```go
err := gabric.Run(ctx, calls,
	gabricssh.WithSSH,
	gabricssh.WithHost("deploy@example.com"),
)
```

Explicit context:

```go
ctx = gabricssh.NewConfigContext(ctx, config)
ctx = gabricssh.NewManagerContext(ctx, manager)
ctx = gabricssh.NewHostContext(ctx, host)
ctx = gabricssh.NewRunnerContext(ctx)
```

Target operations:

```go
ctx = gabric.NewTargetContext(ctx, gabricssh.Target("deploy@example.com"))
gabric.TargetOps(ctx).Exec("uname", "-a").Run()
```
