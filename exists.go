package gabric

import "context"

type FileKind int

const (
	FileKindAny FileKind = iota // default: exists, any type
	FileKindDir
	FileKindFile
	FileKindLink
)

type existsCall struct {
	path string
	kind FileKind
}

type existsFunc func(ctx context.Context, call existsCall) (bool, error)

// ExistsOps probes whether a path exists, optionally restricted to a kind.
// Build with Local(ctx).Exists(path) / Remote(ctx).Exists(path), then Run
// (panics) or RunErr.
type ExistsOps struct {
	ctx  context.Context
	run  existsFunc
	call existsCall
}

func newExists(ctx context.Context, run existsFunc, path string) *ExistsOps {
	return &ExistsOps{ctx: ctx, run: run, call: existsCall{path: path}}
}

// Dir restricts the probe to directories (os.Stat / test -d).
func (e *ExistsOps) Dir() *ExistsOps { e.call.kind = FileKindDir; return e }

// File restricts the probe to regular files (os.Stat / test -f).
func (e *ExistsOps) File() *ExistsOps { e.call.kind = FileKindFile; return e }

// Link restricts the probe to symlinks, without following them (os.Lstat /
// test -L).
func (e *ExistsOps) Link() *ExistsOps { e.call.kind = FileKindLink; return e }

// Run probes the path and panics on failure.
func (e *ExistsOps) Run() bool {
	ok, err := e.RunErr()
	if err != nil {
		panic(err)
	}
	return ok
}

// RunErr probes the path. A missing path (or a path of the wrong kind) is
// (false, nil).
func (e *ExistsOps) RunErr() (bool, error) {
	return e.run(e.ctx, e.call)
}
