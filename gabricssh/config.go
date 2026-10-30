package gabricssh

import (
	"context"

	"github.com/bancek/gabric/sshremote"
)

const configKey contextKey[*sshremote.Config] = "gabricssh.config"

// NewConfigContext returns a context carrying the SSH config.
func NewConfigContext(ctx context.Context, config *sshremote.Config) context.Context {
	return withContextValue(ctx, configKey, config)
}

// GetConfig returns the SSH config from ctx. It panics if no config is in
// context.
func GetConfig(ctx context.Context) *sshremote.Config {
	config, ok := contextValue(ctx, configKey)
	if !ok || config == nil {
		panic("gabricssh: config not in context; use NewConfigContext or WithSSH")
	}

	return config
}

// HasConfig reports whether ctx carries an SSH config.
func HasConfig(ctx context.Context) bool {
	config, ok := contextValue(ctx, configKey)

	return ok && config != nil
}

// ResolveHost parses hostStr and merges matching ssh_config settings using the
// config from ctx.
func ResolveHost(ctx context.Context, hostStr string) (*sshremote.Host, error) {
	return GetConfig(ctx).ResolveHost(hostStr)
}
