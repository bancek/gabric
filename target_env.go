package gabric

import "context"

const (
	targetOpsExecEnvKey        contextKey[ExecEnv] = "gabric.targetOpsExecEnv"
	targetOpsExecEnvAppliedKey contextKey[bool]    = "gabric.targetOpsExecEnvApplied"
)

// NewTargetOpsExecEnvContext returns a derived context with the target ops exec
// environment replaced by vars. If the target ops overlays are already applied,
// vars are applied to the target immediately.
func NewTargetOpsExecEnvContext(ctx context.Context, vars ...EnvVar) context.Context {
	ctx = withContextValue(ctx, targetOpsExecEnvKey, NewExecEnv(vars...))

	if targetOpsExecEnvApplied(ctx) {
		if target, ok := targetFromContext(ctx); ok {
			return target.execEnvContext(ctx, vars...)
		}
	}

	return ctx
}

// AppendTargetOpsExecEnvContext returns a derived context with vars appended to
// the current target ops exec environment. If the target ops overlays are
// already applied, vars are applied to the target immediately.
func AppendTargetOpsExecEnvContext(ctx context.Context, vars ...EnvVar) context.Context {
	ctx = withContextValue(ctx, targetOpsExecEnvKey, GetTargetOpsExecEnv(ctx).With(vars...))

	if targetOpsExecEnvApplied(ctx) {
		if target, ok := targetFromContext(ctx); ok {
			return target.execEnvContext(ctx, vars...)
		}
	}

	return ctx
}

// GetTargetOpsExecEnv returns the target ops exec environment from ctx.
func GetTargetOpsExecEnv(ctx context.Context) ExecEnv {
	env, _ := contextValue(ctx, targetOpsExecEnvKey)
	return env
}

// HasTargetOpsExecEnv reports whether ctx carries a target ops exec environment.
func HasTargetOpsExecEnv(ctx context.Context) bool {
	_, ok := contextValue(ctx, targetOpsExecEnvKey)
	return ok
}

func targetOpsExecEnvApplied(ctx context.Context) bool {
	applied, _ := contextValue(ctx, targetOpsExecEnvAppliedKey)
	return applied
}

func withTargetOpsExecEnvApplied(ctx context.Context) context.Context {
	return withContextValue(ctx, targetOpsExecEnvAppliedKey, true)
}

func withoutTargetOpsExecEnvApplied(ctx context.Context) context.Context {
	return withContextValue(ctx, targetOpsExecEnvAppliedKey, false)
}
