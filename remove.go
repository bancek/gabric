package gabric

import (
	"context"
)

type removeCall struct {
	path      string
	recursive bool
	force     bool
}

type removeFunc func(ctx context.Context, call removeCall) error

// RemoveOps removes a local or remote file or directory. The backend is
// selected by Local(ctx).Remove or Remote(ctx).Remove.
//
// Remove removes a file, symlink, or empty directory. Recursive also removes a
// directory and its contents. A missing path is an error unless Force is set.
type RemoveOps struct {
	ctx  context.Context
	run  removeFunc
	call removeCall
}

func newRemove(ctx context.Context, run removeFunc, path string) *RemoveOps {
	return &RemoveOps{
		ctx:  ctx,
		run:  run,
		call: removeCall{path: path},
	}
}

// Recursive removes directories and their contents.
func (r *RemoveOps) Recursive() *RemoveOps {
	r.call.recursive = true
	return r
}

// Force ignores a missing path. It does not make a non-empty directory
// removable; use Recursive for that.
func (r *RemoveOps) Force() *RemoveOps {
	r.call.force = true
	return r
}

// Run removes the path and panics on failure.
func (r *RemoveOps) Run() *RemoveOps {
	if err := r.RunErr(); err != nil {
		panic(err)
	}
	return r
}

// RunErr removes the path.
func (r *RemoveOps) RunErr() error {
	return r.run(r.ctx, r.call)
}
