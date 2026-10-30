package reconcile

import (
	"context"
	"fmt"
)

// Invalidate clears the node's memoized value and every node that (transitively)
// depends on it, so they re-evaluate on their next Value. Dependencies declared
// with Needs or CheckNeeds are not touched. It returns ErrNoRun outside a Run.
//
// Group nodes are not memoized and are not part of the dependency graph, so
// invalidating one clears its consumers; it does not reset its parts.
// Start a new Run to reset a group (and everything else).
//
// Invalidate may be called from any goroutine; overlapping calls are
// serialized. Invalidating a node (or a dependent) that is being evaluated
// returns ErrConcurrentUse.
//
// Dependency edges are intentionally left in place: they stay valid (a
// dependent still depends on n until it re-evaluates), and Value clears a
// node's outgoing edges before it re-evaluates, so a changed dependency set is
// reconciled there rather than here.
func (n *Node[T]) Invalidate(ctx context.Context) error {
	return invalidate(ctx, &n.node)
}

// invalidate is the operation behind Node.Invalidate and Builder.Invalidates: it
// clears id and its dependents.
func invalidate(ctx context.Context, id *node) error {
	rs, ok := runFrom(ctx)
	if !ok {
		return fmt.Errorf("reconcile: %w", ErrNoRun)
	}

	rs.mu.Lock()
	defer rs.mu.Unlock()

	set := map[*node]bool{}
	collectAffected(rs, id, set)

	for n := range set {
		if st, ok := rs.states[n]; ok && !st.completed() {
			return fmt.Errorf("reconcile: cannot invalidate node %q: %w", n.name, ErrConcurrentUse)
		}
	}

	for n := range set {
		delete(rs.states, n)
	}

	return nil
}

// collectAffected adds n and the dependents of n (transitively) to set. The
// caller must hold rs.mu.
func collectAffected(rs *runState, n *node, set map[*node]bool) {
	if set[n] {
		return
	}

	set[n] = true

	for dependent := range rs.dependents[n] {
		collectAffected(rs, dependent, set)
	}
}
