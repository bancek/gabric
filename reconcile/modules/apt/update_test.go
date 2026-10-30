package apt

import (
	"strconv"
	"testing"
	"time"

	"github.com/bancek/gabric/dryrun"
	"github.com/bancek/gabric/gabrictest"
	"github.com/bancek/gabric/input"
	"github.com/bancek/gabric/reconcile"
)

func TestUpdate(t *testing.T) {
	gt := gabrictest.New(t)
	gt.Remote().Handle("apt-get", func(gabrictest.Command) gabrictest.Result {
		return gabrictest.Result{}
	})
	collector := &reconcile.ReportCollector{}

	gt.RunNode(reconcile.WithReportCollector(gt.Context(), collector), Update().Build().Node())

	gt.Remote().AssertCommands(gt.Command("apt-get", "update"))
	gt.Remote().AssertCalls()
	assertUpdateReport(t, collector, reconcile.ReportChanged)
}

func TestUpdateFresh(t *testing.T) {
	gt := gabrictest.New(t)
	gt.Remote().Handle("stat", func(gabrictest.Command) gabrictest.Result {
		return gabrictest.Result{Stdout: strconv.FormatInt(time.Now().Unix(), 10)}
	})
	gt.Remote().Handle("apt-get", func(gabrictest.Command) gabrictest.Result {
		t.Error("apt-get ran with a fresh cache")
		return gabrictest.Result{}
	})
	collector := &reconcile.ReportCollector{}

	gt.RunNode(reconcile.WithReportCollector(gt.Context(), collector), Update().MaxAge(input.Static(time.Hour)).Build().Node())

	gt.Remote().AssertCommands(gt.Command("stat", "-c", "%Y", aptCachePaths[0]))
	assertUpdateReport(t, collector, reconcile.ReportOK)
}

func TestUpdateFallbackToLists(t *testing.T) {
	gt := gabrictest.New(t)
	gt.Remote().Handle("stat", func(cmd gabrictest.Command) gabrictest.Result {
		if cmd.HasArg(aptCachePaths[0]) {
			return gabrictest.Result{ExitCode: 1}
		}

		return gabrictest.Result{Stdout: strconv.FormatInt(time.Now().Unix(), 10)}
	})
	gt.Remote().Handle("apt-get", func(gabrictest.Command) gabrictest.Result {
		t.Error("apt-get ran with a fresh cache")
		return gabrictest.Result{}
	})
	collector := &reconcile.ReportCollector{}

	gt.RunNode(reconcile.WithReportCollector(gt.Context(), collector), Update().MaxAge(input.Static(time.Hour)).Build().Node())

	gt.Remote().AssertCommands(
		gt.Command("stat", "-c", "%Y", aptCachePaths[0]),
		gt.Command("stat", "-c", "%Y", aptCachePaths[1]),
	)
	assertUpdateReport(t, collector, reconcile.ReportOK)
}

func TestUpdateStale(t *testing.T) {
	gt := gabrictest.New(t)
	gt.Remote().Handle("stat", func(gabrictest.Command) gabrictest.Result {
		return gabrictest.Result{Stdout: strconv.FormatInt(time.Now().Add(-2*time.Hour).Unix(), 10)}
	})
	gt.Remote().Handle("apt-get", func(gabrictest.Command) gabrictest.Result {
		return gabrictest.Result{}
	})
	collector := &reconcile.ReportCollector{}

	gt.RunNode(reconcile.WithReportCollector(gt.Context(), collector), Update().MaxAge(input.Static(time.Hour)).Build().Node())

	gt.Remote().AssertCommands(
		gt.Command("stat", "-c", "%Y", aptCachePaths[0]),
		gt.Command("stat", "-c", "%Y", aptCachePaths[1]),
		gt.Command("apt-get", "update"),
	)
	assertUpdateReport(t, collector, reconcile.ReportChanged)
}

func TestUpdateMissingCache(t *testing.T) {
	gt := gabrictest.New(t)
	gt.Remote().Handle("stat", func(gabrictest.Command) gabrictest.Result {
		return gabrictest.Result{ExitCode: 1}
	})
	gt.Remote().Handle("apt-get", func(gabrictest.Command) gabrictest.Result {
		return gabrictest.Result{}
	})
	collector := &reconcile.ReportCollector{}

	gt.RunNode(reconcile.WithReportCollector(gt.Context(), collector), Update().MaxAge(input.Static(time.Hour)).Build().Node())

	gt.Remote().AssertCommands(
		gt.Command("stat", "-c", "%Y", aptCachePaths[0]),
		gt.Command("stat", "-c", "%Y", aptCachePaths[1]),
		gt.Command("apt-get", "update"),
	)
	assertUpdateReport(t, collector, reconcile.ReportChanged)
}

func TestUpdateDryRunFresh(t *testing.T) {
	gt := gabrictest.New(t)
	gt.Remote().Handle("stat", func(gabrictest.Command) gabrictest.Result {
		return gabrictest.Result{Stdout: strconv.FormatInt(time.Now().Unix(), 10)}
	})
	gt.Remote().Handle("apt-get", func(gabrictest.Command) gabrictest.Result {
		t.Error("apt-get ran during a dry run")
		return gabrictest.Result{}
	})
	collector := &reconcile.ReportCollector{}

	gt.RunNode(
		dryrun.With(reconcile.WithReportCollector(gt.Context(), collector)),
		Update().MaxAge(input.Static(time.Hour)).Build().Node(),
	)

	gt.Remote().AssertCommands(gt.Command("stat", "-c", "%Y", aptCachePaths[0]))
	assertUpdateReport(t, collector, reconcile.ReportOK)
}

func TestUpdateDryRunStale(t *testing.T) {
	gt := gabrictest.New(t)
	gt.Remote().Handle("stat", func(gabrictest.Command) gabrictest.Result {
		return gabrictest.Result{Stdout: strconv.FormatInt(time.Now().Add(-2*time.Hour).Unix(), 10)}
	})
	gt.Remote().Handle("apt-get", func(gabrictest.Command) gabrictest.Result {
		t.Error("apt-get ran during a dry run")
		return gabrictest.Result{}
	})
	collector := &reconcile.ReportCollector{}

	gt.RunNode(
		dryrun.With(reconcile.WithReportCollector(gt.Context(), collector)),
		Update().MaxAge(input.Static(time.Hour)).Build().Node(),
	)

	gt.Remote().AssertCommands(
		gt.Command("stat", "-c", "%Y", aptCachePaths[0]),
		gt.Command("stat", "-c", "%Y", aptCachePaths[1]),
	)
	assertUpdateReport(t, collector, reconcile.ReportWouldChange)
}

func TestUpdateStatError(t *testing.T) {
	gt := gabrictest.New(t)
	gt.Remote().Handle("stat", func(gabrictest.Command) gabrictest.Result {
		return gabrictest.Result{ExitCode: 2}
	})

	if _, err := gt.RunNodeErr(gt.Context(), Update().MaxAge(input.Static(time.Hour)).Build().Node()); err == nil {
		t.Error("Update() error = nil, want error")
	}
}

func TestUpdateParseError(t *testing.T) {
	gt := gabrictest.New(t)
	gt.Remote().Handle("stat", func(gabrictest.Command) gabrictest.Result {
		return gabrictest.Result{Stdout: "not-a-number\n"}
	})

	if _, err := gt.RunNodeErr(gt.Context(), Update().MaxAge(input.Static(time.Hour)).Build().Node()); err == nil {
		t.Error("Update() error = nil, want error")
	}
}

func assertUpdateReport(t *testing.T, collector *reconcile.ReportCollector, status reconcile.ReportStatus) {
	t.Helper()

	results := collector.Results()
	if len(results) != 1 || results[0].Name != "apt.update" || results[0].Status != status {
		t.Fatalf("report = %#v, want one %s apt.update", results, status)
	}
}
