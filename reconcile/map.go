package reconcile

import "context"

// Map returns a data node whose value is fn applied to n's value. Like Data, it
// runs outside Do (so it runs in a dry run) and its result is memoized once per
// run. It is named "<n>:map". Invalidating the Map recomputes it from n's
// current value; invalidate n (or start a new Run) to force a re-probe.
func Map[T, U any](n *Node[T], fn func(ctx context.Context, v T) (U, error)) *Node[U] {
	return Data(n.Name()+":map", func(ctx context.Context) (U, error) {
		v, err := n.Value(ctx)
		if err != nil {
			var zero U
			return zero, err
		}

		return fn(ctx, v)
	}).Node()
}
