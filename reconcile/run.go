package reconcile

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/bancek/gabric/dryrun"
)

// Sentinel errors returned by the engine.
var (
	// ErrNoRun is returned when a reconciliation operation is used outside a Run.
	ErrNoRun = errors.New("no active run")

	// ErrCycle is returned (wrapped with the node or flush involved) when a
	// dependency cycle is detected, including a cycle spanning goroutines.
	ErrCycle = errors.New("dependency cycle")

	// ErrConcurrentUse is returned when one node evaluation is used concurrently,
	// for example when a node's fn calls Value/Flush from a goroutine it spawns,
	// or when a node that is being evaluated is invalidated.
	ErrConcurrentUse = errors.New("concurrent use of a single evaluation")

	// ErrMaxTopicFlushes is returned when a topic is flushed more times than the
	// run's flush bound within one Flush, which indicates a flush cycle.
	ErrMaxTopicFlushes = errors.New("topic flush limit exceeded")
)

// DefaultMaxTopicFlushes bounds how many times a single topic may be flushed
// within one Flush call (a runaway-effect / flush-cycle guard). Because topics
// are finite and each is bounded, every Flush terminates. Override it with
// WithMaxTopicFlushes.
const DefaultMaxTopicFlushes = 100

const maxTopicFlushesKey contextKey[int] = "reconcile.maxTopicFlushes"

// WithMaxTopicFlushes returns a context whose runs use n as the per-topic,
// per-flush bound. Values below 1 are clamped to 1.
func WithMaxTopicFlushes(ctx context.Context, n int) context.Context {
	if n < 1 {
		n = 1
	}

	return withContextValue(ctx, maxTopicFlushesKey, n)
}

// MaxTopicFlushes returns the flush bound for ctx, or DefaultMaxTopicFlushes
// when unset.
func MaxTopicFlushes(ctx context.Context) int {
	if n, ok := contextValue(ctx, maxTopicFlushesKey); ok {
		return n
	}

	return DefaultMaxTopicFlushes
}

const runKey contextKey[*runState] = "reconcile.run"

// nodeState is the per-node evaluation slot for a run. It is created when the
// node starts evaluating and is published (done closed) once it finishes, so
// concurrent readers can wait for and share a single evaluation.
type nodeState struct {
	done chan struct{}
	val  any
	err  error
}

// completed reports whether the evaluation has published its result.
func (st *nodeState) completed() bool {
	select {
	case <-st.done:
		return true
	default:
		return false
	}
}

// flushState is the active flush's coordination state: the node that owns it (or
// a per-flush sentinel for a caller with no evaluation frame), a signal closed
// when it finishes, and its error (read by joiners after done).
type flushState struct {
	flusher *node
	done    chan struct{}
	err     error
}

// runState is all per-run state. It lives in the context, so a run is
// independent of every other run and nodes stay stateless. Every field except
// dryRun (which is immutable after Run starts) is guarded by mu.
type runState struct {
	mu              sync.Mutex
	dryRun          bool
	maxTopicFlushes int
	states          map[*node]*nodeState // memoized or in-flight evaluations
	waitsOn         map[*node]*node      // evaluating node -> in-flight node it blocks on
	flush           *flushState          // active flush, nil when none
	dependsOn       map[*node]map[*node]bool
	dependents      map[*node]map[*node]bool
	dirty           map[*Topic]bool
	dirtyOrder      []*Topic
	results         map[*node]*ReportResult // resource outcomes, by node
	resultOrder     []*node                 // first-record order of results
}

func newRunState(dryRun bool, maxTopicFlushes int) *runState {
	return &runState{
		dryRun:          dryRun,
		maxTopicFlushes: maxTopicFlushes,
		states:          map[*node]*nodeState{},
		waitsOn:         map[*node]*node{},
		dependsOn:       map[*node]map[*node]bool{},
		dependents:      map[*node]map[*node]bool{},
		dirty:           map[*Topic]bool{},
		results:         map[*node]*ReportResult{},
	}
}

// recordResult records a resource node's outcome for the run. An existing entry
// is overwritten in place, so re-evaluation updates it without duplicating; the
// node keeps its first-record position.
func (rs *runState) recordResult(id *node, status ReportStatus, err error) {
	rs.mu.Lock()
	defer rs.mu.Unlock()

	if _, ok := rs.results[id]; !ok {
		rs.resultOrder = append(rs.resultOrder, id)
	}

	rs.results[id] = &ReportResult{Name: id.name, Status: status, Err: err}
}

func withRun(ctx context.Context, rs *runState) context.Context {
	return withContextValue(ctx, runKey, rs)
}

func runFrom(ctx context.Context) (*runState, bool) {
	return contextValue(ctx, runKey)
}

// addEdge records that reader depends on source, and its inverse.
func (rs *runState) addEdge(reader, source *node) {
	if rs.dependsOn[reader] == nil {
		rs.dependsOn[reader] = map[*node]bool{}
	}
	if rs.dependents[source] == nil {
		rs.dependents[source] = map[*node]bool{}
	}

	rs.dependsOn[reader][source] = true
	rs.dependents[source][reader] = true
}

// clearEdgesFrom drops the outgoing edges of reader before it re-evaluates.
func (rs *runState) clearEdgesFrom(reader *node) {
	for source := range rs.dependsOn[reader] {
		delete(rs.dependents[source], reader)
	}

	delete(rs.dependsOn, reader)
}

// wouldCycle reports whether blocking on target would close a dependency cycle
// for from, by walking the run's wait-for chain. from is the node currently
// evaluating in this goroutine; target is the in-flight node it is about to
// wait for. The caller must hold rs.mu.
func (rs *runState) wouldCycle(from, target *node) bool {
	for n := target; n != nil; n = rs.waitsOn[n] {
		if n == from {
			return true
		}
	}

	return false
}

// clearWait drops from's wait-for edge once it stops blocking on id.
func (rs *runState) clearWait(from, id *node) {
	rs.mu.Lock()
	defer rs.mu.Unlock()

	if rs.waitsOn[from] == id {
		delete(rs.waitsOn, from)
	}
}

// Run opens an isolated reconciliation run and flushes its effects when fn
// returns. A nested Run starts with fresh state; to share an ambient run, use
// its context directly (see InRun). The per-topic flush bound comes from ctx
// (see WithMaxTopicFlushes). If ctx carries a ReportCollector, the run appends
// its results to it when it finishes.
func Run(ctx context.Context, fn func(ctx context.Context) error) error {
	ctx = withRun(ctx, newRunState(dryrun.Enabled(ctx), MaxTopicFlushes(ctx)))
	ctx = withoutEvalFrame(ctx)

	defer collectReport(ctx)

	if err := fn(ctx); err != nil {
		return err
	}

	return Flush(ctx)
}

// RunNode opens an isolated run, evaluates n, flushes its effects and returns
// the node's value. Use it to evaluate another graph from inside a node's fn.
// The run inherits dry-run mode and the flush bound from ctx.
func RunNode[T any](ctx context.Context, n *Node[T]) (T, error) {
	var out T

	err := Run(ctx, func(ctx context.Context) error {
		var err error

		out, err = n.Value(ctx)

		return err
	})

	return out, err
}

// RunNodeVoid is RunNode for a node that yields Void.
func RunNodeVoid(ctx context.Context, n *Node[Void]) error {
	_, err := RunNode(ctx, n)
	return err
}

// InRun reports whether ctx is inside a reconciliation Run.
func InRun(ctx context.Context) bool {
	_, ok := runFrom(ctx)

	return ok
}

// Flush runs the effects queued by Notify, once each, in FCFS order, following
// cascading notifications until the worklist is empty. In dry-run mode no
// effects run. It returns ErrNoRun outside a Run.
//
// Flush may be called from any goroutine, including a node's fn; overlapping
// calls are serialized (a joiner waits for the active flush, then drains
// anything new). A flush that would wait on a node whose evaluation is itself
// flushing is a dependency cycle, reported as an error rather than deadlocking.
func Flush(ctx context.Context) error {
	rs, ok := runFrom(ctx)
	if !ok {
		return fmt.Errorf("reconcile: %w", ErrNoRun)
	}

	var frame *evalFrame
	if f := evalFrameFrom(ctx); f != nil {
		frame = f
	}

	for {
		rs.mu.Lock()

		if rs.flush == nil {
			fid := &node{name: "flush"}
			if frame != nil {
				fid = frame.node
			}

			f := &flushState{flusher: fid, done: make(chan struct{})}
			rs.flush = f

			rs.mu.Unlock()

			effectCtx := ctx
			if frame == nil {
				effectCtx = withEvalFrame(ctx, &evalFrame{node: fid})
			}

			err := rs.flushEffects(effectCtx)

			rs.mu.Lock()
			f.err = err
			rs.flush = nil
			close(f.done)
			rs.mu.Unlock()

			return err
		}

		f := rs.flush

		if frame != nil && frame.contains(f.flusher) {
			rs.mu.Unlock()

			return fmt.Errorf("reconcile: flush: %w", ErrCycle)
		}

		joined := false

		if frame != nil {
			if rs.waitsOn[frame.node] != nil {
				rs.mu.Unlock()

				return fmt.Errorf("reconcile: node %q: %w", frame.node.name, ErrConcurrentUse)
			}

			if rs.wouldCycle(frame.node, f.flusher) {
				rs.mu.Unlock()

				return fmt.Errorf("reconcile: flush: %w", ErrCycle)
			}

			rs.waitsOn[frame.node] = f.flusher
			joined = true
		}

		rs.mu.Unlock()

		select {
		case <-f.done:
		case <-ctx.Done():
			if joined {
				rs.clearWait(frame.node, f.flusher)
			}

			return ctx.Err()
		}

		if joined {
			rs.clearWait(frame.node, f.flusher)
		}

		if f.err != nil {
			return f.err
		}
	}
}

// flushEffects drains dirty topics in first-notified order, running each
// effect once and following cascades, until the worklist is empty.
func (rs *runState) flushEffects(ctx context.Context) error {
	flushed := map[*Topic]int{}

	for {
		rs.mu.Lock()

		if len(rs.dirtyOrder) == 0 {
			rs.mu.Unlock()
			return nil
		}

		t := rs.dirtyOrder[0]
		rs.dirtyOrder = rs.dirtyOrder[1:]
		delete(rs.dirty, t)

		rs.mu.Unlock()

		flushed[t]++
		if flushed[t] > rs.maxTopicFlushes {
			return fmt.Errorf("reconcile: topic %q flushed more than %d times: %w", t.name, rs.maxTopicFlushes, ErrMaxTopicFlushes)
		}

		if rs.dryRun {
			continue
		}

		for _, e := range t.snapshotEffects() {
			if err := e.run(ctx); err != nil {
				return fmt.Errorf("reconcile: effect %q: %w", e.name, err)
			}
		}
	}
}
