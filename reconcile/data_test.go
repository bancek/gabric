package reconcile

import (
	"context"
	"errors"
	"github.com/bancek/gabric/dryrun"
	"slices"
	"testing"
)

func TestDataMemoized(t *testing.T) {
	calls := 0

	node := Data("data", func(ctx context.Context) (string, error) {
		calls++
		return "x", nil
	}).Node()

	ctx := dryrun.With(context.Background())

	if err := Run(ctx, func(ctx context.Context) error {
		for range 2 { // memoized: evaluated once per run
			if _, err := node.Value(ctx); err != nil {
				return err
			}
		}

		return nil
	}); err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	if calls != 1 {
		t.Errorf("data calls = %d, want 1 (dry run + memoized)", calls)
	}
}

func TestDataNeedsReadBeforeData(t *testing.T) {
	var order []string

	dep := Data("dep", func(ctx context.Context) (struct{}, error) {
		order = append(order, "dep")
		return struct{}{}, nil
	}).Node()

	node := Data("data", func(ctx context.Context) (string, error) {
		order = append(order, "data")
		return "x", nil
	}).Needs(dep).Node()

	if err := Run(context.Background(), func(ctx context.Context) error {
		_, err := node.Value(ctx)
		return err
	}); err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	if want := []string{"dep", "data"}; !slices.Equal(order, want) {
		t.Errorf("order = %v, want %v", order, want)
	}
}

func TestDataErrorPropagates(t *testing.T) {
	sentinel := errors.New("boom")

	node := Data("data", func(ctx context.Context) (Void, error) {
		return Void{}, sentinel
	}).Node()

	err := Run(context.Background(), func(ctx context.Context) error {
		_, err := node.Value(ctx)
		return err
	})

	if !errors.Is(err, sentinel) {
		t.Errorf("Run() error = %v, want %v", err, sentinel)
	}
}

func TestDataNeedsAcceptsGroupCover(t *testing.T) {
	evals := 0
	dep := Data("dep", func(ctx context.Context) (Void, error) {
		evals++
		return Void{}, nil
	}).Node()
	cover := &testGroupCover{Node: dep}

	node := Data("data", func(ctx context.Context) (Void, error) {
		return Void{}, nil
	}).Needs(cover).Node()

	if err := Run(context.Background(), func(ctx context.Context) error {
		_, err := node.Value(ctx)
		return err
	}); err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	if evals != 1 {
		t.Errorf("dependency evals = %d, want 1", evals)
	}
}

// TestDataNeedsNotInvalidated checks that Data.Needs is read but not owned:
// invalidating the data node does not clear its needs.
func TestDataNeedsNotInvalidated(t *testing.T) {
	evals := map[string]int{}

	dep := NewNode("dep", func(ctx context.Context) (int, error) { evals["dep"]++; return 1, nil })

	node := Data("n", func(ctx context.Context) (int, error) { return 0, nil }).Needs(dep).Node()

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

	if evals["dep"] != 1 {
		t.Errorf("Needs target evaluated %d times, want 1 (Needs must not be invalidated)", evals["dep"])
	}
}
