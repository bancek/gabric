package gabric

import (
	"context"
	"errors"
)

// Exists probes whether a remote path exists. Use Remote(ctx).Exists(path) to
// build the operation, optionally restricted with Dir, File or Link, and
// Run/RunErr to perform it.
func (o *RemoteOps) Exists(path string) *ExistsOps {
	return newExists(o.ctx, runRemoteExists, path)
}

func runRemoteExists(ctx context.Context, call existsCall) (bool, error) {
	return GetRemoteBackend(ctx).Exists(withoutRemoteWorkingDir(ctx), ExistsRequest{
		Path: ResolveRemotePath(ctx, call.path),
		Kind: call.kind,
	})
}

func (r realRemoteBackend) Exists(ctx context.Context, req ExistsRequest) (bool, error) {
	arg := "-e"

	switch req.Kind {
	case FileKindDir:
		arg = "-d"
	case FileKindFile:
		arg = "-f"
	case FileKindLink:
		arg = "-L"
	}

	err := r.execQuiet(ctx, []string{"test", arg, req.Path}, nil)
	if err != nil {
		if exitErr, ok := errors.AsType[*ExecExitError](err); ok && exitErr.Status == 1 {
			return false, nil
		}

		return false, err
	}

	return true, nil
}

func (r realRemoteBackend) notExist(ctx context.Context, p string) bool {
	exists, err := r.Exists(ctx, ExistsRequest{Path: p, Kind: FileKindAny})

	return err == nil && !exists
}
