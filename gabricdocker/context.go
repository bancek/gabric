package gabricdocker

import (
	"context"
	"errors"
)

const containerKey contextKey[string] = "gabricdocker.container"

// ErrNoContainer reports that no Docker container is configured.
var ErrNoContainer = errors.New("gabricdocker: no container")

// NewContainerContext returns a context carrying container.
func NewContainerContext(ctx context.Context, container string) context.Context {
	return withContextValue(ctx, containerKey, container)
}

// GetContainer returns the container from ctx. It returns ErrNoContainer when
// the container is unset or empty.
func GetContainer(ctx context.Context) (string, error) {
	container, ok := contextValue(ctx, containerKey)
	if !ok || container == "" {
		return "", ErrNoContainer
	}

	return container, nil
}

// HasContainer reports whether ctx carries a non-empty container.
func HasContainer(ctx context.Context) bool {
	container, ok := contextValue(ctx, containerKey)

	return ok && container != ""
}
