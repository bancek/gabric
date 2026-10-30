package reconcile_test

import (
	"context"
	"errors"
	"testing"

	"github.com/bancek/gabric"
	"github.com/bancek/gabric/middleware"
	"github.com/bancek/gabric/reconcile"
)

func TestNodeWithGabricMiddleware(t *testing.T) {
	sudo := gabric.WithRemoteSudo(&gabric.Sudo{Flags: []string{"-n"}})

	node := reconcile.NewNode("sudo", func(ctx context.Context) (bool, error) {
		spec := gabric.GetRemoteSudo(ctx)
		return spec != nil && len(spec.Flags) == 1 && spec.Flags[0] == "-n", nil
	}).With(sudo)

	got, err := reconcile.RunNode(context.Background(), node)
	if err != nil {
		t.Fatalf("RunNode() error: %v", err)
	}
	if !got {
		t.Error("node did not see the gabric middleware")
	}
}

func TestEffectWithGabricMiddleware(t *testing.T) {
	topic := reconcile.NewTopic("sudo-effect")

	var got bool

	reconcile.NewEffect("sudo-effect", []*reconcile.Topic{topic}, func(ctx context.Context) error {
		spec := gabric.GetRemoteSudo(ctx)
		got = spec != nil && len(spec.Flags) == 1 && spec.Flags[0] == "-n"
		return nil
	}).With(gabric.WithRemoteSudo(&gabric.Sudo{Flags: []string{"-n"}}))

	err := reconcile.Run(context.Background(), func(ctx context.Context) error {
		return topic.Notify(ctx)
	})
	if err != nil {
		t.Fatalf("Run() error: %v", err)
	}
	if !got {
		t.Error("effect did not see the gabric middleware")
	}
}

func TestEffectWithGabricMiddlewareError(t *testing.T) {
	topic := reconcile.NewTopic("error")

	errEffect := errors.New("effect failed")

	reconcile.NewEffect("error", []*reconcile.Topic{topic}, func(context.Context) error {
		return errEffect
	}).With(func(ctx context.Context, next middleware.Next) error {
		return next(ctx)
	})

	err := reconcile.Run(context.Background(), func(ctx context.Context) error {
		return topic.Notify(ctx)
	})
	if !errors.Is(err, errEffect) {
		t.Errorf("error = %v, want %v", err, errEffect)
	}
}
