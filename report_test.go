package gabric

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/bancek/gabric/reconcile"
)

func TestWithReconcileReportLogs(t *testing.T) {
	ctx, buf := testContextBuffer(t)

	g := New()
	g.Use(WithReconcileReport)

	node := reconcile.ResourceBool("res",
		func(ctx context.Context) (bool, error) { return false, nil },
		func(ctx context.Context) error { return nil }).
		Node()

	g.With().NodeTask(node)

	if err := g.Run(ctx, parseCalls(t, g, "res")); err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	out := buf.String()

	if !strings.Contains(out, "reconcile: ") {
		t.Errorf("log = %q, want lines prefixed with 'reconcile: '", out)
	}

	if !strings.Contains(out, "changed      res") {
		t.Errorf("log = %q, want the resource's changed line", out)
	}

	if !strings.Contains(out, "1 changed") {
		t.Errorf("log = %q, want a '1 changed' summary", out)
	}
}

func TestWithReportIsolatedRuns(t *testing.T) {
	ctx, buf := testContextBuffer(t)

	g := New()
	g.Use(WithReport)

	app1 := reconcile.ResourceBool("app1",
		func(ctx context.Context) (bool, error) { return false, nil },
		func(ctx context.Context) error { return nil }).
		Node()
	app2 := reconcile.ResourceBool("app2",
		func(ctx context.Context) (bool, error) { return false, nil },
		func(ctx context.Context) error { return nil }).
		Node()

	g.TaskErr("deploy", func(ctx context.Context) error {
		if _, err := reconcile.RunNode(ctx, app1); err != nil {
			return err
		}

		_, err := reconcile.RunNode(ctx, app2)

		return err
	})

	if err := g.Run(ctx, parseCalls(t, g, "deploy")); err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	out := buf.String()

	for _, want := range []string{"changed      app1", "changed      app2", "2 changed"} {
		if !strings.Contains(out, want) {
			t.Errorf("log = %q, want %q", out, want)
		}
	}
}

func TestWithReportReusesCollector(t *testing.T) {
	ctx, buf := testContextBuffer(t)

	collector := &reconcile.ReportCollector{}
	ctx = reconcile.WithReportCollector(ctx, collector)

	g := New()
	g.Use(WithReport)

	node := reconcile.ResourceBool("res",
		func(ctx context.Context) (bool, error) { return false, nil },
		func(ctx context.Context) error { return nil }).
		Node()

	g.TaskErr("deploy", func(ctx context.Context) error {
		_, err := reconcile.RunNode(ctx, node)
		return err
	})

	if err := g.Run(ctx, parseCalls(t, g, "deploy")); err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	results := collector.Results()
	if len(results) != 1 || results[0].Name != "res" {
		t.Fatalf("collector results = %#v, want one res", results)
	}

	if !strings.Contains(buf.String(), "changed      res") {
		t.Errorf("log = %q, want the resource's changed line", buf.String())
	}
}

func TestIsolatedReportCollector(t *testing.T) {
	ctx, buf := testContextBuffer(t)

	outer := &reconcile.ReportCollector{}
	ctx = reconcile.WithReportCollector(ctx, outer)

	g := New()
	g.Use(WithReport)

	outerNode := reconcile.ResourceBool("outer",
		func(ctx context.Context) (bool, error) { return false, nil },
		func(ctx context.Context) error { return nil }).
		Node()
	nestedNode := reconcile.ResourceBool("nested",
		func(ctx context.Context) (bool, error) { return false, nil },
		func(ctx context.Context) error { return nil }).
		Node()

	var isolated *reconcile.ReportCollector

	g.TaskErr("deploy", func(ctx context.Context) error {
		if _, err := reconcile.RunNode(ctx, outerNode); err != nil {
			return err
		}

		isolated = &reconcile.ReportCollector{}
		_, err := reconcile.RunNode(reconcile.WithReportCollector(ctx, isolated), nestedNode)

		return err
	})

	if err := g.Run(ctx, parseCalls(t, g, "deploy")); err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	outerResults := outer.Results()
	if len(outerResults) != 1 || outerResults[0].Name != "outer" {
		t.Fatalf("outer results = %#v, want only outer", outerResults)
	}

	isolatedResults := isolated.Results()
	if len(isolatedResults) != 1 || isolatedResults[0].Name != "nested" {
		t.Fatalf("isolated results = %#v, want only nested", isolatedResults)
	}

	out := buf.String()
	if !strings.Contains(out, "changed      outer") {
		t.Errorf("log = %q, want the outer result", out)
	}

	if strings.Contains(out, "changed      nested") {
		t.Errorf("log = %q, want the isolated result excluded", out)
	}
}

func TestLogReport(t *testing.T) {
	ctx, buf := testContextBuffer(t)

	LogReport(ctx, []reconcile.ReportResult{
		{Name: "a", Status: reconcile.ReportChanged},
		{Name: "b", Status: reconcile.ReportFailed, Err: errors.New("boom")},
	})

	out := buf.String()

	for _, want := range []string{
		"reconcile:   changed      a",
		"reconcile:   failed       b (boom)",
		"1 changed",
		"1 failed",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("log = %q, want %q", out, want)
		}
	}
}

func TestLogReportEmpty(t *testing.T) {
	ctx, buf := testContextBuffer(t)

	LogReport(ctx, nil)

	if buf.Len() != 0 {
		t.Errorf("log = %q, want empty", buf.String())
	}
}

func TestWithReconcileReportLogsOnError(t *testing.T) {
	ctx, buf := testContextBuffer(t)

	g := New()
	g.Use(WithReconcileReport)

	node := reconcile.Resource("res", nil,
		func(ctx context.Context) (reconcile.Void, error) {
			return reconcile.Void{}, errors.New("boom")
		}).
		Node()

	g.With().NodeTask(node) // panic-style: the node error becomes a task error

	if err := g.Run(ctx, parseCalls(t, g, "res")); err == nil {
		t.Fatal("Run() error = nil, want the node's error")
	}

	out := buf.String()

	if !strings.Contains(out, "reconcile: ") {
		t.Errorf("log = %q, want the report logged on failure", out)
	}

	if !strings.Contains(out, "failed       res") {
		t.Errorf("log = %q, want the resource's failed line", out)
	}

	if !strings.Contains(out, "1 failed") {
		t.Errorf("log = %q, want a '1 failed' summary", out)
	}
}
