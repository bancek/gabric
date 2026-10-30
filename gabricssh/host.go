package gabricssh

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"sync/atomic"

	"github.com/bancek/gabric/logger"
	"github.com/bancek/gabric/middleware"
	"github.com/bancek/gabric/sshremote"
)

const hostKey contextKey[*hostContext] = "gabricssh.host"

// ErrNoHost is returned by GetHostErr when ctx carries no host.
var ErrNoHost = errors.New("gabricssh: host not in context; use NewHostContext, NewHostNameContext, or run with -ssh-host")

type hostContext struct {
	spec string

	once     sync.Once
	resolved atomic.Pointer[sshremote.Host]
	err      error
}

// NewHostContext returns a context scoped to an already-resolved host. The host
// is added to the logger fields.
func NewHostContext(ctx context.Context, host *sshremote.Host) context.Context {
	hc := &hostContext{}
	hc.resolved.Store(host)

	ctx = withContextValue(ctx, hostKey, hc)
	ctx = logger.NewLoggerContext(ctx, logger.LoggerFromContext(ctx).With("host", hc))

	return ctx
}

// NewHostNameContext returns a context scoped to a host spec to be resolved
// lazily with the context's config. The spec is added to the logger fields; once
// resolved, the full host is logged.
func NewHostNameContext(ctx context.Context, spec string) context.Context {
	hc := &hostContext{spec: spec}

	ctx = withContextValue(ctx, hostKey, hc)
	ctx = logger.NewLoggerContext(ctx, logger.LoggerFromContext(ctx).With("host", hc))

	return ctx
}

// WithHost sets the SSH host spec. It does not install a runner.
func WithHost(host string) middleware.Middleware {
	return func(ctx context.Context, next middleware.Next) error {
		return next(NewHostNameContext(ctx, host))
	}
}

// GetHost returns the resolved host from ctx. It panics if no host is in
// context or resolution fails.
func GetHost(ctx context.Context) *sshremote.Host {
	host, err := GetHostErr(ctx)
	if err != nil {
		panic(err)
	}
	return host
}

// GetHostErr returns the resolved host from ctx.
func GetHostErr(ctx context.Context) (*sshremote.Host, error) {
	hc, ok := contextValue(ctx, hostKey)
	if !ok {
		return nil, ErrNoHost
	}
	return hc.resolve(ctx)
}

// HasHost reports whether ctx carries a host or a host name.
func HasHost(ctx context.Context) bool {
	_, ok := contextValue(ctx, hostKey)
	return ok
}

// resolve returns the host, resolving it once with the context's config.
func (hc *hostContext) resolve(ctx context.Context) (*sshremote.Host, error) {
	if host := hc.resolved.Load(); host != nil {
		return host, nil
	}

	hc.once.Do(func() {
		host, err := GetConfig(ctx).ResolveHost(hc.spec)
		if err != nil {
			hc.err = err
			return
		}

		hc.resolved.Store(host)
	})

	if hc.err != nil {
		return nil, hc.err
	}

	return hc.resolved.Load(), nil
}

// LogValue implements slog.LogValuer: the resolved host once known, otherwise
// the requested spec. It is evaluated when the log record is handled, so a
// lazily-resolved host shows its full form on later logs.
func (hc *hostContext) LogValue() slog.Value {
	if host := hc.resolved.Load(); host != nil {
		return slog.StringValue(host.String())
	}
	return slog.StringValue(hc.spec)
}
