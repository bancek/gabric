package gabric

import (
	"context"
)

// Ops is the common operation surface implemented by *LocalOps and *RemoteOps.
// Operations are bound to the context the ops were created with (Local(ctx) /
// Remote(ctx)).
//
// Builders (Exec, Put) are configured and terminated with Run (panics) or
// RunErr (returns the error); Get is a builder too, with per-terminal panic and
// Err forms. Exists is a builder whose Run returns a bool (panicking on error)
// and whose RunErr returns (bool, error). Stat is a builder whose Run returns
// the metadata (panicking on error) and whose RunErr returns (FileStat, error).
// Remove is a builder for deleting files and directories. List is direct;
// ListErr is the error-returning form. WithWorkingDir and WithExecEnv scope a
// working directory or exec environment for a callback.
type Ops interface {
	Exec(cmd ...string) *ExecOps

	Put(path string) *PutOps

	Get(path string) *GetOps

	Mkdir(path string) *MkdirOps

	Remove(path string) *RemoveOps

	Exists(path string) *ExistsOps

	Stat(path string) *StatOps

	List(path string) []ListEntry
	ListErr(path string) ([]ListEntry, error)

	WithWorkingDir(path string, f func(ctx context.Context))
	WithWorkingDirErr(path string, f func(ctx context.Context) error) error

	WithExecEnv(env ExecEnv, f func(ctx context.Context))
	WithExecEnvErr(env ExecEnv, f func(ctx context.Context) error) error
}

// OpsWithRsync is the optional rsync capability of a target. Remote targets
// implement it; a plain local target does not.
type OpsWithRsync interface {
	Ops
	Rsync() *RsyncOps
}

var (
	_ Ops          = (*LocalOps)(nil)
	_ Ops          = (*RemoteOps)(nil)
	_ OpsWithRsync = (*RemoteOps)(nil)
)
