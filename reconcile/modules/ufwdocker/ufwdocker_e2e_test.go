package ufwdocker_test

import (
	"strings"
	"testing"

	"github.com/bancek/gabric/input"
	"github.com/bancek/gabric/reconcile/modules/moduletest"
	"github.com/bancek/gabric/reconcile/modules/systemd"
	"github.com/bancek/gabric/reconcile/modules/ufwdocker"
)

func TestUfwDockerE2E(t *testing.T) {
	moduletest.ForEachUfwDockerTarget(t, func(g *moduletest.Target) {
		t := g.T()

		// Latest ufw-docker master commit at the time of writing.
		const ref = "020a8699f95592561f254d8d4ad1bb40d401dfc7"

		if err := g.Ops().Exec("ufw", "--force", "enable").RunErr(); err != nil {
			t.Fatalf("enable ufw: %v", err)
		}
		if err := g.Ops().Exec("systemctl", "start", "ufw").RunErr(); err != nil {
			t.Fatalf("start ufw: %v", err)
		}

		_ = ufwdocker.RestartEffect()
		ufwDocker := ufwdocker.UfwDocker(input.Static(ref)).Build()

		moduletest.AssertDryRun(t, g, ufwDocker)
		moduletest.AssertConverges(t, g, ufwDocker)

		assertUFWDockerCheck(t, g)
		assertUFWDockerRules(t, g)
		assertActive(t, g, "docker")
		assertActive(t, g, "ufw-docker")
	})
}

func assertUFWDockerCheck(t testing.TB, g *moduletest.Target) {
	t.Helper()

	e := g.Ops().Exec("ufw-docker", "check").Capture()
	if err := e.RunErr(); err != nil {
		t.Fatalf("ufw-docker check: %v\nstdout:\n%s\nstderr:\n%s",
			err, e.StdoutStr(), e.StderrStr())
	}
}

func assertUFWDockerRules(t testing.TB, g *moduletest.Target) {
	t.Helper()

	e := g.Ops().Exec("iptables", "-S", "DOCKER-USER").Capture()
	if err := e.RunErr(); err != nil {
		t.Fatalf("iptables -S DOCKER-USER: %v\nstderr:\n%s", err, e.StderrStr())
	}

	rules := e.StdoutStr()
	for _, want := range []string{
		"ufw-user-forward",
		"ufw-docker-logging-deny",
	} {
		if !strings.Contains(rules, want) {
			t.Errorf("DOCKER-USER missing %q:\n%s", want, rules)
		}
	}
}

func assertActive(t testing.TB, g *moduletest.Target, name string) {
	t.Helper()

	active, err := g.RunNodeErr(g.Context(), systemd.ActiveData(input.Static(name)))
	if err != nil {
		t.Fatalf("ActiveData(%s): %v", name, err)
	}
	if !active {
		t.Errorf("%s is not active", name)
	}
}
