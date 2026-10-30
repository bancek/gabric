package service

import (
	"testing"

	"github.com/bancek/gabric"
	"github.com/bancek/gabric/gabrictest"
	"github.com/bancek/gabric/input"
)

func registerSystemd(t *testing.T, gt *gabrictest.Test, enabled, active string) {
	t.Helper()

	gt.Remote().MkdirAll("/run/systemd/system", 0o755)
	gt.Remote().Handle("systemctl", func(cmd gabrictest.Command) gabrictest.Result {
		switch {
		case cmd.HasArg("is-enabled"):
			return gabrictest.Result{Stdout: enabled}
		case cmd.HasArg("is-active"):
			return gabrictest.Result{Stdout: active}
		}

		return gabrictest.Result{}
	})
}

func TestServiceEnabled(t *testing.T) {
	gt := gabrictest.New(t)
	registerSystemd(t, gt, "enabled", "")

	gt.RunNode(gt.Context(), Service(input.Static("docker")).Build().Node())

	gt.Remote().AssertCommands(gt.Command("systemctl", "is-enabled", "docker"))
	gt.Remote().AssertCalls(existsCall("/run/systemd/system"))
}

func TestServiceDrift(t *testing.T) {
	gt := gabrictest.New(t)
	registerSystemd(t, gt, "disabled", "")

	gt.RunNode(gt.Context(), Service(input.Static("docker")).Build().Node())

	gt.Remote().AssertCommands(
		gt.Command("systemctl", "is-enabled", "docker"),
		gt.Command("systemctl", "enable", "docker"),
	)
	gt.Remote().AssertCalls(existsCall("/run/systemd/system"))
}

func TestServiceUnsupported(t *testing.T) {
	gt := gabrictest.New(t)

	if _, err := gt.RunNodeErr(gt.Context(), Service(input.Static("docker")).Build().Node()); err == nil {
		t.Error("Service() error = nil, want error")
	}
}

func existsCall(path string) gabrictest.Call {
	return gabrictest.Call{Exists: &gabric.ExistsRequest{Path: path, Kind: gabric.FileKindAny}}
}
