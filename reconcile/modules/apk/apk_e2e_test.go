package apk_test

import (
	"testing"

	"github.com/bancek/gabric/input"
	"github.com/bancek/gabric/reconcile"
	"github.com/bancek/gabric/reconcile/modules"
	"github.com/bancek/gabric/reconcile/modules/apk"
	"github.com/bancek/gabric/reconcile/modules/moduletest"
)

func TestApkE2E(t *testing.T) {
	moduletest.ForEachAlpineTarget(t, func(g *moduletest.Target) {
		t := g.T()
		name := "curl"

		installed, err := g.RunNodeErr(g.Context(), apk.InstalledData(input.Static(name)))
		if err != nil {
			t.Fatalf("read installed state: %v", err)
		}
		if installed {
			t.Errorf("package %s is already installed", name)
		}

		version, err := g.RunNodeErr(g.Context(), apk.VersionData(input.Static(name)))
		if err != nil {
			t.Fatalf("read version: %v", err)
		}
		if version != "" {
			t.Errorf("version = %q, want empty", version)
		}

		satisfied, err := g.RunNodeErr(g.Context(), apk.SatisfiedData(input.Static(name), input.False))
		if err != nil {
			t.Fatalf("read satisfied state: %v", err)
		}
		if satisfied {
			t.Error("package is satisfied, want drift")
		}

		node := apk.Package(input.Static(name)).Build().Node()

		moduletest.AssertDryRun(t, g, node)
		assertApkInstalled(t, g, name, false)

		moduletest.AssertConverges(t, g, node)
		assertApkInstalled(t, g, name, true)

		installed, err = g.RunNodeErr(g.Context(), apk.InstalledData(input.Static(name)))
		if err != nil {
			t.Fatalf("read installed state: %v", err)
		}
		if !installed {
			t.Error("package is not installed")
		}

		version, err = g.RunNodeErr(g.Context(), apk.VersionData(input.Static(name)))
		if err != nil {
			t.Fatalf("read version: %v", err)
		}
		if version == "" {
			t.Error("version is empty")
		}

		candidate, err := g.RunNodeErr(g.Context(), apk.CandidateData(input.Static(name)))
		if err != nil {
			t.Fatalf("read candidate: %v", err)
		}
		if candidate == "" {
			t.Error("candidate is empty")
		}

		satisfied, err = g.RunNodeErr(g.Context(), apk.SatisfiedData(input.Static(name), input.False))
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
		if len(results) != 1 || results[0].Name != "apk.package:"+name || results[0].Status != reconcile.ReportOK {
			t.Fatalf("report = %#v, want one ok apk.package:%s", results, name)
		}
	})
}

func assertApkInstalled(t testing.TB, g *moduletest.Target, name string, want bool) {
	t.Helper()

	e := g.Ops().Exec("apk", "info", "-e", name).CaptureStdout()
	err := e.RunErr()
	if err != nil {
		if !modules.IsExitStatus(err, 1) {
			t.Fatalf("apk info -e %s: %v", name, err)
		}
		if want {
			t.Errorf("package %s is not installed", name)
		}
		return
	}

	if !want {
		t.Errorf("package %s is installed", name)
	}
}
