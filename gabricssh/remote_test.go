//go:build unix

package gabricssh

import (
	"context"
	"fmt"
	"testing"

	"github.com/bancek/gabric"
	"github.com/bancek/gabric/internal/sshtest"
	"github.com/bancek/gabric/sshremote"
)

func TestRemote(t *testing.T) {
	srv := startSSHServer(t, sshtest.WithExecHandler(execLocally))
	ctx := remoteServerContext(t, srv, false)

	host := fmt.Sprintf("%s@127.0.0.1:%d", srv.User(), srv.Port())
	out := Remote(ctx, host).Exec("echo", "hi").CaptureStdout().Run().StdoutStr()

	if out != "hi\n" {
		t.Errorf("stdout = %q, want %q", out, "hi\n")
	}
}

func TestRemotePanicsWithoutConfig(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("Remote() without config did not panic")
		}
	}()

	Remote(testContext(t), "example.com")
}

func TestRemotePanicsWithoutManager(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("Remote() without manager did not panic")
		}
	}()

	ctx := NewConfigContext(testContext(t), sshremote.NewConfig())
	Remote(ctx, "example.com")
}

func TestTarget(t *testing.T) {
	srv := startSSHServer(t, sshtest.WithExecHandler(execLocally))
	ctx := remoteServerContext(t, srv, false)

	host := fmt.Sprintf("%s@127.0.0.1:%d", srv.User(), srv.Port())
	ctx = gabric.NewTargetContext(ctx, Target(host))

	out := gabric.TargetOps(ctx).Exec("echo", "hi").CaptureStdout().Run().StdoutStr()
	if out != "hi\n" {
		t.Errorf("stdout = %q, want %q", out, "hi\n")
	}
}

func TestWithRemote(t *testing.T) {
	srv := startSSHServer(t, sshtest.WithExecHandler(execLocally))

	config := sshremote.NewConfig().
		WithStrictHostKeyChecking(sshremote.StrictHostKeyCheckingNo).
		WithConnectionTimeout(sshremote.DefaultConnectionTimeout).
		WithPrivateKeysPaths([]string{srv.ClientKeyPath()})
	ctx := NewConfigContext(testContext(t), config)

	host := fmt.Sprintf("%s@127.0.0.1:%d", srv.User(), srv.Port())

	if err := WithRemote(ctx, host, func(ctx context.Context) error {
		out := gabric.Remote(ctx).Exec("echo", "hi").CaptureStdout().Run().StdoutStr()
		if out != "hi\n" {
			t.Errorf("stdout = %q, want %q", out, "hi\n")
		}

		return nil
	}); err != nil {
		t.Fatalf("WithRemote() error: %v", err)
	}
}

func TestRunner(t *testing.T) {
	if Runner() == nil {
		t.Error("Runner() = nil")
	}
}
