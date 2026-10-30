# The `gabricdocker` package

`gabricdocker` installs a Docker runner and stores the current container in
context. It uses the local backend to run `docker exec`.

## Install and flags

```go
func Install(g *gabric.Gabric)
func InstallDefault()
```

`Install` registers `-docker-container` and a setup middleware through
`UseBuiltin`. It must run before `Main` parses flags. `InstallDefault` uses
`gabric.Default`.

`Install` installs the Docker runner even when the container is empty. An empty
container produces `ErrNoContainer` when an operation uses it.

## Context

```go
func NewContainerContext(ctx context.Context, container string) context.Context
func GetContainer(ctx context.Context) (string, error)
func HasContainer(ctx context.Context) bool

var ErrNoContainer = errors.New("gabricdocker: no container")
```

`GetContainer` returns `ErrNoContainer` when the container is unset or empty.
`HasContainer` reports a non-empty container.

## Runner

```go
func Runner() gabric.RemoteRunner
```

The runner reads the container from the context on each call, then executes:

```sh
docker exec -i <container> /bin/sh -c <cmd>
```

It uses `gabric.NewLocalRunner`, so local backend injection and streams work the
same as other local runners. The real local backend returns
`*gabric.ExecExitError` for a non-zero command status. It passes `-i` but not
`-t`.

## Choosing a form

| Goal                                                            | API                                   |
| --------------------------------------------------------------- | ------------------------------------- |
| Run tasks with the `-docker-container` flag                     | `Install` plus `gabric.Remote(ctx)`   |
| Run one operation in a specific container                       | `gabricdocker.Remote(ctx, container)` |
| Make target operations, including Reconcile modules, use Docker | `gabricdocker.Target(container)`      |
| Build a runner without installing context                       | `gabricdocker.Runner()`               |

## Middleware

```go
func WithDocker(ctx context.Context, next middleware.Next) error
func WithContainer(container string) middleware.Middleware
```

`WithDocker` installs the Docker runner and `NoRsyncRsh` when no remote runner
is installed:

```go
gabric.NewRemoteRunnerContext(ctx, Runner())
gabric.NewRsyncRshContext(ctx, gabric.NoRsyncRsh)
```

The `HasRemoteRunner` guard means the first transport wins.

`WithContainer` only sets the container. It does not install a runner.

The runner is installed once, but it reads the current container per command.
A later `WithContainer` changes the container without replacing the runner.

## Explicit remote operations

```go
func Remote(ctx context.Context, container string) *gabric.RemoteOps
func Target(container string) gabric.Target
```

### `Remote`

```go
ops := gabricdocker.Remote(ctx, "app")
ops.Exec("uname", "-a").Run()
```

`Remote` does three things:

```go
ctx = NewContainerContext(ctx, container)
ctx = gabric.NewRemoteRunnerContext(ctx, Runner())
ctx = gabric.NewRsyncRshContext(ctx, gabric.NoRsyncRsh)
return gabric.Remote(ctx)
```

It overrides an existing runner and bypasses the `HasRemoteRunner` guard because
the caller asked for this container explicitly.

The caller's `ctx` is not modified. The returned ops use a derived context. This
does not configure the caller's context for later operations:

```go
gabricdocker.Remote(ctx, "app").Exec("true").Run()

// Still uses the original context and runner.
gabric.Remote(ctx).Exec("true").Run()
```

### `Target`

`Target` returns a `gabric.Target`:

```go
func Target(container string) gabric.Target
```

It is used with `NewTargetContext`:

```go
targetCtx := gabric.NewTargetContext(ctx, gabricdocker.Target("app"))
gabric.TargetOps(targetCtx).Exec("uname", "-a").Run()
```

The target calls `Remote(ctx, container)` with the context passed to `TargetOps`
and appends the remote exec environment for Docker commands. Reconcile modules
need this wiring: they call `TargetOps(ctx)` throughout a run, and each call must
build Docker operations from the current context.

`Target` supplies the target value without setting the global target:

```go
ctx = gabric.NewTargetContext(ctx, gabricdocker.Target("app"))
```

`Remote` and `Target` do not register tasks.

### `Remote` versus `Target`

| API                      | Result              | Use                          |
| ------------------------ | ------------------- | ---------------------------- |
| `Remote(ctx, container)` | `*gabric.RemoteOps` | Run operations now           |
| `Target(container)`      | `gabric.Target`     | Wire Docker into `TargetOps` |

## Rsync

Docker rsync uses the native sidecar transport only. `gabricdocker` installs
`NoRsyncRsh`, so a platform without inherited file descriptors cannot fall back
to system `ssh`.

## Library use

```go
ctx = gabric.NewRemoteRunnerContext(ctx, gabricdocker.Runner())
ctx = gabric.NewRsyncRshContext(ctx, gabric.NoRsyncRsh)
ctx = gabricdocker.NewContainerContext(ctx, "app")

err := gabric.Remote(ctx).Exec("uname", "-a").RunErr()
```

Or install the middleware and set the container for a scope:

```go
err := gabric.Run(ctx, calls,
	gabricdocker.WithDocker,
	gabricdocker.WithContainer("app"),
)
```

## No shell

There is no `gabricdocker.Shell`. Interactive shells remain SSH-only in
`gabricssh`.
