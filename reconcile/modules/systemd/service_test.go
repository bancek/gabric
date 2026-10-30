package systemd

import (
	"testing"

	"github.com/bancek/gabric/gabrictest"
	"github.com/bancek/gabric/input"
)

func TestServiceEnabled(t *testing.T) {
	gt := gabrictest.New(t)
	registerSystemctlHandler(gt, "enabled", "")

	gt.RunNode(gt.Context(), Service(input.Static("docker")).Build().Node())

	gt.Remote().AssertCommands(gt.Command("systemctl", "is-enabled", "docker"))
	gt.Remote().AssertCalls()
}

func TestServiceDrift(t *testing.T) {
	gt := gabrictest.New(t)
	registerSystemctlHandler(gt, "disabled", "")

	gt.RunNode(gt.Context(), Service(input.Static("docker")).Build().Node())

	gt.Remote().AssertCommands(
		gt.Command("systemctl", "is-enabled", "docker"),
		gt.Command("systemctl", "enable", "docker"),
	)
	gt.Remote().AssertCalls()
}

func TestServiceStartNotActive(t *testing.T) {
	gt := gabrictest.New(t)
	registerSystemctlHandler(gt, "enabled", "inactive")

	gt.RunNode(gt.Context(), Service(input.Static("docker")).Started().Build().Node())

	gt.Remote().AssertCommands(
		gt.Command("systemctl", "is-enabled", "docker"),
		gt.Command("systemctl", "is-active", "docker"),
		gt.Command("systemctl", "enable", "--now", "docker"),
	)
	gt.Remote().AssertCalls()
}

func TestServiceDisabled(t *testing.T) {
	gt := gabrictest.New(t)
	registerSystemctlHandler(gt, "enabled", "")

	gt.RunNode(gt.Context(), Service(input.Static("docker")).Enabled(input.False).Build().Node())

	gt.Remote().AssertCommands(
		gt.Command("systemctl", "is-enabled", "docker"),
		gt.Command("systemctl", "disable", "--now", "docker"),
	)
	gt.Remote().AssertCalls()
}
