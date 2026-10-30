package gabric

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"

	"github.com/bancek/gabric/logger"
	"github.com/bancek/gabric/middleware"
)

// MainOption configures a Main invocation.
type MainOption func(*mainOptions)

type mainOptions struct {
	ctx         context.Context
	middlewares []middleware.Middleware
}

// WithContext sets the base context for Main. It can carry a logger, which is
// preserved instead of the default logger built from Level and LogFormat.
// Repeated WithContext options use the last context.
func WithContext(ctx context.Context) MainOption {
	if ctx == nil {
		ctx = context.Background()
	}

	return func(cfg *mainOptions) {
		cfg.ctx = ctx
	}
}

// WithMiddleware appends middlewares applied around the whole Main invocation.
// They run inside the built-in middlewares and outside global middleware. The
// first middleware is outermost. Repeated options append in call order.
func WithMiddleware(middlewares ...middleware.Middleware) MainOption {
	return func(cfg *mainOptions) {
		cfg.middlewares = append(cfg.middlewares, middlewares...)
	}
}

// ParseFlags parses args (without the program name) with g's FlagSet. It syncs
// the flag output from Stderr so later output overrides take effect.
func (g *Gabric) ParseFlags(args []string) error {
	g.FlagSet.SetOutput(g.stderr())

	return g.FlagSet.Parse(args)
}

// ParseFlags is equivalent to Default.ParseFlags.
func ParseFlags(args []string) error {
	return Default.ParseFlags(args)
}

func (g *Gabric) newBootstrapLogger() *slog.Logger {
	return slog.New(logger.NewTextHandler(g.stderr(), &slog.HandlerOptions{
		Level: g.Level,
	}))
}

func (g *Gabric) newLogger() *slog.Logger {
	opts := &slog.HandlerOptions{Level: g.Level}

	var handler slog.Handler
	if g.LogFormat == LogFormatJSON {
		handler = slog.NewJSONHandler(g.stderr(), opts)
	} else {
		handler = logger.NewTextHandler(g.stderr(), opts)
	}

	return slog.New(handler)
}

// Main parses g.Args and flags, builds the runtime context, and runs the
// requested tasks. It exits non-zero on failure.
func (g *Gabric) Main(opts ...MainOption) {
	cfg := mainOptions{ctx: context.Background()}

	for _, opt := range opts {
		if opt != nil {
			opt(&cfg)
		}
	}

	if code := g.runMain(cfg); code != 0 {
		os.Exit(code)
	}
}

// Main is equivalent to Default.Main.
func Main(opts ...MainOption) {
	Default.Main(opts...)
}

func (g *Gabric) runMain(cfg mainOptions) int {
	ctx := cfg.ctx
	if ctx == nil {
		ctx = context.Background()
	}

	hadLogger := logger.HasLogger(ctx)
	if !hadLogger {
		ctx = logger.NewLoggerContext(ctx, g.newBootstrapLogger())
	}

	if err := g.ParseFlags(g.Args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}

		logger.LoggerFromContext(ctx).With("error", err).Error("Invalid flags")

		return 2
	}

	if !hadLogger {
		ctx = logger.NewLoggerContext(ctx, g.newLogger())
	}

	calls, err := g.ParseTasks(g.FlagSet.Args())
	if err != nil {
		logger.LoggerFromContext(ctx).With("error", err).Error("Invalid task")

		return 1
	}

	if len(calls) == 0 {
		g.listTasks(g.stdout())

		return 0
	}

	if err := g.Run(ctx, calls, cfg.middlewares...); err != nil {
		logger.LoggerFromContext(ctx).With("error", err).Error("Task failed")

		return 1
	}

	return 0
}

func (g *Gabric) listTasks(w io.Writer) {
	for _, name := range g.getTaskNames() {
		fmt.Fprintln(w, name)
	}
}
