package gabricssh

import "context"

type contextKey[T any] string

func contextValue[T any](ctx context.Context, key contextKey[T]) (T, bool) {
	value, ok := ctx.Value(key).(T)

	return value, ok
}

func withContextValue[T any](ctx context.Context, key contextKey[T], value T) context.Context {
	return context.WithValue(ctx, key, value)
}
