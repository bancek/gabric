package gabric

import (
	"context"
	"errors"
	"io"
	"slices"
	"strings"
	"testing"

	"github.com/bancek/gabric/middleware"
)

func TestRemoteRunnerOverride(t *testing.T) {
	var (
		gotCmd   string
		gotStdin string
	)

	runner := func(_ context.Context, cmd string, stdin io.Reader, stdout, stderr io.Writer) error {
		gotCmd = cmd

		content, err := io.ReadAll(stdin)
		if err != nil {
			return err
		}
		gotStdin = string(content)

		if _, err := io.WriteString(stdout, "out"); err != nil {
			return err
		}

		_, err = io.WriteString(stderr, "err")

		return err
	}

	ctx := NewRemoteRunnerContext(testContext(t), runner)
	e := Remote(ctx).Exec("echo", "hi").Stdin(strings.NewReader("in")).Capture().Run()

	if gotCmd != "/bin/sh -c 'echo hi'" {
		t.Errorf("runner cmd = %q, want %q", gotCmd, "/bin/sh -c 'echo hi'")
	}
	if gotStdin != "in" {
		t.Errorf("runner stdin = %q, want %q", gotStdin, "in")
	}
	if e.StdoutStr() != "out" {
		t.Errorf("StdoutStr() = %q, want %q", e.StdoutStr(), "out")
	}
	if e.StderrStr() != "err" {
		t.Errorf("StderrStr() = %q, want %q", e.StderrStr(), "err")
	}
}

func TestRemoteRunnerSudo(t *testing.T) {
	var gotCmd string

	runner := func(_ context.Context, cmd string, _ io.Reader, _, _ io.Writer) error {
		gotCmd = cmd

		return nil
	}

	ctx := NewRemoteRunnerContext(testContext(t), runner)
	ctx = NewRemoteSudoContext(ctx, &Sudo{})

	if err := Remote(ctx).Exec("true").RunErr(); err != nil {
		t.Fatalf("RunErr() error: %v", err)
	}

	if !strings.HasPrefix(gotCmd, "sudo /bin/sh -c ") {
		t.Errorf("runner cmd = %q, want a sudo-prefixed shell command", gotCmd)
	}
}

func TestGetRemoteRunner(t *testing.T) {
	ctx := NewRemoteRunnerContext(testContext(t), localRemoteRunner)

	if GetRemoteRunner(ctx) == nil {
		t.Error("GetRemoteRunner() = nil, want runner")
	}
}

func TestGetRemoteRunnerPanics(t *testing.T) {
	defer func() {
		err, ok := recover().(error)
		if !ok || !errors.Is(err, ErrNoRemoteRunner) {
			t.Errorf("panic = %v, want ErrNoRemoteRunner", err)
		}
	}()

	GetRemoteRunner(testContext(t))
}

func TestHasRemoteRunner(t *testing.T) {
	ctx := testContext(t)

	if HasRemoteRunner(ctx) {
		t.Error("HasRemoteRunner() = true without a runner, want false")
	}

	ctx = NewRemoteRunnerContext(ctx, localRemoteRunner)

	if !HasRemoteRunner(ctx) {
		t.Error("HasRemoteRunner() = false with a runner, want true")
	}
}

func TestExecExitError(t *testing.T) {
	inner := errors.New("inner")

	err := &ExecExitError{Status: 3, Err: inner}

	if err.Error() != "command exited with status 3" {
		t.Errorf("Error() = %q", err.Error())
	}
	if err.ExitCode() != 3 {
		t.Errorf("ExitCode() = %d, want 3", err.ExitCode())
	}
	if !errors.Is(err, inner) {
		t.Error("errors.Is() = false, want true")
	}
}

func TestNewRemoteRunnerContextPanicsWithNilRunner(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("NewRemoteRunnerContext() did not panic with a nil runner")
		}
	}()

	NewRemoteRunnerContext(testContext(t), nil)
}

func TestLocalRemoteTargetOps(t *testing.T) {
	var got ExecRequest

	backend := LocalBackendFunc{
		ExecFunc: func(_ context.Context, req ExecRequest) error {
			got = req

			_, err := io.WriteString(req.Stdout, "out")

			return err
		},
	}

	ctx := NewLocalBackendContext(testContext(t), backend)
	ctx = NewTargetContext(ctx, LocalRemoteTarget)

	out := TargetOps(ctx).Exec("echo", "hi").Capture().Run().StdoutStr()

	if out != "out" {
		t.Errorf("StdoutStr() = %q, want %q", out, "out")
	}
	if want := []string{"/bin/sh", "-c", "/bin/sh -c 'echo hi'"}; !slices.Equal(got.Cmd, want) {
		t.Errorf("Cmd = %v, want %v", got.Cmd, want)
	}
}

func TestLocalRemoteTargetOpsMiddleware(t *testing.T) {
	var got ExecRequest

	backend := LocalBackendFunc{
		ExecFunc: func(_ context.Context, req ExecRequest) error {
			got = req

			return nil
		},
	}

	ctx := NewLocalBackendContext(testContext(t), backend)

	run := middleware.Apply([]middleware.Middleware{WithLocalRemoteTarget}, func(ctx context.Context) error {
		return TargetOps(ctx).Exec("true").RunErr()
	})

	if err := run(ctx); err != nil {
		t.Fatalf("run error: %v", err)
	}

	if want := []string{"/bin/sh", "-c", "/bin/sh -c true"}; !slices.Equal(got.Cmd, want) {
		t.Errorf("Cmd = %v, want %v", got.Cmd, want)
	}
}
