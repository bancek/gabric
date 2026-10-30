package gabrice2e

import (
	"runtime"
	"testing"

	"github.com/bancek/gabric"
)

// LocalRemote starts a LocalRemote target with a temporary remote working
// directory.
func LocalRemote(t testing.TB) *Test {
	t.Helper()

	if runtime.GOOS == "windows" {
		t.Skip("gabrice2e: LocalRemote requires /bin/sh")
	}

	ctx := t.Context()
	ctx = gabric.NewRemoteWorkingDirContext(ctx, t.TempDir())
	ctx = gabric.NewLocalRemoteContext(ctx)
	ctx = gabric.NewTargetContext(ctx, gabric.LocalRemoteTarget)

	return &Test{
		t:   t,
		ctx: ctx,
	}
}
