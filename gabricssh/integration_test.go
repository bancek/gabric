//go:build unix

package gabricssh

import (
	"context"
	"errors"
	"strings"
	"testing"

	"golang.org/x/crypto/ssh"

	"github.com/bancek/gabric"
	"github.com/bancek/gabric/internal/shelltest"
	"github.com/bancek/gabric/internal/sshtest"
	"github.com/bancek/gabric/sshremote"
)

func newSSHContext(t *testing.T, core *shelltest.Core) context.Context {
	t.Helper()

	srv, err := sshtest.Start(sshtest.WithExecHandler(core.ExecHandler(context.Background())))
	if err != nil {
		t.Fatalf("sshtest.Start() error: %v", err)
	}
	t.Cleanup(func() { srv.Close() })

	ctx := testContext(t)

	config := sshremote.NewConfig().
		WithStrictHostKeyChecking(sshremote.StrictHostKeyCheckingNo).
		WithConnectionTimeout(sshremote.DefaultConnectionTimeout)
	ctx = NewConfigContext(ctx, config)

	manager := sshremote.NewRemoteManager()
	t.Cleanup(manager.Close)
	ctx = NewManagerContext(ctx, manager)

	host := sshremote.NewHost("127.0.0.1").
		WithPort(srv.Port()).
		WithUser(srv.User()).
		WithPrivateKeysPaths([]string{srv.ClientKeyPath()}).
		WithStrictHostKeyChecking(sshremote.StrictHostKeyCheckingNo)
	ctx = NewHostContext(ctx, host)

	return NewRunnerContext(ctx)
}

func TestIntegrationExec(t *testing.T) {
	core := shelltest.New()
	assertions := shelltest.NewAssertions(t, core.Commands)
	ctx := newSSHContext(t, core)

	out := gabric.Remote(ctx).Exec("echo", "hi").CaptureStdout().Run().StdoutStr()
	if got, want := out, "hi\n"; got != want {
		t.Errorf("stdout = %q, want %q", got, want)
	}

	assertions.AssertCommands(shelltest.Command{
		Name: "echo",
		Args: []string{"hi"},
		Dir:  "/",
	})
}

func TestIntegrationSudo(t *testing.T) {
	core := shelltest.New()
	assertions := shelltest.NewAssertions(t, core.Commands)
	ctx := newSSHContext(t, core)
	ctx = gabric.NewRemoteSudoContext(ctx, &gabric.Sudo{})

	out := gabric.Remote(ctx).Exec("echo", "hi").CaptureStdout().Run().StdoutStr()
	if got, want := out, "hi\n"; got != want {
		t.Errorf("stdout = %q, want %q", got, want)
	}

	assertions.AssertCommands(shelltest.Command{
		Name: "echo",
		Args: []string{"hi"},
		Dir:  "/",
		Sudo: &shelltest.Sudo{},
	})
}

func TestIntegrationNestedSudoSh(t *testing.T) {
	core := shelltest.New()
	assertions := shelltest.NewAssertions(t, core.Commands)
	ctx := newSSHContext(t, core)
	ctx = gabric.NewRemoteSudoContext(ctx, &gabric.Sudo{})

	out := gabric.Remote(ctx).Exec("sh", "-c", `sh -c "echo nested"`).CaptureStdout().Run().StdoutStr()
	if got, want := out, "nested\n"; got != want {
		t.Errorf("stdout = %q, want %q", got, want)
	}

	assertions.AssertCommands(shelltest.Command{
		Name: "echo",
		Args: []string{"nested"},
		Dir:  "/",
		Sudo: &shelltest.Sudo{},
	})
}

func TestIntegrationCat(t *testing.T) {
	core := shelltest.New()
	assertions := shelltest.NewAssertions(t, core.Commands)

	if err := core.FS().MkdirAll("/srv/app", 0o755); err != nil {
		t.Fatalf("MkdirAll() error: %v", err)
	}
	if err := core.FS().WriteFile("/srv/app/config", []byte("hello"), 0o644); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}

	ctx := newSSHContext(t, core)

	out := gabric.Remote(ctx).Exec("cat", "/srv/app/config").CaptureStdout().Run().StdoutStr()
	if got, want := out, "hello"; got != want {
		t.Errorf("stdout = %q, want %q", got, want)
	}

	assertions.AssertCommands(shelltest.Command{
		Name: "cat",
		Args: []string{"/srv/app/config"},
		Dir:  "/",
	})
}

func TestIntegrationCwd(t *testing.T) {
	core := shelltest.New()
	assertions := shelltest.NewAssertions(t, core.Commands)

	if err := core.FS().MkdirAll("/srv/app", 0o755); err != nil {
		t.Fatalf("MkdirAll() error: %v", err)
	}

	ctx := newSSHContext(t, core)

	var out string

	gabric.Remote(ctx).WithWorkingDir("/srv/app", func(ctx context.Context) {
		out = gabric.Remote(ctx).Exec("pwd").CaptureStdout().Run().StdoutStr()
	})

	if got, want := out, "/srv/app\n"; got != want {
		t.Errorf("stdout = %q, want %q", got, want)
	}

	assertions.AssertCommands(
		shelltest.Command{
			Name: "cd",
			Args: []string{"/srv/app"},
			Dir:  "/",
		},
		shelltest.Command{
			Name: "pwd",
			Args: []string{},
			Dir:  "/srv/app",
		},
	)
}

func TestIntegrationExitStatus(t *testing.T) {
	core := shelltest.New()
	assertions := shelltest.NewAssertions(t, core.Commands)
	ctx := newSSHContext(t, core)

	err := gabric.Remote(ctx).Exec("sh", "-c", "exit 3").RunErr()

	if exitErr, ok := errors.AsType[*ssh.ExitError](err); !ok {
		t.Fatalf("Run() error = %v, want ExitError", err)
	} else if got, want := exitErr.ExitStatus(), 3; got != want {
		t.Errorf("exit status = %d, want %d", got, want)
	}

	assertions.AssertCommands(shelltest.Command{
		Name: "exit",
		Args: []string{"3"},
		Dir:  "/",
	})
}

func TestIntegrationUnknownCommand(t *testing.T) {
	core := shelltest.New()
	assertions := shelltest.NewAssertions(t, core.Commands)
	ctx := newSSHContext(t, core)

	e := gabric.Remote(ctx).Exec("not-a-real-command").Capture()
	err := e.RunErr()

	if exitErr, ok := errors.AsType[*ssh.ExitError](err); !ok {
		t.Fatalf("RunErr() error = %v, want ExitError", err)
	} else if got, want := exitErr.ExitStatus(), 127; got != want {
		t.Errorf("exit status = %d, want %d", got, want)
	}

	if !strings.Contains(e.StderrStr(), "not-a-real-command: command not found") {
		t.Errorf("stderr = %q, want command not found", e.StderrStr())
	}

	assertions.AssertCommands(shelltest.Command{
		Name: "not-a-real-command",
		Args: []string{},
		Dir:  "/",
	})
}
