package modules_test

import (
	"io/fs"
	"testing"

	"github.com/bancek/gabric/input"
	"github.com/bancek/gabric/reconcile/modules"
	"github.com/bancek/gabric/reconcile/modules/moduletest"
)

func TestDirE2E(t *testing.T) {
	moduletest.ForEachBaseTarget(t, func(g *moduletest.Target) {
		t := g.T()
		path := g.Path("created-dir")

		dir := modules.Dir(input.Static(path))
		if g.POSIXModes {
			dir = dir.Mode(modules.FileMode(0o750))
		}
		node := dir.Build().Node()

		moduletest.AssertDryRun(t, g, node)
		moduletest.AssertConverges(t, g, node)

		if g.POSIXModes {
			stat, err := g.Ops().Stat(path).RunErr()
			if err != nil {
				t.Fatalf("stat %s: %v", path, err)
			}
			if got, want := stat.Mode.Perm(), fs.FileMode(0o750); got != want {
				t.Errorf("mode = %v, want %v", got, want)
			}
		}

		if !g.POSIXModes {
			return
		}

		wrongModePath := g.Path("wrong-mode-dir")
		if err := g.Ops().Mkdir(wrongModePath).Mode(0o755).Recursive().RunErr(); err != nil {
			t.Fatalf("mkdir %s: %v", wrongModePath, err)
		}

		fixMode := modules.Dir(input.Static(wrongModePath)).Mode(modules.FileMode(0o700)).Build().Node()
		moduletest.AssertDryRun(t, g, fixMode)
		moduletest.AssertConverges(t, g, fixMode)

		stat, err := g.Ops().Stat(wrongModePath).RunErr()
		if err != nil {
			t.Fatalf("stat %s: %v", wrongModePath, err)
		}
		if got, want := stat.Mode.Perm(), fs.FileMode(0o700); got != want {
			t.Errorf("mode = %v, want %v", got, want)
		}
	})
}
