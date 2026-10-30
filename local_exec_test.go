//go:build unix

package gabric

import (
	"bytes"
	"errors"
	"strings"
	"testing"
)

func TestLocalExecCaptureStdout(t *testing.T) {
	ctx := testContext(t)

	e := Local(ctx).Exec("echo", "hello").CaptureStdout().Run()

	if got := e.StdoutStr(); got != "hello\n" {
		t.Errorf("StdoutStr() = %q, want %q", got, "hello\n")
	}
}

func TestLocalExecCaptureStderr(t *testing.T) {
	ctx := testContext(t)

	e := Local(ctx).Exec("sh", "-c", "echo oops 1>&2").CaptureStderr().Run()

	if got := e.StderrStr(); got != "oops\n" {
		t.Errorf("StderrStr() = %q, want %q", got, "oops\n")
	}
}

func TestLocalExecCaptureBoth(t *testing.T) {
	ctx := testContext(t)

	e := Local(ctx).Exec("sh", "-c", "echo out; echo err 1>&2").Capture().Run()

	if got := e.StdoutStr(); got != "out\n" {
		t.Errorf("StdoutStr() = %q, want %q", got, "out\n")
	}

	if got := e.StderrStr(); got != "err\n" {
		t.Errorf("StderrStr() = %q, want %q", got, "err\n")
	}
}

func TestLocalExecStdin(t *testing.T) {
	ctx := testContext(t)

	e := Local(ctx).Exec("cat").Stdin(strings.NewReader("input")).CaptureStdout().Run()

	if got := e.StdoutStr(); got != "input" {
		t.Errorf("StdoutStr() = %q, want %q", got, "input")
	}
}

func TestLocalExecStdoutWriter(t *testing.T) {
	ctx := testContext(t)

	var buf bytes.Buffer

	Local(ctx).Exec("echo", "hello").Stdout(&buf).Run()

	if got := buf.String(); got != "hello\n" {
		t.Errorf("Stdout(w) = %q, want %q", got, "hello\n")
	}
}

func TestLocalExecStderrWriter(t *testing.T) {
	ctx := testContext(t)

	var buf bytes.Buffer

	Local(ctx).Exec("sh", "-c", "echo oops 1>&2").Stderr(&buf).Run()

	if got := buf.String(); got != "oops\n" {
		t.Errorf("Stderr(w) = %q, want %q", got, "oops\n")
	}
}

func TestLocalExecFailure(t *testing.T) {
	ctx := testContext(t)

	err := Local(ctx).Exec("sh", "-c", "exit 1").RunErr()

	exitErr, ok := errors.AsType[*ExecExitError](err)
	if !ok {
		t.Fatalf("RunErr() error = %T, want *ExecExitError", err)
	}
	if exitErr.ExitCode() != 1 {
		t.Errorf("ExitCode() = %d, want 1", exitErr.ExitCode())
	}
}

func TestLocalExecRunPanics(t *testing.T) {
	ctx := testContext(t)

	defer func() {
		if recover() == nil {
			t.Fatal("Run() did not panic")
		}
	}()

	Local(ctx).Exec("sh", "-c", "exit 1").Run()
}
