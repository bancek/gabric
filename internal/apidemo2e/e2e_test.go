package apidemo2e

import (
	"os/exec"
	"testing"

	"github.com/bancek/gabric/examples/apidemo"
	"github.com/bancek/gabric/gabrice2e"
	"github.com/bancek/gabric/internal/e2eimages"
)

func TestMain(m *testing.M) {
	gabrice2e.Main(m)
}

func TestAPIDemo(t *testing.T) {
	requireHostRsync(t)

	t.Run("docker", func(t *testing.T) {
		g := gabrice2e.Docker(t, e2eimages.DockerRsync)
		apidemo.Run(g.Context())
	})

	t.Run("ssh", func(t *testing.T) {
		g := gabrice2e.SSH(t, e2eimages.DockerSSH, "gabric")
		apidemo.Run(g.Context())
	})
}

func requireHostRsync(t testing.TB) {
	t.Helper()

	if _, err := exec.LookPath("rsync"); err != nil {
		t.Skip("rsync unavailable")
	}
}
