package gabric

import (
	"context"
	"os"
)

// Mkdir creates a local directory. Use Local(ctx).Mkdir(path) to build the
// operation and Run/RunErr to perform it.
func (o *LocalOps) Mkdir(path string) *MkdirOps {
	return newMkdir(o.ctx, runLocalMkdir, path)
}

func runLocalMkdir(ctx context.Context, call mkdirCall) error {
	return GetLocalBackend(ctx).Mkdir(ctx, MkdirRequest{
		Path:      ResolveLocalPath(ctx, call.path),
		Recursive: call.recursive,
		Mode:      call.mode,
		User:      call.user,
		Group:     call.group,
	})
}

func (realLocalBackend) Mkdir(ctx context.Context, req MkdirRequest) error {
	mode := req.Mode
	if mode == 0 {
		mode = 0o777
	}

	if info, err := os.Stat(req.Path); err == nil && info.IsDir() {
		// already exists
	} else if req.Recursive {
		if err := os.MkdirAll(req.Path, mode); err != nil {
			return err
		}
	} else if err := os.Mkdir(req.Path, mode); err != nil {
		return err
	}

	if err := localChown(req.Path, req.User, req.Group); err != nil {
		return err
	}

	if req.Mode != 0 {
		if err := os.Chmod(req.Path, req.Mode); err != nil {
			return err
		}
	}

	return nil
}
