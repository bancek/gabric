package gabrice2e

import (
	"context"
	"testing"
	"uuid"
)

// Network is a Docker network created by RunNetwork.
type Network struct {
	t    testing.TB
	name string
}

// RunNetwork creates a Docker network and removes it with t.Cleanup.
func RunNetwork(t testing.TB) *Network {
	t.Helper()
	requireDocker(t)

	name := "gabrice2e-net-" + uuid.NewV4().String()

	if _, err := runDocker(t.Context(), "network", "create", name); err != nil {
		t.Fatalf("gabrice2e: docker network create %s: %v", name, err)
	}

	t.Cleanup(func() {
		_, _ = runDocker(context.Background(), "network", "rm", name)
	})

	return &Network{t: t, name: name}
}

// Name returns the Docker network name.
func (n *Network) Name() string {
	return n.name
}
