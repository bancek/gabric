# Reconcile

Reconcile is a standalone engine for idempotent desired-state work. It is lazy
and topic-driven, and includes a small node builder. It does not import the
`gabric` core package and can be used on its own.

- [Architecture](architecture.md): the model, covering nodes, topics, effects,
  runs, dry run, concurrency, and design notes.
- [Inputs](inputs.md): the standalone `input` package used by module builders
  for static and dynamic values.
- [API](api.md): the node builders (`Data`, `Resource`, `Group`; `Check`/`Apply`,
  `CheckNeeds`/`ApplyNeeds`, `ApplyInvalidates`, `Notify`, `Node`), the
  `Map`/`Filter`/`IfChanged` helpers, and `DynamicResource` for runtime resource
  selection.
- [Modules](modules.md): ready-made nodes for packages, files, templates,
  systemd units, git, and more.
- [Module e2e testing](module-e2e-testing.md): writing and running tests against
  real targets.
- [Cookbook](cookbook.md): recipes for bootstrap, dynamic content, restarts, dry
  run, and parallelism.

Ready-made modules for common system resources (packages, files, templates,
systemd units, ufw, docker compose, git, …) live in
`github.com/bancek/gabric/reconcile/modules`; each is assembled from the builder
in this package. See [Modules](modules.md).

## The model

- **Node**: a lazy, memoized unit of work. Reading it (`Value()`) runs it once
  per run. A value node is either a **resource** (probes with `Check`, mutates
  with `Apply` only on drift) or a **data** node (a read via `Data`).
- **Group**: a container that runs its members in order. It has no value and is
  not memoized, so reading it runs them every time (they are memoized). To reset
  a group, start a new `Run`.
- **Topic**: a valueless change signal. A node that changed calls `Notify`.
- **Effect**: a reaction subscribed to one or more topics; it runs once per
  notification when the run flushes.
- **Run**: the transaction. `Run(ctx, fn)` opens an isolated scope and flushes
  its effects when `fn` returns. All state lives in `ctx`; nodes are stateless.

## Quick start

```go
package main

import (
	"context"
	"log"

	"github.com/bancek/gabric/reconcile"
)

// A node that changed notifies this topic.
var ssh = reconcile.NewTopic("ssh")

// A resource: probes the remote config and rewrites it only on drift.
var sshdHardening = reconcile.ResourceBool("sshd-hardening",
	func(ctx context.Context) (bool, error) {
		return remoteFileMatches(ctx, sshdConf, hardening) // probe: read-only
	},
	func(ctx context.Context) error {
		return writeRemoteFile(ctx, sshdConf, hardening, 0o644)
	}).
	Notify(ssh).
	Node()

// Restart ssh once, and only when the config changed.
var _ = reconcile.NewEffect("restart-ssh", []*reconcile.Topic{ssh},
	func(ctx context.Context) error {
		return restartSSH(ctx)
	})

func main() {
	if _, err := reconcile.RunNode(context.Background(), sshdHardening); err != nil {
		log.Fatal(err)
	}
}
```

`sshdHardening` probes the remote config; if it differs it writes the hardening
file and notifies `ssh`; the `restart-ssh` effect runs once when `RunNode`
returns. In a dry run (`dryrun.With`) the probe still runs but `Apply`
and effects do not.

## Using with Gabric

Register the global middleware so every task in an invocation shares one run:

```go
var _ = gabric.Use(gabric.WithReconcile)
```

Modules use `gabric.TargetOps(ctx)`. `gabric.Main` and `gabric.Run` default to a
remote target; use `WithLocalTarget` or `NewTargetContext` for a local target, or
`LocalRemoteTarget` for remote-style commands on the local machine. See
[Modules](modules.md#target-operations) and the
[built-in target comparison](../api.md#target-operations).

A run can span several hosts when each host uses distinct nodes. The unsafe case
is evaluating the same node or node graph with different host contexts in one
run: node state is memoized by node identity, so one host's result can be reused
for another. If one node task is reused across hosts, give each call a fresh
run, either by wrapping the `NodeTask` with `WithReconcile` or by calling
`reconcile.Run` explicitly. Separate per-host graphs, including graphs returned
by a factory, can share a run, including in parallel.

Then a task can be built directly from a node. `NodeTask` evaluates the node and
flushes its effects at the end of the task:

```go
var Bootstrap = gabric.With(gabric.WithRemoteSudo(&gabric.Sudo{})).NodeTask(bootstrapNode)
var All = gabric.Task("all", func(ctx context.Context) { Bootstrap(ctx) })
```

Use `WithReport` to log a summary of every resource (`N ok, N changed,
N would-change, N failed`; see [api.md](api.md#report)) when the tasks finish.
It does not open a run, so it can be global on its own with isolated runs:

```go
var _ = gabric.Use(gabric.WithReport)
```

Combine it with `WithReconcile` when the tasks should share one run:

```go
var _ = gabric.Use(gabric.WithReport, gabric.WithReconcile)
```

`WithReconcileReport` is shorthand for that combination. The report includes
isolated runs started by tasks as well as the outer run.

See [architecture.md](architecture.md) for the execution model and
[api.md](api.md) for the builder reference.

## Worked example

[`examples/homelab`](../../examples/homelab) is a complete,
buildable example: a generic personal server plus WireGuard VPN. It combines
per-service `Group` covers, the `pkg`/`File`/`TemplateData`/`docker.Compose`/`git`/`ufw`
modules, topics and effects, a shared run and combined report via
`WithReconcileReport` (or `WithReport` + `WithReconcile`), age secrets, and
`-dry-run` mode. It mirrors the structure of a real deployment repo and is an
end-to-end companion to this document and the [cookbook](cookbook.md).

## Relationship to other tools

Reconcile's model maps closely to configuration-management and IaC tools, but in
ordinary Go and without a state file.

- **Fabric**: tasks as ordinary functions over a context (this is Gabric's
  base). Reconcile adds idempotent desired state and change reactions, which
  Fabric lacks.
- **Ansible**: resources plus `notify` handlers. A `Topic`/`Effect` is the same
  deferred, once-per-change reaction, but typed and subscribed in Go instead of
  string-named handlers. `dryrun.With` corresponds to `-dry-run`. No YAML,
  roles, or inventory.
- **Chef**: resources with `notifies`, `remote_file` checksums, and data bags.
  The resource builder maps to a Chef resource, `Effect` to `notifies`, and
  **data nodes** to computed attributes or data bags. A module like `httpfile.File`
  mirrors `remote_file` (checksum-gated).
- **Terraform**: the closest. `resource` vs `data` maps to Reconcile's
  **resource vs data nodes**, reference-based dependencies map to implicit
  `Value` reads, and `plan`/`apply` map to dry run/`Run`. Unlike Terraform,
  Reconcile has no state file. It is stateless and probes the actual system on
  every run.

Reconcile stays Go-native and in-process: typed nodes and effects, and no DSL,
agent, or state file.
