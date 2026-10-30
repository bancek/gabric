package reconcile

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/bancek/gabric/middleware"
)

func TestEffectRunsOncePerDirtyTopic(t *testing.T) {
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

func TestEffectNotRunWhenNoNotify(t *testing.T) {
	ufw := NewTopic("ufw")
	runs := 0

	NewEffect("reload", []*Topic{ufw}, func(ctx context.Context) error {
		runs++
		return nil
	})

	if err := Run(context.Background(), func(ctx context.Context) error {
		return nil
	}); err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	if runs != 0 {
		t.Errorf("effect ran %d times, want 0", runs)
	}
}

func TestEffectMultipleTopics(t *testing.T) {
	a := NewTopic("a")
	b := NewTopic("b")
	runs := 0

	NewEffect("e", []*Topic{a, b}, func(ctx context.Context) error {
		runs++
		return nil
	})

	if err := Run(context.Background(), func(ctx context.Context) error {
		return b.Notify(ctx)
	}); err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	if runs != 1 {
		t.Errorf("effect ran %d times, want 1", runs)
	}
}

func TestEffectCascade(t *testing.T) {
	a := NewTopic("a")
	b := NewTopic("b")

	var order []string

	NewEffect("ea", []*Topic{a}, func(ctx context.Context) error {
		order = append(order, "a")
		return b.Notify(ctx)
	})
	NewEffect("eb", []*Topic{b}, func(ctx context.Context) error {
		order = append(order, "b")
		return nil
	})

	if err := Run(context.Background(), func(ctx context.Context) error {
		return a.Notify(ctx)
	}); err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	if len(order) != 2 || order[0] != "a" || order[1] != "b" {
		t.Errorf("effect order = %v, want [a b]", order)
	}
}

func TestEffectCascadeBound(t *testing.T) {
	a := NewTopic("a")

	NewEffect("ea", []*Topic{a}, func(ctx context.Context) error {
		return a.Notify(ctx)
	})

	err := Run(WithMaxTopicFlushes(context.Background(), 5), func(ctx context.Context) error {
		return a.Notify(ctx)
	})
	if !errors.Is(err, ErrMaxTopicFlushes) {
		t.Fatalf("Run() error = %v, want ErrMaxTopicFlushes", err)
	}
}

func TestEffectError(t *testing.T) {
	boom := errors.New("boom")
	a := NewTopic("a")

	NewEffect("ea", []*Topic{a}, func(ctx context.Context) error {
		return boom
	})

	err := Run(context.Background(), func(ctx context.Context) error {
		return a.Notify(ctx)
	})
	if !errors.Is(err, boom) {
		t.Errorf("Run() error = %v, want %v", err, boom)
	}
}

// TestWithMaxTopicFlushesClamp checks that a non-positive bound is clamped to 1
// (a single non-cascading flush succeeds).
func TestWithMaxTopicFlushesClamp(t *testing.T) {
	topic := NewTopic("t")
	runs := 0

	NewEffect("e", []*Topic{topic}, func(ctx context.Context) error {
		runs++
		return nil
	})

	err := Run(WithMaxTopicFlushes(context.Background(), 0), func(ctx context.Context) error {
		return topic.Notify(ctx)
	})
	if err != nil {
		t.Fatalf("Run() error = %v, want nil", err)
	}

	if runs != 1 {
		t.Errorf("effect ran %d times, want 1", runs)
	}
}

// TestNewEffectConcurrent checks that effects can be registered concurrently
// (the topic's effect list is synchronized).
func TestNewEffectConcurrent(t *testing.T) {
	topic := NewTopic("t")

	var runs atomic.Int32

	const count = 16

	var wg sync.WaitGroup

	for range count {
		wg.Go(func() {
			NewEffect("e", []*Topic{topic}, func(ctx context.Context) error {
				runs.Add(1)
				return nil
			})
		})
	}

	wg.Wait()

	err := Run(context.Background(), func(ctx context.Context) error {
		return topic.Notify(ctx)
	})
	if err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	if got := runs.Load(); got != count {
		t.Errorf("effects ran %d times, want %d", got, count)
	}
}

func TestEffectWithMiddlewareContext(t *testing.T) {
	topic := NewTopic("t")
	got := ""

	NewEffect("effect", []*Topic{topic}, func(ctx context.Context) error {
		got, _ = ctx.Value(middlewareContextKey{}).(string)
		return nil
	}).With(withMiddlewareValue("wrapped"))

	err := Run(context.Background(), func(ctx context.Context) error {
		return topic.Notify(ctx)
	})
	if err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	if got != "wrapped" {
		t.Errorf("value = %q, want wrapped", got)
	}
}

func TestEffectWithMiddlewareSkip(t *testing.T) {
	topic := NewTopic("t")
	runs := 0

	NewEffect("effect", []*Topic{topic}, func(context.Context) error {
		runs++
		return nil
	}).With(func(context.Context, middleware.Next) error {
		return nil
	})

	err := Run(context.Background(), func(ctx context.Context) error {
		return topic.Notify(ctx)
	})
	if err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	if runs != 0 {
		t.Errorf("effect ran %d times, want 0", runs)
	}
}

func TestEffectWithMiddlewareNextTwice(t *testing.T) {
	topic := NewTopic("t")

	NewEffect("effect", []*Topic{topic}, func(context.Context) error {
		return nil
	}).With(func(ctx context.Context, next middleware.Next) error {
		if err := next(ctx); err != nil {
			return err
		}
		return next(ctx)
	})

	err := Run(context.Background(), func(ctx context.Context) error {
		return topic.Notify(ctx)
	})
	if !errors.Is(err, errMiddlewareNextTwice) {
		t.Errorf("error = %v, want errMiddlewareNextTwice", err)
	}
}

func TestEffectWithReplacesSubscription(t *testing.T) {
	topic := NewTopic("t")
	runs := 0

	NewEffect("effect", []*Topic{topic}, func(context.Context) error {
		runs++
		return nil
	}).With(func(ctx context.Context, next middleware.Next) error {
		return next(ctx)
	})

	err := Run(context.Background(), func(ctx context.Context) error {
		return topic.Notify(ctx)
	})
	if err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	if runs != 1 {
		t.Errorf("effect ran %d times, want 1", runs)
	}
}
