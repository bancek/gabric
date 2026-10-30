package github_test

import (
	"io/fs"
	"testing"

	"github.com/bancek/gabric/input"
	"github.com/bancek/gabric/reconcile"
	"github.com/bancek/gabric/reconcile/modules"
	"github.com/bancek/gabric/reconcile/modules/github"
	"github.com/bancek/gabric/reconcile/modules/moduletest"
)

func TestRawFileE2E(t *testing.T) {
	moduletest.ForEachHTTPTarget(t, func(g *moduletest.Target, server *moduletest.FileServer) {
		t := g.T()

		const (
			owner = "owner"
			repo  = "repo"
			file  = "bin/app"
		)

		server.Put("owner/repo/v1/bin/app", []byte("one\n"))
		server.Put("owner/repo/v2/bin/app", []byte("two\n"))

		dest := g.Path("app")
		refFile := dest + ".ref"

		raw := func(ref string) *github.RawFileGroup {
			return github.RawFile(input.Static(owner), input.Static(repo), input.Static(file), input.Static(ref)).
				BaseURL(input.Static(server.URL)).
				Dest(input.Static(dest)).
				Mode(modules.FileMode(0o755)).
				Build()
		}

		v1 := raw("v1")

		moduletest.AssertDryRun(t, g, v1)
		assertRawFileConverges(t, g, v1)
		moduletest.AssertFile(t, g, dest, []byte("one\n"))
		assertRawFile(t, g, dest, 0o755)
		moduletest.AssertFile(t, g, refFile, []byte("v1"))

		ref, err := g.RunNodeErr(g.Context(), v1.Ref)
		if err != nil {
			t.Fatalf("Ref(): %v", err)
		}
		if ref != "v1" {
			t.Errorf("Ref() = %q, want v1", ref)
		}

		v2 := raw("v2")

		assertRawFileConverges(t, g, v2)
		moduletest.AssertFile(t, g, dest, []byte("two\n"))
		moduletest.AssertFile(t, g, refFile, []byte("v2"))

		server.Put("owner/repo/v2/bin/app", []byte("three\n"))
		if _, err := g.RunNodeErr(g.Context(), v2.Node); err != nil {
			t.Fatalf("same-ref run: %v", err)
		}
		moduletest.AssertFile(t, g, dest, []byte("two\n"))
		moduletest.AssertFile(t, g, refFile, []byte("v2"))

		missing := github.RawFile(input.Static(owner), input.Static(repo), input.Static(file), input.Static("missing")).
			BaseURL(input.Static(server.URL)).
			Dest(input.Static(dest)).
			Mode(modules.FileMode(0o755)).
			Build()

		if _, err := g.RunNodeErr(g.Context(), missing.Node); err == nil {
			t.Error("missing ref error = nil, want error")
		}
		moduletest.AssertFile(t, g, dest, []byte("two\n"))
		moduletest.AssertFile(t, g, refFile, []byte("v2"))
	})
}

func assertRawFileConverges(t testing.TB, g *moduletest.Target, ref reconcile.NodeRef) {
	t.Helper()

	first, err := g.RunReportRef(g.Context(), ref)
	if err != nil {
		t.Fatalf("first run: %v\n%s", err, reconcile.FormatReport(first))
	}
	if !hasReportStatus(first, reconcile.ReportChanged) {
		t.Fatalf("first run report:\n%s", reconcile.FormatReport(first))
	}

	second, err := g.RunReportRef(g.Context(), ref)
	if err != nil {
		t.Fatalf("second run: %v\n%s", err, reconcile.FormatReport(second))
	}
	for _, status := range []reconcile.ReportStatus{
		reconcile.ReportChanged,
		reconcile.ReportWouldChange,
		reconcile.ReportFailed,
	} {
		if hasReportStatus(second, status) {
			t.Fatalf("second run report:\n%s", reconcile.FormatReport(second))
		}
	}
}

func hasReportStatus(results []reconcile.ReportResult, status reconcile.ReportStatus) bool {
	for _, result := range results {
		if result.Status == status {
			return true
		}
	}

	return false
}

func assertRawFile(t testing.TB, g *moduletest.Target, path string, mode fs.FileMode) {
	t.Helper()

	stat, err := g.Ops().Stat(path).RunErr()
	if err != nil {
		t.Fatalf("stat %s: %v", path, err)
	}
	if got := stat.Mode.Perm(); got != mode {
		t.Errorf("%s mode = %v, want %v", path, got, mode)
	}
	if stat.User != g.User {
		t.Errorf("%s user = %q, want %q", path, stat.User, g.User)
	}
	if stat.Group != g.Group {
		t.Errorf("%s group = %q, want %q", path, stat.Group, g.Group)
	}
}
