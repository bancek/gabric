package gabric

import (
	"bytes"
	"context"
	"io"
)

type getFunc func(ctx context.Context, path string, f func(io.Reader) error) error

// GetOps reads a local or remote path. The backend is selected by
// Local(ctx).Get or Remote(ctx).Get.
//
// The terminal methods read the file into memory (Bytes, String), stream it to
// a writer (Copy), or hand the live reader to a callback (WithReader). In
// WithReader the reader is only valid for the duration of the callback.
type GetOps struct {
	ctx  context.Context
	run  getFunc
	path string
}

func newGet(ctx context.Context, run getFunc, path string) *GetOps {
	return &GetOps{
		ctx:  ctx,
		run:  run,
		path: path,
	}
}

// Bytes reads the file and returns its contents, panicking on failure.
func (g *GetOps) Bytes() []byte {
	b, err := g.BytesErr()
	if err != nil {
		panic(err)
	}

	return b
}

// BytesErr reads the file and returns its contents.
func (g *GetOps) BytesErr() ([]byte, error) {
	var b bytes.Buffer

	if err := g.WithReaderErr(func(r io.Reader) error {
		_, err := b.ReadFrom(r)
		return err
	}); err != nil {
		return nil, err
	}

	return b.Bytes(), nil
}

// String reads the file and returns its contents as a string, panicking on
// failure.
func (g *GetOps) String() string {
	s, err := g.StringErr()
	if err != nil {
		panic(err)
	}

	return s
}

// StringErr reads the file and returns its contents as a string.
func (g *GetOps) StringErr() (string, error) {
	b, err := g.BytesErr()
	if err != nil {
		return "", err
	}

	return string(b), nil
}

// Copy writes the file to w, panicking on failure.
func (g *GetOps) Copy(w io.Writer) {
	if err := g.CopyErr(w); err != nil {
		panic(err)
	}
}

// CopyErr writes the file to w.
func (g *GetOps) CopyErr(w io.Writer) error {
	return g.WithReaderErr(func(r io.Reader) error {
		_, err := io.Copy(w, r)
		return err
	})
}

// WithReader calls f with a reader over the file, panicking on failure. The
// reader is only valid for the duration of f.
func (g *GetOps) WithReader(f func(io.Reader) error) {
	if err := g.WithReaderErr(f); err != nil {
		panic(err)
	}
}

// WithReaderErr calls f with a reader over the file. The reader is only valid
// for the duration of f.
func (g *GetOps) WithReaderErr(f func(io.Reader) error) error {
	return g.run(g.ctx, g.path, f)
}
