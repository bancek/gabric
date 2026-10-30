package reconcile

import (
	"context"
	"slices"

	"github.com/bancek/gabric/middleware"
)

// Effect is a reaction subscribed to one or more topics. It runs at Flush if
// any of its topics is dirty, and it does not run in dry-run mode.
type Effect struct {
	name        string
	topics      []*Topic
	fn          func(ctx context.Context) error
	middlewares []middleware.Middleware
}

// NewEffect creates an effect subscribed to the given topics.
func NewEffect(name string, topics []*Topic, fn func(ctx context.Context) error) *Effect {
	e := &Effect{name: name, topics: slices.Clone(topics), fn: fn}

	for _, t := range e.topics {
		t.subscribe(e)
	}

	return e
}

// Name returns the effect name.
func (e *Effect) Name() string {
	return e.name
}

// With returns a new effect with middlewares appended. The middleware wraps the
// effect at flush and may call next zero or one time: zero skips the effect.
//
// The returned effect replaces e in every topic subscription; the original is no
// longer subscribed. Chain further With calls on the returned effect. Call With
// while building the effect, before any run flushes.
func (e *Effect) With(middlewares ...middleware.Middleware) *Effect {
	if len(middlewares) == 0 {
		return e
	}

	next := &Effect{
		name:        e.name,
		topics:      slices.Clone(e.topics),
		fn:          e.fn,
		middlewares: append(slices.Clone(e.middlewares), middlewares...),
	}

	for _, t := range e.topics {
		t.replaceEffect(e, next)
	}

	return next
}

func (e *Effect) run(ctx context.Context) error {
	if len(e.middlewares) == 0 {
		return e.fn(ctx)
	}

	calls := 0

	next := func(ctx context.Context) error {
		calls++
		if calls > 1 {
			return errMiddlewareNextTwice
		}

		return e.fn(ctx)
	}

	err := middleware.Apply(e.middlewares, next)(ctx)
	if calls > 1 {
		return errMiddlewareNextTwice
	}

	return err
}
