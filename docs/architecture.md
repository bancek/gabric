# Gabric architecture

Contributor-facing description of how Gabric is put together and why. For the
callable API, signatures, defaults and examples, see [api.md](api.md).

The design rule is:

> **Tasks are Go functions first; the CLI registry is a thin layer around them,
> `Gabric` holds registration and pre-run configuration, and runtime state lives
> in `context.Context`.**

---

## Two layers

Gabric is built from two layers plus installable transports.

### 1. Core: plain functions over a context

The core is a set of ordinary functions that operate on a `context.Context`.
They have no global state and can be used from any Go program with a `main` of
your own:

```go
gabric.Local(ctx).Exec("go", "build", "./cmd/app").Run()
gabric.Remote(ctx).Exec("systemctl", "restart", "app").Run()
gabric.Remote(ctx).Rsync().Local("./app").Remote("/srv/app").Upload().Run()
```

These functions read everything they need (runner, target, sudo, working
directory, environment, logger, ...) from `ctx`.

### 2. Optional: registry and CLI runner

On top of the core, `Gabric` registers tasks and runs them from the command
line:

```go
var Hello = gabric.Task("hello", func(ctx context.Context) {
	gabric.Local(ctx).Exec("echo", "Hello").Run()
})

func main() {
	gabric.Main()
}
```

The package-level `Task`, `TaskErr`, `With`, `Use` and `Main` operate on
`gabric.Default`. `gabric.New()` returns an isolated `*Gabric` with its own
task registry, flag set and runner configuration.

Using this layer is optional. If you prefer, build your own `context` and call
the core functions directly.

### 3. Transports: SSH and Docker

Transports install a runner and their own context setup. The SSH transport is
[`gabricssh`](ssh/gabricssh.md), built on the low-level
[`sshremote`](ssh/sshremote.md) package. The Docker transport is
[`gabricdocker`](docker/gabricdocker.md), built on `docker exec`.

The core imports no integration. An application installs one before `Main`:

```go
func main() {
	gabricssh.InstallDefault()
	gabric.Main()
}
```

Without a transport, `gabric.Main()` runs tasks that use `Local(ctx)`. The
default target remains `RemoteTarget`, and remote operations need an installed
runner.

Docker has no interactive shell. Docker rsync uses the native transport only,
because `gabricdocker` installs `NoRsyncRsh`.

---

## Context is the execution environment

A Gabric task body is either a panic-style `func(ctx context.Context)`
(registered with `Task`) or an error-first `func(ctx context.Context) error`
(registered with `TaskErr`). Both are stored as a `TaskFunc` and receive all
execution state through `ctx`.

Everything a task needs travels through `ctx`. The context is the execution
environment; there is no global "current host" or mutable `env`.

Execution state is stored under private context keys: logger, target
operations, remote runner, sudo, working directories, local and remote exec
environments, backends, current task argument and dry-run marker. SSH host,
config and manager state use private keys in `gabricssh`.
Deriving a context produces an independent branch without mutating the parent.
The generated accessors are documented in [api.md](api.md#context-helpers); this
document only relies on the property that all runtime state travels with the
context.

The logger is a `*slog.Logger`. `logger.NewTextHandler` deduplicates attribute
keys so the last value wins (matching the previous logrus behaviour); nested
tasks therefore log a single `task=` field.

---

## No global runtime state

The package-level structures are the registry and runner configuration:

- `gabric.Default` (a `*Gabric`), used by the package-level functions;
- the task registry, global middlewares, flag set and pre-run configuration
  held by a `Gabric`.

No per-execution state is global. Hosts, connections, sudo, working directories
and user values are all in the context. `Gabric` fields are inputs to context
construction (`Level`, `LogFormat`, `DryRun`); `Run` never reads them. As a
result:

- tasks can be composed like any other Go value;
- different branches of one execution can have different hosts or settings;
- execution is concurrent.

---

## Tasks and the CLI

`Task` and `TaskErr` register panic-style and error-first task bodies. Both are
stored as the same `TaskFunc` shape, and the registry stores the wrapped function
directly. `ParseTasks` resolves names and `task:arg` arguments into `TaskCall`
values; duplicate invocations remain separate calls.

`Gabric.Main` parses runner flags and task names, builds the base context, and
dispatches through the middleware chain described below. Runner flags are
documented in [api.md](api.md#runner-flags). Application flags belong to the
application and are applied in a main middleware, which can close over `main`
locals.

`-dry-run` is built in: the built-in middleware marks the context with
`dryrun.With`; operations themselves remain advisory, and each task or Reconcile
node is responsible for honoring the marker.

---

## Initialization flow

`Main` works in explicit phases:

```text
Install(g)                → register transport flags and UseBuiltin setup
ParseFlags(g.Args)        → core flags and installed transport flags
logger                    → bootstrap, then configured from Level/LogFormat
ParseTasks(FlagSet.Args)  → []TaskCall
Run(ctx, calls, main...)  → middleware chain
  core built-in: dry-run marker, default remote target
  installed:     transport setup from UseBuiltin
  main:          WithMiddleware(...)
  global:        Use(...)
  terminal:      Dispatch(calls)
```

`ParseFlags` and `ParseTasks` never run tasks. `Run` never parses flags or task
names. If a logger is supplied through `WithContext`, it is preserved and
`-level`/`-log-format` do not replace it.

The SSH host is stored by `gabricssh` as a spec and resolved **lazily** on the
first remote use, so a main middleware can change the config before resolution.
Resolution is memoized, so the same `*Host` and pooled connection are used
throughout the run. The `host` logger field shows the spec until resolution and
the full host afterwards.

If you write your own `main`, use `ParseFlags`, `ParseTasks`, `WithContext`,
`WithMiddleware` and `Run` directly, or compose `BuiltinMiddlewares` /
`GlobalMiddlewares` with `middleware.Apply`.

---

## Local and remote operations

`Local(ctx)` and `Remote(ctx)` return ctx-bound `Ops` implementations. Most
operations are builders that translate fluent calls into backend requests;
`List` and `WithWorkingDir` are direct calls. The backend is selected by the
constructor (`Local` vs `Remote`). Public method contracts, output modes and
examples are in [api.md](api.md#local-and-remote-operations).

### Command execution

`Exec` is one shared builder backed by different execution functions. It owns
stdio selection, capture buffers, per-command environment and the panic/error
terminals. The operation layer resolves the context environment plus the
per-command overlay into `ExecRequest.Env`; the backend receives that request.
See [api.md](api.md#exec).

### Files

`Put`, `Get`, `Mkdir`, `Remove`, `Exists`, `Stat` and `List` translate into
backend requests in `requests.go`. Remote operations use `/bin/sh` and
coreutils (`dd`, `cat`, `mkdir`, `rm`, `test`, `stat`, `ls`); there is no SFTP
path. Working directories and the exec environments are context state applied
by the operation layer. Local commands inherit the local environment; remote
commands export the remote environment inside the wrapped shell command, after
sudo.

`Stat` follows symlinks. Locally it uses `os.Stat` and resolves uid/gid names.
Remotely one `/bin/sh` command selects GNU or BSD `stat` via `uname -s` and emits
a uniform mode/user/group line. See [api.md](api.md#put), [api.md](api.md#get),
[api.md](api.md#mkdir), [api.md](api.md#exists), [api.md](api.md#stat) and
[api.md](api.md#list).

### Interactive shell

`gabricssh.Shell(ctx)` opens a login shell over the SSH pool, with PTY
negotiation, local raw mode and resize forwarding. The implementation sits above
`sshremote.Remote.WithSession`; builder options are in
[ssh/gabricssh.md](ssh/gabricssh.md) and terminal details are in
[ssh/shell.md](ssh/shell.md).

### Local port forwarding

`gabricssh.Forward(ctx)` binds locally and forwards through the SSH server.
Unlike pooled command execution it owns a dedicated connection, so it neither
consumes the pool limit nor leaves a pooled connection in an indeterminate state
when cancelled. Builder usage is in [ssh/gabricssh.md](ssh/gabricssh.md) and
the lifecycle details are in [ssh/forward.md](ssh/forward.md).

---

## SSH transport

Core keeps a transport-neutral `RemoteRunner` in context. `gabricssh` installs
the SSH runner and keeps host, config and manager state in its own private
context keys. The low-level `sshremote` package provides `Host`, `Config`,
`HostConfig`, `RemoteManager` and `Remote`; it reads no context values itself.

`HostConfig` resolves effective private keys, known hosts and host key policy
from host-specific ssh_config values and config defaults. Each `Remote` pools
connections and is cached per host identity and `*Config` identity, with limits
from its config. Different config instances get separate pools instead of
rebuilding one another's connections.

Pooling is internal: `remotePool` combines a mutex, a counting semaphore for
checked-out resources and a pool-lifetime context. It has no dispatcher
goroutine, and pool-owned connection establishment survives operation
cancellation.

See [ssh/gabricssh.md](ssh/gabricssh.md) for installation, flags and context
helpers. See [ssh/sshremote.md](ssh/sshremote.md) for hosts and resolution, the
config lifecycle (`Clone`, `Equal`, `With...`), authentication, host key
verification and jump hosts. Remote-specific design decisions (pooling, the ssh
agent, `dd` for remote writes) are in
[ssh/sshremote.md](ssh/sshremote.md#design-decisions). The interactive shell,
local port forwarding and the native rsync transport are described in
[ssh/shell.md](ssh/shell.md), [ssh/forward.md](ssh/forward.md) and
[rsync-transport.md](rsync-transport.md).

---

## Rsync

Rsync is a builder over the current host and sudo settings. The user-facing
builder and options are documented in [api.md](api.md#rsync).

Gabric has a native Unix transport and a transport-provided rsh fallback. Both
require a local `rsync`. On Unix, Gabric re-executes itself as the rsync remote
shell (the sidecar) and bridges inherited file descriptors to the context's
remote runner. This avoids the system `ssh` binary. Platforms without inherited
descriptors use the rsh resolver installed by the transport. `gabricssh`
installs the SSH fallback together with its runner. `LocalRemote` and custom
runners install `NoRsyncRsh`, so they have no fallback. The sidecar protocol, fd
layout and the reasons behind it are in
[rsync-transport.md](rsync-transport.md).

---

## Reconcile

Reconcile is an engine for idempotent desired-state work. It is lazy and
topic-driven. It is a separate, **standalone** package
(`github.com/bancek/gabric/reconcile`) that imports only the standard library
and the tiny `gabric/dryrun` context package. Root `gabric` imports it to provide `WithReconcile`, `WithReport`,
`WithReconcileReport`, and `NodeTask`/`NodeTaskErr`, so the dependency is acyclic
(`gabric → reconcile → dryrun`, and `gabric → dryrun`).

Module builders are separate from the engine. They live in
`reconcile/modules` and take values from the standalone `gabric/input` package.
`input` imports only the standard library and is usable outside Reconcile.
Reconcile itself does not import modules, `input`, or `gabric`; it deals only
with nodes, topics, effects, and runs.

- `Node[T]`: a lazy, memoized, idempotent unit that yields a `T`; `Node.Value`
  triggers it, and dependencies are implicit (a node reads the nodes it needs,
  so the graph is discovered per run).
- `Topic`: a valueless change signal; a node that changed calls `Topic.Notify`.
- `Effect`: a reaction subscribed to topics; runs at `Flush`, once per
  notification.
- `Run(ctx, fn)`: the transaction; it opens the run state in the context and
  flushes effects when `fn` returns. All Reconcile state lives in that context
  and nodes are stateless, so one run can be evaluated concurrently and separate
  runs (one per host/task) stay fully independent.

Effects are the handler idea (Ansible handlers / Chef `notify`) with typed
subscriptions instead of a global string registry. There is no graph object;
dependencies are recorded as nodes read one another. See
[reconcile/architecture.md](reconcile/architecture.md).

---

## Concurrency

With no global execution state, concurrent tasks need no coordination.
`gabric.Parallel` runs task callables concurrently:

```go
gabric.Parallel(ctx,
	func(ctx context.Context) {
		Deploy(gabricssh.NewHostContext(ctx, blue))
	},
	func(ctx context.Context) {
		Deploy(gabricssh.NewHostContext(ctx, green))
	},
)
```

`Parallel` starts each task callable (`Task`/`TaskBuilder.Task`, or any ordinary
`func(ctx)`) in its own goroutine, waits for all of them and panics with their
joined errors; `ParallelErr` is the error-first form (it takes error-first
functions and returns `errors.Join`). It passes the same `ctx` to every callable
and does not derive per-branch contexts; derive them inside each task, as the
example does with `gabricssh.NewHostContext`. Each derived branch gets its own host,
working directory and sudo setting, and its own pooled connection. `Gabric`'s
registry is only read during dispatch and the execution context is never shared
mutably.

Node state is keyed by node identity, not by host, so a run can span hosts when
each host uses distinct nodes. Evaluating the same node or node graph with
different host contexts in one run can reuse one host's result for another; if
one node task is reused across hosts, give each call a fresh run (a task wrapped
with `WithReconcile`, or `reconcile.Run`). Within one run, evaluation is
concurrent: a node is still evaluated at most once per run, and a cross-goroutine
cycle is detected.

---

## Task composition and middleware

Tasks compose because they are ordinary functions. Middleware has the shape
`func(ctx, next middleware.Next) error`; it may call `next` zero or more times,
and returning `nil` without calling it short-circuits the task. `With` builds an
immutable middleware chain and its terminals register a wrapped task. Middleware
runs in declaration order and may gate, scope or decorate `next`.

The builder can be shared and extended without mutation. Calling a registered
task from another task re-enters its middleware. See the API reference for the
builder and examples:
[api.md](api.md#with-taskbuilder-and-middleware).

### Global middleware (`Use`)

`Use` wraps every task run on a registry. Each middleware is invoked once around
the run of **all** dispatched tasks. `WithReconcile` uses this to share one
Reconcile run across the invocation. Effects flush once when the tasks finish.
Main middleware runs outside global middleware, so it can establish context
before the Reconcile run opens.

### Main middleware (`WithMiddleware`)

Main middleware runs after core and installed transport setup and before global
middleware. It can close over `main` locals, so application flags and
configuration enter the context. `BuiltinMiddlewares` supplies dry-run and
default remote target state, plus transport setup registered with `UseBuiltin`;
`AllMiddlewares` is the complete chain used by `Run`.

---

## Error handling and panic semantics

Gabric targets scripts and deployment tasks, not business logic. Deployment
failures are usually not locally recoverable, so operation terminals panic by
default. Error-returning terminals exist for probes and control flow.

Every task runs through a wrapper that recovers panics into errors, logs the
outcome and returns the error. `Main` logs a failed task and exits non-zero.
Panics are normalized with `PanicError`, so panicking with an error and returning
it produce equivalent errors for `errors.Is`/`errors.As`. `Must`, `TaskToTaskErr`
and `TaskErrToTask` bridge between the two task styles.

User-facing call patterns are in
[api.md](api.md#error-handling-patterns).

---

## Design principles

1. **Go is the DSL.** No separate task language.
2. **Tasks are ordinary functions.** Registration does not change composition.
3. **Runtime state belongs in `context.Context`, not globals.**
4. **Remote connections are execution-scoped resources.**
5. **Local and remote share the same operation model.**
6. **Deployment code should stay concise and imperative.**
7. **Concurrency should not fight global mutable state.**
