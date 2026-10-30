package gabric

import (
	"bytes"
	"context"
	"errors"
	"flag"
	"log/slog"
	"regexp"
	"strings"
	"testing"

	"github.com/bancek/gabric/dryrun"
	"github.com/bancek/gabric/logger"
	"github.com/bancek/gabric/middleware"
)

func parseCalls(t *testing.T, g *Gabric, args ...string) []TaskCall {
	t.Helper()

	calls, err := g.ParseTasks(args)
	if err != nil {
		t.Fatalf("ParseTasks(%v) error: %v", args, err)
	}

	return calls
}

func TestRun(t *testing.T) {
	ctx := testContext(t)

	g := New()

	var arg string

	g.Task("hello", func(ctx context.Context) {
		arg = GetCurrentTaskArg(ctx)
	})

	if err := g.Run(ctx, parseCalls(t, g, "hello:world")); err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	if arg != "world" {
		t.Errorf("task arg = %q, want %q", arg, "world")
	}
}

func TestRunNoArg(t *testing.T) {
	ctx := testContext(t)

	g := New()

	var arg string

	g.Task("hello", func(ctx context.Context) {
		arg = GetCurrentTaskArg(ctx)
	})

	if err := g.Run(ctx, parseCalls(t, g, "hello")); err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	if arg != "" {
		t.Errorf("task arg = %q, want empty", arg)
	}
}

func TestRunNoArgClearsCurrentArg(t *testing.T) {
	ctx := NewCurrentTaskArgContext(testContext(t), "outer")

	g := New()

	var arg string

	g.Task("hello", func(ctx context.Context) {
		arg = GetCurrentTaskArg(ctx)
	})

	if err := g.Run(ctx, parseCalls(t, g, "hello")); err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	if arg != "" {
		t.Errorf("task arg = %q, want empty (a top-level call resets the current arg)", arg)
	}
}

func TestParseTasksUnknownTask(t *testing.T) {
	if _, err := New().ParseTasks([]string{"missing"}); err == nil {
		t.Error("ParseTasks() with unknown task = nil, want error")
	}
}

func TestRunDuplicateCallsKeepArguments(t *testing.T) {
	ctx := testContext(t)

	g := New()

	var args []string

	g.Task("hello", func(ctx context.Context) {
		args = append(args, GetCurrentTaskArg(ctx))
	})

	if err := g.Run(ctx, parseCalls(t, g, "hello:first", "hello:second")); err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	want := []string{"first", "second"}
	if len(args) != len(want) {
		t.Fatalf("args = %v, want %v", args, want)
	}

	for i := range want {
		if args[i] != want[i] {
			t.Fatalf("args = %v, want %v", args, want)
		}
	}
}

func TestRunReturnsTaskError(t *testing.T) {
	ctx := testContext(t)

	g := New()

	sentinel := errors.New("boom")

	g.TaskErr("boom", func(ctx context.Context) error {
		return sentinel
	})

	if err := g.Run(ctx, parseCalls(t, g, "boom")); !errors.Is(err, sentinel) {
		t.Errorf("Run() error = %v, want %v", err, sentinel)
	}
}

func TestRunStopsOnFirstError(t *testing.T) {
	ctx := testContext(t)

	g := New()

	sentinel := errors.New("boom")

	var secondRan bool

	g.TaskErr("boom", func(ctx context.Context) error {
		return sentinel
	})
	g.Task("second", func(ctx context.Context) {
		secondRan = true
	})

	if err := g.Run(ctx, parseCalls(t, g, "boom", "second")); !errors.Is(err, sentinel) {
		t.Fatalf("Run() error = %v, want %v", err, sentinel)
	}

	if secondRan {
		t.Error("Run() continued after a task error")
	}
}

func TestParseFlags(t *testing.T) {
	g := New()
	g.Stderr = &bytes.Buffer{}

	err := g.ParseFlags([]string{
		"-level", "info",
		"-log-format", "json",
		"-dry-run",
	})
	if err != nil {
		t.Fatalf("ParseFlags() error: %v", err)
	}

	if g.Level != slog.LevelInfo {
		t.Errorf("Level = %v, want info", g.Level)
	}
	if g.LogFormat != LogFormatJSON {
		t.Errorf("LogFormat = %v, want json", g.LogFormat)
	}
	if !g.DryRun {
		t.Error("DryRun = false, want true")
	}
}

func TestParseFlagsInvalidLevel(t *testing.T) {
	g := New()
	g.Stderr = &bytes.Buffer{}

	if err := g.ParseFlags([]string{"-level", "nope"}); err == nil {
		t.Error("ParseFlags() with invalid level = nil, want error")
	}
}

func TestParseFlagsInvalidLogFormat(t *testing.T) {
	g := New()
	g.Stderr = &bytes.Buffer{}

	if err := g.ParseFlags([]string{"-log-format", "yaml"}); err == nil {
		t.Error("ParseFlags() with invalid log format = nil, want error")
	}
}

func TestParseFlagsHelp(t *testing.T) {
	g := New()
	buf := &bytes.Buffer{}
	g.Stderr = buf

	err := g.ParseFlags([]string{"-h"})
	if !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("ParseFlags(-h) error = %v, want flag.ErrHelp", err)
	}

	if !strings.Contains(buf.String(), "Usage") {
		t.Errorf("help output = %q, want usage", buf.String())
	}
}

func TestMainDryRun(t *testing.T) {
	g := New()
	g.Args = []string{"-dry-run", "check"}
	g.Stdout = &bytes.Buffer{}
	g.Stderr = &bytes.Buffer{}

	var got bool

	g.Task("check", func(ctx context.Context) {
		got = dryrun.Enabled(ctx)
	})

	if code := g.runMain(mainOptions{ctx: context.Background()}); code != 0 {
		t.Fatalf("runMain() = %d, want 0", code)
	}

	if !got {
		t.Error("dryrun.Enabled(ctx) = false, want true")
	}
}

func TestMainListTasks(t *testing.T) {
	g := New()
	g.Args = nil
	g.Stdout = &bytes.Buffer{}
	g.Stderr = &bytes.Buffer{}

	g.Task("b", func(ctx context.Context) {})
	g.Task("a", func(ctx context.Context) {})

	if code := g.runMain(mainOptions{ctx: context.Background()}); code != 0 {
		t.Fatalf("runMain() = %d, want 0", code)
	}

	if got, want := g.Stdout.(*bytes.Buffer).String(), "a\nb\n"; got != want {
		t.Errorf("task list = %q, want %q", got, want)
	}
}

func TestMainHelp(t *testing.T) {
	g := New()
	g.Args = []string{"-h"}
	g.Stderr = &bytes.Buffer{}

	if code := g.runMain(mainOptions{ctx: context.Background()}); code != 0 {
		t.Fatalf("runMain(-h) = %d, want 0", code)
	}

	if !strings.Contains(g.Stderr.(*bytes.Buffer).String(), "Usage") {
		t.Errorf("help output = %q, want usage", g.Stderr.(*bytes.Buffer).String())
	}
}

func TestMainInvalidFlag(t *testing.T) {
	g := New()
	g.Args = []string{"-nope"}
	g.Stderr = &bytes.Buffer{}

	if code := g.runMain(mainOptions{ctx: context.Background()}); code != 2 {
		t.Fatalf("runMain(-nope) = %d, want 2", code)
	}
}

func TestMainUnknownTask(t *testing.T) {
	g := New()
	g.Args = []string{"missing"}
	g.Stderr = &bytes.Buffer{}

	if code := g.runMain(mainOptions{ctx: context.Background()}); code != 1 {
		t.Fatalf("runMain(missing) = %d, want 1", code)
	}

	if !strings.Contains(g.Stderr.(*bytes.Buffer).String(), "unknown task") {
		t.Errorf("log = %q, want unknown task", g.Stderr.(*bytes.Buffer).String())
	}
}

func TestMainTaskError(t *testing.T) {
	g := New()
	g.Args = []string{"fail"}
	g.Stderr = &bytes.Buffer{}
	g.TaskErr("fail", func(ctx context.Context) error {
		return errors.New("boom")
	})

	if code := g.runMain(mainOptions{ctx: context.Background()}); code != 1 {
		t.Fatalf("runMain(fail) = %d, want 1", code)
	}

	out := g.Stderr.(*bytes.Buffer).String()
	if !strings.Contains(out, "Task failed") || !strings.Contains(out, "boom") {
		t.Errorf("log = %q, want task failure", out)
	}
}

func TestMainDefaultLoggerWritesToStderr(t *testing.T) {
	g := New()
	g.Args = []string{"log"}
	g.Stderr = &bytes.Buffer{}
	g.Level = slog.LevelDebug
	g.TaskErr("log", func(ctx context.Context) error {
		Logger(ctx).Info("hello")
		return nil
	})

	if code := g.runMain(mainOptions{ctx: context.Background()}); code != 0 {
		t.Fatalf("runMain() = %d, want 0", code)
	}

	if !strings.Contains(g.Stderr.(*bytes.Buffer).String(), "hello") {
		t.Errorf("log = %q, want task log", g.Stderr.(*bytes.Buffer).String())
	}
}

func TestMainJSONLoggerNestedTasks(t *testing.T) {
	g := New()
	g.Args = []string{"-log-format", "json", "outer"}
	g.Stderr = &bytes.Buffer{}
	g.Level = slog.LevelDebug

	inner := g.Task("inner", func(ctx context.Context) {
		Logger(ctx).Info("hello")
	})
	g.Task("outer", func(ctx context.Context) {
		inner(ctx)
	})

	if code := g.runMain(mainOptions{ctx: context.Background()}); code != 0 {
		t.Fatalf("runMain() = %d, want 0", code)
	}

	out := g.Stderr.(*bytes.Buffer).String()

	var helloLine string
	for _, line := range strings.Split(out, "\n") {
		if strings.Contains(line, `"msg":"hello"`) {
			helloLine = line
			break
		}
	}

	if helloLine == "" {
		t.Fatalf("log = %q, want a JSON record for hello", out)
	}

	want := regexp.MustCompile(`^\{"time":"[^"]+","level":"INFO","msg":"hello","task":"outer","task":"inner"\}$`)
	if !want.MatchString(helloLine) {
		t.Errorf("hello log line = %q, want %s", helloLine, want)
	}
}

func TestMainPreservesContextLogger(t *testing.T) {
	g := New()
	g.Args = []string{"-level", "error", "-log-format", "json", "log"}
	g.Stderr = &bytes.Buffer{}
	g.TaskErr("log", func(ctx context.Context) error {
		Logger(ctx).Info("custom")
		return nil
	})

	custom := &bytes.Buffer{}
	customLogger := slog.New(slog.NewTextHandler(custom, &slog.HandlerOptions{Level: slog.LevelDebug}))
	ctx := logger.NewLoggerContext(context.Background(), customLogger)

	if code := g.runMain(mainOptions{ctx: ctx}); code != 0 {
		t.Fatalf("runMain() = %d, want 0", code)
	}

	if !strings.Contains(custom.String(), "custom") {
		t.Errorf("custom log = %q, want task log", custom.String())
	}

	if g.Stderr.(*bytes.Buffer).Len() != 0 {
		t.Errorf("Stderr = %q, want empty when a context logger is supplied", g.Stderr.(*bytes.Buffer).String())
	}
}

func TestMainPreservesContextLoggerOnError(t *testing.T) {
	g := New()
	g.Args = []string{"fail"}
	g.Stderr = &bytes.Buffer{}
	g.TaskErr("fail", func(ctx context.Context) error {
		return errors.New("boom")
	})

	custom := &bytes.Buffer{}
	customLogger := slog.New(slog.NewTextHandler(custom, &slog.HandlerOptions{Level: slog.LevelDebug}))
	ctx := logger.NewLoggerContext(context.Background(), customLogger)

	if code := g.runMain(mainOptions{ctx: ctx}); code != 1 {
		t.Fatalf("runMain() = %d, want 1", code)
	}

	if !strings.Contains(custom.String(), "Task failed") || !strings.Contains(custom.String(), "boom") {
		t.Errorf("custom log = %q, want task failure", custom.String())
	}

	if g.Stderr.(*bytes.Buffer).Len() != 0 {
		t.Errorf("Stderr = %q, want empty when a context logger is supplied", g.Stderr.(*bytes.Buffer).String())
	}
}

func TestWithContextLastWins(t *testing.T) {
	type key struct{}

	g := New()
	g.Args = []string{"read"}
	g.Stdout = &bytes.Buffer{}
	g.Stderr = &bytes.Buffer{}

	var got any
	g.Task("read", func(ctx context.Context) {
		got = ctx.Value(key{})
	})

	first := context.WithValue(testContext(t), key{}, "first")
	second := context.WithValue(testContext(t), key{}, "second")

	cfg := mainOptions{ctx: context.Background()}
	WithContext(first)(&cfg)
	WithContext(second)(&cfg)

	if code := g.runMain(cfg); code != 0 {
		t.Fatalf("runMain() = %d, want 0", code)
	}

	if got != "second" {
		t.Errorf("context value = %v, want second (last WithContext wins)", got)
	}
}

func TestWithMiddlewareOrder(t *testing.T) {
	g := New()
	g.Args = []string{"task"}
	g.Stdout = &bytes.Buffer{}
	g.Stderr = &bytes.Buffer{}

	var order []string

	g.Use(func(ctx context.Context, next middleware.Next) error {
		order = append(order, "global before")
		err := next(ctx)
		order = append(order, "global after")
		return err
	})

	g.Task("task", func(ctx context.Context) {
		order = append(order, "task")
	})

	first := func(ctx context.Context, next middleware.Next) error {
		order = append(order, "first before")
		err := next(ctx)
		order = append(order, "first after")
		return err
	}
	second := func(ctx context.Context, next middleware.Next) error {
		order = append(order, "second before")
		err := next(ctx)
		order = append(order, "second after")
		return err
	}

	cfg := mainOptions{ctx: testContext(t)}
	WithMiddleware(first, second)(&cfg)

	if code := g.runMain(cfg); code != 0 {
		t.Fatalf("runMain() = %d, want 0", code)
	}

	want := []string{
		"first before",
		"second before",
		"global before",
		"task",
		"global after",
		"second after",
		"first after",
	}

	if len(order) != len(want) {
		t.Fatalf("order = %v, want %v", order, want)
	}

	for i := range want {
		if order[i] != want[i] {
			t.Fatalf("order = %v, want %v", order, want)
		}
	}
}
