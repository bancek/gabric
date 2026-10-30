package gabric

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"slices"
	"sync"
	"testing"

	"github.com/bancek/gabric/middleware"
)

func TestNew(t *testing.T) {
	g := New()
	if g == nil {
		t.Fatal("New() = nil")
	}

	if g.FlagSet == nil {
		t.Fatal("FlagSet = nil")
	}

	if g.Stdout == nil || g.Stderr == nil {
		t.Error("Stdout/Stderr = nil")
	}

	if g.Level != slog.LevelDebug {
		t.Errorf("Level = %v, want debug", g.Level)
	}

	if g.LogFormat != LogFormatText {
		t.Errorf("LogFormat = %v, want text", g.LogFormat)
	}

	for _, name := range []string{"level", "log-format", "dry-run"} {
		if g.FlagSet.Lookup(name) == nil {
			t.Errorf("FlagSet has no -%s flag", name)
		}
	}

	for _, name := range []string{"H", "port", "i"} {
		if g.FlagSet.Lookup(name) != nil {
			t.Errorf("FlagSet still has the removed -%s flag", name)
		}
	}

	if g.FlagSet.Lookup("set") != nil {
		t.Error("FlagSet still has the removed -set flag")
	}

	if Default == nil {
		t.Fatal("Default = nil")
	}
	if Default.FlagSet == nil {
		t.Fatal("Default.FlagSet = nil")
	}
}

func TestNewCopiesArgs(t *testing.T) {
	g := New()

	if !slices.Equal(g.Args, os.Args[1:]) {
		t.Errorf("Args = %v, want os.Args[1:]", g.Args)
	}

	if len(g.Args) > 0 {
		original := g.Args[0]
		g.Args[0] = original + "-changed"

		if os.Args[1] == g.Args[0] {
			t.Error("Args aliases os.Args")
		}
	}
}

func TestGabricIsolation(t *testing.T) {
	a := New()
	b := New()

	a.Task("only-a", func(ctx context.Context) {})

	if _, ok := b.getTask("only-a"); ok {
		t.Error("task registered in one Gabric is visible in another")
	}
}

func TestTaskConcurrentRegistration(t *testing.T) {
	g := New()

	const n = 50

	var wg sync.WaitGroup

	for i := 0; i < n; i++ {
		wg.Add(1)

		go func(i int) {
			defer wg.Done()

			g.Task(fmt.Sprintf("task-%d", i), func(ctx context.Context) {})
		}(i)
	}

	wg.Wait()

	if got := len(g.getTaskNames()); got != n {
		t.Errorf("registered tasks = %d, want %d", got, n)
	}
}

func TestUseBuiltin(t *testing.T) {
	g := New()

	called := false

	builtin := func(ctx context.Context, next middleware.Next) error {
		called = true
		return next(ctx)
	}

	if got := g.UseBuiltin(builtin); got != g {
		t.Error("UseBuiltin() did not return the receiver")
	}

	run := middleware.Apply(g.BuiltinMiddlewares(), func(context.Context) error {
		return nil
	})

	if err := run(testContext(t)); err != nil {
		t.Fatalf("run error: %v", err)
	}

	if !called {
		t.Error("UseBuiltin() middleware was not called")
	}
}
