# gabrice2e

`gabrice2e` is Gabric's real-target end-to-end test harness. It starts Docker
containers, SSH servers, LocalRemote targets, and plain local targets, and it
keeps image builds, readiness, and cleanup together.

Use it for tests that need real tools such as sshd, sudo, rsync, `stat`, or
`docker exec`. Use [`gabrictest`](testing.md#high-level-harness-gabrictest) when
the in-memory shell and virtual filesystem are enough.

## Quick start

```go
package mymodule_test

import (
	_ "embed"
	"testing"

	"github.com/bancek/gabric"
	"github.com/bancek/gabric/gabrice2e"
)

//go:embed Dockerfile
var dockerfile []byte

var image = gabrice2e.ImageFromDockerfileBytes("mymodule", dockerfile)

func TestMain(m *testing.M) {
	gabrice2e.Main(m)
}

func TestMyModule(t *testing.T) {
	g := gabrice2e.Docker(t, image)

	ctx := g.Context()
	ops := gabric.TargetOps(ctx)

	if err := ops.Put("/tmp/hello.txt").String("hello").RunErr(); err != nil {
		t.Fatalf("Put() error: %v", err)
	}

	if got, want := ops.Get("/tmp/hello.txt").String(), "hello"; got != want {
		t.Fatalf("content = %q, want %q", got, want)
	}
}
```

`gabrice2e.Main` is a helper for `TestMain` that skips the package unless
`GABRIC_E2E` is set, so plain `go test ./...` stays Docker-free. Run e2e
tests with `GABRIC_E2E=1 go test ./mymodule`.

The embedded Dockerfile needs the tools the module uses and a long-running
command, for example:

```dockerfile
FROM alpine:3.24

RUN apk add --no-cache rsync sudo

CMD ["sleep", "86400"]
```

## Targets

### Docker

```go
g := gabrice2e.Docker(t, image, opts...)
c := g.Container()
```

`DockerTest` implements the common `Test` methods and exposes the live
`Container`.

The context has:

- Docker container context
- Docker runner
- `gabric.NoRsyncRsh` resolver: rsync uses the native transport only, with no
  system `ssh` fallback
- target operations through `gabric.Remote`

### SSH

```go
g := gabrice2e.SSH(t, image, "deploy", opts...)

host := g.Host()
networkHost := g.NetworkHost()
c := g.Container()
user := g.User()
```

`SSH` starts a container from an image with sshd and a real user, generates an
Ed25519 key, installs the public key in that user's `authorized_keys`, waits for
sshd, and keeps the private key in memory as an `ssh.Signer`.

The user must exist in the image with a passwd entry and primary group entry.
`SSH` resolves the home directory and primary group from the container.

The context has SSH config, manager, host, runner, and target operations.

`Host()` uses `127.0.0.1:<published port>` and the supplied user.

`NetworkHost()` uses `<container-name>:22` and the supplied user, and is for
connections made from another container on the same Docker network.

Custom host and transport, with `target` as the target under test and `jump` as
the jump host:

```go
host := target.NetworkHost().
	WithTransport(sshremote.JumpTransport{
		Host: jump.Host(),
	})

g := target.WithHost(host)
g = target.WithHostTransport(transport)
```

`WithHost` shares the container, config, manager, and signer, and builds a new
context with the supplied host. The original `SSH` owns the manager cleanup.

### LocalRemote

```go
g := gabrice2e.LocalRemote(t)
```

Uses remote-style operations through the local machine and `/bin/sh`. The remote
working directory is a fresh `t.TempDir()`. The target supports rsync.

`LocalRemote` skips on Windows.

### Local

```go
g := gabrice2e.Local(t)
```

Uses plain local operations with a fresh local working directory. The target has
no rsync and runs on Windows. `LocalTarget` uses `Local` and the local exec
environment; it does not mirror the remote working directory or remote env.

### Common Test methods

```go
func (g *Test) T() testing.TB
func (g *Test) Context() context.Context
```

Run and report helpers:

```go
func (g *Test) RunNode[T any](ctx context.Context, node *reconcile.Node[T]) T
func (g *Test) RunNodeErr[T any](ctx context.Context, node *reconcile.Node[T]) (T, error)

func (g *Test) RunReport[T any](
	ctx context.Context,
	node *reconcile.Node[T],
) (T, []reconcile.ReportResult)

func (g *Test) RunReportErr[T any](
	ctx context.Context,
	node *reconcile.Node[T],
) (T, []reconcile.ReportResult, error)

func (g *Test) RunReportRef(
	ctx context.Context,
	ref reconcile.NodeRef,
) ([]reconcile.ReportResult, error)
```

`RunReport*` installs a fresh `reconcile.ReportCollector` for the call, so the
results contain only that run. `RunReportErr` returns the collected report even
when the run fails. `RunReportRef` accepts any node or group cover.

Operations are target-selected for every target:

```go
ops := gabric.TargetOps(g.Context())
```

Docker, SSH, and LocalRemote also support direct remote operations:

```go
ops := gabric.Remote(g.Context())
```

Local is available only through `gabric.TargetOps`, not `gabric.Remote`.

### Rsync capability

Docker, SSH, and LocalRemote support rsync. Local does not. Check the capability
before using `TargetRsyncOps`:

```go
rsync, err := gabric.TargetRsyncOps(g.Context())
if errors.Is(err, gabric.ErrNoRsyncTarget) {
	t.Skip("target does not support rsync")
}
if err != nil {
	t.Fatal(err)
}
```

## Images

```go
type Image struct{ /* ... */ }

func ImageFromDockerfile(path string) Image
func ImageFromDockerfileBytes(name string, content []byte) Image
func ImageFromName(ref string) Image

func (i Image) Name() string
func (i Image) Append(dockerfile string) Image
```

- `ImageFromName` uses an existing image.
- `ImageFromDockerfile` reads the Dockerfile and uses its directory as context.
- `ImageFromDockerfileBytes` uses an empty temporary context.
- `Append` returns a new image with the Dockerfile text appended.
- `Append` panics for an existing image reference.

The image is built once per image spec per test process:

```text
docker image inspect gabrice2e-<name>:<hash>  # reuse when present
docker build -f <temp>/Dockerfile -t gabrice2e-<name>:<hash> <context>
```

The tag includes the Dockerfile and context hash. Build commands use
`gabric.Local(t.Context())`.

The repository's own suites use private images in `internal/e2eimages`:

```text
internal/e2eimages/
  Dockerfile.alpine-rsync
  Dockerfile.alpine-openssh
```

That package is not importable by external users. Provide your own image with
`ImageFromDockerfile`, `ImageFromDockerfileBytes`, or `ImageFromName`.

The public module e2e harness exposes its target images from
`reconcile/modules/moduletest`.

## Containers

```go
c := gabrice2e.RunContainer(t, image, opts...)

c.Name()
c.Image()
c.Port(22)
c.Addr(22)
c.Logs()
c.Stop()
```

`RunContainer` builds the image if needed and starts a new container:

```text
docker run -d --rm --name <name> ... <image> [command]
```

The name defaults to:

```text
gabrice2e-<uuid.NewV4()>
```

The container removes itself when it stops. `Stop` runs `docker stop` and is
idempotent. `t.Cleanup` calls `Stop`.

`Logs` returns the container logs. When a test has failed, `Stop` logs them
before stopping the container. `RunContainer` also includes logs when the
container exits immediately.

`Port` and `Addr` are valid only while the container is running.

Options:

```go
gabrice2e.WithContainerName(name)
gabrice2e.WithContainerPrivileged()
gabrice2e.WithContainerCgroupnsHost()
gabrice2e.WithContainerUser(user)
gabrice2e.WithContainerEnv(kv...)
gabrice2e.WithContainerVolume(hostPath, containerPath, readOnly)
gabrice2e.WithContainerAddHost(host, ip)
gabrice2e.WithContainerStopSignal(signal)
gabrice2e.WithContainerSecurityOpt(opts...)
gabrice2e.WithContainerTmpfs(path)
gabrice2e.WithContainerPort(containerPort)
gabrice2e.WithContainerCommand(argv...)
gabrice2e.WithContainerNetwork(n)
```

`RunContainer` and `RunNetwork` skip when Docker is unavailable or when
`GABRIC_E2E_DOCKER_DISABLED` is set. Use the variable where the Docker daemon
cannot run Linux containers, such as a Windows runner with Windows containers.

### Networks

```go
net := gabrice2e.RunNetwork(t)
g := gabrice2e.SSH(t, image, "gabric",
	gabrice2e.WithContainerNetwork(net),
)
```

`RunNetwork` creates a Docker network and removes it with `t.Cleanup`.

## Context wiring

These are the contexts the target constructors build. Most tests only call
`g.Context()`.

Docker:

```go
ctx = gabricdocker.NewContainerContext(ctx, container)
ctx = gabric.NewRemoteRunnerContext(ctx, gabricdocker.Runner())
ctx = gabric.NewRsyncRshContext(ctx, gabric.NoRsyncRsh)
ctx = gabric.NewTargetContext(ctx, gabric.RemoteTarget)
```

SSH:

```go
ctx = gabricssh.NewConfigContext(ctx, config)
ctx = gabricssh.NewManagerContext(ctx, manager)
ctx = gabricssh.NewHostContext(ctx, host)
ctx = gabricssh.NewRunnerContext(ctx)
ctx = gabric.NewTargetContext(ctx, gabric.RemoteTarget)
```

LocalRemote:

```go
ctx = gabric.NewRemoteWorkingDirContext(ctx, t.TempDir())
ctx = gabric.NewLocalRemoteContext(ctx)
ctx = gabric.NewTargetContext(ctx, gabric.LocalRemoteTarget)
```

Local:

```go
ctx = gabric.NewLocalWorkingDirContext(ctx, t.TempDir())
ctx = gabric.NewTargetContext(ctx, gabric.LocalTarget)
```

## Assertions

Use `g.T()` and the normal testing methods:

```go
out := gabric.TargetOps(g.Context()).
	Exec("echo", "hello").
	CaptureStdout().
	Run().
	StdoutStr()

if out != "hello\n" {
	g.T().Fatalf("stdout = %q, want %q", out, "hello\n")
}
```

The existing suites use direct checks. The module e2e harness adds focused
assertions in `reconcile/modules/moduletest`; see
[module e2e testing](reconcile/module-e2e-testing.md).

## Cleanup

`gabrice2e` owns cleanup:

- images stay cached
- containers use `--rm` and `Stop`
- networks are removed
- SSH managers are closed
- local working directories come from `t.TempDir()`

Use `g.T().Cleanup` for extra fixtures.

## Existing suites

- `internal/gabricremote2e` runs target operations against Docker, SSH,
  LocalRemote, and Local.
- `internal/gabricsshe2e` covers SSH-specific behavior.
- `internal/apidemo2e` runs the shared apidemo against Docker and SSH.

See [testing.md](testing.md#end-to-end-tests) for commands and suite details.
