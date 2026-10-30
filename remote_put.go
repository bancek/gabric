package gabric

import (
	"context"
	"path"
)

// Put writes a file to the remote path. Use Remote(ctx).Put(path) to build the
// write and Run/RunErr to perform it. The content is streamed to dd.
func (o *RemoteOps) Put(path string) *PutOps {
	return newPut(o.ctx, runRemotePut, path)
}

func runRemotePut(ctx context.Context, call putCall) error {
	return GetRemoteBackend(ctx).Put(withoutRemoteWorkingDir(ctx), PutRequest{
		Path:         ResolveRemotePath(ctx, call.path),
		Content:      call.reader,
		Mode:         call.mode,
		User:         call.user,
		Group:        call.group,
		EnsureParent: call.ensureParent,
	})
}

func (r realRemoteBackend) Put(ctx context.Context, req PutRequest) error {
	if req.EnsureParent {
		if dir := parentDir(req.Path); dir != "" {
			if err := r.execQuiet(ctx, []string{"mkdir", "-p", dir}, nil); err != nil {
				return err
			}
		}
	}

	if err := r.execQuiet(ctx, []string{"dd", "bs=1048576", "of=" + req.Path}, req.Content); err != nil {
		return err
	}

	if owner := ownerSpec(req.User, req.Group); owner != "" {
		if err := r.execQuiet(ctx, []string{"chown", owner, req.Path}, nil); err != nil {
			return err
		}
	}

	if req.Mode != 0 {
		if err := r.execQuiet(ctx, []string{"chmod", modeString(req.Mode), req.Path}, nil); err != nil {
			return err
		}
	}

	return nil
}

// parentDir returns the parent of the remote path, or "" when there is nothing
// to create.
func parentDir(p string) string {
	dir := path.Dir(p)

	if dir == "" || dir == "." || dir == "/" {
		return ""
	}

	return dir
}
