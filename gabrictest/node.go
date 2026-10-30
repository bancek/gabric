package gabrictest

import (
	"context"

	"github.com/bancek/gabric/reconcile"
)

// RunNodeErr evaluates node inside a new reconcile run and returns its value.
func (g *Test) RunNodeErr[T any](ctx context.Context, node *reconcile.Node[T]) (T, error) {
	g.t.Helper()

	var value T

	err := reconcile.Run(ctx, func(ctx context.Context) error {
		var err error
		value, err = node.Value(ctx)

		return err
	})

	return value, err
}

// RunNode evaluates node inside a new reconcile run and fails the test on error.
func (g *Test) RunNode[T any](ctx context.Context, node *reconcile.Node[T]) T {
	g.t.Helper()

	value, err := g.RunNodeErr(ctx, node)
	if err != nil {
		g.t.Fatalf("RunNode() error: %v", err)
	}

	return value
}
