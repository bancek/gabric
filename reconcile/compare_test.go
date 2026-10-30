package reconcile

import (
	"context"
	"errors"
	"testing"
)

func TestValueMatches(t *testing.T) {
	n := NewNode("n", func(ctx context.Context) (int, error) { return 2, nil })

	err := Run(context.Background(), func(ctx context.Context) error {
		even := ValueMatches(n, func(v int) bool { return v%2 == 0 })

		ok, err := even(ctx)
		if err != nil {
			return err
		}

		if !ok {
			t.Error("ValueMatches(even) = false, want true")
		}

		odd := ValueMatches(n, func(v int) bool { return v%2 == 1 })

		ok, err = odd(ctx)
		if err != nil {
			return err
		}

		if ok {
			t.Error("ValueMatches(odd) = true, want false")
		}

		return nil
	})
	if err != nil {
		t.Fatalf("Run() error: %v", err)
	}
}

func TestValueMatchesError(t *testing.T) {
	sentinel := errors.New("boom")

	n := NewNode("n", func(ctx context.Context) (int, error) { return 0, sentinel })

	err := Run(context.Background(), func(ctx context.Context) error {
		_, err := ValueMatches(n, func(int) bool { return true })(ctx)

		return err
	})
	if !errors.Is(err, sentinel) {
		t.Errorf("error = %v, want %v", err, sentinel)
	}
}

func TestValueEquals(t *testing.T) {
	n := Data("d", func(ctx context.Context) (string, error) { return "abc", nil }).Node()

	err := Run(context.Background(), func(ctx context.Context) error {
		if ok, err := ValueEquals(n, "abc")(ctx); err != nil || !ok {
			t.Errorf("ValueEquals(abc) = (%v, %v), want (true, nil)", ok, err)
		}

		if ok, err := ValueEquals(n, "def")(ctx); err != nil || ok {
			t.Errorf("ValueEquals(def) = (%v, %v), want (false, nil)", ok, err)
		}

		return nil
	})
	if err != nil {
		t.Fatalf("Run() error: %v", err)
	}
}

func TestNodesEqual(t *testing.T) {
	a := Data("a", func(ctx context.Context) (int, error) { return 1, nil }).Node()
	same := Data("b", func(ctx context.Context) (int, error) { return 1, nil }).Node()
	diff := Data("c", func(ctx context.Context) (int, error) { return 2, nil }).Node()

	err := Run(context.Background(), func(ctx context.Context) error {
		if ok, err := NodesEqual(a, same)(ctx); err != nil || !ok {
			t.Errorf("NodesEqual(a, same) = (%v, %v), want (true, nil)", ok, err)
		}

		if ok, err := NodesEqual(a, diff)(ctx); err != nil || ok {
			t.Errorf("NodesEqual(a, diff) = (%v, %v), want (false, nil)", ok, err)
		}

		return nil
	})
	if err != nil {
		t.Fatalf("Run() error: %v", err)
	}
}

func TestNodesEqualError(t *testing.T) {
	sentinel := errors.New("boom")

	a := Data("a", func(ctx context.Context) (int, error) { return 1, nil }).Node()
	b := Data("b", func(ctx context.Context) (int, error) { return 0, sentinel }).Node()

	err := Run(context.Background(), func(ctx context.Context) error {
		_, err := NodesEqual(a, b)(ctx)

		return err
	})
	if !errors.Is(err, sentinel) {
		t.Errorf("error = %v, want %v", err, sentinel)
	}
}
