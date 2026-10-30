package gabrice2e

import (
	"context"
	"errors"
	"testing"

	"github.com/bancek/gabric/reconcile"
)

func TestRunNode(t *testing.T) {
	g := &Test{t: t, ctx: t.Context()}
	applied := false

	node := reconcile.ResourceBool("res", nil, func(ctx context.Context) error {
		applied = true
		return nil
	}).Node()

	if got := g.RunNode(g.Context(), node); !got {
		t.Error("RunNode() = false, want true")
	}
	if !applied {
		t.Error("RunNode() did not apply the resource")
	}
}

func TestRunReport(t *testing.T) {
	g := &Test{t: t, ctx: t.Context()}

	node := reconcile.ResourceBool("res", nil, func(ctx context.Context) error {
		return nil
	}).Node()

	changed, results := g.RunReport(g.Context(), node)
	if !changed {
		t.Error("RunReport() = false, want true")
	}
	if len(results) != 1 {
		t.Fatalf("report results = %d, want 1", len(results))
	}
	if results[0].Name != "res" || results[0].Status != reconcile.ReportChanged {
		t.Errorf("report = %#v, want changed res", results[0])
	}
}

func TestRunReportRef(t *testing.T) {
	g := &Test{t: t, ctx: t.Context()}

	node := reconcile.ResourceBool("res", nil, func(ctx context.Context) error {
		return nil
	}).Node()

	results, err := g.RunReportRef(g.Context(), node)
	if err != nil {
		t.Fatalf("RunReportRef() error: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("report results = %d, want 1", len(results))
	}
	if results[0].Name != "res" || results[0].Status != reconcile.ReportChanged {
		t.Errorf("report = %#v, want changed res", results[0])
	}
}

func TestRunReportErrReturnsReport(t *testing.T) {
	g := &Test{t: t, ctx: t.Context()}
	sentinel := errors.New("boom")

	node := reconcile.Resource("res", nil, func(ctx context.Context) (reconcile.Void, error) {
		return reconcile.Void{}, sentinel
	}).Node()

	_, results, err := g.RunReportErr(g.Context(), node)
	if !errors.Is(err, sentinel) {
		t.Fatalf("RunReportErr() error = %v, want %v", err, sentinel)
	}
	if len(results) != 1 {
		t.Fatalf("report results = %d, want 1", len(results))
	}
	if results[0].Name != "res" || results[0].Status != reconcile.ReportFailed {
		t.Errorf("report = %#v, want failed res", results[0])
	}
}

func TestRunReportUsesFreshCollector(t *testing.T) {
	g := &Test{t: t, ctx: t.Context()}

	node := reconcile.ResourceBool("res", nil, func(ctx context.Context) error {
		return nil
	}).Node()

	_, first := g.RunReport(g.Context(), node)
	_, second := g.RunReport(g.Context(), node)

	if len(first) != 1 || len(second) != 1 {
		t.Errorf("report lengths = %d, %d; want 1, 1", len(first), len(second))
	}
}
