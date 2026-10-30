package reconcile

import (
	"context"
	"errors"
	"testing"
)

func TestFilterTrue(t *testing.T) {
	evals := 0

	source := Data("src", func(ctx context.Context) (string, error) { evals++; return "x", nil }).Node()

	node := Filter(source, func(ctx context.Context) (bool, error) { return true, nil })

	var got string

	if err := Run(context.Background(), func(ctx context.Context) error {
		var err error
		got, err = node.Value(ctx)
		return err
	}); err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	if got != "x" {
		t.Errorf("Filter() = %q, want %q", got, "x")
	}

	if evals != 1 {
		t.Errorf("source evaluated %d times, want 1", evals)
	}
}

func TestFilterFalseSkipsSource(t *testing.T) {
	evals := 0

	source := Data("src", func(ctx context.Context) (string, error) { evals++; return "x", nil }).Node()

	node := Filter(source, func(ctx context.Context) (bool, error) { return false, nil })

	var got string

	if err := Run(context.Background(), func(ctx context.Context) error {
		var err error
		got, err = node.Value(ctx)
		return err
	}); err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	if got != "" {
		t.Errorf("Filter() = %q, want zero", got)
	}

	if evals != 0 {
		t.Errorf("source evaluated %d times, want 0 (condition false)", evals)
	}
}

func TestFilterConditionError(t *testing.T) {
	sentinel := errors.New("boom")

	source := Data("src", func(ctx context.Context) (string, error) { return "x", nil }).Node()

	node := Filter(source, func(ctx context.Context) (bool, error) { return false, sentinel })

	err := Run(context.Background(), func(ctx context.Context) error {
		_, err := node.Value(ctx)
		return err
	})

	if !errors.Is(err, sentinel) {
		t.Errorf("Filter() error = %v, want %v", err, sentinel)
	}
}

// TestFilterDoesNotInvalidateSource checks that a filter is not an owner:
// invalidating it does not clear its (hidden) source.
func TestFilterDoesNotInvalidateSource(t *testing.T) {
	evals := 0

	source := Data("src", func(ctx context.Context) (string, error) { evals++; return "x", nil }).Node()

	node := Filter(source, func(ctx context.Context) (bool, error) { return true, nil })

	err := Run(context.Background(), func(ctx context.Context) error {
		if _, err := node.Value(ctx); err != nil {
			return err
		}

		if err := node.Invalidate(ctx); err != nil {
			return err
		}

		_, err := node.Value(ctx)

		return err
	})
	if err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	if evals != 1 {
		t.Errorf("source evaluated %d times, want 1 (the filter does not own it)", evals)
	}
}

func TestIfChanged(t *testing.T) {
	evals := 0

	source := Data("src", func(ctx context.Context) (string, error) { evals++; return "x", nil }).Node()
	cond := Data("cond", func(ctx context.Context) (bool, error) { return false, nil }).Node()

	node := IfChanged(source, cond)

	var got string

	if err := Run(context.Background(), func(ctx context.Context) error {
		var err error
		got, err = node.Value(ctx)
		return err
	}); err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	if got != "" {
		t.Errorf("IfChanged() = %q, want zero", got)
	}

	if evals != 0 {
		t.Errorf("source evaluated %d times, want 0", evals)
	}
}
