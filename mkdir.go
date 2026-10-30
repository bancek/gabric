package gabric

import (
	"context"
	"os"
)

type mkdirCall struct {
	path      string
	mode      os.FileMode
	user      string
	group     string
	recursive bool
}

type mkdirFunc func(ctx context.Context, call mkdirCall) error

// MkdirOps creates a local or remote directory. The backend is selected by
// Local(ctx).Mkdir or Remote(ctx).Mkdir.
//
// Mkdir creates a single directory; call Recursive to also create missing
// parents (mkdir -p). An existing directory is not an error: its owner and mode
// (when set) are still applied.
type MkdirOps struct {
	ctx  context.Context
	run  mkdirFunc
	call mkdirCall
}

func newMkdir(ctx context.Context, run mkdirFunc, path string) *MkdirOps {
	return &MkdirOps{
		ctx:  ctx,
		run:  run,
		call: mkdirCall{path: path},
	}
}

// Recursive also creates missing parent directories (mkdir -p).
func (m *MkdirOps) Recursive() *MkdirOps {
	m.call.recursive = true
	return m
}

// Mode sets the directory mode. A zero mode leaves the mode unchanged.
func (m *MkdirOps) Mode(mode os.FileMode) *MkdirOps {
	m.call.mode = mode
	return m
}

// User sets the owning user. An empty string leaves the owner unchanged.
func (m *MkdirOps) User(user string) *MkdirOps {
	m.call.user = user
	return m
}

// Group sets the owning group. An empty string leaves the group unchanged.
func (m *MkdirOps) Group(group string) *MkdirOps {
	m.call.group = group
	return m
}

// Run creates the directory and panics on failure.
func (m *MkdirOps) Run() *MkdirOps {
	if err := m.RunErr(); err != nil {
		panic(err)
	}
	return m
}

// RunErr creates the directory.
func (m *MkdirOps) RunErr() error {
	return m.run(m.ctx, m.call)
}
