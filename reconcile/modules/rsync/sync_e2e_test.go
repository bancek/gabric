package rsync_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/bancek/gabric/input"
	"github.com/bancek/gabric/reconcile/modules/moduletest"
	"github.com/bancek/gabric/reconcile/modules/rsync"
)

func TestRsyncE2E(t *testing.T) {
	if _, err := exec.LookPath("rsync"); err != nil {
		t.Skip("rsync is not installed on the host")
	}

	moduletest.ForEachToolsTarget(t, func(g *moduletest.Target) {
		t := g.T()
		src := t.TempDir()
		dest := g.Path("dst")

		writeSource(t, filepath.Join(src, "a.txt"), "one\n")
		writeSource(t, filepath.Join(src, "sub", "b.txt"), "bee\n")

		node := rsync.Sync(input.Static(src + "/")).Remote(input.Static(dest)).Build().Node()

		moduletest.AssertDryRun(t, g, node)
		assertPathExists(t, g, dest, false)

		moduletest.AssertConverges(t, g, node)
		assertContent(t, g, dest+"/a.txt", "one\n")
		assertContent(t, g, dest+"/sub/b.txt", "bee\n")

		satisfied, err := g.RunNodeErr(g.Context(), rsync.SatisfiedData(input.Static(src+"/"), input.Static(dest), input.False, nil))
		if err != nil {
			t.Fatalf("SatisfiedData(): %v", err)
		}
		if !satisfied {
			t.Error("SatisfiedData() = false, want true")
		}

		writeSource(t, filepath.Join(src, "a.txt"), "two changed\n")
		satisfied, err = g.RunNodeErr(g.Context(), rsync.SatisfiedData(input.Static(src+"/"), input.Static(dest), input.False, nil))
		if err != nil {
			t.Fatalf("SatisfiedData(): %v", err)
		}
		if satisfied {
			t.Error("SatisfiedData() = true, want drift")
		}

		moduletest.AssertConverges(t, g, node)
		assertContent(t, g, dest+"/a.txt", "two changed\n")

		putTargetFile(t, g, dest+"/extra.txt", "extra\n")
		del := rsync.Sync(input.Static(src + "/")).Remote(input.Static(dest)).Delete().Build().Node()
		moduletest.AssertConverges(t, g, del)
		assertPathExists(t, g, dest+"/extra.txt", false)

		putTargetFile(t, g, dest+"/keep.txt", "keep\n")
		excluded := rsync.Sync(input.Static(src + "/")).Remote(input.Static(dest)).Delete().Exclude(input.List("keep.txt")).Build().Node()

		changed, err := g.RunNodeErr(g.Context(), excluded)
		if err != nil {
			t.Fatalf("excluded delete: %v", err)
		}
		if changed {
			t.Error("excluded delete changed the target")
		}
		assertContent(t, g, dest+"/keep.txt", "keep\n")
	})
}

func writeSource(t testing.TB, path, content string) {
	t.Helper()

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func putTargetFile(t testing.TB, g *moduletest.Target, path, content string) {
	t.Helper()

	if err := g.Ops().Put(path).EnsureParent().String(content).Mode(0o644).RunErr(); err != nil {
		t.Fatalf("put %s: %v", path, err)
	}
}

func assertContent(t testing.TB, g *moduletest.Target, path, want string) {
	t.Helper()

	got, err := g.Ops().Get(path).StringErr()
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	if got != want {
		t.Errorf("%s = %q, want %q", path, got, want)
	}
}

func assertPathExists(t testing.TB, g *moduletest.Target, path string, want bool) {
	t.Helper()

	got, err := g.Ops().Exists(path).RunErr()
	if err != nil {
		t.Fatalf("exists %s: %v", path, err)
	}
	if got != want {
		t.Errorf("%s exists = %v, want %v", path, got, want)
	}
}
