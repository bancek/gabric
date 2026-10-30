package gabric

import (
	"context"
	"errors"
	"slices"
	"sync"
	"testing"
	"time"
)

func TestParallelRunsAll(t *testing.T) {
	ctx := testContext(t)

	var mu sync.Mutex

	var ran []string

	task := func(name string) func(ctx context.Context) {
		return func(ctx context.Context) {
			mu.Lock()
			defer mu.Unlock()

			ran = append(ran, name)
		}
	}

	Parallel(ctx, task("a"), task("b"), task("c"))

	mu.Lock()
	defer mu.Unlock()

	slices.Sort(ran)

	want := []string{"a", "b", "c"}
	if !slices.Equal(ran, want) {
		t.Errorf("ran = %v, want %v", ran, want)
	}
}

func TestParallelWaitsForAllOnPanic(t *testing.T) {
	ctx := testContext(t)

	var ran bool

	defer func() {
		if recover() == nil {
			t.Error("Parallel() did not panic")
		}

		if !ran {
			t.Error("sibling task did not finish despite a panic")
		}
	}()

	Parallel(ctx,
		func(ctx context.Context) { panic("boom") },
		func(ctx context.Context) { ran = true },
	)
}

func TestParallelPanicsJoined(t *testing.T) {
	ctx := testContext(t)

	first := errors.New("first")
	second := errors.New("second")

	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("Parallel() did not panic")
		}

		err, ok := r.(error)
		if !ok {
			t.Fatalf("panic value = %T, want error", r)
		}

		if !errors.Is(err, first) || !errors.Is(err, second) {
			t.Errorf("panic error = %v, want both %v and %v", err, first, second)
		}
	}()

	Parallel(ctx,
		func(ctx context.Context) { panic(first) },
		func(ctx context.Context) { panic(second) },
	)
}

func TestParallelPanicNonError(t *testing.T) {
	ctx := testContext(t)

	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("Parallel() did not panic")
		}

		err, ok := r.(error)
		if !ok {
			t.Fatalf("panic value = %T, want error", r)
		}

		if err.Error() != "boom" {
			t.Errorf("panic error = %q, want %q", err.Error(), "boom")
		}
	}()

	Parallel(ctx, func(ctx context.Context) { panic("boom") })
}

func TestParallelConcurrent(t *testing.T) {
	ctx := testContext(t)

	done := make(chan struct{})

	go func() {
		defer close(done)

		var wg sync.WaitGroup
		wg.Add(2)

		rendezvous := func(ctx context.Context) {
			wg.Done()
			wg.Wait()
		}

		Parallel(ctx, rendezvous, rendezvous)
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("tasks did not run concurrently")
	}
}

func TestParallelErrJoins(t *testing.T) {
	ctx := testContext(t)

	first := errors.New("first")
	second := errors.New("second")

	err := ParallelErr(ctx,
		func(ctx context.Context) error { return first },
		func(ctx context.Context) error { return nil },
		func(ctx context.Context) error { return second },
	)

	if !errors.Is(err, first) || !errors.Is(err, second) {
		t.Errorf("ParallelErr() = %v, want both %v and %v", err, first, second)
	}
}

func TestParallelErrNil(t *testing.T) {
	ctx := testContext(t)

	err := ParallelErr(ctx,
		func(ctx context.Context) error { return nil },
		func(ctx context.Context) error { return nil },
	)

	if err != nil {
		t.Errorf("ParallelErr() = %v, want nil", err)
	}
}

func TestParallelErrFuncSlices(t *testing.T) {
	ctx := testContext(t)

	funcs := []func(context.Context) error{
		func(ctx context.Context) error { return nil },
	}
	if err := ParallelErr(ctx, funcs...); err != nil {
		t.Errorf("ParallelErr(funcs...) = %v, want nil", err)
	}

	tasks := []TaskFunc{
		func(ctx context.Context) error { return nil },
	}
	if err := ParallelErr(ctx, tasks...); err != nil {
		t.Errorf("ParallelErr(tasks...) = %v, want nil", err)
	}
}

func TestParallelEmpty(t *testing.T) {
	ctx := testContext(t)

	Parallel(ctx)

	if err := ParallelErr(ctx); err != nil {
		t.Errorf("ParallelErr() = %v, want nil", err)
	}
}

// TestParallelPanicErrorIdentity checks that a panic-style task's panic keeps
// error identity in the joined panic value.
func TestParallelPanicErrorIdentity(t *testing.T) {
	ctx := testContext(t)

	sentinel := errors.New("boom")

	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("Parallel() did not panic")
		}

		err, ok := r.(error)
		if !ok || !errors.Is(err, sentinel) {
			t.Fatalf("panic value = %v, want %v", r, sentinel)
		}
	}()

	Parallel(ctx, func(ctx context.Context) { panic(sentinel) })
}
