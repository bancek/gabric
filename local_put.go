package gabric

import (
	"context"
	"os"
	"path/filepath"
)

// Put writes a file to the local path. Use Local(ctx).Put(path) to build the
// write and Run/RunErr to perform it.
func (o *LocalOps) Put(path string) *PutOps {
	return newPut(o.ctx, runLocalPut, path)
}

func runLocalPut(ctx context.Context, call putCall) error {
	return GetLocalBackend(ctx).Put(ctx, PutRequest{
		Path:         ResolveLocalPath(ctx, call.path),
		Content:      call.reader,
		Mode:         call.mode,
		User:         call.user,
		Group:        call.group,
		EnsureParent: call.ensureParent,
	})
}

func (realLocalBackend) Put(ctx context.Context, req PutRequest) error {
	if req.EnsureParent {
		if err := mkdirLocalParent(req.Path); err != nil {
			return err
		}
	}

	Logger(ctx).With("localPath", req.Path).Debug("Local put")

	file, err := os.Create(req.Path)
	if err != nil {
		return err
	}

	if err := copyAndCloseWriter(file, req.Content); err != nil {
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

// mkdirLocalParent creates the parent directory of path, recursively. The mode
// is left to the process umask (0777 before umask, like "mkdir -p").
func mkdirLocalParent(path string) error {
	dir := filepath.Dir(path)

	if dir == "" || dir == "." || dir == "/" {
		return nil
	}

	return os.MkdirAll(dir, 0o777)
}
