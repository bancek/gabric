package httpfile_test

import (
	"crypto/sha256"
	"encoding/hex"
	"io/fs"
	"testing"

	"github.com/bancek/gabric/input"
	"github.com/bancek/gabric/reconcile"
	"github.com/bancek/gabric/reconcile/modules"
	"github.com/bancek/gabric/reconcile/modules/httpfile"
	"github.com/bancek/gabric/reconcile/modules/moduletest"
)

func TestHTTPFileE2E(t *testing.T) {
	moduletest.ForEachHTTPTarget(t, func(g *moduletest.Target, server *moduletest.FileServer) {
		t := g.T()

		content := []byte("one\n")
		server.Put("app.bin", content)

		url := server.URL + "/app.bin"
		dest := g.Path("app.bin")
		node := httpfile.File(input.Static(url)).Path(input.Static(dest)).Mode(modules.FileMode(0o644)).SHA256(input.Static(sha256Hex(content))).Build().Node()

		moduletest.AssertDryRun(t, g, node)
		assertFileExists(t, g, dest, false)

		moduletest.AssertConverges(t, g, node)
		moduletest.AssertFile(t, g, dest, content)
		assertFileState(t, g, dest, 0o644)

		content2 := []byte("two\n")
		server.Put("app.bin", content2)

		node2 := httpfile.File(input.Static(url)).Path(input.Static(dest)).Mode(modules.FileMode(0o644)).SHA256(input.Static(sha256Hex(content2))).Build().Node()
		moduletest.AssertConverges(t, g, node2)
		moduletest.AssertFile(t, g, dest, content2)

		noHashDest := g.Path("nohash.bin")
		noHash := httpfile.File(input.Static(url)).Path(input.Static(noHashDest)).Mode(modules.FileMode(0o644)).Build().Node()
		moduletest.AssertConverges(t, g, noHash)
		moduletest.AssertFile(t, g, noHashDest, content2)

		server.Put("app.bin", []byte("three\n"))
		changed, err := g.RunNodeErr(g.Context(), noHash)
		if err != nil {
			t.Fatalf("no-hash run: %v", err)
		}
		if changed {
			t.Error("no-hash file changed after the server content changed")
		}
		moduletest.AssertFile(t, g, noHashDest, content2)

		forceDest := g.Path("force.bin")
		force := httpfile.File(input.Static(url)).Path(input.Static(forceDest)).Mode(modules.FileMode(0o644)).ForceWrite().Build().Node()

		moduletest.AssertDryRun(t, g, force)
		assertHTTPFileChanged(t, g, force, forceDest)
		assertHTTPFileChanged(t, g, force, forceDest)
		moduletest.AssertFile(t, g, forceDest, []byte("three\n"))

		missingDest := g.Path("missing.bin")
		missing := httpfile.File(input.Static(server.URL + "/missing")).Path(input.Static(missingDest)).Build().Node()

		if _, err := g.RunNodeErr(g.Context(), missing); err == nil {
			t.Error("missing URL error = nil, want error")
		}
		assertFileExists(t, g, missingDest, false)
		assertFileExists(t, g, missingDest+".tmp", false)
	})
}

func assertHTTPFileChanged(t testing.TB, g *moduletest.Target, node reconcile.NodeRef, dest string) {
	t.Helper()

	results, err := g.RunReportRef(g.Context(), node)
	if err != nil {
		t.Fatalf("report: %v\n%s", err, reconcile.FormatReport(results))
	}
	if len(results) != 1 || results[0].Name != "httpfile.file:"+dest || results[0].Status != reconcile.ReportChanged {
		t.Fatalf("report = %#v, want one changed httpfile.file:%s", results, dest)
	}
}

func assertFileExists(t testing.TB, g *moduletest.Target, path string, want bool) {
	t.Helper()

	got, err := g.Ops().Exists(path).RunErr()
	if err != nil {
		t.Fatalf("exists %s: %v", path, err)
	}
	if got != want {
		t.Errorf("%s exists = %v, want %v", path, got, want)
	}
}

func assertFileState(t testing.TB, g *moduletest.Target, path string, mode fs.FileMode) {
	t.Helper()

	stat, err := g.Ops().Stat(path).RunErr()
	if err != nil {
		t.Fatalf("stat %s: %v", path, err)
	}
	if got := stat.Mode.Perm(); got != mode {
		t.Errorf("%s mode = %v, want %v", path, got, mode)
	}
	if stat.User != g.User {
		t.Errorf("%s user = %q, want %q", path, stat.User, g.User)
	}
	if stat.Group != g.Group {
		t.Errorf("%s group = %q, want %q", path, stat.Group, g.Group)
	}
}

func sha256Hex(content []byte) string {
	sum := sha256.Sum256(content)

	return hex.EncodeToString(sum[:])
}
