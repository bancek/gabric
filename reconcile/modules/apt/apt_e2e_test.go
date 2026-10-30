package apt_test

import (
	"strings"
	"testing"

	"github.com/bancek/gabric/input"
	"github.com/bancek/gabric/reconcile"
	"github.com/bancek/gabric/reconcile/modules"
	"github.com/bancek/gabric/reconcile/modules/apt"
	"github.com/bancek/gabric/reconcile/modules/moduletest"
)

func TestAptE2E(t *testing.T) {
	moduletest.ForEachUbuntuTarget(t, func(g *moduletest.Target) {
		t := g.T()
		name := "cron"

		installed, err := g.RunNodeErr(g.Context(), apt.InstalledData(input.Static(name)))
		if err != nil {
			t.Fatalf("read installed state: %v", err)
		}
		if installed {
			t.Errorf("package %s is already installed", name)
		}

		version, err := g.RunNodeErr(g.Context(), apt.VersionData(input.Static(name)))
		if err != nil {
			t.Fatalf("read version: %v", err)
		}
		if version != "" {
			t.Errorf("version = %q, want empty", version)
		}

		satisfied, err := g.RunNodeErr(g.Context(), apt.SatisfiedData(input.Static(name), input.False))
		if err != nil {
			t.Fatalf("read satisfied state: %v", err)
		}
		if satisfied {
			t.Error("package is satisfied, want drift")
		}

		node := apt.Package(input.Static(name)).Build().Node()

		moduletest.AssertDryRun(t, g, node)
		assertDpkgInstalled(t, g, name, false)

		moduletest.AssertConverges(t, g, node)
		assertDpkgInstalled(t, g, name, true)

		installed, err = g.RunNodeErr(g.Context(), apt.InstalledData(input.Static(name)))
		if err != nil {
			t.Fatalf("read installed state: %v", err)
		}
		if !installed {
			t.Error("package is not installed")
		}

		version, err = g.RunNodeErr(g.Context(), apt.VersionData(input.Static(name)))
		if err != nil {
			t.Fatalf("read version: %v", err)
		}
		if version == "" {
			t.Error("version is empty")
		}

		candidate, err := g.RunNodeErr(g.Context(), apt.CandidateData(input.Static(name)))
		if err != nil {
			t.Fatalf("read candidate: %v", err)
		}
		if candidate == "" {
			t.Error("candidate is empty")
		}

		satisfied, err = g.RunNodeErr(g.Context(), apt.SatisfiedData(input.Static(name), input.False))
		if err != nil {
			t.Fatalf("read satisfied state: %v", err)
		}
		if !satisfied {
			t.Error("package is not satisfied")
		}

		results, err := g.RunReportRef(g.Context(), node)
		if err != nil {
			t.Fatalf("report: %v\n%s", err, reconcile.FormatReport(results))
		}
		if len(results) != 1 || results[0].Name != "apt.package:"+name || results[0].Status != reconcile.ReportOK {
			t.Fatalf("report = %#v, want one ok apt.package:%s", results, name)
		}
	})
}

func assertDpkgInstalled(t testing.TB, g *moduletest.Target, name string, want bool) {
	t.Helper()

	e := g.Ops().Exec("dpkg-query", "-W", "-f=${Status}", name).CaptureStdout()
	err := e.RunErr()
	if err != nil {
		if !modules.IsExitStatus(err, 1) {
			t.Fatalf("dpkg-query %s: %v", name, err)
		}
		if want {
			t.Errorf("package %s is not installed", name)
		}
		return
	}

	if got := strings.Contains(e.StdoutStr(), "install ok installed"); got != want {
		t.Errorf("package %s installed = %v, want %v", name, got, want)
	}
}
