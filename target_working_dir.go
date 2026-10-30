package gabric

import "context"

const (
	targetOpsWorkingDirKey        contextKey[string] = "gabric.targetOpsWorkingDir"
	targetOpsWorkingDirAppliedKey contextKey[bool]   = "gabric.targetOpsWorkingDirApplied"
)

// NewTargetOpsWorkingDirContext returns a derived context with the target ops
// working directory replaced by dir. If the target ops overlays are already
// applied, dir is applied to the target immediately.
func NewTargetOpsWorkingDirContext(ctx context.Context, dir string) context.Context {
	ctx = withContextValue(ctx, targetOpsWorkingDirKey, dir)

	if targetOpsWorkingDirApplied(ctx) {
		if target, ok := targetFromContext(ctx); ok {
			return target.workingDirContext(ctx, dir)
		}
	}

	return ctx
}

// GetTargetOpsWorkingDir returns the target ops working directory from ctx.
func GetTargetOpsWorkingDir(ctx context.Context) (string, bool) {
	dir, ok := contextValue(ctx, targetOpsWorkingDirKey)
	return dir, ok
}

// HasTargetOpsWorkingDir reports whether ctx carries a target ops working
// directory.
func HasTargetOpsWorkingDir(ctx context.Context) bool {
	_, ok := contextValue(ctx, targetOpsWorkingDirKey)
	return ok
}

func targetOpsWorkingDirApplied(ctx context.Context) bool {
	applied, _ := contextValue(ctx, targetOpsWorkingDirAppliedKey)
	return applied
}

func withTargetOpsWorkingDirApplied(ctx context.Context) context.Context {
	return withContextValue(ctx, targetOpsWorkingDirAppliedKey, true)
}

func withoutTargetOpsWorkingDirApplied(ctx context.Context) context.Context {
	return withContextValue(ctx, targetOpsWorkingDirAppliedKey, false)
}
