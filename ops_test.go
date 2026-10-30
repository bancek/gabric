package gabric

import (
	"context"
	"testing"
)

func TestOpsWithExecEnv(t *testing.T) {
	ctx := testContext(t)

	var ops Ops = Local(ctx)

	ops.WithExecEnv(NewExecEnv(Env("GABRIC_TEST_ENV", "value")), func(ctx context.Context) {
		if got, ok := GetLocalExecEnv(ctx).Get("GABRIC_TEST_ENV"); !ok || got != "value" {
			t.Errorf("Get(GABRIC_TEST_ENV) = %q, %v, want value, true", got, ok)
		}
	})

	err := ops.WithExecEnvErr(NewExecEnv(Env("GABRIC_TEST_ENV", "value")), func(ctx context.Context) error {
		if got, ok := GetLocalExecEnv(ctx).Get("GABRIC_TEST_ENV"); !ok || got != "value" {
			t.Errorf("Get(GABRIC_TEST_ENV) = %q, %v, want value, true", got, ok)
		}

		return nil
	})
	if err != nil {
		t.Fatalf("WithExecEnvErr() error: %v", err)
	}

	var remote Ops = Remote(ctx)

	remote.WithExecEnv(NewExecEnv(Env("GABRIC_TEST_ENV", "value")), func(ctx context.Context) {
		if got, ok := GetRemoteExecEnv(ctx).Get("GABRIC_TEST_ENV"); !ok || got != "value" {
			t.Errorf("Get(GABRIC_TEST_ENV) = %q, %v, want value, true", got, ok)
		}
	})
}

func TestTargetOpsWithExecEnv(t *testing.T) {
	ctx := NewTargetContext(testContext(t), LocalTarget)

	ops := TargetOps(ctx)
	ops.WithExecEnv(NewExecEnv(Env("GABRIC_TEST_ENV", "value")), func(ctx context.Context) {
		local := TargetOps(ctx).(*LocalOps)
		if got, ok := GetLocalExecEnv(local.ctx).Get("GABRIC_TEST_ENV"); !ok || got != "value" {
			t.Errorf("Get(GABRIC_TEST_ENV) = %q, %v, want value, true", got, ok)
		}
	})

	err := TargetOps(ctx).WithExecEnvErr(NewExecEnv(Env("GABRIC_TEST_ENV", "value")), func(ctx context.Context) error {
		local := TargetOps(ctx).(*LocalOps)
		if got, ok := GetLocalExecEnv(local.ctx).Get("GABRIC_TEST_ENV"); !ok || got != "value" {
			t.Errorf("Get(GABRIC_TEST_ENV) = %q, %v, want value, true", got, ok)
		}

		return nil
	})
	if err != nil {
		t.Fatalf("WithExecEnvErr() error: %v", err)
	}
}

func TestOpsWithRsync(t *testing.T) {
	if _, ok := any(Remote(context.Background())).(OpsWithRsync); !ok {
		t.Error("*RemoteOps does not implement OpsWithRsync")
	}

	if _, ok := any(Local(context.Background())).(OpsWithRsync); ok {
		t.Error("*LocalOps implements OpsWithRsync")
	}
}
