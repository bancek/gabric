package reconcile

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
)

func TestInvalidateTransitive(t *testing.T) {
	evals := map[string]int{}

	a := NewNode("a", func(ctx context.Context) (int, error) {
		evals["a"]++
		return 1, nil
	})
	b := NewNode("b", func(ctx context.Context) (int, error) {
		v, err := a.Value(ctx)
		evals["b"]++
		return v + 1, err
	})
	c := NewNode("c", func(ctx context.Context) (int, error) {
		v, err := b.Value(ctx)
		evals["c"]++
		return v + 1, err
	})

	err := Run(context.Background(), func(ctx context.Context) error {
		v, err := c.Value(ctx)
		if err != nil {
			return err
		}

		if v != 3 {
			t.Errorf("Value(c) = %d, want 3", v)
		}

		if err := a.Invalidate(ctx); err != nil {
			return err
		}

		v, err = c.Value(ctx)
		if err != nil {
			return err
		}

		if v != 3 {
			t.Errorf("Value(c) after invalidate = %d, want 3", v)
		}

		return nil
	})
	if err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	for _, name := range []string{"a", "b", "c"} {
		if evals[name] != 2 {
			t.Errorf("node %q evaluated %d times, want 2", name, evals[name])
		}
	}
}

func TestInvalidatePartial(t *testing.T) {
	evals := map[string]int{}

	a := NewNode("a", func(ctx context.Context) (int, error) {
		evals["a"]++
		return 1, nil
	})
	b := NewNode("b", func(ctx context.Context) (int, error) {
		v, err := a.Value(ctx)
		evals["b"]++
		return v + 1, err
	})
	c := NewNode("c", func(ctx context.Context) (int, error) {
		v, err := b.Value(ctx)
		evals["c"]++
		return v + 1, err
	})

	err := Run(context.Background(), func(ctx context.Context) error {
		if _, err := c.Value(ctx); err != nil {
			return err
		}

		if err := b.Invalidate(ctx); err != nil {
			return err
		}

		_, err := c.Value(ctx)

		return err
	})
	if err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	if evals["a"] != 1 {
		t.Errorf("node a evaluated %d times, want 1 (upstream untouched)", evals["a"])
	}

	if evals["b"] != 2 || evals["c"] != 2 {
		t.Errorf("evals = %v, want b and c evaluated twice", evals)
	}
}

func TestInvalidateClearsStaleEdges(t *testing.T) {
	useA := true
	evals := map[string]int{}

	a := NewNode("a", func(ctx context.Context) (int, error) {
		evals["a"]++
		return 1, nil
	})
	b := NewNode("b", func(ctx context.Context) (int, error) {
		evals["b"]++

		if useA {
			return a.Value(ctx)
		}

		return 0, nil
	})

	err := Run(context.Background(), func(ctx context.Context) error {
		if _, err := b.Value(ctx); err != nil {
			return err
		}

		// Drop the dependency on a and re-evaluate b.
		useA = false

		if err := b.Invalidate(ctx); err != nil {
			return err
		}

		if _, err := b.Value(ctx); err != nil {
			return err
		}

		// a no longer has b as a dependent, so invalidating a must not touch b.
		if err := a.Invalidate(ctx); err != nil {
			return err
		}

		_, err := b.Value(ctx)

		return err
	})
	if err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	if evals["a"] != 1 {
		t.Errorf("node a evaluated %d times, want 1", evals["a"])
	}

	if evals["b"] != 2 {
		t.Errorf("node b evaluated %d times, want 2", evals["b"])
	}
}

func TestInvalidateNoRun(t *testing.T) {
	n := NewNode("n", func(ctx context.Context) (int, error) {
		return 0, nil
	})

	if err := n.Invalidate(context.Background()); !errors.Is(err, ErrNoRun) {
		t.Errorf("Invalidate() error = %v, want ErrNoRun", err)
	}
}

// TestInvalidateInFlightNode checks that invalidating a node while it is being
// evaluated is rejected and does not cause a second evaluation.
func TestInvalidateInFlightNode(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})

	var calls atomic.Int32

	n := NewNode("n", func(ctx context.Context) (int, error) {
		calls.Add(1)
		close(started)
		<-release

		return 1, nil
	})

	err := Run(context.Background(), func(ctx context.Context) error {
		var wg sync.WaitGroup

		wg.Go(func() { _, _ = n.Value(ctx) })
		<-started

		err := n.Invalidate(ctx)

		close(release)
		wg.Wait()

		if !errors.Is(err, ErrConcurrentUse) {
			return fmt.Errorf("Invalidate() error = %v, want ErrConcurrentUse", err)
		}

		return nil
	})
	if err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	if got := calls.Load(); got != 1 {
		t.Errorf("node evaluated %d times, want 1", got)
	}
}

// TestInvalidateInFlightDependent checks that invalidating a node is rejected
// while a dependent that has read it is being evaluated.
func TestInvalidateInFlightDependent(t *testing.T) {
	a := NewNode("a", func(ctx context.Context) (int, error) {
		return 1, nil
	})

	started := make(chan struct{})
	release := make(chan struct{})

	b := NewNode("b", func(ctx context.Context) (int, error) {
		if _, err := a.Value(ctx); err != nil {
			return 0, err
		}

		close(started)
		<-release

		return 2, nil
	})

	err := Run(context.Background(), func(ctx context.Context) error {
		var wg sync.WaitGroup

		wg.Go(func() { _, _ = b.Value(ctx) })
		<-started

		err := a.Invalidate(ctx)

		close(release)
		wg.Wait()

		if !errors.Is(err, ErrConcurrentUse) {
			return fmt.Errorf("Invalidate() error = %v, want ErrConcurrentUse", err)
		}

		return nil
	})
	if err != nil {
		t.Fatalf("Run() error: %v", err)
	}
}

// TestInvalidateConcurrentDistinct checks that invalidating different nodes from
// different goroutines is serialized and clears both.
func TestInvalidateConcurrentDistinct(t *testing.T) {
	var aEvals, bEvals atomic.Int32

	a := NewNode("a", func(ctx context.Context) (int, error) {
		aEvals.Add(1)
		return 1, nil
	})
	b := NewNode("b", func(ctx context.Context) (int, error) {
		bEvals.Add(1)
		return 1, nil
	})

	err := Run(context.Background(), func(ctx context.Context) error {
		if _, err := a.Value(ctx); err != nil {
			return err
		}
		if _, err := b.Value(ctx); err != nil {
			return err
		}

		var wg sync.WaitGroup

		errs := make([]error, 2)

		wg.Go(func() { errs[0] = a.Invalidate(ctx) })
		wg.Go(func() { errs[1] = b.Invalidate(ctx) })
		wg.Wait()

		for _, err := range errs {
			if err != nil {
				return err
			}
		}

		if _, err := a.Value(ctx); err != nil {
			return err
		}

		_, err := b.Value(ctx)

		return err
	})
	if err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	if aEvals.Load() != 2 || bEvals.Load() != 2 {
		t.Errorf("evals = (%d, %d), want (2, 2)", aEvals.Load(), bEvals.Load())
	}
}

func TestGroupInvalidationDoesNotError(t *testing.T) {
	shared := NewNode("shared", func(ctx context.Context) (int, error) { return 1, nil })

	reader := ResourceBool("reader",
		func(ctx context.Context) (bool, error) {
			_, err := shared.Value(ctx)
			return false, err
		},
		func(ctx context.Context) error { return nil }).Node()

	mutator := ResourceBool("mutator",
		func(ctx context.Context) (bool, error) { return false, nil },
		func(ctx context.Context) error { return nil }).
		ApplyInvalidates(shared).
		Node()

	group := Group("group").Contains(reader, mutator).Node()

	err := Run(context.Background(), func(ctx context.Context) error {
		_, err := group.Value(ctx)
		return err
	})
	if err != nil {
		t.Fatalf("Run() error: %v", err)
	}
}
