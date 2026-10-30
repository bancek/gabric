package ufw_test

import (
	"strings"
	"testing"

	"github.com/bancek/gabric/input"
	"github.com/bancek/gabric/reconcile"
	"github.com/bancek/gabric/reconcile/modules/moduletest"
	"github.com/bancek/gabric/reconcile/modules/ufw"
)

func TestUFWE2E(t *testing.T) {
	moduletest.ForEachSystemdTarget(t, func(g *moduletest.Target) {
		t := g.T()

		fw := ufw.Firewall().
			Rule(ufw.Rule().Port(input.Static("22")).Proto(input.Static("tcp"))).
			Incoming(input.Static("deny")).
			Outgoing(input.Static("allow")).
			Enabled().
			Build()

		moduletest.AssertDryRun(t, g, fw)
		moduletest.AssertConverges(t, g, fw)

		status := ufwStatus(t, g)
		if !strings.Contains(status, "Status: active") {
			t.Errorf("ufw status is not active:\n%s", status)
		}
		if !strings.Contains(status, "Default: deny (incoming), allow (outgoing)") {
			t.Errorf("ufw defaults are wrong:\n%s", status)
		}
		if !strings.Contains(status, "22/tcp") || !strings.Contains(status, "ALLOW") {
			t.Errorf("ufw rule 22/tcp is missing:\n%s", status)
		}

		added := showAdded(t, g)
		if !strings.Contains(added, "ufw allow 22/tcp") {
			t.Errorf("ufw show added = %q, want allow 22/tcp", added)
		}

		info, err := g.RunNodeErr(g.Context(), ufw.StatusData())
		if err != nil {
			t.Fatalf("StatusData(): %v", err)
		}
		if !info.Active || info.Incoming != "deny" || info.Outgoing != "allow" {
			t.Errorf("StatusData() = %+v", info)
		}

		addedData, err := g.RunNodeErr(g.Context(), ufw.ShowAddedData())
		if err != nil {
			t.Fatalf("ShowAddedData(): %v", err)
		}
		if !strings.Contains(addedData, "ufw allow 22/tcp") {
			t.Errorf("ShowAddedData() = %q, want allow 22/tcp", addedData)
		}

		extra := fw.AddRule(ufw.Rule().Port(input.Static("80")).Proto(input.Static("tcp")))
		group := reconcile.Group("ufw-e2e-add").
			Contains(extra, fw.Node).
			Node()

		if _, err := g.RunNodeErr(g.Context(), group); err != nil {
			t.Fatalf("add rule: %v", err)
		}
		if added := showAdded(t, g); !strings.Contains(added, "ufw allow 80/tcp") {
			t.Errorf("ufw show added after add = %q, want allow 80/tcp", added)
		}
	})
}

func ufwStatus(t testing.TB, g *moduletest.Target) string {
	t.Helper()

	e := g.Ops().Exec("ufw", "status", "verbose").CaptureStdout()
	if err := e.RunErr(); err != nil {
		t.Fatalf("ufw status verbose: %v", err)
	}

	return e.StdoutStr()
}

func showAdded(t testing.TB, g *moduletest.Target) string {
	t.Helper()

	e := g.Ops().Exec("ufw", "show", "added").CaptureStdout()
	if err := e.RunErr(); err != nil {
		t.Fatalf("ufw show added: %v", err)
	}

	return e.StdoutStr()
}
