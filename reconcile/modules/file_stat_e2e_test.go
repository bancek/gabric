package modules_test

import (
	"os"
	"testing"

	"github.com/bancek/gabric/input"
	"github.com/bancek/gabric/reconcile/modules"
	"github.com/bancek/gabric/reconcile/modules/moduletest"
)

func TestFileStatE2E(t *testing.T) {
	moduletest.ForEachBaseTarget(t, func(g *moduletest.Target) {
		t := g.T()
		path := g.Path("stat-file")
		missing := g.Path("stat-missing")

		if err := g.Ops().Put(path).EnsureParent().String("hello\n").Mode(0o644).RunErr(); err != nil {
			t.Fatalf("write %s: %v", path, err)
		}

		stat, err := g.RunNodeErr(g.Context(), modules.FileStatData(input.Static(path)))
		if err != nil {
			t.Fatalf("FileStatData(): %v", err)
		}
		if !stat.Exists {
			t.Fatal("FileStatData() Exists = false, want true")
		}

		missingStat, err := g.RunNodeErr(g.Context(), modules.FileStatData(input.Static(missing)))
		if err != nil {
			t.Fatalf("FileStatData(missing): %v", err)
		}
		if missingStat.Exists {
			t.Error("FileStatData(missing) Exists = true, want false")
		}

		if !g.POSIXModes {
			return
		}

		if got := stat.Mode.Perm(); got != os.FileMode(0o644) {
			t.Errorf("mode = %v, want %v", got, os.FileMode(0o644))
		}

		node := modules.FileStatData(input.Static(path))
		if got, err := g.RunNodeErr(g.Context(), modules.FileModeMatchesData(node, modules.FileMode(0o644))); err != nil || !got {
			t.Errorf("FileModeMatchesData() = %v, %v; want true, nil", got, err)
		}
		if got, err := g.RunNodeErr(g.Context(), modules.FileModeMatchesData(node, modules.FileMode(0o600))); err != nil || got {
			t.Errorf("FileModeMatchesData(wrong) = %v, %v; want false, nil", got, err)
		}
		if got, err := g.RunNodeErr(g.Context(), modules.FileOwnerMatchesData(node, input.Static(g.User), input.Static(g.Group))); err != nil || !got {
			t.Errorf("FileOwnerMatchesData() = %v, %v; want true, nil", got, err)
		}
		if got, err := g.RunNodeErr(g.Context(), modules.FileOwnerMatchesData(node, input.Static("moduletest-missing-user"), input.Static(""))); err != nil || got {
			t.Errorf("FileOwnerMatchesData(wrong) = %v, %v; want false, nil", got, err)
		}
	})
}
