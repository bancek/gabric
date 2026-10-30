package gabric

import (
	"context"
	"errors"
	"io/fs"
	"os"
)

// Remove removes a local path. Use Local(ctx).Remove(path) to build the
// operation and Run/RunErr to perform it.
func (o *LocalOps) Remove(path string) *RemoveOps {
	return newRemove(o.ctx, runLocalRemove, path)
}

func runLocalRemove(ctx context.Context, call removeCall) error {
	return GetLocalBackend(ctx).Remove(ctx, RemoveRequest{
		Path:      ResolveLocalPath(ctx, call.path),
		Recursive: call.recursive,
		Force:     call.force,
	})
}

func (realLocalBackend) Remove(ctx context.Context, req RemoveRequest) error {
	Logger(ctx).With("localPath", req.Path).Debug("Local remove")

	if req.Recursive {
		if !req.Force {
			if _, err := os.Lstat(req.Path); err != nil {
				return err
			}
		}

		return os.RemoveAll(req.Path)
	}

	err := os.Remove(req.Path)
	if err != nil {
		if req.Force && errors.Is(err, fs.ErrNotExist) {
			return nil
		}

		return err
	}

	return nil
}
