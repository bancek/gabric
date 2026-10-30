//go:build unix

package gabric

import (
	"context"
	"io"
	"strings"
	"testing"
)

func TestNewRemoteExecEnvContext(t *testing.T) {
	ctx := NewRemoteExecEnvContext(testContext(t), Env("GABRIC_TEST_ENV", "value"))

	out := LocalRemote(ctx).
		Exec("sh", "-c", "echo $GABRIC_TEST_ENV").
		CaptureStdout().
		Run().
		StdoutStr()

	if got, want := out, "value\n"; got != want {
		t.Errorf("StdoutStr() = %q, want %q", got, want)
	}
}

func TestRemoteOpsWithExecEnv(t *testing.T) {
	ctx := testContext(t)

	if HasRemoteExecEnv(ctx) {
		t.Error("HasRemoteExecEnv() = true before WithExecEnv, want false")
	}

	Remote(ctx).WithExecEnv(NewExecEnv(Env("GABRIC_TEST_ENV", "value")), func(ctx context.Context) {
		if !HasRemoteExecEnv(ctx) {
			t.Error("HasRemoteExecEnv() = false with an env, want true")
		}

		env := GetRemoteExecEnv(ctx)
		if got, ok := env.Get("GABRIC_TEST_ENV"); !ok || got != "value" {
			t.Errorf("Get(GABRIC_TEST_ENV) = %q, %v, want value, true", got, ok)
		}
	})

	if _, ok := GetRemoteExecEnv(ctx).Get("GABRIC_TEST_ENV"); ok {
		t.Error("Get(GABRIC_TEST_ENV) outside WithExecEnv = true, want false")
	}
}

func TestRemoteOpsWithExecEnvErr(t *testing.T) {
	ctx := testContext(t)

	err := Remote(ctx).WithExecEnvErr(NewExecEnv(Env("GABRIC_TEST_ENV", "value")), func(ctx context.Context) error {
		if got, ok := GetRemoteExecEnv(ctx).Get("GABRIC_TEST_ENV"); !ok || got != "value" {
			t.Errorf("Get(GABRIC_TEST_ENV) = %q, %v, want value, true", got, ok)
		}

		return nil
	})
	if err != nil {
		t.Fatalf("WithExecEnvErr() error: %v", err)
	}
}

func TestRemoteExecEnvPerCommand(t *testing.T) {
	var got string

	runner := func(_ context.Context, cmd string, _ io.Reader, _, _ io.Writer) error {
		got = cmd
		return nil
	}

	ctx := NewRemoteRunnerContext(testContext(t), runner)

	if err := Remote(ctx).Exec("echo", "hi").WithEnv(Env("GABRIC_TEST_ENV", "value")).RunErr(); err != nil {
		t.Fatalf("RunErr() error: %v", err)
	}

	if want := "/bin/sh -c 'export GABRIC_TEST_ENV=value; echo hi'"; got != want {
		t.Errorf("runner cmd = %q, want %q", got, want)
	}
}

func TestRemoteExecEnvPerCommandOverridesContext(t *testing.T) {
	var got string

	runner := func(_ context.Context, cmd string, _ io.Reader, _, _ io.Writer) error {
		got = cmd
		return nil
	}

	ctx := NewRemoteRunnerContext(testContext(t), runner)
	ctx = NewRemoteExecEnvContext(ctx, Env("GABRIC_TEST_ENV", "context"))

	if err := Remote(ctx).Exec("echo", "hi").WithEnv(Env("GABRIC_TEST_ENV", "command")).RunErr(); err != nil {
		t.Fatalf("RunErr() error: %v", err)
	}

	if want := "/bin/sh -c 'export GABRIC_TEST_ENV=command; echo hi'"; got != want {
		t.Errorf("runner cmd = %q, want %q", got, want)
	}
}

func TestRemoteExecEnvSudo(t *testing.T) {
	var got string

	runner := func(_ context.Context, cmd string, _ io.Reader, _, _ io.Writer) error {
		got = cmd
		return nil
	}

	ctx := NewRemoteRunnerContext(testContext(t), runner)
	ctx = NewRemoteSudoContext(ctx, &Sudo{})
	ctx = NewRemoteExecEnvContext(ctx, Env("GABRIC_TEST_ENV", "value"))

	if err := Remote(ctx).Exec("echo", "hi").RunErr(); err != nil {
		t.Fatalf("RunErr() error: %v", err)
	}

	if want := "sudo /bin/sh -c 'export GABRIC_TEST_ENV=value; echo hi'"; got != want {
		t.Errorf("runner cmd = %q, want %q", got, want)
	}
}

func TestRemoteExecEnvLogRedactsSensitive(t *testing.T) {
	ctx, buf := testContextBuffer(t)

	runner := func(_ context.Context, _ string, _ io.Reader, _, _ io.Writer) error {
		return nil
	}
	ctx = NewRemoteRunnerContext(ctx, runner)

	if err := Remote(ctx).
		Exec("echo", "hi").
		WithEnv(Env("GABRIC_TEST_ENV", "value"), SensitiveEnv("GABRIC_TEST_SECRET", "secret")).
		RunErr(); err != nil {
		t.Fatalf("RunErr() error: %v", err)
	}

	out := buf.String()
	if strings.Contains(out, "secret") {
		t.Errorf("log output = %q, want no secret", out)
	}
	if !strings.Contains(out, "cmd=") {
		t.Errorf("log output = %q, want a cmd attribute", out)
	}
	if !strings.Contains(out, "GABRIC_TEST_ENV=value") {
		t.Errorf("log output = %q, want the normal value in cmd", out)
	}
	if !strings.Contains(out, "GABRIC_TEST_SECRET=<sensitive>") {
		t.Errorf("log output = %q, want the redacted marker in cmd", out)
	}
}

func TestRemoteExecEnvAppliesToFileOperations(t *testing.T) {
	var cmds []string

	runner := func(_ context.Context, cmd string, _ io.Reader, _, _ io.Writer) error {
		cmds = append(cmds, cmd)
		return nil
	}

	ctx := NewRemoteRunnerContext(testContext(t), runner)
	ctx = NewRemoteExecEnvContext(ctx, Env("GABRIC_TEST_ENV", "value"))

	if err := Remote(ctx).Put("/tmp/gabric-env-test").String("data").RunErr(); err != nil {
		t.Fatalf("Put() error: %v", err)
	}

	if len(cmds) == 0 {
		t.Fatal("Put() did not run a command")
	}
	for _, cmd := range cmds {
		if !strings.Contains(cmd, "export GABRIC_TEST_ENV=value; ") {
			t.Errorf("command = %q, want the remote env prefix", cmd)
		}
	}
}

func TestRemoteExecEnvSensitiveStillRuns(t *testing.T) {
	ctx := NewRemoteExecEnvContext(testContext(t), SensitiveEnv("GABRIC_TEST_SECRET", "secret"))

	out := LocalRemote(ctx).
		Exec("sh", "-c", "echo $GABRIC_TEST_SECRET").
		CaptureStdout().
		Run().
		StdoutStr()

	if got, want := out, "secret\n"; got != want {
		t.Errorf("StdoutStr() = %q, want %q", got, want)
	}
}
