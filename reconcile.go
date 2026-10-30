package gabric

import (
	"context"

	"github.com/bancek/gabric/middleware"
	"github.com/bancek/gabric/reconcile"
)

// WithReconcile is middleware that runs next inside a single
// reconcile.Run, so every task dispatched in the invocation shares one run and
// its node state. Register it around the whole run with Use:
//
//	var _ = gabric.Use(gabric.WithReconcile)
//
// A task can also carry it to get its own run, for example when reconciling
// another host:
//
//	var appTask = gabric.With(gabric.WithReconcile).NodeTask(appNode)
//
// Nested runs do not share state. Effects are flushed once when the run
// finishes. A task that needs effects sooner calls reconcile.Flush.
func WithReconcile(ctx context.Context, next middleware.Next) error {
	return reconcile.Run(ctx, next)
}
