package gabric

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/bancek/gabric/reconcile"
)

func TestWithReconcileSharedRun(t *testing.T) {
	ctx := testContext(t)

	g := New()
	g.Use(WithReconcile)

	evaluations := 0
	node := reconcile.NewNode("shared", func(ctx context.Context) (int, error) {
		evaluations++
		return 42, nil
	})

	reloads := 0
	topic := reconcile.NewTopic("reload")
	reconcile.NewEffect("reload", []*reconcile.Topic{topic}, func(ctx context.Context) error {
		reloads++
		return nil
	})

	g.TaskErr("a", func(ctx context.Context) error {
		v, err := node.Value(ctx)
		if err != nil {
			return err
		}

		if v != 42 {
			t.Errorf("Value() = %d, want 42", v)
		}

		return topic.Notify(ctx)
	})

	g.TaskErr("b", func(ctx context.Context) error {
		_, err := node.Value(ctx) // same run: not re-evaluated
		return err
	})

	if err := g.Run(ctx, parseCalls(t, g, "a", "b")); err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	if evaluations != 1 {
		t.Errorf("node evaluated %d times, want 1 (shared run)", evaluations)
	}

	if reloads != 1 {
		t.Errorf("effect ran %d times, want 1 (flushed once at the end)", reloads)
	}
}

func TestWithReconcileManualFlush(t *testing.T) {
	ctx := testContext(t)

	g := New()
	g.Use(WithReconcile)

	reloads := 0
	topic := reconcile.NewTopic("reload")
	reconcile.NewEffect("reload", []*reconcile.Topic{topic}, func(ctx context.Context) error {
		reloads++
		return nil
	})

	g.TaskErr("a", func(ctx context.Context) error {
		if err := topic.Notify(ctx); err != nil {
			return err
		}

		return reconcile.Flush(ctx)
	})

	if err := g.Run(ctx, parseCalls(t, g, "a")); err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	if reloads != 1 {
		t.Errorf("effect ran %d times, want 1", reloads)
	}
}

func TestWithReconcileErrorSkipsFlush(t *testing.T) {
	ctx := testContext(t)

	g := New()
	g.Use(WithReconcile)

	boom := errors.New("boom")
	reloads := 0
	topic := reconcile.NewTopic("reload")
	reconcile.NewEffect("reload", []*reconcile.Topic{topic}, func(ctx context.Context) error {
		reloads++
		return nil
	})

	g.TaskErr("a", func(ctx context.Context) error {
		if err := topic.Notify(ctx); err != nil {
			return err
		}

		return boom
	})

	err := g.Run(ctx, parseCalls(t, g, "a"))
	if !errors.Is(err, boom) {
		t.Fatalf("Run() error = %v, want %v", err, boom)
	}

	if reloads != 0 {
		t.Errorf("effect ran %d times, want 0 (flush skipped on error)", reloads)
	}
}

func TestGlobalComposition(t *testing.T) {
	ctx := testContext(t)

	g := New()

	var order []string

	first := func(ctx context.Context, next TaskFunc) error {
		order = append(order, "first")
		return next(ctx)
	}
	second := func(ctx context.Context, next TaskFunc) error {
		order = append(order, "second")
		return next(ctx)
	}

	g.Use(first, second)
	g.TaskErr("a", func(ctx context.Context) error {
		order = append(order, "task")
		return nil
	})

	if err := g.Run(ctx, parseCalls(t, g, "a")); err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	want := []string{"first", "second", "task"}

	if len(order) != len(want) {
		t.Fatalf("order = %v, want %v", order, want)
	}

	for i := range want {
		if order[i] != want[i] {
			t.Fatalf("order = %v, want %v", order, want)
		}
	}
}

func TestParallelSharesReconcileRun(t *testing.T) {
	ctx := testContext(t)

	g := New()
	g.Use(WithReconcile)

	var evaluations atomic.Int32

	dep := reconcile.NewNode("dep", func(ctx context.Context) (int, error) {
		evaluations.Add(1)
		time.Sleep(20 * time.Millisecond)

		return 42, nil
	})

	a := reconcile.NewNode("a", func(ctx context.Context) (int, error) {
		v, err := dep.Value(ctx)

		return v + 1, err
	})
	b := reconcile.NewNode("b", func(ctx context.Context) (int, error) {
		v, err := dep.Value(ctx)

		return v + 2, err
	})

	g.TaskErr("all", func(ctx context.Context) error {
		return ParallelErr(ctx,
			func(ctx context.Context) error {
				v, err := a.Value(ctx)
				if err != nil {
					return err
				}

				if v != 43 {
					return fmt.Errorf("a = %d, want 43", v)
				}

				return nil
			},
			func(ctx context.Context) error {
				v, err := b.Value(ctx)
				if err != nil {
					return err
				}

				if v != 44 {
					return fmt.Errorf("b = %d, want 44", v)
				}

				return nil
			},
		)
	})

	if err := g.Run(ctx, parseCalls(t, g, "all")); err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	if got := evaluations.Load(); got != 1 {
		t.Errorf("shared dependency evaluated %d times, want 1 (one shared run)", got)
	}
}
