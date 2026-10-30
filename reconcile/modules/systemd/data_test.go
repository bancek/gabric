package systemd

import (
	"errors"
	"testing"

	"github.com/bancek/gabric/gabrictest"
	"github.com/bancek/gabric/input"
)

func TestEnabledData(t *testing.T) {
	tests := []struct {
		name   string
		status string
		exit   int
		want   bool
	}{
		{"enabled", "enabled\n", 0, true},
		{"disabled", "disabled\n", 1, false},
		{"not found", "not-found\n", 4, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gt := gabrictest.New(t)
			gt.Remote().Handle("systemctl", func(gabrictest.Command) gabrictest.Result {
				return gabrictest.Result{Stdout: tt.status, ExitCode: tt.exit}
			})

			if got := gt.RunNode(gt.Context(), EnabledData(input.Static("docker"))); got != tt.want {
				t.Errorf("EnabledData() = %v, want %v", got, tt.want)
			}

			gt.Remote().AssertCommands(gt.Command("systemctl", "is-enabled", "docker"))
		})
	}
}

func TestActiveData(t *testing.T) {
	tests := []struct {
		name   string
		status string
		exit   int
		want   bool
	}{
		{"active", "active\n", 0, true},
		{"inactive", "inactive\n", 3, false},
		{"not found", "inactive\n", 4, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gt := gabrictest.New(t)
			gt.Remote().Handle("systemctl", func(gabrictest.Command) gabrictest.Result {
				return gabrictest.Result{Stdout: tt.status, ExitCode: tt.exit}
			})

			if got := gt.RunNode(gt.Context(), ActiveData(input.Static("docker"))); got != tt.want {
				t.Errorf("ActiveData() = %v, want %v", got, tt.want)
			}

			gt.Remote().AssertCommands(gt.Command("systemctl", "is-active", "docker"))
		})
	}
}

func TestEnabledDataError(t *testing.T) {
	gt := gabrictest.New(t)
	gt.Remote().Handle("systemctl", func(gabrictest.Command) gabrictest.Result {
		return gabrictest.Result{ExitCode: 1}
	})

	if _, err := gt.RunNodeErr(gt.Context(), EnabledData(input.Static("docker"))); err == nil {
		t.Error("node error = nil, want error")
	}
}

func TestSystemdDataUnexpectedExit(t *testing.T) {
	gt := gabrictest.New(t)
	gt.Remote().Handle("systemctl", func(gabrictest.Command) gabrictest.Result {
		return gabrictest.Result{Stdout: "enabled\n", ExitCode: 2}
	})

	if _, err := gt.RunNodeErr(gt.Context(), EnabledData(input.Static("docker"))); err == nil {
		t.Error("EnabledData() error = nil, want error")
	}
	if _, err := gt.RunNodeErr(gt.Context(), ActiveData(input.Static("docker"))); err == nil {
		t.Error("ActiveData() error = nil, want error")
	}
}

func TestSystemdDataTransportErrors(t *testing.T) {
	sentinel := errors.New("transport")
	gt := gabrictest.New(t)
	ctx := gt.WithRemoteExecError(sentinel)

	t.Run("EnabledData", func(t *testing.T) {
		if _, err := gt.RunNodeErr(ctx, EnabledData(input.Static("docker"))); !errors.Is(err, sentinel) {
			t.Errorf("EnabledData() error = %v, want %v", err, sentinel)
		}
	})
	t.Run("ActiveData", func(t *testing.T) {
		if _, err := gt.RunNodeErr(ctx, ActiveData(input.Static("docker"))); !errors.Is(err, sentinel) {
			t.Errorf("ActiveData() error = %v, want %v", err, sentinel)
		}
	})
}
