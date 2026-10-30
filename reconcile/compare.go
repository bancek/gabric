package reconcile

import "context"

// ValueMatches returns a Check/condition that reads n and applies fn to its
// value, propagating a read error. Use it to build a Resource Check or Filter
// condition from a node value:
//
//	reconcile.Resource("app",
//		reconcile.ValueMatches(version, func(v string) bool { return v == desired }), apply)
//
// The returned func is not a node: it reads the (memoized) node each call, and
// its predicate cannot propagate an error, so it fits single-read conditions
// (use a node for IfChanged).
func ValueMatches[T any](n *Node[T], fn func(T) bool) func(ctx context.Context) (bool, error) {
	return func(ctx context.Context) (bool, error) {
		v, err := n.Value(ctx)
		if err != nil {
			return false, err
		}

		return fn(v), nil
	}
}

// ValueEquals is ValueMatches(n, func(v T) bool { return v == want }).
func ValueEquals[T comparable](n *Node[T], want T) func(ctx context.Context) (bool, error) {
	return ValueMatches(n, func(v T) bool { return v == want })
}

// NodesEqual returns a Check/condition reporting whether two nodes' values are
// equal, propagating either read error.
func NodesEqual[T comparable](a, b *Node[T]) func(ctx context.Context) (bool, error) {
	return func(ctx context.Context) (bool, error) {
		av, err := a.Value(ctx)
		if err != nil {
			return false, err
		}

		bv, err := b.Value(ctx)
		if err != nil {
			return false, err
		}

		return av == bv, nil
	}
}
