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

func TestUserServiceEnabled(t *testing.T) {
	gt := gabrictest.New(t)
	registerSystemctlHandler(gt, "enabled", "")

	gt.RunNode(gt.Context(), Service(input.Static("app.service")).User(input.True).Build().Node())

	gt.Remote().AssertCommands(gt.Command("systemctl", "--user", "is-enabled", "app.service"))
	gt.Remote().AssertCalls()
}

func TestUserServiceFlagFalse(t *testing.T) {
	gt := gabrictest.New(t)
	registerSystemctlHandler(gt, "enabled", "")

	gt.RunNode(gt.Context(), Service(input.Static("app.service")).User(input.False).Build().Node())

	gt.Remote().AssertCommands(gt.Command("systemctl", "is-enabled", "app.service"))
	gt.Remote().AssertCalls()
}

func TestUserServiceDrift(t *testing.T) {
	gt := gabrictest.New(t)
	registerSystemctlHandler(gt, "disabled", "")

	gt.RunNode(gt.Context(), Service(input.Static("app.service")).User().Build().Node())

	gt.Remote().AssertCommands(
		gt.Command("systemctl", "--user", "is-enabled", "app.service"),
		gt.Command("systemctl", "--user", "enable", "app.service"),
	)
	gt.Remote().AssertCalls()
}

func TestUserServiceStartNotActive(t *testing.T) {
	gt := gabrictest.New(t)
	registerSystemctlHandler(gt, "enabled", "inactive")

	gt.RunNode(gt.Context(), Service(input.Static("app.service")).User().Started().Build().Node())

	gt.Remote().AssertCommands(
		gt.Command("systemctl", "--user", "is-enabled", "app.service"),
		gt.Command("systemctl", "--user", "is-active", "app.service"),
		gt.Command("systemctl", "--user", "enable", "--now", "app.service"),
	)
	gt.Remote().AssertCalls()
}

func TestUserServiceDisabled(t *testing.T) {
	gt := gabrictest.New(t)
	registerSystemctlHandler(gt, "enabled", "")

	gt.RunNode(gt.Context(), Service(input.Static("app.service")).User().Enabled(input.False).Build().Node())

	gt.Remote().AssertCommands(
		gt.Command("systemctl", "--user", "is-enabled", "app.service"),
		gt.Command("systemctl", "--user", "disable", "--now", "app.service"),
	)
	gt.Remote().AssertCalls()
}

func TestServiceNames(t *testing.T) {
	system := Service(input.Static("app.service")).Build().Node()
	user := Service(input.Static("app.service")).User().Build().Node()

	if got, want := system.Name(), "systemd.service:app.service"; got != want {
		t.Errorf("system name = %q, want %q", got, want)
	}
	if got, want := user.Name(), "systemd.service:app.service"; got != want {
		t.Errorf("user name = %q, want %q", got, want)
	}

	if got, want := SatisfiedData(input.Static("app.service"), nil, nil).Name(), "systemd.satisfied:app.service"; got != want {
		t.Errorf("system satisfied name = %q, want %q", got, want)
	}
	if got, want := UserSatisfiedData(input.Static("app.service"), nil, nil).Name(), "systemd.user-satisfied:app.service"; got != want {
		t.Errorf("user satisfied name = %q, want %q", got, want)
	}
}

func TestUserSatisfiedData(t *testing.T) {
	gt := gabrictest.New(t)
	registerSystemctlHandler(gt, "enabled", "active")

	if got := gt.RunNode(gt.Context(), UserSatisfiedData(input.Static("app.service"), nil, input.True)); !got {
		t.Error("UserSatisfiedData() = false, want true")
	}

	gt.Remote().AssertCommands(
		gt.Command("systemctl", "--user", "is-enabled", "app.service"),
		gt.Command("systemctl", "--user", "is-active", "app.service"),
	)
}
