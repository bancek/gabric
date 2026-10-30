package reconcile

import (
	"context"
	"errors"
	"testing"

	"github.com/bancek/gabric/dryrun"
)

func TestDynamicResourceSelectsBuilderAndReportsFacade(t *testing.T) {
	builds := 0
	selectedApplied := 0
	otherApplied := 0

	selected := ResourceBool("selected",
		func(ctx context.Context) (bool, error) { return false, nil },
		func(ctx context.Context) error {
			selectedApplied++
			return nil
		})
	other := ResourceBool("other",
		func(ctx context.Context) (bool, error) { return false, nil },
		func(ctx context.Context) error {
			otherApplied++
			return nil
		})

	useOther := false
	node := DynamicResource("cover", func(ctx context.Context) (*ResourceBuilder[bool], error) {
		builds++

		if useOther {
			return other, nil
		}

		return selected, nil
	}).Node()

	collector := &ReportCollector{}
	if _, err := RunNode(WithReportCollector(context.Background(), collector), node); err != nil {
		t.Fatalf("RunNode() error: %v", err)
	}

	if builds != 1 {
		t.Errorf("build calls = %d, want 1", builds)
	}
	if selectedApplied != 1 {
		t.Errorf("selected apply calls = %d, want 1", selectedApplied)
	}
	if otherApplied != 0 {
		t.Errorf("unselected apply calls = %d, want 0", otherApplied)
	}

	results := collector.Results()
	if len(results) != 1 {
		t.Fatalf("report results = %d, want 1", len(results))
	}
	if results[0].Name != "cover" || results[0].Status != ReportChanged {
		t.Errorf("report = %#v, want changed cover", results[0])
	}
}

func TestDynamicResourceRunsSelectedDependenciesAndEffects(t *testing.T) {
	reads := 0
	notified := false

	dep := Data("dep", func(ctx context.Context) (Void, error) {
		reads++
		return Void{}, nil
	}).Node()

	topic := NewTopic("selected")
	NewEffect("selected", []*Topic{topic}, func(ctx context.Context) error {
		notified = true
		return nil
	})

	selected := ResourceBool("selected",
		func(ctx context.Context) (bool, error) { return false, nil },
		func(ctx context.Context) error { return nil }).
		ApplyNeeds(dep).
		Notify(topic)

	node := DynamicResource("cover", func(ctx context.Context) (*ResourceBuilder[bool], error) {
		return selected, nil
	}).Node()

	if _, err := RunNode(context.Background(), node); err != nil {
		t.Fatalf("RunNode() error: %v", err)
	}

	if reads != 1 {
		t.Errorf("ApplyNeeds reads = %d, want 1", reads)
	}
	if !notified {
		t.Error("selected topic was not notified")
	}
}

func TestDynamicResourceDryRunSkipsSelectedApply(t *testing.T) {
	applied := false

	node := DynamicResource("cover", func(ctx context.Context) (*ResourceBuilder[bool], error) {
		return ResourceBool("selected",
			func(ctx context.Context) (bool, error) { return false, nil },
			func(ctx context.Context) error {
				applied = true
				return nil
			}), nil
	}).Node()

	collector := &ReportCollector{}
	if _, err := RunNode(WithReportCollector(dryrun.With(context.Background()), collector), node); err != nil {
		t.Fatalf("RunNode() error: %v", err)
	}

	if applied {
		t.Error("selected apply ran during a dry run")
	}

	results := collector.Results()
	if len(results) != 1 || results[0].Status != ReportWouldChange {
		t.Errorf("report = %#v, want one would-change resource", results)
	}
}

func TestDynamicResourceSelectionError(t *testing.T) {
	sentinel := errors.New("boom")

	node := DynamicResource("cover", func(ctx context.Context) (*ResourceBuilder[bool], error) {
		return nil, sentinel
	}).Node()

	collector := &ReportCollector{}
	_, err := RunNode(WithReportCollector(context.Background(), collector), node)
	if !errors.Is(err, sentinel) {
		t.Fatalf("RunNode() error = %v, want %v", err, sentinel)
	}

	results := collector.Results()
	if len(results) != 1 {
		t.Fatalf("report results = %d, want 1", len(results))
	}
	if results[0].Name != "cover" || results[0].Status != ReportFailed || !errors.Is(results[0].Err, sentinel) {
		t.Errorf("report = %#v, want failed cover", results[0])
	}
}
