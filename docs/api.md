# API reference

User-facing reference for the API you are most likely to use, with a short
example for each. For internals and design rationale see
[architecture.md](architecture.md); for task-oriented recipes see
[cookbook.md](cookbook.md). For every exported symbol use
`go doc github.com/bancek/gabric` (and `./input`, `./gabricssh`, `./gabricdocker`, `./sshremote`, `./logger`).

Items are marked as `pkg.Symbol`.

## Tasks and registry

### Types

```go
// Package middleware
type Next       = func(ctx context.Context) error
type Middleware = func(ctx context.Context, next Next) error

// Package gabric
type TaskFunc = middleware.Next
```

`TaskFunc` and `middleware.Next` are aliases for the same function shape, so
ordinary functions with those signatures work directly. The middleware types
live in `github.com/bancek/gabric/middleware`. A task receives all state through
`ctx` and returns an error. `Task` wraps a panic-style `func(ctx)` body into a
`TaskFunc`; `TaskErr` registers an error-first body directly.

### `Task`, `TaskErr`

```go
var HelloLocal = gabric.Task("hello_local", func(ctx context.Context) {
	gabric.Local(ctx).Exec("echo", "hello").Run()
})

var Check = gabric.TaskErr("check", func(ctx context.Context) error {
	return gabric.Remote(ctx).Exec("nginx", "-t").RunErr()
})
```

Both return a callable (`func(ctx)` / `TaskFunc`), so tasks compose as normal
Go calls:

```go
var Hello = gabric.Task("hello", func(ctx context.Context) {
	HelloLocal(ctx)
	gabric.Must(Check(ctx))
})
```

`(*Gabric).Task`/`TaskErr` register on a `Gabric`; `gabric.Task`/`TaskErr` use
`gabric.Default`.

### `With`, `TaskBuilder`, and middleware

`middleware.Middleware` is an alias of `func(ctx context.Context, next middleware.Next) error`:
it may call `next` zero or more times and return its error. Returning `nil`
without calling `next` short-circuits.
`gabric.With(middlewares...)` returns an immutable `*TaskBuilder`; its `.Task` /
`.TaskErr` terminals register a task wrapped with those middlewares.

`gabric.WithRemoteSudo(spec)` is a middleware that runs `next` with the given
sudo spec. `&gabric.Sudo{}` means sudo as root; set `User` and `Flags` for
`sudo -u` and other options. Write your own as an ordinary function; middleware
runs in declaration order:

```go
func withRemoteProjectDir(ctx context.Context, next middleware.Next) error {
	return next(gabric.NewRemoteWorkingDirContext(ctx, "/srv/project"))
}

var Install = gabric.With(
	gabric.WithRemoteSudo(&gabric.Sudo{User: "deploy", Flags: []string{"-n"}}),
	withRemoteProjectDir,
).Task("install", func(ctx context.Context) {
	gabric.Remote(ctx).Exec("make", "install").Run()
})
```

Sudo is enabled and the remote working directory is `/srv/project` for
`make install`.

A builder is a reusable base: `With` appends and returns a new builder, so a
shared base can be extended without changing it:

```go
var Base = gabric.With(gabric.WithRemoteSudo(&gabric.Sudo{}), withRemoteProjectDir)

var Deploy = Base.Task("deploy", deployBody)
var Check = Base.With(withEnv).TaskErr("check", checkBody)
```

`middleware.Apply` applies a slice directly to a `middleware.Next`:

```go
task := middleware.Apply(
	[]middleware.Middleware{gabric.WithRemoteSudo(&gabric.Sudo{}), withRemoteProjectDir},
	func(ctx context.Context) error {
		gabric.Remote(ctx).Exec("make", "install").Run()
		return nil
	},
)

err := task(ctx)
```

### `Use`

`gabric.Use(middlewares...)` registers global middlewares around every task run
and returns the `*Gabric` for chaining. Global middleware wraps the run of all
dispatched tasks once per invocation:

```go
var _ = gabric.Use(gabric.WithReconcile)
```

`(*Gabric).UseBuiltin(middlewares...)` registers setup middleware that runs
after core built-ins and before main middleware. It must be registered before
the first run:

```go
g := gabric.New()
g.UseBuiltin(myTransportSetup)
```

Transports register their setup this way. `gabricssh.Install(g)` adds the SSH
setup and flags; see [ssh/gabricssh.md](ssh/gabricssh.md).

### `WithReport`

`WithReport` collects Reconcile reports from every run started by the tasks,
including isolated `Run`/`RunNode` calls, and logs a combined summary when the
tasks finish. It does not open a run:

```go
var _ = gabric.Use(gabric.WithReport)
```

Combine it with `WithReconcile` when tasks should share one run:

```go
var _ = gabric.Use(gabric.WithReport, gabric.WithReconcile)
```

`WithReconcileReport` is shorthand for that combination.

Use `LogReport(ctx, results)` to log a collector's results yourself, for example
an isolated collector:

```go
gabric.LogReport(ctx, collector.Results())
```

### `Parallel`, `ParallelErr`

Run independent tasks concurrently and wait for all of them:

```go
var CI = gabric.Task("ci", func(ctx context.Context) {
	gabric.Parallel(ctx, CheckGofmt, Vet, Test)
})
```

`Parallel` takes panic-style task callables (`Task`/`TaskBuilder.Task`): it runs
them concurrently, waits for all to finish, and panics with their joined
(`errors.Join`) errors if any failed. `ParallelErr` is the error-first form: it
takes error-first functions and returns the joined errors. `TaskFunc` is an
alias of `func(ctx) error`, so `[]TaskFunc` and `[]func(ctx) error` both work.

All tasks share `ctx`, so `Parallel` does not derive a context per branch: do
that inside each task. See [architecture.md](architecture.md#concurrency) for
the Reconcile run caveats, and
[reconcile/README.md](reconcile/README.md#using-with-gabric) for node-level
guidance.

### `Main` options and application flags

Application flags are registered on the runner's `FlagSet` and applied in a
main middleware. Main middleware runs after the built-in context setup and
before global middleware, and has access to `main` locals:

```go
var sudo = gabric.Default.FlagSet.Bool("sudo", false, "run remote commands with sudo")

func main() {
	gabric.Main(gabric.WithMiddleware(func(ctx context.Context, next middleware.Next) error {
		if *sudo {
			ctx = gabric.NewRemoteSudoContext(ctx, &gabric.Sudo{})
		}
		return next(ctx)
	}))
}
```

`WithContext(ctx)` supplies the base context (including a custom logger).
`WithMiddleware(middlewares...)` appends main middleware; repeated options
append in call order.

### Dry run

`-dry-run` sets `Gabric.DryRun`; the built-in middleware marks the context with
`dryrun.With`, so tasks and Reconcile nodes can check it:

```go
if dryrun.Enabled(ctx) {
	return nil
}
```

It is advisory: Gabric operations do not skip automatically; each task or
Reconcile node is responsible for handling dry-run.

### `Must`

Bridges an error-first call into panic-style task code:

```go
var Deploy = gabric.Task("deploy", func(ctx context.Context) {
	gabric.Must(Dist(ctx))
	gabric.Remote(ctx).Rsync().Local("./app").Remote("/srv/app").Upload().Run()
})
```

### `PanicError`, `TaskToTaskErr`, `TaskErrToTask`

A task that panics with an error and one that returns it are equivalent: panics
are normalized with `PanicError`, so `errors.Is`/`errors.As` work across both
styles. `TaskToTaskErr` adapts a panic-style callable (from `Task`) into a
`TaskFunc`, and `TaskErrToTask` is the inverse:

```go
err := gabric.ParallelErr(ctx,
	gabric.TaskToTaskErr(Ssh), // panic-style -> error-first
	Check,                     // already a TaskFunc
)
```

### `New`, `ParseFlags`, `ParseTasks`, `Run`, `Main`

```go
g := gabric.New()
g.Task("deploy", func(ctx context.Context) { /* ... */ })

if err := g.ParseFlags([]string{"-level", "info"}); err != nil {
	panic(err)
}

calls, err := g.ParseTasks([]string{"deploy:staging"})
if err != nil {
	panic(err)
}

if err := g.Run(ctx, calls); err != nil {
	panic(err)
}
```

`ParseFlags` parses runner flags into `Gabric` configuration. `ParseTasks`
resolves `name`/`name:arg` arguments into `[]TaskCall` and does not run anything.
`Run` builds the middleware chain and dispatches the calls, returning the first
error. `gabric.Main()` (or `(*Gabric).Main`) additionally manages the CLI
logger, task listing and exit codes.

`g.Run` does not parse flags or task names; use the phases above when writing a
custom entry point. The initialization pipeline is described in
[architecture.md](architecture.md#initialization-flow).

### Runner flags

| Flag          | Meaning                                          |
| ------------- | ------------------------------------------------ |
| `-level`      | log level (default `debug`)                      |
| `-log-format` | log format: `text` (default) or `json`           |
| `-dry-run`    | mark the context as a dry run (`dryrun.Enabled`) |

SSH flags are installed by `gabricssh.Install`: `-ssh-host` and
`-ssh-identity`. See [ssh/gabricssh.md](ssh/gabricssh.md).

### `Default`

`gabric.Default` is the `*Gabric` used by the package-level functions
(`Task`, `TaskErr`, `With`, `Use`, `ParseFlags`, `ParseTasks`, `Run`, `Main`).

## Context helpers

Execution state lives in `context.Context`. Constructors derive a context;
getters read it.

| Concern             | Constructor                                                   | Getter                                             |
| ------------------- | ------------------------------------------------------------- | -------------------------------------------------- |
| Logger              | `logger.NewLoggerContext`                                     | `gabric.Logger`, `logger.HasLogger`                |
| Target              | `NewTargetContext`                                            | `TargetOps`, `HasTarget`                           |
| Target ops workdir  | `NewTargetOpsWorkingDirContext`                               | `GetTargetOpsWorkingDir`, `HasTargetOpsWorkingDir` |
| Target ops exec env | `NewTargetOpsExecEnvContext`, `AppendTargetOpsExecEnvContext` | `GetTargetOpsExecEnv`, `HasTargetOpsExecEnv`       |
| Remote runner       | `NewRemoteRunnerContext`                                      | `GetRemoteRunner`, `HasRemoteRunner`               |
| Remote sudo         | `NewRemoteSudoContext`                                        | `GetRemoteSudo`, `HasRemoteSudo`                   |
| Remote working dir  | `NewRemoteWorkingDirContext`                                  | `GetRemoteWorkingDir`, `HasRemoteWorkingDir`       |
| Local working dir   | `NewLocalWorkingDirContext`                                   | `GetLocalWorkingDir`, `HasLocalWorkingDir`         |
| Local exec env      | `NewLocalExecEnvContext`, `AppendLocalExecEnvContext`         | `GetLocalExecEnv`, `HasLocalExecEnv`               |
| Remote exec env     | `NewRemoteExecEnvContext`, `AppendRemoteExecEnvContext`       | `GetRemoteExecEnv`, `HasRemoteExecEnv`             |
| Local backend       | `NewLocalBackendContext`                                      | `GetLocalBackend`, `HasLocalBackend`               |
| Remote backend      | `NewRemoteBackendContext`                                     | `GetRemoteBackend`, `HasRemoteBackend`             |
| Current task arg    | `NewCurrentTaskArgContext`                                    | `GetCurrentTaskArg`, `HasCurrentTaskArg`           |
| Dry run             | `dryrun.With`                                                 | `dryrun.Enabled`                                   |

SSH host, config and manager helpers live in `gabricssh`. See
[ssh/gabricssh.md](ssh/gabricssh.md).

`GetFoo` returns a documented default when one exists. Required state with no
default (`GetRemoteRunner`, `GetRemoteBackend`, `TargetOps`) panics.
`GetRemoteRunner` panics with `ErrNoRemoteRunner` when no transport is installed.
Defaults are `slog.Default` for the logger, the real backends for local and
remote operations, the process working directory for the local working dir,
`nil` for remote sudo, `""` for remote working dir and current task arg, and no
variables for the local or remote exec env. Every stored value has a presence
predicate:
`HasFoo`, or `dryrun.Enabled` for the dry-run marker.

### Target operations

`TargetOps(ctx)` returns the operations selected for the current target. A
`Target` bundles the operations provider with the exec environment and working
directory context constructors for that target:

```go
type Target struct { /* ... */ }

func NewTarget[O Ops](
	ops func(context.Context) O,
	execEnvContext ExecEnvContextFunc,
	workingDirContext WorkingDirContextFunc,
) Target

func NewTargetContext(ctx context.Context, target Target) context.Context

var (
	LocalTarget       = NewTarget(Local, AppendLocalExecEnvContext, NewLocalWorkingDirContext)
	RemoteTarget      = NewTarget(Remote, AppendRemoteExecEnvContext, NewRemoteWorkingDirContext)
	LocalRemoteTarget = NewTarget(LocalRemote, AppendRemoteExecEnvContext, NewRemoteWorkingDirContext)
)
```

How `TargetOps(ctx)` behaves for each built-in target:

| Target              | Commands                | File ops       | Sudo             |
| ------------------- | ----------------------- | -------------- | ---------------- |
| `LocalTarget`       | direct Go `exec`        | native Go `os` | none             |
| `LocalRemoteTarget` | `/bin/sh`               | coreutils      | `WithRemoteSudo` |
| `RemoteTarget`      | installed remote runner | coreutils      | `WithRemoteSudo` |

The remote runner is whatever the transport installed: SSH, Docker, or a custom
runner.

Selecting a target changes only `TargetOps`:

| Active target       | `TargetOps(ctx)` | `Remote(ctx)`           | `Local(ctx)` |
| ------------------- | ---------------- | ----------------------- | ------------ |
| `LocalTarget`       | `Local`          | installed remote runner | `Local`      |
| `LocalRemoteTarget` | `LocalRemote`    | installed remote runner | `Local`      |
| `RemoteTarget`      | `Remote`         | installed remote runner | `Local`      |

`Remote(ctx)` always uses the installed remote runner, regardless of the active
target. In particular, `LocalRemoteTarget` does not make `Remote(ctx)` local;
use `TargetOps(ctx)`, or `gabric.LocalRemote(ctx)` for explicit local
remote-style operations.

Choose `LocalTarget` for native local work, for example using Reconcile modules
to declaratively manage local files and directories: commands run directly
through Go `exec`/`os`, paths and environment are local, and there is no
implicit shell or sudo. `LocalTarget` operations work on Windows; individual
module support still depends on the module and available tools.

Choose `LocalRemoteTarget` when the local run should match a remote Unix host:
the same `/bin/sh` commands, coreutils, POSIX working directories, remote-style
environment, and `WithRemoteSudo` behavior. Use this target to run a
binary built for a remote machine in place, such as the
[homelab example's](../examples/homelab) `deploy-local`/`-local` path.
It needs `/bin/sh` and coreutils, so it is Unix-like only; use it when you need
local sudo or rsync. `LocalTarget` has no rsync.

Transport packages provide additional targets:

| Target                           | `TargetOps(ctx)` uses                 | Notes                                                    |
| -------------------------------- | ------------------------------------- | -------------------------------------------------------- |
| `gabricssh.Target(host)`         | `gabricssh.Remote(ctx, host)`         | Needs an SSH config and manager; does not install them.  |
| `gabricdocker.Target(container)` | `gabricdocker.Remote(ctx, container)` | Installs a Docker runner, overriding the context runner. |

Use `RemoteTarget` when the context already identifies the remote, and a
transport target to scope `TargetOps` to a specific host or container. These are
ordinary `gabric.Target` values; direct `Remote(ctx)` and `Local(ctx)` calls
still do not follow them. See [SSH](ssh/gabricssh.md) and
[Docker](docker/gabricdocker.md).

Use it directly:

```go
ctx = gabric.NewTargetContext(ctx, gabric.LocalTarget)
gabric.TargetOps(ctx).Exec("systemctl", "restart", "app").Run()
```

For the local remote-style case:

```go
ctx = gabric.NewTargetContext(ctx, gabric.LocalRemoteTarget)

gabric.TargetOps(ctx).Exec("systemctl", "restart", "app").Run() // LocalRemote
gabric.Remote(ctx).Exec("systemctl", "restart", "app").Run()    // installed remote runner
gabric.Local(ctx).Exec("echo", "always local").Run()            // local backend
```

`LocalTarget` uses `Local`, `AppendLocalExecEnvContext` and
`NewLocalWorkingDirContext`; `RemoteTarget` and `LocalRemoteTarget` use the
remote equivalents `Remote`, `AppendRemoteExecEnvContext` and
`NewRemoteWorkingDirContext`

Target ops apply explicit working-directory and exec-environment overlays:

```go
ctx = gabric.NewTargetOpsWorkingDirContext(ctx, "/srv/app")
ctx = gabric.NewTargetOpsExecEnvContext(ctx,
	gabric.Env("DEBIAN_FRONTEND", "noninteractive"),
)
ctx = gabric.AppendTargetOpsExecEnvContext(ctx,
	gabric.Env("GIT_TERMINAL_PROMPT", "0"),
)
```

`TargetOps` applies each overlay once to the context used for the returned
operations, using the target's working-directory or exec-environment context
constructor. The overlays apply only to target operations, not direct `Local(ctx)`
or `Remote(ctx)` calls.

- `NewTargetOpsWorkingDirContext` replaces the working-dir overlay.
- `NewTargetOpsExecEnvContext` replaces the exec-env overlay;
  `AppendTargetOpsExecEnvContext` merges into it.
- Scoped `ops.WithWorkingDir` and `ops.WithExecEnv` override the target overlays
  for their callback. The applied markers prevent a nested `TargetOps(ctx)` call
  from re-applying them.
- If the overlays are already applied, setting or appending applies the change
  immediately through the target's working-dir or exec-env context constructor
  and keeps the markers. Otherwise it updates the overlay for the next
  `TargetOps`. `NewTargetContext` clears both markers.

For example, inside a scoped working-directory callback the overlays are already
applied and marked. A nested `TargetOps` keeps the scoped value, so the relative
`dist` resolves against `/tmp`; without the marker it would be re-based on
`/srv/app` instead:

```go
ctx = gabric.NewTargetContext(ctx, gabric.LocalTarget)
ctx = gabric.NewTargetOpsWorkingDirContext(ctx, "/srv/app")

gabric.TargetOps(ctx).WithWorkingDir("/tmp", func(ctx context.Context) {
	gabric.TargetOps(ctx).Exec("pwd").Run() // /tmp: the overlay is not re-applied

	gabric.TargetOps(ctx).WithWorkingDir("dist", func(ctx context.Context) {
		gabric.TargetOps(ctx).Exec("pwd").Run() // /tmp/dist, not /srv/app/dist: the marker prevents re-applying the overlay
	})
})
```

`OpsWithRsync` is the optional rsync capability:

```go
type OpsWithRsync interface {
	Ops
	Rsync() *RsyncOps
}

var ErrNoRsyncTarget = errors.New("gabric: target does not support rsync")

func TargetRsyncOps(ctx context.Context) (*RsyncOps, error)
```

`RemoteTarget` and `LocalRemoteTarget` support rsync; `LocalTarget` returns
`ErrNoRsyncTarget`.

`NewTarget` accepts concrete providers such as `Local` and `Remote`, so they keep
their concrete return types and extra methods. The context stores a single
`Target`, not separate ops and env settings.

`WithLocalTarget` and `WithRemoteTarget` are middleware wrappers:

```go
gabric.Main(gabric.WithMiddleware(gabric.WithLocalTarget))
```

`Main` and `Run` install `RemoteTarget` when none is present, through built-in
middleware. Remote operations need an installed runner, normally from
`gabricssh.Install`. Direct Reconcile use must set a target explicitly:

```go
ctx = gabric.NewTargetContext(ctx, gabric.RemoteTarget)
```

A plain local target runs as the user running Gabric. Remote sudo does not apply.

`LocalRemote` runs remote-style commands on the local machine without SSH. It
keeps remote working directories and `WithRemoteSudo` behavior:

```go
gabric.Main(gabric.WithMiddleware(gabric.WithLocalRemoteTarget))
```

or:

```go
ctx = gabric.NewTargetContext(ctx, gabric.LocalRemoteTarget)
```

If remote sudo is configured, `LocalRemote` still needs working sudo
permissions. Without it, commands run as the current user. `LocalRemote` expects
`/bin/sh` and the Unix tools used by the modules. It covers command, file, and
rsync operations. SSH shell and forwarding are in `gabricssh`. Rsync still needs
the native transport, so it is Unix-only.

For privileged local operations, use `LocalRemoteTarget` with
`NewRemoteSudoContext` or `WithRemoteSudo`:

```go
ctx = gabric.NewTargetContext(ctx, gabric.LocalRemoteTarget)
ctx = gabric.NewRemoteSudoContext(ctx, &gabric.Sudo{})
```

There is no separate local-sudo context API; remote sudo is the sudo setting for
remote-style operations, including `LocalRemote`.

Remote SSH works with macOS targets. Module support depends on the tools
installed on the target. `Pkg`, `Unit`, `UFW`, `Sysctl`, and `User` are
Linux-specific.

Custom remote runners can replace SSH for command and file operations:

```go
type RemoteRunner func(ctx context.Context, cmd string, stdin io.Reader, stdout, stderr io.Writer) error

ctx = gabric.NewRemoteRunnerContext(ctx, runner)
```

A runner receives the same shell command that would be sent over SSH, including
working directory, sudo and exec-environment wrapping. `NewRemoteRunnerContext`
only changes how the command is executed. The target still needs to be set.
`gabricssh` provides the SSH shell and forwarding. Rsync runs its remote side
through the runner when one is set, but needs the native transport, so it is
Unix-only.

The runner contract is:

- Run `cmd` with a shell. It is already a complete shell command.
- Wire `stdin`, `stdout`, and `stderr` directly. Do not allocate a TTY.
- Honor context cancellation and return `ctx.Err()` or an error that matches it.
- Return `nil` for exit status 0.
- Return `*ExecExitError` for a non-zero command status. Set `Status` to the
  status and `Err` to the underlying runner error.
- Return an ordinary error when the command cannot start or the runner itself
  fails.
- Do not return `*ExecExitError` for cancellation or transport failure.

`ExecExitError` provides `Status`, `ExitCode()`, and an `Unwrap` to the
original error. `Exists` uses `ExecExitError` with status 1 to mean "not
found."

`GetRemoteRunner(ctx)` returns the installed runner and panics with
`ErrNoRemoteRunner` when none is present. `HasRemoteRunner(ctx)` reports whether
a runner is installed. There is no default SSH runner; a transport installs one.
`gabricssh.NewRunnerContext` installs the SSH runner and rsync fallback.

### Backends

`Local(ctx)` and `Remote(ctx)` are builders over the backend carried in the
context. The default local backend uses the OS. The default remote backend builds
shell commands and uses the context runner; tests and library code can inject
their own:

```go
ctx = gabric.NewLocalBackendContext(ctx, myLocalBackend)
ctx = gabric.NewRemoteBackendContext(ctx, myRemoteBackend)
```

`LocalBackendFunc` and `RemoteBackendFunc` implement the interfaces with optional
function fields. Missing operations return `ErrLocalBackendUnsupported` or
`ErrRemoteBackendUnsupported`. The request types in `requests.go`
(`ExecRequest`, `PutRequest`, `GetRequest`, `ExistsRequest`, `StatRequest`,
`MkdirRequest`, `RemoveRequest`, `ListRequest`, `RsyncRequest`) are the inputs
to those methods.
`ExecRequest.Env` is the resolved exec environment for the command: the
context environment with the per-command entries applied. Backends use it instead of
reading env from the context. `RemoteBackend` covers `Rsync`; `LocalBackend` does
not. `LocalBackend.Exec` returns `*ExecExitError` for a non-zero command status.
`LocalBackendFunc` returns the error from its `ExecFunc` unchanged.

`NewLocalRunner` builds a `RemoteRunner` from a local argv function:

```go
runner := gabric.NewLocalRunner(func(cmd string) []string {
	return []string{"ssh", "deploy@example.com", "/bin/sh", "-c", cmd}
})
ctx = gabric.NewRemoteRunnerContext(ctx, runner)
```

It runs `argv(cmd)` through the local backend. The local backend returns
`*ExecExitError` for a non-zero command status. `LocalRemote` is built on it.

### Logger

```go
log := slog.New(logger.NewTextHandler(os.Stderr, nil))
ctx = logger.NewLoggerContext(ctx, log)

gabric.Logger(ctx).Info("deploying", "stage", "prod")
```

### Application values

Application configuration is ordinary Go: flags, files, or values in the
context. Keep it out of Gabric's runner config. A main middleware can attach it:

```go
type siteKey struct{}

ctx = context.WithValue(ctx, siteKey{}, siteConfig{...})
```

### Remote sudo and working directories

```go
sudo := gabric.GetRemoteSudo(ctx)             // *gabric.Sudo, nil when unset
dir := gabric.GetRemoteWorkingDir(ctx)        // "" when unset

ctx = gabric.NewRemoteSudoContext(ctx, &gabric.Sudo{User: "deploy", Flags: []string{"-n"}})
ctx = gabric.NewRemoteWorkingDirContext(ctx, "/srv/app")

remotePath := gabric.ResolveRemotePath(ctx, "config") // /srv/app/config
absolutePath := gabric.ResolveRemotePath(ctx, "/etc/config") // /etc/config
```

A nil `*Sudo` means no sudo. `&Sudo{}` means sudo as root; set `User` and
`Flags` for `sudo -u` and other options. `Sudo.Prefix()` / `Sudo.WrapCommand`
expose the same command construction used by remote operations.

SSH host, config and manager context helpers are in `gabricssh`. See
[ssh/gabricssh.md](ssh/gabricssh.md).

### Working directories and exec environments

```go
ctx = gabric.NewLocalWorkingDirContext(ctx, "/tmp/build")
dir := gabric.GetLocalWorkingDir(ctx)        // falls back to os.Getwd
abs := gabric.ResolveLocalPath(ctx, "app")   // join with the working dir if relative
```

Sensitive values are marked per entry:

```go
ctx = gabric.NewLocalExecEnvContext(ctx,
	gabric.Env("GOFLAGS", "-trimpath"),
	gabric.SensitiveEnv("GITHUB_TOKEN", token),
)

localEnv := gabric.GetLocalExecEnv(ctx)
flags, ok := localEnv.Get("GOFLAGS")
entries := localEnv.Environ() // real values
```

Remote commands use the same entries:

```go
ctx = gabric.NewRemoteExecEnvContext(ctx, gabric.Env("DEBIAN_FRONTEND", "noninteractive"))

gabric.Remote(ctx).Exec("apt-get", "install", "-y", "ufw").Run()
```

`ExecEnv` is immutable. `NewExecEnv`, `With`, `Merge` and `MarkSensitive` return
new values; `Environ()` returns an independent slice of real `KEY=VALUE`
entries, `EnvironLog()` the same entries with sensitive values redacted, and
`Vars()` returns a copy of the entries for reuse. `ParseExecEnv` parses
`KEY=VALUE` strings as normal entries; call `MarkSensitive` for keys that must be
redacted.

`NewLocalExecEnvContext` and `NewRemoteExecEnvContext` replace the corresponding
local or remote environment with the given entries. `AppendLocalExecEnvContext`
and `AppendRemoteExecEnvContext` merge new entries into it. Both forms take any
number of entries. `NewTargetOpsExecEnvContext` and
`AppendTargetOpsExecEnvContext` do the same for the target ops overlay that
`TargetOps` applies. `LocalOps.WithExecEnv` and `RemoteOps.WithExecEnv` merge
entries for the duration of a callback; both also have a `WithExecEnvErr` form.

The remote environment is exported inside the remote shell command, after any
sudo wrapper, so it reaches the command even when sudo resets the environment.
It therefore applies to every remote command, including the internal file
operations. Rsync's remote server command is not covered.

`EnvVar` is an opaque entry; `Env` creates a normal entry and `SensitiveEnv` a
sensitive one. `ExecEnv` redacts sensitive values in `String` and `LogValue`.
`fmt` on an entry shows only opaque pointers; `ExecEnv`'s `%v`/`%+v` use the
redacted `String` view, while `%#v` stays opaque. Debug logs show the full
remote command including the env prefix, with sensitive values replaced by
`<sensitive>`. Log an environment as its own attribute to get the redacted
group:

```go
gabric.Logger(ctx).Info("deploy", "env", env)
// text: env.GOFLAGS=-trimpath env.GITHUB_TOKEN=<sensitive>
// json: {"env":{"GOFLAGS":"-trimpath","GITHUB_TOKEN":"<sensitive>"}}
```

Nested in another struct or slice, JSON renders the env as `{}` because its data
is private. There is no redacting `MarshalJSON`;
`EnvironLog()`, `String` and `LogValue` are the redacted views, while
`Environ()` and `Get` are the explicit ways to read real values. Sensitivity is
display-only: it does not encrypt values, and a remote value is still part of the
target's shell command, where it can appear in the process list or audit logs.
Large remote environments also count toward the command length. A sensitive
value embedded in a normal value or a command argument is not detected.

### Task arguments

```go
ctx = gabric.NewCurrentTaskArgContext(ctx, "staging")

arg := gabric.GetCurrentTaskArg(ctx) // "" when unset
hasArg := gabric.HasCurrentTaskArg(ctx)
```

`ParseTasks` returns `[]TaskCall` with the call argument in `Arg` and preserves
duplicate invocations, so `deploy:staging deploy:prod` runs the task twice with
different arguments.

## Local and remote operations

`Local(ctx)` and `Remote(ctx)` both implement `Ops`:

```go
type Ops interface {
	Exec(cmd ...string) *ExecOps

	Put(path string) *PutOps

	Get(path string) *GetOps

	Mkdir(path string) *MkdirOps

	Remove(path string) *RemoveOps

	Exists(path string) *ExistsOps

	Stat(path string) *StatOps

	List(path string) []ListEntry
	ListErr(path string) ([]ListEntry, error)

	WithWorkingDir(path string, f func(ctx context.Context))
	WithWorkingDirErr(path string, f func(ctx context.Context) error) error

	WithExecEnv(env ExecEnv, f func(ctx context.Context))
	WithExecEnvErr(env ExecEnv, f func(ctx context.Context) error) error
}
```

### `Exec`

```go
gabric.Local(ctx).Exec("echo", "hi").Run()
gabric.Remote(ctx).Exec("systemctl", "restart", "app").Run()
```

Output model:

| Method          | Effect                                             |
| --------------- | -------------------------------------------------- |
| (default)       | stdout and stderr stream to the process            |
| `Stdin`         | attach standard input (`io.Reader`)                |
| `Stdout`        | redirect stdout to an `io.Writer`                  |
| `Stderr`        | redirect stderr to an `io.Writer`                  |
| `Capture`       | capture both                                       |
| `CaptureStdout` | capture stdout only                                |
| `CaptureStderr` | capture stderr only                                |
| `LogErrors`     | capture stderr and log it when the command fails   |
| `Quiet`         | capture both and log stderr when the command fails |

An explicit `Stdout`/`Stderr` writer takes precedence over the capture options.

`WithEnv` sets one or more environment entries for the command. They
override the context's exec environment for the same keys:

```go
gabric.Remote(ctx).Exec("apt-get", "install", "-y", "ufw").
	WithEnv(gabric.Env("DEBIAN_FRONTEND", "noninteractive")).
	Run()

gabric.Local(ctx).Exec("deploy").
	WithEnv(gabric.Env("REGION", "eu"), gabric.SensitiveEnv("DEPLOY_TOKEN", token)).
	Run()
```

```go
out := gabric.Local(ctx).Exec("git", "rev-parse", "HEAD").
	CaptureStdout().
	Run().
	StdoutStr()

err := gabric.Local(ctx).Exec("test", "-d", "/srv/app").RunErr()

gabric.Remote(ctx).Exec("tee", "/etc/app/config").
	Stdin(strings.NewReader(config)).
	Run()

gabric.Local(ctx).Exec("tar", "-cf", "-", "dir").Stdout(file).Run()
```

Captured output: `StdoutBytes`/`StdoutStr` and `StderrBytes`/`StderrStr`. `Run`
panics on failure; `RunErr` returns the error.

### `Put`

`Put` is a builder (like `Exec` and `Rsync`) for both local and remote
operations:

```go
func (o *LocalOps)  Put(path string) *PutOps
func (o *RemoteOps) Put(path string) *PutOps

func (p *PutOps) Reader(reader io.Reader) *PutOps
func (p *PutOps) String(s string) *PutOps
func (p *PutOps) Bytes(b []byte) *PutOps
func (p *PutOps) User(user string) *PutOps
func (p *PutOps) Group(group string) *PutOps
func (p *PutOps) Mode(mode os.FileMode) *PutOps
func (p *PutOps) EnsureParent() *PutOps
func (p *PutOps) Run() *PutOps
func (p *PutOps) RunErr() error
```

```go
gabric.Local(ctx).Put("build/app").Bytes(data).Mode(0o644).Run()
gabric.Remote(ctx).Put("/srv/app/config").String(config).Mode(0o600).Run()

err := gabric.Remote(ctx).Put("/etc/app/config").
	EnsureParent().
	String(config).
	User("root").
	Group("root").
	RunErr()
```

Content is set with one of `Reader`, `String` or `Bytes` (the last call wins);
omitting all three is an error. `User`/`Group` and `Mode` are optional and
applied after the content is written. `EnsureParent` creates the target's parent
directory recursively (`mkdir -p` semantics, permissions left to the umask)
before the write. Remote writes stream to `dd`; with remote sudo they run under
the configured sudo spec (root by default).

### `Get`

`Get` is a builder for both local and remote reads:

```go
func (o *LocalOps)  Get(path string) *GetOps
func (o *RemoteOps) Get(path string) *GetOps

func (g *GetOps) Bytes() []byte
func (g *GetOps) BytesErr() ([]byte, error)
func (g *GetOps) String() string
func (g *GetOps) StringErr() (string, error)
func (g *GetOps) Copy(w io.Writer)
func (g *GetOps) CopyErr(w io.Writer) error
func (g *GetOps) WithReader(f func(io.Reader) error)
func (g *GetOps) WithReaderErr(f func(io.Reader) error) error
```

```go
data := gabric.Remote(ctx).Get("/srv/app/version").Bytes()
text := gabric.Local(ctx).Get("build/app").String()

f, _ := os.Create("build/app")
defer f.Close()
gabric.Remote(ctx).Get("/srv/app/app").Copy(f)

err := gabric.Remote(ctx).Get("/srv/app/app").WithReaderErr(func(r io.Reader) error {
	_, err := io.Copy(hash, r)
	return err
})
```

Each terminal has a panic form and an `Err` form. `Bytes`/`String` buffer the
file; `Copy` streams to a writer; `WithReader` hands the live reader to a
callback. The reader is only valid for the duration of that callback. Reads
stream with constant memory: remote reads run `cat` (prefixed with the configured
sudo spec when remote sudo is set).

### `Mkdir`

`Mkdir` is a builder for both local and remote directories:

```go
func (o *LocalOps)  Mkdir(path string) *MkdirOps
func (o *RemoteOps) Mkdir(path string) *MkdirOps

func (m *MkdirOps) Recursive() *MkdirOps
func (m *MkdirOps) Mode(mode os.FileMode) *MkdirOps
func (m *MkdirOps) User(user string) *MkdirOps
func (m *MkdirOps) Group(group string) *MkdirOps
func (m *MkdirOps) Run() *MkdirOps
func (m *MkdirOps) RunErr() error
```

```go
gabric.Remote(ctx).Mkdir("/srv/app").Run()

err := gabric.Remote(ctx).Mkdir("/srv/app/state").
	Recursive().
	Mode(0o700).
	RunErr()
```

`Mkdir` creates a single directory; `Recursive` also creates missing parents
(`mkdir -p`). An existing directory is not an error, and `User`/`Group`/`Mode`
are still applied to it (the mode is enforced when set). `Run` panics, `RunErr`
returns the error.

### `Remove`

`Remove` is a builder for both local and remote paths:

```go
func (o *LocalOps)  Remove(path string) *RemoveOps
func (o *RemoteOps) Remove(path string) *RemoveOps

func (r *RemoveOps) Recursive() *RemoveOps
func (r *RemoveOps) Force() *RemoveOps
func (r *RemoveOps) Run() *RemoveOps
func (r *RemoveOps) RunErr() error
```

```go
gabric.Remote(ctx).Remove("/srv/app/cache/file").Run()

err := gabric.Remote(ctx).Remove("/srv/app/state").
	Recursive().
	Force().
	RunErr()
```

`Remove` deletes a file, symlink, or empty directory. `Recursive` also removes a
directory's contents (`rm -r`). A missing path is an error unless `Force` is set
(`rm -f`). A symlink is removed without following it, and sudo applies to remote
removal like the other remote file operations. `Recursive` and `Force` are
independent: `Recursive` alone keeps `rm -r` semantics, while
`Recursive().Force()` is `rm -rf`. `Run` panics, `RunErr` returns the error.

### `Exists`

`Exists` is a builder for both local and remote paths:

```go
func (o *LocalOps)  Exists(path string) *ExistsOps
func (o *RemoteOps) Exists(path string) *ExistsOps

func (e *ExistsOps) Dir() *ExistsOps
func (e *ExistsOps) File() *ExistsOps
func (e *ExistsOps) Link() *ExistsOps
func (e *ExistsOps) Run() bool
func (e *ExistsOps) RunErr() (bool, error)
```

```go
if gabric.Remote(ctx).Exists("/srv/app/app").Run() {
	// ...
}

exists, err := gabric.Remote(ctx).Exists("/srv/app").Dir().RunErr()
```

With no filter the probe reports whether the path exists at all (`test -e`).
`Dir`, `File` and `Link` restrict it to a directory, a regular file or a symlink
(`test -d`/`-f`/`-L`). `Dir` and `File` follow symlinks; `Link` does not. A
missing path, or one of the wrong kind, is `(false, nil)`. `Run` panics on an
unexpected error, `RunErr` returns it.

### `Stat`

`Stat` is a builder for both local and remote path metadata:

```go
func (o *LocalOps)  Stat(path string) *StatOps
func (o *RemoteOps) Stat(path string) *StatOps

func (s *StatOps) Run() FileStat
func (s *StatOps) RunErr() (FileStat, error)

type FileStat struct {
	Mode  fs.FileMode
	User  string
	Group string
}
```

```go
stat, err := gabric.Remote(ctx).Stat("/srv/app").RunErr()
if err != nil {
	return err
}

mode := stat.Mode.Perm() // 0o755
user, group := stat.User, stat.Group
```

`Stat` follows symlinks (`os.Stat` semantics); use `Exists(...).Link()` for a
no-follow probe. A missing path returns an error wrapping `fs.ErrNotExist`.
`Mode` carries the type, permission and special bits, so `Mode.IsDir()` and
`Mode.Perm()` work as on `os.FileMode`.

Local `Stat` resolves uid/gid to names and falls back to the numeric ID; Windows
leaves `User`/`Group` empty. Remote `Stat` uses one `/bin/sh` command that
selects GNU or BSD `stat`; Linux needs `stat -c` support (GNU coreutils, BusyBox
with `FEATURE_STAT_FORMAT`, or Toybox). `Run` panics on failure, `RunErr`
returns the error.

### `List`

`List` returns the entries of a local or remote directory, sorted by name:

```go
type ListEntry struct {
	Name  string
	IsDir bool
}

func (o *LocalOps)  List(path string) []ListEntry
func (o *RemoteOps) List(path string) []ListEntry
func (o *LocalOps)  ListErr(path string) ([]ListEntry, error)
func (o *RemoteOps) ListErr(path string) ([]ListEntry, error)
```

```go
entries := gabric.Remote(ctx).List("/srv/app")

entries, err := gabric.Remote(ctx).ListErr("/srv/app")
if err != nil {
	return err
}
```

`List` panics on failure; `ListErr` returns the error. Entries include hidden files. `IsDir` does not follow symlinks, matching `os.ReadDir`. The remote implementation uses `ls -1Ap`; filenames containing newlines are not supported.

### `WithWorkingDir`

```go
gabric.Remote(ctx).WithWorkingDir("/srv/app", func(ctx context.Context) {
	gabric.Remote(ctx).Exec("git", "pull").Run()
})
```

`WithWorkingDirErr` is the error-first form; use it when the callback needs to
return an error:

```go
err := gabric.Remote(ctx).WithWorkingDirErr("/srv/app", func(ctx context.Context) error {
	return gabric.Remote(ctx).Exec("git", "pull").RunErr()
})
```

### `LocalOps.WithExecEnv` / `RemoteOps.WithExecEnv`

```go
gabric.Local(ctx).WithExecEnv(
	gabric.NewExecEnv(gabric.Env("CGO_ENABLED", "0")),
	func(ctx context.Context) {
		gabric.Local(ctx).Exec("go", "build", "./...").Run()
	},
)

err := gabric.Remote(ctx).WithExecEnvErr(
	gabric.NewExecEnv(gabric.SensitiveEnv("GITHUB_TOKEN", token)),
	func(ctx context.Context) error {
		return gabric.Remote(ctx).Exec("gh", "release", "create").RunErr()
	},
)
```

The entries merge into the current environment for the callback and do not
escape it. Both `LocalOps` and `RemoteOps` provide `WithExecEnvErr` for
error-returning callbacks. These methods are part of `Ops`, so
`TargetOps(ctx).WithExecEnv(...)` works the same way.

## Rsync

```go
func (o *RemoteOps) Rsync() *RsyncOps

func (r *RsyncOps) Local(path string) *RsyncOps
func (r *RsyncOps) Remote(path string) *RsyncOps
func (r *RsyncOps) Upload() *RsyncOps
func (r *RsyncOps) Download() *RsyncOps
func (r *RsyncOps) Delete() *RsyncOps
func (r *RsyncOps) Exclude(patterns ...string) *RsyncOps
func (r *RsyncOps) AddFlags(flags ...string) *RsyncOps
func (r *RsyncOps) SetFlags(flags ...string) *RsyncOps
func (r *RsyncOps) Capture() *RsyncOps
func (r *RsyncOps) CaptureStdout() *RsyncOps
func (r *RsyncOps) CaptureStderr() *RsyncOps
func (r *RsyncOps) Stdout() []byte
func (r *RsyncOps) StdoutStr() string
func (r *RsyncOps) Stderr() []byte
func (r *RsyncOps) StderrStr() string
func (r *RsyncOps) Run() *RsyncOps
func (r *RsyncOps) RunErr() error
```

```go
gabric.Remote(ctx).Rsync().
	Local("./srv").
	Remote("/srv").
	Upload().
	Delete().
	Exclude("**/.env", "**/acme.json", ".state/**").
	Run()
```

The direction (`Upload`/`Download`) and both paths are required. `Exclude` adds
`--exclude` patterns; excluded paths are not transferred and, with `Delete`, are
not deleted because Gabric never sets `--delete-excluded`. `AddFlags` appends
flags on top of the default `-pthrvz`; `SetFlags` replaces the flag set (call it
with no arguments to run rsync with no extra flags). Both transports require a
local `rsync`. On Unix the transport uses the context's remote runner. A
transport installs that runner; `gabricssh` uses a pooled SSH session.
`LocalRemote` uses a local runner. On platforms without inherited file
descriptors, including Windows, rsync uses the transport's rsh resolver.
`gabricssh` provides an SSH fallback; a custom runner has no fallback. See
[rsync-transport.md](rsync-transport.md).

`Capture` captures both streams of the **local** rsync process; `CaptureStdout`
and `CaptureStderr` capture one. Read them with `Stdout`/`StdoutStr` and
`Stderr`/`StderrStr` after `Run`/`RunErr`. The remote rsync server's stderr is
not captured; it stays on `os.Stderr`. To inspect a dry-run itemize:

```go
out := gabric.Remote(ctx).Rsync().
	Local("./srv").
	Remote("/srv").
	Upload().
	AddFlags("-n", "-i").
	CaptureStdout().
	Run().
	StdoutStr()
```

## SSH transport

SSH support is in [`gabricssh`](ssh/gabricssh.md). It installs the runner and
provides host/config/manager helpers, `Shell`, `Forward` and SSH task
registration. The low-level client is [`sshremote`](ssh/sshremote.md).

## Docker transport

Docker support is in [`gabricdocker`](docker/gabricdocker.md). It installs a
`docker exec` runner, a container context, `Remote(ctx, container)` and
`Target(container)`. It provides no shell and no rsync rsh fallback.

## `logger` package

```go
log := slog.New(logger.NewTextHandler(os.Stderr, &slog.HandlerOptions{
	Level: slog.LevelDebug,
}))

ctx = logger.NewLoggerContext(ctx, log)
gabric.Logger(ctx).Debug("hello") // logger.LoggerFromContext(ctx)
```

`NewTextHandler` keeps the last value for duplicate attribute keys (so nested
`task=`/`host=` fields behave as expected).

## Error handling patterns

- Panic-style ops (`Run`, `Put(...).Run()`, `Get(...).Bytes()`,
  `Remove(...).Run()`, `Exists(...).Run()`, `Stat(...).Run()`) abort the task;
  `*Err` variants (`RunErr`, `Put(...).RunErr()`, `Get(...).BytesErr()`,
  `Remove(...).RunErr()`, `Exists(...).RunErr()`, `Stat(...).RunErr()`) return
  the error for probes and control flow.
- `Task` bodies panic; `TaskErr` bodies return errors. Panics are recovered at
  the task boundary and reported as task errors.
- `gabric.Must(err)` bridges an error-first call into panic-style code.
- `Main` logs a failed task and exits non-zero; `(*Gabric).Run` returns the first
  error.
