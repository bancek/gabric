package modules_test

import (
	"crypto/sha256"
	"encoding/hex"
	"testing"

	"github.com/bancek/gabric/input"
	"github.com/bancek/gabric/reconcile/modules"
	"github.com/bancek/gabric/reconcile/modules/moduletest"
)

func TestFileDataE2E(t *testing.T) {
	moduletest.ForEachBaseTarget(t, func(g *moduletest.Target) {
		t := g.T()
		path := g.Path("data-file")
		missing := g.Path("data-missing")

		if err := g.Ops().Put(path).EnsureParent().String("hello\n").RunErr(); err != nil {
			t.Fatalf("write %s: %v", path, err)
		}

		if got, err := g.RunNodeErr(g.Context(), modules.FileContentData(input.Static(path))); err != nil || got != "hello\n" {
			t.Errorf("FileContentData() = %q, %v; want %q, nil", got, err, "hello\n")
		}
		if got, err := g.RunNodeErr(g.Context(), modules.FileContentData(input.Static(missing))); err != nil || got != "" {
			t.Errorf("FileContentData(missing) = %q, %v; want empty, nil", got, err)
		}

		if got, err := g.RunNodeErr(g.Context(), modules.PathExistsData(input.Static(path))); err != nil || !got {
			t.Errorf("PathExistsData() = %v, %v; want true, nil", got, err)
		}
		if got, err := g.RunNodeErr(g.Context(), modules.FileExistsData(input.Static(path))); err != nil || !got {
			t.Errorf("FileExistsData() = %v, %v; want true, nil", got, err)
		}
		if got, err := g.RunNodeErr(g.Context(), modules.DirExistsData(input.Static(path))); err != nil || got {
			t.Errorf("DirExistsData(file) = %v, %v; want false, nil", got, err)
		}
		if got, err := g.RunNodeErr(g.Context(), modules.PathExistsData(input.Static(missing))); err != nil || got {
			t.Errorf("PathExistsData(missing) = %v, %v; want false, nil", got, err)
		}

		dir := g.Path("data-dir")
		if err := g.Ops().Mkdir(dir).Recursive().RunErr(); err != nil {
			t.Fatalf("mkdir %s: %v", dir, err)
		}
		if got, err := g.RunNodeErr(g.Context(), modules.DirExistsData(input.Static(dir))); err != nil || !got {
			t.Errorf("DirExistsData() = %v, %v; want true, nil", got, err)
		}
		if got, err := g.RunNodeErr(g.Context(), modules.FileExistsData(input.Static(dir))); err != nil || got {
			t.Errorf("FileExistsData(dir) = %v, %v; want false, nil", got, err)
		}

		sum := sha256.Sum256([]byte("hello\n"))
		wantHash := hex.EncodeToString(sum[:])

		hash, err := g.RunNodeErr(g.Context(), modules.FileHashData(input.Static(path)))
		if err != nil {
			t.Fatalf("FileHashData(): %v", err)
		}
		if hash != wantHash {
			t.Errorf("FileHashData() = %q, want %q", hash, wantHash)
		}
		if got, err := g.RunNodeErr(g.Context(), modules.FileHashData(input.Static(missing))); err != nil || got != "" {
			t.Errorf("FileHashData(missing) = %q, %v; want empty, nil", got, err)
		}

		if got, err := g.RunNodeErr(g.Context(), modules.FileHashMatchesData(modules.FileHashData(input.Static(path)), input.Static(wantHash))); err != nil || !got {
			t.Errorf("FileHashMatchesData() = %v, %v; want true, nil", got, err)
		}
		if got, err := g.RunNodeErr(g.Context(), modules.FileHashMatchesData(modules.FileHashData(input.Static(missing)), input.Static(""))); err != nil || got {
			t.Errorf("FileHashMatchesData(missing) = %v, %v; want false, nil", got, err)
		}
		if got, err := g.RunNodeErr(g.Context(), modules.FileContentMatchesData(modules.FileHashData(input.Static(path)), input.Static([]byte("hello\n")))); err != nil || !got {
			t.Errorf("FileContentMatchesData() = %v, %v; want true, nil", got, err)
		}
	})
}
