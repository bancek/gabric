package gabric

import (
	"context"
	"errors"
	"testing"
)

func TestRemoveOps(t *testing.T) {
	ctx := testContext(t)

	var got removeCall

	run := func(_ context.Context, call removeCall) error {
		got = call
		return nil
	}

	err := newRemove(ctx, run, "/srv/app").
		Recursive().
		Force().
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

	if !got.force {
		t.Error("force = false, want true")
	}
}

func TestRemoveOpsRunErr(t *testing.T) {
	ctx := testContext(t)
	boom := errors.New("boom")

	run := func(_ context.Context, call removeCall) error {
		return boom
	}

	if err := newRemove(ctx, run, "/p").RunErr(); !errors.Is(err, boom) {
		t.Errorf("RunErr() = %v, want %v", err, boom)
	}
}

func TestRemoveOpsRunPanics(t *testing.T) {
	ctx := testContext(t)

	run := func(_ context.Context, call removeCall) error {
		return errors.New("boom")
	}

	defer func() {
		if recover() == nil {
			t.Error("Run() did not panic")
		}
	}()

	newRemove(ctx, run, "/p").Run()
}
