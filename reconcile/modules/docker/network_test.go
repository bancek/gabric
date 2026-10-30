package docker

import (
	"errors"
	"testing"

	"github.com/bancek/gabric/gabrictest"
	"github.com/bancek/gabric/input"
)

func TestNetworkExistsData(t *testing.T) {
	gt := gabrictest.New(t)
	gt.Remote().Handle("docker", func(gabrictest.Command) gabrictest.Result {
		return gabrictest.Result{}
	})

	if !gt.RunNode(gt.Context(), NetworkExistsData(input.Static("web"))) {
		t.Error("NetworkExistsData() = false, want true")
	}

	gt.Remote().AssertCommands(gt.Command("docker", "network", "inspect", "web"))
}

func TestNetworkExistsDataMissing(t *testing.T) {
	gt := gabrictest.New(t)
	gt.Remote().Handle("docker", func(gabrictest.Command) gabrictest.Result {
		return gabrictest.Result{ExitCode: 1}
	})

	if gt.RunNode(gt.Context(), NetworkExistsData(input.Static("web"))) {
		t.Error("NetworkExistsData() = true, want false")
	}

	gt.Remote().AssertCommands(gt.Command("docker", "network", "inspect", "web"))
}

func TestNetworkExistsDataUnexpectedExit(t *testing.T) {
	gt := gabrictest.New(t)
	gt.Remote().Handle("docker", func(gabrictest.Command) gabrictest.Result {
		return gabrictest.Result{ExitCode: 2}
	})

	if _, err := gt.RunNodeErr(gt.Context(), NetworkExistsData(input.Static("web"))); err == nil {
		t.Error("NetworkExistsData() error = nil, want error")
	}
}

func TestNetworkExistsDataTransportError(t *testing.T) {
	sentinel := errors.New("transport")
	gt := gabrictest.New(t)
	ctx := gt.WithRemoteExecError(sentinel)

	if _, err := gt.RunNodeErr(ctx, NetworkExistsData(input.Static("web"))); !errors.Is(err, sentinel) {
		t.Errorf("NetworkExistsData() error = %v, want %v", err, sentinel)
	}
}

func TestNetworkNoChange(t *testing.T) {
	gt := gabrictest.New(t)
	gt.Remote().Handle("docker", func(gabrictest.Command) gabrictest.Result {
		return gabrictest.Result{}
	})

	gt.RunNode(gt.Context(), Network(input.Static("web")).Node())

	gt.Remote().AssertCommands(gt.Command("docker", "network", "inspect", "web"))
	gt.Remote().AssertCalls()
}

func TestNetworkDrift(t *testing.T) {
	gt := gabrictest.New(t)
	gt.Remote().Handle("docker", func(cmd gabrictest.Command) gabrictest.Result {
		if cmd.HasArg("inspect") {
			return gabrictest.Result{ExitCode: 1}
		}

		return gabrictest.Result{}
	})

	gt.RunNode(gt.Context(), Network(input.Static("web")).Node())

	gt.Remote().AssertCommands(
		gt.Command("docker", "network", "inspect", "web"),
		gt.Command("docker", "network", "create", "web"),
	)
	gt.Remote().AssertCalls()
}
