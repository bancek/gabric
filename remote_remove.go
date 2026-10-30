package gabric

import (
	"context"
)

// Remove removes a remote path. Use Remote(ctx).Remove(path) to build the
// operation and Run/RunErr to perform it.
func (o *RemoteOps) Remove(path string) *RemoveOps {
	return newRemove(o.ctx, runRemoteRemove, path)
}

func runRemoteRemove(ctx context.Context, call removeCall) error {
	backendCtx := withoutRemoteWorkingDir(ctx)

	return GetRemoteBackend(backendCtx).Remove(backendCtx, RemoveRequest{
		Path:      ResolveRemotePath(ctx, call.path),
		Recursive: call.recursive,
		Force:     call.force,
	})
}

func (r realRemoteBackend) Remove(ctx context.Context, req RemoveRequest) error {
	Logger(ctx).With("remotePath", req.Path).Debug("Remote remove")

	flags := ""
	if req.Recursive {
		flags += "r"
	}
	if req.Force {
		flags += "f"
	}

	args := []string{"rm"}
	if flags != "" {
		args = append(args, "-"+flags)
	}
	args = append(args, "--", req.Path)

	return r.execQuiet(ctx, args, nil)
}
