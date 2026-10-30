package gabric

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"strings"
)

type putCall struct {
	path         string
	reader       io.Reader
	hasContent   bool
	user         string
	group        string
	mode         os.FileMode
	ensureParent bool
}

type putFunc func(ctx context.Context, call putCall) error

// PutOps writes a file to a local or remote path. The backend is selected by
// Local(ctx).Put or Remote(ctx).Put.
//
// Content is set with Reader, String or Bytes; the last one wins. An optional
// owner (User/Group) and Mode are applied after the content is written.
type PutOps struct {
	ctx  context.Context
	run  putFunc
	call putCall
}

func newPut(ctx context.Context, run putFunc, path string) *PutOps {
	return &PutOps{
		ctx:  ctx,
		run:  run,
		call: putCall{path: path},
	}
}

// Reader sets the content from reader, replacing any previously set content.
func (p *PutOps) Reader(reader io.Reader) *PutOps {
	p.call.reader = reader
	p.call.hasContent = true
	return p
}

// String sets the content from s, replacing any previously set content.
func (p *PutOps) String(s string) *PutOps {
	return p.Reader(strings.NewReader(s))
}

// Bytes sets the content from b, replacing any previously set content.
// Bytes(nil) writes an empty file.
func (p *PutOps) Bytes(b []byte) *PutOps {
	return p.Reader(bytes.NewReader(b))
}

// User sets the owning user. An empty string leaves the owner unchanged.
func (p *PutOps) User(user string) *PutOps {
	p.call.user = user
	return p
}

// Group sets the owning group. An empty string leaves the group unchanged.
func (p *PutOps) Group(group string) *PutOps {
	p.call.group = group
	return p
}

// Mode sets the file mode. A zero mode leaves the mode unchanged.
func (p *PutOps) Mode(mode os.FileMode) *PutOps {
	p.call.mode = mode
	return p
}

// EnsureParent creates the target's parent directory, recursively, before
// writing. Created directories use the process umask (local) or the mkdir
// default (remote).
func (p *PutOps) EnsureParent() *PutOps {
	p.call.ensureParent = true
	return p
}

// Run writes the file and panics on failure.
func (p *PutOps) Run() *PutOps {
	if err := p.RunErr(); err != nil {
		panic(err)
	}
	return p
}

// RunErr writes the file.
func (p *PutOps) RunErr() error {
	if !p.call.hasContent {
		return errors.New("put: no content set (use Reader, String or Bytes)")
	}

	if p.call.reader == nil {
		return errors.New("put: nil reader (use Bytes(nil) or String(\"\") for empty content)")
	}

	return p.run(p.ctx, p.call)
}
