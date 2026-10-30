package gabric

import (
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"sync"

	"github.com/bancek/gabric/middleware"
)

// LogFormat is the format used for the default logger.
type LogFormat int

const (
	LogFormatText LogFormat = iota
	LogFormatJSON
)

// String returns the flag representation of the log format.
func (f LogFormat) String() string {
	switch f {
	case LogFormatJSON:
		return "json"
	default:
		return "text"
	}
}

// MarshalText implements encoding.TextMarshaler for flag.TextVar.
func (f LogFormat) MarshalText() ([]byte, error) {
	return []byte(f.String()), nil
}

// UnmarshalText implements encoding.TextUnmarshaler for flag.TextVar.
func (f *LogFormat) UnmarshalText(text []byte) error {
	switch strings.ToLower(string(text)) {
	case "", "text":
		*f = LogFormatText
	case "json":
		*f = LogFormatJSON
	default:
		return fmt.Errorf("invalid log format %q", text)
	}

	return nil
}

// Gabric is a task runner: it owns the task registry, global middlewares, the
// command-line flag set and the pre-run configuration. Execution state is never
// stored on a Gabric instance; hosts, loggers, remote connections, sudo,
// working directories and user values all live in context.Context, so tasks can
// be composed, scoped and run concurrently without shared mutable state.
//
// The package level Task, TaskErr, With, Use, ParseFlags, ParseTasks, Run and
// Main functions operate on Default. Create a separate instance with New for
// isolation.
type Gabric struct {
	// FlagSet is the command-line flag set Main parses. It is initialized by New
	// with gabric's runner flags; application flags can be added to it before
	// Main is called. Replacing it drops the runner flags.
	FlagSet *flag.FlagSet

	// Args is the command-line input Main parses. It excludes the program name.
	// New copies os.Args[1:]; set it to run Main with explicit arguments.
	Args []string

	// Stdout receives task listings. It defaults to os.Stdout.
	Stdout io.Writer
	// Stderr receives logs, flag usage and flag parse errors. It defaults to
	// os.Stderr.
	Stderr io.Writer

	// Level is the default logger level (-level).
	Level slog.Level
	// LogFormat selects the default logger handler (-log-format).
	LogFormat LogFormat
	// DryRun marks the runtime context as a dry run (-dry-run). It is advisory:
	// tasks and reconcile nodes check it before mutating.
	DryRun bool

	mu                 sync.RWMutex
	tasks              map[string]TaskFunc
	builtinMiddlewares []middleware.Middleware
	globalMiddlewares  []middleware.Middleware
}

// Default is the Gabric instance used by the package level functions.
var Default = New()

// New returns a Gabric with gabric's core flags registered on its FlagSet.
func New() *Gabric {
	g := &Gabric{
		Args:      slices.Clone(os.Args[1:]),
		Stdout:    os.Stdout,
		Stderr:    os.Stderr,
		Level:     slog.LevelDebug,
		LogFormat: LogFormatText,
		tasks:     map[string]TaskFunc{},
	}

	g.FlagSet = flag.NewFlagSet(filepath.Base(os.Args[0]), flag.ContinueOnError)
	g.FlagSet.SetOutput(g.Stderr)
	g.FlagSet.TextVar(&g.Level, "level", g.Level, "log level")
	g.FlagSet.TextVar(&g.LogFormat, "log-format", g.LogFormat, "log format (text or json)")
	g.FlagSet.BoolVar(&g.DryRun, "dry-run", false, "probe only; do not mutate")

	return g
}

func (g *Gabric) getTask(name string) (TaskFunc, bool) {
	g.mu.RLock()
	defer g.mu.RUnlock()

	task, ok := g.tasks[name]

	return task, ok
}

func (g *Gabric) getTaskNames() []string {
	g.mu.RLock()
	defer g.mu.RUnlock()

	names := make([]string, 0, len(g.tasks))

	for name := range g.tasks {
		names = append(names, name)
	}

	sort.Strings(names)

	return names
}

// Use adds global middlewares applied around every task run on g. They sit
// inside Main middleware and outside task middleware. The first middleware is
// outermost. Use returns g for chaining.
func (g *Gabric) Use(middlewares ...middleware.Middleware) *Gabric {
	g.mu.Lock()
	defer g.mu.Unlock()

	g.globalMiddlewares = append(g.globalMiddlewares, middlewares...)

	return g
}

// UseBuiltin adds middlewares applied after gabric's core built-ins and before
// main middleware. Install uses it to install transport setup. It must be
// called before the first Run. The first middleware is outermost. UseBuiltin
// returns g for chaining.
func (g *Gabric) UseBuiltin(middlewares ...middleware.Middleware) *Gabric {
	g.mu.Lock()
	defer g.mu.Unlock()

	g.builtinMiddlewares = append(g.builtinMiddlewares, middlewares...)

	return g
}

// Use is equivalent to Default.Use.
func Use(middlewares ...middleware.Middleware) *Gabric {
	return Default.Use(middlewares...)
}

// GlobalMiddlewares returns a copy of the middlewares registered with Use.
func (g *Gabric) GlobalMiddlewares() []middleware.Middleware {
	g.mu.RLock()
	defer g.mu.RUnlock()

	return slices.Clone(g.globalMiddlewares)
}

func (g *Gabric) stdout() io.Writer {
	if g.Stdout != nil {
		return g.Stdout
	}

	return os.Stdout
}

func (g *Gabric) stderr() io.Writer {
	if g.Stderr != nil {
		return g.Stderr
	}

	return os.Stderr
}
