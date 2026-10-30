package reconcile

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"sync/atomic"

	"github.com/bancek/gabric/middleware"
)

var (
	errMiddlewareNoNext    = errors.New("middleware did not call next")
	errMiddlewareNextTwice = errors.New("middleware called next more than once")
)

// node is the type-erased identity shared by every Node[T].
//
// Node[T] is generic, but the run's state is heterogeneous: its result map, its
// dependency edges and its evaluation frames reference nodes of different
// output types at the same time. A single non-generic map or struct cannot hold
// *Node[T] for a fixed T, so every node carries this small non-generic base,
// which is the comparable key used throughout the engine.
//
// Erasing to a concrete *node (rather than keying the maps by any) keeps the
// engine's internals statically typed; the only erased value is the node's
// output, stored as any and asserted back to T in Output.
//
// (A *Node[T] cannot be used as the identity directly: frame and runState are
// non-generic types shared by all nodes, so their fields could not hold
// *Node[X] for a fixed X while also holding Node[Y].)
//
// It holds the node's name (so it is non-zero-sized: the address of a zero-size
// embedded field is not guaranteed to be unique) and whether it is a group: a
// group runs its members on every read, is never memoized, and is not part of
// the dependency/invalidation graph.
type node struct {
	name  string
	group bool
}

// Void is the empty value for nodes that produce no meaningful result.
type Void = struct{}

// Node is a lazy, memoized, idempotent unit of work that yields a value. Its
// dependencies are the nodes it reads while evaluating (implicit, discovered at
// evaluation time). A Node's fn must be stateless; per-run state lives in the
// run, and middleware is immutable construction-time configuration.
//
// It embeds node by value; the embedded value's address (&n.node) is the
// type-erased identity used by the run state and evaluation frames.
type Node[T any] struct {
	node
	fn          func(ctx context.Context) (T, error)
	middlewares []middleware.Middleware
}

// NewNode creates a node. It is evaluated at most once per run.
func NewNode[T any](name string, fn func(ctx context.Context) (T, error)) *Node[T] {
	return &Node[T]{node: node{name: name}, fn: fn}
}

// NodeRef is a node or a type embedding one, such as a group cover. The
// unexported nodeRef method keeps the interface sealed to reconcile types.
type NodeRef interface {
	Evaluate(ctx context.Context) error
	nodeRef() *node
}

// Evaluate evaluates the node and discards its value.
func (n *Node[T]) Evaluate(ctx context.Context) error {
	_, err := n.Value(ctx)

	return err
}

func (n *Node[T]) nodeRef() *node {
	return &n.node
}

// Evaluate evaluates node references in order. It returns ErrNoRun outside a
// run.
func Evaluate(ctx context.Context, refs ...NodeRef) error {
	for _, ref := range refs {
		if ref == nil {
			return fmt.Errorf("reconcile: nil node reference")
		}
		if err := ref.Evaluate(ctx); err != nil {
			return err
		}
	}

	return nil
}

// Name returns the node name.
func (n *Node[T]) Name() string {
	return n.name
}

// Clone returns an independent node with the same name and function as n but a
// distinct identity: it is memoized, invalidated and depended on separately. Use
// it to derive the observed counterpart of a probe from a single read (read n
// in a resource's Check and invalidate n.Clone() after Apply), rather than
// constructing the read twice.
//
// The clone shares n's fn and middleware, so cloning a resource duplicates its
// mutation; Clone is intended for read (Data) nodes.
func (n *Node[T]) Clone() *Node[T] {
	c := *n
	c.middlewares = slices.Clone(n.middlewares)

	return &c
}

// With returns a clone of n with middlewares appended. The middleware wraps the
// node's whole evaluation, including its dependencies, and must call next
// exactly once. It does not receive or change the node's value.
//
// With returns a distinct node identity: use the returned node in place of n in
// groups, dependencies, and ApplyInvalidates.
func (n *Node[T]) With(middlewares ...middleware.Middleware) *Node[T] {
	if len(middlewares) == 0 {
		return n
	}

	c := n.Clone()
	c.middlewares = append(c.middlewares, middlewares...)

	return c
}

// evalFrame is the goroutine-local chain of nodes being evaluated, carried in
// the context so cycle detection does not need shared state. It holds the
// type-erased *node identity (not *Node[T]) for the same reason node exists.
//
// busy is set while the node's fn is inside a Value call; a non-casable busy
// means two goroutines are using the same evaluation (a node's fn spawning
// goroutines that call Value), which is reported as ErrConcurrentUse.
type evalFrame struct {
	node   *node
	parent *evalFrame
	busy   atomic.Bool
}

const evalFrameKey contextKey[*evalFrame] = "reconcile.evalFrame"

func withEvalFrame(ctx context.Context, f *evalFrame) context.Context {
	return withContextValue(ctx, evalFrameKey, f)
}

// withoutEvalFrame returns a context with no evaluation frame, so Run can start
// an isolated run even when called from inside a node's fn.
func withoutEvalFrame(ctx context.Context) context.Context {
	return withEvalFrame(ctx, nil)
}

func evalFrameFrom(ctx context.Context) *evalFrame {
	f, _ := contextValue(ctx, evalFrameKey)

	return f
}

func (f *evalFrame) contains(n *node) bool {
	for ; f != nil; f = f.parent {
		if f.node == n {
			return true
		}
	}

	return false
}

// Value evaluates the node (if not already memoized this run) and returns its
// value. Reading a node is what triggers it.
//
// Value is safe to call concurrently on one run from different node
// evaluations: a value node is evaluated at most once per run, and concurrent
// callers wait for and share the single result. A group is not memoized: it
// runs its members on every read. A dependency cycle that spans goroutines is
// detected and returned as ErrCycle.
//
// A node's fn must call Value sequentially; calling it from goroutines the fn
// spawns returns ErrConcurrentUse. Returns ErrNoRun outside a Run.
func (n *Node[T]) Value(ctx context.Context) (T, error) {
	var zero T

	rs, ok := runFrom(ctx)
	if !ok {
		return zero, fmt.Errorf("reconcile: %w", ErrNoRun)
	}

	id := &n.node
	frame := evalFrameFrom(ctx)

	if frame != nil {
		if !frame.busy.CompareAndSwap(false, true) {
			return zero, fmt.Errorf("reconcile: node %q: %w", frame.node.name, ErrConcurrentUse)
		}

		defer frame.busy.Store(false)
	}

	if frame.contains(id) {
		return zero, fmt.Errorf("reconcile: node %q: %w", n.name, ErrCycle)
	}

	// A group is never memoized: it runs its members (which are memoized
	// value nodes) in order on every read.
	if n.group {
		if frame != nil && !frame.node.group {
			rs.mu.Lock()
			rs.addEdge(frame.node, id)
			rs.mu.Unlock()
		}

		return n.call(ctx, frame)
	}

	rs.mu.Lock()

	// Record the edge from the node currently evaluating to this one, even when
	// this one is already memoized or in flight. Group reads are not
	// dependencies, so they record no edge.
	if frame != nil && !frame.node.group {
		rs.addEdge(frame.node, id)
	}

	if st, ok := rs.states[id]; ok {
		// Completed: the receive in completed() (or the wait below) synchronizes
		// with the owner's close(done), so val/err are safe to read.
		if st.completed() {
			rs.mu.Unlock()

			if st.err != nil {
				return zero, st.err
			}

			return st.val.(T), nil
		}

		// In flight. A top-level caller holds no node, so it cannot be part of a
		// cycle; only a node evaluation needs a wait-for edge and cycle check.
		if frame == nil {
			rs.mu.Unlock()

			select {
			case <-st.done:
			case <-ctx.Done():
				return zero, ctx.Err()
			}

			if st.err != nil {
				return zero, st.err
			}

			return st.val.(T), nil
		}

		if rs.wouldCycle(frame.node, id) {
			rs.mu.Unlock()

			return zero, fmt.Errorf("reconcile: node %q: %w", n.name, ErrCycle)
		}

		rs.waitsOn[frame.node] = id
		rs.mu.Unlock()

		select {
		case <-st.done:
		case <-ctx.Done():
			rs.clearWait(frame.node, id)

			return zero, ctx.Err()
		}

		rs.clearWait(frame.node, id)

		if st.err != nil {
			return zero, st.err
		}

		return st.val.(T), nil
	}

	// This goroutine becomes the owner: it evaluates fn, publishes the result,
	// and unblocks any waiters. Re-evaluating drops stale outgoing edges so a
	// changed dependency set does not linger.
	rs.clearEdgesFrom(id)

	st := &nodeState{done: make(chan struct{})}
	rs.states[id] = st
	rs.mu.Unlock()

	val, err := n.call(ctx, frame)

	st.val = val
	st.err = err
	close(st.done)

	if err != nil {
		return zero, err
	}

	return val, nil
}

// PanicError converts a recovered panic value into an error: an error value is
// returned unchanged; anything else is wrapped. It lets a node's fn panic with
// an error and still keep errors.Is/errors.As semantics.
func PanicError(r any) error {
	if err, ok := r.(error); ok {
		return err
	}

	return fmt.Errorf("%v", r)
}

// call evaluates the node's fn with the node's eval frame, converting a panic
// into an error so waiters are always unblocked.
func (n *Node[T]) call(ctx context.Context, parent *evalFrame) (v T, err error) {
	defer func() {
		if r := recover(); r != nil {
			var zero T

			v, err = zero, fmt.Errorf("reconcile: node %q panicked: %w", n.name, PanicError(r))
		}
	}()

	fn := n.fn
	if len(n.middlewares) > 0 {
		fn = func(ctx context.Context) (T, error) {
			var (
				value T
				calls int
			)

			next := func(ctx context.Context) error {
				calls++
				if calls > 1 {
					return errMiddlewareNextTwice
				}

				var err error
				value, err = n.fn(ctx)

				return err
			}

			err := middleware.Apply(n.middlewares, next)(ctx)
			if calls > 1 {
				var zero T

				return zero, errMiddlewareNextTwice
			}
			if err == nil && calls == 0 {
				var zero T

				return zero, errMiddlewareNoNext
			}

			return value, err
		}
	}

	v, err = fn(withEvalFrame(ctx, &evalFrame{node: &n.node, parent: parent}))
	if err != nil {
		var zero T

		return zero, fmt.Errorf("reconcile: node %q: %w", n.name, err)
	}

	return v, nil
}

// Do runs the mutation fn unless the run is in dry-run mode. Resources probe
// outside Do and mutate inside it. It returns ErrNoRun outside a Run.
func Do(ctx context.Context, fn func() error) error {
	rs, ok := runFrom(ctx)
	if !ok {
		return fmt.Errorf("reconcile: %w", ErrNoRun)
	}

	if rs.dryRun {
		return nil
	}

	return fn()
}
