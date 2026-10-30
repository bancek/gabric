//go:build unix

package gabric

import (
	"context"
	"slices"
	"testing"
)

func TestNewLocalExecEnvContext(t *testing.T) {
	ctx := testContext(t)
	ctx = NewLocalExecEnvContext(ctx, Env("GABRIC_TEST_ENV", "value"))

	e := Local(ctx).Exec("sh", "-c", "echo $GABRIC_TEST_ENV").CaptureStdout().Run()

	if got := e.StdoutStr(); got != "value\n" {
		t.Errorf("StdoutStr() = %q, want %q", got, "value\n")
	}
}

func TestLocalOpsWithExecEnv(t *testing.T) {
	ctx := testContext(t)

	if HasLocalExecEnv(ctx) {
		t.Error("HasLocalExecEnv() = true before WithExecEnv, want false")
	}

	Local(ctx).WithExecEnv(NewExecEnv(Env("GABRIC_TEST_ENV", "value")), func(ctx context.Context) {
		if !HasLocalExecEnv(ctx) {
			t.Error("HasLocalExecEnv() = false with an env, want true")
		}

		env := GetLocalExecEnv(ctx)
		if got, ok := env.Get("GABRIC_TEST_ENV"); !ok || got != "value" {
			t.Errorf("Get(GABRIC_TEST_ENV) = %q, %v, want value, true", got, ok)
		}
		if entries := env.Environ(); !slices.Contains(entries, "GABRIC_TEST_ENV=value") {
			t.Errorf("Environ() = %v, want it to contain GABRIC_TEST_ENV=value", entries)
		}
	})

	if _, ok := GetLocalExecEnv(ctx).Get("GABRIC_TEST_ENV"); ok {
		t.Error("Get(GABRIC_TEST_ENV) outside WithExecEnv = true, want false")
	}
}

func TestLocalOpsWithExecEnvErr(t *testing.T) {
	ctx := testContext(t)

	err := Local(ctx).WithExecEnvErr(NewExecEnv(Env("GABRIC_TEST_ENV", "value")), func(ctx context.Context) error {
		if got, ok := GetLocalExecEnv(ctx).Get("GABRIC_TEST_ENV"); !ok || got != "value" {
			t.Errorf("Get(GABRIC_TEST_ENV) = %q, %v, want value, true", got, ok)
		}

		return nil
	})
	if err != nil {
		t.Fatalf("WithExecEnvErr() error: %v", err)
	}
}

func TestLocalExecEnvPerCommand(t *testing.T) {
	ctx := testContext(t)

	out := Local(ctx).
		Exec("sh", "-c", "echo $GABRIC_TEST_ENV").
		WithEnv(Env("GABRIC_TEST_ENV", "value")).
		CaptureStdout().
		Run().
		StdoutStr()

	if got, want := out, "value\n"; got != want {
		t.Errorf("StdoutStr() = %q, want %q", got, want)
	}
}

func TestLocalExecEnvPerCommandOverridesContext(t *testing.T) {
	ctx := NewLocalExecEnvContext(testContext(t), Env("GABRIC_TEST_ENV", "context"))

	out := Local(ctx).
		Exec("sh", "-c", "echo $GABRIC_TEST_ENV").
		WithEnv(Env("GABRIC_TEST_ENV", "command")).
		CaptureStdout().
		Run().
		StdoutStr()

	if got, want := out, "command\n"; got != want {
		t.Errorf("StdoutStr() = %q, want %q", got, want)
	}
}

func TestLocalExecEnvPerCommandDoesNotLeak(t *testing.T) {
	ctx := testContext(t)

	Local(ctx).
		Exec("sh", "-c", "true").
		WithEnv(Env("GABRIC_TEST_ENV", "value")).
		Run()

	out := Local(ctx).
		Exec("sh", "-c", "echo ${GABRIC_TEST_ENV:-unset}").
		CaptureStdout().
		Run().
		StdoutStr()

	if got, want := out, "unset\n"; got != want {
		t.Errorf("StdoutStr() = %q, want %q", got, want)
	}
}

func TestLocalExecEnvSensitiveStillRuns(t *testing.T) {
	ctx := testContext(t)

	out := Local(ctx).
		Exec("sh", "-c", "echo $GABRIC_TEST_SECRET").
		WithEnv(SensitiveEnv("GABRIC_TEST_SECRET", "secret")).
		CaptureStdout().
		Run().
		StdoutStr()

	if got, want := out, "secret\n"; got != want {
		t.Errorf("StdoutStr() = %q, want %q", got, want)
	}
}
