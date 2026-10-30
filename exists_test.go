package gabric

import (
	"context"
	"errors"
	"testing"
)

func TestExistsOps(t *testing.T) {
	ctx := testContext(t)

	var got existsCall

	run := func(_ context.Context, call existsCall) (bool, error) {
		got = call
		return true, nil
	}

	ok, err := newExists(ctx, run, "/srv/app").RunErr()
	if err != nil {
		t.Fatalf("RunErr() = %v, want nil", err)
	}
	if !ok {
		t.Error("RunErr() = false, want true")
	}

	if got.path != "/srv/app" {
		t.Errorf("path = %q, want %q", got.path, "/srv/app")
	}

	if got.kind != FileKindAny {
		t.Errorf("kind = %v, want FileKindAny", got.kind)
	}
}

func TestExistsOpsKind(t *testing.T) {
	ctx := testContext(t)

	tests := []struct {
		name string
		set  func(e *ExistsOps)
		want FileKind
	}{
		{name: "default", set: func(e *ExistsOps) {}, want: FileKindAny},
		{name: "dir", set: func(e *ExistsOps) { e.Dir() }, want: FileKindDir},
		{name: "file", set: func(e *ExistsOps) { e.File() }, want: FileKindFile},
		{name: "link", set: func(e *ExistsOps) { e.Link() }, want: FileKindLink},
		{name: "last wins", set: func(e *ExistsOps) { e.Dir().File() }, want: FileKindFile},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got existsCall

			run := func(_ context.Context, call existsCall) (bool, error) {
				got = call
				return false, nil
			}

			e := newExists(ctx, run, "/p")
			tt.set(e)

			if _, err := e.RunErr(); err != nil {
				t.Fatalf("RunErr() = %v, want nil", err)
			}

			if got.kind != tt.want {
				t.Errorf("kind = %v, want %v", got.kind, tt.want)
			}
		})
	}
}

func TestExistsOpsRunErr(t *testing.T) {
	ctx := testContext(t)
	boom := errors.New("boom")

	run := func(_ context.Context, call existsCall) (bool, error) {
		return false, boom
	}

	if _, err := newExists(ctx, run, "/p").RunErr(); !errors.Is(err, boom) {
		t.Errorf("RunErr() = %v, want %v", err, boom)
	}
}

func TestExistsOpsRunPanics(t *testing.T) {
	ctx := testContext(t)

	run := func(_ context.Context, call existsCall) (bool, error) {
		return false, errors.New("boom")
	}

	defer func() {
		if recover() == nil {
			t.Error("Run() did not panic")
		}
	}()

	newExists(ctx, run, "/p").Run()
}
