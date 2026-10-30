package modules

import (
	"context"
	"testing"

	"github.com/bancek/gabric/gabrictest"
	"github.com/bancek/gabric/input"
	"github.com/bancek/gabric/reconcile"
)

func TestExecEffect(t *testing.T) {
	gt := gabrictest.New(t)
	gt.Remote().Handle("systemctl", func(gabrictest.Command) gabrictest.Result {
		return gabrictest.Result{}
	})

	topic := reconcile.NewTopic("t")
	_ = ExecEffect("restart-ssh").Topics(topic).Exec(input.List("systemctl", "restart", "ssh")).Effect()

	if err := reconcile.Run(gt.Context(), func(ctx context.Context) error {
		return topic.Notify(ctx)
	}); err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	gt.Remote().AssertCommands(gt.Command("systemctl", "restart", "ssh"))
	gt.Remote().AssertCalls()
}

func TestExecEffectNoNotify(t *testing.T) {
	gt := gabrictest.New(t)
	gt.Remote().Handle("systemctl", func(gabrictest.Command) gabrictest.Result {
		return gabrictest.Result{}
	})

	topic := reconcile.NewTopic("t")
	_ = ExecEffect("restart-ssh").Topics(topic).Exec(input.List("systemctl", "restart", "ssh")).Effect()

	if err := reconcile.Run(gt.Context(), func(ctx context.Context) error { return nil }); err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	gt.Remote().AssertCommands()
	gt.Remote().AssertCalls()
}

func TestExecEffectRequiresTopics(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("expected panic for an effect with no topics")
		}
	}()

	ExecEffect("x").Exec(input.List("true")).Effect()
}

func TestExecEffectRequiresCommand(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("expected panic for an effect with no command")
		}
	}()

	ExecEffect("x").Topics(reconcile.NewTopic("t")).Effect()
}
