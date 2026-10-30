package ufwdocker

import (
	"bytes"
	"context"
	"errors"
	"testing"

	"github.com/bancek/gabric"
	"github.com/bancek/gabric/gabrictest"
	"github.com/bancek/gabric/input"
	"github.com/bancek/gabric/reconcile"
)

func dummyScript() *reconcile.Node[reconcile.Void] {
	return reconcile.Data("script", func(ctx context.Context) (reconcile.Void, error) {
		return reconcile.Void{}, nil
	}).Node()
}

func TestInstalledData(t *testing.T) {
	gt := gabrictest.New(t)
	gt.Remote().Handle("grep", func(gabrictest.Command) gabrictest.Result {
		return gabrictest.Result{}
	})

	if !gt.RunNode(gt.Context(), InstalledData()) {
		t.Error("InstalledData() = false, want true")
	}

	gt.Remote().AssertCommands(gt.Command("grep", "-q", ufwDockerMarker, ufwDockerAfterRules))
}

func TestInstalledDataMissing(t *testing.T) {
	gt := gabrictest.New(t)
	gt.Remote().Handle("grep", func(gabrictest.Command) gabrictest.Result {
		return gabrictest.Result{ExitCode: 1}
	})

	if gt.RunNode(gt.Context(), InstalledData()) {
		t.Error("InstalledData() = true, want false")
	}

	gt.Remote().AssertCommands(gt.Command("grep", "-q", ufwDockerMarker, ufwDockerAfterRules))
}

func TestInstalledDataUnexpectedExit(t *testing.T) {
	gt := gabrictest.New(t)
	gt.Remote().Handle("grep", func(gabrictest.Command) gabrictest.Result {
		return gabrictest.Result{ExitCode: 2}
	})

	if _, err := gt.RunNodeErr(gt.Context(), InstalledData()); err == nil {
		t.Error("InstalledData() error = nil, want error")
	}
}

func TestInstalledDataTransportError(t *testing.T) {
	sentinel := errors.New("transport")
	gt := gabrictest.New(t)
	ctx := gt.WithRemoteExecError(sentinel)

	if _, err := gt.RunNodeErr(ctx, InstalledData()); !errors.Is(err, sentinel) {
		t.Errorf("InstalledData() error = %v, want %v", err, sentinel)
	}
}

func TestInstallNoChange(t *testing.T) {
	gt := gabrictest.New(t)
	gt.Remote().Handle("grep", func(gabrictest.Command) gabrictest.Result {
		return gabrictest.Result{}
	})

	gt.RunNode(gt.Context(), Install(dummyScript()).Node())

	gt.Remote().AssertCommands(
		gt.Command("grep", "-q", ufwDockerMarker, ufwDockerAfterRules),
	)
	gt.Remote().AssertCalls()
}

func TestInstallDrift(t *testing.T) {
	gt := gabrictest.New(t)
	gt.Remote().Handle("grep", func(gabrictest.Command) gabrictest.Result {
		return gabrictest.Result{ExitCode: 1}
	})
	gt.Remote().Handle("ufw-docker", func(gabrictest.Command) gabrictest.Result {
		return gabrictest.Result{}
	})

	gt.RunNode(gt.Context(), Install(dummyScript()).Node())

	gt.Remote().AssertCommands(
		gt.Command("grep", "-q", ufwDockerMarker, ufwDockerAfterRules),
		gt.Command("ufw-docker", "install"),
	)
	gt.Remote().AssertCalls()
}

func TestInstallNotifies(t *testing.T) {
	old := Topic
	Topic = reconcile.NewTopic("ufw-test")
	t.Cleanup(func() { Topic = old })

	_ = RestartEffect()

	gt := gabrictest.New(t)
	gt.Remote().Handle("grep", func(gabrictest.Command) gabrictest.Result {
		return gabrictest.Result{ExitCode: 1}
	})
	gt.Remote().Handle("ufw-docker", func(gabrictest.Command) gabrictest.Result {
		return gabrictest.Result{}
	})
	gt.Remote().Handle("systemctl", func(gabrictest.Command) gabrictest.Result {
		return gabrictest.Result{}
	})

	gt.RunNode(gt.Context(), Install(dummyScript()).Node())

	gt.Remote().AssertCommands(
		gt.Command("grep", "-q", ufwDockerMarker, ufwDockerAfterRules),
		gt.Command("ufw-docker", "install"),
		gt.Command("systemctl", "restart", "ufw"),
	)
	gt.Remote().AssertCalls()
}

func TestInstallServiceNoChange(t *testing.T) {
	gt := gabrictest.New(t)
	gt.Remote().WriteFile(ufwDockerServicePath, []byte("x"), 0o644)

	gt.RunNode(gt.Context(), InstallService(dummyScript()).Node())

	gt.Remote().AssertCommands()
	gt.Remote().AssertCalls(gabrictest.Call{Exists: &gabric.ExistsRequest{
		Path: ufwDockerServicePath,
		Kind: gabric.FileKindFile,
	}})
}

func TestInstallServiceDrift(t *testing.T) {
	gt := gabrictest.New(t)
	gt.Remote().Handle("ufw-docker", func(gabrictest.Command) gabrictest.Result {
		return gabrictest.Result{}
	})

	gt.RunNode(gt.Context(), InstallService(dummyScript()).Node())

	gt.Remote().AssertCommands(gt.Command("ufw-docker", "install-service"))
	gt.Remote().AssertCalls(gabrictest.Call{Exists: &gabric.ExistsRequest{
		Path: ufwDockerServicePath,
		Kind: gabric.FileKindFile,
	}})
}

func TestUfwDocker(t *testing.T) {
	gt := gabrictest.New(t)
	registerCurlDownload(gt, ufwDockerScriptPath+".tmp", []byte("script"))
	gt.Remote().Handle("grep", func(gabrictest.Command) gabrictest.Result {
		return gabrictest.Result{ExitCode: 1}
	})
	gt.Remote().Handle("ufw-docker", func(gabrictest.Command) gabrictest.Result {
		return gabrictest.Result{}
	})

	ufw := UfwDocker(input.Static("abc123")).Build()
	gt.RunNode(gt.Context(), ufw.Node)

	gt.Remote().AssertCommands(
		gt.Command("curl", "-fsSL",
			"https://raw.githubusercontent.com/chaifeng/ufw-docker/abc123/ufw-docker",
			"-o", ufwDockerScriptPath+".tmp"),
		gt.Command("mv", ufwDockerScriptPath+".tmp", ufwDockerScriptPath),
		gt.Command("chmod", "755", ufwDockerScriptPath),
		gt.Command("sha256sum", ufwDockerScriptPath+".ref"),
		gt.Command("grep", "-q", ufwDockerMarker, ufwDockerAfterRules),
		gt.Command("ufw-docker", "install"),
		gt.Command("ufw-docker", "install-service"),
	)
	gt.Remote().AssertCalls(
		gabrictest.Call{Get: &gabric.GetRequest{Path: ufwDockerScriptPath + ".ref"}},
		gabrictest.Call{Mkdir: &gabric.MkdirRequest{
			Path:      "/usr/local/bin",
			Recursive: true,
		}},
		existsCall(ufwDockerScriptPath+".ref", gabric.FileKindAny),
		gabrictest.Call{Put: &gabric.PutRequest{
			Path:         ufwDockerScriptPath + ".ref",
			Content:      bytes.NewBufferString("abc123"),
			Mode:         0o644,
			EnsureParent: true,
		}},
		gabrictest.Call{Exists: &gabric.ExistsRequest{
			Path: ufwDockerServicePath,
			Kind: gabric.FileKindFile,
		}},
	)
}
