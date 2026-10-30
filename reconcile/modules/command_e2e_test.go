package modules_test

import (
	"testing"

	"github.com/bancek/gabric/input"
	"github.com/bancek/gabric/reconcile/modules"
	"github.com/bancek/gabric/reconcile/modules/moduletest"
)

func TestCommandE2E(t *testing.T) {
	moduletest.ForEachBaseTarget(t, func(g *moduletest.Target) {
		if !g.POSIXModes {
			return
		}

		t := g.T()
		marker := g.Path("command.marker")
		node := modules.Command(input.List("sh", "-c", `printf x >> "$1"`, "sh", marker)).Node()

		moduletest.AssertDryRun(t, g, node)
		if exists, err := g.Ops().Exists(marker).RunErr(); err != nil || exists {
			t.Errorf("marker after dry run: exists=%v, err=%v; want false, nil", exists, err)
		}

		if !g.RunNode(g.Context(), node) {
			t.Error("first Command run = false, want true")
		}
		if !g.RunNode(g.Context(), node) {
			t.Error("second Command run = false, want true")
		}

		content, err := g.Ops().Get(marker).StringErr()
		if err != nil {
			t.Fatalf("read marker: %v", err)
		}
		if content != "xx" {
			t.Errorf("marker = %q, want %q", content, "xx")
		}

		out, err := g.RunNodeErr(g.Context(), modules.CommandOutputData(input.List("sh", "-c", "printf hello")))
		if err != nil {
			t.Fatalf("CommandOutputData(): %v", err)
		}
		if out != "hello" {
			t.Errorf("CommandOutputData() = %q, want hello", out)
		}

		if _, err := g.RunNodeErr(g.Context(), modules.CommandOutputData(input.List("sh", "-c", "exit 2"))); err == nil {
			t.Error("CommandOutputData(exit 2) error = nil, want error")
		}
	})
}
