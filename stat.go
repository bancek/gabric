package gabric

import (
	"context"
	"io/fs"
)

// FileStat is the metadata returned by Stat.
type FileStat struct {
	Mode  fs.FileMode
	User  string
	Group string
}

type statCall struct {
	path string
}

type statFunc func(ctx context.Context, call statCall) (FileStat, error)

// StatOps reads file metadata. Build with Local(ctx).Stat(path) or
// Remote(ctx).Stat(path), then Run (panics) or RunErr.
type StatOps struct {
	ctx  context.Context
	run  statFunc
	call statCall
}

func newStat(ctx context.Context, run statFunc, path string) *StatOps {
	return &StatOps{ctx: ctx, run: run, call: statCall{path: path}}
}

// Run reads the metadata and panics on failure.
func (s *StatOps) Run() FileStat {
	stat, err := s.RunErr()
	if err != nil {
		panic(err)
	}

	return stat
}

// RunErr reads the metadata. A missing path returns an error wrapping
// fs.ErrNotExist.
func (s *StatOps) RunErr() (FileStat, error) {
	return s.run(s.ctx, s.call)
}
