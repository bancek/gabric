package gabric

import "context"

const localExecEnvKey contextKey[ExecEnv] = "gabric.localExecEnv"

// GetLocalExecEnv returns the local exec environment from ctx.
func GetLocalExecEnv(ctx context.Context) ExecEnv {
	env, _ := contextValue(ctx, localExecEnvKey)
	return env
}

// HasLocalExecEnv reports whether ctx carries a local exec environment.
func HasLocalExecEnv(ctx context.Context) bool {
	_, ok := contextValue(ctx, localExecEnvKey)
	return ok
}

// NewLocalExecEnvContext returns a derived context with the local exec
// environment replaced by vars.
func NewLocalExecEnvContext(ctx context.Context, vars ...EnvVar) context.Context {
	return withContextValue(ctx, localExecEnvKey, NewExecEnv(vars...))
}

// AppendLocalExecEnvContext returns a derived context with vars appended to the
// current local exec environment.
func AppendLocalExecEnvContext(ctx context.Context, vars ...EnvVar) context.Context {
	return withContextValue(ctx, localExecEnvKey, GetLocalExecEnv(ctx).With(vars...))
}

// WithExecEnv merges env into the local exec environment for the duration of f.
func (o *LocalOps) WithExecEnv(env ExecEnv, f func(ctx context.Context)) {
	f(AppendLocalExecEnvContext(o.ctx, env.Vars()...))
}

// WithExecEnvErr is the error-first form of WithExecEnv.
func (o *LocalOps) WithExecEnvErr(env ExecEnv, f func(ctx context.Context) error) error {
	return f(AppendLocalExecEnvContext(o.ctx, env.Vars()...))
}
