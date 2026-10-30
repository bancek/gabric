# Gabric

Write build, deploy, and server provisioning tasks in Go.

Gabric is a Go library that replaces Makefiles, deploy scripts, and Ansible
playbooks with one Go program. Use any Go library. Define idempotent resources
and use the same commands and file operations against local, SSH, Docker, or
custom targets.

## Short example

```go
package main

import (
	"context"

	"github.com/bancek/gabric"
	"github.com/bancek/gabric/gabricssh"
	"github.com/bancek/gabric/input"
	"github.com/bancek/gabric/reconcile"
	"github.com/bancek/gabric/reconcile/modules"
	"github.com/bancek/gabric/reconcile/modules/docker"
)

func S[T any](value T) input.Input[T] {
	return input.Static(value)
}

const imageName = "registry.example.com/app"

var tag = gabric.Default.FlagSet.String("tag", "dev", "image tag")

const composeTemplate = `services:
  app:
    image: {{ .image }}
    ports:
      - "8080:8080"
`

var _ = gabric.Use(
	gabric.WithRemoteSudo(&gabric.Sudo{}),
	gabric.WithReconcile,
)

var Build = gabric.Task("build", func(ctx context.Context) {
	image := imageName + ":" + *tag
	gabric.Local(ctx).Exec("docker", "build", "-t", image, ".").Run()
	gabric.Local(ctx).Exec("docker", "push", image).Run()
})

var provisionNode = reconcile.Group("provision").
	Contains(docker.DockerEngine().Build()).
	Node()

var Provision = gabric.NodeTask(provisionNode)

var deployNode = reconcile.Group("deploy").
	Contains(
		modules.File(S("/srv/app/compose.yaml")).
			Content(modules.TemplateData(
				S("/srv/app/compose.yaml"),
				S(composeTemplate),
				reconcile.Data("compose", func(ctx context.Context) (map[string]string, error) {
					return map[string]string{"image": imageName + ":" + *tag}, nil
				}).Node(),
			)).
			Mode(modules.FileMode(0o644)).
			Build().
			Node(),
		docker.Compose(S("/srv/app/compose.yaml")).Build().Node(),
	).
	Node()

var Deploy = gabric.NodeTask(deployNode)

var All = gabric.Task("all", func(ctx context.Context) {
	Build(ctx)
	Provision(ctx)
	Deploy(ctx)
})

func main() {
	gabricssh.InstallDefault()
	gabric.Main()
}
```

```bash
go run . -tag v1 build
go run . -ssh-host deploy@example.com provision
go run . -ssh-host deploy@example.com -tag v1 deploy
go run . -ssh-host deploy@example.com -tag v1 all
```

`Build` runs locally and pushes `registry.example.com/app:<tag>`. `Provision`
installs Docker Engine and the Compose plugin on the remote host. `Deploy` writes
the Compose file with the same tag and starts the project. `All` runs the three
tasks in order. The same `-tag` links the local build and the remote deploy.

Provisioning and deploy use idempotent resources. A converged host reports no
changes on the next run.

The example assumes Docker locally, a registry you can push to, a Debian or
Ubuntu host, and passwordless sudo. Add `--platform linux/amd64` to the build if
the host architecture differs.

For a minimal local task, see [Getting started](docs/getting-started.md).

## Install

Requires Go 1.27 or newer.

```bash
go get github.com/bancek/gabric
```

## Examples

- [homelab](examples/homelab): a complete server setup in one Go program.
- [internal/dev](internal/dev/dev.go): the build, test, format, and e2e tasks for this repository.
- [SSH](examples/ssh): a minimal SSH example.
- [Docker](examples/docker): a minimal Docker example.
- [API demo](examples/apidemo): the full remote API demo.

## Documentation

Start here:

- [Getting started](docs/getting-started.md): tasks, flags, local and remote operations, files, shells, and port forwarding.
- [Cookbook](docs/cookbook.md): recipes for hosts, config, sudo, middleware, concurrency, and more.
- [API reference](docs/api.md): signatures, defaults, and examples.

Deep dives:

- [Architecture](docs/architecture.md): execution model, context, concurrency, middleware, and errors.
- [SSH transport](docs/ssh/README.md): `gabricssh`, hosts, config, pooling, auth, host keys, and jump hosts.
- [Docker transport](docs/docker/README.md): `gabricdocker`, container context, runner, and target helpers.
- [Rsync transport](docs/rsync-transport.md): native rsync and the transport-provided fallback.
- [Interactive shell](docs/ssh/shell.md): the `ssh` task and terminal behavior.
- [Local port forwarding](docs/ssh/forward.md): the `forward` task and connection lifecycle.
- [Provisioning](docs/reconcile/README.md): idempotent desired state, with [modules](docs/reconcile/modules.md) and a [cookbook](docs/reconcile/cookbook.md).
- [Testing](docs/testing.md): test conventions and SSH/Docker infrastructure.

For the full API, run `go doc github.com/bancek/gabric`.

## Why Gabric

- Automation is Go. Import any Go library, use your editor and debugger, and keep tasks in the same language as the rest of the program.
- Tasks are normal functions. Registering a function with the CLI does not change how you call it from code.
- Runtime state lives in `context.Context`. Hosts, SSH pools, sudo, working directories, exec environments, and task arguments belong to the context. `Gabric` holds the registry and pre-run configuration, not per-run state.
- Local and remote share one API. `Local(ctx)` and `Remote(ctx)` provide the same command and file operations, and targets can be local, SSH, Docker, or custom runners.
- Transports are included. Gabric runs commands, transfers files, syncs with rsync, opens shells, and forwards ports over its own SSH stack. Docker uses `docker exec`.
- Provisioning is included. Idempotent resources probe the system, apply only on drift, and notify reactions when something changes. The built-in resources cover common server setup, and you can add your own in Go.
- One program. Use `go run` during development, or build one binary for the server.

## Provisioning

Gabric includes an idempotent resource layer. Resources probe the system and apply
only when something differs. A change can notify a reaction that runs once at the
end of the run. The built-in resources cover common server setup, and you can add
your own resources in Go.

See the [provisioning guide](docs/reconcile/README.md), the [modules reference](docs/reconcile/modules.md), and the [cookbook](docs/reconcile/cookbook.md).

## Relationship to Fabric

Gabric follows the Fabric 1.x style: short automation functions, commands over SSH, and file transfer.

Fabric kept most state in a global `env` dictionary and worked with one active host at a time. That made concurrent work and nested composition awkward. Gabric keeps that state in `context.Context`, so branches can use different hosts and run at the same time.

## Why the name Gabric?

The name Gabric is Fabric with the F replaced by G, for Go. It also refers to "gab" (to talk), which fits the debug logging.

## Relationship to other tools

- **Make** centers on file dependencies, targets, and child processes.
- **just** is a command runner with shell-string recipes.
- **go-task** uses a YAML Taskfile instead of Go code.
- **Invoke** is a Python task runner.
- **Ansible** uses YAML playbooks; Gabric uses Go code for both tasks and desired state.

## License

MIT. See [LICENSE](LICENSE). Copyright (c) Luka Zakrajsek.
