package moduletest

import (
	"bytes"
	"testing"

	"github.com/bancek/gabric/dryrun"
	"github.com/bancek/gabric/reconcile"
)

// AssertDryRun asserts that node would change on a clean target without
// mutating it.
func AssertDryRun(t testing.TB, g *Target, ref reconcile.NodeRef) {
	t.Helper()

	results, err := g.RunReportRef(dryrun.With(g.Context()), ref)
	if err != nil {
		t.Fatalf("dry run: %v\n%s", err, reconcile.FormatReport(results))
	}
	wouldChange, changed, failed := countStatuses(results)

	if wouldChange == 0 || changed != 0 || failed != 0 {
		t.Fatalf("dry run report:\n%s", reconcile.FormatReport(results))
	}
}

// AssertConverges asserts that node changes once and is satisfied on the next
// run.
func AssertConverges(t testing.TB, g *Target, ref reconcile.NodeRef) {
	t.Helper()

	first, err := g.RunReportRef(g.Context(), ref)
	if err != nil {
		t.Fatalf("first run: %v\n%s", err, reconcile.FormatReport(first))
	}
	_, changed, failed := countStatuses(first)
	if changed == 0 || failed != 0 {
		t.Fatalf("first run report:\n%s", reconcile.FormatReport(first))
	}

	second, err := g.RunReportRef(g.Context(), ref)
	if err != nil {
		t.Fatalf("second run: %v\n%s", err, reconcile.FormatReport(second))
	}
	wouldChange, changed, failed := countStatuses(second)
	if changed != 0 || wouldChange != 0 || failed != 0 {
		t.Fatalf("second run report:\n%s", reconcile.FormatReport(second))
	}

	if ok := countOK(second); ok == 0 {
		t.Fatalf("second run reported no satisfied resources:\n%s", reconcile.FormatReport(second))
	}
}

// AssertFile asserts that path has the expected content on the target.
func AssertFile(t testing.TB, g *Target, path string, content []byte) {
	t.Helper()

	got, err := g.Ops().Get(path).BytesErr()
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	if !bytes.Equal(got, content) {
		t.Errorf("%s content = %q, want %q", path, got, content)
	}
}

func countStatuses(results []reconcile.ReportResult) (wouldChange, changed, failed int) {
	for _, result := range results {
		switch result.Status {
		case reconcile.ReportWouldChange:
			wouldChange++
		case reconcile.ReportChanged:
			changed++
		case reconcile.ReportFailed:
			failed++
		}
	}

	return wouldChange, changed, failed
}

func countOK(results []reconcile.ReportResult) int {
	var ok int

	for _, result := range results {
		if result.Status == reconcile.ReportOK {
			ok++
		}
	}

	return ok
}
