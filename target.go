package gabric

import (
	"context"
	"errors"

	"github.com/bancek/gabric/middleware"
)

// OpsProvider returns the operations for a target.
type OpsProvider func(context.Context) Ops

// ExecEnvContextFunc appends environment entries to a context for a target.
type ExecEnvContextFunc func(context.Context, ...EnvVar) context.Context

// WorkingDirContextFunc sets the working directory in a context for a target.
type WorkingDirContextFunc func(context.Context, string) context.Context

// Target pairs a target's operations with its exec environment and working
// directory context constructors.
type Target struct {
	ops               OpsProvider
	execEnvContext    ExecEnvContextFunc
	workingDirContext WorkingDirContextFunc
}

// NewTarget returns a target from an ops provider, an exec environment context
// constructor and a working directory context constructor. All are required.
func NewTarget[O Ops](
	ops func(context.Context) O,
	execEnvContext ExecEnvContextFunc,
	workingDirContext WorkingDirContextFunc,
) Target {
	if ops == nil {
		panic("gabric: nil target ops provider")
	}
	if execEnvContext == nil {
		panic("gabric: nil target exec env context provider")
	}
	if workingDirContext == nil {
		panic("gabric: nil target working dir context provider")
	}

	return Target{
		ops:               func(ctx context.Context) Ops { return ops(ctx) },
		execEnvContext:    execEnvContext,
		workingDirContext: workingDirContext,
	}
}

var (
	// LocalTarget runs target operations locally.
	LocalTarget = NewTarget(Local, AppendLocalExecEnvContext, NewLocalWorkingDirContext)

	// RemoteTarget runs target operations remotely.
	RemoteTarget = NewTarget(Remote, AppendRemoteExecEnvContext, NewRemoteWorkingDirContext)

	// LocalRemoteTarget runs remote-style target operations locally.
	LocalRemoteTarget = NewTarget(LocalRemote, AppendRemoteExecEnvContext, NewRemoteWorkingDirContext)
)

const targetKey contextKey[Target] = "gabric.target"

func targetFromContext(ctx context.Context) (Target, bool) {
	return contextValue(ctx, targetKey)
}

// NewTargetContext returns a context whose TargetOps uses target.
func NewTargetContext(ctx context.Context, target Target) context.Context {
	if target.ops == nil || target.execEnvContext == nil || target.workingDirContext == nil {
		panic("gabric: nil target")
	}

	ctx = withContextValue(ctx, targetKey, target)
	ctx = withoutTargetOpsExecEnvApplied(ctx)
	ctx = withoutTargetOpsWorkingDirApplied(ctx)

	return ctx
}

// TargetOps returns the operations selected for the target from ctx. Target ops
// working directory and exec environment overlays are applied once to the
// context used for the returned operations. TargetOps panics when the context
// has no target.
func TargetOps(ctx context.Context) Ops {
	target, ok := targetFromContext(ctx)
	if !ok {
		panic("gabric: no target in context; use gabric.NewTargetContext(ctx, gabric.LocalTarget) or gabric.NewTargetContext(ctx, gabric.RemoteTarget)")
	}

	if !targetOpsWorkingDirApplied(ctx) {
		if dir, ok := GetTargetOpsWorkingDir(ctx); ok {
			ctx = target.workingDirContext(ctx, dir)
		}

		ctx = withTargetOpsWorkingDirApplied(ctx)
	}

	if !targetOpsExecEnvApplied(ctx) {
		if overlay := GetTargetOpsExecEnv(ctx); overlay.Len() != 0 {
			ctx = target.execEnvContext(ctx, overlay.Vars()...)
		}

		ctx = withTargetOpsExecEnvApplied(ctx)
	}

	ops := target.ops(ctx)
	if ops == nil {
		panic("gabric: target ops provider returned nil")
	}

	return ops
}

// ErrNoRsyncTarget reports that the selected target has no rsync support.
var ErrNoRsyncTarget = errors.New("gabric: target does not support rsync")

// TargetRsyncOps returns the rsync builder for the target in ctx. It returns
// ErrNoRsyncTarget when the target does not support rsync.
func TargetRsyncOps(ctx context.Context) (*RsyncOps, error) {
	target, ok := TargetOps(ctx).(OpsWithRsync)
	if !ok {
		return nil, ErrNoRsyncTarget
	}

	return target.Rsync(), nil
}

// HasTarget reports whether ctx carries a target.
func HasTarget(ctx context.Context) bool {
	_, ok := contextValue(ctx, targetKey)

	return ok
}

func withDefaultRemoteTarget(ctx context.Context, next middleware.Next) error {
	if HasTarget(ctx) {
		return next(ctx)
	}

	return next(NewTargetContext(ctx, RemoteTarget))
}

// WithLocalTarget returns middleware that runs next with a local target. A
// remote working directory is mirrored into the local working directory when no
// local working directory is set.
func WithLocalTarget(ctx context.Context, next middleware.Next) error {
	return next(NewTargetContext(ctx, LocalTarget))
}

// WithRemoteTarget returns middleware that runs next with a remote target.
func WithRemoteTarget(ctx context.Context, next middleware.Next) error {
	return next(NewTargetContext(ctx, RemoteTarget))
}
