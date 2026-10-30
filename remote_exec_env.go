package gabric

import "context"

const remoteExecEnvKey contextKey[ExecEnv] = "gabric.remoteExecEnv"

// GetRemoteExecEnv returns the remote exec environment from ctx.
func GetRemoteExecEnv(ctx context.Context) ExecEnv {
	env, _ := contextValue(ctx, remoteExecEnvKey)
	return env
}

// HasRemoteExecEnv reports whether ctx carries a remote exec environment.
func HasRemoteExecEnv(ctx context.Context) bool {
	_, ok := contextValue(ctx, remoteExecEnvKey)
	return ok
}

// NewRemoteExecEnvContext returns a derived context with the remote exec
// environment replaced by vars.
func NewRemoteExecEnvContext(ctx context.Context, vars ...EnvVar) context.Context {
	return withContextValue(ctx, remoteExecEnvKey, NewExecEnv(vars...))
}

// AppendRemoteExecEnvContext returns a derived context with vars appended to
// the current remote exec environment.
func AppendRemoteExecEnvContext(ctx context.Context, vars ...EnvVar) context.Context {
	return withContextValue(ctx, remoteExecEnvKey, GetRemoteExecEnv(ctx).With(vars...))
}

// WithExecEnv merges env into the remote exec environment for the duration of f.
func (o *RemoteOps) WithExecEnv(env ExecEnv, f func(ctx context.Context)) {
	f(AppendRemoteExecEnvContext(o.ctx, env.Vars()...))
}

// WithExecEnvErr is the error-first form of WithExecEnv.
func (o *RemoteOps) WithExecEnvErr(env ExecEnv, f func(ctx context.Context) error) error {
	return f(AppendRemoteExecEnvContext(o.ctx, env.Vars()...))
}
