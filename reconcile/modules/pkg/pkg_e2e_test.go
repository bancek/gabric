package pkg_test

import (
	"strings"
	"testing"

	"github.com/bancek/gabric/input"
	"github.com/bancek/gabric/reconcile"
	"github.com/bancek/gabric/reconcile/modules"
	"github.com/bancek/gabric/reconcile/modules/moduletest"
	"github.com/bancek/gabric/reconcile/modules/pkg"
)

func TestPkgE2E(t *testing.T) {
	moduletest.ForEachUbuntuTarget(t, func(g *moduletest.Target) {
		assertPkg(g.T(), g, pkg.ManagerApt)
	})
	moduletest.ForEachAlpineTarget(t, func(g *moduletest.Target) {
		assertPkg(g.T(), g, pkg.ManagerApk)
	})
}

func assertPkg(t testing.TB, g *moduletest.Target, want pkg.Manager) {
	t.Helper()

	name := "curl"

	manager, err := g.RunNodeErr(g.Context(), pkg.ManagerData())
	if err != nil {
		t.Fatalf("read package manager: %v", err)
	}
	if manager != want {
		t.Fatalf("package manager = %v, want %v", manager, want)
	}

	node := pkg.Package(input.Static(name)).Build().Node()

	moduletest.AssertDryRun(t, g, node)
	assertPkgInstalled(t, g, manager, name, false)

	moduletest.AssertConverges(t, g, node)
	assertPkgInstalled(t, g, manager, name, true)

	results, err := g.RunReportRef(g.Context(), node)
	if err != nil {
		t.Fatalf("report: %v\n%s", err, reconcile.FormatReport(results))
	}
	if len(results) != 1 || results[0].Name != "pkg.package:"+name || results[0].Status != reconcile.ReportOK {
		t.Fatalf("report = %#v, want one ok pkg.package:%s", results, name)
	}
}

func assertPkgInstalled(t testing.TB, g *moduletest.Target, manager pkg.Manager, name string, want bool) {
	t.Helper()

	switch manager {
	case pkg.ManagerApt:
		assertDpkgInstalled(t, g, name, want)
	case pkg.ManagerApk:
		assertApkInstalled(t, g, name, want)
	default:
		t.Fatalf("unsupported package manager %v", manager)
	}
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
