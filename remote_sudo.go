package gabric

import (
	"context"
	"slices"

	"github.com/bancek/gabric/middleware"
)

const remoteSudoKey contextKey[*Sudo] = "gabric.remoteSudo"

// NewRemoteSudoContext returns a context carrying the sudo spec. A nil spec
// disables remote sudo. The spec is copied.
func NewRemoteSudoContext(ctx context.Context, sudo *Sudo) context.Context {
	return withContextValue(ctx, remoteSudoKey, cloneSudo(sudo))
}

// GetRemoteSudo returns a copy of the sudo spec from ctx, or nil when remote
// sudo is disabled.
func GetRemoteSudo(ctx context.Context) *Sudo {
	sudo, _ := contextValue(ctx, remoteSudoKey)

	return cloneSudo(sudo)
}

// HasRemoteSudo reports whether ctx carries a remote sudo spec.
func HasRemoteSudo(ctx context.Context) bool {
	_, ok := contextValue(ctx, remoteSudoKey)
	return ok
}

func cloneSudo(sudo *Sudo) *Sudo {
	if sudo == nil {
		return nil
	}

	clone := *sudo
	clone.Flags = slices.Clone(sudo.Flags)

	return &clone
}

// WithRemoteSudo returns middleware that runs next with the given sudo spec. A
// nil spec disables remote sudo.
func WithRemoteSudo(sudo *Sudo) middleware.Middleware {
	sudo = cloneSudo(sudo)

	return func(ctx context.Context, next middleware.Next) error {
		return next(NewRemoteSudoContext(ctx, sudo))
	}
}
