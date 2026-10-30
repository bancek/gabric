package gabric

import (
	"context"
	"os"
	"path/filepath"
)

const localWorkingDirKey contextKey[string] = "gabric.localWorkingDir"

// NewLocalWorkingDirContext returns a context with the local working directory
// set.
func NewLocalWorkingDirContext(ctx context.Context, workingDir string) context.Context {
	return withContextValue(ctx, localWorkingDirKey, workingDir)
}

// GetLocalWorkingDir returns the local working directory from ctx, falling back
// to the process working directory.
func GetLocalWorkingDir(ctx context.Context) string {
	workingDir, ok := contextValue(ctx, localWorkingDirKey)
	if !ok {
		workingDir, err := os.Getwd()
		if err != nil {
			panic(err)
		}
		return workingDir
	}
	return workingDir
}

// HasLocalWorkingDir reports whether ctx carries a local working directory.
func HasLocalWorkingDir(ctx context.Context) bool {
	_, ok := contextValue(ctx, localWorkingDirKey)
	return ok
}

// ResolveLocalPath returns path if it is absolute, otherwise path joined to the
// local working directory from ctx.
func ResolveLocalPath(ctx context.Context, path string) string {
	if filepath.IsAbs(path) {
		return path
	}

	return filepath.Join(GetLocalWorkingDir(ctx), path)
}
