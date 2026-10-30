package gabric

import (
	"context"
	"path"
)

const remoteWorkingDirKey contextKey[string] = "gabric.remoteWorkingDir"

// NewRemoteWorkingDirContext returns a context with the remote working
// directory set.
func NewRemoteWorkingDirContext(ctx context.Context, workingDir string) context.Context {
	return withContextValue(ctx, remoteWorkingDirKey, workingDir)
}

// GetRemoteWorkingDir returns the remote working directory, or an empty string
// when none is set.
func GetRemoteWorkingDir(ctx context.Context) string {
	workingDir, _ := contextValue(ctx, remoteWorkingDirKey)
	return workingDir
}

// HasRemoteWorkingDir reports whether ctx carries a remote working directory.
func HasRemoteWorkingDir(ctx context.Context) bool {
	_, ok := contextValue(ctx, remoteWorkingDirKey)
	return ok
}

// ResolveRemotePath returns p if it is absolute, otherwise p joined to the
// remote working directory from ctx.
func ResolveRemotePath(ctx context.Context, p string) string {
	if path.IsAbs(p) || !HasRemoteWorkingDir(ctx) {
		return p
	}

	return path.Join(GetRemoteWorkingDir(ctx), p)
}

func withoutRemoteWorkingDir(ctx context.Context) context.Context {
	return NewRemoteWorkingDirContext(ctx, "")
}
