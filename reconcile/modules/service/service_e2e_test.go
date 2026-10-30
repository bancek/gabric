package service_test

import (
	"testing"

	"github.com/bancek/gabric/input"
	"github.com/bancek/gabric/reconcile"
	"github.com/bancek/gabric/reconcile/modules/moduletest"
	"github.com/bancek/gabric/reconcile/modules/service"
	"github.com/bancek/gabric/reconcile/modules/systemd"
)

func TestServiceE2E(t *testing.T) {
	moduletest.ForEachSystemdTarget(t, func(g *moduletest.Target) {
		t := g.T()
		name := "cron"

		manager, err := g.RunNodeErr(g.Context(), service.ManagerData())
		if err != nil {
			t.Fatalf("ManagerData(): %v", err)
		}
		if manager != service.ManagerSystemd {
			t.Fatalf("ManagerData() = %v, want systemd", manager)
		}

		disabled := service.Service(input.Static(name)).Enabled(input.False).Build().Node()
		if _, err := g.RunReportRef(g.Context(), disabled); err != nil {
			t.Fatalf("disable %s: %v", name, err)
		}
		assertState(t, g, name, false, false)

		node := service.Service(input.Static(name)).Started().Build().Node()

		moduletest.AssertDryRun(t, g, node)
		moduletest.AssertConverges(t, g, node)
		assertState(t, g, name, true, true)

		results, err := g.RunReportRef(g.Context(), node)
		if err != nil {
			t.Fatalf("report: %v\n%s", err, reconcile.FormatReport(results))
		}
		if len(results) != 1 || results[0].Name != "service.service:"+name || results[0].Status != reconcile.ReportOK {
			t.Fatalf("report = %#v, want one ok service.service:%s", results, name)
		}
	})
}

func assertState(t testing.TB, g *moduletest.Target, name string, wantEnabled, wantActive bool) {
	t.Helper()

	enabled, err := g.RunNodeErr(g.Context(), systemd.EnabledData(input.Static(name)))
	if err != nil {
		t.Fatalf("EnabledData(%s): %v", name, err)
	}
	if enabled != wantEnabled {
		t.Errorf("%s enabled = %v, want %v", name, enabled, wantEnabled)
	}

	active, err := g.RunNodeErr(g.Context(), systemd.ActiveData(input.Static(name)))
	if err != nil {
		t.Fatalf("ActiveData(%s): %v", name, err)
	}
	if active != wantActive {
		t.Errorf("%s active = %v, want %v", name, active, wantActive)
	}
}
