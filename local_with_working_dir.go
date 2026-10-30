package gabric

import (
	"context"
)

// WithWorkingDirErr runs f with the local working directory set to path and
// returns f's error.
func (o *LocalOps) WithWorkingDirErr(path string, f func(ctx context.Context) error) error {
	path = ResolveLocalPath(o.ctx, path)

	return f(NewLocalWorkingDirContext(o.ctx, path))
}

// WithWorkingDir runs f with the local working directory set to path. Use
// WithWorkingDirErr for an error-returning callback.
func (o *LocalOps) WithWorkingDir(path string, f func(ctx context.Context)) {
	Must(o.WithWorkingDirErr(path, func(ctx context.Context) error {
		f(ctx)
		return nil
	}))
}
