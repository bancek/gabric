package gabric

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"slices"
	"testing"

	"github.com/bancek/gabric/dryrun"
	"github.com/bancek/gabric/logger"
	"github.com/bancek/gabric/middleware"
)

func testLoggerContext() context.Context {
	return logger.NewLoggerContext(context.Background(), slog.New(slog.NewTextHandler(io.Discard, nil)))
}

func TestBuiltinMiddlewares(t *testing.T) {
	g := New()
	g.DryRun = true

	var got context.Context

	run := middleware.Apply(g.BuiltinMiddlewares(), func(ctx context.Context) error {
		got = ctx
		return nil
	})

	if err := run(testLoggerContext()); err != nil {
		t.Fatalf("run error: %v", err)
	}

	if !dryrun.Enabled(got) {
		t.Error("built-ins did not mark the context as a dry run")
	}
	if _, ok := TargetOps(got).(*RemoteOps); !ok {
		t.Errorf("TargetOps() = %T, want *RemoteOps", TargetOps(got))
	}
}

func TestGlobalMiddlewaresReturnsCopy(t *testing.T) {
	g := New()

	mw := func(ctx context.Context, next middleware.Next) error {
		return next(ctx)
	}

	g.Use(mw)

	first := g.GlobalMiddlewares()
	first[0] = nil

	if g.GlobalMiddlewares()[0] == nil {
		t.Error("GlobalMiddlewares() returned the live slice")
	}
}

func TestAllMiddlewaresIncludesBuiltinsAndGlobal(t *testing.T) {
	g := New()
	g.Use(func(ctx context.Context, next middleware.Next) error { return next(ctx) })

	all := g.AllMiddlewares(func(ctx context.Context, next middleware.Next) error { return next(ctx) })

	// Core built-ins are the default target plus the main and global middleware.
	if got, want := len(all), 1+1+1; got != want {
		t.Errorf("len(AllMiddlewares()) = %d, want %d", got, want)
	}
}

func TestAllMiddlewaresOrderWithUseBuiltin(t *testing.T) {
	g := New()

	var order []string

	record := func(name string) middleware.Middleware {
		return func(ctx context.Context, next middleware.Next) error {
			order = append(order, name)
			return next(ctx)
		}
	}

	g.UseBuiltin(record("builtin"))
	g.Use(record("global"))

	main := record("main")

	run := middleware.Apply(g.AllMiddlewares(main), func(context.Context) error {
		order = append(order, "terminal")
		return nil
	})

	if err := run(testContext(t)); err != nil {
		t.Fatalf("run error: %v", err)
	}

	if want := []string{"builtin", "main", "global", "terminal"}; !slices.Equal(order, want) {
		t.Errorf("order = %v, want %v", order, want)
	}
}

func TestDispatchRunsCallsInOrder(t *testing.T) {
	g := New()

	var order []string

	g.Task("run", func(ctx context.Context) {
		order = append(order, GetCurrentTaskArg(ctx))
	})

	run := Dispatch(parseCalls(t, g, "run:first", "run:second"))

	if err := run(testContext(t)); err != nil {
		t.Fatalf("Dispatch() error: %v", err)
	}

	want := []string{"first", "second"}
	if !slices.Equal(order, want) {
		t.Errorf("order = %v, want %v", order, want)
	}
}

func TestDispatchStopsOnFirstError(t *testing.T) {
	g := New()

	sentinel := errors.New("boom")
	secondRan := false

	g.TaskErr("fail", func(ctx context.Context) error {
		return sentinel
	})
	g.Task("second", func(ctx context.Context) {
		secondRan = true
	})

	run := Dispatch(parseCalls(t, g, "fail", "second"))

	if err := run(testContext(t)); !errors.Is(err, sentinel) {
		t.Fatalf("Dispatch() error = %v, want %v", err, sentinel)
	}

	if secondRan {
		t.Error("Dispatch() continued after an error")
	}
}
