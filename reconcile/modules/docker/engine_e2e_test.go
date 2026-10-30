package docker_test

import (
	"strings"
	"testing"
	"time"

	"github.com/bancek/gabric/input"
	"github.com/bancek/gabric/internal/testutils"
	"github.com/bancek/gabric/reconcile/modules/docker"
	"github.com/bancek/gabric/reconcile/modules/moduletest"
)

func TestDockerEngineE2E(t *testing.T) {
	moduletest.ForEachSystemdTarget(t, func(g *moduletest.Target) {
		t := g.T()
		engine := docker.DockerEngine().Build()

		moduletest.AssertDryRun(t, g, engine)
		moduletest.AssertConverges(t, g, engine)

		testutils.Eventually(t, 60*time.Second, func() bool {
			return g.Ops().Exec("docker", "info").Capture().RunErr() == nil
		})

		e := g.Ops().Exec("systemctl", "is-active", "docker").CaptureStdout()
		if err := e.RunErr(); err != nil {
			t.Fatalf("docker service is not active: %v\n%s", err, e.StdoutStr())
		}
		if got := strings.TrimSpace(e.StdoutStr()); got != "active" {
			t.Errorf("docker is-active = %q, want active", got)
		}

		network := docker.Network(input.Static("reconcile-engine-e2e")).Node()
		moduletest.AssertConverges(t, g, network)
	})
}
