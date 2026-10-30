package gabric

import (
	"context"
	"errors"
	"testing"
)

func TestStatOps(t *testing.T) {
	ctx := testContext(t)
	want := FileStat{Mode: 0o644, User: "root", Group: "root"}

	var got statCall

	run := func(_ context.Context, call statCall) (FileStat, error) {
		got = call
		return want, nil
	}

	stat := newStat(ctx, run, "/srv/app").Run()
	if stat != want {
		t.Errorf("Run() = %+v, want %+v", stat, want)
	}

	if got.path != "/srv/app" {
		t.Errorf("path = %q, want %q", got.path, "/srv/app")
	}
}

func TestStatOpsRunErr(t *testing.T) {
	ctx := testContext(t)
	boom := errors.New("boom")

	run := func(_ context.Context, call statCall) (FileStat, error) {
		return FileStat{}, boom
	}

	if _, err := newStat(ctx, run, "/p").RunErr(); !errors.Is(err, boom) {
		t.Errorf("RunErr() = %v, want %v", err, boom)
	}
}

func TestStatOpsRunPanics(t *testing.T) {
	ctx := testContext(t)

	run := func(_ context.Context, call statCall) (FileStat, error) {
		return FileStat{}, errors.New("boom")
	}

	defer func() {
		if recover() == nil {
			t.Error("Run() did not panic")
		}
	}()

	newStat(ctx, run, "/p").Run()
}
