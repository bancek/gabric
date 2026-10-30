package reconcile

import (
	"context"
	"errors"
	"fmt"
	"github.com/bancek/gabric/dryrun"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestDryRunSkipsMutationsAndEffects(t *testing.T) {
	mutated := false
	ran := false

	res := NewNode("res", func(ctx context.Context) (struct{}, error) {
		if err := Do(ctx, func() error {
			mutated = true
			return nil
		}); err != nil {
			return struct{}{}, err
		}

		return struct{}{}, nil
	})

	ufw := NewTopic("ufw")

	NewEffect("reload", []*Topic{ufw}, func(ctx context.Context) error {
		ran = true
		return nil
	})

	ctx := dryrun.With(context.Background())

	if err := Run(ctx, func(ctx context.Context) error {
		if _, err := res.Value(ctx); err != nil {
			return err
		}

		return ufw.Notify(ctx)
	}); err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	if mutated {
		t.Error("resource mutated during dry run")
	}

	if ran {
		t.Error("effect ran during dry run")
	}
}

func TestRunIsolation(t *testing.T) {
	calls := 0

	n := NewNode("n", func(ctx context.Context) (int, error) {
		calls++
		return calls, nil
	})

	run := func(want int) {
		if err := Run(context.Background(), func(ctx context.Context) error {
			v, err := n.Value(ctx)
			if err != nil {
				return err
			}

			if v != want {
				t.Errorf("Value() = %d, want %d", v, want)
			}

			return nil
		}); err != nil {
			t.Fatalf("Run() error: %v", err)
		}
	}

	run(1)
	run(2)

	if calls != 2 {
		t.Errorf("node evaluated %d times, want 2 (once per run)", calls)
	}
}

func TestRunCollectsReport(t *testing.T) {
	collector := &ReportCollector{}
	ctx := WithReportCollector(context.Background(), collector)

	n := ResourceBool("res",
		func(ctx context.Context) (bool, error) { return false, nil },
		func(ctx context.Context) error { return nil }).
		Node()

	if _, err := RunNode(ctx, n); err != nil {
		t.Fatalf("RunNode() error: %v", err)
	}

	results := collector.Results()
	if len(results) != 1 {
		t.Fatalf("len(Results()) = %d, want 1", len(results))
	}

	if results[0].Name != "res" || results[0].Status != ReportChanged {
		t.Errorf("Results() = %#v, want changed res", results[0])
	}
}

func TestRunCollectsReportOnError(t *testing.T) {
	collector := &ReportCollector{}
	ctx := WithReportCollector(context.Background(), collector)

	sentinel := errors.New("boom")

	n := Resource("res", nil,
		func(ctx context.Context) (Void, error) { return Void{}, sentinel }).
		Node()

	if _, err := RunNode(ctx, n); !errors.Is(err, sentinel) {
		t.Fatalf("RunNode() error = %v, want %v", err, sentinel)
	}

	results := collector.Results()
	if len(results) != 1 {
		t.Fatalf("len(Results()) = %d, want 1", len(results))
	}

	if results[0].Name != "res" || results[0].Status != ReportFailed || !errors.Is(results[0].Err, sentinel) {
		t.Errorf("Results() = %#v, want failed res with %v", results[0], sentinel)
	}
}

func TestRunNode(t *testing.T) {
	n := NewNode("n", func(ctx context.Context) (int, error) {
		if !InRun(ctx) {
			t.Error("node evaluated outside a run")
		}

		return 42, nil
	})

	got, err := RunNode(context.Background(), n)
	if err != nil {
		t.Fatalf("RunNode() error: %v", err)
	}

	if got != 42 {
		t.Errorf("RunNode() = %d, want 42", got)
	}
}

func TestRunNodeError(t *testing.T) {
	sentinel := errors.New("boom")

	n := NewNode("n", func(ctx context.Context) (int, error) {
		return 0, sentinel
	})

	if _, err := RunNode(context.Background(), n); !errors.Is(err, sentinel) {
		t.Fatalf("RunNode() error = %v, want %v", err, sentinel)
	}
}

func TestRunNodeVoidFlushes(t *testing.T) {
	topic := NewTopic("run-node")
	flushed := 0

	NewEffect("e", []*Topic{topic}, func(ctx context.Context) error {
		flushed++
		return nil
	})

	n := NewNode("n", func(ctx context.Context) (Void, error) {
		return Void{}, topic.Notify(ctx)
	})

	if err := RunNodeVoid(context.Background(), n); err != nil {
		t.Fatalf("RunNodeVoid() error: %v", err)
	}

	if flushed != 1 {
		t.Errorf("effect flushed %d times, want 1", flushed)
	}
}

func TestRunNodeDryRun(t *testing.T) {
	mutated := false
	ran := false

	topic := NewTopic("run-node-dry")

	NewEffect("e", []*Topic{topic}, func(ctx context.Context) error {
		ran = true
		return nil
	})

	n := NewNode("n", func(ctx context.Context) (Void, error) {
		if err := Do(ctx, func() error {
			mutated = true
			return nil
		}); err != nil {
			return Void{}, err
		}

		return Void{}, topic.Notify(ctx)
	})

	if err := RunNodeVoid(dryrun.With(context.Background()), n); err != nil {
		t.Fatalf("RunNodeVoid() error: %v", err)
	}

	if mutated {
		t.Error("node mutated during dry run")
	}

	if ran {
		t.Error("effect ran during dry run")
	}
}

func TestRunNodeFromNode(t *testing.T) {
	inner := NewNode("inner", func(ctx context.Context) (int, error) {
		return 42, nil
	})

	outer := NewNode("outer", func(ctx context.Context) (int, error) {
		return RunNode(ctx, inner)
	})

	got, err := RunNode(context.Background(), outer)
	if err != nil {
		t.Fatalf("RunNode() error: %v", err)
	}

	if got != 42 {
		t.Errorf("RunNode() = %d, want 42", got)
	}
}

func TestManualFlushAndForce(t *testing.T) {
	ufw := NewTopic("ufw")
	runs := 0

	NewEffect("reload", []*Topic{ufw}, func(ctx context.Context) error {
		runs++
		return nil
	})

	if err := Run(context.Background(), func(ctx context.Context) error {
		if err := ufw.Notify(ctx); err != nil {
			return err
		}

		if err := Flush(ctx); err != nil {
			return err
		}

		if runs != 1 {
			t.Errorf("runs after manual flush = %d, want 1", runs)
		}

		return ufw.Notify(ctx)
	}); err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	if runs != 2 {
		t.Errorf("runs = %d, want 2", runs)
	}
}

func TestFlushNoRun(t *testing.T) {
	if err := Flush(context.Background()); !errors.Is(err, ErrNoRun) {
		t.Errorf("Flush() error = %v, want ErrNoRun", err)
	}
}

func TestDoNoRun(t *testing.T) {
	if err := Do(context.Background(), func() error { return nil }); !errors.Is(err, ErrNoRun) {
		t.Errorf("Do() error = %v, want ErrNoRun", err)
	}
}

func TestInRun(t *testing.T) {
	if InRun(context.Background()) {
		t.Error("InRun() = true outside a Run")
	}

	err := Run(context.Background(), func(ctx context.Context) error {
		if !InRun(ctx) {
			t.Error("InRun() = false inside a Run")
		}

		return Run(ctx, func(ctx context.Context) error {
			if !InRun(ctx) {
				t.Error("InRun() = false inside a nested Run")
			}

			return nil
		})
	})
	if err != nil {
		t.Fatalf("Run() error: %v", err)
	}
}

// TestNestedRunIsolated checks that a nested Run gets fresh state and flushes
// its own effects, independent of the ambient run.
func TestNestedRunIsolated(t *testing.T) {
	var evals atomic.Int32

	n := NewNode("n", func(ctx context.Context) (int, error) {
		evals.Add(1)

		return 1, nil
	})

	topic := NewTopic("reload")

	reloads := 0
	innerFlushed := false

	NewEffect("reload", []*Topic{topic}, func(ctx context.Context) error {
		reloads++

		return nil
	})

	err := Run(context.Background(), func(ctx context.Context) error {
		if _, err := n.Value(ctx); err != nil {
			return err
		}

		innerErr := Run(ctx, func(ctx context.Context) error {
			if _, err := n.Value(ctx); err != nil {
				return err
			}

			if err := topic.Notify(ctx); err != nil {
				return err
			}

			if err := Flush(ctx); err != nil {
				return err
			}

			innerFlushed = reloads == 1

			return nil
		})
		if innerErr != nil {
			return innerErr
		}

		_, err := n.Value(ctx)

		return err
	})
	if err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	if got := evals.Load(); got != 2 {
		t.Errorf("node evaluated %d times, want 2 (once per isolated run)", got)
	}

	if !innerFlushed {
		t.Error("inner Run did not flush its own effect")
	}

	if reloads != 1 {
		t.Errorf("effect ran %d times, want 1", reloads)
	}
}

// TestFlushFromNodeFn checks that a node's fn can Flush: its effect runs before
// the node returns.
func TestFlushFromNodeFn(t *testing.T) {
	topic := NewTopic("reload")
	ran := false

	NewEffect("reload", []*Topic{topic}, func(ctx context.Context) error {
		ran = true
		return nil
	})

	flushed := false

	n := NewNode("n", func(ctx context.Context) (struct{}, error) {
		if err := topic.Notify(ctx); err != nil {
			return struct{}{}, err
		}

		if err := Flush(ctx); err != nil {
			return struct{}{}, err
		}

		flushed = ran

		return struct{}{}, nil
	})

	err := Run(context.Background(), func(ctx context.Context) error {
		_, err := n.Value(ctx)

		return err
	})
	if err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	if !flushed {
		t.Error("effect did not run during Flush from a node fn")
	}
}

// TestFlushConcurrentSerialized checks that overlapping Flush calls from
// parallel node fns serialize: effects never execute concurrently.
func TestFlushConcurrentSerialized(t *testing.T) {
	topic := NewTopic("reload")

	var runs, inside, maxInside atomic.Int32

	NewEffect("reload", []*Topic{topic}, func(ctx context.Context) error {
		cur := inside.Add(1)

		for {
			old := maxInside.Load()
			if cur <= old || maxInside.CompareAndSwap(old, cur) {
				break
			}
		}

		time.Sleep(5 * time.Millisecond)
		inside.Add(-1)
		runs.Add(1)

		return nil
	})

	mk := func(name string) *Node[struct{}] {
		return NewNode(name, func(ctx context.Context) (struct{}, error) {
			if err := topic.Notify(ctx); err != nil {
				return struct{}{}, err
			}

			return struct{}{}, Flush(ctx)
		})
	}

	a := mk("a")
	b := mk("b")

	err := Run(context.Background(), func(ctx context.Context) error {
		var wg sync.WaitGroup

		var ea, eb error

		wg.Go(func() { _, ea = a.Value(ctx) })
		wg.Go(func() { _, eb = b.Value(ctx) })
		wg.Wait()

		if ea != nil {
			return ea
		}

		return eb
	})
	if err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	if runs.Load() < 1 {
		t.Error("effect did not run")
	}

	if got := maxInside.Load(); got > 1 {
		t.Errorf("effects ran concurrently (max in flight = %d), want serialized", got)
	}
}

// TestFlushReentrantFromEffect checks that calling Flush from inside an effect is
// reported as a cycle, not a deadlock.
func TestFlushReentrantFromEffect(t *testing.T) {
	topic := NewTopic("reload")

	NewEffect("reload", []*Topic{topic}, func(ctx context.Context) error {
		return Flush(ctx)
	})

	done := make(chan error, 1)

	go func() {
		done <- Run(context.Background(), func(ctx context.Context) error {
			if err := topic.Notify(ctx); err != nil {
				return err
			}

			return Flush(ctx)
		})
	}()

	select {
	case err := <-done:
		if err != nil {
			if !errors.Is(err, ErrCycle) {
				t.Fatalf("Run() error = %v, want ErrCycle", err)
			}
		} else {
			t.Fatal("Run() = nil, want cycle error")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("deadlock: reentrant Flush was not detected")
	}
}

// TestFlushCrossGoroutineCycle checks that a flush whose effect waits on a node
// that is itself flushing is reported as a cycle, not a deadlock.
func TestFlushCrossGoroutineCycle(t *testing.T) {
	topic := NewTopic("reload")

	var x *Node[struct{}]

	effectReadingX := make(chan struct{})

	var once sync.Once

	NewEffect("reload", []*Topic{topic}, func(ctx context.Context) error {
		once.Do(func() { close(effectReadingX) })

		_, err := x.Value(ctx)

		return err
	})

	x = NewNode("x", func(ctx context.Context) (struct{}, error) {
		<-effectReadingX

		return struct{}{}, Flush(ctx)
	})

	o := NewNode("o", func(ctx context.Context) (struct{}, error) {
		if err := topic.Notify(ctx); err != nil {
			return struct{}{}, err
		}

		return struct{}{}, Flush(ctx)
	})

	done := make(chan error, 1)

	go func() {
		done <- Run(context.Background(), func(ctx context.Context) error {
			var wg sync.WaitGroup

			var ex, eo error

			wg.Go(func() { _, ex = x.Value(ctx) })
			wg.Go(func() { _, eo = o.Value(ctx) })
			wg.Wait()

			if eo != nil {
				return eo
			}

			return ex
		})
	}()

	select {
	case err := <-done:
		if err != nil {
			if !errors.Is(err, ErrCycle) {
				t.Fatalf("Run() error = %v, want ErrCycle", err)
			}
		} else {
			t.Fatal("Run() = nil, want cycle error")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("deadlock: cross-goroutine flush cycle was not detected")
	}
}

// TestNestedRunDryRun checks that a nested Run inherits dry-run mode, so Do is a
// no-op.
func TestNestedRunDryRun(t *testing.T) {
	mutated := false

	n := NewNode("n", func(ctx context.Context) (struct{}, error) {
		if err := Do(ctx, func() error {
			mutated = true
			return nil
		}); err != nil {
			return struct{}{}, err
		}

		return struct{}{}, nil
	})

	ctx := dryrun.With(context.Background())

	err := Run(ctx, func(ctx context.Context) error {
		return Run(ctx, func(ctx context.Context) error {
			_, err := n.Value(ctx)

			return err
		})
	})
	if err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	if mutated {
		t.Error("Do ran inside a nested dry run")
	}
}

// TestNestedRunFromNodeFn checks that a node's fn can open an isolated nested
// Run.
func TestNestedRunFromNodeFn(t *testing.T) {
	shared := NewNode("shared", func(ctx context.Context) (int, error) {
		return 1, nil
	})

	inner := NewNode("inner", func(ctx context.Context) (int, error) {
		v, err := shared.Value(ctx)

		return v + 1, err
	})

	outer := NewNode("outer", func(ctx context.Context) (int, error) {
		if _, err := shared.Value(ctx); err != nil {
			return 0, err
		}

		var got int

		err := Run(ctx, func(ctx context.Context) error {
			v, err := inner.Value(ctx)
			got = v

			return err
		})

		return got, err
	})

	err := Run(context.Background(), func(ctx context.Context) error {
		v, err := outer.Value(ctx)
		if err != nil {
			return err
		}

		if v != 2 {
			return fmt.Errorf("outer = %d, want 2", v)
		}

		return nil
	})
	if err != nil {
		t.Fatalf("Run() error: %v", err)
	}
}

// TestMaxTopicFlushesContext checks the flush bound is read from the context and
// inherited by a nested Run.
func TestMaxTopicFlushesContext(t *testing.T) {
	if got := MaxTopicFlushes(context.Background()); got != DefaultMaxTopicFlushes {
		t.Errorf("MaxTopicFlushes() = %d, want %d", got, DefaultMaxTopicFlushes)
	}

	base := WithMaxTopicFlushes(context.Background(), 5)
	if got := MaxTopicFlushes(base); got != 5 {
		t.Errorf("MaxTopicFlushes() = %d, want 5", got)
	}

	topic := NewTopic("t")

	NewEffect("e", []*Topic{topic}, func(ctx context.Context) error {
		return topic.Notify(ctx)
	})

	err := Run(base, func(ctx context.Context) error {
		return Run(ctx, func(ctx context.Context) error {
			return topic.Notify(ctx)
		})
	})
	if !errors.Is(err, ErrMaxTopicFlushes) {
		t.Fatalf("Run() error = %v, want ErrMaxTopicFlushes", err)
	}
}
