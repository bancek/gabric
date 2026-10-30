package gabric

import (
	"context"
	"errors"
	"testing"
)

func TestTaskWithoutTaskArgContext(t *testing.T) {
	ctx := testContext(t)

	var called bool

	task := New().Task("example", func(ctx context.Context) {
		called = true

		if arg := GetCurrentTaskArg(ctx); arg != "" {
			t.Errorf("GetCurrentTaskArg() = %q, want empty", arg)
		}
	})

	task(ctx)

	if !called {
		t.Fatal("task was not called")
	}
}

func TestTaskCallSetsArgument(t *testing.T) {
	g := New()

	var arg string

	g.Task("hello", func(ctx context.Context) {
		arg = GetCurrentTaskArg(ctx)
	})

	ctx := NewCurrentTaskArgContext(testContext(t), "outer")

	if err := parseCalls(t, g, "hello:world")[0].Call(ctx); err != nil {
		t.Fatalf("Call() error: %v", err)
	}

	if arg != "world" {
		t.Errorf("task arg = %q, want world", arg)
	}
}

func TestTaskCallEmptyArgumentClearsOuter(t *testing.T) {
	g := New()

	var arg string

	g.Task("hello", func(ctx context.Context) {
		arg = GetCurrentTaskArg(ctx)
	})

	ctx := NewCurrentTaskArgContext(testContext(t), "outer")

	if err := parseCalls(t, g, "hello")[0].Call(ctx); err != nil {
		t.Fatalf("Call() error: %v", err)
	}

	if arg != "" {
		t.Errorf("task arg = %q, want empty", arg)
	}
}

func TestTaskPanicRecovery(t *testing.T) {
	ctx := testContext(t)

	g := New()

	task := g.Task("boom", func(ctx context.Context) {
		panic("boom")
	})

	if err := g.tasks["boom"](ctx); err == nil {
		t.Error("registered task returned nil, want panic error")
	}

	defer func() {
		if recover() == nil {
			t.Error("task callable did not panic")
		}
	}()

	task(ctx)
}

func TestTaskErr(t *testing.T) {
	ctx := testContext(t)

	g := New()

	sentinel := errors.New("boom")

	fail := g.TaskErr("fail", func(ctx context.Context) error {
		return sentinel
	})

	if err := fail(ctx); !errors.Is(err, sentinel) {
		t.Errorf("TaskErr() error = %v, want %v", err, sentinel)
	}

	ok := g.TaskErr("ok", func(ctx context.Context) error {
		return nil
	})

	if err := ok(ctx); err != nil {
		t.Errorf("TaskErr() error = %v, want nil", err)
	}
}

func TestTaskDuplicate(t *testing.T) {
	g := New()

	g.Task("duplicate", func(ctx context.Context) {})

	defer func() {
		if recover() == nil {
			t.Error("duplicate Task() did not panic")
		}
	}()

	g.Task("duplicate", func(ctx context.Context) {})
}

func TestMust(t *testing.T) {
	Must(nil)

	defer func() {
		if recover() == nil {
			t.Error("Must() did not panic")
		}
	}()

	Must(errors.New("boom"))
}

func TestTaskPanicErrorIdentity(t *testing.T) {
	ctx := testContext(t)

	g := New()

	sentinel := errors.New("boom")

	g.Task("boom", func(ctx context.Context) {
		panic(sentinel)
	})

	if err := g.Run(ctx, parseCalls(t, g, "boom")); !errors.Is(err, sentinel) {
		t.Errorf("Run() error = %v, want %v", err, sentinel)
	}
}

func TestTaskNestedPanicErrorIdentity(t *testing.T) {
	ctx := testContext(t)

	g := New()

	sentinel := errors.New("boom")

	inner := g.Task("inner", func(ctx context.Context) {
		panic(sentinel)
	})
	g.Task("outer", func(ctx context.Context) {
		inner(ctx)
	})

	if err := g.Run(ctx, parseCalls(t, g, "outer")); !errors.Is(err, sentinel) {
		t.Errorf("Run() error = %v, want %v", err, sentinel)
	}
}

func TestTaskToTaskErr(t *testing.T) {
	ctx := testContext(t)

	sentinel := errors.New("boom")

	g := New()

	boom := g.Task("boom", func(ctx context.Context) {
		panic(sentinel)
	})

	if err := TaskToTaskErr(boom)(ctx); !errors.Is(err, sentinel) {
		t.Errorf("TaskToTaskErr() error = %v, want %v", err, sentinel)
	}

	ok := g.Task("ok", func(ctx context.Context) {})

	if err := TaskToTaskErr(ok)(ctx); err != nil {
		t.Errorf("TaskToTaskErr() error = %v, want nil", err)
	}
}

func TestTaskErrToTask(t *testing.T) {
	ctx := testContext(t)

	sentinel := errors.New("boom")

	task := TaskErrToTask(func(ctx context.Context) error {
		return sentinel
	})

	defer func() {
		r := recover()
		if r == nil {
			t.Error("TaskErrToTask() callable did not panic")
			return
		}

		err, ok := r.(error)
		if !ok || !errors.Is(err, sentinel) {
			t.Errorf("panic value = %v, want %v", r, sentinel)
		}
	}()

	task(ctx)
}
