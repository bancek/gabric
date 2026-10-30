package reconcile

import (
	"context"
	"errors"
	"github.com/bancek/gabric/dryrun"
	"testing"
)

func TestResourceNoDrift(t *testing.T) {
	applied := false

	node := ResourceBool("res",
		func(ctx context.Context) (bool, error) { return true, nil },
		func(ctx context.Context) error {
			applied = true
			return nil
		}).
		Node()

	if err := Run(context.Background(), func(ctx context.Context) error {
		_, err := node.Value(ctx)
		return err
	}); err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	if applied {
		t.Error("Apply ran for a satisfied resource")
	}
}

func TestResourceDrift(t *testing.T) {
	applied := 0
	reloaded := 0

	topic := NewTopic("reload")

	NewEffect("reload", []*Topic{topic}, func(ctx context.Context) error {
		reloaded++
		return nil
	})

	node := Resource("res",
		func(ctx context.Context) (bool, error) { return false, nil },
		func(ctx context.Context) (string, error) {
			applied++
			return "v1", nil
		}).
		Notify(topic).
		Node()

	var got string

	if err := Run(context.Background(), func(ctx context.Context) error {
		v, err := node.Value(ctx)
		got = v
		return err
	}); err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	if applied != 1 {
		t.Errorf("Apply calls = %d, want 1", applied)
	}

	if got != "v1" {
		t.Errorf("Value() = %q, want %q", got, "v1")
	}

	if reloaded != 1 {
		t.Errorf("effect runs = %d, want 1", reloaded)
	}
}

func TestResourceAlwaysApplies(t *testing.T) {
	applied := 0

	node := ResourceBool("cmd", nil, func(ctx context.Context) error {
		applied++
		return nil
	}).Node()

	if err := Run(context.Background(), func(ctx context.Context) error {
		_, err := node.Value(ctx)
		return err
	}); err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	if applied != 1 {
		t.Errorf("Apply calls = %d, want 1", applied)
	}
}

func TestResourceCheckNeedsBeforeCheck(t *testing.T) {
	reads := 0

	dep := Data("dep", func(ctx context.Context) (struct{}, error) {
		reads++
		return struct{}{}, nil
	}).Node()

	node := ResourceBool("res",
		func(ctx context.Context) (bool, error) { return true, nil },
		func(ctx context.Context) error { return nil }).
		CheckNeeds(dep).
		Node()

	if err := Run(context.Background(), func(ctx context.Context) error {
		_, err := node.Value(ctx)
		return err
	}); err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	if reads != 1 {
		t.Errorf("CheckNeeds reads = %d, want 1 even without drift", reads)
	}
}

func TestResourceApplyNeedsLazy(t *testing.T) {
	reads := 0

	dep := Data("dep", func(ctx context.Context) (struct{}, error) {
		reads++
		return struct{}{}, nil
	}).Node()

	build := func(satisfied bool) *Node[bool] {
		return ResourceBool("res",
			func(ctx context.Context) (bool, error) { return satisfied, nil },
			func(ctx context.Context) error { return nil }).
			ApplyNeeds(dep).
			Node()
	}

	if err := Run(context.Background(), func(ctx context.Context) error {
		_, err := build(true).Value(ctx)
		return err
	}); err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	if reads != 0 {
		t.Errorf("ApplyNeeds read for a satisfied resource (reads = %d, want 0)", reads)
	}

	if err := Run(context.Background(), func(ctx context.Context) error {
		_, err := build(false).Value(ctx)
		return err
	}); err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	if reads != 1 {
		t.Errorf("ApplyNeeds not read for a drifting resource (reads = %d, want 1)", reads)
	}
}

func TestResourceNeedsAcceptGroupCover(t *testing.T) {
	checkReads := 0
	checkDep := Data("check-dep", func(ctx context.Context) (Void, error) {
		checkReads++
		return Void{}, nil
	}).Node()
	checkCover := &testGroupCover{Node: checkDep}

	checkNode := ResourceBool("check-res",
		func(ctx context.Context) (bool, error) { return true, nil },
		func(ctx context.Context) error { return nil }).
		CheckNeeds(checkCover).
		Node()

	if _, err := RunNode(context.Background(), checkNode); err != nil {
		t.Fatalf("check RunNode() error: %v", err)
	}
	if checkReads != 1 {
		t.Errorf("CheckNeeds evals = %d, want 1", checkReads)
	}

	applyReads := 0
	applyDep := Data("apply-dep", func(ctx context.Context) (Void, error) {
		applyReads++
		return Void{}, nil
	}).Node()
	applyCover := &testGroupCover{Node: applyDep}

	applyNode := ResourceBool("apply-res",
		func(ctx context.Context) (bool, error) { return false, nil },
		func(ctx context.Context) error { return nil }).
		ApplyNeeds(applyCover).
		Node()

	if _, err := RunNode(context.Background(), applyNode); err != nil {
		t.Fatalf("apply RunNode() error: %v", err)
	}
	if applyReads != 1 {
		t.Errorf("ApplyNeeds evals = %d, want 1", applyReads)
	}
}

func TestResourceDryRunSkipsApply(t *testing.T) {
	applied := false
	reloaded := false

	topic := NewTopic("reload")

	NewEffect("reload", []*Topic{topic}, func(ctx context.Context) error {
		reloaded = true
		return nil
	})

	node := ResourceBool("res",
		func(ctx context.Context) (bool, error) { return false, nil },
		func(ctx context.Context) error {
			applied = true
			return nil
		}).
		Notify(topic).
		Node()

	if err := Run(dryrun.With(context.Background()), func(ctx context.Context) error {
		_, err := node.Value(ctx)
		return err
	}); err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	if applied {
		t.Error("Apply ran during a dry run")
	}

	if reloaded {
		t.Error("effect ran during a dry run")
	}
}

func TestResourceReportUsesRunDryRun(t *testing.T) {
	applied := false

	node := ResourceBool("res",
		func(ctx context.Context) (bool, error) { return false, nil },
		func(ctx context.Context) error {
			applied = true
			return nil
		}).
		Node()

	var results []ReportResult

	if err := Run(context.Background(), func(ctx context.Context) error {
		// Marking a derived context must not turn an already-active non-dry run
		// into a dry one: Do and the report both follow the run state.
		if _, err := node.Value(dryrun.With(ctx)); err != nil {
			return err
		}

		results = Report(ctx)

		return nil
	}); err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	if !applied {
		t.Error("Apply did not run; the run was not started in dry-run mode")
	}

	if len(results) != 1 || results[0].Status != ReportChanged {
		t.Errorf("report = %#v, want one changed resource", results)
	}
}

func TestResourceBool(t *testing.T) {
	applied := 0

	node := ResourceBool("v", nil, func(ctx context.Context) error {
		applied++
		return nil
	}).Node()

	if err := Run(context.Background(), func(ctx context.Context) error {
		_, err := node.Value(ctx)
		return err
	}); err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	if applied != 1 {
		t.Errorf("apply calls = %d, want 1", applied)
	}
}

func TestResourceImmutable(t *testing.T) {
	notified := 0

	topic := NewTopic("t")

	NewEffect("e", []*Topic{topic}, func(ctx context.Context) error {
		notified++
		return nil
	})

	base := ResourceBool("n", nil, func(ctx context.Context) error { return nil })

	// Extending a base must not change it: the sibling without Notify must not
	// notify.
	a := base.Notify(topic).Node()
	b := base.Node()

	if err := Run(context.Background(), func(ctx context.Context) error {
		_, err := a.Value(ctx)
		return err
	}); err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	if notified != 1 {
		t.Fatalf("notified = %d, want 1", notified)
	}

	notified = 0

	if err := Run(context.Background(), func(ctx context.Context) error {
		_, err := b.Value(ctx)
		return err
	}); err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	if notified != 0 {
		t.Errorf("sibling builder leaked Notify: notified = %d, want 0", notified)
	}
}

func TestResourceApplyRequired(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("expected panic for Resource without Apply")
		}
	}()

	Resource[Void]("r", nil, nil)
}

func TestResourceApplyInvalidates(t *testing.T) {
	reads := 0

	r := Data("r", func(ctx context.Context) (int, error) { reads++; return 1, nil }).Node()

	mutator := ResourceBool("m",
		func(ctx context.Context) (bool, error) { return false, nil },
		func(ctx context.Context) error { return nil }).
		ApplyInvalidates(r).
		Node()

	if err := Run(context.Background(), func(ctx context.Context) error {
		if _, err := r.Value(ctx); err != nil {
			return err
		}

		if _, err := mutator.Value(ctx); err != nil {
			return err
		}

		_, err := r.Value(ctx)

		return err
	}); err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	if reads != 2 {
		t.Errorf("read node evaluated %d times, want 2 (invalidated after apply)", reads)
	}
}

func TestResourceApplyInvalidatesDryRun(t *testing.T) {
	reads := 0

	r := Data("r", func(ctx context.Context) (int, error) { reads++; return 1, nil }).Node()

	mutator := ResourceBool("m",
		func(ctx context.Context) (bool, error) { return false, nil },
		func(ctx context.Context) error { return nil }).
		ApplyInvalidates(r).
		Node()

	if err := Run(dryrun.With(context.Background()), func(ctx context.Context) error {
		if _, err := r.Value(ctx); err != nil {
			return err
		}

		if _, err := mutator.Value(ctx); err != nil {
			return err
		}

		_, err := r.Value(ctx)

		return err
	}); err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	if reads != 1 {
		t.Errorf("read node evaluated %d times, want 1 (a dry run does not invalidate)", reads)
	}
}

func TestResourceApplyInvalidatesSurvivesApply(t *testing.T) {
	reads := 0

	r := Data("r", func(ctx context.Context) (int, error) { reads++; return 1, nil }).Node()

	mutator := ResourceBool("m", nil, func(ctx context.Context) error { return nil }).
		ApplyInvalidates(r).
		Node()

	if err := Run(context.Background(), func(ctx context.Context) error {
		if _, err := r.Value(ctx); err != nil {
			return err
		}

		if _, err := mutator.Value(ctx); err != nil {
			return err
		}

		_, err := r.Value(ctx)

		return err
	}); err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	if reads != 2 {
		t.Errorf("read node evaluated %d times, want 2 (ApplyInvalidates survived Apply)", reads)
	}
}

func TestResourceApplyInvalidatesCycle(t *testing.T) {
	r := Data("r", func(ctx context.Context) (int, error) { return 1, nil }).Node()

	mutator := ResourceBool("m",
		func(ctx context.Context) (bool, error) { return false, nil },
		func(ctx context.Context) error { return nil }).
		ApplyNeeds(r).
		ApplyInvalidates(r).
		Node()

	err := Run(context.Background(), func(ctx context.Context) error {
		_, err := mutator.Value(ctx)
		return err
	})

	if !errors.Is(err, ErrConcurrentUse) {
		t.Errorf("Run() error = %v, want ErrConcurrentUse", err)
	}
}
