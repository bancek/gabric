package middleware

import (
	"context"
	"slices"
	"testing"
)

func TestApplyOrder(t *testing.T) {
	var order []string

	run := Apply(
		[]Middleware{
			func(ctx context.Context, next Next) error {
				order = append(order, "middleware1")
				return next(ctx)
			},
			func(ctx context.Context, next Next) error {
				order = append(order, "middleware2")
				return next(ctx)
			},
		},
		func(ctx context.Context) error {
			order = append(order, "task")
			return nil
		},
	)

	if err := run(context.Background()); err != nil {
		t.Fatalf("run error: %v", err)
	}

	want := []string{"middleware1", "middleware2", "task"}
	if !slices.Equal(order, want) {
		t.Errorf("order = %v, want %v", order, want)
	}
}

func TestApplyFuncSlice(t *testing.T) {
	middlewares := []Middleware{
		func(ctx context.Context, next Next) error {
			return next(ctx)
		},
	}

	run := Apply(middlewares, func(ctx context.Context) error {
		return nil
	})
	if err := run(context.Background()); err != nil {
		t.Fatalf("run error: %v", err)
	}
}

func TestApplyShortCircuit(t *testing.T) {
	var ran bool

	run := Apply(
		[]Middleware{
			func(ctx context.Context, next Next) error {
				return nil
			},
		},
		func(ctx context.Context) error {
			ran = true
			return nil
		},
	)

	if err := run(context.Background()); err != nil {
		t.Fatalf("run error: %v", err)
	}

	if ran {
		t.Error("task ran despite a short-circuiting middleware")
	}
}
