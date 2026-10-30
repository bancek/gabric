# Docker transport

`gabricdocker` is the Docker transport for Gabric. It runs remote operations
with `docker exec` and installs a container context.

- [gabricdocker.md](gabricdocker.md): installation, context, runner and target
  helpers.

## Install

```go
package main

import (
	"github.com/bancek/gabric"
	"github.com/bancek/gabric/gabricdocker"
)

func main() {
	gabricdocker.InstallDefault()
	gabric.Main()
}
```

`gabricdocker.Install(g)` installs on an explicit `*gabric.Gabric`. Calling it
twice panics.

## Flags

| Flag                | Meaning          |
| ------------------- | ---------------- |
| `-docker-container` | Docker container |

An empty container is allowed at install time. It produces `ErrNoContainer` when
a command runs.

## Ways to use it

| Goal                                              | API                                   |
| ------------------------------------------------- | ------------------------------------- |
| Run tasks with `-docker-container`                | `Install` plus `gabric.Remote(ctx)`   |
| Run one operation in a specific container         | `gabricdocker.Remote(ctx, container)` |
| Make `TargetOps` and Reconcile modules use Docker | `gabricdocker.Target(container)`      |
| Build a runner without installing context         | `gabricdocker.Runner()`               |

`Remote` returns `*gabric.RemoteOps` and is used immediately:

```go
gabricdocker.Remote(ctx, "app").Exec("uname", "-a").Run()
```

`Target` returns a `gabric.Target`, not ops. It is for `TargetOps`:

```go
targetCtx := gabric.NewTargetContext(ctx, gabricdocker.Target("app"))
gabric.TargetOps(targetCtx).Exec("uname", "-a").Run()
```

For a whole run, use the middleware:

```go
err := gabric.Run(ctx, calls,
	gabricdocker.WithDocker,
	gabricdocker.WithContainer("app"),
)
```

`WithDocker` installs the runner if none exists. `WithContainer` only sets the
container. See [gabricdocker.md](gabricdocker.md#choosing-a-form) for the full
comparison.

## Example

[`examples/docker/main.go`](../../examples/docker/main.go) is a minimal runnable
example:

```bash
docker run -d --name gabric-demo alpine sleep 60
go run ./examples/docker -docker-container gabric-demo hello_remote
docker rm -f gabric-demo
```

[`examples/apidemo/docker/main.go`](../../examples/apidemo/docker/main.go) runs the
full remote API demo:

```bash
docker run -d --name gabric-demo alpine sh -c 'apk add --no-cache rsync; sleep 3600'
go run ./examples/apidemo/docker -docker-container gabric-demo api
docker rm -f gabric-demo
```

## End-to-end

`./gab e2e-api-demo` runs the shared apidemo against Docker and SSH.
`./gab e2e` includes this task.

## Rsync

Both the host and the container need `rsync`. The `apidemo` command above
installs it in the container.

The native rsync sidecar runs locally. The container only needs `rsync` and a
shell, not the Gabric binary.

## Limits

- There is no Docker shell. `Shell` is provided by `gabricssh` only.
- Docker rsync uses the native transport only. The Docker transport installs
  `NoRsyncRsh`, so there is no system `ssh` fallback.
- No container provisioning helpers.
