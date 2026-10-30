package main

import (
	"errors"
	"testing"

	"github.com/bancek/gabric/gabrictest"
)

func TestComposeServiceRunning(t *testing.T) {
	gt := gabrictest.New(t)
	gt.Remote().Handle("docker", func(gabrictest.Command) gabrictest.Result {
		return gabrictest.Result{Stdout: "abc123\n"}
	})

	running, err := composeServiceRunning(gt.Context(), "/srv/www", "www")
	if err != nil {
		t.Fatalf("composeServiceRunning() error: %v", err)
	}
	if !running {
		t.Error("composeServiceRunning() = false, want true")
	}
}

func TestComposeServiceRunningUnexpectedExit(t *testing.T) {
	gt := gabrictest.New(t)
	gt.Remote().Handle("docker", func(gabrictest.Command) gabrictest.Result {
		return gabrictest.Result{ExitCode: 2}
	})

	if _, err := composeServiceRunning(gt.Context(), "/srv/www", "www"); err == nil {
		t.Error("composeServiceRunning() error = nil, want error")
	}
}

func TestComposeServiceRunningTransportError(t *testing.T) {
	sentinel := errors.New("transport")
	gt := gabrictest.New(t)
	ctx := gt.WithRemoteExecError(sentinel)

	if _, err := composeServiceRunning(ctx, "/srv/www", "www"); !errors.Is(err, sentinel) {
		t.Errorf("composeServiceRunning() error = %v, want %v", err, sentinel)
	}
}
