package gabric

import (
	"context"
	"errors"
	"testing"
)

func TestMkdirOps(t *testing.T) {
	ctx := testContext(t)

	var got mkdirCall

	run := func(_ context.Context, call mkdirCall) error {
		got = call
		return nil
	}

	err := newMkdir(ctx, run, "/srv/app").
		Recursive().
		Mode(0o700).
		User("root").
		Group("root").
		RunErr()
	if err != nil {
		t.Fatalf("RunErr() = %v, want nil", err)
	}

	if got.path != "/srv/app" {
		t.Errorf("path = %q, want %q", got.path, "/srv/app")
	}

	if !got.recursive {
		t.Error("recursive = false, want true")
	}

	if got.mode != 0o700 {
		t.Errorf("mode = %v, want %v", got.mode, 0o700)
	}

	if got.user != "root" || got.group != "root" {
		t.Errorf("owner = %q:%q, want root:root", got.user, got.group)
	}
}

func TestMkdirOpsRunErr(t *testing.T) {
	ctx := testContext(t)
	boom := errors.New("boom")

	run := func(_ context.Context, call mkdirCall) error {
		return boom
	}

	if err := newMkdir(ctx, run, "/p").RunErr(); !errors.Is(err, boom) {
		t.Errorf("RunErr() = %v, want %v", err, boom)
	}
}

func TestMkdirOpsRunPanics(t *testing.T) {
	ctx := testContext(t)

	run := func(_ context.Context, call mkdirCall) error {
		return errors.New("boom")
	}

	defer func() {
		if recover() == nil {
			t.Error("Run() did not panic")
		}
	}()

	newMkdir(ctx, run, "/p").Run()
}
