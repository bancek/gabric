package gabric

import (
	"context"
)

const currentTaskArgKey contextKey[string] = "gabric.currentTaskArg"

// NewCurrentTaskArgContext returns a context carrying the argument of the
// current task invocation.
func NewCurrentTaskArgContext(ctx context.Context, arg string) context.Context {
	return withContextValue(ctx, currentTaskArgKey, arg)
}

// GetCurrentTaskArg returns the current task argument from ctx, or an empty
// string when it is not in context.
func GetCurrentTaskArg(ctx context.Context) string {
	arg, _ := contextValue(ctx, currentTaskArgKey)
	return arg
}

// HasCurrentTaskArg reports whether ctx carries a current task argument.
func HasCurrentTaskArg(ctx context.Context) bool {
	_, ok := contextValue(ctx, currentTaskArgKey)
	return ok
}
