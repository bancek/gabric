# Reconcile API

The `reconcile` API: the engine primitives and the node builders. For how the
engine works see [architecture.md](architecture.md); for the model and quick
start see [README.md](README.md). Module examples use the `input` package; see
[inputs.md](inputs.md).

## Engine primitives

| Symbol                                                             | Purpose                                              |
| ------------------------------------------------------------------ | ---------------------------------------------------- |
| `NewNode[T](name, fn)`, `Node[T].Name()`                           | Create / name a node.                                |
| `n.Value(ctx)`                                                     | Evaluate (once per run) and return the node's value. |
| `n.Evaluate(ctx)`                                                  | Evaluate the node, discarding its value.             |
| `n.Invalidate(ctx)`                                                | Clear a node and its dependents.                     |
| `n.Clone()`                                                        | Duplicate a node with a distinct identity.           |
| `n.With(middlewares...)`                                           | Add context middleware to a node.                    |
| `NodeRef`                                                          | A node or group cover accepted as a dependency.      |
| `Evaluate(ctx, refs...)`                                           | Evaluate node refs, discarding their values.         |
| `Map[T, U](node, fn)`                                              | Transform a node's value (a data node).              |
| `Filter[T](node, cond)`, `IfChanged[T](node, cond)`                | Gate a node's evaluation (data nodes).               |
| `DynamicResource[T](name, build)`                                  | Build a resource implementation at evaluation time.  |
| `ValueMatches[T](node, fn)`, `ValueEquals[T](node, want)`          | Build a Check/cond from one node's value.            |
| `NodesEqual[T](a, b)`                                              | Build a Check/cond comparing two nodes' values.      |
| `Do(ctx, fn)`                                                      | Dry-run-guarded mutation.                            |
| `NewTopic(name)`, `Topic.Name()`                                   | Create / name a topic.                               |
| `t.Notify(ctx)`                                                    | Mark a topic dirty (deduped).                        |
| `NewEffect(name, topics, fn)`, `Effect.Name()`                     | Subscribe an effect to topics.                       |
| `e.With(middlewares...)`                                           | Add context middleware to an effect.                 |
| `Run(ctx, fn)`                                                     | Open an isolated run; flush at the end.              |
| `RunNode[T](ctx, n)`, `RunNodeVoid(ctx, n)`                        | Run one node in an isolated run.                     |
| `Flush(ctx)`                                                       | Run pending effects now.                             |
| `InRun(ctx)`                                                       | Report whether a run is active.                      |
| `Report(ctx)`, `FormatReport(results)`                             | Resource outcomes and a rendered summary.            |
| `ReportCollector`, `WithReportCollector`, `GetReportCollector`     | Aggregate results across runs.                       |
| `WithMaxTopicFlushes`, `MaxTopicFlushes`, `DefaultMaxTopicFlushes` | Per-topic, per-flush bound (default `100`).          |
| `dryrun.With`, `dryrun.Enabled`                                    | Dry-run marker shared with Gabric and tasks.         |
| `PanicError(r)`                                                    | Convert a recovered panic into an error.             |
| `ErrNoRun`, `ErrCycle`, `ErrConcurrentUse`, `ErrMaxTopicFlushes`   | Sentinel errors.                                     |

### `RunNode`, `RunNodeVoid`

```go
func RunNode[T any](ctx context.Context, n *Node[T]) (T, error)
func RunNodeVoid(ctx context.Context, n *Node[Void]) error
```

`RunNode` opens an isolated run, evaluates `n`, flushes its effects and returns
the node's value. It is the one-node shorthand for `Run`, and works inside a
node's `fn` because the nested run does not share the caller's evaluation frame.
The run inherits `dryrun.Enabled(ctx)` and the per-topic flush bound from `ctx`.
`RunNodeVoid` is the form for group and other `Void` nodes.

```go
config, err := reconcile.RunNode(ctx, appConfig)
if err != nil {
	return err
}
```

### `Clone`

```go
func (n *Node[T]) Clone() *Node[T]
```

`Clone` returns an independent node with the same name and function as `n` but a
**distinct identity**, so it is memoized, invalidated and depended on separately.
It is the primitive behind the
[probe/observed pattern](cookbook.md#invalidate-a-read-your-own-mutation-changed):
read the original in a resource's `Check` and invalidate the clone after `Apply`,
rather than constructing the read twice.

```go
probe := reconcile.Data("app-version", readVersion).Node()
observed := probe.Clone() // same read, separate identity
```

The clone shares `n`'s function and middleware. Cloning a resource duplicates
its mutation, so `Clone` is intended for read (Data) nodes. It works on any node,
including one returned by a module helper or `Map`/`Filter`.

### `Map`

```go
func Map[T, U any](n *Node[T], fn func(ctx context.Context, v T) (U, error)) *Node[U]
```

`Map` returns a **data node** whose value is `fn` applied to `n`'s value. It runs
outside `Do` (so it runs in a dry run) and is memoized once per run. Use it to
turn a read into what a consumer needs, for example a `map[string]any` for a
template's data:

```go
revision := reconcile.Data("revision", readRevision).Node()

data := reconcile.Map[string, any](revision, func(ctx context.Context, rev string) (any, error) {
	return map[string]any{"Revision": rev}, nil
})
```

Errors from `n` and from `fn` propagate. Invalidating the `Map` recomputes it
from `n`'s current value; invalidate `n` (or start a new `Run`) to force a
re-probe.

### `Filter` / `IfChanged`

```go
func Filter[T any](source *Node[T], cond func(ctx context.Context) (bool, error)) *Node[T]
func IfChanged[T any](source *Node[T], cond *Node[bool]) *Node[T]
```

`Filter` returns a data node whose value is `source`'s value when `cond` returns
true, and the zero value otherwise. When `cond` is false the source is **not**
evaluated. `IfChanged` is `Filter` with a node condition. Invalidating the filter
does not invalidate the source; invalidate the source (or start a new `Run`) to
force a re-probe:

```go
downloadMaybe := /* a resource that downloads */

download := reconcile.Filter(downloadMaybe, reconcile.ValueMatches(currentRev, func(cur string) bool {
	return cur != rev // only run the download when the revision changed
}))

writeRev := reconcile.IfChanged(writeRevMaybe, download) // only after a download
```

### `ValueMatches` / `ValueEquals` / `NodesEqual`

```go
func ValueMatches[T any](n *Node[T], fn func(T) bool) func(ctx context.Context) (bool, error)
func ValueEquals[T comparable](n *Node[T], want T) func(ctx context.Context) (bool, error)
func NodesEqual[T comparable](a, b *Node[T]) func(ctx context.Context) (bool, error)
```

These build a `Check` (for `Resource`) or condition (for `Filter`) from node
value(s), so the read and its error handling are not repeated at each call site:

```go
reconcile.Resource("app",
	reconcile.ValueMatches(version, func(v string) bool { return v == desired }),
	upgrade)

reconcile.Filter(download, reconcile.ValueEquals(rev, deployedRev))
reconcile.ResourceBool("reset", reconcile.NodesEqual(local, remote), resetHard)
```

`ValueEquals` is `ValueMatches` with `==`. The returned func is **not** a node: it
reads the (memoized) node on each call, and a `ValueMatches` predicate cannot
propagate an error, so a condition that reads more than one node (or needs an
error) is written as an ordinary closure. `IfChanged` takes a `*Node[bool]`, not
one of these.

### `Report`

```go
func Report(ctx context.Context) []ReportResult
func FormatReport(results []ReportResult) string

type ReportCollector struct { /* ... */ }

func WithReportCollector(ctx context.Context, c *ReportCollector) context.Context
func GetReportCollector(ctx context.Context) *ReportCollector
func HasReportCollector(ctx context.Context) bool
func (c *ReportCollector) Collect(results []ReportResult)
func (c *ReportCollector) Results() []ReportResult
```

Every resource node records its outcome in the run's report as it evaluates, and
`Report(ctx)` reads it back **inside the run** (it returns `nil` outside one):

```go
type ReportStatus int // ReportOK | ReportChanged | ReportWouldChange | ReportFailed

type ReportResult struct {
	Name   string
	Status ReportStatus
	Err    error // set only for ReportFailed
}
```

- `ReportOK`: the probe found the resource already satisfied.
- `ReportChanged`: `Apply` ran and succeeded.
- `ReportWouldChange`: the resource drifted in a dry run (the apply was skipped).
- `ReportFailed`: the resource's evaluation returned an error (`Err`). Because
  evaluation aborts on error, a sequential run's failed entry is normally the
  last one; a concurrently evaluated run may record several.

Only resources are recorded; data and group nodes are not (a data node's error
surfaces in the dependent resource's `Err`). `FormatReport` renders one line per
result (`  <status> <name> (err)`) and a summary
(`N ok, N changed, N would-change, N failed`).

`Report` is per-run. To aggregate results across runs, put a `ReportCollector` in
the context before starting them. Every `Run` appends its results when it
finishes, including on error or panic, so isolated `Run`/`RunNode` calls can feed
one combined report:

```go
collector := &reconcile.ReportCollector{}
ctx = reconcile.WithReportCollector(ctx, collector)

// ... run one or more isolated graphs ...

results := collector.Results()
```

`Results` is a copy, and the collector is safe for concurrent runs. Gabric's
`WithReport` installs or reuses a collector and logs the combined report;
`WithReconcileReport` is its shared-run shorthand. `gabric.LogReport(ctx,
results)` logs a collected snapshot directly.

A collector installed on a derived context scopes the report to that subtree.
Wrapping an isolated run with a fresh collector keeps its results separate from
the outer report:

```go
isolated := &reconcile.ReportCollector{}
isolatedCtx := reconcile.WithReportCollector(ctx, isolated)

_, err := reconcile.RunNode(isolatedCtx, appGraph(host))
// isolated.Results() contains only this run's results
```

`WithReport` reuses an existing collector rather than creating a child, so use an
explicit collector (and `FormatReport` if needed) for an isolated logged report.

## Node builders

A node is built with one of three constructors: `Data`, `Resource`, or `Group`.
It is finalized with `Node`. Each builder is immutable: every method returns a
new builder, so a base can be shared and safely extended (like `gabric.With`).

```go
// Data: a read
func Data[T any](name string, fn func(ctx context.Context) (T, error)) *DataBuilder[T]
func (b *DataBuilder[T]) Needs(refs ...NodeRef) *DataBuilder[T]
func (b *DataBuilder[T]) Node() *Node[T]

// Resource: a probe (check) and a mutation (apply)
func Resource[T any](name string, check func(ctx context.Context) (bool, error), apply func(ctx context.Context) (T, error)) *ResourceBuilder[T]
func ResourceBool(name string, check func(ctx context.Context) (bool, error), apply func(ctx context.Context) error) *ResourceBuilder[bool]
func (b *ResourceBuilder[T]) CheckNeeds(refs ...NodeRef) *ResourceBuilder[T]
func (b *ResourceBuilder[T]) ApplyNeeds(refs ...NodeRef) *ResourceBuilder[T]
func (b *ResourceBuilder[T]) ApplyInvalidates(refs ...NodeRef) *ResourceBuilder[T]
func (b *ResourceBuilder[T]) Notify(topics ...*Topic) *ResourceBuilder[T]
func (b *ResourceBuilder[T]) Node() *Node[T]

// Group: a container that runs its members, is not memoized, and yields Void
func Group(name string) *GroupBuilder
func (b *GroupBuilder) Contains(refs ...NodeRef) *GroupBuilder
func (b *GroupBuilder) Node() *Node[Void]
```

`Void` (`type Void = struct{}`) is the value type for nodes that produce no
meaningful result.

`NodeRef` is a sealed interface satisfied by `*Node[T]` and by types embedding
one, such as group covers. Dependency methods accept `NodeRef`, so a group cover
can be passed directly:

```go
reconcile.Group("user").
	Contains(deployUser, deployKeys)
```

`NodeRef.Evaluate(ctx)` evaluates a node or group cover and discards its value.
`Evaluate(ctx, refs...)` does the same for several refs in order:

```go
err := deployUser.Evaluate(ctx)
err = reconcile.Evaluate(ctx, deployUser, keys)
```

### `Data`

```go
reconcile.Data(name, func(ctx context.Context) (T, error) { ... })
```

A **data** node: the function runs unconditionally (it is _not_ wrapped in `Do`),
so it runs in a dry run and its result is the node's value. Use it for reads
(`GitRevision`, host facts, secret plaintext).

A data node is also the per-run **memoization** unit: reading it with `Value`
evaluates it once, and every later reader in the same run shares the result. Use
one whenever a value is needed by both `Check` and `Apply` (a resource's `Apply`
is skipped in a dry run), or is expensive/side-effecting. Don't inline the read
in a `Check`/`Apply` closure, which would run it twice:

```go
var arch = reconcile.Data("host-arch", readArch).Node()

var conf = reconcile.Resource("conf",
	func(ctx context.Context) (bool, error) {
		a, err := arch.Value(ctx) // evaluated once per run
		if err != nil {
			return false, err
		}
		return remoteMatches(ctx, a), nil
	},
	func(ctx context.Context) (string, error) {
		a, _ := arch.Value(ctx) // shared: still the single evaluation
		return write(ctx, a)
	}).
	Node()
```

`File` content can be a data node, and `TemplateData` is a data node, for this
reason.

- **`Needs(...)`**: read dependencies before the data function; they are **not**
  invalidated with the node.

### `Resource`

```go
reconcile.Resource(name, check, apply)
reconcile.ResourceBool(name, check, applyErr)
```

A **resource** node: `check` is the drift probe (return `true` when the system
already matches the desired state), `apply` is the mutation, run inside `Do` (so a
dry run skips it) and only on drift. A nil `check` means always apply
(command-like). Its value is produced by `apply`, so it is **not** available in a
dry run. `apply` is required. Calling `Resource` without it panics.
`ResourceBool` is the common form for a `bool` "changed" value: its `apply` is
`func(ctx) error` and the node yields `true` when it applied (a satisfied
resource yields `false`).

```go
reconcile.ResourceBool("sshd-hardening",
	func(ctx context.Context) (bool, error) {
		return remoteFileMatches(ctx, sshdConf, hardening)
	},
	func(ctx context.Context) error {
		return writeRemoteFile(ctx, sshdConf, hardening, 0o644)
	})
```

- **`CheckNeeds(...)`**: read _before_ `Check` (unconditional). Use when the
  probe itself needs the dependency in place (e.g. the probe command only exists
  once a package is installed).
- **`ApplyNeeds(...)`**: read _in the Apply path_ (lazily: only when the node
  drifts, inside `Do`, so a dry run skips them). Use for mutation prerequisites;
  e.g. `pkg.Package(input.Static("curl")).Build().ApplyNeeds(pkg.Update().Build().Node())`
  runs the selected apt or apk update only when it will install.
- **`ApplyInvalidates(...)`**: clear these nodes **after `Apply`** runs (inside
  `Do`, before `Notify`), so aliased reads re-evaluate on their next `Value`.
- **`Notify(...)`**: mark topics dirty when the resource changes.

### `DynamicResource`

```go
func DynamicResource[T any](name string, build func(ctx context.Context) (*ResourceBuilder[T], error)) *ResourceBuilder[T]
```

`DynamicResource` builds a resource whose implementation builder is constructed
when the resource is evaluated. The returned builder owns the resource name and
report entry; the built builder contributes its check, apply, dependencies,
invalidations and notifications. Its builder name is not used. Middleware
belongs to the returned node: `DynamicResource(...).Node().With(...)` wraps the
facade evaluation.

`build` is read through a data node, so `Check` and `Apply` share one builder.
A built builder's own `CheckNeeds`, `ApplyNeeds`, `ApplyInvalidates` and
`Notify` run as part of the facade resource. `pkg.Package` uses this to select
`apt.Package` or `apk.Package`, and `service.Service` uses it to select
`systemd.Service`:

```go
pkgNode := reconcile.DynamicResource("pkg.package:curl",
	func(ctx context.Context) (*reconcile.ResourceBuilder[bool], error) {
		manager, err := pkg.ManagerData().Value(ctx)
		if err != nil {
			return nil, err
		}

		switch manager {
		case pkg.ManagerApt:
			return apt.Package(input.Static("curl")).Build(), nil
		case pkg.ManagerApk:
			return apk.Package(input.Static("curl")).Build(), nil
		default:
			return nil, fmt.Errorf("pkg: unsupported package manager")
		}
	})
```

`DynamicResource` does not create a group or evaluate the built builder as its
own node, so the report contains the facade name, not the implementation name.

### `Group`

```go
reconcile.Group(name).Contains(a, b)
```

A **group**: `Contains` lists the nodes to read, in order, every time the group
is read. It accepts nodes and group covers. The group is not memoized, produces
`Void`, and records no dependencies. Use it to group the parts of a task:

```go
repo := reconcile.Group("repo:" + dir).
	Contains(clone, fetch, reset).
	Node()

repo.Value(ctx) // runs the nodes (each is memoized)
```

A node that is itself a group runs its own nodes (nested groups).

Invalidation is upstream-only, so invalidating a node does not reset the group,
and `repo.Invalidate(ctx)` does not reset its parts. To reset a group, start a
new `Run` (or invalidate the specific value nodes). A group never owns its
nodes, so it can contain unexported probes safely.

### `ApplyInvalidates`

```go
reconcile.ResourceBool("ufw-rule:allow 22/tcp", checkDrift,
	func(ctx context.Context) error { return allow22(ctx) }).
	ApplyInvalidates(ufwStatus). // a rule change makes the cached status stale
	Node()
```

Records nodes to invalidate **after `apply` runs** (inside `Do`, before `Notify`),
so reads that alias the state this node mutates re-evaluate on their next `Value`.
The node that _changes_ the state declares what its change makes stale; the read
does not subscribe to anything. `ApplyInvalidates` requires an `apply` (always
present on a resource). A resource must **not** invalidate a node it reads,
directly or indirectly. The engine rejects that with `ErrConcurrentUse`
(invalidating a read the in-flight node depends on). When the mutator's own
`Check` must read the aliased state, split it into a probe the resource reads and
an observed node it invalidates. This is the
[probe/observed pattern](cookbook.md#invalidate-a-read-your-own-mutation-changed).

### `Notify`

```go
reconcile.ResourceBool("sshd-hardening", checkDrift, writeHardening).Notify(ssh)
```

Topics marked dirty when the node changes (i.e. when `apply` runs). Subscribed
effects run at the next `Flush` (the end of `Run`, or an explicit `Flush`). Only a
resource can `Notify`.

### `Node`

```go
.Node() *Node[T]
```

Returns the node. The builder's configuration is snapshotted, so further changes
to the builder do not affect the node.

## Node kinds

| kind     | constructor                 | runs                             | dry run    |
| -------- | --------------------------- | -------------------------------- | ---------- |
| resource | `Resource(n, check, apply)` | probe, then `Do(apply)` on drift | probe only |
| data     | `Data(n, fn)`               | the read, unconditionally        | read runs  |
| group    | `Group(n).Contains(...)`    | its members, on every read       | reads run  |

A resource's value is produced by `Apply`, so it is **not** available in a dry
run. Anything a `Check` needs must come from a data node or a `CheckNeeds` read.

## Middleware

### `Node.With`

```go
func (n *Node[T]) With(middlewares ...middleware.Middleware) *Node[T]
```

Returns a clone with context middleware appended. The middleware wraps the
node's whole evaluation, including `CheckNeeds`, `ApplyNeeds`, and dependencies,
so context changes such as sudo reach everything the node reads. It runs in dry
runs and must call `next` exactly once.

```go
var sudo = gabric.WithRemoteSudo(&gabric.Sudo{})

var ufwPkg = pkg.Package(input.Static("ufw")).
	Build().
	ApplyNeeds(aptUpdate).
	Node().
	With(sudo)
```

`With` returns a distinct node identity. Use the returned node in groups,
dependencies, and `ApplyInvalidates`. Node state is memoized by identity, not
context, so a shared dependency may already be cached from another context; wrap
the dependency itself if it must always use the middleware.

### `Effect.With`

```go
func (e *Effect) With(middlewares ...middleware.Middleware) *Effect
```

Returns a new effect with context middleware appended and replaces the old
effect's subscription in each topic. The middleware runs at flush and may call
`next` zero or one time: zero skips the effect. Effects do not run in dry-run
mode.

```go
var _ = systemd.RestartEffect("ssh", sshTopic).With(sudo)
```

The original effect is no longer subscribed; chain further `With` calls on the
returned effect. Call `With` while building the effect, before any run flushes.

## Example

```go
// Two producers notify the same topic; one effect restarts ssh once if either
// changes.
var ssh = reconcile.NewTopic("ssh")

var sshdHardening = reconcile.ResourceBool("sshd-hardening",
	func(ctx context.Context) (bool, error) {
		return remoteFileMatches(ctx, sshdConf, hardening)
	},
	func(ctx context.Context) error {
		return writeRemoteFile(ctx, sshdConf, hardening, 0o644)
	}).
	Notify(ssh).
	Node()

var sshdBanner = reconcile.ResourceBool("sshd-banner",
	func(ctx context.Context) (bool, error) {
		return remoteFileMatches(ctx, bannerConf, banner)
	},
	func(ctx context.Context) error {
		return writeRemoteFile(ctx, bannerConf, banner, 0o644)
	}).
	Notify(ssh).
	Node()

var _ = reconcile.NewEffect("restart-ssh", []*reconcile.Topic{ssh},
	func(ctx context.Context) error { return restartSSH(ctx) })
```
