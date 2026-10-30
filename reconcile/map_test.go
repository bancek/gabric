package reconcile

import (
	"context"
	"errors"
	"fmt"
	"github.com/bancek/gabric/dryrun"
	"testing"
)

func TestMapTransforms(t *testing.T) {
	src := NewNode("src", func(ctx context.Context) (string, error) { return "hello", nil })

	node := Map(src, func(ctx context.Context, v string) (int, error) {
		return len(v), nil
	})

	var got int

	if err := Run(context.Background(), func(ctx context.Context) error {
		var err error
		got, err = node.Value(ctx)
		return err
	}); err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	if got != 5 {
		t.Errorf("Map() = %d, want 5", got)
	}
}

func TestMapInputError(t *testing.T) {
	wantErr := errors.New("boom")

	src := NewNode("src", func(ctx context.Context) (string, error) { return "", wantErr })

	node := Map(src, func(ctx context.Context, v string) (int, error) {
		return len(v), nil
	})

	err := Run(context.Background(), func(ctx context.Context) error {
		_, err := node.Value(ctx)
		return err
	})

	if !errors.Is(err, wantErr) {
		t.Errorf("Map() error = %v, want %v", err, wantErr)
	}
}

func TestMapFuncError(t *testing.T) {
	wantErr := errors.New("boom")

	src := NewNode("src", func(ctx context.Context) (string, error) { return "x", nil })

	node := Map(src, func(ctx context.Context, v string) (int, error) {
		return 0, wantErr
	})

	err := Run(context.Background(), func(ctx context.Context) error {
		_, err := node.Value(ctx)
		return err
	})

	if !errors.Is(err, wantErr) {
		t.Errorf("Map() error = %v, want %v", err, wantErr)
	}
}

func TestMapMemoized(t *testing.T) {
	calls := 0

	src := NewNode("src", func(ctx context.Context) (string, error) { return "x", nil })

	node := Map(src, func(ctx context.Context, v string) (string, error) {
		calls++
		return v, nil
	})

	if err := Run(context.Background(), func(ctx context.Context) error {
		if _, err := node.Value(ctx); err != nil {
			return err
		}

		_, err := node.Value(ctx)

		return err
	}); err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	if calls != 1 {
		t.Errorf("fn calls = %d, want 1 (memoized per run)", calls)
	}
}

func TestMapDryRun(t *testing.T) {
	ran := false

	src := NewNode("src", func(ctx context.Context) (string, error) { return "x", nil })

	node := Map(src, func(ctx context.Context, v string) (string, error) {
		ran = true
		return v, nil
	})

	if err := Run(dryrun.With(context.Background()), func(ctx context.Context) error {
		_, err := node.Value(ctx)
		return err
	}); err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	if !ran {
		t.Error("Map did not run in a dry run")
	}
}

func TestMapName(t *testing.T) {
	src := NewNode("src", func(ctx context.Context) (string, error) { return "x", nil })

	node := Map(src, func(ctx context.Context, v string) (string, error) {
		return fmt.Sprintf("<%s>", v), nil
	})

	if got, want := node.Name(), "src:map"; got != want {
		t.Errorf("Name() = %q, want %q", got, want)
	}
}
