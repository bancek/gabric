package reconcile

import (
	"context"
	"slices"
)

// DataBuilder builds a data node: a read whose value is produced by a function
// and memoized once per run. It runs outside Do, so it runs in a dry run.
//
// A DataBuilder is immutable: every method returns a new builder.
type DataBuilder[T any] struct {
	name  string
	data  func(ctx context.Context) (T, error)
	needs []func(ctx context.Context) error
}

// Data starts a data node whose value is fn(ctx).
func Data[T any](name string, fn func(ctx context.Context) (T, error)) *DataBuilder[T] {
	return &DataBuilder[T]{name: name, data: fn}
}

func (b *DataBuilder[T]) clone() *DataBuilder[T] {
	c := *b
	c.needs = slices.Clone(b.needs)

	return &c
}

// Needs adds dependencies read before the data function. They are not
// invalidated with this node. It accepts nodes and group covers. It returns a
// new builder.
func (b *DataBuilder[T]) Needs(refs ...NodeRef) *DataBuilder[T] {
	next := b.clone()

	for _, ref := range refs {
		next.needs = append(next.needs, ref.Evaluate)
	}

	return next
}

// Node builds the node. The builder's configuration is snapshotted, so later
// changes to the builder do not affect the returned node.
func (b *DataBuilder[T]) Node() *Node[T] {
	name := b.name
	data := b.data
	needs := slices.Clone(b.needs)

	return NewNode(name, func(ctx context.Context) (T, error) {
		var zero T

		for _, need := range needs {
			if err := need(ctx); err != nil {
				return zero, err
			}
		}

		return data(ctx)
	})
}
