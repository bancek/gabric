package gabric

import (
	"context"
)

// Mkdir creates a remote directory. Use Remote(ctx).Mkdir(path) to build the
// operation and Run/RunErr to perform it.
func (o *RemoteOps) Mkdir(p string) *MkdirOps {
	return newMkdir(o.ctx, runRemoteMkdir, p)
}

func runRemoteMkdir(ctx context.Context, call mkdirCall) error {
	return GetRemoteBackend(ctx).Mkdir(withoutRemoteWorkingDir(ctx), MkdirRequest{
		Path:      ResolveRemotePath(ctx, call.path),
		Recursive: call.recursive,
		Mode:      call.mode,
		User:      call.user,
		Group:     call.group,
	})
}

func (r realRemoteBackend) Mkdir(ctx context.Context, req MkdirRequest) error {
	exists, err := r.Exists(ctx, ExistsRequest{Path: req.Path, Kind: FileKindDir})
	if err != nil {
		return err
	}

	if !exists {
		args := []string{"mkdir"}
		if req.Recursive {
			args = append(args, "-p")
		}
		args = append(args, req.Path)

		if err := r.execQuiet(ctx, args, nil); err != nil {
			return err
		}
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
