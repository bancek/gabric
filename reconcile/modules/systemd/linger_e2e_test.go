package systemd_test

import (
	"testing"

	"github.com/bancek/gabric/input"
	"github.com/bancek/gabric/reconcile/modules/moduletest"
	"github.com/bancek/gabric/reconcile/modules/systemd"
)

func TestLingerE2E(t *testing.T) {
	moduletest.ForEachSystemdTarget(t, func(g *moduletest.Target) {
		t := g.T()
		user := "gabric-linger-e2e"
		ensureUser(t, g, user)

		waitForLogind(t, g)

		moduletest.AssertConverges(t, g, systemd.Linger(input.Static(user)).Build().Node())

		enabled, err := g.RunNodeErr(g.Context(), systemd.LingerData(input.Static(user)))
		if err != nil {
			t.Fatalf("LingerData(): %v", err)
		}
		if !enabled {
			t.Error("LingerData() = false, want true")
		}

		moduletest.AssertConverges(t, g, systemd.Linger(input.Static(user)).Enabled(input.False).Build().Node())

		enabled, err = g.RunNodeErr(g.Context(), systemd.LingerData(input.Static(user)))
		if err != nil {
			t.Fatalf("LingerData(): %v", err)
		}
		if enabled {
			t.Error("LingerData() = true, want false")
		}
	})
}
