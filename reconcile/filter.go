package reconcile

import "context"

// Filter returns a data node whose value is source's value when cond returns
// true, and the zero value otherwise. When cond returns false the source is not
// evaluated. It is a data node: it runs in a dry run and is memoized once per
// run.
func Filter[T any](source *Node[T], cond func(ctx context.Context) (bool, error)) *Node[T] {
	return Data(source.Name()+":filter", func(ctx context.Context) (T, error) {
		ok, err := cond(ctx)
		if err != nil {
			var zero T
			return zero, err
		}

		if !ok {
			var zero T
			return zero, nil
		}

		return source.Value(ctx)
	}).Node()
}

// IfChanged is Filter with a node condition: it evaluates source only when cond's
// value is true.
func IfChanged[T any](source *Node[T], cond *Node[bool]) *Node[T] {
	return Filter(source, func(ctx context.Context) (bool, error) {
		return cond.Value(ctx)
	})
}
