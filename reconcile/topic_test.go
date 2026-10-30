package reconcile

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
)

func TestNotifyDedup(t *testing.T) {
	ufw := NewTopic("ufw")
	runs := 0

	NewEffect("reload", []*Topic{ufw}, func(ctx context.Context) error {
		runs++
		return nil
	})

	err := Run(context.Background(), func(ctx context.Context) error {
		if err := ufw.Notify(ctx); err != nil {
			return err
		}

		return ufw.Notify(ctx)
	})
	if err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	if runs != 1 {
		t.Errorf("effect ran %d times, want 1", runs)
	}
}

func TestNotifyNoRun(t *testing.T) {
	if err := NewTopic("x").Notify(context.Background()); !errors.Is(err, ErrNoRun) {
		t.Errorf("Notify() error = %v, want ErrNoRun", err)
	}
}

// TestNotifyConcurrent checks that concurrent notifications of one topic still
// collapse into a single flush.
func TestNotifyConcurrent(t *testing.T) {
	ufw := NewTopic("ufw")

	var runs atomic.Int32

	NewEffect("reload", []*Topic{ufw}, func(ctx context.Context) error {
		runs.Add(1)
		return nil
	})

	err := Run(context.Background(), func(ctx context.Context) error {
		var wg sync.WaitGroup

		for range 16 {
			wg.Go(func() {
				_ = ufw.Notify(ctx)
			})
		}

		wg.Wait()

		return nil
	})
	if err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	if got := runs.Load(); got != 1 {
		t.Errorf("effect ran %d times, want 1", got)
	}
}
