// Package reconcile is a lazy, topic-driven engine for idempotent desired-state
// work.
//
// # Model
//
// Nodes are lazy, memoized, idempotent units of work that yield a typed value.
// Calling Node.Value triggers a node's evaluation, returns its value, and
// records a dependency edge; dependencies are implicit via Value calls, so the
// graph is discovered at evaluation time and may differ from run to run. A
// node's action must be idempotent (rerunning it changes nothing), so
// rerunning the whole graph is safe.
//
// A Group is a container: it runs its members on every read, has no value, is
// not memoized, and records no dependencies. Invalidation flows upstream only
// (to dependents); a group is not in that graph, so to reset a group (or
// anything else) start a new Run.
//
// Topics are valueless change signals. A node that changes calls Topic.Notify;
// an Effect subscribes to one or more topics and runs at Flush if any of them
// is dirty.
//
// # Runs
//
// Run opens an isolated reconciliation run bound to the context, evaluates fn,
// and flushes its effects when fn returns. All state lives in the context and
// nodes are stateless, so independent goroutines (for example one run per host)
// share nothing, and a nested Run starts fresh (use InRun to test whether a run
// is active). The per-topic flush bound comes from the context (see
// WithMaxTopicFlushes).
//
// A single run can also be evaluated concurrently: Node.Value and Topic.Notify
// are safe to call from different node evaluations, a value node is evaluated
// at most once per run (concurrent readers wait for and share the single
// result; groups run on every read), and a dependency cycle spanning
// goroutines is reported as ErrCycle, not a deadlock.
// Flush and Invalidate may be called from any goroutine (overlapping calls are
// serialized; a flush cycle is ErrCycle). Misusing one evaluation concurrently
// returns ErrConcurrentUse: invalidating a node (or a dependent) that is being
// evaluated, or calling Value/Flush from goroutines a node's fn spawns.
//
// # Dry run
//
// dryrun.With marks a context as a dry run. In a dry run node probes still run,
// Do is a no-op and effects do not run, so the graph reports what it would do
// without changing the system.
//
// # Idempotency
//
// Reconcile does not enforce idempotency; node implementations must ensure that
// rerunning the graph changes nothing (mirroring Ansible-style idempotent
// modules). Non-idempotent, one-shot work does not belong in nodes.
package reconcile
