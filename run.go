package gabric

import (
	"context"
	"slices"

	"github.com/bancek/gabric/dryrun"
	"github.com/bancek/gabric/middleware"
)

// BuiltinMiddlewares returns the middleware gabric adds to every Run: the
// dry-run marker, default remote target, and middleware registered with
// UseBuiltin. They only add context state that is not already present.
func (g *Gabric) BuiltinMiddlewares() []middleware.Middleware {
	g.mu.RLock()
	installed := slices.Clone(g.builtinMiddlewares)
	g.mu.RUnlock()

	middlewares := make([]middleware.Middleware, 0, 2+len(installed))

	if g.DryRun {
		middlewares = append(middlewares, withDryRun)
	}

	middlewares = append(middlewares, withDefaultRemoteTarget)
	middlewares = append(middlewares, installed...)

	return middlewares
}

func withDryRun(ctx context.Context, next middleware.Next) error {
	if dryrun.Enabled(ctx) {
		return next(ctx)
	}

	return next(dryrun.With(ctx))
}

// AllMiddlewares returns the complete middleware chain for a run: built-in
// middlewares, then main middlewares, then global middlewares registered with
// Use. The terminal (Dispatch) is not included.
func (g *Gabric) AllMiddlewares(main ...middleware.Middleware) []middleware.Middleware {
	return slices.Concat(
		g.BuiltinMiddlewares(),
		main,
		g.GlobalMiddlewares(),
	)
}

// Run executes calls with the context and middleware chain. Calls are already
// parsed; Run does not parse flags or task names.
func (g *Gabric) Run(ctx context.Context, calls []TaskCall, main ...middleware.Middleware) error {
	return middleware.Apply(g.AllMiddlewares(main...), Dispatch(calls))(ctx)
}

// Run is equivalent to Default.Run.
func Run(ctx context.Context, calls []TaskCall, main ...middleware.Middleware) error {
	return Default.Run(ctx, calls, main...)
}
