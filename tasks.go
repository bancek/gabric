package gabric

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/bancek/gabric/logger"
	"github.com/bancek/gabric/middleware"
)

// TaskFunc is an error-first task body: it receives all execution state through
// ctx and returns an error on failure. It is an alias of middleware.Next, so
// ordinary error-first functions work wherever a TaskFunc is expected. Use Task
// for panic-style tasks or TaskErr to register an error-first body.
type TaskFunc = middleware.Next

// TaskCall is one task invocation from the command line: the registered task
// and its colon-separated argument. The argument belongs to the call, so the
// same task can be invoked more than once with different arguments.
type TaskCall struct {
	task TaskFunc
	Arg  string
}

// Call runs the task with this call's argument installed in the context. The
// argument always overrides an outer current argument, including when it is
// empty.
func (c TaskCall) Call(ctx context.Context) error {
	return c.task(NewCurrentTaskArgContext(ctx, c.Arg))
}

// register wraps task with task argument and logger handling, recovers panics
// into errors and stores it under name.
func (g *Gabric) register(name string, task TaskFunc) TaskFunc {
	wrapped := func(ctx context.Context) (err error) {
		arg := GetCurrentTaskArg(ctx)

		log := Logger(ctx).With("task", name)

		ctx = logger.NewLoggerContext(ctx, log)

		start := time.Now()

		log.With("arg", arg).Debug("Task started")

		defer func() {
			if r := recover(); r != nil {
				err = fmt.Errorf("task %s panicked: %w", name, PanicError(r))
			}

			if err != nil {
				log.With("duration", time.Since(start)).With("error", err).Debug("Task done")
				return
			}

			log.With("duration", time.Since(start)).Debug("Task done")
		}()

		return task(ctx)
	}

	g.mu.Lock()
	defer g.mu.Unlock()

	if _, ok := g.tasks[name]; ok {
		panic("task already registered: " + name)
	}

	g.tasks[name] = wrapped

	return wrapped
}

// ParseTasks resolves command-line task names (name or name:arg) against the
// registry. It does not run anything. Duplicate task names are preserved as
// separate calls.
func (g *Gabric) ParseTasks(args []string) ([]TaskCall, error) {
	calls := make([]TaskCall, 0, len(args))

	for _, arg := range args {
		name, taskArg, _ := strings.Cut(arg, ":")

		task, ok := g.getTask(name)
		if !ok {
			return nil, fmt.Errorf("unknown task: %s", name)
		}

		calls = append(calls, TaskCall{task: task, Arg: taskArg})
	}

	return calls, nil
}

// ParseTasks is equivalent to Default.ParseTasks.
func ParseTasks(args []string) ([]TaskCall, error) {
	return Default.ParseTasks(args)
}

// Dispatch returns the terminal middleware.Next that runs calls in order,
// stopping at the first error. Each call gets its own current-task argument.
func Dispatch(calls []TaskCall) middleware.Next {
	return func(ctx context.Context) error {
		for _, call := range calls {
			if err := call.Call(ctx); err != nil {
				return err
			}
		}

		return nil
	}
}

// Task registers a panic-style task and returns a callable that panics on
// failure, so tasks compose like ordinary Go calls. Panics from the task body
// are recovered at the task boundary and reported as task errors.
func (g *Gabric) Task(name string, task func(ctx context.Context)) func(ctx context.Context) {
	return g.With().Task(name, task)
}

// TaskErr registers an error-first task.
func (g *Gabric) TaskErr(name string, task TaskFunc) TaskFunc {
	return g.With().TaskErr(name, task)
}

// Task is equivalent to Default.Task.
func Task(name string, task func(ctx context.Context)) func(ctx context.Context) {
	return Default.Task(name, task)
}

// TaskErr is equivalent to Default.TaskErr.
func TaskErr(name string, task TaskFunc) TaskFunc {
	return Default.TaskErr(name, task)
}

// TaskErrToTask adapts an error-first task into a callable that panics on
// failure. It is the inverse of TaskToTaskErr.
func TaskErrToTask(task TaskFunc) func(ctx context.Context) {
	return func(ctx context.Context) {
		if err := task(ctx); err != nil {
			panic(err)
		}
	}
}

// TaskToTaskErr adapts a panic-style task callable (from Task or
// TaskBuilder.Task) into a TaskFunc, recovering panics into errors, so it can be
// used where an error-first task is expected (for example ParallelErr). It is
// the inverse of TaskErrToTask.
func TaskToTaskErr(task func(ctx context.Context)) TaskFunc {
	return func(ctx context.Context) (err error) {
		defer func() {
			if r := recover(); r != nil {
				err = PanicError(r)
			}
		}()

		task(ctx)

		return nil
	}
}
