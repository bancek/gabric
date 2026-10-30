// Package dryrun carries a dry-run marker in context. It has no dependencies
// beyond the standard library, so gabric, reconcile and application code can
// share the same marker.
package dryrun

import "context"

type contextKey[T any] string

const dryrunKey contextKey[bool] = "dryrun.enabled"

func contextValue[T any](ctx context.Context, key contextKey[T]) (T, bool) {
	value, ok := ctx.Value(key).(T)
	return value, ok
}

func withContextValue[T any](ctx context.Context, key contextKey[T], value T) context.Context {
	return context.WithValue(ctx, key, value)
}

// With returns a context marked as a dry run.
func With(ctx context.Context) context.Context {
	return withContextValue(ctx, dryrunKey, true)
}

// Enabled reports whether ctx is marked as a dry run.
func Enabled(ctx context.Context) bool {
	enabled, _ := contextValue(ctx, dryrunKey)
	return enabled
}
