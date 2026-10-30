package gabric

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"
)

func TestExecRunErrResults(t *testing.T) {
	ctx := testContext(t)

	var got execCall

	run := func(_ context.Context, call execCall) ([]byte, []byte, error) {
		got = call
		return []byte("out"), []byte("err"), nil
	}

	e := newExec(ctx, run, []string{"echo", "hi"})
	e.Capture().Stdin(strings.NewReader("in"))

	if err := e.RunErr(); err != nil {
		t.Fatalf("RunErr() = %v, want nil", err)
	}

	if !got.captureStdout || !got.captureStderr {
		t.Errorf("Capture() flags = stdout %v, stderr %v; want both true", got.captureStdout, got.captureStderr)
	}

	if !slices.Equal(got.cmd, []string{"echo", "hi"}) {
		t.Errorf("cmd = %#v, want %#v", got.cmd, []string{"echo", "hi"})
	}

	if got.stdin == nil {
		t.Error("stdin = nil, want reader")
	}

	if e.StdoutStr() != "out" {
		t.Errorf("StdoutStr() = %q, want %q", e.StdoutStr(), "out")
	}

	if e.StderrStr() != "err" {
		t.Errorf("StderrStr() = %q, want %q", e.StderrStr(), "err")
	}
}

func TestExecOpsEnv(t *testing.T) {
	ctx := testContext(t)

	var got ExecEnv

	run := func(_ context.Context, call execCall) ([]byte, []byte, error) {
		got = call.env
		return nil, nil, nil
	}

	e := newExec(ctx, run, []string{"true"}).
		WithEnv(Env("A", "1"), Env("B", "2")).
		WithEnv(Env("A", "3"))

	if err := e.RunErr(); err != nil {
		t.Fatalf("RunErr() error: %v", err)
	}

	if want := []string{"A=3", "B=2"}; !slices.Equal(got.Environ(), want) {
		t.Errorf("env = %v, want %v", got.Environ(), want)
	}
}

func TestExecCaptureIndependent(t *testing.T) {
	ctx := testContext(t)

	tests := []struct {
		name       string
		configure  func(e *ExecOps)
		wantStdout bool
		wantStderr bool
	}{
		{name: "none", configure: func(e *ExecOps) {}, wantStdout: false, wantStderr: false},
		{name: "stdout", configure: func(e *ExecOps) { e.CaptureStdout() }, wantStdout: true, wantStderr: false},
		{name: "stderr", configure: func(e *ExecOps) { e.CaptureStderr() }, wantStdout: false, wantStderr: true},
		{name: "both", configure: func(e *ExecOps) { e.Capture() }, wantStdout: true, wantStderr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got execCall

			run := func(_ context.Context, call execCall) ([]byte, []byte, error) {
				got = call
				return nil, nil, nil
			}

			e := newExec(ctx, run, []string{"x"})
			tt.configure(e)

			if err := e.RunErr(); err != nil {
				t.Fatalf("RunErr() = %v, want nil", err)
			}

			if got.captureStdout != tt.wantStdout || got.captureStderr != tt.wantStderr {
				t.Errorf("capture flags = stdout %v, stderr %v; want stdout %v, stderr %v",
					got.captureStdout, got.captureStderr, tt.wantStdout, tt.wantStderr)
			}
		})
	}
}

func TestExecRunPanics(t *testing.T) {
	ctx := testContext(t)

	run := func(_ context.Context, call execCall) ([]byte, []byte, error) {
		return nil, nil, errors.New("boom")
	}

	defer func() {
		if recover() == nil {
			t.Fatal("Run() did not panic")
		}
	}()

	newExec(ctx, run, []string{"x"}).Run()
}

func TestExecLogErrors(t *testing.T) {
	ctx, buf := testContextBuffer(t)

	var got execCall

	run := func(_ context.Context, call execCall) ([]byte, []byte, error) {
		got = call
		return nil, []byte("no such file"), errors.New("exit status 1")
	}

	e := newExec(ctx, run, []string{"ls"})
	e.LogErrors()

	if err := e.RunErr(); err == nil {
		t.Fatal("RunErr() = nil, want error")
	}

	if !got.captureStderr {
		t.Error("LogErrors() did not capture stderr")
	}

	if !strings.Contains(buf.String(), "no such file") {
		t.Errorf("log output = %q, want it to contain captured stderr", buf.String())
	}
}

func TestExecLogErrorsSuccess(t *testing.T) {
	ctx, buf := testContextBuffer(t)

	run := func(_ context.Context, call execCall) ([]byte, []byte, error) {
		return nil, []byte("warning"), nil
	}

	e := newExec(ctx, run, []string{"ls"}).LogErrors()

	if err := e.RunErr(); err != nil {
		t.Fatalf("RunErr() = %v, want nil", err)
	}

	if strings.Contains(buf.String(), "warning") {
		t.Errorf("log output = %q, want no stderr logged on success", buf.String())
	}
}

func TestExecQuiet(t *testing.T) {
	ctx, buf := testContextBuffer(t)

	var got execCall

	run := func(_ context.Context, call execCall) ([]byte, []byte, error) {
		got = call
		return nil, []byte("no such file"), errors.New("exit status 1")
	}

	e := newExec(ctx, run, []string{"ls"}).Quiet()

	if err := e.RunErr(); err == nil {
		t.Fatal("RunErr() = nil, want error")
	}

	if !got.captureStdout || !got.captureStderr {
		t.Errorf("Quiet() flags = stdout %v, stderr %v; want both true", got.captureStdout, got.captureStderr)
	}

	if !strings.Contains(buf.String(), "no such file") {
		t.Errorf("log output = %q, want it to contain captured stderr", buf.String())
	}
}

func TestExecQuietSuccess(t *testing.T) {
	ctx, buf := testContextBuffer(t)

	run := func(_ context.Context, call execCall) ([]byte, []byte, error) {
		return []byte("out"), []byte("warning"), nil
	}

	e := newExec(ctx, run, []string{"ls"}).Quiet()

	if err := e.RunErr(); err != nil {
		t.Fatalf("RunErr() = %v, want nil", err)
	}

	if strings.Contains(buf.String(), "warning") {
		t.Errorf("log output = %q, want nothing logged on success", buf.String())
	}
}
