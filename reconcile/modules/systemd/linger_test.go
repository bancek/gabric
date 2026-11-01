package systemd

import (
	"testing"

	"github.com/bancek/gabric/gabrictest"
	"github.com/bancek/gabric/input"
)

func TestLingerData(t *testing.T) {
	tests := []struct {
		name   string
		output string
		exit   int
		want   bool
	}{
		{"enabled", "Linger=yes\n", 0, true},
		{"disabled", "Linger=no\n", 0, false},
		{"no session", "", 1, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gt := gabrictest.New(t)
			gt.Remote().Handle("loginctl", func(gabrictest.Command) gabrictest.Result {
				return gabrictest.Result{Stdout: tt.output, ExitCode: tt.exit}
			})

			if got := gt.RunNode(gt.Context(), LingerData(input.Static("app"))); got != tt.want {
				t.Errorf("LingerData() = %v, want %v", got, tt.want)
			}

			gt.Remote().AssertCommands(gt.Command("loginctl", "show-user", "app", "-p", "Linger"))
		})
	}
}

func TestLingerDataUnexpectedOutput(t *testing.T) {
	gt := gabrictest.New(t)
	gt.Remote().Handle("loginctl", func(gabrictest.Command) gabrictest.Result {
		return gabrictest.Result{Stdout: "unknown\n"}
	})

	if _, err := gt.RunNodeErr(gt.Context(), LingerData(input.Static("app"))); err == nil {
		t.Error("LingerData() error = nil, want error")
	}
}

func TestLingerDataError(t *testing.T) {
	gt := gabrictest.New(t)
	gt.Remote().Handle("loginctl", func(gabrictest.Command) gabrictest.Result {
		return gabrictest.Result{ExitCode: 2}
	})

	if _, err := gt.RunNodeErr(gt.Context(), LingerData(input.Static("app"))); err == nil {
		t.Error("LingerData() error = nil, want error")
	}
}

func TestLingerEnabled(t *testing.T) {
	gt := gabrictest.New(t)
	registerLoginctlHandler(gt, "Linger=yes\n")

	gt.RunNode(gt.Context(), Linger(input.Static("app")).Build().Node())

	gt.Remote().AssertCommands(gt.Command("loginctl", "show-user", "app", "-p", "Linger"))
	gt.Remote().AssertCalls()
}

func TestLingerDrift(t *testing.T) {
	gt := gabrictest.New(t)
	registerLoginctlHandler(gt, "Linger=no\n")

	gt.RunNode(gt.Context(), Linger(input.Static("app")).Build().Node())

	gt.Remote().AssertCommands(
		gt.Command("loginctl", "show-user", "app", "-p", "Linger"),
		gt.Command("loginctl", "enable-linger", "app"),
	)
	gt.Remote().AssertCalls()
}

func TestLingerDisabled(t *testing.T) {
	gt := gabrictest.New(t)
	registerLoginctlHandler(gt, "Linger=yes\n")

	gt.RunNode(gt.Context(), Linger(input.Static("app")).Enabled(input.False).Build().Node())

	gt.Remote().AssertCommands(
		gt.Command("loginctl", "show-user", "app", "-p", "Linger"),
		gt.Command("loginctl", "disable-linger", "app"),
	)
	gt.Remote().AssertCalls()
}

func TestLingerAlreadyDisabled(t *testing.T) {
	gt := gabrictest.New(t)
	registerLoginctlHandler(gt, "Linger=no\n")

	gt.RunNode(gt.Context(), Linger(input.Static("app")).Enabled(input.False).Build().Node())

	gt.Remote().AssertCommands(gt.Command("loginctl", "show-user", "app", "-p", "Linger"))
	gt.Remote().AssertCalls()
}

func TestLingerNames(t *testing.T) {
	if got, want := Linger(input.Static("app")).Build().Node().Name(), "systemd.linger:app"; got != want {
		t.Errorf("Linger() name = %q, want %q", got, want)
	}
	if got, want := LingerData(input.Static("app")).Name(), "systemd.linger-data:app"; got != want {
		t.Errorf("LingerData() name = %q, want %q", got, want)
	}
}

func registerLoginctlHandler(gt *gabrictest.Test, linger string) {
	gt.Remote().Handle("loginctl", func(cmd gabrictest.Command) gabrictest.Result {
		if cmd.HasArg("show-user") {
			return gabrictest.Result{Stdout: linger}
		}

		return gabrictest.Result{}
	})
}
