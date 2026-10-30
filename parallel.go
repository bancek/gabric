package gabric

import (
	"context"
	"errors"
	"sync"
)

// Parallel runs task-like functions concurrently and waits for all of them to
// finish. Each task is a func(ctx context.Context); it may be a callable from
// Task or TaskBuilder.Task, or any ordinary function with that shape. Tasks are
// started in order and receive the same context. If a task panics, the other
// tasks still run; Parallel then panics with their joined errors.
//
// Tasks share one context, and a reconcile run carried by it (for example with
// gabric.Use(gabric.WithReconcile)) is evaluated concurrently and safely: a node
// is still evaluated at most once per run. Node state is keyed by node identity,
// not host: a run can span hosts when each host uses distinct nodes. Do not
// evaluate the same node or node graph with different host contexts in one run;
// if one node task is reused across hosts, give each call a fresh run.
func Parallel(ctx context.Context, tasks ...func(ctx context.Context)) {
	fns := make([]func() error, len(tasks))

	for i, task := range tasks {
		fns[i] = func() (err error) {
			defer func() {
				if r := recover(); r != nil {
					err = PanicError(r)
				}
			}()

			task(ctx)

			return nil
		}
	}

	if err := errors.Join(runParallel(fns)...); err != nil {
		panic(err)
	}
}

// ParallelErr is the error-first form of Parallel: it runs error-first
// functions concurrently, waits for all of them to finish and returns their
// joined errors. TaskFunc is an alias of func(ctx) error, so []TaskFunc values
// can be expanded here too.
func ParallelErr(ctx context.Context, tasks ...func(ctx context.Context) error) error {
	fns := make([]func() error, len(tasks))

	for i, task := range tasks {
		fns[i] = func() error {
			return task(ctx)
		}
	}

	return errors.Join(runParallel(fns)...)
}

// runParallel runs fns concurrently and returns their errors, in order.
func runParallel(fns []func() error) []error {
	errs := make([]error, len(fns))

	var wg sync.WaitGroup

	for i, fn := range fns {
		wg.Go(func() {
			errs[i] = fn()
		})
	}

	wg.Wait()

	return errs
}
