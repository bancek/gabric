package gabric

import (
	"context"
	"os"
)

// Exists probes whether a local path exists. Use Local(ctx).Exists(path) to
// build the operation, optionally restricted with Dir, File or Link, and
// Run/RunErr to perform it.
func (o *LocalOps) Exists(path string) *ExistsOps {
	return newExists(o.ctx, runLocalExists, path)
}

func runLocalExists(ctx context.Context, call existsCall) (bool, error) {
	return GetLocalBackend(ctx).Exists(ctx, ExistsRequest{
		Path: ResolveLocalPath(ctx, call.path),
		Kind: call.kind,
	})
}

func (realLocalBackend) Exists(ctx context.Context, req ExistsRequest) (bool, error) {
	var (
		info os.FileInfo
		err  error
	)

	if req.Kind == FileKindLink {
		info, err = os.Lstat(req.Path) // do not follow
	} else {
		info, err = os.Stat(req.Path) // follows symlinks (-d/-f semantics)
	}

	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}

	switch req.Kind {
	case FileKindDir:
		return info.IsDir(), nil
	case FileKindFile:
		return info.Mode().IsRegular(), nil
	case FileKindLink:
		return info.Mode()&os.ModeSymlink != 0, nil
	default:
		return true, nil
	}
}
