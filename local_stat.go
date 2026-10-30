package gabric

import (
	"context"
	"os"
)

// Stat reads metadata for a local path. Use Local(ctx).Stat(path) to build the
// operation and Run/RunErr to perform it. It follows symlinks.
func (o *LocalOps) Stat(path string) *StatOps {
	return newStat(o.ctx, runLocalStat, path)
}

func runLocalStat(ctx context.Context, call statCall) (FileStat, error) {
	return GetLocalBackend(ctx).Stat(ctx, StatRequest{
		Path: ResolveLocalPath(ctx, call.path),
	})
}

func (realLocalBackend) Stat(_ context.Context, req StatRequest) (FileStat, error) {
	info, err := os.Stat(req.Path)
	if err != nil {
		return FileStat{}, err
	}

	user, group := fileOwner(info)

	return FileStat{Mode: info.Mode(), User: user, Group: group}, nil
}
