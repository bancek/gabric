package modules_test

import (
	"context"
	"testing"

	"github.com/bancek/gabric/dryrun"
	"github.com/bancek/gabric/input"
	"github.com/bancek/gabric/reconcile"
	"github.com/bancek/gabric/reconcile/modules"
	"github.com/bancek/gabric/reconcile/modules/moduletest"
)

func TestExecEffectE2E(t *testing.T) {
	moduletest.ForEachBaseTarget(t, func(g *moduletest.Target) {
		if !g.POSIXModes {
			return
		}

		t := g.T()
		marker := g.Path("effect.marker")
		topic := reconcile.NewTopic("effect-e2e")
		_ = modules.ExecEffect("effect-e2e").
			Topics(topic).
			Exec(input.List("sh", "-c", `printf x >> "$1"`, "sh", marker)).
			Effect()

		if err := reconcile.Run(g.Context(), func(ctx context.Context) error {
			return topic.Notify(ctx)
		}); err != nil {
			t.Fatalf("first effect run: %v", err)
		}

		content, err := g.Ops().Get(marker).StringErr()
		if err != nil {
			t.Fatalf("read marker: %v", err)
		}
		if content != "x" {
			t.Fatalf("marker after first effect = %q, want x", content)
		}

		if err := reconcile.Run(g.Context(), func(ctx context.Context) error {
			return nil
		}); err != nil {
			t.Fatalf("second effect run: %v", err)
		}

		content, err = g.Ops().Get(marker).StringErr()
		if err != nil {
			t.Fatalf("read marker: %v", err)
		}
		if content != "x" {
			t.Errorf("effect ran again: marker = %q, want x", content)
		}

		dryMarker := g.Path("effect-dry.marker")
		dryTopic := reconcile.NewTopic("effect-e2e-dry")
		_ = modules.ExecEffect("effect-e2e-dry").
			Topics(dryTopic).
			Exec(input.List("sh", "-c", `printf x >> "$1"`, "sh", dryMarker)).
			Effect()

		if err := reconcile.Run(dryrun.With(g.Context()), func(ctx context.Context) error {
			return dryTopic.Notify(ctx)
		}); err != nil {
			t.Fatalf("dry effect run: %v", err)
		}

		if exists, err := g.Ops().Exists(dryMarker).RunErr(); err != nil || exists {
			t.Errorf("dry-run effect marker: exists=%v, err=%v; want false, nil", exists, err)
		}
	})
}
