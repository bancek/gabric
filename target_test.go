package gabric

import (
	"bytes"
	"context"
	"errors"
	"path/filepath"
	"testing"

	"github.com/bancek/gabric/middleware"
)

func TestHasTarget(t *testing.T) {
	ctx := context.Background()

	if HasTarget(ctx) {
		t.Error("HasTarget() = true without a target, want false")
	}

	ctx = NewTargetContext(ctx, LocalTarget)

	if !HasTarget(ctx) {
		t.Error("HasTarget() = false with a target, want true")
	}
}

func TestTargetOps(t *testing.T) {
	ctx := NewTargetContext(context.Background(), LocalTarget)

	if _, ok := TargetOps(ctx).(*LocalOps); !ok {
		t.Errorf("TargetOps() = %T, want *LocalOps", TargetOps(ctx))
	}

	ctx = NewTargetContext(context.Background(), RemoteTarget)

	if _, ok := TargetOps(ctx).(*RemoteOps); !ok {
		t.Errorf("TargetOps() = %T, want *RemoteOps", TargetOps(ctx))
	}

	ctx = NewTargetContext(context.Background(), LocalRemoteTarget)

	if _, ok := TargetOps(ctx).(*RemoteOps); !ok {
		t.Errorf("TargetOps() = %T, want *RemoteOps", TargetOps(ctx))
	}
}

func TestTargetOpsPanicsWithoutTarget(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("TargetOps() did not panic without a target")
		}
	}()

	TargetOps(context.Background())
}

func TestNewTargetPanicsWithNilOps(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("NewTarget() did not panic with nil ops")
		}
	}()

	var ops func(context.Context) Ops

	NewTarget(ops, AppendLocalExecEnvContext, NewLocalWorkingDirContext)
}

func TestNewTargetPanicsWithNilExecEnvContext(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("NewTarget() did not panic with a nil exec env context")
		}
	}()

	NewTarget(Local, nil, NewLocalWorkingDirContext)
}

func TestNewTargetPanicsWithNilWorkingDirContext(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("NewTarget() did not panic with a nil working dir context")
		}
	}()

	NewTarget(Local, AppendLocalExecEnvContext, nil)
}

func TestNewTargetContextPanicsWithZeroTarget(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("NewTargetContext() did not panic with a zero target")
		}
	}()

	NewTargetContext(context.Background(), Target{})
}

func TestTargetOpsPanicsWithNilResult(t *testing.T) {
	target := NewTarget(func(context.Context) Ops { return nil }, AppendLocalExecEnvContext, NewLocalWorkingDirContext)
	ctx := NewTargetContext(context.Background(), target)

	defer func() {
		if recover() == nil {
			t.Error("TargetOps() did not panic with a nil provider result")
		}
	}()

	TargetOps(ctx)
}

func TestTargetOpsMiddleware(t *testing.T) {
	ctx := context.Background()

	run := middleware.Apply([]middleware.Middleware{WithLocalTarget}, func(ctx context.Context) error {
		if _, ok := TargetOps(ctx).(*LocalOps); !ok {
			t.Errorf("TargetOps() = %T, want *LocalOps", TargetOps(ctx))
		}

		return nil
	})

	if err := run(ctx); err != nil {
		t.Fatalf("run error: %v", err)
	}

	run = middleware.Apply([]middleware.Middleware{WithRemoteTarget}, func(ctx context.Context) error {
		if _, ok := TargetOps(ctx).(*RemoteOps); !ok {
			t.Errorf("TargetOps() = %T, want *RemoteOps", TargetOps(ctx))
		}

		return nil
	})

	if err := run(ctx); err != nil {
		t.Fatalf("run error: %v", err)
	}
}

func TestRunSetsRemoteTarget(t *testing.T) {
	g := New()

	var got Ops

	g.Task("check", func(ctx context.Context) {
		got = TargetOps(ctx)
	})

	if err := g.Run(testContext(t), parseCalls(t, g, "check")); err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	if _, ok := got.(*RemoteOps); !ok {
		t.Errorf("TargetOps() = %T, want *RemoteOps", got)
	}
}

func TestMainUsesRemoteTargetByDefault(t *testing.T) {
	g := New()
	g.Args = []string{"check"}
	g.Stdout = &bytes.Buffer{}
	g.Stderr = &bytes.Buffer{}

	var got Ops

	g.Task("check", func(ctx context.Context) {
		got = TargetOps(ctx)
	})

	if code := g.runMain(mainOptions{ctx: context.Background()}); code != 0 {
		t.Fatalf("runMain() = %d, want 0", code)
	}

	if _, ok := got.(*RemoteOps); !ok {
		t.Errorf("TargetOps() = %T, want *RemoteOps", got)
	}
}

func TestMainPreservesExplicitTarget(t *testing.T) {
	g := New()
	g.Args = []string{"check"}
	g.Stdout = &bytes.Buffer{}
	g.Stderr = &bytes.Buffer{}

	var got Ops

	g.Task("check", func(ctx context.Context) {
		got = TargetOps(ctx)
	})

	ctx := NewTargetContext(context.Background(), LocalTarget)
	if code := g.runMain(mainOptions{ctx: ctx}); code != 0 {
		t.Fatalf("runMain() = %d, want 0", code)
	}

	if _, ok := got.(*LocalOps); !ok {
		t.Errorf("TargetOps() = %T, want *LocalOps", got)
	}
}

func TestMainTargetMiddlewareOverride(t *testing.T) {
	g := New()
	g.Args = []string{"check"}
	g.Stdout = &bytes.Buffer{}
	g.Stderr = &bytes.Buffer{}

	var got Ops

	g.Task("check", func(ctx context.Context) {
		got = TargetOps(ctx)
	})

	cfg := mainOptions{
		ctx:         context.Background(),
		middlewares: []middleware.Middleware{WithLocalTarget},
	}

	if code := g.runMain(cfg); code != 0 {
		t.Fatalf("runMain() = %d, want 0", code)
	}

	if _, ok := got.(*LocalOps); !ok {
		t.Errorf("TargetOps() = %T, want *LocalOps", got)
	}
}

func TestTargetRsyncOps(t *testing.T) {
	ctx := NewTargetContext(testContext(t), RemoteTarget)

	ops, err := TargetRsyncOps(ctx)
	if err != nil {
		t.Fatalf("TargetRsyncOps(RemoteTarget) error: %v", err)
	}
	if ops == nil {
		t.Fatal("TargetRsyncOps(RemoteTarget) = nil")
	}

	ctx = NewTargetContext(testContext(t), LocalRemoteTarget)

	if _, err := TargetRsyncOps(ctx); err != nil {
		t.Fatalf("TargetRsyncOps(LocalRemoteTarget) error: %v", err)
	}

	ctx = NewTargetContext(testContext(t), LocalTarget)

	if _, err := TargetRsyncOps(ctx); !errors.Is(err, ErrNoRsyncTarget) {
		t.Errorf("TargetRsyncOps(LocalTarget) error = %v, want ErrNoRsyncTarget", err)
	}
}

func TestNewLocalExecEnvContextReplaces(t *testing.T) {
	ctx := NewLocalExecEnvContext(testContext(t), Env("A", "1"))
	ctx = NewLocalExecEnvContext(ctx, Env("B", "2"))

	env := GetLocalExecEnv(ctx)
	if _, ok := env.Get("A"); ok {
		t.Error("Get(A) = true, want the new environment to drop A")
	}
	if got, ok := env.Get("B"); !ok || got != "2" {
		t.Errorf("Get(B) = %q, %v, want 2, true", got, ok)
	}
}

func TestAppendLocalExecEnvContext(t *testing.T) {
	ctx := NewLocalExecEnvContext(testContext(t), Env("A", "1"))
	ctx = AppendLocalExecEnvContext(ctx, Env("B", "2"), Env("A", "3"))

	env := GetLocalExecEnv(ctx)
	if got, ok := env.Get("A"); !ok || got != "3" {
		t.Errorf("Get(A) = %q, %v, want 3, true", got, ok)
	}
	if got, ok := env.Get("B"); !ok || got != "2" {
		t.Errorf("Get(B) = %q, %v, want 2, true", got, ok)
	}
}

func TestNewRemoteExecEnvContextReplaces(t *testing.T) {
	ctx := NewRemoteExecEnvContext(testContext(t), Env("A", "1"))
	ctx = NewRemoteExecEnvContext(ctx, Env("B", "2"))

	env := GetRemoteExecEnv(ctx)
	if _, ok := env.Get("A"); ok {
		t.Error("Get(A) = true, want the new environment to drop A")
	}
	if got, ok := env.Get("B"); !ok || got != "2" {
		t.Errorf("Get(B) = %q, %v, want 2, true", got, ok)
	}
}

func TestAppendRemoteExecEnvContext(t *testing.T) {
	ctx := NewRemoteExecEnvContext(testContext(t), Env("A", "1"))
	ctx = AppendRemoteExecEnvContext(ctx, Env("B", "2"), Env("A", "3"))

	env := GetRemoteExecEnv(ctx)
	if got, ok := env.Get("A"); !ok || got != "3" {
		t.Errorf("Get(A) = %q, %v, want 3, true", got, ok)
	}
	if got, ok := env.Get("B"); !ok || got != "2" {
		t.Errorf("Get(B) = %q, %v, want 2, true", got, ok)
	}
}

func TestNewTargetOpsExecEnvContextReplaces(t *testing.T) {
	ctx := NewTargetOpsExecEnvContext(testContext(t), Env("A", "1"))
	ctx = NewTargetOpsExecEnvContext(ctx, Env("B", "2"))

	env := GetTargetOpsExecEnv(ctx)
	if _, ok := env.Get("A"); ok {
		t.Error("Get(A) = true, want the new environment to drop A")
	}
	if got, ok := env.Get("B"); !ok || got != "2" {
		t.Errorf("Get(B) = %q, %v, want 2, true", got, ok)
	}
	if targetOpsExecEnvApplied(ctx) {
		t.Error("NewTargetOpsExecEnvContext() marked the overlay as applied")
	}
}

func TestAppendTargetOpsExecEnvContext(t *testing.T) {
	ctx := NewTargetOpsExecEnvContext(testContext(t), Env("A", "1"))
	ctx = AppendTargetOpsExecEnvContext(ctx, Env("B", "2"), Env("A", "3"))

	env := GetTargetOpsExecEnv(ctx)
	if got, ok := env.Get("A"); !ok || got != "3" {
		t.Errorf("Get(A) = %q, %v, want 3, true", got, ok)
	}
	if got, ok := env.Get("B"); !ok || got != "2" {
		t.Errorf("Get(B) = %q, %v, want 2, true", got, ok)
	}
}

func TestTargetOpsAppliesTargetOpsExecEnvLocally(t *testing.T) {
	ctx := NewLocalExecEnvContext(testContext(t), Env("A", "1"))
	ctx = NewTargetContext(ctx, LocalTarget)
	ctx = AppendTargetOpsExecEnvContext(ctx, Env("B", "2"))

	ops := TargetOps(ctx).(*LocalOps)
	env := GetLocalExecEnv(ops.ctx)

	if got, ok := env.Get("A"); !ok || got != "1" {
		t.Errorf("Get(A) = %q, %v, want 1, true", got, ok)
	}
	if got, ok := env.Get("B"); !ok || got != "2" {
		t.Errorf("Get(B) = %q, %v, want 2, true", got, ok)
	}
}

func TestTargetOpsAppliesTargetOpsExecEnvRemotely(t *testing.T) {
	ctx := NewRemoteExecEnvContext(testContext(t), Env("A", "1"))
	ctx = NewTargetContext(ctx, RemoteTarget)
	ctx = AppendTargetOpsExecEnvContext(ctx, Env("B", "2"))

	ops := TargetOps(ctx).(*RemoteOps)
	env := GetRemoteExecEnv(ops.ctx)

	if got, ok := env.Get("A"); !ok || got != "1" {
		t.Errorf("Get(A) = %q, %v, want 1, true", got, ok)
	}
	if got, ok := env.Get("B"); !ok || got != "2" {
		t.Errorf("Get(B) = %q, %v, want 2, true", got, ok)
	}
}

func TestNewTargetOpsWorkingDirContextReplaces(t *testing.T) {
	ctx := NewTargetOpsWorkingDirContext(testContext(t), "/a")
	ctx = NewTargetOpsWorkingDirContext(ctx, "/b")

	if got, ok := GetTargetOpsWorkingDir(ctx); !ok || got != "/b" {
		t.Errorf("GetTargetOpsWorkingDir() = %q, %v, want /b, true", got, ok)
	}
	if targetOpsWorkingDirApplied(ctx) {
		t.Error("NewTargetOpsWorkingDirContext() marked the overlay as applied")
	}
}

func TestTargetOpsAppliesTargetOpsWorkingDirLocally(t *testing.T) {
	ctx := NewLocalWorkingDirContext(testContext(t), "/ambient")
	ctx = NewTargetContext(ctx, LocalTarget)
	ctx = NewTargetOpsWorkingDirContext(ctx, "/target")

	ops := TargetOps(ctx).(*LocalOps)
	if got, want := GetLocalWorkingDir(ops.ctx), "/target"; got != want {
		t.Errorf("GetLocalWorkingDir() = %q, want %q", got, want)
	}
}

func TestTargetOpsAppliesTargetOpsWorkingDirRemotely(t *testing.T) {
	ctx := NewRemoteWorkingDirContext(testContext(t), "/ambient")
	ctx = NewTargetContext(ctx, RemoteTarget)
	ctx = NewTargetOpsWorkingDirContext(ctx, "/target")

	ops := TargetOps(ctx).(*RemoteOps)
	if got, want := GetRemoteWorkingDir(ops.ctx), "/target"; got != want {
		t.Errorf("GetRemoteWorkingDir() = %q, want %q", got, want)
	}
}

func TestTargetOpsWorkingDirDoesNotLeakToDirectOps(t *testing.T) {
	ctx := NewLocalWorkingDirContext(testContext(t), "/ambient")
	ctx = NewTargetContext(ctx, LocalTarget)
	ctx = NewTargetOpsWorkingDirContext(ctx, "/target")

	if got, want := GetLocalWorkingDir(ctx), "/ambient"; got != want {
		t.Errorf("GetLocalWorkingDir(caller) = %q, want %q", got, want)
	}

	ops := TargetOps(ctx).(*LocalOps)
	if got, want := GetLocalWorkingDir(ops.ctx), "/target"; got != want {
		t.Errorf("GetLocalWorkingDir(target ops) = %q, want %q", got, want)
	}
}

func TestWithWorkingDirExtendsTargetOpsWorkingDir(t *testing.T) {
	base := t.TempDir()
	targetDir := filepath.Join(base, "target")

	ctx := NewLocalWorkingDirContext(testContext(t), filepath.Join(base, "ambient"))
	ctx = NewTargetContext(ctx, LocalTarget)
	ctx = NewTargetOpsWorkingDirContext(ctx, targetDir)

	want := filepath.Join(targetDir, "sub")

	ops := TargetOps(ctx).(*LocalOps)
	ops.WithWorkingDir("sub", func(ctx context.Context) {
		if got := GetLocalWorkingDir(ctx); got != want {
			t.Errorf("GetLocalWorkingDir(callback) = %q, want %q", got, want)
		}

		local := TargetOps(ctx).(*LocalOps)
		if got := GetLocalWorkingDir(local.ctx); got != want {
			t.Errorf("GetLocalWorkingDir(nested target ops) = %q, want %q", got, want)
		}
	})
}

func TestWithWorkingDirAbsoluteOverridesTargetOpsWorkingDir(t *testing.T) {
	base := t.TempDir()
	absolute := filepath.Join(base, "absolute")

	ctx := NewLocalWorkingDirContext(testContext(t), filepath.Join(base, "ambient"))
	ctx = NewTargetContext(ctx, LocalTarget)
	ctx = NewTargetOpsWorkingDirContext(ctx, filepath.Join(base, "target"))

	ops := TargetOps(ctx).(*LocalOps)
	ops.WithWorkingDir(absolute, func(ctx context.Context) {
		if got := GetLocalWorkingDir(ctx); got != absolute {
			t.Errorf("GetLocalWorkingDir(callback) = %q, want %q", got, absolute)
		}
	})
}

func TestTargetOpsWorkingDirMarkerIndependentFromExecEnv(t *testing.T) {
	base := t.TempDir()
	targetDir := filepath.Join(base, "target")
	absolute := filepath.Join(base, "absolute")

	ctx := NewLocalWorkingDirContext(testContext(t), filepath.Join(base, "ambient"))
	ctx = NewTargetContext(ctx, LocalTarget)
	ctx = NewTargetOpsWorkingDirContext(ctx, targetDir)

	ops := TargetOps(ctx).(*LocalOps)
	ops.WithWorkingDir(absolute, func(ctx context.Context) {
		ctx = NewTargetOpsExecEnvContext(ctx, Env("GABRIC_TEST_ENV", "value"))

		local := TargetOps(ctx).(*LocalOps)
		if got := GetLocalWorkingDir(local.ctx); got != absolute {
			t.Errorf("GetLocalWorkingDir() = %q, want %q", got, absolute)
		}
		if got, ok := GetLocalExecEnv(local.ctx).Get("GABRIC_TEST_ENV"); !ok || got != "value" {
			t.Errorf("Get(GABRIC_TEST_ENV) = %q, %v, want value, true", got, ok)
		}
	})
}

func TestTargetOpsExecEnvMarkerIndependentFromWorkingDir(t *testing.T) {
	ctx := NewTargetContext(testContext(t), LocalTarget)
	ctx = NewTargetOpsExecEnvContext(ctx, Env("GABRIC_TEST_ENV", "target"))

	ops := TargetOps(ctx).(*LocalOps)
	ops.WithExecEnv(NewExecEnv(Env("GABRIC_TEST_ENV", "scoped")), func(ctx context.Context) {
		ctx = NewTargetOpsWorkingDirContext(ctx, "/newdir")

		local := TargetOps(ctx).(*LocalOps)
		if got, want := GetLocalWorkingDir(local.ctx), "/newdir"; got != want {
			t.Errorf("GetLocalWorkingDir() = %q, want %q", got, want)
		}
		if got, ok := GetLocalExecEnv(local.ctx).Get("GABRIC_TEST_ENV"); !ok || got != "scoped" {
			t.Errorf("Get(GABRIC_TEST_ENV) = %q, %v, want scoped, true", got, ok)
		}
	})
}

func TestAppendTargetOpsExecEnvContextAppliesImmediately(t *testing.T) {
	ctx := NewTargetContext(testContext(t), LocalTarget)
	ctx = NewTargetOpsExecEnvContext(ctx, Env("A", "1"))

	applied := TargetOps(ctx).(*LocalOps).ctx
	if !targetOpsExecEnvApplied(applied) {
		t.Fatal("TargetOps() did not mark the exec env overlay as applied")
	}

	ctx = AppendTargetOpsExecEnvContext(applied, Env("B", "2"))
	if !targetOpsExecEnvApplied(ctx) {
		t.Error("AppendTargetOpsExecEnvContext() cleared the applied marker")
	}
	if got, ok := GetLocalExecEnv(ctx).Get("B"); !ok || got != "2" {
		t.Errorf("Get(B) = %q, %v, want 2, true", got, ok)
	}
	if got, ok := GetTargetOpsExecEnv(ctx).Get("B"); !ok || got != "2" {
		t.Errorf("GetTargetOpsExecEnv(B) = %q, %v, want 2, true", got, ok)
	}

	local := TargetOps(ctx).(*LocalOps)
	if got, ok := GetLocalExecEnv(local.ctx).Get("B"); !ok || got != "2" {
		t.Errorf("Get(B) after TargetOps = %q, %v, want 2, true", got, ok)
	}
}

func TestNewTargetOpsExecEnvContextAppliesImmediately(t *testing.T) {
	ctx := NewTargetContext(testContext(t), LocalTarget)
	ctx = NewTargetOpsExecEnvContext(ctx, Env("A", "1"))

	applied := TargetOps(ctx).(*LocalOps).ctx
	ctx = NewTargetOpsExecEnvContext(applied, Env("B", "2"))

	if !targetOpsExecEnvApplied(ctx) {
		t.Error("NewTargetOpsExecEnvContext() cleared the applied marker")
	}
	if got, ok := GetLocalExecEnv(ctx).Get("B"); !ok || got != "2" {
		t.Errorf("Get(B) = %q, %v, want 2, true", got, ok)
	}
	if _, ok := GetTargetOpsExecEnv(ctx).Get("A"); ok {
		t.Error("target ops overlay still contains A after replacing it")
	}
}

func TestNewTargetOpsWorkingDirContextAppliesImmediately(t *testing.T) {
	ctx := NewTargetContext(testContext(t), LocalTarget)
	ctx = NewTargetOpsWorkingDirContext(ctx, "/old")

	applied := TargetOps(ctx).(*LocalOps).ctx
	if !targetOpsWorkingDirApplied(applied) {
		t.Fatal("TargetOps() did not mark the working dir overlay as applied")
	}

	ctx = NewTargetOpsWorkingDirContext(applied, "/new")
	if !targetOpsWorkingDirApplied(ctx) {
		t.Error("NewTargetOpsWorkingDirContext() cleared the applied marker")
	}
	if got, want := GetLocalWorkingDir(ctx), "/new"; got != want {
		t.Errorf("GetLocalWorkingDir() = %q, want %q", got, want)
	}

	local := TargetOps(ctx).(*LocalOps)
	if got, want := GetLocalWorkingDir(local.ctx), "/new"; got != want {
		t.Errorf("GetLocalWorkingDir() after TargetOps = %q, want %q", got, want)
	}
}

func TestTargetOpsExecEnvDoesNotOverrideScopedExecEnv(t *testing.T) {
	ctx := NewTargetContext(testContext(t), LocalTarget)
	ctx = NewTargetOpsExecEnvContext(ctx, Env("GABRIC_TEST_ENV", "target"))

	ops := TargetOps(ctx).(*LocalOps)
	ops.WithExecEnv(NewExecEnv(Env("GABRIC_TEST_ENV", "scoped")), func(ctx context.Context) {
		local := TargetOps(ctx).(*LocalOps)
		if got, ok := GetLocalExecEnv(local.ctx).Get("GABRIC_TEST_ENV"); !ok || got != "scoped" {
			t.Errorf("Get(GABRIC_TEST_ENV) = %q, %v, want scoped, true", got, ok)
		}
	})
}

func TestNewTargetContextClearsAppliedMarkers(t *testing.T) {
	ctx := NewTargetContext(testContext(t), LocalTarget)
	ctx = NewTargetOpsWorkingDirContext(ctx, "/target")
	ctx = NewTargetOpsExecEnvContext(ctx, Env("GABRIC_TEST_ENV", "value"))

	applied := TargetOps(ctx).(*LocalOps).ctx
	if !targetOpsWorkingDirApplied(applied) || !targetOpsExecEnvApplied(applied) {
		t.Fatal("TargetOps() did not mark target ops overlays as applied")
	}

	applied = NewTargetContext(applied, LocalTarget)
	if targetOpsWorkingDirApplied(applied) || targetOpsExecEnvApplied(applied) {
		t.Error("NewTargetContext() did not clear the applied markers")
	}
}

func TestTargetOpsExecEnvDoesNotLeakToDirectOps(t *testing.T) {
	ctx := NewLocalExecEnvContext(testContext(t), Env("A", "1"))
	ctx = NewTargetContext(ctx, LocalTarget)
	ctx = AppendTargetOpsExecEnvContext(ctx, Env("B", "2"))

	if _, ok := GetLocalExecEnv(ctx).Get("B"); ok {
		t.Error("Get(B) in the caller context = true, want the overlay to stay target-scoped")
	}

	ops := TargetOps(ctx).(*LocalOps)
	if got, ok := GetLocalExecEnv(ops.ctx).Get("B"); !ok || got != "2" {
		t.Errorf("Get(B) on target ops = %q, %v, want 2, true", got, ok)
	}
}
