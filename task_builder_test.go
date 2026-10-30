package gabric

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/bancek/gabric/middleware"
	"github.com/bancek/gabric/reconcile"
)

func TestTaskBuilderTask(t *testing.T) {
	ctx := testContext(t)

	g := New()

	var order []string

	task := g.With(func(ctx context.Context, next TaskFunc) error {
		order = append(order, "middleware")
		return next(ctx)
	}).Task("task", func(ctx context.Context) {
		order = append(order, "task")
	})

	task(ctx)

	want := []string{"middleware", "task"}
	if !slices.Equal(order, want) {
		t.Errorf("order = %v, want %v", order, want)
	}
}

func TestTaskBuilderTaskErr(t *testing.T) {
	ctx := testContext(t)

	g := New()

	sentinel := errors.New("boom")

	task := g.With(func(ctx context.Context, next TaskFunc) error {
		return next(ctx)
	}).TaskErr("task", func(ctx context.Context) error {
		return sentinel
	})

	if err := task(ctx); !errors.Is(err, sentinel) {
		t.Errorf("TaskErr() error = %v, want %v", err, sentinel)
	}
}

func TestTaskBuilderImmutable(t *testing.T) {
	ctx := testContext(t)

	g := New()

	var order []string

	record := func(label string) middleware.Middleware {
		return func(ctx context.Context, next TaskFunc) error {
			order = append(order, label)
			return next(ctx)
		}
	}

	base := g.With(record("base"))
	baseTask := base.Task("base", func(ctx context.Context) {
		order = append(order, "task")
	})

	baseTask(ctx)
	if want := []string{"base", "task"}; !slices.Equal(order, want) {
		t.Errorf("base order = %v, want %v", order, want)
	}

	order = nil
	extended := base.With(record("extra"))
	extendedTask := extended.Task("extended", func(ctx context.Context) {
		order = append(order, "task")
	})

	baseTask(ctx)
	if want := []string{"base", "task"}; !slices.Equal(order, want) {
		t.Errorf("base order after With = %v, want %v (base must not change)", order, want)
	}

	order = nil
	extendedTask(ctx)
	if want := []string{"base", "extra", "task"}; !slices.Equal(order, want) {
		t.Errorf("extended order = %v, want %v", order, want)
	}
}

func TestUseReturnsGabric(t *testing.T) {
	g := New()

	if got := g.Use(WithRemoteSudo(&Sudo{})); got != g {
		t.Error("Use() did not return the Gabric")
	}
}

func TestGlobalMiddlewareAppliedAroundRun(t *testing.T) {
	ctx := testContext(t)

	g := New()

	var order []string

	g.Use(func(ctx context.Context, next TaskFunc) error {
		order = append(order, "middleware")
		return next(ctx)
	})

	g.TaskErr("a", func(ctx context.Context) error {
		order = append(order, "task")
		return nil
	})

	if err := g.Run(ctx, parseCalls(t, g, "a")); err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	want := []string{"middleware", "task"}
	if !slices.Equal(order, want) {
		t.Errorf("order = %v, want %v", order, want)
	}
}

func TestWithDefault(t *testing.T) {
	Default.mu.Lock()
	old := Default.globalMiddlewares
	Default.mu.Unlock()

	t.Cleanup(func() {
		Default.mu.Lock()
		Default.globalMiddlewares = old
		Default.mu.Unlock()
	})

	if got := Use(WithRemoteSudo(&Sudo{})); got != Default {
		t.Errorf("Use() = %p, want Default %p", got, Default)
	}
}

func TestNodeTaskFlushes(t *testing.T) {
	ctx := testContext(t)

	g := New()
	g.Use(WithReconcile)

	evaluated := false
	flushed := false

	topic := reconcile.NewTopic("t")

	reconcile.NewEffect("e", []*reconcile.Topic{topic}, func(ctx context.Context) error {
		flushed = true
		return nil
	})

	node := reconcile.Resource("node", nil,
		func(ctx context.Context) (reconcile.Void, error) {
			evaluated = true
			return reconcile.Void{}, topic.Notify(ctx)
		}).
		Node()

	g.With().NodeTask(node)

	if err := g.Run(ctx, parseCalls(t, g, "node")); err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	if !evaluated {
		t.Error("node was not evaluated")
	}

	if !flushed {
		t.Error("NodeTask did not flush the node's effect")
	}
}

func TestNodeTaskWithTaskReconcileStartsFreshRun(t *testing.T) {
	ctx := testContext(t)

	g := New()

	evaluations := 0
	flushes := 0

	topic := reconcile.NewTopic("t")

	reconcile.NewEffect("e", []*reconcile.Topic{topic}, func(ctx context.Context) error {
		flushes++
		return nil
	})

	node := reconcile.Resource("node", nil,
		func(ctx context.Context) (reconcile.Void, error) {
			evaluations++
			return reconcile.Void{}, topic.Notify(ctx)
		}).
		Node()

	task := g.With(WithReconcile).NodeTask(node)

	task(ctx)
	task(ctx)

	if evaluations != 2 {
		t.Errorf("node evaluated %d times, want 2 (one run per task call)", evaluations)
	}

	if flushes != 2 {
		t.Errorf("effect flushed %d times, want 2 (one run per task call)", flushes)
	}
}

func TestNodeTaskErrPropagates(t *testing.T) {
	ctx := testContext(t)

	g := New()
	g.Use(WithReconcile)

	sentinel := errors.New("boom")

	node := reconcile.Data("node", func(ctx context.Context) (reconcile.Void, error) {
		return reconcile.Void{}, sentinel
	}).Node()

	g.With().NodeTaskErr(node)

	if err := g.Run(ctx, parseCalls(t, g, "node")); !errors.Is(err, sentinel) {
		t.Errorf("Run() error = %v, want %v", err, sentinel)
	}
}

func TestNodeTaskErrorBecomesTaskError(t *testing.T) {
	ctx := testContext(t)

	g := New()
	g.Use(WithReconcile)

	node := reconcile.Data("node", func(ctx context.Context) (reconcile.Void, error) {
		return reconcile.Void{}, errors.New("boom")
	}).Node()

	g.With().NodeTask(node) // panic-style

	if err := g.Run(ctx, parseCalls(t, g, "node")); err == nil {
		t.Error("Run() error = nil, want the node's error")
	}
}
