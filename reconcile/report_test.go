package reconcile

import (
	"context"
	"errors"
	"github.com/bancek/gabric/dryrun"
	"sync"
	"testing"
)

func TestReportOK(t *testing.T) {
	node := ResourceBool("res",
		func(ctx context.Context) (bool, error) { return true, nil },
		func(ctx context.Context) error { return nil }).
		Node()

	var results []ReportResult

	if err := Run(context.Background(), func(ctx context.Context) error {
		if _, err := node.Value(ctx); err != nil {
			return err
		}

		results = Report(ctx)

		return nil
	}); err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("Report() len = %d, want 1", len(results))
	}

	if results[0].Name != "res" || results[0].Status != ReportOK || results[0].Err != nil {
		t.Errorf("result = %+v, want ok/res", results[0])
	}
}

func TestReportChanged(t *testing.T) {
	node := ResourceBool("res",
		func(ctx context.Context) (bool, error) { return false, nil },
		func(ctx context.Context) error { return nil }).
		Node()

	var results []ReportResult

	if err := Run(context.Background(), func(ctx context.Context) error {
		if _, err := node.Value(ctx); err != nil {
			return err
		}

		results = Report(ctx)

		return nil
	}); err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	if len(results) != 1 || results[0].Status != ReportChanged {
		t.Fatalf("results = %+v, want one changed", results)
	}
}

func TestReportWouldChange(t *testing.T) {
	applied := false

	node := ResourceBool("res",
		func(ctx context.Context) (bool, error) { return false, nil },
		func(ctx context.Context) error {
			applied = true
			return nil
		}).
		Node()

	var results []ReportResult

	if err := Run(dryrun.With(context.Background()), func(ctx context.Context) error {
		if _, err := node.Value(ctx); err != nil {
			return err
		}

		results = Report(ctx)

		return nil
	}); err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	if applied {
		t.Error("Apply ran during a dry run")
	}

	if len(results) != 1 || results[0].Status != ReportWouldChange {
		t.Fatalf("results = %+v, want one would-change", results)
	}
}

func TestReportAlwaysApplies(t *testing.T) {
	node := ResourceBool("cmd", nil, func(ctx context.Context) error { return nil }).Node()

	var changed, wouldChange []ReportResult

	if err := Run(context.Background(), func(ctx context.Context) error {
		if _, err := node.Value(ctx); err != nil {
			return err
		}

		changed = Report(ctx)

		return nil
	}); err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	if len(changed) != 1 || changed[0].Status != ReportChanged {
		t.Fatalf("results = %+v, want one changed", changed)
	}

	if err := Run(dryrun.With(context.Background()), func(ctx context.Context) error {
		if _, err := node.Value(ctx); err != nil {
			return err
		}

		wouldChange = Report(ctx)

		return nil
	}); err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	if len(wouldChange) != 1 || wouldChange[0].Status != ReportWouldChange {
		t.Fatalf("results = %+v, want one would-change", wouldChange)
	}
}

func TestReportCheckError(t *testing.T) {
	sentinel := errors.New("check boom")

	node := ResourceBool("res",
		func(ctx context.Context) (bool, error) { return false, sentinel },
		func(ctx context.Context) error { return nil }).
		Node()

	var results []ReportResult

	err := Run(context.Background(), func(ctx context.Context) error {
		_, err := node.Value(ctx)
		results = Report(ctx)

		return err
	})

	if !errors.Is(err, sentinel) {
		t.Fatalf("Run() error = %v, want %v", err, sentinel)
	}

	if len(results) != 1 || results[0].Status != ReportFailed || !errors.Is(results[0].Err, sentinel) {
		t.Fatalf("results = %+v, want one failed with sentinel", results)
	}
}

func TestReportApplyError(t *testing.T) {
	sentinel := errors.New("apply boom")

	node := ResourceBool("res",
		func(ctx context.Context) (bool, error) { return false, nil },
		func(ctx context.Context) error { return sentinel }).
		Node()

	var results []ReportResult

	err := Run(context.Background(), func(ctx context.Context) error {
		_, err := node.Value(ctx)
		results = Report(ctx)

		return err
	})

	if !errors.Is(err, sentinel) {
		t.Fatalf("Run() error = %v, want %v", err, sentinel)
	}

	if len(results) != 1 || results[0].Status != ReportFailed || !errors.Is(results[0].Err, sentinel) {
		t.Fatalf("results = %+v, want one failed with sentinel", results)
	}
}

func TestReportCheckNeedsError(t *testing.T) {
	sentinel := errors.New("dep boom")

	dep := Data("dep", func(ctx context.Context) (int, error) { return 0, sentinel }).Node()

	node := ResourceBool("res",
		func(ctx context.Context) (bool, error) { return true, nil },
		func(ctx context.Context) error { return nil }).
		CheckNeeds(dep).
		Node()

	var results []ReportResult

	err := Run(context.Background(), func(ctx context.Context) error {
		_, err := node.Value(ctx)
		results = Report(ctx)

		return err
	})

	if !errors.Is(err, sentinel) {
		t.Fatalf("Run() error = %v, want %v", err, sentinel)
	}

	if len(results) != 1 || results[0].Status != ReportFailed {
		t.Fatalf("results = %+v, want one failed (the resource), no data entry", results)
	}
}

func TestReportOverwrite(t *testing.T) {
	calls := 0

	node := ResourceBool("res",
		func(ctx context.Context) (bool, error) {
			calls++
			return calls > 1, nil // drift on the first probe, satisfied after
		},
		func(ctx context.Context) error { return nil }).
		Node()

	var results []ReportResult

	if err := Run(context.Background(), func(ctx context.Context) error {
		if _, err := node.Value(ctx); err != nil {
			return err
		}

		if err := node.Invalidate(ctx); err != nil {
			return err
		}

		if _, err := node.Value(ctx); err != nil {
			return err
		}

		results = Report(ctx)

		return nil
	}); err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("Report() len = %d, want 1 (overwritten in place)", len(results))
	}

	if results[0].Status != ReportOK {
		t.Errorf("status = %v, want ok", results[0].Status)
	}
}

func TestReportOrder(t *testing.T) {
	mk := func(name string) *Node[bool] {
		return ResourceBool(name,
			func(ctx context.Context) (bool, error) { return true, nil },
			func(ctx context.Context) error { return nil }).
			Node()
	}

	a, b := mk("a"), mk("b")

	var results []ReportResult

	if err := Run(context.Background(), func(ctx context.Context) error {
		if _, err := b.Value(ctx); err != nil {
			return err
		}

		if _, err := a.Value(ctx); err != nil {
			return err
		}

		results = Report(ctx)

		return nil
	}); err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	if len(results) != 2 || results[0].Name != "b" || results[1].Name != "a" {
		t.Fatalf("results = %+v, want first-record order [b, a]", results)
	}
}

func TestReportDataAndGroupExcluded(t *testing.T) {
	data := Data("d", func(ctx context.Context) (int, error) { return 1, nil }).Node()
	group := Group("g").Contains(data).Node()

	var results []ReportResult

	if err := Run(context.Background(), func(ctx context.Context) error {
		if _, err := group.Value(ctx); err != nil {
			return err
		}

		results = Report(ctx)

		return nil
	}); err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	if len(results) != 0 {
		t.Errorf("results = %+v, want none (data/group are not resources)", results)
	}
}

func TestReportOutsideRun(t *testing.T) {
	if got := Report(context.Background()); got != nil {
		t.Errorf("Report() outside a run = %+v, want nil", got)
	}
}

func TestFormatReport(t *testing.T) {
	got := FormatReport([]ReportResult{
		{Name: "a", Status: ReportOK},
		{Name: "b", Status: ReportChanged},
		{Name: "c", Status: ReportWouldChange},
		{Name: "d", Status: ReportFailed, Err: errors.New("boom")},
	})

	want := `  ok           a
  changed      b
  would-change c
  failed       d (boom)
1 ok, 1 changed, 1 would-change, 1 failed`

	if got != want {
		t.Errorf("FormatReport() =\n%q\nwant\n%q", got, want)
	}
}

func TestReportCollector(t *testing.T) {
	collector := &ReportCollector{}

	collector.Collect([]ReportResult{{Name: "first", Status: ReportOK}})
	collector.Collect([]ReportResult{{Name: "second", Status: ReportChanged}})

	results := collector.Results()
	if len(results) != 2 {
		t.Fatalf("len(Results()) = %d, want 2", len(results))
	}

	if results[0].Name != "first" || results[1].Name != "second" {
		t.Errorf("Results() = %#v, want first then second", results)
	}

	results[0].Name = "mutated"
	if collector.Results()[0].Name != "first" {
		t.Error("Results() returned the collector's backing slice")
	}
}

func TestReportCollectorConcurrent(t *testing.T) {
	collector := &ReportCollector{}

	var wg sync.WaitGroup

	for range 100 {
		wg.Go(func() {
			collector.Collect([]ReportResult{{Name: "n", Status: ReportOK}})
		})
	}

	wg.Wait()

	if got := len(collector.Results()); got != 100 {
		t.Errorf("len(Results()) = %d, want 100", got)
	}
}
