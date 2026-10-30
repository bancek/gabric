# AGENTS.md

Guidance for agents (and humans) working in this repository.

## Commands

```bash
./gab ci               # gofmt check + go vet + go test -race (dogfoods Gabric)
./gab build            # go build ./...
./gab test             # go test -race ./...
./gab test-windows     # build ./gabrictest for Windows and run it under Wine
./gab format           # gofmt + prettier for .md files
./gab e2e              # e2e-test + e2e-api-demo
./gab e2e-test         # e2e-remote + e2e-ssh
./gab e2e-remote       # target ops: Docker, SSH, LocalRemote, Local
./gab e2e-ssh          # SSH-specific tests
./gab e2e-api-demo     # shared Docker and SSH apidemo
./gab e2e-modules      # manual module e2e (Docker/LocalRemote/Local)
./gab e2e-modules-heavy # manual heavy module e2e
./gab                  # list tasks

go run ./internal/dev ci   # same as ./gab ci
```

CI runs `./gab ci` plus separate Ubuntu e2e, macOS LocalRemote, and Windows Local
jobs (see `.github/workflows/ci.yml`); the runner is defined in `internal/dev`
using Gabric itself. The homelab Vagrant e2e is a task in
`examples/homelab` (`./gab e2e` from that directory).

## What this project is

Gabric is a Go-native task/deployment runner (Fabric 1.x inspired). Read:

- [docs/architecture.md](docs/architecture.md): execution model, context
  layout, concurrency, middleware, error semantics, design decisions.
- [docs/getting-started.md](docs/getting-started.md): usage.
- [docs/cookbook.md](docs/cookbook.md): recipes (hosts/config overrides, sudo,
  middleware, concurrency, library use).
- [docs/api.md](docs/api.md): curated API reference with examples.
- [docs/ssh/README.md](docs/ssh/README.md): the SSH transport overview.
- [docs/ssh/gabricssh.md](docs/ssh/gabricssh.md): `gabricssh` installation,
  flags, context helpers, runner, shell, forward and tasks.
- [docs/ssh/sshremote.md](docs/ssh/sshremote.md): the low-level `sshremote`
  package: hosts, config, pooling, auth, host keys, jump hosts.
- [docs/ssh/shell.md](docs/ssh/shell.md): the interactive shell task
  (`gabricssh.Shell(ctx)`).
- [docs/ssh/forward.md](docs/ssh/forward.md): local port forwarding
  (`gabricssh.Forward(ctx)`).
- [docs/docker/README.md](docs/docker/README.md): the Docker transport overview.
- [docs/rsync-transport.md](docs/rsync-transport.md): the native rsync transport (sidecar design,
  GNU rsync vs openrsync).
- [docs/reconcile/README.md](docs/reconcile/README.md): the standalone
  Reconcile engine and node builder (see also `architecture.md`, `api.md`,
  `modules.md`, `cookbook.md`) (lazy nodes, topics, effects, runs, dry run).
- [docs/reconcile/inputs.md](docs/reconcile/inputs.md): the standalone `input`
  package used by module builders for static and dynamic values.
- [docs/testing.md](docs/testing.md): test conventions and the SSH/Docker test
  infrastructure.
- [docs/gabrice2e.md](docs/gabrice2e.md): the real-target end-to-end test
  harness.
- [docs/reconcile/module-e2e-testing.md](docs/reconcile/module-e2e-testing.md):
  writing and running module e2e tests.

Key idea: **tasks are ordinary Go functions; runtime state lives in
`context.Context`; `Gabric` holds registration and pre-run configuration. The
CLI registry is optional (`gabric.Default` / `gabric.New()`).**

Toolchain: Go 1.27 (see `./gab` and CI); the module requires Go 1.27 (`go.mod`).

## Conventions

- **One test file per source file** (`foo.go` → `foo_test.go`). Shared test
  helpers go in their own `_test.go` file (e.g. `gabricssh/helpers_test.go`);
  root `testContext(t)` lives in `helpers_test.go`.
- **Error checks are error-first:** use `if err != nil`; never `if err == nil`.
- **Use `errors.AsType`:** prefer `errors.AsType[T](err)` over `errors.As(err, &target)`.
- **Ops are bound to context:** `Local(ctx).Put(...).Run()`,
  `Remote(ctx).Exec(...).Run()`, `Remote(ctx).Rsync().Local(...).Upload().Run()`.
  There are no argumentless `Local`/`Remote` singletons.
- **Target ops:** `TargetOps(ctx)` returns the local or remote operations
  selected by the context. `NewTargetContext` sets a `Target` value bundling the
  ops provider and its exec env context; `LocalTarget`, `RemoteTarget` and
  `LocalRemoteTarget` are the built-ins. `NewTargetOpsExecEnvContext` /
  `AppendTargetOpsExecEnvContext` configure the target ops env overlay.
  `LocalRemote` runs remote operations through a local runner. `Main` and `Run`
  default to remote.
- **Reconcile modules use composable data nodes.** Use the exported `...Data`
  nodes for reads instead of ad-hoc command helpers. Build a node outside
  `Check`/`Apply` to share one read, or inside to force a fresh probe after a
  mutation.
- Module input fields and parameters use `fooInput`; resolved values use `foo`.
- **Ops panic; `*Err` variants return errors:** `Run`, `Put`, `Get`, `Mkdir`,
  `Remove`, `Exists`, `Stat`, and `List` panic; `RunErr`,
  `Put(...).RunErr()`, `Get(...).BytesErr()`, `Remove(...).RunErr()`,
  `Exists(...).RunErr()`, `Stat(...).RunErr()`, and `ListErr` return errors.
  `Put`, `Get`, `Mkdir`, `Remove`, `Exists`, and `Stat` are builders
  (`Put(path).String(...).Mode(...).Run()`, `Get(path).Bytes()`,
  `Mkdir(path).Recursive().Run()`, `Remove(path).Recursive().Force().Run()`,
  `Exists(path).Dir().Run()`, `Stat(path).Run()`) like `Exec` and `Rsync`;
  `Exists.Run` returns a bool and `Stat.Run` returns a `FileStat` instead of the
  builder, and `List` returns entries directly.
- **Tasks:** `Task` is panic-style, `TaskErr` is error-first; `TaskFunc` is an
  alias for `middleware.Next` (`func(ctx) error`). The task wrapper recovers
  panics into errors and `Main` logs a failed task and exits non-zero.
- **Logging:** `*slog.Logger` in context; read with `gabric.Logger(ctx)` or
  `logger.LoggerFromContext(ctx)`. `Main` builds the base logger from `Level`
  and `LogFormat`; a logger supplied through `WithContext` wins.
- **Middleware scopes:** main middleware via `Main(WithMiddleware(...))`; global
  middleware via `gabric.Use(middlewares...)`, wrapping every task run; task
  middleware via `With(...).Task`; core built-ins (dry-run, default remote
  target) are added by `Run`, and transports add setup via `UseBuiltin` from
  `Install`. `WithReconcile` uses `Use` to run every task in one shared
  `reconcile.Run`. Node state is keyed by node identity, not
  host: distinct nodes can share a run across hosts, while reusing one node task
  under different host contexts needs a fresh run per call.
- **Reconcile middleware:** `Node.With` and `Effect.With` take
  `middleware.Middleware`. Node middleware scopes the whole node evaluation,
  including dependencies, and must call next exactly once. Effect middleware
  runs at flush and may skip the effect by not calling next.
- **CLI phases:** `ParseFlags` parses runner flags; `ParseTasks` resolves task
  calls; `Main`/`MainOption` compose them. `Run(ctx, calls, ...)` never parses.
- **Remote tests:** core operations use `internal/shelltest` through
  `LocalRemote`; SSH transport tests in `gabricssh` use `internal/sshtest`
  (in-process SSH server with a pluggable exec handler and `direct-tcpip` for
  jump hosts).
- **Do not add new dependencies** without strong justification; prefer stdlib
  and existing deps.
- **No code comments unless they add real value**; match surrounding style.
- Unix-only tests (needing `/bin/sh`, unix sockets, or POSIX path semantics)
  should be excluded with `//go:build unix` or skipped on Windows.

## Documentation style

Docs are reference text. Write plain statements and avoid constructions that
stage a reveal or add emphasis:

- Do not use em dashes or en dashes. Use commas, semicolons, parentheses, or a
  new sentence. Use straight apostrophes.
- Do not stage a reveal or manufacture suspense. Avoid lead-ins such as
  `The catch:`, `The upshot:`, `The idea:`, `The key difference is`,
  `Here's the thing`, and `This is where X comes in`.
- Do not use the two-beat negation `It does not X. It only Y.` State the
  behavior once.
- Do not write `This is how ...` or `This is what ...` when the subject can
  lead the sentence.
- Do not anthropomorphize code. Avoid `the reader honest`, `the engine knows`,
  `the engine cannot see`, and `the node decides`. Describe the behavior.
- Do not add narrative payoffs to explanations or postmortems. Avoid phrases
  such as `This was the last hang`, `waited forever`, and `a dead end`.
- Remove filler emphasis: `simply`, `just`, `actually`, `exactly`, `usually`,
  `which is handy`, `worth noting`, `worth recording`, and
  `One possible shape`.
- Do not reassure the reader with `you never have to` or `you do not have to`.
  Say `X is optional`.
- Prefer plain statements over aphorisms. A short contract such as
  `The first transport wins` is fine, but do not repeat signature lines across
  files.
- Keep repeated tables and context across docs. The duplication is deliberate
  and helps each page stand alone.
- Apply these rules to Go doc comments and code comments too. Comments describe
  behavior; they do not narrate it.

## Layout

```
doc.go                                 package docs
gabric.go                              Gabric runner: registry, config, FlagSet, Use (New/Default)
tasks.go                               Task/TaskErr, TaskCall, ParseTasks, Dispatch
run.go                                 BuiltinMiddlewares/AllMiddlewares/Run
target.go                              Target/TargetOps/NewTargetContext, target middleware
target_env.go                          target ops exec environment context helpers
target_working_dir.go                  target ops working directory context helpers
remote_runner.go                       RemoteRunner contract and context
local_runner.go                        NewLocalRunner
exec_exit_error.go                     ExecExitError
local_remote.go                        LocalRemote target and local runner
task_builder.go                        TaskBuilder (With/Task/TaskErr)
parallel.go                            Parallel/ParallelErr
must.go                                Must(err error) panic helper
main.go                                CLI (Main/MainOption/ParseFlags)
exec.go local_exec.go remote_exec.go   Exec + backends
local_backend.go remote_backend.go     backend interfaces, Func adapters, context
requests.go                            shared backend request types
local_*.go remote_*.go                 local/remote ops, context helpers
exec_env.go                            opaque ordered environment value (ExecEnv, EnvVar)
local_exec_env.go                      local exec environment context helpers
remote_exec_env.go                     remote exec environment context helpers
list.go local_list.go remote_list.go   directory listing
stat.go local_stat.go remote_stat.go   file metadata (FileStat, StatOps)
rsync.go                               RsyncOps builder
rsync_*.go                             native rsync transport + sidecar
ops.go                                 Ops interface
reconcile.go                           WithReconcile (gabric <-> reconcile middleware)
report.go                              WithReport/WithReconcileReport (gabric <-> reconcile report)
gabricssh/                             SSH integration: install, context, runner, shell, forward, tasks
gabricdocker/                          Docker integration: install, container context, runner, target
sshremote/                             SSH: hosts, managers, clients, auth, known hosts
internal/terminal/                     terminal helpers: size, raw mode, resize
input/                                 lazy values for builders (standalone, stdlib only)
reconcile/                             lazy desired-state engine + node builder (standalone)
reconcile/modules/                     base: File, Dir, TemplateData, Command, composable data nodes
reconcile/modules/linux/               User, SSHAuthorizedKeys, Sysctl
reconcile/modules/ufw/                 UFW firewall
reconcile/modules/ufwdocker/           ufw-docker integration
reconcile/modules/systemd/             systemd services and effects
reconcile/modules/service/             service cover and init-system detection
reconcile/modules/apt/                 apt packages
reconcile/modules/apk/                 apk packages
reconcile/modules/pkg/                 package cover and distro detection
reconcile/modules/git/                 git repositories
reconcile/modules/docker/              Docker networks and Compose
reconcile/modules/httpfile/            HTTP file downloads
reconcile/modules/github/              GitHub raw downloads
reconcile/modules/rsync/               rsync uploads
reconcile/modules/moduletest/           public module e2e harness and images
logger/                                slog context logger
dryrun/                                shared dry-run context marker (dryrun.With/Enabled)
middleware/                            shared context middleware type and chain (Next/Middleware/Apply)
gabrictest/                            high-level in-process test harness (Test, Side)
gabrice2e/                             public end-to-end test harness (Test, DockerTest, SSHTest)
internal/command/                      shared parsed-command record types
internal/shelltest/                    in-process shell and virtual filesystem test core
internal/sshtest/                      in-process SSH test server (test support)
internal/dev/                          Gabric-based dev/CI task runner (./gab)
internal/e2eimages/                    internal Alpine images shared by e2e suites
internal/apidemo2e/                    shared Docker and SSH apidemo e2e test
internal/gabricremote2e/               target operations e2e (Docker, SSH, LocalRemote, Local)
internal/gabricsshe2e/                 SSH-specific e2e tests
internal/testutils/                    wait/assert helpers shared by tests
docs/                                  architecture, getting-started, API, cookbook, ssh/, docker/, rsync, testing
examples/ssh/                          runnable SSH example CLI
examples/docker/                       runnable Docker example CLI
examples/apidemo/                      shared full remote API demo, SSH and Docker entry points
examples/homelab/                      complete Reconcile/homelab example (Vagrant, local mode, e2e)
```
