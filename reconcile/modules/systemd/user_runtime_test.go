package systemd

import (
	"context"
	"testing"

	"github.com/bancek/gabric"
	"github.com/bancek/gabric/gabrictest"
	"github.com/bancek/gabric/input"
	"github.com/bancek/gabric/reconcile"
)

func TestUserRuntimeDir(t *testing.T) {
	gt := gabrictest.New(t)
	gt.Remote().Handle("loginctl", func(gabrictest.Command) gabrictest.Result {
		return gabrictest.Result{Stdout: "RuntimePath=/run/user/1001\n"}
	})

	got := gt.RunNode(gt.Context(), UserRuntimeDir(input.Static("app")))
	if want := "/run/user/1001"; got != want {
		t.Errorf("UserRuntimeDir() = %q, want %q", got, want)
	}

	gt.Remote().AssertCommands(gt.Command("loginctl", "show-user", "app", "-p", "RuntimePath"))
}

func TestUserRuntimeDirUnexpectedOutput(t *testing.T) {
	gt := gabrictest.New(t)
	gt.Remote().Handle("loginctl", func(gabrictest.Command) gabrictest.Result {
		return gabrictest.Result{Stdout: "unknown\n"}
	})

	if _, err := gt.RunNodeErr(gt.Context(), UserRuntimeDir(input.Static("app"))); err == nil {
		t.Error("UserRuntimeDir() error = nil, want error")
	}
}

func TestUserRuntimeDirEmptyPath(t *testing.T) {
	gt := gabrictest.New(t)
	gt.Remote().Handle("loginctl", func(gabrictest.Command) gabrictest.Result {
		return gabrictest.Result{Stdout: "RuntimePath=\n"}
	})

	if _, err := gt.RunNodeErr(gt.Context(), UserRuntimeDir(input.Static("app"))); err == nil {
		t.Error("UserRuntimeDir() error = nil, want error")
	}
}

func TestUserRuntimeDirError(t *testing.T) {
	gt := gabrictest.New(t)
	gt.Remote().Handle("loginctl", func(gabrictest.Command) gabrictest.Result {
		return gabrictest.Result{ExitCode: 1}
	})

	if _, err := gt.RunNodeErr(gt.Context(), UserRuntimeDir(input.Static("app"))); err == nil {
		t.Error("UserRuntimeDir() error = nil, want error")
	}
}

func TestUserRuntimeDirName(t *testing.T) {
	if got, want := UserRuntimeDir(input.Static("app")).Name(), "systemd.user-runtime-dir:app"; got != want {
		t.Errorf("UserRuntimeDir() name = %q, want %q", got, want)
	}
}

func TestWithUserRuntimeDir(t *testing.T) {
	gt := gabrictest.New(t)
	gt.Remote().Handle("loginctl", func(gabrictest.Command) gabrictest.Result {
		return gabrictest.Result{Stdout: "RuntimePath=/run/user/1001\n"}
	})
	gt.Remote().Handle("systemctl", func(gabrictest.Command) gabrictest.Result {
		return gabrictest.Result{}
	})

	node := reconcile.Data("probe", func(ctx context.Context) (bool, error) {
		return true, gabric.TargetOps(ctx).Exec("systemctl", "is-enabled", "app.service").RunErr()
	}).Node().With(WithUserRuntimeDir(input.Static("app")))

	gt.RunNode(gt.Context(), node)

	gt.Remote().AssertCommands(
		gt.Command("loginctl", "show-user", "app", "-p", "RuntimePath"),
		gt.Command("systemctl", "is-enabled", "app.service").WithEnv("XDG_RUNTIME_DIR=/run/user/1001"),
	)
}
