package gabric

import (
	"context"

	"github.com/bancek/gabric/middleware"
	"github.com/bancek/gabric/reconcile"
)

// TaskBuilder registers tasks wrapped with a set of middlewares. It is created
// with With and is immutable: With returns a new builder, so a base can be
// shared and safely extended.
type TaskBuilder struct {
	g   *Gabric
	mws []middleware.Middleware
}

// With returns a TaskBuilder that registers tasks on g wrapped with mws.
func (g *Gabric) With(mws ...middleware.Middleware) *TaskBuilder {
	return &TaskBuilder{g: g, mws: append([]middleware.Middleware{}, mws...)}
}

// With is equivalent to Default.With.
func With(mws ...middleware.Middleware) *TaskBuilder {
	return Default.With(mws...)
}

// NodeTask is equivalent to Default.With().NodeTask.
func NodeTask[T any](n *reconcile.Node[T]) func(ctx context.Context) {
	return Default.With().NodeTask(n)
}

// NodeTaskErr is equivalent to Default.With().NodeTaskErr.
func NodeTaskErr[T any](n *reconcile.Node[T]) TaskFunc {
	return Default.With().NodeTaskErr(n)
}

// With returns a new builder with mws appended to the receiver's middlewares.
// The receiver is unchanged.
func (b *TaskBuilder) With(mws ...middleware.Middleware) *TaskBuilder {
	combined := make([]middleware.Middleware, 0, len(b.mws)+len(mws))
	combined = append(combined, b.mws...)
	combined = append(combined, mws...)

	return &TaskBuilder{g: b.g, mws: combined}
}

// Task registers a panic-style task with the builder's middlewares.
func (b *TaskBuilder) Task(name string, task func(ctx context.Context)) func(ctx context.Context) {
	return TaskErrToTask(b.g.register(name, middleware.Apply(b.mws, func(ctx context.Context) error {
		task(ctx)
		return nil
	})))
}

// TaskErr registers an error-first task with the builder's middlewares.
func (b *TaskBuilder) TaskErr(name string, task TaskFunc) TaskFunc {
	return b.g.register(name, middleware.Apply(b.mws, task))
}

// NodeTask registers a panic-style task that evaluates reconcile node n and then
// flushes its effects. The task name is the node's name. It requires an active
// reconcile run (for example from a global Use(WithReconcile) middleware, or
// from a builder with WithReconcile).
func (b *TaskBuilder) NodeTask[T any](n *reconcile.Node[T]) func(ctx context.Context) {
	return b.Task(n.Name(), func(ctx context.Context) {
		_, err := n.Value(ctx)
		Must(err)

		Must(reconcile.Flush(ctx))
	})
}

// NodeTaskErr is the error-first form of NodeTask: it returns the node's error
// instead of panicking.
func (b *TaskBuilder) NodeTaskErr[T any](n *reconcile.Node[T]) TaskFunc {
	return b.TaskErr(n.Name(), func(ctx context.Context) error {
		if _, err := n.Value(ctx); err != nil {
			return err
		}

		return reconcile.Flush(ctx)
	})
}
