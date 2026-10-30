package gabrice2e

import (
	"context"

	"github.com/bancek/gabric/reconcile"
)

// RunNodeErr evaluates node in a fresh run and returns its value and error.
func (g *Test) RunNodeErr[T any](ctx context.Context, node *reconcile.Node[T]) (T, error) {
	g.t.Helper()

	return reconcile.RunNode(ctx, node)
}

// RunNode evaluates node in a fresh run and fails the test on error.
func (g *Test) RunNode[T any](ctx context.Context, node *reconcile.Node[T]) T {
	g.t.Helper()

	value, err := g.RunNodeErr(ctx, node)
	if err != nil {
		g.t.Fatalf("gabrice2e: run %s: %v", node.Name(), err)
	}

	return value
}

// RunReportErr evaluates node in a fresh run with a private report collector.
// It returns the report even when the run fails.
func (g *Test) RunReportErr[T any](ctx context.Context, node *reconcile.Node[T]) (T, []reconcile.ReportResult, error) {
	g.t.Helper()

	collector := &reconcile.ReportCollector{}
	ctx = reconcile.WithReportCollector(ctx, collector)

	var value T

	err := reconcile.Run(ctx, func(ctx context.Context) error {
		var err error
		value, err = node.Value(ctx)

		return err
	})

	return value, collector.Results(), err
}

// RunReport evaluates node in a fresh run and fails the test on error.
func (g *Test) RunReport[T any](ctx context.Context, node *reconcile.Node[T]) (T, []reconcile.ReportResult) {
	g.t.Helper()

	value, results, err := g.RunReportErr(ctx, node)
	if err != nil {
		g.t.Fatalf("gabrice2e: run %s: %v\n%s", node.Name(), err, reconcile.FormatReport(results))
	}

	return value, results
}

// RunReportRef evaluates a node or group cover in a fresh run with a private
// report collector. It returns the report even when the run fails.
func (g *Test) RunReportRef(ctx context.Context, ref reconcile.NodeRef) ([]reconcile.ReportResult, error) {
	g.t.Helper()

	collector := &reconcile.ReportCollector{}
	ctx = reconcile.WithReportCollector(ctx, collector)

	err := reconcile.Run(ctx, func(ctx context.Context) error {
		return ref.Evaluate(ctx)
	})

	return collector.Results(), err
}
