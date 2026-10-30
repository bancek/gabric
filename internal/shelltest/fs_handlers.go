package shelltest

import (
	"context"
	"io"
	"io/fs"

	"mvdan.cc/sh/v3/interp"
)

func (c *Core) statHandler(ctx context.Context, name string, followSymlinks bool) (fs.FileInfo, error) {
	hc := interp.HandlerCtx(ctx)

	return c.fs.statAt(hc.Dir, name, followSymlinks)
}

func (c *Core) readDirHandler(ctx context.Context, name string) ([]fs.DirEntry, error) {
	hc := interp.HandlerCtx(ctx)

	return c.fs.readDirAt(hc.Dir, name)
}

func (c *Core) accessHandler(ctx context.Context, name string, mode interp.AccessMode) error {
	hc := interp.HandlerCtx(ctx)

	return c.fs.accessAt(hc.Dir, name, mode)
}

func (c *Core) openHandler(ctx context.Context, name string, flag int, perm fs.FileMode) (io.ReadWriteCloser, error) {
	if name == "/dev/null" {
		return devNull{}, nil
	}

	hc := interp.HandlerCtx(ctx)

	return c.fs.openAt(hc.Dir, name, flag, perm)
}

func (f *FS) accessAt(cwd, name string, _ interp.AccessMode) error {
	_, err := f.statAt(cwd, name, true)

	return err
}
