package gabrice2e

import (
	"testing"

	"github.com/bancek/gabric"
)

// Local returns a plain local target with a temporary working directory.
func Local(t testing.TB) *Test {
	t.Helper()

	ctx := t.Context()
	ctx = gabric.NewLocalWorkingDirContext(ctx, t.TempDir())
	ctx = gabric.NewTargetContext(ctx, gabric.LocalTarget)

	return &Test{
		t:   t,
		ctx: ctx,
	}
}
