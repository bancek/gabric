package gabric

import (
	"context"
	"errors"
	"fmt"
	"io"
)

// ErrRemoteBackendUnsupported reports that a RemoteBackendFunc operation has no
// implementation.
var ErrRemoteBackendUnsupported = errors.New("remote backend operation not supported")

// RemoteBackend executes remote operations.
type RemoteBackend interface {
	Exec(ctx context.Context, req ExecRequest) error
	Put(ctx context.Context, req PutRequest) error
	Get(ctx context.Context, req GetRequest, f func(io.Reader) error) error
	Exists(ctx context.Context, req ExistsRequest) (bool, error)
	Stat(ctx context.Context, req StatRequest) (FileStat, error)
	Mkdir(ctx context.Context, req MkdirRequest) error
	Remove(ctx context.Context, req RemoveRequest) error
	List(ctx context.Context, req ListRequest) ([]ListEntry, error)
	Rsync(ctx context.Context, req RsyncRequest, stdout, stderr io.Writer) error
}

// RemoteBackendFunc implements RemoteBackend with optional functions.
type RemoteBackendFunc struct {
	ExecFunc   func(ctx context.Context, req ExecRequest) error
	PutFunc    func(ctx context.Context, req PutRequest) error
	GetFunc    func(ctx context.Context, req GetRequest, f func(io.Reader) error) error
	ExistsFunc func(ctx context.Context, req ExistsRequest) (bool, error)
	StatFunc   func(ctx context.Context, req StatRequest) (FileStat, error)
	MkdirFunc  func(ctx context.Context, req MkdirRequest) error
	RemoveFunc func(ctx context.Context, req RemoveRequest) error
	ListFunc   func(ctx context.Context, req ListRequest) ([]ListEntry, error)
	RsyncFunc  func(ctx context.Context, req RsyncRequest, stdout, stderr io.Writer) error
}

// Exec calls ExecFunc, or returns ErrRemoteBackendUnsupported.
func (b RemoteBackendFunc) Exec(ctx context.Context, req ExecRequest) error {
	if b.ExecFunc == nil {
		return fmt.Errorf("exec: %w", ErrRemoteBackendUnsupported)
	}

	return b.ExecFunc(ctx, req)
}

// Put calls PutFunc, or returns ErrRemoteBackendUnsupported.
func (b RemoteBackendFunc) Put(ctx context.Context, req PutRequest) error {
	if b.PutFunc == nil {
		return fmt.Errorf("put: %w", ErrRemoteBackendUnsupported)
	}

	return b.PutFunc(ctx, req)
}

// Get calls GetFunc, or returns ErrRemoteBackendUnsupported.
func (b RemoteBackendFunc) Get(ctx context.Context, req GetRequest, f func(io.Reader) error) error {
	if b.GetFunc == nil {
		return fmt.Errorf("get: %w", ErrRemoteBackendUnsupported)
	}

	return b.GetFunc(ctx, req, f)
}

// Exists calls ExistsFunc, or returns ErrRemoteBackendUnsupported.
func (b RemoteBackendFunc) Exists(ctx context.Context, req ExistsRequest) (bool, error) {
	if b.ExistsFunc == nil {
		return false, fmt.Errorf("exists: %w", ErrRemoteBackendUnsupported)
	}

	return b.ExistsFunc(ctx, req)
}

// Stat calls StatFunc, or returns ErrRemoteBackendUnsupported.
func (b RemoteBackendFunc) Stat(ctx context.Context, req StatRequest) (FileStat, error) {
	if b.StatFunc == nil {
		return FileStat{}, fmt.Errorf("stat: %w", ErrRemoteBackendUnsupported)
	}

	return b.StatFunc(ctx, req)
}

// Mkdir calls MkdirFunc, or returns ErrRemoteBackendUnsupported.
func (b RemoteBackendFunc) Mkdir(ctx context.Context, req MkdirRequest) error {
	if b.MkdirFunc == nil {
		return fmt.Errorf("mkdir: %w", ErrRemoteBackendUnsupported)
	}

	return b.MkdirFunc(ctx, req)
}

// Remove calls RemoveFunc, or returns ErrRemoteBackendUnsupported.
func (b RemoteBackendFunc) Remove(ctx context.Context, req RemoveRequest) error {
	if b.RemoveFunc == nil {
		return fmt.Errorf("remove: %w", ErrRemoteBackendUnsupported)
	}

	return b.RemoveFunc(ctx, req)
}

// Rsync calls RsyncFunc, or returns ErrRemoteBackendUnsupported.
func (b RemoteBackendFunc) Rsync(ctx context.Context, req RsyncRequest, stdout, stderr io.Writer) error {
	if b.RsyncFunc == nil {
		return fmt.Errorf("rsync: %w", ErrRemoteBackendUnsupported)
	}

	return b.RsyncFunc(ctx, req, stdout, stderr)
}

// List calls ListFunc, or returns ErrRemoteBackendUnsupported.
func (b RemoteBackendFunc) List(ctx context.Context, req ListRequest) ([]ListEntry, error) {
	if b.ListFunc == nil {
		return nil, fmt.Errorf("list: %w", ErrRemoteBackendUnsupported)
	}

	return b.ListFunc(ctx, req)
}

const remoteBackendKey contextKey[RemoteBackend] = "gabric.remoteBackend"

// NewRemoteBackendContext returns a context carrying backend.
func NewRemoteBackendContext(ctx context.Context, backend RemoteBackend) context.Context {
	return withContextValue(ctx, remoteBackendKey, backend)
}

// GetRemoteBackend returns the remote backend from ctx, falling back to the
// default remote backend.
func GetRemoteBackend(ctx context.Context) RemoteBackend {
	if backend, ok := contextValue(ctx, remoteBackendKey); ok {
		return backend
	}

	return realRemoteBackend{}
}

// HasRemoteBackend reports whether ctx carries a remote backend.
func HasRemoteBackend(ctx context.Context) bool {
	backend, ok := contextValue(ctx, remoteBackendKey)

	return ok && backend != nil
}

type realRemoteBackend struct{}
