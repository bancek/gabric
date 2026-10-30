package docker_test

import (
	"strings"
	"testing"

	"github.com/bancek/gabric/input"
	"github.com/bancek/gabric/reconcile/modules/docker"
	"github.com/bancek/gabric/reconcile/modules/moduletest"
)

func TestDockerE2E(t *testing.T) {
	moduletest.ForEachDockerTarget(t, moduletest.Dind, func(g *moduletest.Target) {
		t := g.T()
		const networkName = "reconcile-e2e"

		network := docker.Network(input.Static(networkName)).Node()

		moduletest.AssertDryRun(t, g, network)
		moduletest.AssertConverges(t, g, network)

		networkOut := dockerRun(t, g, "network", "inspect", networkName)
		if !strings.Contains(networkOut, networkName) {
			t.Errorf("network inspect = %q, want %q", networkOut, networkName)
		}

		composePath := g.Path("compose.yaml")
		putCompose(t, g, composePath, "1")
		if err := g.Ops().Exec("docker", "pull", "alpine:3.24").RunErr(); err != nil {
			t.Fatalf("docker pull: %v", err)
		}

		compose := docker.Compose(input.Static(composePath)).Build().Node()

		moduletest.AssertDryRun(t, g, compose)
		moduletest.AssertConverges(t, g, compose)

		assertComposeLabel(t, g, composePath, "1")

		putCompose(t, g, composePath, "2")
		moduletest.AssertConverges(t, g, compose)
		assertComposeLabel(t, g, composePath, "2")
	})
}

func putCompose(t testing.TB, g *moduletest.Target, path, revision string) {
	t.Helper()

	content := `services:
  app:
    image: alpine:3.24
    command: ["sleep", "300"]
    labels:
      revision: "` + revision + `"
`

	if err := g.Ops().Put(path).String(content).Mode(0o644).RunErr(); err != nil {
		t.Fatalf("write compose file: %v", err)
	}
}

func assertComposeLabel(t testing.TB, g *moduletest.Target, composePath, want string) {
	t.Helper()

	e := g.Ops().Exec("docker", "compose", "-f", composePath, "ps", "-q", "--status", "running").CaptureStdout()
	if err := e.RunErr(); err != nil {
		t.Fatalf("compose ps: %v", err)
	}

	ids := strings.Fields(e.StdoutStr())
	if len(ids) != 1 {
		t.Fatalf("compose ps = %q, want one running container", e.StdoutStr())
	}

	got := dockerRun(t, g, "inspect", "-f", `{{ index .Config.Labels "revision" }}`, ids[0])
	if got != want {
		t.Errorf("compose label = %q, want %q", got, want)
	}
}

func dockerRun(t testing.TB, g *moduletest.Target, args ...string) string {
	t.Helper()

	e := g.Ops().Exec(append([]string{"docker"}, args...)...).CaptureStdout()
	if err := e.RunErr(); err != nil {
		t.Fatalf("docker %s: %v\n%s", strings.Join(args, " "), err, e.StderrStr())
	}

	return strings.TrimSpace(e.StdoutStr())
}
