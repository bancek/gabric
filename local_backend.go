package gabric

import (
	"context"
	"errors"
	"fmt"
	"io"
)

// ErrLocalBackendUnsupported reports that a LocalBackendFunc operation has no
// implementation.
var ErrLocalBackendUnsupported = errors.New("local backend operation not supported")

// LocalBackend executes local operations. Exec returns ExecExitError for a
// non-zero command status.
type LocalBackend interface {
	Exec(ctx context.Context, req ExecRequest) error
	Put(ctx context.Context, req PutRequest) error
	Get(ctx context.Context, req GetRequest, f func(io.Reader) error) error
	Exists(ctx context.Context, req ExistsRequest) (bool, error)
	Stat(ctx context.Context, req StatRequest) (FileStat, error)
	Mkdir(ctx context.Context, req MkdirRequest) error
	Remove(ctx context.Context, req RemoveRequest) error
	List(ctx context.Context, req ListRequest) ([]ListEntry, error)
}

// LocalBackendFunc implements LocalBackend with optional functions.
type LocalBackendFunc struct {
	ExecFunc   func(ctx context.Context, req ExecRequest) error
	PutFunc    func(ctx context.Context, req PutRequest) error
	GetFunc    func(ctx context.Context, req GetRequest, f func(io.Reader) error) error
	ExistsFunc func(ctx context.Context, req ExistsRequest) (bool, error)
	StatFunc   func(ctx context.Context, req StatRequest) (FileStat, error)
	MkdirFunc  func(ctx context.Context, req MkdirRequest) error
	RemoveFunc func(ctx context.Context, req RemoveRequest) error
	ListFunc   func(ctx context.Context, req ListRequest) ([]ListEntry, error)
}

// Exec calls ExecFunc, or returns ErrLocalBackendUnsupported.
func (b LocalBackendFunc) Exec(ctx context.Context, req ExecRequest) error {
	if b.ExecFunc == nil {
		return fmt.Errorf("exec: %w", ErrLocalBackendUnsupported)
	}

	return b.ExecFunc(ctx, req)
}

// Put calls PutFunc, or returns ErrLocalBackendUnsupported.
func (b LocalBackendFunc) Put(ctx context.Context, req PutRequest) error {
	if b.PutFunc == nil {
		return fmt.Errorf("put: %w", ErrLocalBackendUnsupported)
	}

	return b.PutFunc(ctx, req)
}

// Get calls GetFunc, or returns ErrLocalBackendUnsupported.
func (b LocalBackendFunc) Get(ctx context.Context, req GetRequest, f func(io.Reader) error) error {
	if b.GetFunc == nil {
		return fmt.Errorf("get: %w", ErrLocalBackendUnsupported)
	}

	return b.GetFunc(ctx, req, f)
}

// Exists calls ExistsFunc, or returns ErrLocalBackendUnsupported.
func (b LocalBackendFunc) Exists(ctx context.Context, req ExistsRequest) (bool, error) {
	if b.ExistsFunc == nil {
		return false, fmt.Errorf("exists: %w", ErrLocalBackendUnsupported)
	}

	return b.ExistsFunc(ctx, req)
}

// Stat calls StatFunc, or returns ErrLocalBackendUnsupported.
func (b LocalBackendFunc) Stat(ctx context.Context, req StatRequest) (FileStat, error) {
	if b.StatFunc == nil {
		return FileStat{}, fmt.Errorf("stat: %w", ErrLocalBackendUnsupported)
	}

	return b.StatFunc(ctx, req)
}

// Mkdir calls MkdirFunc, or returns ErrLocalBackendUnsupported.
func (b LocalBackendFunc) Mkdir(ctx context.Context, req MkdirRequest) error {
	if b.MkdirFunc == nil {
		return fmt.Errorf("mkdir: %w", ErrLocalBackendUnsupported)
	}

	return b.MkdirFunc(ctx, req)
}

// Remove calls RemoveFunc, or returns ErrLocalBackendUnsupported.
func (b LocalBackendFunc) Remove(ctx context.Context, req RemoveRequest) error {
	if b.RemoveFunc == nil {
		return fmt.Errorf("remove: %w", ErrLocalBackendUnsupported)
	}

	return b.RemoveFunc(ctx, req)
}

// List calls ListFunc, or returns ErrLocalBackendUnsupported.
func (b LocalBackendFunc) List(ctx context.Context, req ListRequest) ([]ListEntry, error) {
	if b.ListFunc == nil {
		return nil, fmt.Errorf("list: %w", ErrLocalBackendUnsupported)
	}

	return b.ListFunc(ctx, req)
}

const localBackendKey contextKey[LocalBackend] = "gabric.localBackend"

// NewLocalBackendContext returns a context carrying backend.
func NewLocalBackendContext(ctx context.Context, backend LocalBackend) context.Context {
	return withContextValue(ctx, localBackendKey, backend)
}

// GetLocalBackend returns the local backend from ctx, falling back to the
// OS-backed backend.
func GetLocalBackend(ctx context.Context) LocalBackend {
	if backend, ok := contextValue(ctx, localBackendKey); ok {
		return backend
	}

	return realLocalBackend{}
}

// HasLocalBackend reports whether ctx carries a local backend.
func HasLocalBackend(ctx context.Context) bool {
	backend, ok := contextValue(ctx, localBackendKey)

	return ok && backend != nil
}

type realLocalBackend struct{}
