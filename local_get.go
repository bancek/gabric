package gabric

import (
	"context"
	"io"
	"os"
)

// Get reads a local file. Use Local(ctx).Get(path) to build the read and one of
// its terminal methods (Bytes, String, Copy, WithReader) to perform it.
func (o *LocalOps) Get(path string) *GetOps {
	return newGet(o.ctx, getLocal, path)
}

func getLocal(ctx context.Context, path string, f func(io.Reader) error) error {
	return GetLocalBackend(ctx).Get(ctx, GetRequest{
		Path: ResolveLocalPath(ctx, path),
	}, f)
}

func (realLocalBackend) Get(ctx context.Context, req GetRequest, f func(io.Reader) error) error {
	Logger(ctx).With("localPath", req.Path).Debug("Local get")

	file, err := os.Open(req.Path)
	if err != nil {
		return err
	}
	defer file.Close()

	return f(file)
}
