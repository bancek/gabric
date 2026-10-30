package gabricssh

import (
	"context"

	"github.com/bancek/gabric/middleware"
	"github.com/bancek/gabric/sshremote"
)

const managerKey contextKey[*sshremote.RemoteManager] = "gabricssh.manager"

// NewManagerContext returns a context carrying manager.
func NewManagerContext(ctx context.Context, manager *sshremote.RemoteManager) context.Context {
	return withContextValue(ctx, managerKey, manager)
}

// GetManager returns the manager from ctx. It panics if the manager is not in
// context.
func GetManager(ctx context.Context) *sshremote.RemoteManager {
	manager, ok := contextValue(ctx, managerKey)
	if !ok || manager == nil {
		panic("gabricssh: manager not in context; use NewManagerContext or WithSSH")
	}
	return manager
}

// HasManager reports whether ctx carries a manager.
func HasManager(ctx context.Context) bool {
	manager, ok := contextValue(ctx, managerKey)

	return ok && manager != nil
}

// WithManager is middleware that creates a manager, adds it to the context for
// the duration of next, and closes it afterwards. It leaves an existing manager
// in place.
func WithManager(ctx context.Context, next middleware.Next) error {
	if HasManager(ctx) {
		return next(ctx)
	}

	manager := sshremote.NewRemoteManager()
	defer manager.Close()

	return next(NewManagerContext(ctx, manager))
}

// GetHostConfig returns the resolved host and config from ctx.
func GetHostConfig(ctx context.Context) sshremote.HostConfig {
	return sshremote.NewHostConfig(GetHost(ctx), GetConfig(ctx))
}

// GetRemote returns the remote for the host in ctx, creating it through the
// manager if needed.
func GetRemote(ctx context.Context) *sshremote.Remote {
	return GetManager(ctx).GetRemote(GetHostConfig(ctx))
}
