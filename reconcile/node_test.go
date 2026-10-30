package reconcile

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/bancek/gabric/middleware"
)

func TestNodeMemoized(t *testing.T) {
	calls := 0

	n := NewNode("n", func(ctx context.Context) (int, error) {
		calls++
		return 42, nil
	})

	err := Run(context.Background(), func(ctx context.Context) error {
		v, err := n.Value(ctx)
		if err != nil {
			return err
		}

		if v != 42 {
			t.Errorf("Value() = %d, want 42", v)
		}

		_, err = n.Value(ctx)

		return err
	})
	if err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	if calls != 1 {
		t.Errorf("node evaluated %d times, want 1", calls)
	}
}

func TestNodeEvaluate(t *testing.T) {
	calls := 0

	n := Data("n", func(ctx context.Context) (Void, error) {
		calls++
		return Void{}, nil
	}).Node()

	if err := Run(context.Background(), func(ctx context.Context) error {
		return n.Evaluate(ctx)
	}); err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	if calls != 1 {
		t.Errorf("evaluations = %d, want 1", calls)
	}
}

func TestEvaluateMultiple(t *testing.T) {
	calls := 0

	a := Data("a", func(ctx context.Context) (Void, error) {
		calls++
		return Void{}, nil
	}).Node()
	b := Data("b", func(ctx context.Context) (Void, error) {
		calls++
		return Void{}, nil
	}).Node()

	if err := Run(context.Background(), func(ctx context.Context) error {
		return Evaluate(ctx, a, b)
	}); err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	if calls != 2 {
		t.Errorf("evaluations = %d, want 2", calls)
	}
}

func TestNodeDepsOrder(t *testing.T) {
	var order []string

	a := NewNode("a", func(ctx context.Context) (string, error) {
		order = append(order, "a")
		return "A", nil
	})

	b := NewNode("b", func(ctx context.Context) (string, error) {
		order = append(order, "b")

		v, err := a.Value(ctx)
		if err != nil {
			return "", err
		}

		return "B:" + v, nil
	})

	err := Run(context.Background(), func(ctx context.Context) error {
		v, err := b.Value(ctx)
		if err != nil {
			return err
		}

		if v != "B:A" {
			t.Errorf("Value(b) = %q, want %q", v, "B:A")
		}

		return nil
	})
	if err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	if len(order) != 2 || order[0] != "b" || order[1] != "a" {
		t.Errorf("evaluation order = %v, want [b a]", order)
	}
}

func TestNodeDynamicDeps(t *testing.T) {
	depCalls := 0
	useDep := false

	dep := NewNode("dep", func(ctx context.Context) (int, error) {
		depCalls++
		return 1, nil
	})

	root := NewNode("root", func(ctx context.Context) (int, error) {
		if useDep {
			return dep.Value(ctx)
		}

		return 0, nil
	})

	run := func() {
		if err := Run(context.Background(), func(ctx context.Context) error {
			_, err := root.Value(ctx)
			return err
		}); err != nil {
			t.Fatalf("Run() error: %v", err)
		}
	}

	run()
	if depCalls != 0 {
		t.Errorf("dep evaluated %d times, want 0", depCalls)
	}

	useDep = true

	run()
	if depCalls != 1 {
		t.Errorf("dep evaluated %d times, want 1", depCalls)
	}
}

func TestNodeCycle(t *testing.T) {
	var a, b *Node[int]

	a = NewNode("a", func(ctx context.Context) (int, error) {
		return b.Value(ctx)
	})
	b = NewNode("b", func(ctx context.Context) (int, error) {
		return a.Value(ctx)
	})

	err := Run(context.Background(), func(ctx context.Context) error {
		_, err := a.Value(ctx)
		return err
	})
	if !errors.Is(err, ErrCycle) {
		t.Fatalf("Run() error = %v, want ErrCycle", err)
	}
}

func TestNodeErrorPropagates(t *testing.T) {
	boom := errors.New("boom")

	n := NewNode("n", func(ctx context.Context) (int, error) {
		return 0, boom
	})

	err := Run(context.Background(), func(ctx context.Context) error {
		_, err := n.Value(ctx)
		return err
	})
	if !errors.Is(err, boom) {
		t.Errorf("Run() error = %v, want %v", err, boom)
	}
}

func TestNodeValueNoRun(t *testing.T) {
	n := NewNode("n", func(ctx context.Context) (int, error) {
		return 0, nil
	})

	if _, err := n.Value(context.Background()); !errors.Is(err, ErrNoRun) {
		t.Errorf("Value() error = %v, want ErrNoRun", err)
	}
}

// TestNodeConcurrentSingleFlight checks that concurrent readers of one node
// share a single evaluation.
func TestNodeConcurrentSingleFlight(t *testing.T) {
	var calls atomic.Int32

	started := make(chan struct{})
	release := make(chan struct{})

	n := NewNode("n", func(ctx context.Context) (int, error) {
		if calls.Add(1) == 1 {
			close(started)
		}

		<-release

		return 42, nil
	})

	err := Run(context.Background(), func(ctx context.Context) error {
		const workers = 8

		var wg sync.WaitGroup

		vals := make([]int, workers)
		errs := make([]error, workers)

		for i := range workers {
			wg.Go(func() {
				vals[i], errs[i] = n.Value(ctx)
			})
		}

		<-started
		close(release)
		wg.Wait()

		for i := range workers {
			if errs[i] != nil {
				return errs[i]
			}

			if vals[i] != 42 {
				return fmt.Errorf("Value() = %d, want 42", vals[i])
			}
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

// TestNodeConcurrentSharedDependency is the two-branches-one-dependency shape:
// both roots evaluate concurrently and read the same dependency, which must run
// exactly once with no spurious cycle.
func TestNodeConcurrentSharedDependency(t *testing.T) {
	var depCalls atomic.Int32

	release := make(chan struct{})

	dep := NewNode("dep", func(ctx context.Context) (int, error) {
		depCalls.Add(1)
		<-release

		return 1, nil
	})

	aStarted := make(chan struct{})
	bStarted := make(chan struct{})

	a := NewNode("a", func(ctx context.Context) (int, error) {
		close(aStarted)

		v, err := dep.Value(ctx)

		return v + 1, err
	})
	b := NewNode("b", func(ctx context.Context) (int, error) {
		close(bStarted)

		v, err := dep.Value(ctx)

		return v + 2, err
	})

	err := Run(context.Background(), func(ctx context.Context) error {
		var wg sync.WaitGroup

		var va, vb int
		var ea, eb error

		wg.Go(func() { va, ea = a.Value(ctx) })
		wg.Go(func() { vb, eb = b.Value(ctx) })

		<-aStarted
		<-bStarted
		close(release)
		wg.Wait()

		if ea != nil {
			return ea
		}

		if eb != nil {
			return eb
		}

		if va != 2 || vb != 3 {
			return fmt.Errorf("values = (%d, %d), want (2, 3)", va, vb)
		}

		rs, _ := runFrom(ctx)

		rs.mu.Lock()
		waits := len(rs.waitsOn)
		rs.mu.Unlock()

		if waits != 0 {
			return fmt.Errorf("waitsOn = %d entries after settling, want 0", waits)
		}

		return nil
	})
	if err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	if got := depCalls.Load(); got != 1 {
		t.Errorf("dependency evaluated %d times, want 1", got)
	}
}

// TestNodeErrorMemoized checks that a failed evaluation is cached: later reads
// return the same error without re-running the node.
func TestNodeErrorMemoized(t *testing.T) {
	boom := errors.New("boom")

	var calls atomic.Int32

	n := NewNode("n", func(ctx context.Context) (int, error) {
		calls.Add(1)

		return 0, boom
	})

	err := Run(context.Background(), func(ctx context.Context) error {
		if _, err := n.Value(ctx); !errors.Is(err, boom) {
			return fmt.Errorf("first Value() error = %v, want %v", err, boom)
		}

		if _, err := n.Value(ctx); !errors.Is(err, boom) {
			return fmt.Errorf("second Value() error = %v, want %v", err, boom)
		}

		return nil
	})
	if err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	if got := calls.Load(); got != 1 {
		t.Errorf("node evaluated %d times, want 1 (error must be memoized)", got)
	}
}

// TestNodePanicRecovered checks that a panicking node returns an error and does
// not leave concurrent readers blocked.
func TestNodePanicRecovered(t *testing.T) {
	n := NewNode("n", func(ctx context.Context) (int, error) {
		panic("boom")
	})

	err := Run(context.Background(), func(ctx context.Context) error {
		var wg sync.WaitGroup

		errs := make([]error, 4)

		for i := range errs {
			wg.Go(func() {
				_, errs[i] = n.Value(ctx)
			})
		}

		wg.Wait()

		for i := range errs {
			if errs[i] == nil {
				return errors.New("Value() = nil, want panic error")
			}

			if !strings.Contains(errs[i].Error(), "panicked") {
				return fmt.Errorf("Value() error = %v, want panic error", errs[i])
			}
		}

		return nil
	})
	if err != nil {
		t.Fatalf("Run() error: %v", err)
	}
}

// TestNodeWaitCancellation checks that a node-layer waiter returns ctx.Err()
// when the context is cancelled and clears its wait-for edge.
func TestNodeWaitCancellation(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})

	var startOnce sync.Once

	dep := NewNode("dep", func(ctx context.Context) (int, error) {
		startOnce.Do(func() { close(started) })
		<-release

		return 1, nil
	})

	waiter := NewNode("waiter", func(ctx context.Context) (int, error) {
		return dep.Value(ctx)
	})

	err := Run(context.Background(), func(ctx context.Context) error {
		var wg sync.WaitGroup

		wg.Go(func() { _, _ = dep.Value(ctx) })
		<-started

		waitCtx, cancel := context.WithCancel(ctx)
		cancel()

		_, err := waiter.Value(waitCtx)
		if !errors.Is(err, context.Canceled) {
			close(release)
			wg.Wait()

			return fmt.Errorf("waiter Value() error = %v, want context.Canceled", err)
		}

		rs, _ := runFrom(ctx)

		rs.mu.Lock()
		waits := len(rs.waitsOn)
		rs.mu.Unlock()

		close(release)
		wg.Wait()

		if waits != 0 {
			return fmt.Errorf("waitsOn = %d entries after cancellation, want 0", waits)
		}

		return nil
	})
	if err != nil {
		t.Fatalf("Run() error: %v", err)
	}
}

// TestNodeCrossGoroutineCycle checks that a dependency cycle formed by two
// independently-started evaluations is reported instead of deadlocking.
func TestNodeCrossGoroutineCycle(t *testing.T) {
	var a, b *Node[int]

	var barrier sync.WaitGroup
	barrier.Add(2)

	rendezvous := func() {
		barrier.Done()
		barrier.Wait()
	}

	a = NewNode("a", func(ctx context.Context) (int, error) {
		rendezvous()

		return b.Value(ctx)
	})
	b = NewNode("b", func(ctx context.Context) (int, error) {
		rendezvous()

		return a.Value(ctx)
	})

	done := make(chan error, 1)

	go func() {
		done <- Run(context.Background(), func(ctx context.Context) error {
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
		t.Fatal("deadlock: cross-goroutine cycle was not detected")
	}
}

// TestNodeCrossGoroutineRing checks the same for a three-node ring.
func TestNodeCrossGoroutineRing(t *testing.T) {
	var a, b, c *Node[int]

	var barrier sync.WaitGroup
	barrier.Add(3)

	rendezvous := func() {
		barrier.Done()
		barrier.Wait()
	}

	a = NewNode("a", func(ctx context.Context) (int, error) {
		rendezvous()

		return b.Value(ctx)
	})
	b = NewNode("b", func(ctx context.Context) (int, error) {
		rendezvous()

		return c.Value(ctx)
	})
	c = NewNode("c", func(ctx context.Context) (int, error) {
		rendezvous()

		return a.Value(ctx)
	})

	done := make(chan error, 1)

	go func() {
		done <- Run(context.Background(), func(ctx context.Context) error {
			var wg sync.WaitGroup

			errs := make([]error, 3)

			for i, n := range []*Node[int]{a, b, c} {
				wg.Go(func() { _, errs[i] = n.Value(ctx) })
			}

			wg.Wait()

			for _, err := range errs {
				if err != nil {
					return err
				}
			}

			return nil
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
		t.Fatal("deadlock: cross-goroutine ring was not detected")
	}
}

// TestValueConcurrentFrameDetected checks that calling Value from goroutines a
// node's fn spawns (sharing its evaluation frame) is rejected.
func TestValueConcurrentFrameDetected(t *testing.T) {
	pStarted := make(chan struct{})
	releaseP := make(chan struct{})

	p := NewNode("p", func(ctx context.Context) (int, error) {
		close(pStarted)
		<-releaseP

		return 1, nil
	})

	x := NewNode("x", func(ctx context.Context) (int, error) {
		<-pStarted

		errc := make(chan error, 2)

		for range 2 {
			go func() {
				_, err := p.Value(ctx)
				errc <- err
			}()
		}

		// One spawned goroutine blocks on p; the other trips the guard.
		return 0, <-errc
	})

	err := Run(context.Background(), func(ctx context.Context) error {
		var wg sync.WaitGroup

		wg.Go(func() { _, _ = p.Value(ctx) })

		_, err := x.Value(ctx)

		close(releaseP)
		wg.Wait()

		return err
	})
	if !errors.Is(err, ErrConcurrentUse) {
		t.Fatalf("Run() error = %v, want ErrConcurrentUse", err)
	}
}

// TestValueSpawnedGoroutinesNoWait checks that the shared-frame guard also fires
// for a non-blocking Value call made while a spawned sibling holds the frame.
func TestValueSpawnedGoroutinesNoWait(t *testing.T) {
	pStarted := make(chan struct{})
	releaseP := make(chan struct{})

	p := NewNode("p", func(ctx context.Context) (int, error) {
		close(pStarted)
		<-releaseP

		return 1, nil
	})

	m := NewNode("m", func(ctx context.Context) (int, error) {
		return 1, nil
	})

	var wg sync.WaitGroup

	x := NewNode("x", func(ctx context.Context) (int, error) {
		wg.Go(func() { _, _ = p.Value(ctx) }) // holds x's frame while blocked
		<-pStarted

		// m is memoized, so this does not block: only the busy guard catches it.
		_, err := m.Value(ctx)

		return 0, err
	})

	err := Run(context.Background(), func(ctx context.Context) error {
		if _, err := m.Value(ctx); err != nil {
			return err
		}

		_, err := x.Value(ctx)

		close(releaseP)
		wg.Wait()

		return err
	})
	if !errors.Is(err, ErrConcurrentUse) {
		t.Fatalf("Run() error = %v, want ErrConcurrentUse", err)
	}
}

// TestNodePanicErrorIdentity checks that a node that panics with an error keeps
// errors.Is semantics.
func TestNodePanicErrorIdentity(t *testing.T) {
	sentinel := errors.New("boom")

	n := NewNode("n", func(ctx context.Context) (int, error) {
		panic(sentinel)
	})

	err := Run(context.Background(), func(ctx context.Context) error {
		_, err := n.Value(ctx)

		return err
	})
	if !errors.Is(err, sentinel) {
		t.Errorf("Run() error = %v, want %v", err, sentinel)
	}
}

// TestCloneDistinct checks that a clone shares the name but is memoized
// independently of its original.
func TestCloneDistinct(t *testing.T) {
	calls := 0

	probe := Data("d", func(ctx context.Context) (int, error) {
		calls++
		return calls, nil
	}).Node()
	observed := probe.Clone()

	if observed.Name() != probe.Name() {
		t.Errorf("Clone().Name() = %q, want %q", observed.Name(), probe.Name())
	}

	err := Run(context.Background(), func(ctx context.Context) error {
		pv, err := probe.Value(ctx)
		if err != nil {
			return err
		}

		ov, err := observed.Value(ctx)
		if err != nil {
			return err
		}

		if pv != 1 || ov != 2 {
			t.Errorf("values = (%d, %d), want (1, 2)", pv, ov)
		}

		return nil
	})
	if err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	if calls != 2 {
		t.Errorf("fn calls = %d, want 2 (independent identities)", calls)
	}
}

// TestCloneInvalidateIndependent checks that invalidating one of a clone pair
// does not clear the other.
func TestCloneInvalidateIndependent(t *testing.T) {
	calls := 0

	probe := Data("d", func(ctx context.Context) (int, error) {
		calls++
		return calls, nil
	}).Node()
	observed := probe.Clone()

	err := Run(context.Background(), func(ctx context.Context) error {
		if _, err := probe.Value(ctx); err != nil {
			return err
		}

		if _, err := observed.Value(ctx); err != nil {
			return err
		}

		if err := probe.Invalidate(ctx); err != nil {
			return err
		}

		if _, err := probe.Value(ctx); err != nil {
			return err
		}

		// observed was not invalidated, so it is not re-evaluated.
		_, err := observed.Value(ctx)

		return err
	})
	if err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	if calls != 3 {
		t.Errorf("fn calls = %d, want 3 (probe re-evaluated, observed untouched)", calls)
	}
}

type middlewareContextKey struct{}

func withMiddlewareValue(value string) middleware.Middleware {
	return func(ctx context.Context, next middleware.Next) error {
		return next(context.WithValue(ctx, middlewareContextKey{}, value))
	}
}

func TestNodeWithMiddlewareContext(t *testing.T) {
	read := func(ctx context.Context) (string, error) {
		value, _ := ctx.Value(middlewareContextKey{}).(string)
		return value, nil
	}

	node := NewNode("node", read)
	wrapped := node.With(withMiddlewareValue("wrapped"))

	err := Run(context.Background(), func(ctx context.Context) error {
		original, err := node.Value(ctx)
		if err != nil {
			return err
		}
		if original != "" {
			t.Errorf("original = %q, want empty", original)
		}

		got, err := wrapped.Value(ctx)
		if err != nil {
			return err
		}
		if got != "wrapped" {
			t.Errorf("wrapped = %q, want wrapped", got)
		}

		return nil
	})
	if err != nil {
		t.Fatalf("Run() error: %v", err)
	}
}

func TestNodeWithMiddlewareOrder(t *testing.T) {
	var order []string

	record := func(name string) middleware.Middleware {
		return func(ctx context.Context, next middleware.Next) error {
			order = append(order, name)
			return next(ctx)
		}
	}

	node := NewNode("node", func(context.Context) (int, error) { return 1, nil })
	wrapped := node.With(record("a")).With(record("b"))

	if _, err := RunNode(context.Background(), wrapped); err != nil {
		t.Fatalf("RunNode() error: %v", err)
	}

	want := []string{"a", "b"}
	if !slices.Equal(order, want) {
		t.Errorf("order = %v, want %v", order, want)
	}
}

func TestNodeWithMiddlewareDependencyContext(t *testing.T) {
	dependency := NewNode("dependency", func(ctx context.Context) (string, error) {
		value, _ := ctx.Value(middlewareContextKey{}).(string)
		return value, nil
	})

	node := NewNode("node", func(ctx context.Context) (string, error) {
		return dependency.Value(ctx)
	})
	wrapped := node.With(withMiddlewareValue("wrapped"))

	if got, err := RunNode(context.Background(), wrapped); err != nil {
		t.Fatalf("RunNode() error: %v", err)
	} else if got != "wrapped" {
		t.Errorf("value = %q, want wrapped", got)
	}
}

func TestNodeWithMiddlewareGroupRunsEveryRead(t *testing.T) {
	var calls int

	member := NewNode("member", func(context.Context) (Void, error) { return Void{}, nil })
	group := Group("group").Contains(member).Node()
	wrapped := group.With(func(ctx context.Context, next middleware.Next) error {
		calls++
		return next(ctx)
	})

	err := Run(context.Background(), func(ctx context.Context) error {
		if _, err := wrapped.Value(ctx); err != nil {
			return err
		}
		_, err := wrapped.Value(ctx)
		return err
	})
	if err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	if calls != 2 {
		t.Errorf("middleware calls = %d, want 2", calls)
	}
}

func TestNodeWithMiddlewareRequiresNext(t *testing.T) {
	node := NewNode("node", func(context.Context) (int, error) { return 1, nil })

	_, err := RunNode(context.Background(), node.With(func(context.Context, middleware.Next) error {
		return nil
	}))
	if !errors.Is(err, errMiddlewareNoNext) {
		t.Errorf("error = %v, want errMiddlewareNoNext", err)
	}

	_, err = RunNode(context.Background(), node.With(func(ctx context.Context, next middleware.Next) error {
		if err := next(ctx); err != nil {
			return err
		}
		return next(ctx)
	}))
	if !errors.Is(err, errMiddlewareNextTwice) {
		t.Errorf("error = %v, want errMiddlewareNextTwice", err)
	}
}

func TestNodeWithResourceReportsDistinctIdentity(t *testing.T) {
	resource := ResourceBool("resource",
		func(context.Context) (bool, error) { return true, nil },
		func(context.Context) error { return nil },
	).Node()
	wrapped := resource.With(func(ctx context.Context, next middleware.Next) error {
		return next(ctx)
	})

	var results []ReportResult

	err := Run(context.Background(), func(ctx context.Context) error {
		if _, err := resource.Value(ctx); err != nil {
			return err
		}
		if _, err := wrapped.Value(ctx); err != nil {
			return err
		}

		results = Report(ctx)

		return nil
	})
	if err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	if len(results) != 2 {
		t.Fatalf("Report() len = %d, want 2 distinct identities", len(results))
	}
}
