# Testing

## Running

```bash
./gab ci                       # gofmt check + go vet + go test -race ./...
./gab test-windows             # build ./gabrictest for Windows and run it under Wine
./gab e2e                      # remote, SSH, and apidemo e2e suites (Docker)
go test -race ./... -count=1   # the test step only
go test ./sshremote/ -run TestRemote -v
go test ./gabricssh/ -run TestIntegration -v
go test ./gabricdocker/ -v
go test . -run TestRun -v      # CLI dispatch tests
```

CI runs `./gab ci` on `ubuntu-latest` and `windows-latest`, plus separate Ubuntu
e2e, macOS LocalRemote, and Windows Local jobs (see `.github/workflows/ci.yml`);
the runner is defined in `internal/dev` using Gabric itself. Windows runs the
full race-enabled suite.

The homelab VM e2e is the homelab `e2e` Gabric task, run from
`examples/homelab` with `./gab e2e`. It is local-only, needs Vagrant
plus a provider, and drives Vagrant through `gabric.Local.Exec`; it is not part
of CI.

## Conventions

- **One test file per source file**: `foo.go` → `foo_test.go`. Shared test
  helpers live in their own `_test.go` file (not tied to a source file).
- **Package choice**: use `package gabric` (internal test) when you need
  unexported symbols (e.g. `g.tasks`, `newExec`, `buildRsyncArgs`); use
  `package gabric_test` for runnable examples and purely exported API. The
  `gabricssh` and `sshremote` packages do the same.
- **Logging in tests**: build a discard `*slog.Logger` and put it in the
  context; the logger is required by the dial path.
- Unix-only tests (needing `/bin/sh`, unix sockets, or POSIX host paths) should
  be excluded with `//go:build unix` or skipped on Windows.
- **No sleeps in tests.** Never use `time.Sleep` (or a "wait a bit just in
  case"). To wait for a condition use `testutils.Eventually`, and to assert
  something keeps holding use `testutils.Consistently`
  (`internal/testutils`); prefer real synchronisation (channels, handshakes)
  over both where possible.

### Adding an operation

When adding a local/remote operation:

- implement it on `Ops`, `LocalOps`/`RemoteOps`, and the local/remote backends
- add `local_<op>_test.go` and `remote_<op>_test.go` when the implementation is
  split, plus `<op>_test.go` for shared code
- test core remote behavior through `LocalRemote` and `internal/shelltest`, not
  by stubbing `Exec`; test SSH transport behavior in `gabricssh` with
  `internal/sshtest`
- add the `gabrictest` call record and `Assert<Op>Called` when the operation is
  visible to high-level tests
- add an e2e test when the behavior depends on a real shell, sshd, sudo, or
  process transport

## Helpers

- `testutils.Eventually(tb, timeout, cond)` / `testutils.Consistently(tb, duration,
cond)` (`internal/testutils`): the gomega-style wait and assert helpers described
  above. Shared by the root, `gabricssh`, `sshremote` and
  `internal/gabricsshe2e` test packages.
- `testContext(t)` (`helpers_test.go`, root): a `context.Context` carrying a
  discard `*slog.Logger` (built with `logger.NewTextHandler`).
- `testContextBuffer(t)` (`helpers_test.go`): same, but returns the `*bytes.Buffer`
  the logger writes to (used to assert on `LogErrors`/`Quiet`).
- `localRemoteContext(t)` and `localRemoteContextWithExec(t, handler, sudo)`
  (`local_remote_helpers_test.go`, root): run remote operations through
  `internal/shelltest` without SSH. Use the second for a custom exec handler.
- `AssertCommands` (`internal/shelltest`): compare the full recorded command
  slice, including arguments, directory and sudo metadata.
- `startSSHServer(t, opts...)` and `remoteServerContext(t, srv, sudo)`
  (`gabricssh/helpers_test.go`): start an `internal/sshtest` server and build a
  `gabricssh` context wired to it (config, manager, host, runner, optional sudo).
- `startServer(t, opts...)`, `testHost(srv)`, `testConfig()`,
  `testManagerConfig(idle, max)`, `testLogger()`, `testRemote(...)`,
  `testContext()` (`sshremote/remote_test.go`): the `sshremote`-package equivalents
  (the Remote owns its config; logger is passed explicitly; pool limits come
  from the config).

## Reconcile

Reconcile is tested without SSH or Docker (`reconcile/*_test.go`): nodes and
dependency tracking (memoization, ordering, dynamic deps, cycles, invalidation),
topics and effects (dedup, cascade, `DefaultMaxTopicFlushes`, errors), the node
builder (resource satisfied/drift, data and composition nodes, `CheckNeeds` vs lazy
`Needs`, dry run, immutability), run isolation, the `ufw` scenario,
flush-then-renotify, and the idempotent-rerun invariant.

## High-level harness: `gabrictest`

`gabrictest` runs tasks and Reconcile nodes against independent local and remote
in-memory sandboxes. It records high-level `Put`/`Get`/`Mkdir`/`Remove`/`Exists`/
`Stat`/`List` and `Rsync` calls and runs `Exec` through the shell core, without
SSH. Its context sets a remote target by default; set a local target before
`WithContext` to exercise local modules.

`gabrictest` implements `Stat` natively over `FS.Stat`/`FS.Owner` and records
`Call.Stat`; it does not run the remote `stat` script. The real remote script is
exercised by `remote_stat_test.go` through `localRemoteContext` and the shell
core, whose `stat` handler supports `%f`/`%a`/`%U`/`%G` and whose `uname` handler
reports `Linux`. The Alpine/BusyBox path, including a missing path and sudo, is
covered by `internal/gabricsshe2e` (`TestStat`, `TestSudoPut`, `TestSudoUser`).

```go
gt := gabrictest.New(t)
ctx := gt.Context()

gt.Remote().WriteFile("/srv/www/compose.yaml", []byte("services: {}\n"), 0o644)
gt.Remote().Handle("docker", func(cmd gabrictest.Command) gabrictest.Result {
    if cmd.HasArg("--dry-run") {
        return gabrictest.Result{Stderr: composeDrift}
    }
    return gabrictest.Result{}
})

gt.RunNode(ctx, docker.Compose(input.Static("/srv/www/compose.yaml")).Build().Node())

gt.Remote().AssertExecuted("docker", "compose", "-f", "/srv/www/compose.yaml",
    "up", "-d", "--remove-orphans")
gt.Remote().AssertPutCalled(gabric.PutRequest{
    Path:         "/srv/www/compose.yaml",
    Content:      bytes.NewBufferString("services: {}\n"),
    Mode:         0o644,
    EnsureParent: true,
})
```

Use `gt.Remote().HandleRsync(func(req gabric.RsyncRequest) gabrictest.Result { ... })`
to simulate rsync output or failure without running rsync.

Use `gt.RunNode(ctx, node)` to evaluate a Reconcile node in a run, or
`gt.RunNodeErr(ctx, node)` when the test needs the value and error separately.
`RunNode` fails the test on error.

`gt.Context()` is `gt.WithContext(t.Context())`. `WithContext` fills in the
harness logger, `/` working directories, and the local/remote backends only when
the supplied context does not already have them.

Build expected command records with `gt.Command(name, args...)`.

`gt.Local()` and `gt.Remote()` expose the same `Side` API. Filesystem helpers
(`WriteFile`, `ReadFile`, `ReadDir`, `Stat`, `MkdirAll`, `SetOwner`) inspect and
seed the sandbox. `AssertExecuted` checks shell commands; `AssertPutCalled`,
`AssertGetCalled`, `AssertMkdirCalled`, `AssertRemoveCalled`,
`AssertExistsCalled`, `AssertStatCalled`, `AssertListCalled` and
`AssertRsyncCalled` check backend operations, not filesystem state. Use
`Stat`/`ReadFile` for state. `AssertCommands` and `AssertCalls` compare the
complete ordered logs.

`Calls()` returns the high-level backend operations in execution order. `Call`
has exactly one non-nil request field. Recorded `Put.Content` is always a
`*bytes.Buffer`, and `Call.Equal` compares it without consuming it. Expected
`Put` content in `AssertPutCalled` must also be a `*bytes.Buffer`; other reader
types compare unequal. `Call.Clone` returns a deep copy, cloning Put content
buffers. `ResetCommands()` and `ResetCalls()` clear the recorded command and
call logs independently without changing files.

Paths in `Assert*Called` and `Calls()` comparisons are canonical virtual slash
paths. Normalization happens when operations enter the sandbox, not during
equality checks.

`gabrictest.Command` is `gabric.Command`; `Command.Sudo` is `*gabric.Sudo`
(`nil` means no sudo). Ownership is simulated as labels via `SetOwner`; real
root privileges are not simulated.
`ExecRequest`'s `ExtraFiles`/`AfterStart` are rejected by the local harness.
The harness applies `ExecRequest.Env` on top of its base
`PATH=/usr/bin:/bin` and records the resolved entries in `Command.Env`; nested
`sh` and `sudo` commands inherit them. `AssertCommands` compares the env and
`AssertExecuted` returns the recorded command so a test can check `.Env`.
`Command.Env` contains real values, including sensitive ones; it is a test
record, not a redacted log.
The harness records an `Rsync` call from `Remote(ctx).Rsync()`, including the
direction, paths, flags, excludes and sudo metadata, but it does not run local
rsync or emulate the transport. A `HandleRsync` handler can return
`Stdout`/`Stderr`; they are visible when the transfer uses
`Capture`/`CaptureStdout`/`CaptureStderr`. Without a handler, the call is
recorded and succeeds with empty output. Real transport behavior remains
covered by the rsync integration tests and Docker e2e tests.

### Windows

`gabrictest` supports Windows for file operations and command handlers that use
absolute virtual paths. `./gab test-windows` builds the `gabrictest` test binary
for the host architecture and runs it under Wine using
`internal/dev/Dockerfile.wine`.

CI runs the full suite on `windows-latest` with the race detector. The shell
test core translates between Windows host paths and POSIX virtual paths, so
`cd`, relative paths and `test` work on Windows. Shelltest's own tests remain
Unix-tagged; tests that assert POSIX `pwd` output are also Unix-only.

## `internal/sshtest`

An in-process SSH server (no Docker, no network beyond `127.0.0.1`) used to test
the remote stack deterministically.

```go
srv, err := sshtest.Start(
	sshtest.WithExecHandler(func(cmd string, stdin io.Reader, stdout, stderr io.Writer) int {
		io.WriteString(stdout, "ok")
		return 0
	}),
)
```

Options:

| Option                             | Effect                                                                         |
| ---------------------------------- | ------------------------------------------------------------------------------ |
| `WithExecHandler(ExecHandler)`     | handle `exec` requests; return the exit status                                 |
| `WithShellHandler(ExecHandler)`    | handle interactive `shell` requests (default: the exec handler with `/bin/sh`) |
| `WithAuthorizedKey(ssh.PublicKey)` | accept this client key (default: the generated client key)                     |
| `WithNoClientAuth()`               | disable authentication                                                         |
| `WithDirectTCPIP()`                | enable `direct-tcpip` channels (jump hosts)                                    |

Accessors: `Addr()`, `Port()`, `User()`, `HostKey()`, `KnownHosts()`
(a `known_hosts` line for the server), `ClientKeyPath()` (a private key the
server authorizes), `ConnCount()` (accepted TCP connections, used to assert
connection-pool behavior), `PTYTerm()`/`PTYSize()` (from the last `pty-req`) and
`LastWindowSize()` (from the last `window-change`).

## Remote test strategy

**Coverage principle:** anything that cannot be realistically exercised with
unit tests must have an end-to-end test through a real handshake. Unit tests are
not a substitute for behaviour that only shows up against a real server or a real
process: real sshd/sudo, transport bridging, process lifecycle, the rsync
transport, the interactive shell. Use in-process `internal/sshtest` where that is
enough, and Docker (`internal/gabricsshe2e`) otherwise.

- **Core operations**: remote-style operations are tested over
  `LocalRemote` and `internal/shelltest` without SSH. This covers commands,
  files, stat, list, mkdir and sudo command construction.
- **SSH transport** (`gabricssh` with `internal/sshtest`): fast, Docker-free.
  Covers the runner, shell, forwarding, tasks, host/config/manager context
  helpers, rsync fallback and shelltest-over-SSH integration.
- **Docker transport** (`gabricdocker`): unit tests use a fake local backend to
  check the `docker exec` argv, streams, container context, installation,
  first-transport-wins and target helpers. Real Docker coverage comes from
  `internal/gabricremote2e` and `e2e-api-demo`.
- **Transports**: every `sshremote.Transport` gets an end-to-end test through a real
  handshake. In-process (`internal/sshtest`): `JumpTransport`
  (`TestRemoteClientProxyJump`), `CommandTransport`
  (`TestRemoteClientCommandTransport`), `DialTransport`
  (`TestRemoteClientDialTransport`), `ForwardTransport`
  (`TestRemoteClientForwardTransport`), `SessionCommandTransport`
  (`TestRemoteClientSessionTransport`), and `StdioTransport`
  (`TestRemoteClientStdioTransport`: an orchestrator process hands a child
  process a connected fd as its stdio, and the child runs the client). Docker
  (`internal/gabricsshe2e`): `JumpTransport` (`TestProxyJump`), `CommandTransport`
  (`TestCommandTransport`), `DialTransport` (`TestDialTransport`),
  `ForwardTransport` (`TestForwardTransport`), `SessionCommandTransport`
  (`TestSessionTransport`). `StdioTransport` has no Docker variant.
- **rsync transport** (`rsync_integration_test.go`, root and
  `gabricssh/rsync_integration_test.go`): runs a real local rsync (GNU rsync or
  openrsync) through the native sidecar transport. The core test uses
  `LocalRemote`; the SSH test uses an `internal/sshtest` server whose exec
  handler runs the command locally. It needs a local `rsync` and `/bin/sh` but
  **no** system `ssh`; it skips when rsync is missing.
- **Interactive shell** (`gabricssh/shell_test.go`): drives
  `gabricssh.Shell(ctx)` against `internal/sshtest` with injected IO, covering a
  non-PTY command round-trip, PTY negotiation, resize forwarding and exit-status
  wrapping.
- **End-to-end**: `internal/gabricremote2e` runs the same target operation tests
  against Docker, SSH, LocalRemote, and Local. `internal/gabricsshe2e` covers
  SSH specifics: real sshd, sudo as another user, the interactive shell,
  forwarding, jump hosts, and transports. `internal/apidemo2e` covers the full
  public Docker and SSH apidemo paths. Docker and SSH targets skip when Docker is
  unavailable; LocalRemote runs on macOS and Linux; Local runs on macOS, Linux,
  and Windows.

### End-to-end tests

See [gabrice2e.md](gabrice2e.md) for the real-target test harness API.

```bash
./gab e2e-remote       # Docker, SSH, LocalRemote, and Local target ops
./gab e2e-ssh          # SSH-specific tests
./gab e2e-api-demo     # shared Docker and SSH apidemo
./gab e2e-modules      # manual module e2e (Docker/LocalRemote/Local)
./gab e2e-modules-heavy # manual heavy module e2e
./gab e2e              # all of the above except module e2e
```

`e2e-test` is an alias for `e2e-remote` plus `e2e-ssh`. Module e2e is manual
only; it is not part of CI or `./gab e2e`. See
[module e2e testing](reconcile/module-e2e-testing.md) for writing and running
the module suites.

- `internal/e2eimages/Dockerfile.alpine-rsync` is the minimal Docker target.
- `internal/e2eimages/Dockerfile.alpine-openssh` is the real sshd image with
  `openssh`, `rsync`, `sudo`, `AllowTcpForwarding yes` and test users.
- `e2e-api-demo` runs `internal/apidemo2e`, which runs the shared apidemo
  against Docker and SSH.
- `gabrice2e` builds each image once, starts one container per test with `--rm`,
  installs a freshly generated in-memory key for SSH, and waits for readiness.
- `TestProxyJump` starts a second container on a user-defined network as a jump
  host. `TestCommandTransport` re-executes the test binary as the
  `ProxyCommand` connector.
- Docker and SSH targets skip when Docker is unavailable or when
  `GABRIC_E2E_DOCKER_DISABLED` is set (for example on a Windows runner whose
  Docker daemon only runs Windows containers). The rsync tests skip when the
  host has no `rsync`.
- `GABRIC_E2E` env variable is used to control whether e2e tests run.
  `GABRIC_E2E_DOCKER_DISABLED` disables the Docker-backed targets.
