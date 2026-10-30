package gabrice2e

import (
	"context"
	"testing"
)

// Test is the common base for end-to-end test targets.
type Test struct {
	t   testing.TB
	ctx context.Context
}

// T returns the test handle.
func (g *Test) T() testing.TB {
	return g.t
}

// Context returns the target context.
func (g *Test) Context() context.Context {
	return g.ctx
}
