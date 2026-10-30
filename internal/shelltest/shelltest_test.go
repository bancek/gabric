//go:build unix

package shelltest

import (
	"bytes"
	"context"
	"testing"
)

func TestRunScriptEcho(t *testing.T) {
	core := New()

	var stdout, stderr bytes.Buffer

	code, err := core.RunScript(context.Background(), "echo hi", nil, &stdout, &stderr)
	if err != nil {
		t.Fatalf("RunScript() error: %v", err)
	}

	if code != 0 {
		t.Errorf("exit code = %d, want 0", code)
	}

	if got, want := stdout.String(), "hi\n"; got != want {
		t.Errorf("stdout = %q, want %q", got, want)
	}

	if got := stderr.String(); got != "" {
		t.Errorf("stderr = %q, want empty", got)
	}
}

func TestRunArgs(t *testing.T) {
	core := New()

	if err := core.FS().MkdirAll("/srv/app", 0o755); err != nil {
		t.Fatalf("MkdirAll() error: %v", err)
	}

	var stdout bytes.Buffer

	code, err := core.RunArgs(context.Background(), "/srv/app", []string{"echo", "hi"}, nil, &stdout, nil)
	if err != nil {
		t.Fatalf("RunArgs() error: %v", err)
	}

	if code != 0 {
		t.Errorf("exit code = %d, want 0", code)
	}

	if got, want := stdout.String(), "hi\n"; got != want {
		t.Errorf("stdout = %q, want %q", got, want)
	}

	cmd := NewAssertions(t, core.Commands).AssertExecuted("echo", "hi")
	if cmd.Dir != "/srv/app" {
		t.Errorf("Dir = %q, want %q", cmd.Dir, "/srv/app")
	}
}

func TestRunScriptEnv(t *testing.T) {
	core := New()

	var stdout bytes.Buffer

	code, err := core.RunScriptEnv(
		context.Background(),
		"echo $GABRIC_TEST_ENV",
		[]string{"GABRIC_TEST_ENV=value"},
		nil,
		&stdout,
		nil,
	)
	if err != nil {
		t.Fatalf("RunScriptEnv() error: %v", err)
	}
	if code != 0 {
		t.Errorf("exit code = %d, want 0", code)
	}
	if got, want := stdout.String(), "value\n"; got != want {
		t.Errorf("stdout = %q, want %q", got, want)
	}
}

func TestRunArgsEnvNestedShell(t *testing.T) {
	core := New()

	var stdout bytes.Buffer

	code, err := core.RunArgsEnv(
		context.Background(),
		"/",
		[]string{"sh", "-c", "echo $GABRIC_TEST_ENV"},
		[]string{"GABRIC_TEST_ENV=value"},
		nil,
		&stdout,
		nil,
	)
	if err != nil {
		t.Fatalf("RunArgsEnv() error: %v", err)
	}
	if code != 0 {
		t.Errorf("exit code = %d, want 0", code)
	}
	if got, want := stdout.String(), "value\n"; got != want {
		t.Errorf("stdout = %q, want %q", got, want)
	}
}

func TestRunScriptEnvSudo(t *testing.T) {
	core := New()

	var stdout bytes.Buffer

	code, err := core.RunScriptEnv(
		context.Background(),
		"sudo sh -c 'echo $GABRIC_TEST_ENV'",
		[]string{"GABRIC_TEST_ENV=value"},
		nil,
		&stdout,
		nil,
	)
	if err != nil {
		t.Fatalf("RunScriptEnv() error: %v", err)
	}
	if code != 0 {
		t.Errorf("exit code = %d, want 0", code)
	}
	if got, want := stdout.String(), "value\n"; got != want {
		t.Errorf("stdout = %q, want %q", got, want)
	}
}

func TestRunScriptExitStatus(t *testing.T) {
	core := New()

	code, err := core.RunScript(context.Background(), "exit 7", nil, nil, nil)
	if err != nil {
		t.Fatalf("RunScript() error: %v", err)
	}

	if code != 7 {
		t.Errorf("exit code = %d, want 7", code)
	}
}

func TestRunScriptFalse(t *testing.T) {
	core := New()

	code, err := core.RunScript(context.Background(), "false", nil, nil, nil)
	if err != nil {
		t.Fatalf("RunScript() error: %v", err)
	}

	if code != 1 {
		t.Errorf("exit code = %d, want 1", code)
	}
}

func TestRunScriptStderr(t *testing.T) {
	core := New()

	var stdout, stderr bytes.Buffer

	code, err := core.RunScript(context.Background(), "echo out; echo err >&2", nil, &stdout, &stderr)
	if err != nil {
		t.Fatalf("RunScript() error: %v", err)
	}

	if code != 0 {
		t.Errorf("exit code = %d, want 0", code)
	}

	if got, want := stdout.String(), "out\n"; got != want {
		t.Errorf("stdout = %q, want %q", got, want)
	}

	if got, want := stderr.String(), "err\n"; got != want {
		t.Errorf("stderr = %q, want %q", got, want)
	}
}

func TestRunScriptParseError(t *testing.T) {
	core := New()

	var stderr bytes.Buffer

	code, err := core.RunScript(context.Background(), "if true; then", nil, nil, &stderr)
	if err != nil {
		if code == 0 {
			t.Errorf("exit code = %d, want non-zero", code)
		}
	} else {
		t.Fatal("RunScript() error = nil, want parse error")
	}

	if stderr.Len() == 0 {
		t.Error("stderr is empty, want parse error")
	}
}
