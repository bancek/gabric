# Reconcile architecture

How the `reconcile` engine works. See [README](README.md) for the model and
quick start, and [api.md](api.md) for the API reference.

Module builders live in `reconcile/modules` and take values from the standalone
`gabric/input` package. Inputs are not part of the engine: `input` imports only
the standard library and is usable outside Reconcile. Reconcile is independent
of modules, files, commands, services, and firewall rules.

## Nodes

```go
func NewNode[T any](name string, fn func(ctx context.Context) (T, error)) *Node[T]
func (n *Node[T]) Name() string
func (n *Node[T]) Value(ctx context.Context) (T, error)
func (n *Node[T]) Clone() *Node[T]
```

A node is created with `NewNode`, or with the [builder](api.md), and read by
calling `Node.Value`, which returns its value of type `T`:

- **Lazy and memoized.** A value node's function runs the first time `Value` is
  called in a run; the value is reused for the rest of that run. A **group** is
  the exception: it is a container that runs its members on every read, so its
  nodes (memoized value nodes) are cheap to re-read.
- **Dependencies are implicit and dynamic.** A value node reads the nodes it
  needs (by calling their `Value`) while it runs, so the graph is discovered at
  evaluation time and may differ from run to run (e.g. an `if` that reads a node
  only in some cases). There is no dependency list to keep in sync. Group reads
  are not dependencies: they are not recorded in the graph.
- **Cycles are errors.** If a node transitively reads itself, `Value` returns a
  `dependency cycle` error.
- **Errors are wrapped with the node name** (`reconcile: node "x": …`).
- **A shared read belongs in its own node.** Because evaluation is memoized per
  run, a value needed by both a resource's `Check` and `Apply` (or expensive or
  side-effecting to compute) should be a **data** node read with `Value`, so it
  is evaluated once per run rather than once per read. See
  [api.md](api.md#data).
- **A clone is a distinct identity.** `Node.Clone` returns a node with the same
  name and function but its own memoized value, edges and invalidation. This is
  the basis of the
  [probe/observed pattern](cookbook.md#invalidate-a-read-your-own-mutation-changed).
- **Node middleware scopes evaluation.** `Node.With` returns a clone with
  context middleware around the node's function. It applies to the node's whole
  evaluation, including dependencies, and must call `next` exactly once. Node
  state is keyed by identity, not context.

A node must be **idempotent** in the _system_ sense: running it again must change
nothing (probe, then apply only on drift). The engine does not enforce this; it
is what makes rerunning the whole graph safe.

### Invalidate

```go
func (n *Node[T]) Invalidate(ctx context.Context) error
```

`Node.Invalidate` clears the node's memoized value and every node that
(transitively) depends on it, so they re-evaluate on their next `Value`.
Dependencies declared with `Needs`/`CheckNeeds` are **not** touched. Use it when
something outside the graph makes a node's value stale. Dependency edges are
tracked as nodes are read; invalidation leaves them in place (a node's outgoing
edges are reconciled when it re-evaluates).

Invalidation only flows **upstream** (to dependents). A group is a container,
not a value node: it is not memoized and is not in the dependency graph, so
invalidating a part never cascades through a group, and invalidating a group does
not reset its parts. To reset a group (or anything else), start a new `Run`.

## Topics and effects

```go
func NewTopic(name string) *Topic
func (t *Topic) Name() string
func (t *Topic) Notify(ctx context.Context) error

func NewEffect(name string, topics []*Topic, fn func(ctx context.Context) error) *Effect
func (e *Effect) Name() string
func (e *Effect) With(middlewares ...middleware.Middleware) *Effect
```

A topic is a **valueless** signal. A node that changed calls `Topic.Notify`;
repeated notifications before a flush collapse into one.

An effect subscribes to one or more topics and runs **once per notification**
(not once per run): if a topic is flushed and later notified again in the same
run, the effect runs again.

Effects run at `Flush`, which is:

- the end of `Run` (automatic), and
- `Flush(ctx)`: call it to run pending effects early (for example before a step
  that needs the service reloaded), and again later if needed.

`Flush` drains dirty topics in first-notified order, following cascades: an
effect may `Topic.Notify` another topic, whose effects run in the same flush.
`DefaultMaxTopicFlushes` (`100`) bounds how many times a single topic may be
flushed within one `Flush`; exceeding it is a flush cycle and an error. Set a
different bound on the context with `reconcile.WithMaxTopicFlushes(ctx, n)`
(for example in a main middleware); runs built from that context inherit it.

`Effect.With` adds context middleware to an effect at flush. It returns a new
effect and replaces the old effect's topic subscriptions. The middleware may
call `next` zero or one time: zero skips the effect.

## Runs, contexts and dry run

```go
func Run(ctx context.Context, fn func(ctx context.Context) error) error
func RunNode[T any](ctx context.Context, n *Node[T]) (T, error)
func RunNodeVoid(ctx context.Context, n *Node[Void]) error
func Flush(ctx context.Context) error
func InRun(ctx context.Context) bool
func WithMaxTopicFlushes(ctx context.Context, n int) context.Context
func MaxTopicFlushes(ctx context.Context) int
```

`Run` opens an isolated reconciliation scope on the context, calls `fn`, and
flushes its effects when `fn` returns. A nested `Run` starts with fresh state.
Use it to reconcile a different target from within an active run. `InRun(ctx)`
reports whether a run is active. `RunNode` is the one-node shorthand;
`RunNodeVoid` is its `Void` form. Both inherit dry-run mode and the flush bound
from the context.

- **All state lives in the context.** The run's memoized values, dependency edges
  and dirty topics live in the context `Run` derives; nodes are stateless.
  Independent goroutines (one `Run` each) share nothing, so you can reconcile
  several hosts concurrently.
- **A single run is evaluated concurrently.** `Node.Value` and `Topic.Notify` are
  safe to call from different goroutines (for example `gabric.Parallel` tasks
  sharing one run under `WithReconcile`). A value node is evaluated at most once
  per run: concurrent readers wait for and share the single result, and errors
  (and panics) are memoized like values. Groups are not memoized and run on
  every read. A dependency cycle that spans goroutines is reported as `ErrCycle`
  instead of deadlocking.
- **`Flush` and `Invalidate` may be called from any goroutine.** Overlapping
  calls are serialized, so they never execute concurrently, including `Flush`
  from a node's `fn`. A flush that would wait on a node whose evaluation is
  itself flushing is reported as `ErrCycle`, not a deadlock. (Sequential `Flush`
  calls are fine and drain whatever is pending.) Invalidating a node (or a
  dependent) that is being evaluated, or calling `Value`/`Flush` concurrently
  from goroutines a node's `fn` spawns, returns `ErrConcurrentUse`.
- **Errors.** Using an operation outside a `Run` returns `ErrNoRun`; dependency
  cycles return `ErrCycle`; concurrent misuse of one evaluation returns
  `ErrConcurrentUse`; exceeding a run's flush bound (`DefaultMaxTopicFlushes`,
  or `WithMaxTopicFlushes`) returns `ErrMaxTopicFlushes`.

### Dry run

Mark a context with `dryrun.With` before `Run` (or check with
`dryrun.Enabled`). In a dry run node functions still run (so they probe), but
`Do` does not call its mutation, and effects do not run. The `-dry-run` runner
flag does this automatically for every task.

`Do` is the single place a node's mutation happens:

```go
func Do(ctx context.Context, fn func() error) error
```

Write nodes as _probe outside `Do`, mutate inside `Do`_:

```go
if !installed(ctx) {
	return version, reconcile.Do(ctx, func() error { return install(ctx) })
}
```

so a dry run reports what would change without changing anything.

### Reporting

Every resource records its outcome in the run as it evaluates: `ReportOK` (in
state), `ReportChanged` (apply ran), `ReportWouldChange` (drift in a dry run) or
`ReportFailed` (its evaluation errored). `Report(ctx)` returns them in
first-record order inside the run, since the state lives in `runState` and is
per-run like everything else. `FormatReport` renders a summary.

Only resources are recorded. A data node's error is not a separate entry; it
aborts the run and surfaces in the dependent resource's `Err` (the chain already
names the node). Effects are not recorded either. Because evaluation aborts on
error, a sequential run's failed entry is normally the last one.

Reports can also be aggregated. If the context carries a `ReportCollector`
(`WithReportCollector`), `Run` appends its results when it finishes, including
on error or panic, and nested isolated runs contribute to the same collector.
`ReportCollector.Results` returns a copy in completion order. A parent task can
therefore report resources from graphs evaluated by `RunNode` or nested `Run`s.
Gabric's `WithReport` installs a collector and logs it after the task run;
`WithReconcileReport` composes it with `WithReconcile`. A collector installed on a
derived context scopes the report to that subtree, so an isolated run can report
separately.

## Gabric integration

Root `gabric` imports `reconcile` to provide `WithReconcile`, a global middleware
that runs every dispatched task inside one run (`WithReconcile` is literally
`reconcile.Run(ctx, next)`; register it with
`gabric.Use(gabric.WithReconcile)`). See
[README](README.md#using-with-gabric) for usage. Internals:

- Node state is shared across the tasks of one invocation, and effects flush
  **once** when all tasks finish; a task can `reconcile.Flush(ctx)` earlier. On a
  task error the flush is skipped.
- Node state is keyed by node identity, not by host. A run can span several
  hosts when each host uses distinct nodes; evaluating the same node or node graph
  with different host contexts in one run can reuse one host's result for
  another. If one node task is reused across hosts, give each call a fresh run
  (for example a `NodeTask` wrapped with `WithReconcile`, or an explicit
  `reconcile.Run`; nested `Run`s do not share state). Distinct per-host graphs,
  including graphs returned by a factory, can share a run.
  `reconcile.InRun(ctx)` reports whether a run is active.
- A single run is evaluated concurrently, so `gabric.Parallel` is safe: the
  tasks dispatched in one invocation share one run, each node is evaluated at
  most once, and a cross-goroutine cycle is reported as `ErrCycle`. Across hosts,
  use distinct nodes, or a fresh run when one node task is reused.
- Main middleware runs before global middleware, so it can configure the run:
  set the flush bound (`reconcile.WithMaxTopicFlushes`) or add application
  values. The `-dry-run` flag already marks the context via `dryrun.With`.
- `Report(ctx)` is per-run; a `ReportCollector` in the context aggregates results
  from it and any nested isolated runs. `gabric.WithReport` installs a collector
  and logs the combined report after the tasks finish, including on error or
  panic; it does not open a run. `WithReconcileReport` composes it with
  `WithReconcile` for a shared run.

## Design notes

- **Implicit dependencies, no graph object.** A node reads what it needs while
  it runs, so dependencies are recorded as they are read. There is no dependency
  list to maintain and the graph can change per run. The trade-off is that the
  graph is only observable by evaluating it.
- **Nodes are for system-idempotent work.** A node is a probe plus an apply on
  drift, so it can be re-read (and the graph re-run) safely. One-shot or
  otherwise non-idempotent work does not belong in a node; put it in a Gabric
  task (ordinary Go) instead.
- **Topics are valueless.** Idempotent probes already detect _whether_ something
  changed, so a signal only needs to say _what_ changed, not carry a value.
  Effects that need data read it from a node with `Node.Value`. (A valued signal
  could be added later; the common case does not need it.)
- **Effects, not handlers.** An effect is the Ansible handler / Chef `notify`
  idea: a deferred, deduped reaction to a change, subscribed by topic rather than
  registered in a global string map. Effects run once per notification.
- **Error-based, no panics.** Every operation returns an error; there is no
  panic/`Err` pair. (A panic inside a node's `fn` is recovered with `PanicError`
  into a memoized error, so `errors.Is` still works.)
- **State changes invalidate aliased reads explicitly.** The engine does not
  track when two nodes touch the same external state, so a resource that mutates
  state can declare `.ApplyInvalidates(readNodes)`: those reads (and their
  dependents) clear after `Apply` and re-probe on their next `Value`. It must not invalidate a
  node it reads, directly or indirectly (rejected with `ErrConcurrentUse`); when
  the mutator must read the same state, split it into a probe it reads and an
  observed node it invalidates. This is the
  [probe/observed pattern](cookbook.md#invalidate-a-read-your-own-mutation-changed).
- **Groups are containers, not value nodes.** A group runs its members on every
  read, is not memoized, and records no dependencies. A group does not own its
  members: invalidating a group does not reset its parts, and invalidating a part
  never cascades up through a group. To reset a group, start a new `Run` (fresh
  state).
  This keeps invalidations from ever reaching an in-flight ancestor.
- **Concurrent evaluation of one run.** State is in the context and nodes are
  stateless. A run can be evaluated from several goroutines: each value node is
  evaluated at most once (concurrent readers share the result; groups run per
  read), a cross-goroutine cycle is detected via a wait-for graph and
  returned as an error, and `Flush`/`Invalidate` calls are serialized (a flush
  cycle is an error, not a deadlock). Concurrency across whole runs (one per
  host/task) also remains the approach for independent targets.
- **Bound per topic.** `DefaultMaxTopicFlushes` (overridable per context with
  `WithMaxTopicFlushes`) bounds _each_ topic and each flush rather than total
  work, so large legitimate runs do not trip the guard while cycles are still
  caught: topics are finite and each is bounded, so every flush terminates.
- **Type-erased node identity.** `Node[T]` is generic but the run's state is
  heterogeneous, so each node carries a small non-generic base (its name and
  address) used as its identity throughout the engine.
