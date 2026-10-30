package gabric

import (
	"context"
)

// WithWorkingDirErr runs f with the remote working directory set to dir and
// returns f's error.
func (o *RemoteOps) WithWorkingDirErr(dir string, f func(ctx context.Context) error) error {
	return f(NewRemoteWorkingDirContext(o.ctx, ResolveRemotePath(o.ctx, dir)))
}

// WithWorkingDir runs f with the remote working directory set to dir. Use
// WithWorkingDirErr for an error-returning callback.
func (o *RemoteOps) WithWorkingDir(dir string, f func(ctx context.Context)) {
	Must(o.WithWorkingDirErr(dir, func(ctx context.Context) error {
		f(ctx)
		return nil
	}))
}
