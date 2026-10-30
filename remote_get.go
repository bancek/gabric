package gabric

import (
	"context"
	"fmt"
	"io"
	"io/fs"
)

// Get reads a remote file. Use Remote(ctx).Get(path) to build the read and one
// of its terminal methods (Bytes, String, Copy, WithReader) to perform it.
func (o *RemoteOps) Get(path string) *GetOps {
	return newGet(o.ctx, getRemote, path)
}

func getRemote(ctx context.Context, p string, f func(io.Reader) error) error {
	return GetRemoteBackend(ctx).Get(
		withoutRemoteWorkingDir(ctx),
		GetRequest{Path: ResolveRemotePath(ctx, p)},
		f,
	)
}

func (r realRemoteBackend) Get(ctx context.Context, req GetRequest, f func(io.Reader) error) error {
	Logger(ctx).With("remotePath", req.Path).Debug("Remote get")

	baseCtx := ctx
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	pr, pw := io.Pipe()

	var cmdErr error

	done := make(chan struct{})

	go func() {
		dir := GetRemoteWorkingDir(ctx)

		cmdErr = r.Exec(ctx, ExecRequest{
			Cmd:    []string{"cat", "--", req.Path},
			Dir:    dir,
			Env:    GetRemoteExecEnv(ctx),
			Stdout: pw,
			Stderr: io.Discard,
		})
		pw.Close()
		close(done)
	}()

	err := f(pr)
	pr.Close()
	cancel()
	<-done

	if err != nil {
		return err
	}

	if cmdErr != nil {
		if r.notExist(baseCtx, req.Path) {
			return fmt.Errorf("%w: %s", fs.ErrNotExist, req.Path)
		}

		return cmdErr
	}

	return nil
}
